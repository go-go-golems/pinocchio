package chatapp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

type recordingEventPublisher struct {
	mu     sync.Mutex
	events []sessionstream.Event
}

func (p *recordingEventPublisher) Publish(_ context.Context, ev sessionstream.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
	return nil
}

func (p *recordingEventPublisher) Events() []sessionstream.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]sessionstream.Event(nil), p.events...)
}

func TestRuntimeEventSinkProtocolMatrix(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	textCorr := runtimeSinkTextCorrelation()
	providerCorr := runtimeSinkProviderCorrelation()

	tests := []struct {
		name      string
		events    []gepevents.Event
		wantNames []string
		check     func(t *testing.T, published []sessionstream.Event, sink *runtimeEventSink)
	}{
		{
			name: "RUNTIME-01 active text plus error closes text and fails run",
			events: []gepevents.Event{
				gepevents.NewTextSegmentStartedEvent(metadata, textCorr, "assistant"),
				gepevents.NewTextDeltaEvent(metadata, textCorr, "partial", "partial", 1),
				gepevents.NewErrorEvent(metadata, errors.New("stream exploded")),
			},
			wantNames: []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunFailed},
			check: func(t *testing.T, published []sessionstream.Event, sink *runtimeEventSink) {
				t.Helper()
				finished := published[2].Payload.(*chatappv1.ChatTextSegmentFinished)
				require.Equal(t, "partial", finished.GetContent())
				require.Equal(t, "failed", finished.GetStatus())
				require.Equal(t, "error", finished.GetFinishReason())
				require.False(t, finished.GetStreaming())
				require.True(t, finished.GetFinal())
				requireRuntimeSinkTextCorrelation(t, finished.GetCorrelation())

				failed := published[3].Payload.(*chatappv1.ChatRunFailed)
				require.Equal(t, "message-1", failed.GetMessageId())
				require.Equal(t, "failed", failed.GetStatus())
				require.Equal(t, "stream exploded", failed.GetError())
				require.True(t, sink.IsTerminal())
			},
		},
		{
			name: "RUNTIME-02 active text plus interrupt closes text and stops run",
			events: []gepevents.Event{
				gepevents.NewTextSegmentStartedEvent(metadata, textCorr, "assistant"),
				gepevents.NewTextDeltaEvent(metadata, textCorr, "partial", "partial", 1),
				gepevents.NewInterruptEvent(metadata, ""),
			},
			wantNames: []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunStopped},
			check: func(t *testing.T, published []sessionstream.Event, sink *runtimeEventSink) {
				t.Helper()
				finished := published[2].Payload.(*chatappv1.ChatTextSegmentFinished)
				require.Equal(t, "partial", finished.GetContent())
				require.Equal(t, "stopped", finished.GetStatus())
				require.Equal(t, "stopped", finished.GetFinishReason())

				stopped := published[3].Payload.(*chatappv1.ChatRunStopped)
				require.Equal(t, "message-1", stopped.GetMessageId())
				require.Equal(t, "stopped", stopped.GetStatus())
				require.True(t, sink.IsTerminal())
			},
		},
		{
			name: "RUNTIME-03 closed text is not rewritten by later error",
			events: []gepevents.Event{
				gepevents.NewTextSegmentStartedEvent(metadata, textCorr, "assistant"),
				gepevents.NewTextDeltaEvent(metadata, textCorr, "done", "done", 1),
				gepevents.NewTextSegmentFinishedEvent(metadata, textCorr, "done", "stop"),
				gepevents.NewErrorEvent(metadata, errors.New("late error")),
			},
			wantNames: []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunFailed},
			check: func(t *testing.T, published []sessionstream.Event, _ *runtimeEventSink) {
				t.Helper()
				finished := published[2].Payload.(*chatappv1.ChatTextSegmentFinished)
				require.Equal(t, "done", finished.GetContent())
				require.Equal(t, "finished", finished.GetStatus())
				require.Equal(t, "stop", finished.GetFinishReason())
				requireRuntimeSinkEventCount(t, published, EventChatTextSegmentFinished, 1)
			},
		},
		{
			name: "RUNTIME-04 error without active text does not manufacture text finish",
			events: []gepevents.Event{
				gepevents.NewErrorEvent(metadata, errors.New("setup failed")),
			},
			wantNames: []string{EventChatRunFailed},
			check: func(t *testing.T, published []sessionstream.Event, sink *runtimeEventSink) {
				t.Helper()
				requireRuntimeSinkEventCount(t, published, EventChatTextSegmentFinished, 0)
				failed := published[0].Payload.(*chatappv1.ChatRunFailed)
				require.Equal(t, "setup failed", failed.GetError())
				require.True(t, sink.IsTerminal())
			},
		},
		{
			name: "RUNTIME-05 provider finish after text finish does not rewrite text",
			events: []gepevents.Event{
				gepevents.NewTextSegmentStartedEvent(metadata, textCorr, "assistant"),
				gepevents.NewTextDeltaEvent(metadata, textCorr, "done", "done", 1),
				gepevents.NewTextSegmentFinishedEvent(metadata, textCorr, "done", "stop"),
				gepevents.NewProviderCallFinishedEvent(metadata, providerCorr, "stop", "completed", nil, nil, false),
			},
			wantNames: []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextSegmentFinished, EventChatProviderCallFinished},
			check: func(t *testing.T, published []sessionstream.Event, _ *runtimeEventSink) {
				t.Helper()
				requireRuntimeSinkEventCount(t, published, EventChatTextSegmentFinished, 1)
				providerFinished := published[3].Payload.(*chatappv1.ChatProviderCallFinished)
				require.Equal(t, "provider-call-1", providerFinished.GetCorrelation().GetProviderCallId())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &recordingEventPublisher{}
			sink := newRuntimeSinkForProtocolTest(pub)
			for _, ev := range tt.events {
				require.NoError(t, sink.PublishEvent(ev))
			}
			published := pub.Events()
			require.Equal(t, tt.wantNames, runtimeSinkEventNames(published))
			if tt.check != nil {
				tt.check(t, published, sink)
			}
		})
	}
}

func TestRuntimeEventSinkBatchesTextPatchesAfterImmediateFirst(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	corr := runtimeSinkTextCorrelation()
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.batchInterval = 20 * time.Millisecond

	require.NoError(t, sink.PublishEvent(gepevents.NewTextSegmentStartedEvent(metadata, corr, "assistant")))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, "The", "The", 1)))
	require.Equal(t, []string{EventChatTextSegmentStarted, EventChatTextPatch}, runtimeSinkEventNames(pub.Events()))

	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, " ", "The ", 2)))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, "Morgan", "The Morgan", 3)))
	require.Eventually(t, func() bool { return len(pub.Events()) == 3 }, time.Second, 5*time.Millisecond)

	patch := pub.Events()[2].Payload.(*chatappv1.ChatTextPatch)
	require.Equal(t, " Morgan", patch.GetText())
	require.Equal(t, uint64(3), patch.GetSequence())
	require.Equal(t, uint64(3), patch.GetOffset())
}

func TestRuntimeEventSinkBatchesReasoningPatchesAfterImmediateFirst(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	corr := runtimeSinkProviderCorrelation()
	corr.SegmentID = "reasoning-1"
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.engine = NewEngine(WithPlugins(runtimeSinkReasoningPlugin{}))
	sink.batchInterval = 20 * time.Millisecond

	require.NoError(t, sink.PublishEvent(gepevents.NewReasoningSegmentStartedEvent(metadata, corr, "thinking")))
	require.NoError(t, sink.PublishEvent(gepevents.NewReasoningDeltaEvent(metadata, corr, "Look", "Look", 1)))
	require.Equal(t, []string{EventChatReasoningSegmentStarted, EventChatReasoningPatch}, runtimeSinkEventNames(pub.Events()))

	require.NoError(t, sink.PublishEvent(gepevents.NewReasoningDeltaEvent(metadata, corr, " up", "Look up", 2)))
	require.NoError(t, sink.PublishEvent(gepevents.NewReasoningDeltaEvent(metadata, corr, " stock", "Look up stock", 3)))
	require.Eventually(t, func() bool { return len(pub.Events()) == 3 }, time.Second, 5*time.Millisecond)

	patch := pub.Events()[2].Payload.(*chatappv1.ChatReasoningPatch)
	require.Equal(t, " up stock", patch.GetText())
	require.Equal(t, uint64(3), patch.GetSequence())
	require.Equal(t, uint64(4), patch.GetOffset())
}

func TestRuntimeEventSinkBatchesToolArgumentPatchesAndFlushesBeforeRequest(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	corr := runtimeSinkProviderCorrelation()
	corr.ToolCallID = "call-1"
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.engine = NewEngine(WithPlugins(runtimeSinkToolPlugin{}))
	sink.batchInterval = time.Hour

	require.NoError(t, sink.PublishEvent(gepevents.NewToolCallStartedEvent(metadata, corr, "call-1", "sql_query")))
	require.NoError(t, sink.PublishEvent(gepevents.NewToolCallArgumentsDeltaEvent(metadata, corr, "call-1", `{`, `{`, 1)))
	require.NoError(t, sink.PublishEvent(gepevents.NewToolCallArgumentsDeltaEvent(metadata, corr, "call-1", `"sql":`, `{"sql":`, 2)))
	require.NoError(t, sink.PublishEvent(gepevents.NewToolCallArgumentsDeltaEvent(metadata, corr, "call-1", `"select 1"}`, `{"sql":"select 1"}`, 3)))
	require.NoError(t, sink.PublishEvent(gepevents.NewToolCallRequestedEvent(metadata, corr, "call-1", "sql_query", `{"sql":"select 1"}`)))

	published := pub.Events()
	require.Equal(t, []string{EventChatToolCallStarted, EventChatToolArgumentsPatch, EventChatToolArgumentsPatch, EventChatToolCallRequested}, runtimeSinkEventNames(published))
	patch := published[2].Payload.(*chatappv1.ChatToolArgumentsPatch)
	require.Equal(t, `"sql":"select 1"}`, patch.GetArguments())
	require.Equal(t, uint64(3), patch.GetSequence())
	require.Equal(t, uint64(1), patch.GetOffset())
}

func TestRuntimeEventSinkFlushesPendingPatchBeforeCrossStreamEvent(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	reasoningCorr := runtimeSinkProviderCorrelation()
	reasoningCorr.SegmentID = "reasoning-1"
	toolCorr := runtimeSinkProviderCorrelation()
	toolCorr.ToolCallID = "call-1"
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.engine = NewEngine(WithPlugins(runtimeSinkReasoningPlugin{}, runtimeSinkToolPlugin{}))
	sink.batchInterval = time.Hour

	require.NoError(t, sink.PublishEvent(gepevents.NewReasoningDeltaEvent(metadata, reasoningCorr, "First", "First", 1)))
	require.NoError(t, sink.PublishEvent(gepevents.NewReasoningDeltaEvent(metadata, reasoningCorr, " pending", "First pending", 2)))
	require.NoError(t, sink.PublishEvent(gepevents.NewToolCallArgumentsDeltaEvent(metadata, toolCorr, "call-1", `{`, `{`, 1)))

	published := pub.Events()
	require.Equal(t, []string{EventChatReasoningPatch, EventChatReasoningPatch, EventChatToolArgumentsPatch}, runtimeSinkEventNames(published))
	require.Equal(t, " pending", published[1].Payload.(*chatappv1.ChatReasoningPatch).GetText())
}

func TestRuntimeEventSinkDrainPreventsPendingPatchAfterFallbackFinish(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	corr := runtimeSinkTextCorrelation()
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.batchInterval = 25 * time.Millisecond

	require.NoError(t, sink.PublishEvent(gepevents.NewTextSegmentStartedEvent(metadata, corr, "assistant")))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, "Hello", "Hello", 1)))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, " world", "Hello world", 2)))

	require.NoError(t, sink.drainStreamPatches())
	require.NoError(t, sink.finishActiveTextSegment("finished", "stop", ""))
	require.NoError(t, sink.engine.publish(context.Background(), sink.sessionID, sink.pub, EventChatRunFinished, &chatappv1.ChatRunFinished{MessageId: sink.messageID, Status: "finished"}))

	require.Equal(t, []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunFinished}, runtimeSinkEventNames(pub.Events()))
	time.Sleep(2 * sink.batchInterval)
	require.Equal(t, []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunFinished}, runtimeSinkEventNames(pub.Events()))
}

func TestRuntimeEventSinkDrainPreventsPendingPatchAfterFallbackStop(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	corr := runtimeSinkTextCorrelation()
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.batchInterval = 25 * time.Millisecond

	require.NoError(t, sink.PublishEvent(gepevents.NewTextSegmentStartedEvent(metadata, corr, "assistant")))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, "Partial", "Partial", 1)))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, " answer", "Partial answer", 2)))

	require.NoError(t, sink.drainStreamPatches())
	require.NoError(t, sink.finishActiveTextSegment("stopped", "stopped", ""))
	require.NoError(t, sink.engine.publish(context.Background(), sink.sessionID, sink.pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: sink.messageID, Status: "stopped"}))

	require.Equal(t, []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunStopped}, runtimeSinkEventNames(pub.Events()))
	time.Sleep(2 * sink.batchInterval)
	require.Equal(t, []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextPatch, EventChatTextSegmentFinished, EventChatRunStopped}, runtimeSinkEventNames(pub.Events()))
}

func TestRuntimeEventSinkFlushesPendingPatchBeforeTextFinish(t *testing.T) {
	metadata := gepevents.EventMetadata{SessionID: "session-1", InferenceID: "inference-1", TurnID: "turn-1"}
	corr := runtimeSinkTextCorrelation()
	pub := &recordingEventPublisher{}
	sink := newRuntimeSinkForProtocolTest(pub)
	sink.batchInterval = time.Hour

	require.NoError(t, sink.PublishEvent(gepevents.NewTextSegmentStartedEvent(metadata, corr, "assistant")))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, "Hello", "Hello", 1)))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextDeltaEvent(metadata, corr, " world", "Hello world", 2)))
	require.NoError(t, sink.PublishEvent(gepevents.NewTextSegmentFinishedEvent(metadata, corr, "Hello world", "stop")))

	published := pub.Events()
	require.Equal(t, []string{EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextPatch, EventChatTextSegmentFinished}, runtimeSinkEventNames(published))
	require.Equal(t, " world", published[2].Payload.(*chatappv1.ChatTextPatch).GetText())
	require.Equal(t, "Hello world", published[3].Payload.(*chatappv1.ChatTextSegmentFinished).GetText())
}

type runtimeSinkReasoningPlugin struct{}

func (runtimeSinkReasoningPlugin) RegisterSchemas(*sessionstream.SchemaRegistry) error { return nil }
func (runtimeSinkReasoningPlugin) ProjectUI(context.Context, sessionstream.Event, *sessionstream.Session, sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	return nil, false, nil
}
func (runtimeSinkReasoningPlugin) ProjectTimeline(context.Context, sessionstream.Event, *sessionstream.Session, sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	return nil, false, nil
}
func (runtimeSinkReasoningPlugin) HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (bool, error) {
	switch ev := event.(type) {
	case *gepevents.EventReasoningSegmentStarted:
		return true, runtime.Publish(ctx, EventChatReasoningSegmentStarted, &chatappv1.ChatReasoningSegmentStarted{MessageId: "reason-1", ParentMessageId: runtime.MessageID})
	case *gepevents.EventReasoningDelta:
		return true, runtime.Publish(ctx, EventChatReasoningPatch, &chatappv1.ChatReasoningPatch{MessageId: "reason-1", ParentMessageId: runtime.MessageID, StreamId: "reason-1", Sequence: Uint64FromInt64(ev.Sequence), Offset: PatchOffset(ev.Text, ev.Delta), Text: ev.Delta, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Correlation: CorrelationInfoFromEvent(ev)})
	default:
		return false, nil
	}
}

type runtimeSinkToolPlugin struct{}

func (runtimeSinkToolPlugin) RegisterSchemas(*sessionstream.SchemaRegistry) error { return nil }
func (runtimeSinkToolPlugin) ProjectUI(context.Context, sessionstream.Event, *sessionstream.Session, sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	return nil, false, nil
}
func (runtimeSinkToolPlugin) ProjectTimeline(context.Context, sessionstream.Event, *sessionstream.Session, sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	return nil, false, nil
}
func (runtimeSinkToolPlugin) HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (bool, error) {
	switch ev := event.(type) {
	case *gepevents.EventToolCallStarted:
		return true, runtime.Publish(ctx, EventChatToolCallStarted, &chatappv1.ChatToolCallStarted{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName})
	case *gepevents.EventToolCallArgumentsDelta:
		return true, runtime.Publish(ctx, EventChatToolArgumentsPatch, &chatappv1.ChatToolArgumentsPatch{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: "sql_query", StreamId: ev.ToolCallID, Sequence: Uint64FromInt64(ev.Sequence), Offset: PatchOffset(ev.Arguments, ev.Delta), Arguments: ev.Delta, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Correlation: CorrelationInfoFromEvent(ev)})
	case *gepevents.EventToolCallRequested:
		return true, runtime.Publish(ctx, EventChatToolCallRequested, &chatappv1.ChatToolCallRequested{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName, Input: ev.Input})
	default:
		return false, nil
	}
}

func newRuntimeSinkForProtocolTest(pub sessionstream.EventPublisher) *runtimeEventSink {
	return &runtimeEventSink{
		publishCtx: context.Background(),
		sessionID:  sessionstream.SessionId("session-1"),
		messageID:  "message-1",
		prompt:     "prompt",
		pub:        pub,
		engine:     NewEngine(),
	}
}

func runtimeSinkEventNames(events []sessionstream.Event) []string {
	names := make([]string, 0, len(events))
	for _, ev := range events {
		names = append(names, ev.Name)
	}
	return names
}

func requireRuntimeSinkEventCount(t *testing.T, events []sessionstream.Event, name string, want int) {
	t.Helper()
	got := 0
	for _, ev := range events {
		if ev.Name == name {
			got++
		}
	}
	require.Equal(t, want, got, "event count for %s", name)
}

func runtimeSinkProviderCorrelation() gepevents.Correlation {
	return gepevents.Correlation{
		SessionID:      "session-1",
		RunID:          "message-1",
		TurnID:         "turn-1",
		ProviderCallID: "provider-call-1",
	}
}

func runtimeSinkTextCorrelation() gepevents.Correlation {
	corr := runtimeSinkProviderCorrelation()
	corr.SegmentID = "segment-text-1"
	return corr
}

func requireRuntimeSinkTextCorrelation(t *testing.T, corr *chatappv1.CorrelationInfo) {
	t.Helper()
	require.NotNil(t, corr)
	require.Equal(t, "session-1", corr.GetSessionId())
	require.Equal(t, "message-1", corr.GetRunId())
	require.Equal(t, "turn-1", corr.GetTurnId())
	require.Equal(t, "provider-call-1", corr.GetProviderCallId())
	require.Equal(t, "segment-text-1", corr.GetSegmentId())
}
