package webchat

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// ChatRequestBody represents the expected JSON body for chat requests.
type ChatRequestBody struct {
	Prompt         string         `json:"prompt"`
	Text           string         `json:"text,omitempty"`
	ConvID         string         `json:"conv_id"`
	Overrides      map[string]any `json:"overrides"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

// EngineBuildInput is the canonical output of request policy resolution.
type EngineBuildInput struct {
	ConvID      string
	ProfileSlug string
	Overrides   map[string]any
}

// EngineFromReqBuilder resolves request policy (conv/profile/overrides) for both HTTP and WS handlers.
type EngineFromReqBuilder interface {
	BuildEngineFromReq(req *http.Request) (EngineBuildInput, *ChatRequestBody, error)
}

// ConversationLookup is the minimal dependency needed to preserve an existing conversation's profile.
type ConversationLookup interface {
	GetConversation(convID string) (*Conversation, bool)
}

// RequestBuildError is a typed error allowing handlers to choose an HTTP status code
// (or a websocket error frame) without duplicating policy logic.
type RequestBuildError struct {
	Status    int
	ClientMsg string
	Err       error
}

func (e *RequestBuildError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.ClientMsg + ": " + e.Err.Error()
	}
	return e.ClientMsg
}

func (e *RequestBuildError) Unwrap() error { return e.Err }

type DefaultEngineFromReqBuilder struct {
	profiles      ProfileRegistry
	conversations ConversationLookup
}

func NewDefaultEngineFromReqBuilder(profiles ProfileRegistry, conversations ConversationLookup) *DefaultEngineFromReqBuilder {
	return &DefaultEngineFromReqBuilder{profiles: profiles, conversations: conversations}
}

func (b *DefaultEngineFromReqBuilder) BuildEngineFromReq(req *http.Request) (EngineBuildInput, *ChatRequestBody, error) {
	if req == nil {
		return EngineBuildInput{}, nil, &RequestBuildError{Status: http.StatusBadRequest, ClientMsg: "bad request"}
	}

	switch req.Method {
	case http.MethodGet:
		in, err := b.buildFromWSReq(req)
		return in, nil, err
	case http.MethodPost:
		return b.buildFromChatReq(req)
	default:
		return EngineBuildInput{}, nil, &RequestBuildError{Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed"}
	}
}

func (b *DefaultEngineFromReqBuilder) buildFromWSReq(req *http.Request) (EngineBuildInput, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return EngineBuildInput{}, &RequestBuildError{Status: http.StatusBadRequest, ClientMsg: "missing conv_id"}
	}

	profileSlug := strings.TrimSpace(req.URL.Query().Get("profile"))
	if profileSlug == "" {
		if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
			profileSlug = strings.TrimSpace(ck.Value)
		}
	}
	if profileSlug == "" && b.conversations != nil {
		if existing, ok := b.conversations.GetConversation(convID); ok && existing != nil && strings.TrimSpace(existing.ProfileSlug) != "" {
			profileSlug = strings.TrimSpace(existing.ProfileSlug)
		}
	}
	if profileSlug == "" {
		profileSlug = "default"
	}
	if _, ok := b.profiles.Get(profileSlug); !ok {
		return EngineBuildInput{}, &RequestBuildError{Status: http.StatusNotFound, ClientMsg: "profile not found: " + profileSlug}
	}
	return EngineBuildInput{ConvID: convID, ProfileSlug: profileSlug}, nil
}

func (b *DefaultEngineFromReqBuilder) buildFromChatReq(req *http.Request) (EngineBuildInput, *ChatRequestBody, error) {
	var body ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return EngineBuildInput{}, nil, &RequestBuildError{Status: http.StatusBadRequest, ClientMsg: "bad request", Err: err}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
		body.ConvID = convID
	}

	profileSlug := strings.TrimSpace(profileSlugFromChatRequest(req))
	if profileSlug == "" && b.conversations != nil {
		if existing, ok := b.conversations.GetConversation(convID); ok && existing != nil && strings.TrimSpace(existing.ProfileSlug) != "" {
			profileSlug = strings.TrimSpace(existing.ProfileSlug)
		}
	}
	if profileSlug == "" {
		if ck, err := req.Cookie("chat_profile"); err == nil && ck != nil {
			profileSlug = strings.TrimSpace(ck.Value)
		}
	}
	if profileSlug == "" {
		profileSlug = "default"
	}
	if _, ok := b.profiles.Get(profileSlug); !ok {
		return EngineBuildInput{}, nil, &RequestBuildError{
			Status:    http.StatusNotFound,
			ClientMsg: "unknown profile",
			Err:       errors.New("profile " + profileSlug + " does not exist"),
		}
	}
	return EngineBuildInput{ConvID: convID, ProfileSlug: profileSlug, Overrides: body.Overrides}, &body, nil
}

func profileSlugFromChatRequest(req *http.Request) string {
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
