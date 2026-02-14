package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/stretchr/testify/require"
)

func TestOpenChatPersistenceStores_NoneConfigured(t *testing.T) {
	timelineStore, turnStore, cleanup, err := openChatPersistenceStores(run.PersistenceSettings{})
	require.NoError(t, err)
	require.Nil(t, timelineStore)
	require.Nil(t, turnStore)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestOpenChatPersistenceStores_OpenBothFromDBPaths(t *testing.T) {
	dir := t.TempDir()
	timelineDB := filepath.Join(dir, "timeline", "timeline.db")
	turnsDB := filepath.Join(dir, "turns", "turns.db")

	timelineStore, turnStore, cleanup, err := openChatPersistenceStores(run.PersistenceSettings{
		TimelineDB: timelineDB,
		TurnsDB:    turnsDB,
	})
	require.NoError(t, err)
	require.NotNil(t, timelineStore)
	require.NotNil(t, turnStore)
	t.Cleanup(cleanup)

	_, err = os.Stat(filepath.Dir(timelineDB))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Dir(turnsDB))
	require.NoError(t, err)
}

func TestCLITurnStorePersister_PersistTurn(t *testing.T) {
	dir := t.TempDir()
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(dir, "turns.db"))
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	persister := newCLITurnStorePersister(store, "conv-a", "", "final")
	require.NotNil(t, persister)

	turn := &turns.Turn{ID: "turn-a"}
	err = turns.KeyTurnMetaSessionID.Set(&turn.Metadata, "session-a")
	require.NoError(t, err)
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello"))

	err = persister.PersistTurn(context.Background(), turn)
	require.NoError(t, err)

	items, err := store.List(context.Background(), chatstore.TurnQuery{ConvID: "conv-a", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "session-a", items[0].SessionID)
	require.Equal(t, "turn-a", items[0].TurnID)
	require.Equal(t, "final", items[0].Phase)
}
