package ui

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
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

	props := snap.Entities[0].GetProps().AsMap()
	require.Equal(t, "assistant", props["role"])
	require.Equal(t, "hello", props["content"])
	require.Equal(t, false, props["streaming"])
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
	props := entity.GetProps().AsMap()
	require.Equal(t, "thinking", props["role"])
	require.Equal(t, "reasoning text", props["content"])
	require.Equal(t, false, props["streaming"])
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

func TestStepTimelinePersistFunc_PersistsRuntimeAttributionFromMetadataExtra(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	h := StepTimelinePersistFunc(store, "conv-attr")

	md := events.EventMetadata{
		ID:        uuid.New(),
		SessionID: "session-attr",
		TurnID:    "turn-attr",
		Extra: map[string]any{
			"runtime_key":         "mento-haiku-4.5",
			"runtime_fingerprint": "fp-1",
			"profile.slug":        "mento-haiku-4.5",
			"profile.registry":    "mento",
			"profile.version":     uint64(7),
		},
	}
	emitPersistEvent(t, h, events.NewPartialCompletionEvent(md, "hi", "hi"))
	emitPersistEvent(t, h, events.NewFinalEvent(md, "hello"))

	snap, err := store.GetSnapshot(context.Background(), "conv-attr", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)

	props := snap.Entities[0].GetProps().AsMap()
	require.Equal(t, "mento-haiku-4.5", props["runtime_key"])
	require.Equal(t, "fp-1", props["runtime_fingerprint"])
	require.Equal(t, "mento-haiku-4.5", props["profile.slug"])
	require.Equal(t, "mento", props["profile.registry"])
	require.Equal(t, float64(7), props["profile.version"])
}

func TestStepTimelinePersistFunc_PersistsProfileSwitchedInfoEvent(t *testing.T) {
	store := chatstore.NewInMemoryTimelineStore(100)
	h := StepTimelinePersistFunc(store, "conv-switch")

	md := events.EventMetadata{
		ID:        uuid.New(),
		SessionID: "session-switch",
		TurnID:    "turn-switch",
		Extra: map[string]any{
			"runtime_key":  "mento-sonnet-4.6",
			"profile.slug": "mento-sonnet-4.6",
		},
	}
	emitPersistEvent(t, h, events.NewInfoEvent(md, "profile-switched", map[string]any{
		"from": "mento-haiku-4.5",
		"to":   "mento-sonnet-4.6",
	}))

	snap, err := store.GetSnapshot(context.Background(), "conv-switch", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)

	entity := snap.Entities[0]
	require.Equal(t, "profile_switch", entity.Kind)
	props := entity.GetProps().AsMap()
	require.Equal(t, "mento-haiku-4.5", props["from"])
	require.Equal(t, "mento-sonnet-4.6", props["to"])
	require.Equal(t, "mento-sonnet-4.6", props["runtime_key"])
}

type recordingTimelineStore struct {
	mu            sync.Mutex
	upsertCalls   int
	canceledCalls int
}

func (s *recordingTimelineStore) Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV2) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		s.canceledCalls++
		return ctx.Err()
	}
	s.upsertCalls++
	return nil
}

func (s *recordingTimelineStore) GetSnapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV2, error) {
	return &timelinepb.TimelineSnapshotV2{ConvId: convID}, nil
}

func (s *recordingTimelineStore) UpsertConversation(context.Context, chatstore.ConversationRecord) error {
	return nil
}

func (s *recordingTimelineStore) GetConversation(context.Context, string) (chatstore.ConversationRecord, bool, error) {
	return chatstore.ConversationRecord{}, false, nil
}

func (s *recordingTimelineStore) ListConversations(context.Context, int, int64) ([]chatstore.ConversationRecord, error) {
	return nil, nil
}

func (s *recordingTimelineStore) Close() error { return nil }

func TestStepTimelinePersistFunc_UsesDetachedContextAfterMessageContextCancellation(t *testing.T) {
	store := &recordingTimelineStore{}
	h := StepTimelinePersistFunc(store, "conv-4")

	md := events.EventMetadata{ID: uuid.New(), SessionID: "session-4", TurnID: "turn-4"}
	b, err := json.Marshal(events.NewPartialCompletionEvent(md, "he", "he"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	msg := message.NewMessage(uuid.NewString(), b)
	msg.SetContext(ctx)

	require.NoError(t, h(msg))

	store.mu.Lock()
	defer store.mu.Unlock()
	require.Equal(t, 1, store.upsertCalls)
	require.Equal(t, 0, store.canceledCalls)
}
