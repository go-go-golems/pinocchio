package chatstore

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteTurnStore_SaveAndList(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	requireTurnSchemaTables(t, s.db)

	ctx := context.Background()
	err = s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 100, "payload-1")
	require.NoError(t, err)
	err = s.Save(ctx, "conv-1", "sess-1", "turn-2", "draft", 200, "payload-2")
	require.NoError(t, err)
	err = s.Save(ctx, "conv-2", "sess-2", "turn-3", "final", 300, "payload-3")
	require.NoError(t, err)

	items, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "turn-2", items[0].TurnID)
	require.Equal(t, "turn-1", items[1].TurnID)

	bySession, err := s.List(ctx, TurnQuery{SessionID: "sess-2", Limit: 10})
	require.NoError(t, err)
	require.Len(t, bySession, 1)
	require.Equal(t, "turn-3", bySession[0].TurnID)

	byPhase, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Phase: "final", Limit: 10})
	require.NoError(t, err)
	require.Len(t, byPhase, 1)
	require.Equal(t, "turn-1", byPhase[0].TurnID)
	require.Equal(t, int64(3), queryRowCount(t, s.db, "SELECT COUNT(1) FROM turn_snapshots"))

	_, err = os.Stat(dbPath)
	require.NoError(t, err)
}

func TestSQLiteTurnStore_Validation(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	err = s.Save(ctx, "", "sess-1", "turn-1", "final", 1, "payload")
	require.Error(t, err)

	_, err = s.List(ctx, TurnQuery{})
	require.Error(t, err)
}

func TestSQLiteTurnStore_MigrateLegacyTurnsTableToTurnSnapshots(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	_, err = db.Exec(`
		CREATE TABLE turns (
			conv_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			created_at_ms INTEGER NOT NULL,
			payload TEXT NOT NULL,
			PRIMARY KEY (conv_id, run_id, turn_id, phase, created_at_ms)
		);
	`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO turns(conv_id, run_id, turn_id, phase, created_at_ms, payload)
		VALUES('conv-legacy', 'sess-legacy', 'turn-1', 'final', 111, 'payload-legacy')
	`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	requireTurnSchemaTables(t, s.db)
	require.True(t, hasColumn(t, s.db, "turn_snapshots", "session_id"))
	require.False(t, hasColumn(t, s.db, "turn_snapshots", "run_id"))

	items, err := s.List(context.Background(), TurnQuery{ConvID: "conv-legacy", SessionID: "sess-legacy", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "turn-1", items[0].TurnID)
	require.Equal(t, "payload-legacy", items[0].Payload)
}

func requireTurnSchemaTables(t *testing.T, db *sql.DB) {
	t.Helper()
	require.True(t, hasTable(t, db, "turn_snapshots"))
	require.True(t, hasTable(t, db, "turns"))
	require.True(t, hasTable(t, db, "blocks"))
	require.True(t, hasTable(t, db, "turn_block_membership"))
}

func hasTable(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	return queryRowCount(t, db, "SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?", name) > 0
}

func hasColumn(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			cid       int
			name      string
			typeName  string
			notNull   int
			dfltValue any
			pk        int
		)
		require.NoError(t, rows.Scan(&cid, &name, &typeName, &notNull, &dfltValue, &pk))
		if name == column {
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}

func queryRowCount(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.QueryRow(query, args...).Scan(&n))
	return n
}
