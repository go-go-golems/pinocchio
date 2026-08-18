package chatstore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

// mysqlTurnTestDSN returns a DSN for the local docker-compose MySQL when one
// is configured. Tests skip (not fail) when no DSN is set, so `go test ./...`
// stays green without MySQL. Set PINOCCHIO_MYSQL_TURNS_DSN to run.
func mysqlTurnTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PINOCCHIO_MYSQL_TURNS_DSN")
	if dsn == "" {
		t.Skipf("PINOCCHIO_MYSQL_TURNS_DSN not set; skipping MySQL turn store integration test")
	}
	return dsn
}

// newTestMySQLTurnStore opens a store against the shared coinvault_chat_dev
// database. Each test inserts distinct (convID, sessionID, turnID) tuples so
// they do not collide; the membership rows are scoped by snapshot timestamp.
func mysqlTurnTestAdminDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PINOCCHIO_MYSQL_TEST_ADMIN_DSN")
	if dsn == "" {
		if os.Getenv("CI") == "true" || os.Getenv("PINOCCHIO_MYSQL_TESTCONTAINERS") == "1" {
			t.Fatal("PINOCCHIO_MYSQL_TEST_ADMIN_DSN is required for rollback integration coverage")
		}
		t.Skip("PINOCCHIO_MYSQL_TEST_ADMIN_DSN not set; skipping admin-only rollback integration test")
	}
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err, "open MySQL test admin connection")
	require.NoError(t, db.PingContext(context.Background()), "ping MySQL test admin connection")
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestMySQLTurnStore(t *testing.T) *MySQLTurnStore {
	t.Helper()
	s, err := NewMySQLTurnStore(context.Background(), mysqlTurnTestDSN(t))
	require.NoError(t, err, "NewMySQLTurnStore")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// chatRowCount is the MySQL analog of queryRowCount for the shared chat DB.
func chatRowCount(t *testing.T, s *MySQLTurnStore, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.QueryRowContext(context.Background(), query, args...).Scan(&n))
	return n
}

// mysqlTurnTablesExist asserts the three tables were created (they are shared
// across tests, so this also confirms migrate is idempotent on re-open).
func mysqlTurnTablesExist(t *testing.T, s *MySQLTurnStore) {
	t.Helper()
	for _, table := range []string{"pinocchio_schema_version", "turns", "blocks", "turn_block_membership"} {
		var n int64
		require.NoError(t, s.db.QueryRowContext(context.Background(),
			`SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n))
		require.Equal(t, int64(1), n, "table %s must exist", table)
	}
	var version int64
	require.NoError(t, s.db.QueryRowContext(context.Background(), `
		SELECT schema_version FROM pinocchio_schema_version WHERE component = ?
	`, mysqlTurnSchemaComponent).Scan(&version))
	require.Equal(t, mysqlTurnSchemaVersion, version)
}

func TestMySQLTurnStore_SaveAndList(t *testing.T) {

	s := newTestMySQLTurnStore(t)
	mysqlTurnTablesExist(t, s)
	// Migrate is idempotent: re-open does not error and tables persist.
	s2 := newTestMySQLTurnStore(t)
	mysqlTurnTablesExist(t, s2)

	ctx := context.Background()
	conv := fmt.Sprintf("conv-mysql-%s", t.Name())
	conv = sanitizeTurnID(conv)

	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "hello"), TurnSaveOptions{
		RuntimeKey: "inventory", InferenceID: "inf-1",
	}))
	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-2", "draft", 200, validTurnPayload("turn-2", "draft"), TurnSaveOptions{
		RuntimeKey: "planner", InferenceID: "inf-2",
	}))
	require.NoError(t, s.Save(ctx, conv+"-other", "sess-2", "turn-3", "final", 300, validTurnPayload("turn-3", "other"), TurnSaveOptions{}))

	items, err := s.List(ctx, TurnQuery{ConvID: conv, Limit: 10})
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
	require.Equal(t, "turn-3", bySession[0].TurnID)

	byPhase, err := s.List(ctx, TurnQuery{ConvID: conv, Phase: "final", Limit: 10})
	require.NoError(t, err)
	require.Len(t, byPhase, 1)
	require.Equal(t, "turn-1", byPhase[0].TurnID)

	// Three turns, three blocks, three membership rows for this test's data.
	require.Equal(t, int64(3), chatRowCount(t, s, "SELECT COUNT(1) FROM turns WHERE conv_id = ? OR conv_id = ?", conv, conv+"-other"))
	require.Equal(t, int64(3), chatRowCount(t, s, "SELECT COUNT(1) FROM turn_block_membership WHERE conv_id = ? OR conv_id = ?", conv, conv+"-other"))
}

func TestMySQLTurnStore_LoadLatestTurn(t *testing.T) {

	s := newTestMySQLTurnStore(t)
	ctx := context.Background()
	conv := sanitizeTurnID("conv-mysql-latest-" + t.Name())

	// Empty store returns nil
	snap, err := s.LoadLatestTurn(ctx, conv, "final")
	require.NoError(t, err)
	require.Nil(t, snap)

	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "hello"), TurnSaveOptions{}))
	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-2", "final", 200, validTurnPayload("turn-2", "world"), TurnSaveOptions{}))
	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-3", "draft", 300, validTurnPayload("turn-3", "draft"), TurnSaveOptions{}))

	snap, err = s.LoadLatestTurn(ctx, conv, "final")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Equal(t, "turn-2", snap.TurnID)
	require.Equal(t, "final", snap.Phase)
	require.Contains(t, snap.Payload, "world")

	snap, err = s.LoadLatestTurn(ctx, conv, "draft")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Equal(t, "turn-3", snap.TurnID)

	snap, err = s.LoadLatestTurn(ctx, conv, "nonexistent")
	require.NoError(t, err)
	require.Nil(t, snap)

	// No phase filter returns the overall latest.
	snap, err = s.LoadLatestTurn(ctx, conv, "")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Equal(t, "turn-3", snap.TurnID)
}

func TestMySQLTurnStore_Validation(t *testing.T) {

	s := newTestMySQLTurnStore(t)
	ctx := context.Background()
	require.Error(t, s.Save(ctx, "", "sess-1", "turn-1", "final", 1, validTurnPayload("turn-1", "x"), TurnSaveOptions{}))
	require.Error(t, s.Save(ctx, "conv-bad", "sess-1", "turn-1", "final", 1, "not yaml", TurnSaveOptions{}))
	_, err := s.List(ctx, TurnQuery{})
	require.Error(t, err)
}

func TestMySQLTurnStore_ExactOpaqueIdentity(t *testing.T) {
	s := newTestMySQLTurnStore(t)
	ctx := context.Background()
	base := "exact-" + sanitizeTurnID(t.Name())
	identities := []string{base + "-Case", base + "-case", base + "-café", base + "-café", base + "-id", base + "-id "}
	for i, conv := range identities {
		turnID := fmt.Sprintf("exact-turn-%d", i)
		require.NoError(t, s.Save(ctx, conv, "session", turnID, "final", int64(100+i), validTurnPayload(turnID, conv), TurnSaveOptions{}))
	}
	for i, conv := range identities {
		items, err := s.List(ctx, TurnQuery{ConvID: conv, Limit: 10})
		require.NoError(t, err)
		require.Len(t, items, 1, "identity %q must select only its own row", conv)
		require.Equal(t, fmt.Sprintf("exact-turn-%d", i), items[0].TurnID)
	}
}

func TestMySQLTurnStore_RejectsOpaqueValuesOverByteLimit(t *testing.T) {
	s := newTestMySQLTurnStore(t)
	tooLong := strings.Repeat("x", 256)
	err := s.Save(context.Background(), tooLong, "session", "turn", "final", 1, validTurnPayload("turn", "value"), TurnSaveOptions{})
	require.ErrorContains(t, err, "convID exceeds 255-byte limit")
}

func TestMySQLTurnStore_ReSaveReplacesMembershipRowset(t *testing.T) {

	s := newTestMySQLTurnStore(t)
	ctx := context.Background()
	conv := sanitizeTurnID("conv-mysql-resave-" + t.Name())

	// First save: two blocks.
	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayloadTwoBlocks("turn-1"), TurnSaveOptions{}))
	require.Equal(t, int64(2), chatRowCount(t, s, "SELECT COUNT(1) FROM turn_block_membership WHERE conv_id = ? AND session_id = ? AND turn_id = ?", conv, "sess-1", "turn-1"))

	// Re-save the same (turn, phase, snapshot) with one block: the membership
	// rowset is replaced (DELETE then INSERT), so the count drops to 1 and the
	// first block's content hash is gone.
	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "only"), TurnSaveOptions{}))
	require.Equal(t, int64(1), chatRowCount(t, s, "SELECT COUNT(1) FROM turn_block_membership WHERE conv_id = ? AND session_id = ? AND turn_id = ?", conv, "sess-1", "turn-1"))

	items, err := s.List(ctx, TurnQuery{ConvID: conv, Phase: "final", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Contains(t, items[0].Payload, "text: only")
}

func TestMySQLTurnStore_BlockDedupByContentHash(t *testing.T) {

	s := newTestMySQLTurnStore(t)
	ctx := context.Background()
	convA := sanitizeTurnID("conv-dedup-a-" + t.Name())
	convB := sanitizeTurnID("conv-dedup-b-" + t.Name())

	// The same block ID and canonical content in two snapshots must reuse one
	// blocks row while retaining two membership references.
	payload := validTurnPayload("shared-turn", "same-text")
	require.NoError(t, s.Save(ctx, convA, "sess-1", "shared-turn", "final", 100, payload, TurnSaveOptions{}))
	require.NoError(t, s.Save(ctx, convB, "sess-1", "shared-turn", "final", 200, payload, TurnSaveOptions{}))

	n := chatRowCount(t, s, "SELECT COUNT(1) FROM blocks WHERE block_id = ? AND content_hash = ?", "shared-turn-b1", contentHashForText(t, "same-text"))
	require.Equal(t, int64(1), n)
	require.Equal(t, int64(2), chatRowCount(t, s, "SELECT COUNT(1) FROM turn_block_membership WHERE turn_id = ? AND (conv_id = ? OR conv_id = ?)", "shared-turn", convA, convB))
}

func TestMySQLTurnStore_SurvivesRestart(t *testing.T) {
	// Turns persist across Close+reopen (the durability test).

	dsn := mysqlTurnTestDSN(t)
	first, err := NewMySQLTurnStore(context.Background(), dsn)
	require.NoError(t, err)
	ctx := context.Background()
	conv := sanitizeTurnID("conv-mysql-restart-" + t.Name())
	require.NoError(t, first.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "durable"), TurnSaveOptions{RuntimeKey: "rt"}))
	require.NoError(t, first.Close())

	second, err := NewMySQLTurnStore(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = second.Close() })
	snap, err := second.LoadLatestTurn(ctx, conv, "final")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Equal(t, "turn-1", snap.TurnID)
	require.Equal(t, "rt", snap.RuntimeKey)
	require.Contains(t, snap.Payload, "durable")
}

func TestMySQLTurnStore_SaveRollsBackOnError(t *testing.T) {

	s := newTestMySQLTurnStore(t)
	ctx := context.Background()
	conv := sanitizeTurnID("conv-mysql-rollback-" + t.Name())
	require.NoError(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "first"), TurnSaveOptions{}))

	adminDB := mysqlTurnTestAdminDB(t)
	trigger := fmt.Sprintf("pinocchio_rb_%d", turnUniqueSeq.Add(1))
	sentinel := fmt.Sprintf("rollback-sentinel-%d", turnUniqueSeq.Add(1))
	quotedTrigger := "`" + trigger + "`"
	_, err := adminDB.ExecContext(ctx, fmt.Sprintf(`
		CREATE TRIGGER %s BEFORE INSERT ON blocks
		FOR EACH ROW
		BEGIN
			IF NEW.block_id = '%s' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'forced rollback';
			END IF;
		END
	`, quotedTrigger, sentinel))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = adminDB.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS "+quotedTrigger) })

	// The trigger fires after the turn upsert and membership delete have begun,
	// proving that the previous committed rowset survives a real transaction
	// rollback rather than only a pre-transaction YAML validation error.
	require.Error(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, validTurnPayloadWithBlockID("turn-1", sentinel, "replacement"), TurnSaveOptions{}))
	_, err = adminDB.ExecContext(ctx, "DROP TRIGGER IF EXISTS "+quotedTrigger)
	require.NoError(t, err)
	snap, err := s.LoadLatestTurn(ctx, conv, "final")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Contains(t, snap.Payload, "first")
	require.NotContains(t, snap.Payload, "replacement")
}

// sanitizeTurnID turns a test name into a conv-safe string with a unique
// per-invocation suffix so re-running against a populated shared database never
// collides with this test's assertions.
func sanitizeTurnID(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, byte(r))
		default:
			out = append(out, '_')
		}
	}
	return string(out) + "-" + strconv.FormatUint(turnUniqueSeq.Add(1), 36)
}

var turnUniqueSeq atomic.Uint64

// validTurnPayloadTwoBlocks builds a payload with two blocks for the
// membership-replacement test.
func validTurnPayloadWithBlockID(turnID, blockID, text string) string {
	return fmt.Sprintf("id: %s\\nblocks:\\n  - id: %s\\n    kind: llm_text\\n    role: assistant\\n    payload:\\n      text: %s\\n", turnID, blockID, text)
}

func validTurnPayloadTwoBlocks(turnID string) string {
	return "id: " + turnID + "\nblocks:\n  - id: " + turnID + "-b1\n    kind: llm_text\n    role: assistant\n    payload:\n      text: one\n  - id: " + turnID + "-b2\n    kind: llm_text\n    role: assistant\n    payload:\n      text: two\n"
}

// contentHashForText computes the canonical block content hash for a single
// llm_text/assistant text block, matching ComputeBlockContentHash.
func contentHashForText(t *testing.T, text string) string {
	t.Helper()
	h, err := ComputeBlockContentHash("llm_text", "assistant", map[string]any{"text": text}, map[string]any{})
	require.NoError(t, err)
	return h
}
