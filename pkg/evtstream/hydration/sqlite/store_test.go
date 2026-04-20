package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStoreApplySnapshotAndCursor(t *testing.T) {
	store := newTestStore(t)
	payload, err := structpb.NewStruct(map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.NoError(t, store.Apply(context.Background(), evtstream.SessionId("s-1"), 7, []evtstream.TimelineEntity{{Kind: "TestEntity", Id: "msg-1", Payload: payload}}))

	snap, err := store.Snapshot(context.Background(), evtstream.SessionId("s-1"), 0)
	require.NoError(t, err)
	require.Equal(t, uint64(7), snap.Ordinal)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, "hello", snap.Entities[0].Payload.(*structpb.Struct).AsMap()["text"])
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evtstream.sqlite")
	dsn, err := FileDSN(path)
	require.NoError(t, err)
	reg := newTestRegistry(t)
	store, err := New(dsn, reg)
	require.NoError(t, err)
	payload, err := structpb.NewStruct(map[string]any{"text": "persisted"})
	require.NoError(t, err)
	require.NoError(t, store.Apply(context.Background(), evtstream.SessionId("s-2"), 9, []evtstream.TimelineEntity{{Kind: "TestEntity", Id: "msg-1", Payload: payload}}))
	require.NoError(t, store.Close())

	reopened, err := New(dsn, reg)
	require.NoError(t, err)
	defer reopened.Close()
	snap, err := reopened.Snapshot(context.Background(), evtstream.SessionId("s-2"), 0)
	require.NoError(t, err)
	require.Equal(t, uint64(9), snap.Ordinal)
	require.Equal(t, "persisted", snap.Entities[0].Payload.(*structpb.Struct).AsMap()["text"])
}

func TestStoreReset(t *testing.T) {
	store := newTestStore(t)
	payload, err := structpb.NewStruct(map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.NoError(t, store.Apply(context.Background(), evtstream.SessionId("s-3"), 3, []evtstream.TimelineEntity{{Kind: "TestEntity", Id: "msg-1", Payload: payload}}))
	require.NoError(t, store.Reset(context.Background()))
	cursor, err := store.Cursor(context.Background(), evtstream.SessionId("s-3"))
	require.NoError(t, err)
	require.Equal(t, uint64(0), cursor)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn, err := FileDSN(filepath.Join(t.TempDir(), "evtstream.sqlite"))
	require.NoError(t, err)
	store, err := New(dsn, newTestRegistry(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newTestRegistry(t *testing.T) *evtstream.SchemaRegistry {
	t.Helper()
	reg := evtstream.NewSchemaRegistry()
	require.NoError(t, reg.RegisterTimelineEntity("TestEntity", &structpb.Struct{}))
	return reg
}
