package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
)

// Error codes in ErrorResponse.Code. Resolver errors that implement
// chatapp.CodedError use their own code.
const (
	CodeInvalidRequest   = "invalid_request"
	CodeRequestTooLarge  = "request_too_large"
	CodeInvalidSessionID = "invalid_session_id"
	CodeSessionNotFound  = "session_not_found"
	CodeSessionExists    = "session_exists"
	CodeUnauthenticated  = "unauthenticated"
	CodeForbidden        = "forbidden"
	CodeRunActive        = "run_active"
	CodeRuntimeError     = "runtime_error"
	CodeNoRuntime        = "no_runtime"
	CodeInternal         = "internal_error"
	// CodeFrontendToolsDisabled answers the tool routes when WithFrontendTools was not given.
	CodeFrontendToolsDisabled = "frontend_tools_disabled"
)

// ErrorResponse is the body of every error answer.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// CreateSessionRequest is the body of POST /sessions. All fields are
// optional; unknown fields are ignored. ID lets the client choose the id
// (1–128 characters of [A-Za-z0-9._:-]); by default the server generates one.
type CreateSessionRequest struct {
	ID     string            `json:"id,omitempty"`
	Title  string            `json:"title,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	// Data is stored as sessions.Info.Data, for the application.
	Data json.RawMessage `json:"data,omitempty"`
}

// SessionResponse answers POST /sessions (no snapshot) and GET /sessions/{id}.
// Running is exact (the engine's state); Snapshot.Status is inferred from the
// timeline entities.
type SessionResponse struct {
	SessionID string                             `json:"sessionId"`
	Session   sessions.Info                      `json:"session"`
	Running   bool                               `json:"running"`
	Snapshot  *serverkit.SessionSnapshotResponse `json:"snapshot,omitempty"`
}

// SessionListResponse answers GET /sessions.
type SessionListResponse struct {
	Sessions []SessionSummary `json:"sessions"`
}

// SessionSummary is a listed session: its metadata and whether it runs.
type SessionSummary struct {
	sessions.Info
	Running bool `json:"running"`
}

// AttachmentRef references an attachment by id. The server passes ids through
// as image attachments without a URL: they are shown to clients but not sent
// to the model. Applications with an upload store resolve them in their
// RuntimeResolver or own route.
type AttachmentRef struct {
	AttachmentID string `json:"attachment_id"`
}

// MessageRequest is the body of POST /sessions/{id}/messages. A message needs
// a prompt or at least one attachment. Profile and Registry select a pinocchio
// profile for resolvers that use one. Raw is the whole body, so a resolver or
// a BlocksFunc can decode the application's own fields from it.
type MessageRequest struct {
	Prompt         string          `json:"prompt"`
	Attachments    []AttachmentRef `json:"attachments,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	Profile        string          `json:"profile,omitempty"`
	Registry       string          `json:"registry,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

// AcceptedResponse answers POST /messages (202) and POST /stop (202).
type AcceptedResponse struct {
	SessionID string `json:"sessionId"`
	Accepted  bool   `json:"accepted"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	serverkit.WriteJSON(w, status, payload)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	msg := code
	if err != nil {
		msg = err.Error()
	}
	writeJSON(w, status, ErrorResponse{Error: msg, Code: code})
}

// decodeBody reads a JSON body of at most maxBodyBytes into v and returns the
// raw bytes. An empty body leaves v unchanged. Unknown fields are ignored.
func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, v any) ([]byte, bool) {
	body, err := readBody(w, r, s.maxBodyBytes)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if asMaxBytes(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeRequestTooLarge, err)
		} else {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, err)
		}
		return nil, false
	}
	if len(body) == 0 {
		return body, true
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, err)
		return nil, false
	}
	return body, true
}
