package thinkingmode

import (
	"context"
	"encoding/json"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	semregistry "github.com/go-go-golems/pinocchio/pkg/sem/registry"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/stretchr/testify/require"
)

func semFrame(t *testing.T, eventType, id string, seq uint64, data map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"sem": true,
		"event": map[string]any{
			"type":      eventType,
			"id":        id,
			"seq":       seq,
			"stream_id": "stream-thinking",
			"data":      data,
		},
	})
	require.NoError(t, err)
	return raw
}

func decodeSemEvent(t *testing.T, frame []byte) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(frame, &env))
	require.Equal(t, true, env["sem"])
	ev, ok := env["event"].(map[string]any)
	require.True(t, ok)
	return ev
}

func TestRegister_ProjectsThinkingModeTimelineEntities(t *testing.T) {
	webchat.ClearTimelineHandlers()
	resetForTests()
	Register()

	store := chatstore.NewInMemoryTimelineStore(100)
	projector := webchat.NewTimelineProjector("conv-thinking-module", store, nil)

	require.NoError(t, projector.ApplySemFrame(context.Background(), semFrame(t, "thinking.mode.completed", "evt-1", 1, map[string]any{
		"itemId": "thinking-item-1",
		"data": map[string]any{
			"mode":      "deep",
			"phase":     "confirmed",
			"reasoning": "use deep mode for this task",
		},
		"success": true,
		"error":   "",
	})))

	snap, err := store.GetSnapshot(context.Background(), "conv-thinking-module", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, "thinking-item-1", snap.Entities[0].Id)
	require.Equal(t, "thinking_mode", snap.Entities[0].Kind)
	props := snap.Entities[0].Props.AsMap()
	require.Equal(t, "completed", props["status"])
	require.Equal(t, true, props["success"])
	require.Equal(t, "deep", props["mode"])
}

func TestRegister_RegistersThinkingModeSemTranslation(t *testing.T) {
	semregistry.Clear()
	t.Cleanup(semregistry.Clear)
	resetForTests()
	Register()

	ev := NewThinkingModeStarted(gepevents.EventMetadata{}, "thinking-item-2", &ThinkingModePayload{
		Mode:      "chain",
		Phase:     "selection",
		Reasoning: "choose chain for speed",
		ExtraData: map[string]any{"source": "test"},
	})

	frames, found, err := semregistry.Handle(ev)
	require.True(t, found)
	require.NoError(t, err)
	require.Len(t, frames, 1)

	semEv := decodeSemEvent(t, frames[0])
	require.Equal(t, "thinking.mode.started", semEv["type"])
	require.Equal(t, "thinking-item-2", semEv["id"])
	data, ok := semEv["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "thinking-item-2", data["itemId"])
}
