package chatapp

import (
	"context"
	"testing"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestCompactChatTextDeltaTransformer(t *testing.T) {
	patch := &chatappv1.ChatTextPatch{
		MessageId:   "message-1:text:1",
		Role:        "assistant",
		StreamId:    "stream-1",
		Sequence:    12,
		Offset:      99,
		Text:        "hello — world",
		Mode:        chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND,
		Status:      "streaming",
		Final:       true,
		Prompt:      "secret prompt should not be copied",
		Correlation: &chatappv1.CorrelationInfo{RunId: "run-1"},
	}
	input := []sessionstream.UIEvent{
		{Name: EventChatTextSegmentStarted, Payload: &chatappv1.ChatTextSegmentStarted{MessageId: patch.GetMessageId()}},
		{Name: EventChatTextPatch, Payload: patch},
		{Name: EventChatTextSegmentFinished, Payload: &chatappv1.ChatTextSegmentFinished{MessageId: patch.GetMessageId(), Final: true}},
	}

	got, err := CompactChatTextDeltaTransformer().TransformUIEvents(context.Background(), sessionstream.Event{Name: EventChatTextPatch}, input)
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, input[0], got[0])
	require.Equal(t, input[2], got[2])
	require.Equal(t, UIEventChatTextDelta, got[1].Name)
	delta, ok := got[1].Payload.(*chatappv1.ChatTextDelta)
	require.True(t, ok)
	require.Equal(t, patch.GetMessageId(), delta.GetMessageId())
	require.Equal(t, patch.GetText(), delta.GetText())
	require.Equal(t, patch.GetMode(), delta.GetMode())
	require.True(t, delta.GetFinal())
}

func TestCompactChatTextDeltaTransformerLeavesCanonicalPatchUnchanged(t *testing.T) {
	patch := &chatappv1.ChatTextPatch{MessageId: "message-1", Text: "hello"}
	original := patch.String()

	_, err := CompactChatTextDeltaTransformer().TransformUIEvents(context.Background(), sessionstream.Event{}, []sessionstream.UIEvent{{Name: EventChatTextPatch, Payload: patch}})
	require.NoError(t, err)
	require.Equal(t, original, patch.String())
}

func TestCompactChatReasoningAndToolArgumentDeltas(t *testing.T) {
	reasoning := &chatappv1.ChatReasoningPatch{
		MessageId: "reason-1", ParentMessageId: "message-1", Role: "thinking", StreamId: "stream-1",
		Sequence: 7, Offset: 8, Text: "lookup inventory", Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND,
		Status: "streaming", Source: "provider", Correlation: &chatappv1.CorrelationInfo{RunId: "run-1"},
	}
	arguments := &chatappv1.ChatToolArgumentsPatch{
		MessageId: "message-1", ToolCallId: "call-1", ToolName: "sql_query", StreamId: "stream-2",
		Sequence: 9, Offset: 10, Arguments: `{"sql":"select 1"}`, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND,
		Status: "streaming_args", Correlation: &chatappv1.CorrelationInfo{RunId: "run-1"},
	}
	got, err := CompactChatTextDeltaTransformer().TransformUIEvents(context.Background(), sessionstream.Event{}, []sessionstream.UIEvent{
		{Name: EventChatReasoningPatch, Payload: reasoning},
		{Name: EventChatToolArgumentsPatch, Payload: arguments},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, UIEventChatReasoningDelta, got[0].Name)
	require.Equal(t, UIEventChatToolArgumentsDelta, got[1].Name)
	require.Equal(t, &chatappv1.ChatReasoningDelta{MessageId: "reason-1", ParentMessageId: "message-1", Text: "lookup inventory", Mode: reasoning.GetMode()}, got[0].Payload)
	require.Equal(t, &chatappv1.ChatToolArgumentsDelta{MessageId: "message-1", ToolCallId: "call-1", ToolName: "sql_query", Arguments: `{"sql":"select 1"}`, Mode: arguments.GetMode()}, got[1].Payload)
	require.Equal(t, "thinking", reasoning.GetRole())
	require.Equal(t, "streaming_args", arguments.GetStatus())
}

func TestEngineUIProjectionAppliesConfiguredTransformers(t *testing.T) {
	engine := NewEngine(WithUIEventTransformers(CompactChatTextDeltaTransformer()))
	patch := &chatappv1.ChatTextPatch{MessageId: "message-1", Text: "hello", Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND}

	got, err := engine.uiProjection(context.Background(), sessionstream.Event{Name: EventChatTextPatch, Payload: patch}, nil, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, UIEventChatTextDelta, got[0].Name)
	require.IsType(t, &chatappv1.ChatTextDelta{}, got[0].Payload)
}
