package chatapp

import (
	"context"
	"testing"
	"time"

	sessionstream "github.com/go-go-golems/sessionstream"
	storememory "github.com/go-go-golems/sessionstream/hydration/memory"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestChatExampleHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Explain ordinals"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-1"), CommandStartInference, payload))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-1")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-1"))
	require.NoError(t, err)
	require.Equal(t, uint64(6), snap.Ordinal)
	require.Len(t, snap.Entities, 2)
	var assistant map[string]any
	var user map[string]any
	for _, entity := range snap.Entities {
		payloadMap := entity.Payload.(*structpb.Struct).AsMap()
		switch payloadMap["role"] {
		case "assistant":
			assistant = payloadMap
		case "user":
			user = payloadMap
		}
	}
	require.Equal(t, "Explain ordinals", user["content"])
	require.Equal(t, "finished", assistant["status"])
	require.Equal(t, "Answer: Explain ordinals", assistant["text"])
}

func TestChatExampleStopPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(10 * time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Stop me"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-2"), CommandStartInference, payload))
	time.Sleep(12 * time.Millisecond)
	stop, err := structpb.NewStruct(map[string]any{})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-2"), CommandStopInference, stop))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-2")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-2"))
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)
	var assistant map[string]any
	for _, entity := range snap.Entities {
		payloadMap := entity.Payload.(*structpb.Struct).AsMap()
		if payloadMap["role"] == "assistant" {
			assistant = payloadMap
		}
	}
	require.Equal(t, "stopped", assistant["status"])
	require.Equal(t, false, assistant["streaming"])
}

func newTestHub(t *testing.T, engine *Engine) *sessionstream.Hub {
	t.Helper()
	reg := sessionstream.NewSchemaRegistry()
	require.NoError(t, RegisterSchemas(reg))
	hub, err := sessionstream.NewHub(
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(storememory.New()),
	)
	require.NoError(t, err)
	require.NoError(t, Install(hub, engine))
	return hub
}
