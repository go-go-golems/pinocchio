package chatstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteTurnStore_SaveAndList(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	err = s.Save(ctx, "conv-1", "sess-1", "turn-1", "final", 100, "payload-1")
	require.NoError(t, err)
	err = s.Save(ctx, "conv-1", "sess-1", "turn-2", "draft", 200, "payload-2")
	require.NoError(t, err)
	err = s.Save(ctx, "conv-2", "sess-2", "turn-3", "final", 300, "payload-3")
	require.NoError(t, err)

	items, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Limit: 10})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "turn-2", items[0].TurnID)
	require.Equal(t, "turn-1", items[1].TurnID)

	bySession, err := s.List(ctx, TurnQuery{SessionID: "sess-2", Limit: 10})
	require.NoError(t, err)
	require.Len(t, bySession, 1)
	require.Equal(t, "turn-3", bySession[0].TurnID)

	byPhase, err := s.List(ctx, TurnQuery{ConvID: "conv-1", Phase: "final", Limit: 10})
	require.NoError(t, err)
	require.Len(t, byPhase, 1)
	require.Equal(t, "turn-1", byPhase[0].TurnID)

	_, err = os.Stat(dbPath)
	require.NoError(t, err)
}

func TestSQLiteTurnStore_Validation(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "turns.db")
	dsn, err := SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)

	s, err := NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	err = s.Save(ctx, "", "sess-1", "turn-1", "final", 1, "payload")
	require.Error(t, err)

	_, err = s.List(ctx, TurnQuery{})
	require.Error(t, err)
}
