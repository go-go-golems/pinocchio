package chatapp

import (
	"context"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
	"github.com/stretchr/testify/require"
)

func TestChatExampleHappyPath(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	payload := &chatappv1.StartInferenceCommand{Prompt: "Explain ordinals"}
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-1"), CommandStartInference, payload))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-1")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-1"))
	require.NoError(t, err)
	require.Equal(t, uint64(6), snap.SnapshotOrdinal)
	require.Len(t, snap.Entities, 2)
	userEntity := snap.Entities[0]
	assistantEntity := snap.Entities[1]
	user := userEntity.Payload.(*chatappv1.ChatMessageEntity)
	assistant := assistantEntity.Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "user", user.GetRole())
	require.Equal(t, "assistant", assistant.GetRole())
	require.Equal(t, "Explain ordinals", user.GetContent())
	require.Equal(t, "finished", assistant.GetStatus())
	require.Equal(t, "Answer: Explain ordinals", assistant.GetText())
	require.Equal(t, uint64(1), userEntity.CreatedOrdinal)
	require.Equal(t, uint64(1), userEntity.LastEventOrdinal)
	require.Equal(t, uint64(3), assistantEntity.CreatedOrdinal)
	require.Equal(t, uint64(6), assistantEntity.LastEventOrdinal)
}

func TestBaseTimelineProjection_DelaysAssistantEntityUntilContentArrives(t *testing.T) {
	startedPayload := &chatappv1.ChatMessageUpdate{MessageId: "chat-msg-start", Prompt: "Explain ordinals", Content: "", Status: "streaming", Streaming: true}

	entities, err := baseTimelineProjection(context.Background(), sessionstream.Event{Name: EventInferenceStarted, SessionId: "chat-projection", Ordinal: 2, Payload: startedPayload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Nil(t, entities)

	finishedPayload := &chatappv1.ChatMessageUpdate{MessageId: "chat-msg-start", Prompt: "Explain ordinals", Content: "Answer: Explain ordinals", Text: "Answer: Explain ordinals", Status: "finished", Streaming: false}
	entities, err = baseTimelineProjection(context.Background(), sessionstream.Event{Name: EventInferenceFinished, SessionId: "chat-projection", Ordinal: 3, Payload: finishedPayload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Len(t, entities, 1)
	payload := entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "assistant", payload.GetRole())
	require.Equal(t, "Answer: Explain ordinals", payload.GetContent())
	require.Equal(t, "Explain ordinals", payload.GetPrompt())
}

func TestFeatureUIProjectionRunsForBaseChatEvents(t *testing.T) {
	engine := NewEngine(WithPlugins(testPlugin{}))
	payload := &chatappv1.ChatMessageUpdate{
		MessageId: "chat-msg-1",
		Role:      "assistant",
		Content:   "done",
		Status:    "finished",
	}

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
	payload := &chatappv1.StartInferenceCommand{Prompt: "Stop me"}
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-2"), CommandStartInference, payload))
	time.Sleep(12 * time.Millisecond)
	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-2"), CommandStopInference, &chatappv1.StopInferenceCommand{}))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-2")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-2"))
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)
	var assistant *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		payloadMsg := entity.Payload.(*chatappv1.ChatMessageEntity)
		if payloadMsg.GetRole() == "assistant" {
			assistant = payloadMsg
		}
	}
	require.Equal(t, "stopped", assistant.GetStatus())
	require.Equal(t, false, assistant.GetStreaming())
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
	payload, ok := ev.Payload.(*chatappv1.ChatMessageUpdate)
	if !ok || payload == nil {
		return nil, true, nil
	}
	return []sessionstream.UIEvent{{Name: "FeatureSawFinished", Payload: &chatappv1.ChatMessageUpdate{MessageId: payload.GetMessageId()}}}, true, nil
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
