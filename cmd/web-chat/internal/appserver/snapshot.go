package appserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

type hydrationSnapshotProvider struct {
	server *Server
}

func (p *hydrationSnapshotProvider) Snapshot(_ context.Context, sid sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if p == nil || p.server == nil {
		return sessionstream.Snapshot{}, fmt.Errorf("snapshot provider is not initialized")
	}
	return p.server.Snapshot(sid)
}

func (s *Server) Snapshot(sessionID sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if s == nil || s.service == nil {
		return sessionstream.Snapshot{}, fmt.Errorf("server is not initialized")
	}
	return s.service.Snapshot(context.Background(), sessionID)
}

func (s *Server) handleSessionSnapshot(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	snap, err := s.service.Snapshot(r.Context(), sid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, encodeSnapshotResponse(snap))
}

func encodeSnapshotResponse(snap sessionstream.Snapshot) SessionSnapshotResponse {
	return serverkit.EncodeSnapshotResponse(snap, snapshotStatus)
}

func snapshotStatus(entities []SnapshotEntity) string {
	hasUser := false
	hasNonUserStatus := false
	fallbackStatus := ""
	for i := len(entities) - 1; i >= 0; i-- {
		payload, ok := entities[i].Payload.(map[string]any)
		if !ok {
			continue
		}
		role, _ := payload["role"].(string)
		status, _ := payload["status"].(string)
		if role == "assistant" && status != "" {
			return status
		}
		if role == "user" {
			hasUser = true
			continue
		}
		if status == "streaming" {
			return status
		}
		if status != "" {
			hasNonUserStatus = true
			if fallbackStatus == "" {
				fallbackStatus = status
			}
		}
	}
	if hasUser && hasNonUserStatus {
		return "streaming"
	}
	if fallbackStatus != "" {
		return fallbackStatus
	}
	if hasUser {
		return "streaming"
	}
	return "idle"
}
