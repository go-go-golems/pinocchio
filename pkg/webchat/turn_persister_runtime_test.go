package webchat

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/stretchr/testify/require"
)

func TestTurnStorePersister_RuntimeSwitchPersistsPerTurnRuntime(t *testing.T) {
	dir := t.TempDir()
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(dir, "turns.db"))
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	conv := &Conversation{
		ID:         "conv-runtime-switch",
		SessionID:  "session-runtime-switch",
		RuntimeKey: "inventory@v1",
	}

	turn1 := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn1, turns.NewUserTextBlock("inventory turn"))
	require.NoError(t, turns.KeyTurnMetaInferenceID.Set(&turn1.Metadata, "inf-1"))
	require.NoError(t, newTurnStorePersister(store, conv, "final").PersistTurn(context.Background(), turn1))

	conv.RuntimeKey = "planner@v1"
	turn2 := &turns.Turn{ID: "turn-2"}
	turns.AppendBlock(turn2, turns.NewUserTextBlock("planner turn"))
	require.NoError(t, turns.KeyTurnMetaInferenceID.Set(&turn2.Metadata, "inf-2"))
	require.NoError(t, newTurnStorePersister(store, conv, "final").PersistTurn(context.Background(), turn2))

	items, err := store.List(context.Background(), chatstore.TurnQuery{ConvID: conv.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	itemsByTurnID := map[string]chatstore.TurnSnapshot{}
	for _, item := range items {
		itemsByTurnID[item.TurnID] = item
	}
	require.Equal(t, "inventory@v1", itemsByTurnID["turn-1"].RuntimeKey)
	require.Equal(t, "inf-1", itemsByTurnID["turn-1"].InferenceID)
	require.Equal(t, "planner@v1", itemsByTurnID["turn-2"].RuntimeKey)
	require.Equal(t, "inf-2", itemsByTurnID["turn-2"].InferenceID)
}

func TestSnapshotHookForConv_RuntimeSwitchPersistsPerTurnRuntime(t *testing.T) {
	dir := t.TempDir()
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(dir, "turns.db"))
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	conv := &Conversation{
		ID:         "conv-snapshot-runtime-switch",
		SessionID:  "session-snapshot-runtime-switch",
		RuntimeKey: "inventory@v1",
	}
	hook := snapshotHookForConv(conv, store)
	require.NotNil(t, hook)

	turn1 := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn1, turns.NewUserTextBlock("inventory snapshot turn"))
	require.NoError(t, turns.KeyTurnMetaInferenceID.Set(&turn1.Metadata, "inf-1"))
	hook(context.Background(), turn1, "final")

	conv.mu.Lock()
	conv.RuntimeKey = "planner@v1"
	conv.mu.Unlock()

	turn2 := &turns.Turn{ID: "turn-2"}
	turns.AppendBlock(turn2, turns.NewUserTextBlock("planner snapshot turn"))
	require.NoError(t, turns.KeyTurnMetaInferenceID.Set(&turn2.Metadata, "inf-2"))
	hook(context.Background(), turn2, "final")

	items, err := store.List(context.Background(), chatstore.TurnQuery{ConvID: conv.ID, Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	itemsByTurnID := map[string]chatstore.TurnSnapshot{}
	for _, item := range items {
		itemsByTurnID[item.TurnID] = item
	}
	require.Equal(t, "inventory@v1", itemsByTurnID["turn-1"].RuntimeKey)
	require.Equal(t, "inf-1", itemsByTurnID["turn-1"].InferenceID)
	require.Equal(t, "planner@v1", itemsByTurnID["turn-2"].RuntimeKey)
	require.Equal(t, "inf-2", itemsByTurnID["turn-2"].InferenceID)
}
