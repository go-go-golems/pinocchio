package webchat

import (
	"context"
	"testing"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/stretchr/testify/require"
)

func resetTimelineHandlerRegistryForTest(t *testing.T) {
	t.Helper()
	ClearTimelineHandlers()
	resetTimelineHandlerBootstrapForTests()
	t.Cleanup(func() {
		ClearTimelineHandlers()
		resetTimelineHandlerBootstrapForTests()
		RegisterDefaultTimelineHandlers()
	})
}

func TestRegisterDefaultTimelineHandlers_IsIdempotent(t *testing.T) {
	resetTimelineHandlerRegistryForTest(t)

	RegisterDefaultTimelineHandlers()
	RegisterDefaultTimelineHandlers()

	timelineHandlersMu.RLock()
	defer timelineHandlersMu.RUnlock()
	require.Len(t, timelineHandlers["chat.message"], 1)
	require.Len(t, timelineHandlers["thinking.mode.started"], 1)
	require.Len(t, timelineHandlers["thinking.mode.update"], 1)
	require.Len(t, timelineHandlers["thinking.mode.completed"], 1)
}

func TestTimelineProjector_CustomHandlersRequireBootstrap(t *testing.T) {
	resetTimelineHandlerRegistryForTest(t)

	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-custom-handler-bootstrap", store, nil)
	ctx := context.Background()

	// No bootstrap yet: custom timeline handlers should not project anything.
	require.NoError(t, p.ApplySemFrame(ctx, semFrame(t, "chat.message", "user-turn-1", 1, map[string]any{
		"schemaVersion": 1,
		"role":          "user",
		"content":       "hello",
		"streaming":     false,
	})))
	require.NoError(t, p.ApplySemFrame(ctx, semFrame(t, "thinking.mode.completed", "thinking-evt-1", 2, map[string]any{
		"itemId": "thinking-item-1",
		"data": map[string]any{
			"mode":      "chain",
			"phase":     "complete",
			"reasoning": "done",
		},
		"success": true,
		"error":   "",
	})))

	snap, err := store.GetSnapshot(ctx, "conv-custom-handler-bootstrap", 0, 100)
	require.NoError(t, err)
	require.Empty(t, snap.Entities)

	RegisterDefaultTimelineHandlers()

	require.NoError(t, p.ApplySemFrame(ctx, semFrame(t, "chat.message", "user-turn-2", 3, map[string]any{
		"schemaVersion": 1,
		"role":          "user",
		"content":       "hello after bootstrap",
		"streaming":     false,
	})))
	require.NoError(t, p.ApplySemFrame(ctx, semFrame(t, "thinking.mode.completed", "thinking-evt-2", 4, map[string]any{
		"itemId": "thinking-item-2",
		"data": map[string]any{
			"mode":      "tree",
			"phase":     "complete",
			"reasoning": "reasoning complete",
		},
		"success": true,
		"error":   "",
	})))

	snap, err = store.GetSnapshot(ctx, "conv-custom-handler-bootstrap", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)

	byID := map[string]string{}
	for _, e := range snap.Entities {
		byID[e.Id] = e.Kind
	}
	require.Equal(t, "message", byID["user-turn-2"])
	require.Equal(t, "thinking_mode", byID["thinking-item-2"])
}
