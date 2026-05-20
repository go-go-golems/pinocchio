package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestChatAppUIFanoutMapsStreamingTextEvents(t *testing.T) {
	sender := &recordingTeaSender{}
	fanout, err := NewChatAppUIFanout(sender)
	require.NoError(t, err)

	err = fanout.PublishUI(context.Background(), "sid", 1, []sessionstream.UIEvent{
		{Name: "ChatTextSegmentStarted", Payload: &chatappv1.ChatTextSegmentStarted{MessageId: "assistant-1", Role: "assistant", Streaming: true}},
		{Name: "ChatTextPatch", Payload: &chatappv1.ChatTextPatch{MessageId: "assistant-1", Role: "assistant", Text: "hello", Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_SNAPSHOT}},
		{Name: "ChatTextSegmentFinished", Payload: &chatappv1.ChatTextSegmentFinished{MessageId: "assistant-1", Role: "assistant", Text: "hello", Content: "hello", Final: true}},
		{Name: "ChatRunFinished", Payload: &chatappv1.ChatRunFinished{MessageId: "assistant-1", Status: "finished"}},
	})
	require.NoError(t, err)

	created := requireMsgType[timeline.UIEntityCreated](t, sender.msgs, 0)
	require.Equal(t, "assistant-1", created.ID.LocalID)
	require.Equal(t, "assistant", created.Props["role"])
	require.Equal(t, "hello", created.Props["text"])

	updated := requireMsgType[timeline.UIEntityUpdated](t, sender.msgs, 1)
	require.Equal(t, "assistant-1", updated.ID.LocalID)
	require.Equal(t, "hello", updated.Patch["text"])
	require.Equal(t, true, updated.Patch["streaming"])

	completed := requireMsgType[timeline.UIEntityCompleted](t, sender.msgs, 2)
	require.Equal(t, "assistant-1", completed.ID.LocalID)
	require.Equal(t, "hello", completed.Result["text"])

	finishedUpdate := requireMsgType[timeline.UIEntityUpdated](t, sender.msgs, 3)
	require.Equal(t, false, finishedUpdate.Patch["streaming"])
	requireMsgType[boba_chat.BackendFinishedMsg](t, sender.msgs, 4)
}

func TestChatAppUIFanoutWaitsForRunFinishedBeforeBackendFinished(t *testing.T) {
	sender := &recordingTeaSender{}
	fanout, err := NewChatAppUIFanout(sender)
	require.NoError(t, err)

	err = fanout.PublishUI(context.Background(), "sid", 1, []sessionstream.UIEvent{
		{Name: "ChatTextSegmentFinished", Payload: &chatappv1.ChatTextSegmentFinished{MessageId: "assistant-1", Role: "assistant", Text: "first", Content: "first", Final: true}},
	})
	require.NoError(t, err)
	for _, msg := range sender.msgs {
		_, isFinished := msg.(boba_chat.BackendFinishedMsg)
		require.False(t, isFinished, "did not expect BackendFinishedMsg before ChatRunFinished")
	}

	err = fanout.PublishUI(context.Background(), "sid", 2, []sessionstream.UIEvent{
		{Name: "ChatRunFinished", Payload: &chatappv1.ChatRunFinished{MessageId: "assistant-1", Status: "finished"}},
	})
	require.NoError(t, err)
	requireMsgType[boba_chat.BackendFinishedMsg](t, sender.msgs, len(sender.msgs)-1)
}

func TestChatAppUIFanoutAccumulatesAppendTextPatches(t *testing.T) {
	sender := &recordingTeaSender{}
	fanout, err := NewChatAppUIFanout(sender)
	require.NoError(t, err)

	err = fanout.PublishUI(context.Background(), "sid", 1, []sessionstream.UIEvent{
		{Name: "ChatTextPatch", Payload: &chatappv1.ChatTextPatch{MessageId: "assistant-1", Role: "assistant", Text: "hel", Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND}},
		{Name: "ChatTextPatch", Payload: &chatappv1.ChatTextPatch{MessageId: "assistant-1", Role: "assistant", Text: "lo", Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND}},
	})
	require.NoError(t, err)

	firstUpdate := requireMsgType[timeline.UIEntityUpdated](t, sender.msgs, 1)
	require.Equal(t, "hel", firstUpdate.Patch["text"])
	secondUpdate := requireMsgType[timeline.UIEntityUpdated](t, sender.msgs, 2)
	require.Equal(t, "hello", secondUpdate.Patch["text"])
}

func TestChatAppUIFanoutMapsReasoningEvents(t *testing.T) {
	sender := &recordingTeaSender{}
	fanout, err := NewChatAppUIFanout(sender)
	require.NoError(t, err)

	err = fanout.PublishUI(context.Background(), "sid", 1, []sessionstream.UIEvent{
		{Name: "ChatReasoningSegmentStarted", Payload: &chatappv1.ChatReasoningSegmentStarted{MessageId: "reason-1", ParentMessageId: "assistant-1", Role: "thinking", Streaming: true}},
		{Name: "ChatReasoningPatch", Payload: &chatappv1.ChatReasoningPatch{MessageId: "reason-1", ParentMessageId: "assistant-1", Text: "thinking"}},
		{Name: "ChatReasoningSegmentFinished", Payload: &chatappv1.ChatReasoningSegmentFinished{MessageId: "reason-1", ParentMessageId: "assistant-1", Text: "thinking", Content: "thinking"}},
	})
	require.NoError(t, err)

	created := requireMsgType[timeline.UIEntityCreated](t, sender.msgs, 0)
	require.Equal(t, "reason-1", created.ID.LocalID)
	require.Equal(t, "thinking", created.Props["role"])
	updated := requireMsgType[timeline.UIEntityUpdated](t, sender.msgs, 1)
	require.Equal(t, "thinking", updated.Patch["text"])
	completed := requireMsgType[timeline.UIEntityCompleted](t, sender.msgs, 3)
	require.Equal(t, "reason-1", completed.ID.LocalID)
}

func TestChatAppUIFanoutHydratesSnapshot(t *testing.T) {
	sender := &recordingTeaSender{}
	fanout, err := NewChatAppUIFanout(sender)
	require.NoError(t, err)

	err = fanout.HydrateSnapshot(sessionstream.Snapshot{SessionId: "sid", Entities: []sessionstream.TimelineEntity{
		{Kind: "ChatMessage", Id: "user-1", Payload: &chatappv1.ChatMessageEntity{MessageId: "user-1", Role: "user", Content: "hi", Status: "accepted"}},
		{Kind: "ChatMessage", Id: "assistant-1", Payload: &chatappv1.ChatMessageEntity{MessageId: "assistant-1", Role: "assistant", Content: "hello", Status: "finished"}},
	}})
	require.NoError(t, err)
	require.Len(t, sender.msgs, 4)
	userCreated := requireMsgType[timeline.UIEntityCreated](t, sender.msgs, 0)
	require.Equal(t, "user-1", userCreated.ID.LocalID)
	require.Equal(t, "user", userCreated.Props["role"])
	assistantCompleted := requireMsgType[timeline.UIEntityCompleted](t, sender.msgs, 3)
	require.Equal(t, "hello", assistantCompleted.Result["text"])
}

func TestChatAppUIFanoutIgnoresLiveUserEventsAndMapsFailureEvents(t *testing.T) {
	sender := &recordingTeaSender{}
	fanout, err := NewChatAppUIFanout(sender)
	require.NoError(t, err)

	err = fanout.PublishUI(context.Background(), "sid", 1, []sessionstream.UIEvent{
		{Name: "ChatUserMessageAccepted", Payload: &chatappv1.ChatUserMessageAccepted{MessageId: "user-1", Role: "user", Content: "hi"}},
		{Name: "ChatRunFailed", Payload: &chatappv1.ChatRunFailed{MessageId: "assistant-err", Error: "boom"}},
	})
	require.NoError(t, err)

	errCreated := requireMsgType[timeline.UIEntityCreated](t, sender.msgs, 0)
	require.Equal(t, "assistant-err", errCreated.ID.LocalID)
	require.Contains(t, errCreated.Props["text"], "boom")
	requireMsgType[boba_chat.BackendFinishedMsg](t, sender.msgs, len(sender.msgs)-1)
}

type recordingTeaSender struct{ msgs []tea.Msg }

func (s *recordingTeaSender) Send(msg tea.Msg) { s.msgs = append(s.msgs, msg) }

func requireMsgType[T any](t *testing.T, msgs []tea.Msg, index int) T {
	t.Helper()
	require.Greater(t, len(msgs), index)
	msg, ok := msgs[index].(T)
	require.Truef(t, ok, "message %d has type %T", index, msgs[index])
	return msg
}
