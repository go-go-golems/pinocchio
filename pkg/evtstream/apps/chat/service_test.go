package chat

import (
	"context"
	"testing"
	"time"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestServiceSubmitPromptHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	svc, err := NewService(hub, engine)
	require.NoError(t, err)

	require.NoError(t, svc.SubmitPrompt(context.Background(), evtstream.SessionId("svc-chat-1"), "Explain ordinals"))
	require.NoError(t, svc.WaitIdle(context.Background(), evtstream.SessionId("svc-chat-1")))

	snap, err := svc.Snapshot(context.Background(), evtstream.SessionId("svc-chat-1"))
	require.NoError(t, err)
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

func TestServiceStopPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(10 * time.Millisecond))
	hub := newTestHub(t, engine)
	svc, err := NewService(hub, engine)
	require.NoError(t, err)

	require.NoError(t, svc.SubmitPrompt(context.Background(), evtstream.SessionId("svc-chat-2"), "Stop me"))
	time.Sleep(12 * time.Millisecond)
	require.NoError(t, svc.Stop(context.Background(), evtstream.SessionId("svc-chat-2")))
	require.NoError(t, svc.WaitIdle(context.Background(), evtstream.SessionId("svc-chat-2")))

	snap, err := svc.Snapshot(context.Background(), evtstream.SessionId("svc-chat-2"))
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
