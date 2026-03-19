package chatstore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
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
	err = s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "hello"), TurnSaveOptions{
		RuntimeKey:  "inventory",
		InferenceID: "inf-1",
	})
	require.NoError(t, err)
	err = s.Save(ctx, "conv-1", "sess-1", "turn-2", "draft", 200, validTurnPayload("turn-2", "draft"), TurnSaveOptions{
		RuntimeKey:  "planner",
		InferenceID: "inf-2",
	})
	require.NoError(t, err)
	err = s.Save(ctx, "conv-2", "sess-2", "turn-3", "final", 300, validTurnPayload("turn-3", "other"), TurnSaveOptions{})
	require.NoError(t, err)

	items, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "turn-2", items[0].TurnID)
	require.Equal(t, "turn-1", items[1].TurnID)
	require.Equal(t, "planner", items[0].RuntimeKey)
	require.Equal(t, "inf-2", items[0].InferenceID)
	require.Equal(t, "inventory", items[1].RuntimeKey)
	require.Equal(t, "inf-1", items[1].InferenceID)
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
	err = s.Save(ctx, "", "sess-1", "turn-1", "final", 1, validTurnPayload("turn-1", "hello"), TurnSaveOptions{})
	require.Error(t, err)

	err = s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 1, "not yaml", TurnSaveOptions{})
	require.Error(t, err)

	_, err = s.List(ctx, TurnQuery{})
	require.Error(t, err)
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
	require.True(t, hasColumn(t, s.db, "turns", "runtime_key"))
	require.True(t, hasColumn(t, s.db, "turns", "inference_id"))
	var updatedAt int64
	require.NoError(t, s.db.QueryRow(`SELECT updated_at_ms FROM turns WHERE conv_id = ? AND session_id = ? AND turn_id = ?`, "conv-1", "sess-1", "turn-1").Scan(&updatedAt))
	require.Equal(t, int64(123), updatedAt)
}

func TestSQLiteTurnStore_BackfillRuntimeAndInferenceFromMetadata(t *testing.T) {
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
			updated_at_ms INTEGER NOT NULL,
			PRIMARY KEY (conv_id, session_id, turn_id)
		);
	`)
	require.NoError(t, err)
	metadata := fmt.Sprintf(`{"%s":"planner","%s":"inf-42"}`, turns.KeyTurnMetaRuntime.String(), turns.KeyTurnMetaInferenceID.String())
	_, err = db.Exec(
		`INSERT INTO turns(conv_id, session_id, turn_id, turn_created_at_ms, turn_metadata_json, turn_data_json, updated_at_ms) VALUES(?, ?, ?, ?, ?, '{}', ?)`,
		"conv-1",
		"sess-1",
		"turn-1",
		int64(123),
		metadata,
		int64(123),
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	var runtimeKey, inferenceID string
	require.NoError(t, s.db.QueryRow(`SELECT runtime_key, inference_id FROM turns WHERE conv_id = ? AND session_id = ? AND turn_id = ?`, "conv-1", "sess-1", "turn-1").Scan(&runtimeKey, &inferenceID))
	require.Equal(t, "planner", runtimeKey)
	require.Equal(t, "inf-42", inferenceID)
}

func TestSQLiteTurnStore_BackfillLeavesEmptyWhenMetadataMissing(t *testing.T) {
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
			updated_at_ms INTEGER NOT NULL,
			PRIMARY KEY (conv_id, session_id, turn_id)
		);
	`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO turns(conv_id, session_id, turn_id, turn_created_at_ms, turn_metadata_json, turn_data_json, updated_at_ms) VALUES(?, ?, ?, ?, '{}', '{}', ?)`,
		"conv-1",
		"sess-1",
		"turn-1",
		int64(123),
		int64(123),
	)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	var runtimeKey, inferenceID string
	require.NoError(t, s.db.QueryRow(`SELECT runtime_key, inference_id FROM turns WHERE conv_id = ? AND session_id = ? AND turn_id = ?`, "conv-1", "sess-1", "turn-1").Scan(&runtimeKey, &inferenceID))
	require.Equal(t, "", runtimeKey)
	require.Equal(t, "", inferenceID)
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
	table, err := normalizeSQLiteIntrospectionTable(table)
	require.NoError(t, err)
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var colName string
		require.NoError(t, rows.Scan(&colName))
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
