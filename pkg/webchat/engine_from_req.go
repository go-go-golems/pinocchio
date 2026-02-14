package webchat

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// ChatRequestBody represents the expected JSON body for chat requests.
type ChatRequestBody struct {
	Prompt         string         `json:"prompt"`
	Text           string         `json:"text,omitempty"`
	ConvID         string         `json:"conv_id"`
	Overrides      map[string]any `json:"overrides"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

// ConversationRequestPlan is the canonical output of request policy resolution.
// It captures request data needed for both chat and websocket flows.
type ConversationRequestPlan struct {
	ConvID         string
	RuntimeKey     string
	Overrides      map[string]any
	Prompt         string
	IdempotencyKey string
}

// ConversationRequestResolver resolves request policy (conv/runtime/overrides) for both HTTP and WS handlers.
type ConversationRequestResolver interface {
	Resolve(req *http.Request) (ConversationRequestPlan, error)
}

// ConversationLookup is the minimal dependency needed to preserve an existing conversation's profile.
type ConversationLookup interface {
	GetConversation(convID string) (*Conversation, bool)
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

type DefaultConversationRequestResolver struct {
	conversations ConversationLookup
}

func NewDefaultConversationRequestResolver(conversations ConversationLookup) *DefaultConversationRequestResolver {
	return &DefaultConversationRequestResolver{conversations: conversations}
}

func (b *DefaultConversationRequestResolver) Resolve(req *http.Request) (ConversationRequestPlan, error) {
	if req == nil {
		return ConversationRequestPlan{}, &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request"}
	}

	switch req.Method {
	case http.MethodGet:
		return b.buildFromWSReq(req)
	case http.MethodPost:
		return b.buildFromChatReq(req)
	default:
		return ConversationRequestPlan{}, &RequestResolutionError{Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed"}
	}
}

func (b *DefaultConversationRequestResolver) buildFromWSReq(req *http.Request) (ConversationRequestPlan, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return ConversationRequestPlan{}, &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "missing conv_id"}
	}

	runtimeKey := strings.TrimSpace(req.URL.Query().Get("runtime"))
	if runtimeKey == "" && b.conversations != nil {
		if existing, ok := b.conversations.GetConversation(convID); ok && existing != nil && strings.TrimSpace(existing.ProfileSlug) != "" {
			runtimeKey = strings.TrimSpace(existing.ProfileSlug)
		}
	}
	if runtimeKey == "" {
		runtimeKey = "default"
	}
	return ConversationRequestPlan{
		ConvID:     convID,
		RuntimeKey: runtimeKey,
	}, nil
}

func (b *DefaultConversationRequestResolver) buildFromChatReq(req *http.Request) (ConversationRequestPlan, error) {
	var body ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return ConversationRequestPlan{}, &RequestResolutionError{Status: http.StatusBadRequest, ClientMsg: "bad request", Err: err}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
		body.ConvID = convID
	}

	runtimeKey := strings.TrimSpace(runtimeKeyFromChatRequest(req))
	if runtimeKey == "" {
		runtimeKey = strings.TrimSpace(req.URL.Query().Get("runtime"))
	}
	if runtimeKey == "" && b.conversations != nil {
		if existing, ok := b.conversations.GetConversation(convID); ok && existing != nil && strings.TrimSpace(existing.ProfileSlug) != "" {
			runtimeKey = strings.TrimSpace(existing.ProfileSlug)
		}
	}
	if runtimeKey == "" {
		runtimeKey = "default"
	}
	return ConversationRequestPlan{
		ConvID:         convID,
		RuntimeKey:     runtimeKey,
		Overrides:      body.Overrides,
		Prompt:         body.Prompt,
		IdempotencyKey: strings.TrimSpace(body.IdempotencyKey),
	}, nil
}

func runtimeKeyFromChatRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	path := req.URL.Path
	if path == "" {
		return ""
	}
	if idx := strings.Index(path, "/chat/"); idx >= 0 {
		rest := path[idx+len("/chat/"):]
		if rest == "" {
			return ""
		}
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[:i]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}
