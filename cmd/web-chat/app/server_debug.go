package app

import (
	"net/http"
	"strings"
)

type debugRecordsResponse struct {
	SessionID string        `json:"sessionId"`
	Kind      string        `json:"kind,omitempty"`
	Records   []DebugRecord `json:"records"`
}

func (s *Server) HandleDebugRoutes(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.debugRecorder == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "debug API is not enabled"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	sessionID, action, ok := parseDebugSessionPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	var kind DebugRecordKind
	switch action {
	case "records":
		kind = ""
	case "pipeline":
		kind = DebugRecordKindPipeline
	case "transport":
		kind = DebugRecordKindTransport
	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	writeJSON(w, http.StatusOK, debugRecordsResponse{
		SessionID: sessionID,
		Kind:      string(kind),
		Records:   s.debugRecorder.Records(sessionID, kind),
	})
}

func parseDebugSessionPath(path string) (string, string, bool) {
	const prefix = "/api/debug/sessions/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
