package chatapp

import (
	"context"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

// answerEngine is a test engine that answers "Answer: <last user text>",
// streamed in chunks with a delay between them. It stops with ctx.Err() when
// the run is cancelled, so it can stand in for a streaming provider in stop
// tests. It replaces the demo inference chatapp used to run when a prompt had
// no runtime.
type answerEngine struct {
	chunk int
	delay time.Duration
}

func answerRuntime() *infruntime.ComposedRuntime {
	return &infruntime.ComposedRuntime{Engine: answerEngine{chunk: 10, delay: time.Millisecond}}
}

func slowAnswerRuntime() *infruntime.ComposedRuntime {
	return &infruntime.ComposedRuntime{Engine: answerEngine{chunk: 4, delay: 10 * time.Millisecond}}
}

func (e answerEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	answer := "Answer: " + lastUserText(t)
	meta := gepevents.EventMetadata{SessionID: "sid"}
	corr := testTextCorrelation(1)
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	chunk := e.chunk
	if chunk <= 0 {
		chunk = len(answer)
	}
	sent := ""
	for i := 0; i < len(answer); i += chunk {
		select {
		case <-ctx.Done():
			return t, ctx.Err()
		case <-time.After(e.delay):
		}
		end := min(i+chunk, len(answer))
		sent += answer[i:end]
		gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, answer[i:end], sent, int64(i/chunk+1)))
	}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, answer, "stop"))
	turns.AppendBlock(t, turns.NewAssistantTextBlock(answer))
	return t, nil
}

func lastUserText(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		if t.Blocks[i].Kind == turns.BlockKindUser {
			text, _ := t.Blocks[i].Payload[turns.PayloadKeyText].(string)
			return text
		}
	}
	return ""
}
