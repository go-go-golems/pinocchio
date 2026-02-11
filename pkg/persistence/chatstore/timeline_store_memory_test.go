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
