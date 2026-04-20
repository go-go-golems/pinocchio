package evtstream

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	testCommandName = "LabStart"
	testEventName   = "LabStarted"
	testEntityKind  = "LabMessage"
)

func TestHubSubmitRunsHandlerProjectionAndStore(t *testing.T) {
	hub := newTestHub(t)
	registerTestHandler(t, hub)

	uiEvents := make([]UIEvent, 0)
	require.NoError(t, hub.RegisterUIProjection(UIProjectionFunc(func(_ context.Context, ev Event, _ *Session, _ TimelineView) ([]UIEvent, error) {
		uiEvents = append(uiEvents, UIEvent{Name: "LabMessageStarted", Payload: ev.Payload})
		return []UIEvent{{Name: "LabMessageStarted", Payload: ev.Payload}}, nil
	})))
	require.NoError(t, hub.RegisterTimelineProjection(TimelineProjectionFunc(func(_ context.Context, ev Event, _ *Session, _ TimelineView) ([]TimelineEntity, error) {
		payload := ev.Payload.(*structpb.Struct).AsMap()
		entityPayload, err := structpb.NewStruct(map[string]any{"prompt": payload["prompt"]})
		require.NoError(t, err)
		return []TimelineEntity{{Kind: testEntityKind, Id: "msg-1", Payload: entityPayload}}, nil
	})))

	cmdPayload, err := structpb.NewStruct(map[string]any{"prompt": "hello"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), "s-1", testCommandName, cmdPayload))

	session, err := hub.Session(context.Background(), "s-1")
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, map[string]any{"sessionId": "s-1"}, session.Metadata)

	cursor, err := hub.Cursor(context.Background(), "s-1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), cursor)
	require.Len(t, uiEvents, 1)

	snap, err := hub.Snapshot(context.Background(), "s-1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), snap.Ordinal)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, "hello", snap.Entities[0].Payload.(*structpb.Struct).AsMap()["prompt"])
}

func TestHubSubmitUnknownCommand(t *testing.T) {
	hub := newTestHub(t)
	cmdPayload, err := structpb.NewStruct(map[string]any{"prompt": "hello"})
	require.NoError(t, err)

	err = hub.Submit(context.Background(), "s-1", testCommandName, cmdPayload)
	require.Error(t, err)
	require.ErrorContains(t, err, `unknown command "LabStart"`)
}

func TestHubProjectionErrorPolicyFailReturnsProjectionError(t *testing.T) {
	hub := newTestHub(t, WithProjectionErrorPolicy(ProjectionErrorPolicyFail))
	registerTestHandler(t, hub)
	boom := errors.New("projection exploded")
	require.NoError(t, hub.RegisterTimelineProjection(TimelineProjectionFunc(func(context.Context, Event, *Session, TimelineView) ([]TimelineEntity, error) {
		return nil, boom
	})))

	cmdPayload, err := structpb.NewStruct(map[string]any{"prompt": "hello"})
	require.NoError(t, err)
	err = hub.Submit(context.Background(), "s-1", testCommandName, cmdPayload)
	require.ErrorIs(t, err, boom)

	cursor, err := hub.Cursor(context.Background(), "s-1")
	require.NoError(t, err)
	require.Equal(t, uint64(0), cursor)
}

func TestHubProjectionErrorPolicyAdvanceStillAdvancesCursor(t *testing.T) {
	hub := newTestHub(t, WithProjectionErrorPolicy(ProjectionErrorPolicyAdvance))
	registerTestHandler(t, hub)
	require.NoError(t, hub.RegisterTimelineProjection(TimelineProjectionFunc(func(context.Context, Event, *Session, TimelineView) ([]TimelineEntity, error) {
		return nil, errors.New("projection exploded")
	})))

	cmdPayload, err := structpb.NewStruct(map[string]any{"prompt": "hello"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), "s-1", testCommandName, cmdPayload))

	cursor, err := hub.Cursor(context.Background(), "s-1")
	require.NoError(t, err)
	require.Equal(t, uint64(1), cursor)
}

func newTestHub(t *testing.T, opts ...HubOption) *Hub {
	t.Helper()
	reg := NewSchemaRegistry()
	require.NoError(t, reg.RegisterCommand(testCommandName, &structpb.Struct{}))
	require.NoError(t, reg.RegisterEvent(testEventName, &structpb.Struct{}))
	require.NoError(t, reg.RegisterUIEvent("LabMessageStarted", &structpb.Struct{}))
	require.NoError(t, reg.RegisterTimelineEntity(testEntityKind, &structpb.Struct{}))

	allOpts := append([]HubOption{
		WithSchemaRegistry(reg),
		WithHydrationStore(newTestHydrationStore()),
		WithSessionMetadataFactory(func(_ context.Context, sid SessionId) (any, error) {
			return map[string]any{"sessionId": string(sid)}, nil
		}),
	}, opts...)
	hub, err := NewHub(allOpts...)
	require.NoError(t, err)
	return hub
}

func registerTestHandler(t *testing.T, hub *Hub) {
	t.Helper()
	require.NoError(t, hub.RegisterCommand(testCommandName, func(ctx context.Context, cmd Command, _ *Session, pub EventPublisher) error {
		payload := cmd.Payload.(*structpb.Struct).AsMap()
		evPayload, err := structpb.NewStruct(map[string]any{"prompt": payload["prompt"]})
		require.NoError(t, err)
		return pub.Publish(ctx, Event{Name: testEventName, SessionId: cmd.SessionId, Payload: evPayload})
	}))
}

type testHydrationStore struct {
	snapshots map[SessionId]Snapshot
}

func newTestHydrationStore() HydrationStore {
	return &testHydrationStore{snapshots: map[SessionId]Snapshot{}}
}

func (s *testHydrationStore) Apply(_ context.Context, sid SessionId, ord uint64, entities []TimelineEntity) error {
	snap := s.snapshots[sid]
	snap.SessionId = sid
	if ord > snap.Ordinal {
		snap.Ordinal = ord
	}
	entityMap := map[string]TimelineEntity{}
	for _, entity := range snap.Entities {
		entityMap[entity.Kind+"/"+entity.Id] = cloneTestEntity(entity)
	}
	for _, entity := range entities {
		key := entity.Kind + "/" + entity.Id
		if entity.Tombstone {
			delete(entityMap, key)
			continue
		}
		entityMap[key] = cloneTestEntity(entity)
	}
	snap.Entities = snap.Entities[:0]
	for _, entity := range entityMap {
		snap.Entities = append(snap.Entities, cloneTestEntity(entity))
	}
	s.snapshots[sid] = snap
	return nil
}

func (s *testHydrationStore) Snapshot(_ context.Context, sid SessionId, _ uint64) (Snapshot, error) {
	snap, ok := s.snapshots[sid]
	if !ok {
		return Snapshot{SessionId: sid}, nil
	}
	out := Snapshot{SessionId: snap.SessionId, Ordinal: snap.Ordinal, Entities: make([]TimelineEntity, 0, len(snap.Entities))}
	for _, entity := range snap.Entities {
		out.Entities = append(out.Entities, cloneTestEntity(entity))
	}
	return out, nil
}

func (s *testHydrationStore) View(ctx context.Context, sid SessionId) (TimelineView, error) {
	snap, err := s.Snapshot(ctx, sid, 0)
	if err != nil {
		return nil, err
	}
	return testTimelineView{snapshot: snap}, nil
}

func (s *testHydrationStore) Cursor(_ context.Context, sid SessionId) (uint64, error) {
	return s.snapshots[sid].Ordinal, nil
}

type testTimelineView struct {
	snapshot Snapshot
}

func (v testTimelineView) Get(kind, id string) (TimelineEntity, bool) {
	for _, entity := range v.snapshot.Entities {
		if entity.Kind == kind && entity.Id == id {
			return cloneTestEntity(entity), true
		}
	}
	return TimelineEntity{}, false
}

func (v testTimelineView) List(kind string) []TimelineEntity {
	ret := make([]TimelineEntity, 0)
	for _, entity := range v.snapshot.Entities {
		if kind != "" && entity.Kind != kind {
			continue
		}
		ret = append(ret, cloneTestEntity(entity))
	}
	return ret
}

func (v testTimelineView) Ordinal() uint64 { return v.snapshot.Ordinal }

func cloneTestEntity(entity TimelineEntity) TimelineEntity {
	out := entity
	if entity.Payload != nil {
		out.Payload = proto.Clone(entity.Payload)
	}
	return out
}
