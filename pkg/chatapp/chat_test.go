package chatapp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
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
	require.Equal(t, uint64(8), snap.SnapshotOrdinal)
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
	require.Equal(t, uint64(4), assistantEntity.CreatedOrdinal)
	require.Equal(t, uint64(7), assistantEntity.LastEventOrdinal)
}

func TestBaseTimelineProjection_DelaysAssistantEntityUntilContentArrives(t *testing.T) {
	startedPayload := &chatappv1.ChatTextSegmentStarted{MessageId: "chat-msg-start:text:1", Prompt: "Explain ordinals", Status: "streaming", Streaming: true, Correlation: &chatappv1.CorrelationInfo{SegmentIndex: 1, SegmentType: "text"}}

	entities, err := baseTimelineProjection(context.Background(), sessionstream.Event{Name: EventChatTextSegmentStarted, SessionId: "chat-projection", Ordinal: 2, Payload: startedPayload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Nil(t, entities)

	finishedPayload := &chatappv1.ChatTextSegmentFinished{MessageId: "chat-msg-start:text:1", Prompt: "Explain ordinals", Content: "Answer: Explain ordinals", Text: "Answer: Explain ordinals", Status: "finished", Streaming: false, Correlation: &chatappv1.CorrelationInfo{SegmentIndex: 1, SegmentType: "text"}}
	entities, err = baseTimelineProjection(context.Background(), sessionstream.Event{Name: EventChatTextSegmentFinished, SessionId: "chat-projection", Ordinal: 3, Payload: finishedPayload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Len(t, entities, 1)
	payload := entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "assistant", payload.GetRole())
	require.Equal(t, "Answer: Explain ordinals", payload.GetContent())
	require.Equal(t, "Explain ordinals", payload.GetPrompt())
}

func TestFeatureUIProjectionRunsForBaseChatEvents(t *testing.T) {
	engine := NewEngine(WithPlugins(testPlugin{}))
	payload := &chatappv1.ChatTextSegmentFinished{
		MessageId: "chat-msg-1:text:1",
		Role:      "assistant",
		Content:   "done",
		Status:    "finished",
	}

	uiEvents, err := engine.uiProjection(context.Background(), sessionstream.Event{Name: EventChatTextSegmentFinished, SessionId: "chat-feature", Ordinal: 3, Payload: payload}, nil, staticTimelineView{})
	require.NoError(t, err)
	require.Len(t, uiEvents, 2)
	require.Equal(t, EventChatTextSegmentFinished, uiEvents[0].Name)
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

func TestRuntimeInferenceLoadsLatestTurnHistory(t *testing.T) {
	ctx := context.Background()
	prior := &turns.Turn{ID: "turn-prior"}
	turns.AppendBlock(prior, turns.NewUserTextBlock("What products do we have?"))
	turns.AppendBlock(prior, turns.NewAssistantTextBlock("We have American Gold Eagles."))
	payload, err := serde.ToYAML(prior, serde.Options{})
	require.NoError(t, err)

	store := &fakeTurnStore{snapshot: &chatstore.TurnSnapshot{
		ConvID:      "chat-history",
		SessionID:   "chat-history",
		TurnID:      "turn-prior",
		Phase:       "final",
		CreatedAtMs: 100,
		Payload:     string(payload),
	}}
	recorder := &recordingHistoryEngine{}
	engine := NewEngine(WithChunkDelay(time.Millisecond), WithTurnStore(store))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-history", PromptRequest{
		Prompt: "Tell me more about the first one",
		Runtime: &infruntime.ComposedRuntime{
			Engine: recorder,
		},
	})

	require.NoError(t, hub.Submit(ctx, sessionstream.SessionId("chat-history"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-history"}))
	require.NoError(t, engine.WaitIdle(ctx, sessionstream.SessionId("chat-history")))

	seen := recorder.seen
	require.NotNil(t, seen)
	require.Equal(t, "chat-history", recorder.sessionID)
	require.Len(t, seen.Blocks, 3)
	require.Equal(t, turns.RoleUser, seen.Blocks[0].Role)
	require.Equal(t, "What products do we have?", seen.Blocks[0].Payload[turns.PayloadKeyText])
	require.Equal(t, turns.RoleAssistant, seen.Blocks[1].Role)
	require.Equal(t, "We have American Gold Eagles.", seen.Blocks[1].Payload[turns.PayloadKeyText])
	require.Equal(t, turns.RoleUser, seen.Blocks[2].Role)
	require.Equal(t, "Tell me more about the first one", seen.Blocks[2].Payload[turns.PayloadKeyText])
}

func TestRuntimeInferenceStartsFreshWhenNoHistoryExists(t *testing.T) {
	ctx := context.Background()
	store := &fakeTurnStore{}
	recorder := &recordingHistoryEngine{}
	engine := NewEngine(WithChunkDelay(time.Millisecond), WithTurnStore(store))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-no-history", PromptRequest{
		Prompt: "First message",
		Runtime: &infruntime.ComposedRuntime{
			Engine: recorder,
		},
	})

	require.NoError(t, hub.Submit(ctx, sessionstream.SessionId("chat-empty"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-no-history"}))
	require.NoError(t, engine.WaitIdle(ctx, sessionstream.SessionId("chat-empty")))

	seen := recorder.seen
	require.NotNil(t, seen)
	require.Equal(t, "chat-empty", recorder.sessionID)
	require.Len(t, seen.Blocks, 1)
	require.Equal(t, turns.RoleUser, seen.Blocks[0].Role)
	require.Equal(t, "First message", seen.Blocks[0].Payload[turns.PayloadKeyText])
}

func TestRuntimeInferenceStopsWhenHistoryLoadFails(t *testing.T) {
	ctx := context.Background()
	store := &fakeTurnStore{err: errors.New("database unavailable")}
	recorder := &recordingHistoryEngine{}
	backendEvents := map[string]map[string]any{}
	engine := NewEngine(WithChunkDelay(time.Millisecond), WithTurnStore(store), WithHooks(Hooks{OnBackendEvent: func(_, eventName string, payload map[string]any) {
		backendEvents[eventName] = payload
	}}))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-history-error", PromptRequest{
		Prompt: "Follow up",
		Runtime: &infruntime.ComposedRuntime{
			Engine: recorder,
		},
	})

	require.NoError(t, hub.Submit(ctx, sessionstream.SessionId("chat-load-error"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-history-error"}))
	require.NoError(t, engine.WaitIdle(ctx, sessionstream.SessionId("chat-load-error")))
	require.Nil(t, recorder.seen)

	failed := backendEvents[EventChatRunFailed]
	require.NotNil(t, failed)
	require.Equal(t, "failed", failed["status"])
	require.Contains(t, failed["error"], "load conversation history")
	require.Contains(t, failed["error"], "database unavailable")
}

func TestRuntimeInferenceStopsWhenHistoryDecodeFails(t *testing.T) {
	ctx := context.Background()
	store := &fakeTurnStore{snapshot: &chatstore.TurnSnapshot{
		ConvID:    "chat-corrupt",
		SessionID: "chat-corrupt",
		TurnID:    "turn-corrupt",
		Phase:     "final",
		Payload:   "not: [valid",
	}}
	recorder := &recordingHistoryEngine{}
	backendEvents := map[string]map[string]any{}
	engine := NewEngine(WithChunkDelay(time.Millisecond), WithTurnStore(store), WithHooks(Hooks{OnBackendEvent: func(_, eventName string, payload map[string]any) {
		backendEvents[eventName] = payload
	}}))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-history-decode-error", PromptRequest{
		Prompt: "Follow up",
		Runtime: &infruntime.ComposedRuntime{
			Engine: recorder,
		},
	})

	require.NoError(t, hub.Submit(ctx, sessionstream.SessionId("chat-corrupt"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-history-decode-error"}))
	require.NoError(t, engine.WaitIdle(ctx, sessionstream.SessionId("chat-corrupt")))
	require.Nil(t, recorder.seen)

	failed := backendEvents[EventChatRunFailed]
	require.NotNil(t, failed)
	require.Equal(t, "failed", failed["status"])
	require.Contains(t, failed["error"], "decode conversation history")
}

func TestRuntimeInterleavedTextToolTextUsesDistinctTextSegments(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-interleaved", PromptRequest{
		Prompt: "Use tools and explain",
		Runtime: &infruntime.ComposedRuntime{
			Engine: interleavedTextToolEngine{},
		},
	})

	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-interleaved"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-interleaved"}))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-interleaved")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-interleaved"))
	require.NoError(t, err)

	ids := map[string]*chatappv1.ChatMessageEntity{}
	for _, entity := range snap.Entities {
		if entity.Kind != TimelineEntityChatMessage {
			continue
		}
		payloadMsg := entity.Payload.(*chatappv1.ChatMessageEntity)
		ids[entity.Id] = payloadMsg
	}
	require.Contains(t, ids, "chat-msg-1:text:1")
	require.Contains(t, ids, "chat-msg-1:text:2")
	require.Equal(t, "first text", ids["chat-msg-1:text:1"].GetContent())
	require.Equal(t, "final text", ids["chat-msg-1:text:2"].GetContent())
	require.Equal(t, int32(1), ids["chat-msg-1:text:1"].GetSegment())
	require.Equal(t, int32(2), ids["chat-msg-1:text:2"].GetSegment())
	require.Equal(t, "text", ids["chat-msg-1:text:1"].GetSegmentType())
	require.Equal(t, "text", ids["chat-msg-1:text:2"].GetSegmentType())
	require.True(t, ids["chat-msg-1:text:2"].GetFinal())
}

func TestRuntimeErrorAfterPartialStopsActiveTextSegment(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-partial-error", PromptRequest{
		Prompt: "Fail after a partial",
		Runtime: &infruntime.ComposedRuntime{
			Engine: partialThenErrorEngine{},
		},
	})

	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-partial-error"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-partial-error"}))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-partial-error")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-partial-error"))
	require.NoError(t, err)

	ids := map[string]*chatappv1.ChatMessageEntity{}
	for _, entity := range snap.Entities {
		if entity.Kind != TimelineEntityChatMessage {
			continue
		}
		ids[entity.Id] = entity.Payload.(*chatappv1.ChatMessageEntity)
	}

	textSegment := ids["chat-msg-1:text:1"]
	require.NotNil(t, textSegment)
	require.Equal(t, "partial text", textSegment.GetContent())
	require.Equal(t, "streaming", textSegment.GetStatus())
	require.True(t, textSegment.GetStreaming())
	require.Equal(t, "chat-msg-1", textSegment.GetParentMessageId())
	require.Equal(t, int32(1), textSegment.GetSegment())
	require.Equal(t, "text", textSegment.GetSegmentType())
	require.False(t, textSegment.GetFinal())
	require.NotContains(t, ids, "chat-msg-1")
}

func TestRuntimeInterruptAfterPartialStopsActiveTextSegment(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-partial-interrupt", PromptRequest{
		Prompt: "Stop after a partial",
		Runtime: &infruntime.ComposedRuntime{
			Engine: partialThenInterruptEngine{},
		},
	})

	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-partial-interrupt"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-partial-interrupt"}))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-partial-interrupt")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-partial-interrupt"))
	require.NoError(t, err)

	ids := map[string]*chatappv1.ChatMessageEntity{}
	for _, entity := range snap.Entities {
		if entity.Kind != TimelineEntityChatMessage {
			continue
		}
		ids[entity.Id] = entity.Payload.(*chatappv1.ChatMessageEntity)
	}

	textSegment := ids["chat-msg-1:text:1"]
	require.NotNil(t, textSegment)
	require.Equal(t, "partial before stop", textSegment.GetContent())
	require.Equal(t, "stopped", textSegment.GetStatus())
	require.False(t, textSegment.GetStreaming())
	require.True(t, textSegment.GetFinal())
	require.NotContains(t, ids, "chat-msg-1")
}

func TestRuntimeErrorAfterClosedTextSegmentDoesNotDuplicateSegmentContent(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-boundary-error", PromptRequest{
		Prompt: "Fail after a boundary",
		Runtime: &infruntime.ComposedRuntime{
			Engine: boundaryThenErrorEngine{},
		},
	})

	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-boundary-error"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-boundary-error"}))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-boundary-error")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-boundary-error"))
	require.NoError(t, err)

	ids := map[string]*chatappv1.ChatMessageEntity{}
	for _, entity := range snap.Entities {
		if entity.Kind != TimelineEntityChatMessage {
			continue
		}
		ids[entity.Id] = entity.Payload.(*chatappv1.ChatMessageEntity)
	}

	finishedSegment := ids["chat-msg-1:text:1"]
	require.NotNil(t, finishedSegment)
	require.Equal(t, "first text", finishedSegment.GetContent())
	require.Equal(t, "finished", finishedSegment.GetStatus())
	require.False(t, finishedSegment.GetStreaming())

	require.NotContains(t, ids, "chat-msg-1")
}

func TestRuntimeMaxIterationsErrorPublishesWarningMessage(t *testing.T) {
	engine := NewEngine(WithChunkDelay(time.Millisecond))
	hub := newTestHub(t, engine)
	engine.setPendingRequest("request-max-iterations", PromptRequest{
		Prompt: "Run many tools",
		Runtime: &infruntime.ComposedRuntime{
			Engine: maxIterationsErrorEngine{},
		},
	})

	require.NoError(t, hub.Submit(context.Background(), sessionstream.SessionId("chat-max-iterations"), CommandStartInference, &chatappv1.StartInferenceCommand{RequestId: "request-max-iterations"}))
	require.NoError(t, engine.WaitIdle(context.Background(), sessionstream.SessionId("chat-max-iterations")))

	snap, err := hub.Snapshot(context.Background(), sessionstream.SessionId("chat-max-iterations"))
	require.NoError(t, err)

	var warning *chatappv1.ChatMessageEntity
	var assistant *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		payloadMsg := entity.Payload.(*chatappv1.ChatMessageEntity)
		switch payloadMsg.GetRole() {
		case "warning":
			warning = payloadMsg
		case "assistant":
			assistant = payloadMsg
		}
	}
	require.NotNil(t, warning)
	require.Contains(t, warning.GetContent(), "max iterations (20) reached")
	require.Contains(t, warning.GetContent(), "answer may be incomplete")
	require.Equal(t, "finished", warning.GetStatus())
	require.False(t, warning.GetStreaming())
	require.Nil(t, assistant)
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
	require.GreaterOrEqual(t, len(snap.Entities), 1)
	var assistant *chatappv1.ChatMessageEntity
	for _, entity := range snap.Entities {
		payloadMsg := entity.Payload.(*chatappv1.ChatMessageEntity)
		if payloadMsg.GetRole() == "assistant" {
			assistant = payloadMsg
		}
	}
	if assistant != nil {
		require.Equal(t, "stopped", assistant.GetStatus())
		require.Equal(t, false, assistant.GetStreaming())
	}
}

type interleavedTextToolEngine struct{}

func (interleavedTextToolEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	publishCanonicalTextSegment(ctx, 1, "first text")
	meta := gepevents.EventMetadata{SessionID: "sid"}
	corr := gepevents.Correlation{SessionID: "sid", ToolCallID: "call-1", CorrelationKey: "tool:call-1"}
	gepevents.PublishEventToContext(ctx, gepevents.NewToolCallRequestedEvent(meta, corr, "call-1", "lookup", `{"q":"x"}`))
	gepevents.PublishEventToContext(ctx, gepevents.NewToolResultReadyEvent(meta, corr, "call-1", "lookup", `{"ok":true}`, "finished"))
	publishCanonicalTextSegment(ctx, 2, "final text")
	return t, nil
}

type partialThenErrorEngine struct{}

func (partialThenErrorEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	meta := gepevents.EventMetadata{SessionID: "sid"}
	corr := testTextCorrelation(1)
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, "partial text", "partial text", 1))
	return t, errors.New("provider failed after partial")
}

type partialThenInterruptEngine struct{}

func (partialThenInterruptEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	meta := gepevents.EventMetadata{SessionID: "sid"}
	corr := testTextCorrelation(1)
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, "partial before stop", "partial before stop", 1))
	gepevents.PublishEventToContext(ctx, gepevents.NewInterruptEvent(meta, ""))
	return t, nil
}

type boundaryThenErrorEngine struct{}

func (boundaryThenErrorEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	publishCanonicalTextSegment(ctx, 1, "first text")
	return t, errors.New("provider failed after boundary")
}

type maxIterationsErrorEngine struct{}

func (maxIterationsErrorEngine) RunInference(context.Context, *turns.Turn) (*turns.Turn, error) {
	return nil, errors.New("max iterations (20) reached")
}

type recordingHistoryEngine struct {
	seen      *turns.Turn
	sessionID string
}

func (e *recordingHistoryEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if t != nil {
		e.seen = t.Clone()
	}
	e.sessionID = gepsession.SessionIDFromContext(ctx)
	turns.AppendBlock(t, turns.NewAssistantTextBlock("ok"))
	publishCanonicalTextSegment(ctx, 1, "ok")
	return t, nil
}

type fakeTurnStore struct {
	snapshot *chatstore.TurnSnapshot
	err      error
}

func (s *fakeTurnStore) Save(context.Context, string, string, string, string, int64, string, chatstore.TurnSaveOptions) error {
	return nil
}

func (s *fakeTurnStore) List(context.Context, chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	if s.snapshot == nil {
		return nil, s.err
	}
	return []chatstore.TurnSnapshot{*s.snapshot}, s.err
}

func (s *fakeTurnStore) LoadLatestTurn(context.Context, string, string) (*chatstore.TurnSnapshot, error) {
	return s.snapshot, s.err
}

func (s *fakeTurnStore) Close() error { return nil }

type testPlugin struct{}

func (testPlugin) RegisterSchemas(*sessionstream.SchemaRegistry) error { return nil }

func (testPlugin) HandleRuntimeEvent(context.Context, RuntimeEventContext, gepevents.Event) (bool, error) {
	return false, nil
}

func (testPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	if ev.Name != EventChatTextSegmentFinished {
		return nil, false, nil
	}
	payload, ok := ev.Payload.(*chatappv1.ChatTextSegmentFinished)
	if !ok || payload == nil {
		return nil, true, nil
	}
	return []sessionstream.UIEvent{{Name: "FeatureSawFinished", Payload: &chatappv1.ChatTextSegmentFinished{MessageId: payload.GetMessageId()}}}, true, nil
}

func publishCanonicalTextSegment(ctx context.Context, segment int32, text string) {
	meta := gepevents.EventMetadata{SessionID: "sid"}
	corr := testTextCorrelation(segment)
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, text, text, 1))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, text, "stop"))
}

func testTextCorrelation(segment int32) gepevents.Correlation {
	return gepevents.Correlation{SessionID: "sid", SegmentID: fmt.Sprintf("segment-%d", segment), SegmentIndex: segment, SegmentType: "text", StreamKind: "content", CorrelationKey: fmt.Sprintf("text:%d", segment)}
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
	return newTestHubWithPlugins(t, engine)
}

func newTestHubWithPlugins(t *testing.T, engine *Engine, features ...ChatPlugin) *sessionstream.Hub {
	t.Helper()
	reg := sessionstream.NewSchemaRegistry()
	require.NoError(t, RegisterSchemas(reg, features...))
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
