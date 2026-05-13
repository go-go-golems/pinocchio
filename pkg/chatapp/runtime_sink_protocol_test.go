package chatapp

import (
	"context"
	"errors"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

type recordingEventPublisher struct {
	events []sessionstream.Event
}

func (p *recordingEventPublisher) Publish(_ context.Context, ev sessionstream.Event) error {
	p.events = append(p.events, ev)
	return nil
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
			require.Equal(t, tt.wantNames, runtimeSinkEventNames(pub.events))
			if tt.check != nil {
				tt.check(t, pub.events, sink)
			}
		})
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
