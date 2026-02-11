package ui

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func emitPersistEvent(t *testing.T, h func(msg *message.Message) error, ev events.Event) {
	t.Helper()
	b, err := json.Marshal(ev)
	require.NoError(t, err)
	msg := message.NewMessage(uuid.NewString(), b)
	require.NoError(t, h(msg))
}

func TestStepTimelinePersistFunc_AssistantLifecycle(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	h := StepTimelinePersistFunc(store, "conv-1")

	md := events.EventMetadata{ID: uuid.New(), SessionID: "session-1", TurnID: "turn-1"}
	emitPersistEvent(t, h, events.NewStartEvent(md))
	emitPersistEvent(t, h, events.NewPartialCompletionEvent(md, "he", "he"))
	emitPersistEvent(t, h, events.NewFinalEvent(md, "hello"))

	snap, err := store.GetSnapshot(context.Background(), "conv-1", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, md.ID.String(), snap.Entities[0].Id)

	msg := snap.Entities[0].GetMessage()
	require.NotNil(t, msg)
	require.Equal(t, "assistant", msg.Role)
	require.Equal(t, "hello", msg.Content)
	require.False(t, msg.Streaming)
}

func TestStepTimelinePersistFunc_ThinkingLifecycle(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	h := StepTimelinePersistFunc(store, "conv-2")

	md := events.EventMetadata{ID: uuid.New(), SessionID: "session-2", TurnID: "turn-2"}
	emitPersistEvent(t, h, events.NewInfoEvent(md, "thinking-started", nil))
	emitPersistEvent(t, h, events.NewThinkingPartialEvent(md, "r", "reasoning text"))
	emitPersistEvent(t, h, events.NewInfoEvent(md, "thinking-ended", nil))

	snap, err := store.GetSnapshot(context.Background(), "conv-2", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)

	entity := snap.Entities[0]
	require.Equal(t, md.ID.String()+":thinking", entity.Id)
	msg := entity.GetMessage()
	require.NotNil(t, msg)
	require.Equal(t, "thinking", msg.Role)
	require.Equal(t, "reasoning text", msg.Content)
	require.False(t, msg.Streaming)
}

func TestStepTimelinePersistFunc_DoesNotCreateEmptyAssistantOnStartOnly(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	h := StepTimelinePersistFunc(store, "conv-3")

	md := events.EventMetadata{ID: uuid.New(), SessionID: "session-3", TurnID: "turn-3"}
	emitPersistEvent(t, h, events.NewStartEvent(md))

	snap, err := store.GetSnapshot(context.Background(), "conv-3", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 0)
}
