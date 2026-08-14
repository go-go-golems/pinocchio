package serverkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
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

func TestOpenTurnStoreSQLitePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "turns", "chat-turns.db")
	store, closeFn, err := OpenTurnStore(StoreOptions{TurnsDB: dbPath})
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

	reopened, closeReopened, err := OpenTurnStore(StoreOptions{TurnsDB: dbPath})
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
	store, closeFn, err := OpenHydrationStore("", filepath.Join(t.TempDir(), "timeline", "chat.db"), reg)
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
	store, closeFn, err := OpenTurnStore(StoreOptions{TurnsDSN: dsn})
	if err != nil {
		t.Fatalf("open mysql turn store: %v", err)
	}
	defer func() { _ = closeFn() }()
	if _, ok := store.(*chatstore.MySQLTurnStore); !ok {
		t.Fatalf("expected *chatstore.MySQLTurnStore, got %T", store)
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
	store, closeFn, err := OpenTurnStore(StoreOptions{TurnsDSN: sqliteDSN})
	if err != nil {
		t.Fatalf("open sqlite turn store: %v", err)
	}
	defer func() { _ = closeFn() }()
	if _, ok := store.(*chatstore.SQLiteTurnStore); !ok {
		t.Fatalf("expected *chatstore.SQLiteTurnStore, got %T", store)
	}
}

func TestIsSQLiteFileDSN(t *testing.T) {
	for dsn, want := range map[string]bool{
		"file:./var/turns.db?_journal_mode=WAL":          true,
		"file:/abs/turns.db":                             true,
		"gec:pass@tcp(127.0.0.1:3306)/db?parseTime=true": false,
		"gec:pass@unix(/tmp/mysql.sock)/db":              false,
		"":                                               true, // empty handled by caller; treat as non-mysql
	} {
		if got := isSQLiteFileDSN(dsn); got != want {
			t.Errorf("isSQLiteFileDSN(%q) = %v, want %v", dsn, got, want)
		}
	}
}
