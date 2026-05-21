package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestOpenCLISessionstreamHydrationStore_NoneConfigured(t *testing.T) {
	store, cleanup, err := openCLISessionstreamHydrationStore(run.PersistenceSettings{}, sessionstream.NewSchemaRegistry())
	require.NoError(t, err)
	require.Nil(t, store)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestOpenCLISessionstreamHydrationStore_OpenFromDBPath(t *testing.T) {
	dir := t.TempDir()
	timelineDB := filepath.Join(dir, "timeline", "timeline.db")

	store, cleanup, err := openCLISessionstreamHydrationStore(run.PersistenceSettings{TimelineDB: timelineDB}, sessionstream.NewSchemaRegistry())
	require.NoError(t, err)
	require.NotNil(t, store)
	t.Cleanup(cleanup)

	_, err = os.Stat(filepath.Dir(timelineDB))
	require.NoError(t, err)
}

func TestLoadLatestCLIFinalTurn(t *testing.T) {
	dir := t.TempDir()
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(dir, "turns.db"))
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	turn := &turns.Turn{ID: "turn-resume"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("first prompt"))
	turns.AppendBlock(turn, turns.NewAssistantTextBlock("first answer"))
	persister := newCLITurnStorePersister(store, "resume-session", "resume-session", "final")
	require.NotNil(t, persister)
	require.NoError(t, persister.PersistTurn(context.Background(), turn))

	loaded, err := loadLatestCLIFinalTurn(context.Background(), store, "resume-session")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, "turn-resume", loaded.ID)
	require.Len(t, loaded.Blocks, 2)
	sessionID, ok, err := turns.KeyTurnMetaSessionID.Get(loaded.Metadata)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "resume-session", sessionID)
}

func TestLoadLatestCLIFinalTurnRequiresStoreAndSession(t *testing.T) {
	_, err := loadLatestCLIFinalTurn(context.Background(), nil, "resume-session")
	require.ErrorContains(t, err, "resume requires --turns-db or --turns-dsn")

	dir := t.TempDir()
	dsn, err := chatstore.SQLiteTurnDSNForFile(filepath.Join(dir, "turns.db"))
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	_, err = loadLatestCLIFinalTurn(context.Background(), store, "")
	require.ErrorContains(t, err, "resume requires --session-id")

	_, err = loadLatestCLIFinalTurn(context.Background(), store, "missing-session")
	require.ErrorContains(t, err, "no persisted final turn found")
}

func TestOpenCLITurnStore_NoneConfigured(t *testing.T) {
	turnStore, cleanup, err := openCLITurnStore(run.PersistenceSettings{})
	require.NoError(t, err)
	require.Nil(t, turnStore)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestOpenCLITurnStore_OpenFromDBPath(t *testing.T) {
	dir := t.TempDir()
	turnsDB := filepath.Join(dir, "turns", "turns.db")

	turnStore, cleanup, err := openCLITurnStore(run.PersistenceSettings{
		TurnsDB: turnsDB,
	})
	require.NoError(t, err)
	require.NotNil(t, turnStore)
	t.Cleanup(cleanup)

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
