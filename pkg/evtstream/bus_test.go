package evtstream

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	busTestCommandName = "PublishOrderedEvent"
	busTestEventName   = "OrderedEvent"
	busTestUIName      = "OrderedEventObserved"
	busTestEntityKind  = "OrderedRecord"
)

func TestHubEventBusGoChannelRoundTrip(t *testing.T) {
	pubsub := gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 64}, watermill.NopLogger{})
	store := newTestHydrationStore()
	fanout := &recordingFanout{}
	observer := &recordingBusObserver{}
	sequence := uint64(0)

	hub := newBusTestHub(t, store, fanout, observer, pubsub, func(_ context.Context, _ Event, msg *message.Message) error {
		sequence++
		msg.Metadata.Set(MetadataKeyStreamID, fmt.Sprintf("1700000000000-%d", sequence))
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hub.Run(ctx))
	defer func() { _ = hub.Shutdown(context.Background()) }()

	require.NoError(t, submitBusCommand(t, hub, "s-a", "a-1"))
	require.NoError(t, submitBusCommand(t, hub, "s-b", "b-1"))
	require.NoError(t, submitBusCommand(t, hub, "s-a", "a-2"))

	require.Eventually(t, func() bool {
		fanout.mu.Lock()
		defer fanout.mu.Unlock()
		return len(fanout.events["s-a"]) == 2 && len(fanout.events["s-b"]) == 1
	}, time.Second, 10*time.Millisecond)

	snapA, err := hub.Snapshot(context.Background(), "s-a")
	require.NoError(t, err)
	require.Greater(t, snapA.Ordinal, uint64(0))
	require.Len(t, snapA.Entities, 2)

	snapB, err := hub.Snapshot(context.Background(), "s-b")
	require.NoError(t, err)
	require.Greater(t, snapB.Ordinal, uint64(0))
	require.Len(t, snapB.Entities, 1)

	observer.mu.Lock()
	defer observer.mu.Unlock()
	require.Len(t, observer.published, 3)
	require.Len(t, observer.consumed, 3)
	require.Equal(t, uint64(0), observer.published[0].event.Ordinal)
	require.Equal(t, "s-a", string(observer.published[0].event.SessionId))
	require.NotEmpty(t, observer.published[0].record.Metadata[MetadataKeyStreamID])
	firstDerived, ok := DeriveOrdinalFromStreamID(observer.consumed[0].record.Metadata[MetadataKeyStreamID])
	require.True(t, ok)
	require.Equal(t, firstDerived, observer.consumed[0].event.Ordinal)
	require.Greater(t, observer.consumed[2].event.Ordinal, observer.consumed[0].event.Ordinal)
}

func TestHubEventBusFallsBackWithoutStreamID(t *testing.T) {
	pubsub := gochannel.NewGoChannel(gochannel.Config{OutputChannelBuffer: 64}, watermill.NopLogger{})
	store := newTestHydrationStore()
	fanout := &recordingFanout{}
	observer := &recordingBusObserver{}

	hub := newBusTestHub(t, store, fanout, observer, pubsub, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, hub.Run(ctx))
	defer func() { _ = hub.Shutdown(context.Background()) }()

	require.NoError(t, submitBusCommand(t, hub, "s-a", "a-1"))
	require.NoError(t, submitBusCommand(t, hub, "s-a", "a-2"))

	require.Eventually(t, func() bool {
		cursor, err := hub.Cursor(context.Background(), "s-a")
		return err == nil && cursor == 2
	}, time.Second, 10*time.Millisecond)

	snap, err := hub.Snapshot(context.Background(), "s-a")
	require.NoError(t, err)
	require.Equal(t, uint64(2), snap.Ordinal)

	observer.mu.Lock()
	defer observer.mu.Unlock()
	require.Len(t, observer.consumed, 2)
	require.Equal(t, uint64(1), observer.consumed[0].event.Ordinal)
	require.Equal(t, uint64(2), observer.consumed[1].event.Ordinal)
}

func newBusTestHub(
	t *testing.T,
	store HydrationStore,
	fanout UIFanout,
	observer BusObserver,
	pubsub *gochannel.GoChannel,
	mutator BusMessageMutator,
) *Hub {
	t.Helper()
	reg := NewSchemaRegistry()
	require.NoError(t, reg.RegisterCommand(busTestCommandName, &structpb.Struct{}))
	require.NoError(t, reg.RegisterEvent(busTestEventName, &structpb.Struct{}))
	require.NoError(t, reg.RegisterUIEvent(busTestUIName, &structpb.Struct{}))
	require.NoError(t, reg.RegisterTimelineEntity(busTestEntityKind, &structpb.Struct{}))

	hub, err := NewHub(
		WithSchemaRegistry(reg),
		WithHydrationStore(store),
		WithUIFanout(fanout),
		WithSessionMetadataFactory(func(_ context.Context, sid SessionId) (any, error) {
			return map[string]any{"sessionId": string(sid)}, nil
		}),
		WithEventBus(pubsub, pubsub, WithBusTopic("evtstream.test"), WithBusObserver(observer), WithBusMessageMutator(mutator)),
	)
	require.NoError(t, err)
	require.NoError(t, hub.RegisterCommand(busTestCommandName, func(ctx context.Context, cmd Command, _ *Session, pub EventPublisher) error {
		payload := cmd.Payload.(*structpb.Struct).AsMap()
		evPayload, err := structpb.NewStruct(map[string]any{"label": payload["label"]})
		require.NoError(t, err)
		return pub.Publish(ctx, Event{Name: busTestEventName, SessionId: cmd.SessionId, Payload: evPayload})
	}))
	require.NoError(t, hub.RegisterUIProjection(UIProjectionFunc(func(_ context.Context, ev Event, _ *Session, _ TimelineView) ([]UIEvent, error) {
		return []UIEvent{{Name: busTestUIName, Payload: proto.Clone(ev.Payload)}}, nil
	})))
	require.NoError(t, hub.RegisterTimelineProjection(TimelineProjectionFunc(func(_ context.Context, ev Event, _ *Session, _ TimelineView) ([]TimelineEntity, error) {
		payload := ev.Payload.(*structpb.Struct).AsMap()
		entityPayload, err := structpb.NewStruct(map[string]any{"label": payload["label"], "ordinal": fmt.Sprintf("%d", ev.Ordinal)})
		require.NoError(t, err)
		return []TimelineEntity{{Kind: busTestEntityKind, Id: fmt.Sprintf("%v", payload["label"]), Payload: entityPayload}}, nil
	})))
	return hub
}

func submitBusCommand(t *testing.T, hub *Hub, sid SessionId, label string) error {
	t.Helper()
	payload, err := structpb.NewStruct(map[string]any{"label": label})
	require.NoError(t, err)
	return hub.Submit(context.Background(), sid, busTestCommandName, payload)
}

type recordedBusEvent struct {
	event  Event
	record BusRecord
}

type recordingBusObserver struct {
	mu        sync.Mutex
	published []recordedBusEvent
	consumed  []recordedBusEvent
}

func (o *recordingBusObserver) Published(_ context.Context, ev Event, rec BusRecord) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.published = append(o.published, recordedBusEvent{event: cloneObservedEvent(ev), record: rec})
}

func (o *recordingBusObserver) Consumed(_ context.Context, ev Event, rec BusRecord) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.consumed = append(o.consumed, recordedBusEvent{event: cloneObservedEvent(ev), record: rec})
}

type recordingFanout struct {
	mu     sync.Mutex
	events map[string][]uint64
}

func (f *recordingFanout) PublishUI(_ context.Context, sid SessionId, ord uint64, _ []UIEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.events == nil {
		f.events = map[string][]uint64{}
	}
	f.events[string(sid)] = append(f.events[string(sid)], ord)
	return nil
}

func cloneObservedEvent(ev Event) Event {
	out := ev
	if ev.Payload != nil {
		out.Payload = proto.Clone(ev.Payload)
	}
	return out
}
