package chatstore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/stretchr/testify/require"
)

func TestSQLiteTurnStore_BackfillNormalizedFromSnapshots(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	turnDraft := &turns.Turn{
		ID: "turn-1",
		Blocks: []turns.Block{
			{ID: "b1", Kind: turns.BlockKindLLMText, Role: "assistant", Payload: map[string]any{"text": "hello"}},
			{Kind: turns.BlockKindToolCall, Payload: map[string]any{"name": "weather"}},
		},
	}
	turnFinal := &turns.Turn{
		ID: "turn-1",
		Blocks: []turns.Block{
			{ID: "b1", Kind: turns.BlockKindLLMText, Role: "assistant", Payload: map[string]any{"text": "hello again"}},
			{Kind: turns.BlockKindToolCall, Payload: map[string]any{"name": "weather"}},
		},
	}

	require.NoError(t, s.Save(ctx, "conv-1", "sess-1", "turn-1", "draft", 100, mustTurnYAML(t, turnDraft)))
	require.NoError(t, s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 200, mustTurnYAML(t, turnFinal)))

	res, err := s.BackfillNormalizedFromSnapshots(ctx, TurnBackfillOptions{})
	require.NoError(t, err)
	require.Equal(t, 2, res.SnapshotsScanned)
	require.Equal(t, 2, res.SnapshotsBackfilled)
	require.Equal(t, 4, res.BlocksScanned)
	require.Equal(t, 4, res.BlockRowsUpserted)
	require.Equal(t, 2, res.TurnRowsUpserted)
	require.Equal(t, 4, res.MembershipInserted)
	require.Equal(t, 0, res.ParseErrors)

	require.Equal(t, int64(1), queryCount(t, s.db, "SELECT COUNT(1) FROM turns"))
	require.Equal(t, int64(3), queryCount(t, s.db, "SELECT COUNT(1) FROM blocks"))
	require.Equal(t, int64(4), queryCount(t, s.db, "SELECT COUNT(1) FROM turn_block_membership"))
	require.Equal(t, int64(100), queryCount(t, s.db, "SELECT MIN(turn_created_at_ms) FROM turns"))
	require.Equal(t, int64(200), queryCount(t, s.db, "SELECT MAX(updated_at_ms) FROM turns"))
}

func TestSQLiteTurnStore_BackfillDryRun(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	turn := &turns.Turn{
		ID: "turn-1",
		Blocks: []turns.Block{
			{ID: "b1", Kind: turns.BlockKindLLMText, Role: "assistant", Payload: map[string]any{"text": "hello"}},
		},
	}
	require.NoError(t, s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 100, mustTurnYAML(t, turn)))

	res, err := s.BackfillNormalizedFromSnapshots(ctx, TurnBackfillOptions{DryRun: true})
	require.NoError(t, err)
	require.Equal(t, 1, res.SnapshotsScanned)
	require.Equal(t, 1, res.SnapshotsBackfilled)
	require.Equal(t, 1, res.BlocksScanned)
	require.Equal(t, 0, res.TurnRowsUpserted)
	require.Equal(t, 0, res.BlockRowsUpserted)
	require.Equal(t, 0, res.MembershipInserted)

	require.Equal(t, int64(0), queryCount(t, s.db, "SELECT COUNT(1) FROM turns"))
	require.Equal(t, int64(0), queryCount(t, s.db, "SELECT COUNT(1) FROM blocks"))
	require.Equal(t, int64(0), queryCount(t, s.db, "SELECT COUNT(1) FROM turn_block_membership"))
}

func TestSQLiteTurnStore_BackfillParseErrorsContinue(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	turn := &turns.Turn{
		ID: "turn-1",
		Blocks: []turns.Block{
			{ID: "b1", Kind: turns.BlockKindLLMText, Role: "assistant", Payload: map[string]any{"text": "ok"}},
		},
	}
	require.NoError(t, s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 100, mustTurnYAML(t, turn)))
	require.NoError(t, s.Save(ctx, "conv-1", "sess-1", "turn-2", "final", 200, "not: [valid"))

	res, err := s.BackfillNormalizedFromSnapshots(ctx, TurnBackfillOptions{})
	require.NoError(t, err)
	require.Equal(t, 2, res.SnapshotsScanned)
	require.Equal(t, 1, res.SnapshotsBackfilled)
	require.Equal(t, 1, res.ParseErrors)
	require.Equal(t, int64(1), queryCount(t, s.db, "SELECT COUNT(1) FROM turns"))
}

func newBackfillTestStore(t *testing.T) *SQLiteTurnStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)
	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustTurnYAML(t *testing.T, tr *turns.Turn) string {
	t.Helper()
	b, err := serde.ToYAML(tr, serde.Options{})
	require.NoError(t, err)
	return string(b)
}

func queryCount(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.QueryRow(query, args...).Scan(&n))
	return n
}
