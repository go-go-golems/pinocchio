package webhttp

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	root "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
)

// ChatRequestBody represents the expected JSON body for chat requests.
type ChatRequestBody struct {
	Prompt         string         `json:"prompt"`
	Text           string         `json:"text,omitempty"`
	ConvID         string         `json:"conv_id"`
	Profile        string         `json:"profile,omitempty"`
	Registry       string         `json:"registry,omitempty"`
	Overrides      map[string]any `json:"overrides"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

// ConversationRequestPlan is the canonical output of request policy resolution.
// It captures request data needed for both chat and websocket flows.
type ConversationRequestPlan struct {
	ConvID          string
	RuntimeKey      string
	ResolvedRuntime *gepprofiles.RuntimeSpec
	Overrides       map[string]any
	Prompt          string
	IdempotencyKey  string
}

// ConversationRequestResolver resolves request policy (conv/runtime/overrides) for both HTTP and WS handlers.
type ConversationRequestResolver interface {
	Resolve(req *http.Request) (ConversationRequestPlan, error)
}

// RequestResolutionError is a typed error allowing handlers to choose an HTTP status code
// (or a websocket error frame) without duplicating policy logic.
type RequestResolutionError struct {
	Status    int
	ClientMsg string
	Err       error
}

func (e *RequestResolutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.ClientMsg + ": " + e.Err.Error()
	}
	return e.ClientMsg
}

func (e *RequestResolutionError) Unwrap() error { return e.Err }

// ChatService describes the chat submission surface used by HTTP handlers.
type ChatService interface {
	SubmitPrompt(ctx context.Context, in root.SubmitPromptInput) (root.SubmitPromptResult, error)
}

// StreamService describes websocket attach lifecycle used by HTTP handlers.
type StreamService interface {
	ResolveAndEnsureConversation(ctx context.Context, req root.AppConversationRequest) (*root.ConversationHandle, error)
	AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts root.WebSocketAttachOptions) error
}

// TimelineService describes timeline snapshot reads used by HTTP handlers.
type TimelineService interface {
	Snapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV2, error)
}

// IdempotencyKeyFromRequest resolves idempotency key from headers/body fallback.
func IdempotencyKeyFromRequest(r *http.Request, body *ChatRequestBody) string {
	var key string
	if r != nil {
		key = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			key = strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
		}
	}
	if key == "" && body != nil {
		key = strings.TrimSpace(body.IdempotencyKey)
	}
	if key == "" {
		key = uuid.NewString()
	}
	return key
}

func NewChatHandler(svc ChatService, resolver ConversationRequestResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if svc == nil {
			http.Error(w, "chat service not initialized", http.StatusServiceUnavailable)
			return
		}
		if resolver == nil {
			http.Error(w, "request resolver not initialized", http.StatusInternalServerError)
			return
		}

		plan, err := resolver.Resolve(req)
		if err != nil {
			status := http.StatusInternalServerError
			msg := "failed to resolve request"
			var rbe *RequestResolutionError
			if stderrors.As(err, &rbe) && rbe != nil {
				if rbe.Status > 0 {
					status = rbe.Status
				}
				if strings.TrimSpace(rbe.ClientMsg) != "" {
					msg = rbe.ClientMsg
				}
			}
			http.Error(w, msg, status)
			return
		}
		if strings.TrimSpace(plan.Prompt) == "" {
			http.Error(w, "missing prompt", http.StatusBadRequest)
			return
		}
		idempotencyKey := strings.TrimSpace(plan.IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = IdempotencyKeyFromRequest(req, nil)
		}

		resp, err := svc.SubmitPrompt(req.Context(), root.SubmitPromptInput{
			ConvID:          plan.ConvID,
			RuntimeKey:      plan.RuntimeKey,
			ResolvedRuntime: plan.ResolvedRuntime,
			Overrides:       plan.Overrides,
			Prompt:          plan.Prompt,
			IdempotencyKey:  idempotencyKey,
		})
		if err != nil {
			http.Error(w, "start session inference failed", http.StatusInternalServerError)
			return
		}
		if resp.HTTPStatus > 0 {
			w.WriteHeader(resp.HTTPStatus)
		}
		_ = json.NewEncoder(w).Encode(resp.Response)
	}
}

func NewWSHandler(svc StreamService, resolver ConversationRequestResolver, upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			http.Error(w, "stream service not initialized", http.StatusServiceUnavailable)
			return
		}
		if resolver == nil {
			http.Error(w, "request resolver not initialized", http.StatusInternalServerError)
			return
		}
		plan, err := resolver.Resolve(req)
		if err != nil {
			status := http.StatusInternalServerError
			msg := "failed to resolve request"
			var rbe *RequestResolutionError
			if stderrors.As(err, &rbe) && rbe != nil {
				if rbe.Status > 0 {
					status = rbe.Status
				}
				if strings.TrimSpace(rbe.ClientMsg) != "" {
					msg = rbe.ClientMsg
				}
			}
			http.Error(w, msg, status)
			return
		}

		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			return
		}
		handle, err := svc.ResolveAndEnsureConversation(req.Context(), root.AppConversationRequest{
			ConvID:          plan.ConvID,
			RuntimeKey:      plan.RuntimeKey,
			ResolvedRuntime: plan.ResolvedRuntime,
			Overrides:       plan.Overrides,
		})
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
			_ = conn.Close()
			return
		}
		if err := svc.AttachWebSocket(req.Context(), handle.ConvID, conn, root.WebSocketAttachOptions{
			SendHello:      true,
			HandlePingPong: true,
		}); err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to attach websocket"}`))
			_ = conn.Close()
			return
		}
	}
}

func NewTimelineHandler(svc TimelineService, logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if svc == nil {
			http.Error(w, "timeline service not enabled", http.StatusNotFound)
			return
		}
		convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
		if convID == "" {
			http.Error(w, "missing conv_id", http.StatusBadRequest)
			return
		}

		var sinceVersion uint64
		if s := strings.TrimSpace(req.URL.Query().Get("since_version")); s != "" {
			_, _ = fmt.Sscanf(s, "%d", &sinceVersion)
		}
		limit := 0
		if s := strings.TrimSpace(req.URL.Query().Get("limit")); s != "" {
			var v int
			_, _ = fmt.Sscanf(s, "%d", &v)
			if v > 0 {
				limit = v
			}
		}

		snap, err := svc.Snapshot(req.Context(), convID, sinceVersion, limit)
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Msg("timeline snapshot failed")
			http.Error(w, "timeline snapshot failed", http.StatusInternalServerError)
			return
		}
		out, err := protojson.MarshalOptions{
			EmitUnpopulated: false,
			UseProtoNames:   false,
		}.Marshal(snap)
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Msg("timeline marshal failed")
			http.Error(w, "timeline marshal failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// #nosec G705 -- payload is protobuf-generated JSON served as application/json.
		if _, err := w.Write(out); err != nil {
			logger.Warn().Err(err).Str("conv_id", convID).Msg("timeline write failed")
		}
	}
}
