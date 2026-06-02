package serverkit

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
)

func TestTurnStorePersisterPersistsFinalTurnYAML(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryTurnStore()
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello"))
	p := NewTurnStorePersister(store, "sess-1", "gpt-test", "final")
	if err := p.PersistTurn(ctx, turn); err != nil {
		t.Fatalf("PersistTurn: %v", err)
	}
	snap, err := store.LoadLatestTurn(ctx, "sess-1", "final")
	if err != nil {
		t.Fatalf("LoadLatestTurn: %v", err)
	}
	if snap == nil || snap.TurnID != "turn-1" || snap.RuntimeKey != "gpt-test" {
		t.Fatalf("unexpected snapshot: %#v", snap)
	}
	if !strings.Contains(snap.Payload, "hello") {
		t.Fatalf("payload does not contain turn text: %s", snap.Payload)
	}
}

func TestTurnSnapshotHookPersistsIntermediatePhase(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryTurnStore()
	hook := NewTurnSnapshotHook("sess-1", "gpt-test", store)
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("inspect me"))
	hook(ctx, turn, "post_tools")
	snap, err := store.LoadLatestTurn(ctx, "sess-1", "post_tools")
	if err != nil {
		t.Fatalf("LoadLatestTurn: %v", err)
	}
	if snap == nil || snap.Phase != "post_tools" || snap.RuntimeKey != "gpt-test" {
		t.Fatalf("unexpected snapshot: %#v", snap)
	}
}
