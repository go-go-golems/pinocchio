package memory

import (
	"context"
	"testing"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStoreApplySnapshotAndViewAreDefensive(t *testing.T) {
	store := New()
	sid := evtstream.SessionId("s-1")
	payload, err := structpb.NewStruct(map[string]any{"text": "hello"})
	require.NoError(t, err)

	require.NoError(t, store.Apply(context.Background(), sid, 3, []evtstream.TimelineEntity{{
		Kind:    "LabMessage",
		Id:      "msg-1",
		Payload: payload,
	}}))

	snap, err := store.Snapshot(context.Background(), sid, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(3), snap.Ordinal)
	require.Len(t, snap.Entities, 1)

	snapPayload := snap.Entities[0].Payload.(*structpb.Struct)
	snapPayload.Fields["text"] = structpb.NewStringValue("mutated")

	view, err := store.View(context.Background(), sid)
	require.NoError(t, err)
	entity, ok := view.Get("LabMessage", "msg-1")
	require.True(t, ok)
	require.Equal(t, "hello", entity.Payload.(*structpb.Struct).AsMap()["text"])
}

func TestStoreSupportsTombstones(t *testing.T) {
	store := New()
	sid := evtstream.SessionId("s-1")
	payload, err := structpb.NewStruct(map[string]any{"text": "hello"})
	require.NoError(t, err)

	require.NoError(t, store.Apply(context.Background(), sid, 1, []evtstream.TimelineEntity{{
		Kind:    "LabMessage",
		Id:      "msg-1",
		Payload: payload,
	}}))
	require.NoError(t, store.Apply(context.Background(), sid, 2, []evtstream.TimelineEntity{{
		Kind:      "LabMessage",
		Id:        "msg-1",
		Tombstone: true,
	}}))

	snap, err := store.Snapshot(context.Background(), sid, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(2), snap.Ordinal)
	require.Empty(t, snap.Entities)
}
