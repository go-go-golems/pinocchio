package webchat

import (
	"net/http"
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
