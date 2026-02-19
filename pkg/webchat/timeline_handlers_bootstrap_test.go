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
}

func TestTimelineProjector_ChatMessageHandlerRequiresBootstrap(t *testing.T) {
	resetTimelineHandlerRegistryForTest(t)

	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-chat-handler-bootstrap", store, nil)
	ctx := context.Background()

	// No bootstrap yet: builtin chat.message handler should not project anything.
	require.NoError(t, p.ApplySemFrame(ctx, semFrame(t, "chat.message", "user-turn-1", 1, map[string]any{
		"schemaVersion": 1,
		"role":          "user",
		"content":       "hello",
		"streaming":     false,
	})))

	snap, err := store.GetSnapshot(ctx, "conv-chat-handler-bootstrap", 0, 100)
	require.NoError(t, err)
	require.Empty(t, snap.Entities)

	RegisterDefaultTimelineHandlers()

	require.NoError(t, p.ApplySemFrame(ctx, semFrame(t, "chat.message", "user-turn-2", 3, map[string]any{
		"schemaVersion": 1,
		"role":          "user",
		"content":       "hello after bootstrap",
		"streaming":     false,
	})))

	snap, err = store.GetSnapshot(ctx, "conv-chat-handler-bootstrap", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, "user-turn-2", snap.Entities[0].Id)
	require.Equal(t, "message", snap.Entities[0].Kind)
}
