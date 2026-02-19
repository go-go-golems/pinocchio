package chatstore

import (
	"context"
	"testing"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/stretchr/testify/require"
)

func TestInMemoryTimelineStore_UpsertAndSnapshot(t *testing.T) {
	s := NewInMemoryTimelineStore(2)
	ctx := context.Background()
	convID := "c1"

	err := s.Upsert(ctx, convID, 0, &timelinepb.TimelineEntityV1{
		Id:   "bad",
		Kind: "message",
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "assistant", Content: "bad", Streaming: true},
		},
	})
	require.Error(t, err)

	err = s.Upsert(ctx, convID, 10, &timelinepb.TimelineEntityV1{
		Id:          "m1",
		Kind:        "message",
		CreatedAtMs: 200,
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "assistant", Content: "hi", Streaming: true},
		},
	})
	require.NoError(t, err)

	err = s.Upsert(ctx, convID, 20, &timelinepb.TimelineEntityV1{
		Id:   "m1",
		Kind: "message",
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "assistant", Content: "hello", Streaming: false},
		},
	})
	require.NoError(t, err)

	err = s.Upsert(ctx, convID, 30, &timelinepb.TimelineEntityV1{
		Id:          "m2",
		Kind:        "message",
		CreatedAtMs: 50,
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "user", Content: "yo", Streaming: false},
		},
	})
	require.NoError(t, err)

	full, err := s.GetSnapshot(ctx, convID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, uint64(30), full.Version)
	require.Len(t, full.Entities, 2)
	require.Equal(t, "m1", full.Entities[0].Id)
	require.Equal(t, "m2", full.Entities[1].Id)

	// Evict oldest (m1) when exceeding limit
	err = s.Upsert(ctx, convID, 40, &timelinepb.TimelineEntityV1{
		Id:   "m3",
		Kind: "message",
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "assistant", Content: "later", Streaming: false},
		},
	})
	require.NoError(t, err)

	after, err := s.GetSnapshot(ctx, convID, 0, 100)
	require.NoError(t, err)
	require.Len(t, after.Entities, 2)
	require.Equal(t, "m2", after.Entities[0].Id)
	require.Equal(t, "m3", after.Entities[1].Id)
}

func TestInMemoryTimelineStore_ConversationIndex(t *testing.T) {
	s := NewInMemoryTimelineStore(100)
	ctx := context.Background()

	err := s.UpsertConversation(ctx, ConversationRecord{
		ConvID:          "conv-1",
		SessionID:       "sess-1",
		RuntimeKey:      "default",
		CreatedAtMs:     100,
		LastActivityMs:  1000,
		LastSeenVersion: 2,
		HasTimeline:     true,
		Status:          "active",
	})
	require.NoError(t, err)

	// partial update should preserve session/runtime, keep max activity/version.
	err = s.UpsertConversation(ctx, ConversationRecord{
		ConvID:          "conv-1",
		LastActivityMs:  2000,
		LastSeenVersion: 5,
	})
	require.NoError(t, err)

	err = s.UpsertConversation(ctx, ConversationRecord{
		ConvID:          "conv-2",
		SessionID:       "sess-2",
		RuntimeKey:      "agent",
		LastActivityMs:  1500,
		LastSeenVersion: 1,
		HasTimeline:     true,
	})
	require.NoError(t, err)

	rec, ok, err := s.GetConversation(ctx, "conv-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "sess-1", rec.SessionID)
	require.Equal(t, "default", rec.RuntimeKey)
	require.Equal(t, int64(100), rec.CreatedAtMs)
	require.Equal(t, int64(2000), rec.LastActivityMs)
	require.Equal(t, uint64(5), rec.LastSeenVersion)
	require.True(t, rec.HasTimeline)

	list, err := s.ListConversations(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "conv-1", list[0].ConvID)
	require.Equal(t, "conv-2", list[1].ConvID)

	filtered, err := s.ListConversations(ctx, 10, 1800)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "conv-1", filtered[0].ConvID)
}
