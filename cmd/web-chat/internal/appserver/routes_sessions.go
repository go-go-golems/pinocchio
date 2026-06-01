package appserver

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

func parseWebChatSessionPath(path string) (string, string, bool) {
	const prefix = "/api/chat/sessions/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		return parts[0], "", true
	}
	if parts[0] == "" {
		return "", "", false
	}
	return parts[0], strings.Join(parts[1:], "/"), true
}

func (s *Server) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in CreateSessionRequest
	if err := serverkit.DecodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	profile := strings.TrimSpace(in.Profile)
	if profile == "" {
		profile = s.defaultProfile
	}
	writeJSON(w, http.StatusOK, CreateSessionResponse{SessionID: uuid.NewString(), Profile: profile})
}

func (s *Server) HandleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	sessionID, action, ok := parseWebChatSessionPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	sid := sessionstream.SessionId(sessionID)
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing session id"})
		return
	}
	if action == "" {
		s.handleSessionSnapshot(w, r, sid)
		return
	}
	if action == "messages" {
		s.handleSubmitMessage(w, r, sid)
		return
	}
	if action == "timeline" {
		s.handleTimelineExport(w, r, sid)
		return
	}
	if action == "turns" {
		s.handleTurnsExport(w, r, sid)
		return
	}
	if action == "export" {
		s.handleFullExport(w, r, sid)
		return
	}
	if action == "tools/manifest" {
		s.handleFrontendToolManifest(w, r, sid)
		return
	}
	if action == "tools/results" {
		s.handleFrontendToolResult(w, r, sid)
		return
	}
	writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
}

func (s *Server) handleSubmitMessage(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in SubmitMessageRequest
	if err := serverkit.DecodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	if strings.TrimSpace(in.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing prompt"})
		return
	}
	var runtime *infruntime.ComposedRuntime
	if s.runtimeResolver != nil {
		resolved, err := s.runtimeResolver.Resolve(r.Context(), r, string(sid), in.Profile, in.Registry)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		runtime = resolved
	}
	if err := s.service.SubmitPromptRequest(r.Context(), sid, chatapp.PromptRequest{
		Prompt:         in.Prompt,
		IdempotencyKey: in.IdempotencyKey,
		Runtime:        runtime,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	profile := strings.TrimSpace(in.Profile)
	if profile == "" {
		profile = s.defaultProfile
	}
	writeJSON(w, http.StatusOK, SubmitMessageResponse{SessionID: string(sid), Accepted: true, Status: "running", Profile: profile})
}
