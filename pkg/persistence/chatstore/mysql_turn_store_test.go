package chatstore

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

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
	for _, table := range []string{"turns", "blocks", "turn_block_membership"} {
		var n int64
		require.NoError(t, s.db.QueryRowContext(context.Background(),
			`SELECT COUNT(1) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n))
		require.Equal(t, int64(1), n, "table %s must exist", table)
	}
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

	// Two different turns in different conversations, same block content.
	require.NoError(t, s.Save(ctx, convA, "sess-1", "turn-1", "final", 100, validTurnPayload("turn-1", "same-text"), TurnSaveOptions{}))
	require.NoError(t, s.Save(ctx, convB, "sess-1", "turn-2", "final", 200, validTurnPayload("turn-2", "same-text"), TurnSaveOptions{}))

	// One block row: the same (block_id, content_hash) upserts to one row.
	n := chatRowCount(t, s, "SELECT COUNT(1) FROM blocks WHERE block_id = ? AND content_hash = ?", "turn-1-b1", contentHashForText(t, "same-text"))
	// turn-2 has a different block_id, so this specific row is from turn-1 only.
	require.Equal(t, int64(1), n)
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
	// A malformed payload triggers a parse error before any write; the turn is
	// not updated and the prior row is intact.
	require.Error(t, s.Save(ctx, conv, "sess-1", "turn-1", "final", 100, "not: valid: yaml: [", TurnSaveOptions{}))
	snap, err := s.LoadLatestTurn(ctx, conv, "final")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Contains(t, snap.Payload, "first")
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
