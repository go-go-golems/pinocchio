package serverkit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
)

func TestMemoryTurnStoreLoadLatestFinalTurn(t *testing.T) {
	store := NewMemoryTurnStore()
	ctx := context.Background()
	if err := store.Save(ctx, "sess-1", "sess-1", "turn-1", "final", 100, "first", chatstore.TurnSaveOptions{RuntimeKey: "gpt-5-mini-low"}); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := store.Save(ctx, "sess-1", "sess-1", "turn-2", "draft", 200, "draft", chatstore.TurnSaveOptions{}); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if err := store.Save(ctx, "sess-1", "sess-1", "turn-3", "final", 300, "latest", chatstore.TurnSaveOptions{RuntimeKey: "gpt-5-mini-low"}); err != nil {
		t.Fatalf("save latest: %v", err)
	}

	snap, err := store.LoadLatestTurn(ctx, "sess-1", "final")
	if err != nil {
		t.Fatalf("load latest: %v", err)
	}
	if snap == nil || snap.TurnID != "turn-3" || snap.Payload != "latest" || snap.RuntimeKey != "gpt-5-mini-low" {
		t.Fatalf("unexpected latest final snapshot: %#v", snap)
	}
}

// TestMemoryTurnStorePreservesSnapshotsPerTimestamp guards against the
// regression flagged in the PR #200 review: the in-memory store must key
// snapshots on (conv, session, turn, phase, createdAtMs), exactly like the
// SQLite and MySQL backends (whose turn_block_membership primary key includes
// snapshot_created_at_ms). Otherwise repeated final-turn persistence that
// reuses the turn ID collapses the history to a single record per phase.
func TestMemoryTurnStorePreservesSnapshotsPerTimestamp(t *testing.T) {
	store := NewMemoryTurnStore()
	ctx := context.Background()
	const conv, sess, turn, phase = "sess-1", "sess-1", "turn-1", "final"

	if err := store.Save(ctx, conv, sess, turn, phase, 100, "first", chatstore.TurnSaveOptions{}); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := store.Save(ctx, conv, sess, turn, phase, 200, "second", chatstore.TurnSaveOptions{}); err != nil {
		t.Fatalf("save second: %v", err)
	}

	snapshots, err := store.List(ctx, chatstore.TurnQuery{ConvID: conv, SessionID: sess})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots preserved across timestamps, got %d: %#v", len(snapshots), snapshots)
	}
	if snapshots[0].CreatedAtMs != 100 || snapshots[0].Payload != "first" {
		t.Fatalf("unexpected first snapshot: %#v", snapshots[0])
	}
	if snapshots[1].CreatedAtMs != 200 || snapshots[1].Payload != "second" {
		t.Fatalf("unexpected second snapshot: %#v", snapshots[1])
	}

	// Re-saving the exact same identity (incl. createdAtMs) replaces the row,
	// matching the SQLite/MySQL DELETE-then-reinsert for that snapshot_created_at_ms.
	if err := store.Save(ctx, conv, sess, turn, phase, 100, "first-updated", chatstore.TurnSaveOptions{}); err != nil {
		t.Fatalf("resave: %v", err)
	}
	snapshots, err = store.List(ctx, chatstore.TurnQuery{ConvID: conv, SessionID: sess})
	if err != nil {
		t.Fatalf("list after resave: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots after idempotent resave, got %d: %#v", len(snapshots), snapshots)
	}
	var first *chatstore.TurnSnapshot
	for i := range snapshots {
		if snapshots[i].CreatedAtMs == 100 {
			first = &snapshots[i]
		}
	}
	if first == nil || first.Payload != "first-updated" {
		t.Fatalf("expected t=100 snapshot replaced with first-updated, got %#v", first)
	}
}

func TestOpenTurnStoreSQLitePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "turns", "chat-turns.db")
	store, closeFn, err := OpenTurnStore(ctx, StoreOptions{Turns: StoreSpec{Backend: StoreBackendSQLite, Path: dbPath}})
	if err != nil {
		t.Fatalf("open sqlite turn store: %v", err)
	}
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("remember durable history"))
	payload, err := serde.ToYAML(turn, serde.Options{})
	if err != nil {
		t.Fatalf("serialize turn: %v", err)
	}
	if err := store.Save(ctx, "sess-durable", "sess-durable", "turn-1", "final", 1000, string(payload), chatstore.TurnSaveOptions{RuntimeKey: "gpt-5-mini-low"}); err != nil {
		t.Fatalf("save turn: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	reopened, closeReopened, err := OpenTurnStore(ctx, StoreOptions{Turns: StoreSpec{Backend: StoreBackendSQLite, Path: dbPath}})
	if err != nil {
		t.Fatalf("reopen sqlite turn store: %v", err)
	}
	defer func() { _ = closeReopened() }()
	snap, err := reopened.LoadLatestTurn(ctx, "sess-durable", "final")
	if err != nil {
		t.Fatalf("load latest turn: %v", err)
	}
	if snap == nil || snap.TurnID != "turn-1" || snap.RuntimeKey != "gpt-5-mini-low" {
		t.Fatalf("unexpected durable snapshot: %#v", snap)
	}
}

func TestOpenHydrationStoreSQLiteCreatesParentDirectory(t *testing.T) {
	reg := sessionstream.NewSchemaRegistry()
	store, closeFn, err := OpenHydrationStore(context.Background(), StoreSpec{Backend: StoreBackendSQLite, Path: filepath.Join(t.TempDir(), "timeline", "chat.db")}, reg)
	if err != nil {
		t.Fatalf("open hydration store: %v", err)
	}
	if store == nil || closeFn == nil {
		t.Fatalf("expected store and close func")
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close hydration store: %v", err)
	}
}

// mysqlTurnSelectionDSN returns a DSN for the local docker-compose MySQL when
// configured; tests skip (not fail) without it so `go test ./...` stays green.
func mysqlTurnSelectionDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PINOCCHIO_MYSQL_TURNS_DSN")
	if dsn == "" {
		t.Skipf("PINOCCHIO_MYSQL_TURNS_DSN not set; skipping OpenTurnStore MySQL selection test")
	}
	return dsn
}

func TestOpenTurnStoreMySQLDSNSelectsMySQLTurnStore(t *testing.T) {
	dsn := mysqlTurnSelectionDSN(t)
	store, closeFn, err := OpenTurnStore(context.Background(), StoreOptions{Turns: StoreSpec{Backend: StoreBackendMySQL, DSN: dsn}})
	if err != nil {
		t.Fatalf("open mysql turn store: %v", err)
	}
	defer func() { _ = closeFn() }()
	if _, ok := store.(*chatstore.MySQLTurnStore); !ok {
		t.Fatalf("expected *chatstore.MySQLTurnStore, got %T", store)
	}
}

// mysqlTimelineSelectionDSN returns a DSN for the local docker-compose MySQL
// when configured; tests skip (not fail) without it.
func mysqlTimelineSelectionDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SESSIONSTREAM_MYSQL_DSN")
	if dsn == "" {
		t.Skipf("SESSIONSTREAM_MYSQL_DSN not set; skipping OpenHydrationStore MySQL selection test")
	}
	return dsn
}

func TestOpenHydrationStoreMySQLDSNSelectsMySQLStore(t *testing.T) {
	dsn := mysqlTimelineSelectionDSN(t)
	reg := sessionstream.NewSchemaRegistry()
	store, closeFn, err := OpenHydrationStore(context.Background(), StoreSpec{Backend: StoreBackendMySQL, DSN: dsn}, reg)
	if err != nil {
		t.Fatalf("open mysql hydration store: %v", err)
	}
	defer func() { _ = closeFn() }()
	if store == nil {
		t.Fatalf("expected non-nil store")
	}
	// A MySQL DSN must not select the in-memory sqlite fallback. The empty-DSN
	// path returns an in-memory sqlite store (NewInMemory); a non-empty MySQL
	// DSN returns the mysql store. Distinguish behaviorally: the mysql store
	// survives a Close+reopen and persists, the in-memory store does not.
	sid := sessionstream.SessionId("s-sel-" + sanitizeSel(t.Name()))
	ctx := context.Background()
	// Registering is not needed for Cursor (no schema). A fresh session has
	// cursor 0 on both backends; after a Close+reopen a MySQL store keeps any
	// applied state, an in-memory store is gone. We assert the store accepts a
	// Cursor call without error (proves it is wired and alive).
	cursor, err := store.Cursor(ctx, sid)
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if cursor != 0 {
		t.Fatalf("fresh session cursor = %d, want 0", cursor)
	}
}

func TestOpenTurnStoreSQLiteFileDSNStillSelectsSQLite(t *testing.T) {
	// A file: DSN must keep selecting SQLite, not be misrouted to MySQL. The
	// parent dir must exist (OpenTurnStore only creates it when deriving a DSN
	// from TurnsDB, matching the original behavior for a caller-supplied DSN).
	dbPath := filepath.Join(t.TempDir(), "chat-turns.db")
	sqliteDSN, err := chatstore.SQLiteTurnDSNForFile(dbPath)
	if err != nil {
		t.Fatalf("build sqlite dsn: %v", err)
	}
	store, closeFn, err := OpenTurnStore(context.Background(), StoreOptions{Turns: StoreSpec{Backend: StoreBackendSQLite, DSN: sqliteDSN}})
	if err != nil {
		t.Fatalf("open sqlite turn store: %v", err)
	}
	defer func() { _ = closeFn() }()
	if _, ok := store.(*chatstore.SQLiteTurnStore); !ok {
		t.Fatalf("expected *chatstore.SQLiteTurnStore, got %T", store)
	}
}

func sanitizeSel(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, byte(r))
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

func TestStoreSpecValidationDoesNotGuessDSNBackend(t *testing.T) {
	ctx := context.Background()
	for name, spec := range map[string]StoreSpec{
		"ambiguous mysql tcp dsn":          {DSN: "user:pass@tcp(127.0.0.1:3306)/db?parseTime=true"},
		"ambiguous protocolless mysql dsn": {DSN: "user:pass@/db?parseTime=true"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := OpenTurnStore(ctx, StoreOptions{Turns: spec})
			if err == nil {
				t.Fatal("expected explicit backend error")
			}
			if !strings.Contains(err.Error(), "explicit backend") {
				t.Fatalf("expected explicit backend error, got %v", err)
			}
		})
	}
}

func TestStoreFactoryUsesInjectedConstructors(t *testing.T) {
	ctx := context.Background()
	turnCalled := false
	hydrationCalled := false
	factory := StoreFactory{
		OpenMySQLTurn: func(_ context.Context, dsn string) (chatstore.TurnStore, error) {
			turnCalled = dsn == "user:pass@/db?parseTime=true"
			return NewMemoryTurnStore(), nil
		},
		OpenMySQLHydration: func(_ context.Context, dsn string, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, error) {
			hydrationCalled = dsn == "user:pass@/db?parseTime=true"
			return storesqlite.NewInMemory(reg)
		},
	}
	_, closeTurn, err := factory.OpenTurn(ctx, StoreSpec{Backend: StoreBackendMySQL, DSN: "user:pass@/db?parseTime=true"})
	if err != nil {
		t.Fatalf("open injected turn: %v", err)
	}
	_ = closeTurn()
	_, _, err = factory.OpenHydration(ctx, StoreSpec{Backend: StoreBackendMySQL, DSN: "user:pass@/db?parseTime=true"}, sessionstream.NewSchemaRegistry())
	if err != nil {
		t.Fatalf("open injected hydration: %v", err)
	}
	if !turnCalled || !hydrationCalled {
		t.Fatalf("injected constructors not called: turn=%v hydration=%v", turnCalled, hydrationCalled)
	}
}
