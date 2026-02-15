package webchat

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func NewChatHandler(svc *ConversationService, resolver ConversationRequestResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if svc == nil {
			http.Error(w, "conversation service not initialized", http.StatusServiceUnavailable)
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

		resp, err := svc.SubmitPrompt(req.Context(), SubmitPromptInput{
			ConvID:         plan.ConvID,
			RuntimeKey:     plan.RuntimeKey,
			Overrides:      plan.Overrides,
			Prompt:         plan.Prompt,
			IdempotencyKey: idempotencyKey,
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

func NewWSHandler(svc *ConversationService, resolver ConversationRequestResolver, upgrader websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			http.Error(w, "conversation service not initialized", http.StatusServiceUnavailable)
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
		handle, err := svc.ResolveAndEnsureConversation(req.Context(), AppConversationRequest{
			ConvID:     plan.ConvID,
			RuntimeKey: plan.RuntimeKey,
			Overrides:  plan.Overrides,
		})
		if err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
			_ = conn.Close()
			return
		}
		if err := svc.AttachWebSocket(req.Context(), handle.ConvID, conn, WebSocketAttachOptions{SendHello: true}); err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to attach websocket"}`))
			_ = conn.Close()
			return
		}
	}
}
