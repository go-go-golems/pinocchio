package webchat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/stretchr/testify/require"
)

func TestSQLiteTimelineStore_UpsertAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "timeline.db")
	dsn, err := SQLiteTimelineDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTimelineStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	convID := "c1"

	v, err := s.Upsert(ctx, convID, &timelinepb.TimelineEntityV1{
		Id:          "m1",
		Kind:        "message",
		CreatedAtMs: 100,
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "assistant", Content: "hi", Streaming: true},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), v)

	v, err = s.Upsert(ctx, convID, &timelinepb.TimelineEntityV1{
		Id:   "m1",
		Kind: "message",
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "assistant", Content: "hello", Streaming: false},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), v)

	v, err = s.Upsert(ctx, convID, &timelinepb.TimelineEntityV1{
		Id:          "m2",
		Kind:        "message",
		CreatedAtMs: 200,
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{SchemaVersion: 1, Role: "user", Content: "yo", Streaming: false},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(3), v)

	full, err := s.GetSnapshot(ctx, convID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, uint64(3), full.Version)
	require.Len(t, full.Entities, 2)
	require.Equal(t, "m1", full.Entities[0].Id)
	require.Equal(t, int64(100), full.Entities[0].CreatedAtMs)
	require.Equal(t, "m2", full.Entities[1].Id)
	require.Equal(t, int64(200), full.Entities[1].CreatedAtMs)

	inc, err := s.GetSnapshot(ctx, convID, 1, 100)
	require.NoError(t, err)
	require.Equal(t, uint64(3), inc.Version)
	require.Len(t, inc.Entities, 2) // m1(v2), m2(v3)

	limited, err := s.GetSnapshot(ctx, convID, 1, 1)
	require.NoError(t, err)
	require.Len(t, limited.Entities, 1)

	// sanity: file exists
	_, err = os.Stat(dbPath)
	require.NoError(t, err)
}
