package chat

import (
	"context"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	storememory "github.com/go-go-golems/pinocchio/pkg/evtstream/hydration/memory"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestChatExampleHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Explain ordinals"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), evtstream.SessionId("chat-1"), CommandStartInference, payload))
	require.NoError(t, engine.WaitIdle(context.Background(), evtstream.SessionId("chat-1")))

	snap, err := hub.Snapshot(context.Background(), evtstream.SessionId("chat-1"))
	require.NoError(t, err)
	require.Equal(t, uint64(6), snap.Ordinal)
	require.Len(t, snap.Entities, 2)
	var assistant map[string]any
	var user map[string]any
	for _, entity := range snap.Entities {
		payloadMap := entity.Payload.(*structpb.Struct).AsMap()
		switch payloadMap["role"] {
		case "assistant":
			assistant = payloadMap
		case "user":
			user = payloadMap
		}
	}
	require.Equal(t, "Explain ordinals", user["content"])
	require.Equal(t, "finished", assistant["status"])
	require.Equal(t, "Answer: Explain ordinals", assistant["text"])
}

func TestChatExampleStopPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(10 * time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Stop me"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), evtstream.SessionId("chat-2"), CommandStartInference, payload))
	time.Sleep(12 * time.Millisecond)
	stop, err := structpb.NewStruct(map[string]any{})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), evtstream.SessionId("chat-2"), CommandStopInference, stop))
	require.NoError(t, engine.WaitIdle(context.Background(), evtstream.SessionId("chat-2")))

	snap, err := hub.Snapshot(context.Background(), evtstream.SessionId("chat-2"))
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)
	var assistant map[string]any
	for _, entity := range snap.Entities {
		payloadMap := entity.Payload.(*structpb.Struct).AsMap()
		if payloadMap["role"] == "assistant" {
			assistant = payloadMap
		}
	}
	require.Equal(t, "stopped", assistant["status"])
	require.Equal(t, false, assistant["streaming"])
}

func TestRuntimeEventSinkPublishesAgentModePreview(t *testing.T) {
	pub := &capturingPublisher{}
	sink := &runtimeEventSink{sessionID: evtstream.SessionId("chat-preview"), messageID: "chat-msg-1", pub: pub, engine: NewEngine()}

	err := sink.PublishEvent(agentmode.NewModeSwitchPreviewEvent(gepevents.EventMetadata{SessionID: "chat-preview"}, "item-1", "reviewer", "hello", "candidate"))
	require.NoError(t, err)
	require.Len(t, pub.events, 1)
	require.Equal(t, EventAgentModePreview, pub.events[0].Name)
	payload := pub.events[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "chat-msg-1", payload["messageId"])
	require.Equal(t, "reviewer", payload["candidateMode"])
	require.Equal(t, "hello", payload["analysis"])
	require.Equal(t, "candidate", payload["parseState"])
	require.Equal(t, true, payload["preview"])
}

func TestRuntimeEventSinkPublishesAgentModeCommitted(t *testing.T) {
	pub := &capturingPublisher{}
	sink := &runtimeEventSink{sessionID: evtstream.SessionId("chat-commit"), messageID: "chat-msg-2", pub: pub, engine: NewEngine()}

	err := sink.PublishEvent(gepevents.NewAgentModeSwitchEvent(gepevents.EventMetadata{SessionID: "chat-commit"}, "analyst", "reviewer", "needs critique"))
	require.NoError(t, err)
	require.Len(t, pub.events, 1)
	require.Equal(t, EventAgentModeCommitted, pub.events[0].Name)
	payload := pub.events[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "chat-msg-2", payload["messageId"])
	require.Equal(t, "analyst", payload["from"])
	require.Equal(t, "reviewer", payload["to"])
	require.Equal(t, "needs critique", payload["analysis"])
	require.Equal(t, false, payload["preview"])
}

func TestUIProjectionEmitsAgentModePreviewAndCommitEvents(t *testing.T) {
	previewPayload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-3", "candidateMode": "reviewer", "analysis": "hello", "parseState": "candidate"})
	require.NoError(t, err)
	previewEvents, err := uiProjection(context.Background(), evtstream.Event{Name: EventAgentModePreview, SessionId: "sid", Ordinal: 7, Payload: previewPayload}, nil, nil)
	require.NoError(t, err)
	require.Len(t, previewEvents, 1)
	require.Equal(t, UIAgentModePreview, previewEvents[0].Name)

	commitPayload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-3", "title": "agentmode: mode switched", "from": "analyst", "to": "reviewer", "analysis": "hello"})
	require.NoError(t, err)
	commitEvents, err := uiProjection(context.Background(), evtstream.Event{Name: EventAgentModeCommitted, SessionId: "sid", Ordinal: 8, Payload: commitPayload}, nil, nil)
	require.NoError(t, err)
	require.Len(t, commitEvents, 2)
	require.Equal(t, UIAgentModeCommitted, commitEvents[0].Name)
	require.Equal(t, UIAgentModePreviewClear, commitEvents[1].Name)
}

func TestTimelineProjectionPersistsCommittedAgentModeEntity(t *testing.T) {
	payload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-4", "title": "agentmode: mode switched", "from": "analyst", "to": "reviewer", "analysis": "hello"})
	require.NoError(t, err)
	entities, err := timelineProjection(context.Background(), evtstream.Event{Name: EventAgentModeCommitted, SessionId: "sid", Ordinal: 9, Payload: payload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Len(t, entities, 1)
	require.Equal(t, TimelineEntityAgentMode, entities[0].Kind)
	require.Equal(t, "session", entities[0].Id)
	entityPayload := entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "agentmode: mode switched", entityPayload["title"])
	require.Equal(t, false, entityPayload["preview"])
	data := entityPayload["data"].(map[string]any)
	require.Equal(t, "analyst", data["from"])
	require.Equal(t, "reviewer", data["to"])
}

func newTestHub(t *testing.T, engine *Engine) *evtstream.Hub {
	t.Helper()
	reg := evtstream.NewSchemaRegistry()
	require.NoError(t, RegisterSchemas(reg))
	hub, err := evtstream.NewHub(
		evtstream.WithSchemaRegistry(reg),
		evtstream.WithHydrationStore(storememory.New()),
	)
	require.NoError(t, err)
	require.NoError(t, Install(hub, engine))
	return hub
}

type capturingPublisher struct {
	events []evtstream.Event
}

func (p *capturingPublisher) Publish(_ context.Context, ev evtstream.Event) error {
	p.events = append(p.events, ev)
	return nil
}

type staticTimelineView struct{}

func (staticTimelineView) Get(string, string) (evtstream.TimelineEntity, bool) {
	return evtstream.TimelineEntity{}, false
}
func (staticTimelineView) List(string) []evtstream.TimelineEntity { return nil }
func (staticTimelineView) Ordinal() uint64                        { return 0 }
