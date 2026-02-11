package webchat

import (
	"context"
	"encoding/json"
	"testing"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
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
			"stream_id": "stream-1",
			"data":      data,
		},
	})
	require.NoError(t, err)
	return raw
}

func TestTimelineProjector_ThinkingFinalPreservesDeltaContent(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-thinking", store, nil)

	const msgID = "msg-thinking-1"
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.thinking.start", msgID, 1, map[string]any{
		"id":   msgID,
		"role": "thinking",
	})))
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.thinking.delta", msgID, 2, map[string]any{
		"id":         msgID,
		"delta":      "a",
		"cumulative": "reasoning content",
	})))
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.thinking.final", msgID, 3, map[string]any{
		"id": msgID,
	})))

	snap, err := store.GetSnapshot(context.Background(), "conv-thinking", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)

	entity := snap.Entities[0]
	require.Equal(t, msgID, entity.Id)
	msg := entity.GetMessage()
	require.NotNil(t, msg)
	require.Equal(t, "thinking", msg.Role)
	require.Equal(t, "reasoning content", msg.Content)
	require.False(t, msg.Streaming)
}

func TestTimelineProjector_LlmFinalFallsBackToDeltaContentWhenFinalTextEmpty(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-final-fallback", store, nil)

	const msgID = "msg-final-1"
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.start", msgID, 1, map[string]any{
		"id":   msgID,
		"role": "assistant",
	})))
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.delta", msgID, 2, map[string]any{
		"id":         msgID,
		"delta":      "a",
		"cumulative": "assistant cumulative",
	})))
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.final", msgID, 3, map[string]any{
		"id":   msgID,
		"text": "",
	})))

	snap, err := store.GetSnapshot(context.Background(), "conv-final-fallback", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)

	entity := snap.Entities[0]
	require.Equal(t, msgID, entity.Id)
	msg := entity.GetMessage()
	require.NotNil(t, msg)
	require.Equal(t, "assistant", msg.Role)
	require.Equal(t, "assistant cumulative", msg.Content)
	require.False(t, msg.Streaming)
}
