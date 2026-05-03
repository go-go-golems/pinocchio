package chatapp

import (
	"context"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestChatExampleHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Explain ordinals"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-1"), CommandStartInference, payload))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-1")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-1"))
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

func TestBaseTimelineProjection_DelaysAssistantEntityUntilContentArrives(t *testing.T) {
	startedPayload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-start", "prompt": "Explain ordinals", "content": "", "status": "streaming", "streaming": true})
	require.NoError(t, err)

	entities, err := baseTimelineProjection(context.Background(), sessionstream.Event{Name: EventInferenceStarted, SessionId: "chat-projection", Ordinal: 2, Payload: startedPayload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Nil(t, entities)

	finishedPayload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-start", "prompt": "Explain ordinals", "content": "Answer: Explain ordinals", "text": "Answer: Explain ordinals", "status": "finished", "streaming": false})
	require.NoError(t, err)
	entities, err = baseTimelineProjection(context.Background(), sessionstream.Event{Name: EventInferenceFinished, SessionId: "chat-projection", Ordinal: 3, Payload: finishedPayload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Len(t, entities, 1)
	payload := entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "assistant", payload["role"])
	require.Equal(t, "Answer: Explain ordinals", payload["content"])
	require.Equal(t, "Explain ordinals", payload["prompt"])
}

func TestFeatureUIProjectionRunsForBaseChatEvents(t *testing.T) {
	engine := NewEngine(WithPlugins(testPlugin{}))
	payload, err := structpb.NewStruct(map[string]any{
		"messageId": "chat-msg-1",
		"role":      "assistant",
		"content":   "done",
		"status":    "finished",
	})
	require.NoError(t, err)

	uiEvents, err := engine.uiProjection(context.Background(), sessionstream.Event{Name: EventInferenceFinished, SessionId: "chat-feature", Ordinal: 3, Payload: payload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Len(t, uiEvents, 2)
	require.Equal(t, UIMessageFinished, uiEvents[0].Name)
	require.Equal(t, "FeatureSawFinished", uiEvents[1].Name)
}

func TestPendingRequestsAreKeyedByRequestID(t *testing.T) {
	engine := NewEngine()
	engine.setPendingRequest("request-1", PromptRequest{Prompt: "first"})
	engine.setPendingRequest("request-2", PromptRequest{Prompt: "second"})

	require.Equal(t, "first", engine.takePendingRequest("request-1").Prompt)
	require.Equal(t, "second", engine.takePendingRequest("request-2").Prompt)
	require.Empty(t, engine.takePendingRequest("request-1").Prompt)
}

func TestChatExampleStopPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(10 * time.Millisecond))
	hub := newTestHub(t, engine)
	payload, err := structpb.NewStruct(map[string]any{"prompt": "Stop me"})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-2"), CommandStartInference, payload))
	time.Sleep(12 * time.Millisecond)
	stop, err := structpb.NewStruct(map[string]any{})
	require.NoError(t, err)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-2"), CommandStopInference, stop))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-2")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-2"))
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

type testPlugin struct{}

func (testPlugin) RegisterSchemas(*sessionstream.SchemaRegistry) error { return nil }

func (testPlugin) HandleRuntimeEvent(context.Context, RuntimeEventContext, gepevents.Event) (bool, error) {
	return false, nil
}

func (testPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	if ev.Name != EventInferenceFinished {
		return nil, false, nil
	}
	payload, err := structpb.NewStruct(map[string]any{"messageId": asString(toMap(ev.Payload)["messageId"])})
	if err != nil {
		return nil, true, err
	}
	return []sessionstream.UIEvent{{Name: "FeatureSawFinished", Payload: payload}}, true, nil
}

func (testPlugin) ProjectTimeline(context.Context, sessionstream.Event, *sessionstream.Session, sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	return nil, false, nil
}

type staticTimelineView struct{}

func (staticTimelineView) Get(string, string) (sessionstream.TimelineEntity, bool) {
	return sessionstream.TimelineEntity{}, false
}

func (staticTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (staticTimelineView) Ordinal() uint64                            { return 0 }

func newTestHub(t *testing.T, engine *Engine) *sessionstream.Hub {
	t.Helper()
	reg := sessionstream.NewSchemaRegistry()
	require.NoError(t, RegisterSchemas(reg))
	store, err := storesqlite.NewInMemory(reg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	hub, err := sessionstream.NewHub(
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
	)
	require.NoError(t, err)
	require.NoError(t, Install(hub, engine))
	return hub
}
