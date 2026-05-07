package chatapp

import (
	"context"
	"time"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

func (e *Engine) runDemoInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, pub sessionstream.EventPublisher) {
	started := newChatMessageUpdate(messageID, "assistant", "", "", prompt, "streaming", true, "")
	if err := e.publish(ctx, sid, pub, EventInferenceStarted, started); err != nil {
		return
	}

	answer := renderAnswer(prompt)
	chunks := chunkText(answer, 10)
	accumulated := ""
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			_ = e.publish(publishContext(ctx), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", accumulated, accumulated, prompt, "stopped", false, ""))
			return
		case <-time.After(e.chunkDelay):
		}
		accumulated += chunk
		if err := e.publish(publishContext(ctx), sid, pub, EventTokensDelta, newChatMessageDelta(messageID, chunk, accumulated, prompt, "streaming", true, "")); err != nil {
			return
		}
	}
	_ = e.publish(publishContext(ctx), sid, pub, EventInferenceFinished, newChatMessageUpdate(messageID, "assistant", accumulated, accumulated, prompt, "finished", false, ""))
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
