package agentd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/google/uuid"
)

// FakeModel describes the scripted engine.
var FakeModel = modelsource.Model{Ref: "fake:scripted", Source: "fake", Label: "Scripted test engine", API: "fake", ContextWindow: 1 << 20}

// FakeEngine is a deterministic engine for tests and UI development. It reads
// the last user message (ignoring context blocks):
//
//	/tool NAME {json}   call tool NAME with the JSON object as input
//	/tools N NAME {json} call tool NAME N times in a row, one per round
//	/sleep MS text      wait MS milliseconds (observes cancellation), then echo text
//	anything else       answer "echo: <text>"
//
// After a tool result it answers "NAME returned: <result>" (or the error).
// It publishes text-delta and provider-call-finished events like a real engine.
type FakeEngine struct {
	// Delay between streamed chunks.
	Delay time.Duration
}

var _ engine.Engine = (*FakeEngine)(nil)

func (f *FakeEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if t == nil {
		t = &turns.Turn{}
	}
	last := t.Blocks
	var tail turns.Block
	if len(last) > 0 {
		tail = last[len(last)-1]
	}
	user := lastUserText(t)
	// Tool choice "none" (the final round of a turn that used all its rounds):
	// answer with a summary, as a real model would.
	if cfg, ok, _ := engine.KeyToolConfig.Get(t.Data); ok && cfg.ToolChoice == engine.ToolChoiceNone {
		return f.say(ctx, t, fmt.Sprintf("final answer after %d tool calls", countAllToolCalls(t)))
	}
	switch {
	case tail.Kind == turns.BlockKindToolUse:
		name := toolNameFor(t, str(tail.Payload, turns.PayloadKeyID))
		if remaining, input, ok := repeatPlan(user); ok && countToolCalls(t, name) < remaining {
			return f.callTool(ctx, t, name, input)
		}
		res := str(tail.Payload, turns.PayloadKeyError)
		if res == "" {
			if v, ok := tail.Payload[turns.PayloadKeyResult].(string); ok {
				res = v
			} else {
				b, _ := json.Marshal(unwrapValue(tail.Payload[turns.PayloadKeyResult]))
				res = string(b)
			}
		} else {
			res = "error: " + res
		}
		return f.say(ctx, t, fmt.Sprintf("%s returned: %s", name, res))
	case strings.HasPrefix(user, "/tool "):
		name, input := parseToolCommand(strings.TrimPrefix(user, "/tool "))
		return f.callTool(ctx, t, name, input)
	case strings.HasPrefix(user, "/tools "):
		if _, input, ok := repeatPlan(user); ok {
			name := strings.Fields(strings.TrimPrefix(user, "/tools "))[1]
			return f.callTool(ctx, t, name, input)
		}
	case strings.HasPrefix(user, "/sleep "):
		var ms int
		var rest string
		_, _ = fmt.Sscanf(strings.TrimPrefix(user, "/sleep "), "%d", &ms)
		if parts := strings.SplitN(strings.TrimPrefix(user, "/sleep "), " ", 2); len(parts) == 2 {
			rest = parts[1]
		}
		select {
		case <-ctx.Done():
			return t, ctx.Err()
		case <-time.After(time.Duration(ms) * time.Millisecond):
		}
		return f.say(ctx, t, "echo: "+rest)
	}
	return f.say(ctx, t, "echo: "+user)
}

func (f *FakeEngine) say(ctx context.Context, t *turns.Turn, text string) (*turns.Turn, error) {
	meta := gepevents.EventMetadata{ID: uuid.New(), LLMInferenceData: gepevents.LLMInferenceData{Model: "fake"}}
	corr := gepevents.Correlation{SegmentID: uuid.NewString()}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	words := strings.SplitAfter(text, " ")
	acc := ""
	for i, w := range words {
		if f.Delay > 0 {
			select {
			case <-ctx.Done():
				return t, ctx.Err()
			case <-time.After(f.Delay):
			}
		}
		acc += w
		gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, w, acc, int64(i+1)))
	}
	turns.AppendBlock(t, turns.NewAssistantTextBlock(text))
	f.usage(ctx, meta, t, len(words), false)
	return t, nil
}

func (f *FakeEngine) callTool(ctx context.Context, t *turns.Turn, name string, input map[string]any) (*turns.Turn, error) {
	turns.AppendBlock(t, turns.NewToolCallBlock("call_"+uuid.NewString()[:8], name, input))
	meta := gepevents.EventMetadata{ID: uuid.New(), LLMInferenceData: gepevents.LLMInferenceData{Model: "fake"}}
	f.usage(ctx, meta, t, 5, true)
	return t, nil
}

func (f *FakeEngine) usage(ctx context.Context, meta gepevents.EventMetadata, t *turns.Turn, out int, tools bool) {
	in := 0
	for _, b := range t.Blocks {
		in += len(str(b.Payload, turns.PayloadKeyText)) / 4
	}
	gepevents.PublishEventToContext(ctx, gepevents.NewProviderCallFinishedEvent(meta, gepevents.Correlation{}, "stop", "", &gepevents.Usage{InputTokens: in, OutputTokens: out}, nil, tools))
}

func str(p map[string]any, k string) string { v, _ := p[k].(string); return v }

func lastUserText(t *turns.Turn) string {
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		b := t.Blocks[i]
		if b.Kind == turns.BlockKindUser {
			if s := str(b.Payload, turns.PayloadKeyText); !strings.HasPrefix(s, ContextPrefix) {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func toolNameFor(t *turns.Turn, callID string) string {
	for _, b := range t.Blocks {
		if b.Kind == turns.BlockKindToolCall && str(b.Payload, turns.PayloadKeyID) == callID {
			return str(b.Payload, turns.PayloadKeyName)
		}
	}
	return "tool"
}

// countToolCalls counts calls to name after the last user message.
func countToolCalls(t *turns.Turn, name string) int {
	n := 0
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		b := t.Blocks[i]
		if b.Kind == turns.BlockKindUser && !strings.HasPrefix(str(b.Payload, turns.PayloadKeyText), ContextPrefix) {
			break
		}
		if b.Kind == turns.BlockKindToolCall && str(b.Payload, turns.PayloadKeyName) == name {
			n++
		}
	}
	return n
}

func repeatPlan(user string) (int, map[string]any, bool) {
	if !strings.HasPrefix(user, "/tools ") {
		return 0, nil, false
	}
	f := strings.Fields(strings.TrimPrefix(user, "/tools "))
	if len(f) < 2 {
		return 0, nil, false
	}
	var n int
	if _, err := fmt.Sscanf(f[0], "%d", &n); err != nil {
		return 0, nil, false
	}
	rest := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(user, "/tools "), " ", 3)[2:][0])
	_, input := parseToolCommand(f[1] + " " + rest)
	return n, input, true
}

func parseToolCommand(s string) (string, map[string]any) {
	s = strings.TrimSpace(s)
	name, rest, _ := strings.Cut(s, " ")
	input := map[string]any{}
	if strings.TrimSpace(rest) != "" {
		if err := json.Unmarshal([]byte(rest), &input); err != nil {
			input = map[string]any{"text": rest}
		}
	}
	return name, input
}

func countAllToolCalls(t *turns.Turn) int {
	n := 0
	for _, b := range t.Blocks {
		if b.Kind == turns.BlockKindToolCall {
			n++
		}
	}
	return n
}
