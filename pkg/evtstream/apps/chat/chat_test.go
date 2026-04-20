package chat

import (
	"context"
	"testing"
	"time"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	storememory "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/memory"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestChatExampleHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Explain ordinals"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), evtstream.SessionId("chat-1"), CommandStartInference, payload))
	require.NoError(t, engine.WaitIdle(context.Background(), evtstream.SessionId("chat-1")))

	snap, err := hub.Snapshot(context.Background(), evtstream.SessionId("chat-1"))
	require.NoError(t, err)
	require.Equal(t, uint64(5), snap.Ordinal)
	require.Len(t, snap.Entities, 1)
	payloadMap := snap.Entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "finished", payloadMap["status"])
	require.Equal(t, "Answer: Explain ordinals", payloadMap["text"])
}

func TestChatExampleStopPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(10 * time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Stop me"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), evtstream.SessionId("chat-2"), CommandStartInference, payload))
	time.Sleep(12 * time.Millisecond)
	stop, err := structpb.NewStruct(map[string]any{})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), evtstream.SessionId("chat-2"), CommandStopInference, stop))
	require.NoError(t, engine.WaitIdle(context.Background(), evtstream.SessionId("chat-2")))

	snap, err := hub.Snapshot(context.Background(), evtstream.SessionId("chat-2"))
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	payloadMap := snap.Entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "stopped", payloadMap["status"])
	require.Equal(t, false, payloadMap["streaming"])
}

func newTestHub(t *testing.T, engine *Engine) *evtstream.Hub {
	t.Helper()
	reg := evtstream.NewSchemaRegistry()
	require.NoError(t, RegisterSchemas(reg))
	hub, err := evtstream.NewHub(
		evtstream.WithSchemaRegistry(reg),
		evtstream.WithHydrationStore(storememory.New()),
	)
	require.NoError(t, err)
	require.NoError(t, Install(hub, engine))
	return hub
}
