package chatapp

import (
	"context"
	"time"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

func (e *Engine) runDemoInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, pub sessionstream.EventPublisher) {
	if err := e.publish(ctx, sid, pub, EventChatRunStarted, &chatappv1.ChatRunStarted{MessageId: messageID, Prompt: prompt, Correlation: runCorrelationInfo(sid, messageID)}); err != nil {
		return
	}

	answer := renderAnswer(prompt)
	chunks := chunkText(answer, 10)
	accumulated := ""
	textMessageID := textSegmentMessageID(messageID, 1)
	corr := &chatappv1.CorrelationInfo{SessionId: string(sid), RunId: messageID, SegmentId: textMessageID}
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Status: "streaming", Streaming: true, Correlation: corr}); err != nil {
		return
	}
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			_ = e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Text: accumulated, Content: accumulated, Status: "stopped", Streaming: false, Final: true, FinishReason: "stopped", Correlation: corr})
			_ = e.publish(publishContext(ctx), sid, pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: messageID, Status: "stopped", Correlation: runCorrelationInfo(sid, messageID)})
			return
		case <-time.After(e.chunkDelay):
		}
		accumulated += chunk
		if err := e.publish(publishContext(ctx), sid, pub, EventChatTextPatch, &chatappv1.ChatTextPatch{MessageId: textMessageID, Role: "assistant", Prompt: prompt, StreamId: textMessageID, Sequence: uint64(i + 1), Offset: uint64(len(accumulated) - len(chunk)), Text: chunk, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Status: "streaming", Correlation: corr}); err != nil {
			return
		}
	}
	if err := e.publish(publishContext(ctx), sid, pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: prompt, Text: accumulated, Content: accumulated, Status: "finished", Streaming: false, Final: true, Correlation: corr}); err != nil {
		return
	}
	_ = e.publish(publishContext(ctx), sid, pub, EventChatRunFinished, &chatappv1.ChatRunFinished{MessageId: messageID, Status: "finished", Correlation: runCorrelationInfo(sid, messageID)})
}

func renderAnswer(prompt string) string {
	return "Answer: " + prompt
}

func chunkText(text string, size int) []string {
	if size <= 0 || len(text) <= size {
		return []string{text}
	}
	out := make([]string, 0, (len(text)+size-1)/size)
	for len(text) > 0 {
		if len(text) <= size {
			out = append(out, text)
			break
		}
		out = append(out, text[:size])
		text = text[size:]
	}
	return out
}
