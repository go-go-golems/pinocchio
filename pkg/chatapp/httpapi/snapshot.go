package httpapi

import (
	"context"
	"fmt"

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

// Snapshot returns a session's timeline snapshot.
func (s *Server) Snapshot(sessionID sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if s == nil || s.service == nil {
		return sessionstream.Snapshot{}, fmt.Errorf("server is not initialized")
	}
	return s.service.Snapshot(context.Background(), sessionID)
}

func encodeSnapshotResponse(snap sessionstream.Snapshot) serverkit.SessionSnapshotResponse {
	return serverkit.EncodeSnapshotResponse(snap, snapshotStatus)
}

func snapshotStatus(entities []serverkit.SnapshotEntity) string {
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
