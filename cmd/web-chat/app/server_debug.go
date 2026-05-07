package app

import (
	"fmt"
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
	sessionID, action, ok := parseDebugSessionPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if action == "reconcile/upload" {
		s.handleDebugReconcileUpload(w, r, sessionID)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
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
	case "reconcile":
		writeJSON(w, http.StatusOK, s.debugRecorder.Reconcile(sessionID))
		return
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

func (s *Server) handleDebugReconcileUpload(w http.ResponseWriter, r *http.Request, sessionID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	body, err := s.debugRecorder.BuildSQLiteReconcileDB(r.Context(), sessionID, r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	filename := fmt.Sprintf("pinocchio-stream-debug-%s.sqlite", safeDownloadName(sessionID))
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
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
	if len(parts) < 2 || parts[0] == "" {
		return "", "", false
	}
	for _, part := range parts[1:] {
		if part == "" {
			return "", "", false
		}
	}
	return parts[0], strings.Join(parts[1:], "/"), true
}

func safeDownloadName(s string) string {
	if s == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return b.String()
}
