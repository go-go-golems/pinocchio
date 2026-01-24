package webchat

import (
	"encoding/json"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/google/uuid"
)

type TimelineEvent struct {
	Type      string            `json:"type"`
	EntityID  string            `json:"entityId"`
	Kind      string            `json:"kind,omitempty"`
	Renderer  map[string]string `json:"renderer,omitempty"`
	Props     map[string]any    `json:"props,omitempty"`
	Patch     map[string]any    `json:"patch,omitempty"`
	Result    map[string]any    `json:"result,omitempty"`
	StartedAt int64             `json:"startedAt,omitempty"`
	UpdatedAt int64             `json:"updatedAt,omitempty"`
	Version   int64             `json:"version,omitempty"`
	Flags     map[string]any    `json:"flags,omitempty"`
}

// TimelineEventsFromEvent retained for compatibility if needed by UI
func TimelineEventsFromEvent(e events.Event) [][]byte {
	md := e.Metadata()
	now := time.Now()
	wrap := func(te TimelineEvent) []byte { b, _ := json.Marshal(map[string]any{"tl": true, "event": te}); return b }
	switch ev := e.(type) {
	case *events.EventLog:
		localID := md.ID.String()
		if md.ID == uuid.Nil {
			localID = "log-" + uuid.NewString()
		}
		props := map[string]any{"level": ev.Level, "message": ev.Message}
		if len(ev.Fields) > 0 {
			props["fields"] = ev.Fields
		}
		return [][]byte{wrap(TimelineEvent{Type: "created", EntityID: localID, Kind: "log_event", Renderer: map[string]string{"kind": "log_event"}, Props: props, StartedAt: now.UnixMilli()}), wrap(TimelineEvent{Type: "completed", EntityID: localID, Result: map[string]any{"message": ev.Message}})}
	}
	return nil
}
