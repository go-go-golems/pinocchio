package chatapp

import (
	"context"
	"testing"
	"time"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestServiceSubmitPromptHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	svc, err := NewService(hub, engine)
	require.NoError(t, err)

	require.NoError(t, svc.SubmitPrompt(context.Background(), sessionstream.SessionId("svc-chat-1"), "Explain ordinals"))
	require.NoError(t, svc.WaitIdle(context.Background(), sessionstream.SessionId("svc-chat-1")))

	snap, err := svc.Snapshot(context.Background(), sessionstream.SessionId("svc-chat-1"))
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)
	var assistant *chatappv1.ChatMessageEntity
	var user *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		payloadMsg := entity.Payload.(*chatappv1.ChatMessageEntity)
		switch payloadMsg.GetRole() {
		case "assistant":
			assistant = payloadMsg
		case "user":
			user = payloadMsg
		}
	}
	require.Equal(t, "Explain ordinals", user.GetContent())
	require.Equal(t, "finished", assistant.GetStatus())
	require.Equal(t, "Answer: Explain ordinals", assistant.GetText())
}

func TestServiceStopPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(10 * time.Millisecond))
	hub := newTestHub(t, engine)
	svc, err := NewService(hub, engine)
	require.NoError(t, err)

	require.NoError(t, svc.SubmitPrompt(context.Background(), sessionstream.SessionId("svc-chat-2"), "Stop me"))
	time.Sleep(12 * time.Millisecond)
	require.NoError(t, svc.Stop(context.Background(), sessionstream.SessionId("svc-chat-2")))
	require.NoError(t, svc.WaitIdle(context.Background(), sessionstream.SessionId("svc-chat-2")))

	snap, err := svc.Snapshot(context.Background(), sessionstream.SessionId("svc-chat-2"))
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)
	var assistant *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		payloadMsg := entity.Payload.(*chatappv1.ChatMessageEntity)
		if payloadMsg.GetRole() == "assistant" {
			assistant = payloadMsg
		}
	}
	require.Equal(t, "stopped", assistant.GetStatus())
	require.Equal(t, false, assistant.GetStreaming())
}
