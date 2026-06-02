package cmds

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	inferenceengine "github.com/go-go-golems/geppetto/pkg/inference/engine"
	gp "github.com/go-go-golems/geppetto/pkg/js/modules/geppetto"
	"github.com/go-go-golems/geppetto/pkg/turns"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

type jsStorageEchoEngine struct{}

var _ inferenceengine.Engine = (*jsStorageEchoEngine)(nil)

func (e *jsStorageEchoEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	out := &turns.Turn{}
	if t != nil {
		out = t.Clone()
	}
	users := []string{}
	for _, block := range out.Blocks {
		if block.Role == turns.RoleUser {
			if text, ok := block.Payload[turns.PayloadKeyText].(string); ok {
				users = append(users, text)
			}
		}
	}
	turns.AppendBlock(out, turns.NewAssistantTextBlock("users="+strings.Join(users, ",")))
	return out, nil
}

func TestPinocchioJSTurnStorePersistsAndReadsGeppettoTurns(t *testing.T) {
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(t.TempDir(), "turns.db"))
	if err != nil {
		t.Fatalf("SQLiteTurnDSNForFile failed: %v", err)
	}
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	if err != nil {
		t.Fatalf("NewSQLiteTurnStore failed: %v", err)
	}
	wrapped := newPinocchioJSTurnStore(store)
	t.Cleanup(func() { _ = wrapped.Close() })

	turn := &turns.Turn{ID: "turn-a"}
	if err := turns.KeyTurnMetaSessionID.Set(&turn.Metadata, "session-a"); err != nil {
		t.Fatalf("set session id: %v", err)
	}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello"))
	turns.AppendBlock(turn, turns.NewAssistantTextBlock("world"))

	if err := wrapped.PersistTurn(context.Background(), turn); err != nil {
		t.Fatalf("PersistTurn failed: %v", err)
	}
	latest, err := wrapped.LoadLatestTurn(context.Background(), geppettoTurnStoreQuery("session-a"))
	if err != nil {
		t.Fatalf("LoadLatestTurn failed: %v", err)
	}
	if latest == nil || latest.Turn == nil {
		t.Fatalf("expected latest turn, got %#v", latest)
	}
	if latest.ConvID != "session-a" || latest.SessionID != "session-a" || latest.TurnID != "turn-a" || latest.Phase != "final" {
		t.Fatalf("unexpected latest metadata: %#v", latest)
	}
	if latest.Turn.ID != "turn-a" || len(latest.Turn.Blocks) != 2 {
		t.Fatalf("unexpected latest turn: %#v", latest.Turn)
	}
}

func TestPinocchioJSRuntimeInstallsDefaultTurnStore(t *testing.T) {
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(t.TempDir(), "turns.db"))
	if err != nil {
		t.Fatalf("SQLiteTurnDSNForFile failed: %v", err)
	}
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	if err != nil {
		t.Fatalf("NewSQLiteTurnStore failed: %v", err)
	}
	wrapped := newPinocchioJSTurnStore(store)

	rt, err := newPinocchioJSRuntime(context.Background(), pinocchioJSRuntimeOptions{
		ScriptDir: ".",
		TurnStore: wrapped,
	})
	if err != nil {
		t.Fatalf("newPinocchioJSRuntime failed: %v", err)
	}
	defer func() { _ = rt.Close(context.Background()) }()

	if err := rt.VM.Set("fakeEngine", &jsStorageEchoEngine{}); err != nil {
		t.Fatalf("set fakeEngine: %v", err)
	}
	value, err := rt.VM.RunString(`
		const gp = require("geppetto");
		const store = gp.turnStores.default();
		const agent = gp.agent().engine(globalThis.fakeEngine).build();
		const session = agent.session().id("js-store-session").defaultStore().build();
		const result = session.next().user("persist me").run();
		const latest = store.loadLatest({ sessionId: "js-store-session", phase: "final" });
		JSON.stringify({
			text: result.text(),
			latestSession: latest.sessionId,
			latestText: latest.turn.toJSON().blocks[latest.turn.toJSON().blocks.length - 1].payload.text,
			listed: store.list({ sessionId: "js-store-session" }).length,
		});
	`)
	if err != nil {
		t.Fatalf("run JS failed: %v", err)
	}
	want := `{"text":"users=persist me","latestSession":"js-store-session","latestText":"users=persist me","listed":1}`
	if got := value.String(); got != want {
		t.Fatalf("JS storage result = %s, want %s", got, want)
	}
}

func geppettoTurnStoreQuery(sessionID string) gp.TurnStoreQuery {
	return gp.TurnStoreQuery{SessionID: sessionID, Phase: "final"}
}
