package chatapp

import (
	"context"
	"errors"
	"strings"
	"testing"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDefaultMessageIDGenerator(t *testing.T) {
	id, err := defaultMessageIDGenerator()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(id, "chat-msg-"))
	_, err = uuid.Parse(strings.TrimPrefix(id, "chat-msg-"))
	require.NoError(t, err)
}

func TestDefaultMessageIDsDoNotCollideAcrossEngineRestarts(t *testing.T) {
	firstEngine := NewEngine()
	firstID, err := firstEngine.newMessageID()
	require.NoError(t, err)

	restartedEngine := NewEngine()
	secondID, err := restartedEngine.newMessageID()
	require.NoError(t, err)

	require.NotEqual(t, firstID, secondID)
}

func TestMessageIDGenerationFailurePublishesNoEvents(t *testing.T) {
	engine := NewEngine(WithMessageIDGenerator(func() (string, error) {
		return "", errors.New("identity service unavailable")
	}))
	hub := newTestHub(t, engine)
	sid := sessionstream.SessionId("chat-id-failure")

	err := hub.Submit(context.Background(), sid, CommandStartInference, &chatappv1.StartInferenceCommand{Prompt: "hello"})
	require.ErrorContains(t, err, "generate chat message ID: identity service unavailable")

	snapshot, snapshotErr := hub.Snapshot(context.Background(), sid)
	require.NoError(t, snapshotErr)
	require.Zero(t, snapshot.SnapshotOrdinal)
	require.Empty(t, snapshot.Entities)
}

func TestNewMessageIDValidation(t *testing.T) {
	tests := []struct {
		name      string
		generator MessageIDGenerator
		want      string
		wantError string
	}{
		{name: "valid and trimmed", generator: func() (string, error) { return "  chat-msg-test  ", nil }, want: "chat-msg-test"},
		{name: "nil", generator: nil, wantError: "generator is not configured"},
		{name: "empty", generator: func() (string, error) { return "  ", nil }, wantError: "ID is empty"},
		{name: "reserved delimiter", generator: func() (string, error) { return "chat-msg:text:test", nil }, wantError: "reserved text delimiter"},
		{name: "generator error", generator: func() (string, error) { return "", errors.New("entropy unavailable") }, wantError: "generate chat message ID: entropy unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(WithMessageIDGenerator(tt.generator))
			got, err := engine.newMessageID()
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
