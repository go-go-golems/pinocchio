package chatstore

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

	err = s.Upsert(ctx, convID, 0, &timelinepb.TimelineEntityV2{
		Id:   "bad",
		Kind: "message",
	})
	require.Error(t, err)

	err = s.Upsert(ctx, convID, 10, &timelinepb.TimelineEntityV2{
		Id:          "m1",
		Kind:        "message",
		CreatedAtMs: 200,
	})
	require.NoError(t, err)

	err = s.Upsert(ctx, convID, 20, &timelinepb.TimelineEntityV2{
		Id:   "m1",
		Kind: "message",
	})
	require.NoError(t, err)

	err = s.Upsert(ctx, convID, 30, &timelinepb.TimelineEntityV2{
		Id:          "m2",
		Kind:        "message",
		CreatedAtMs: 50,
	})
	require.NoError(t, err)

	full, err := s.GetSnapshot(ctx, convID, 0, 100)
	require.NoError(t, err)
	require.Equal(t, uint64(30), full.Version)
	require.Len(t, full.Entities, 2)
	require.Equal(t, "m1", full.Entities[0].Id)
	require.Equal(t, int64(200), full.Entities[0].CreatedAtMs)
	require.Equal(t, "m2", full.Entities[1].Id)
	require.Equal(t, int64(50), full.Entities[1].CreatedAtMs)

	inc, err := s.GetSnapshot(ctx, convID, 1, 100)
	require.NoError(t, err)
	require.Equal(t, uint64(30), inc.Version)
	require.Len(t, inc.Entities, 2) // m1(v20), m2(v30)

	limited, err := s.GetSnapshot(ctx, convID, 1, 1)
	require.NoError(t, err)
	require.Len(t, limited.Entities, 1)

	// sanity: file exists
	_, err = os.Stat(dbPath)
	require.NoError(t, err)
}

func TestSQLiteTimelineStore_ConversationIndex(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "timeline-conversations.db")
	dsn, err := SQLiteTimelineDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTimelineStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	err = s.UpsertConversation(ctx, ConversationRecord{
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

	// partial update should preserve stable metadata while updating max progress.
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
		Status:          "active",
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

func TestSQLiteTimelineStore_UpsertAdvancesConversationProgress(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "timeline-upsert-conversation-progress.db")
	dsn, err := SQLiteTimelineDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTimelineStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	convID := "conv-progress-1"

	err = s.Upsert(ctx, convID, 7, &timelinepb.TimelineEntityV2{
		Id:   "m1",
		Kind: "message",
	})
	require.NoError(t, err)

	err = s.Upsert(ctx, convID, 15, &timelinepb.TimelineEntityV2{
		Id:   "m1",
		Kind: "message",
	})
	require.NoError(t, err)

	rec, ok, err := s.GetConversation(ctx, convID)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, rec.HasTimeline)
	require.Equal(t, uint64(15), rec.LastSeenVersion)
	require.NotZero(t, rec.LastActivityMs)
}
