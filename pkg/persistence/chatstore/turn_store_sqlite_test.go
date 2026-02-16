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
	err = s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "hello"))
	require.NoError(t, err)
	err = s.Save(ctx, "conv-1", "sess-1", "turn-2", "draft", 200, validTurnPayload("turn-2", "draft"))
	require.NoError(t, err)
	err = s.Save(ctx, "conv-2", "sess-2", "turn-3", "final", 300, validTurnPayload("turn-3", "other"))
	require.NoError(t, err)

	items, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "turn-2", items[0].TurnID)
	require.Equal(t, "turn-1", items[1].TurnID)
	require.Contains(t, items[0].Payload, "blocks")
	require.Contains(t, items[1].Payload, "text: hello")

	bySession, err := s.List(ctx, TurnQuery{SessionID: "sess-2", Limit: 10})
	require.NoError(t, err)
	require.Len(t, bySession, 1)
	require.Equal(t, "turn-3", bySession[0].TurnID)

	byPhase, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Phase: "final", Limit: 10})
	require.NoError(t, err)
	require.Len(t, byPhase, 1)
	require.Equal(t, "turn-1", byPhase[0].TurnID)

	require.Equal(t, int64(3), queryRowCount(t, s.db, "SELECT COUNT(1) FROM turns"))
	require.Equal(t, int64(3), queryRowCount(t, s.db, "SELECT COUNT(1) FROM blocks"))
	require.Equal(t, int64(3), queryRowCount(t, s.db, "SELECT COUNT(1) FROM turn_block_membership"))

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
	err = s.Save(ctx, "", "sess-1", "turn-1", "final", 1, validTurnPayload("turn-1", "hello"))
	require.Error(t, err)

	err = s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 1, "not yaml")
	require.Error(t, err)

	_, err = s.List(ctx, TurnQuery{})
	require.Error(t, err)
}

func TestSQLiteTurnStore_MigrateLegacyTurnsPayloadTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	_, err = db.Exec(`
		CREATE TABLE turns (
			conv_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			created_at_ms INTEGER NOT NULL,
			payload TEXT NOT NULL,
			PRIMARY KEY (conv_id, session_id, turn_id, phase, created_at_ms)
		);
	`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX turns_by_conv ON turns(conv_id, created_at_ms DESC);`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX turns_by_session ON turns(session_id, created_at_ms DESC);`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE INDEX turns_by_phase ON turns(phase, created_at_ms DESC);`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO turns(conv_id, session_id, turn_id, phase, created_at_ms, payload) VALUES(?, ?, ?, ?, ?, ?)`,
		"conv-legacy",
		"sess-legacy",
		"turn-legacy",
		"final",
		int64(111),
		validTurnPayload("turn-legacy", "legacy"),
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	items, err := s.List(context.Background(), TurnQuery{ConvID: "conv-legacy", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "turn-legacy", items[0].TurnID)
	require.Contains(t, items[0].Payload, "text: legacy")
	require.False(t, hasTable(t, s.db, "turn_snapshots_legacy"))
}

func TestSQLiteTurnStore_AddsMissingUpdatedAtColumn(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	_, err = db.Exec(`
		CREATE TABLE turns (
			conv_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			turn_created_at_ms INTEGER NOT NULL,
			turn_metadata_json TEXT NOT NULL DEFAULT '{}',
			turn_data_json TEXT NOT NULL DEFAULT '{}',
			PRIMARY KEY (conv_id, session_id, turn_id)
		);
	`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO turns(conv_id, session_id, turn_id, turn_created_at_ms, turn_metadata_json, turn_data_json) VALUES(?, ?, ?, ?, '{}', '{}')`,
		"conv-1",
		"sess-1",
		"turn-1",
		int64(123),
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.True(t, hasColumn(t, s.db, "turns", "updated_at_ms"))
	var updatedAt int64
	require.NoError(t, s.db.QueryRow(`SELECT updated_at_ms FROM turns WHERE conv_id = ? AND session_id = ? AND turn_id = ?`, "conv-1", "sess-1", "turn-1").Scan(&updatedAt))
	require.Equal(t, int64(123), updatedAt)
}

func requireTurnSchemaTables(t *testing.T, db *sql.DB) {
	t.Helper()
	require.True(t, hasTable(t, db, "turns"))
	require.True(t, hasTable(t, db, "blocks"))
	require.True(t, hasTable(t, db, "turn_block_membership"))
}

func hasTable(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	return queryRowCount(t, db, "SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?", name) > 0
}

func hasColumn(t *testing.T, db *sql.DB, table string, name string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			cid       int
			colName   string
			typeName  string
			notNull   int
			dfltValue any
			pk        int
		)
		require.NoError(t, rows.Scan(&cid, &colName, &typeName, &notNull, &dfltValue, &pk))
		if colName == name {
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

func validTurnPayload(turnID string, text string) string {
	return "id: " + turnID + "\nblocks:\n  - id: " + turnID + "-b1\n    kind: llm_text\n    role: assistant\n    payload:\n      text: " + text + "\n"
}
