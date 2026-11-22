package cmds

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestOpenCLISessionstreamHydrationStore_NoneConfigured(t *testing.T) {
	store, cleanup, err := openCLISessionstreamHydrationStore(context.Background(), run.PersistenceSettings{}, sessionstream.NewSchemaRegistry())
	require.NoError(t, err)
	require.Nil(t, store)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestOpenCLISessionstreamHydrationStore_OpenFromDBPath(t *testing.T) {
	dir := t.TempDir()
	timelineDB := filepath.Join(dir, "timeline", "timeline.db")

	store, cleanup, err := openCLISessionstreamHydrationStore(context.Background(), run.PersistenceSettings{TimelineDB: timelineDB}, sessionstream.NewSchemaRegistry())
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
	require.NoError(t, turns.KeyTurnMetaSessionID.Set(&turn.Metadata, "resume-session"))
	payload, err := serde.ToYAML(turn, serde.Options{})
	require.NoError(t, err)
	require.NoError(t, store.Save(context.Background(), "resume-session", "resume-session", turn.ID, "final", time.Now().UnixMilli(), string(payload), chatstore.TurnSaveOptions{}))

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
	turnStore, cleanup, err := openCLITurnStore(context.Background(), run.PersistenceSettings{})
	require.NoError(t, err)
	require.Nil(t, turnStore)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestOpenCLITurnStore_OpenFromDBPath(t *testing.T) {
	dir := t.TempDir()
	turnsDB := filepath.Join(dir, "turns", "turns.db")

	turnStore, cleanup, err := openCLITurnStore(context.Background(), run.PersistenceSettings{
		TurnsDB: turnsDB,
	})
	require.NoError(t, err)
	require.NotNil(t, turnStore)
	t.Cleanup(cleanup)

	_, err = os.Stat(filepath.Dir(turnsDB))
	require.NoError(t, err)
}
