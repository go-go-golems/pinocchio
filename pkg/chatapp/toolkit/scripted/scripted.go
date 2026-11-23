// Package scripted is a deterministic geppetto engine for tests and UI
// development: the person's message is a small script.
//
//	/tool NAME {json}     call tool NAME with the JSON object as input
//	/tools N NAME {json}  call tool NAME N times in a row, one per round
//	/sleep MS text        wait MS milliseconds (observes cancellation), then echo text
//	anything else         answer "echo: <text>"
//
// After a tool result it answers "NAME returned: <result>" (or the error).
// At the round limit's final answer (tool choice "none") it answers "final
// answer after N tool calls". It streams its text and reports usage like a
// provider engine.
//
// The person's message is the first user block of the turn's last run of user
// blocks, so blocks an application appends after the message
// (chatapp.PromptRequest.Blocks) do not change the script.
package scripted

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

// Model describes the engine for model lists.
var Model = modelsource.Model{Ref: "fake:scripted", Source: "fake", Label: "Scripted test engine", API: "fake", ContextWindow: 1 << 20}

// Engine is the scripted engine.
type Engine struct {
	// Delay between streamed words.
	Delay time.Duration
	// FinalError, when set, fails the final answer at the round limit, as a
	// provider error would.
	FinalError error
}

var _ engine.Engine = (*Engine)(nil)

// RunInference runs the script of the turn's person message.
func (f *Engine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if t == nil {
		t = &turns.Turn{}
	}
	var tail turns.Block
	if len(t.Blocks) > 0 {
		tail = t.Blocks[len(t.Blocks)-1]
	}
	msgIdx, user := personMessage(t)
	if cfg, ok, _ := engine.KeyToolConfig.Get(t.Data); ok && cfg.ToolChoice == engine.ToolChoiceNone {
		if f.FinalError != nil {
			return t, f.FinalError
		}
		return f.say(ctx, t, fmt.Sprintf("final answer after %d tool calls", countToolCalls(t, 0, "")))
	}
	switch {
	case tail.Kind == turns.BlockKindToolUse:
		name := toolNameFor(t, str(tail.Payload, turns.PayloadKeyID))
		if remaining, input, ok := repeatPlan(user); ok && countToolCalls(t, msgIdx, name) < remaining {
			return f.callTool(ctx, t, name, input)
		}
		res := str(tail.Payload, turns.PayloadKeyError)
		if res == "" {
			if v, ok := tail.Payload[turns.PayloadKeyResult].(string); ok {
				res = v
			} else {
				b, _ := json.Marshal(tail.Payload[turns.PayloadKeyResult])
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
		rest := ""
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

func (f *Engine) say(ctx context.Context, t *turns.Turn, text string) (*turns.Turn, error) {
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
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, text, "stop"))
	turns.AppendBlock(t, turns.NewAssistantTextBlock(text))
	f.usage(ctx, meta, t, len(words), false)
	return t, nil
}

func (f *Engine) callTool(ctx context.Context, t *turns.Turn, name string, input map[string]any) (*turns.Turn, error) {
	turns.AppendBlock(t, turns.NewToolCallBlock("call_"+uuid.NewString()[:8], name, input))
	meta := gepevents.EventMetadata{ID: uuid.New(), LLMInferenceData: gepevents.LLMInferenceData{Model: "fake"}}
	f.usage(ctx, meta, t, 5, true)
	return t, nil
}

// usage reports roughly a token per four characters of input.
func (f *Engine) usage(ctx context.Context, meta gepevents.EventMetadata, t *turns.Turn, out int, tools bool) {
	in := 0
	for _, b := range t.Blocks {
		in += len(str(b.Payload, turns.PayloadKeyText)) / 4
	}
	gepevents.PublishEventToContext(ctx, gepevents.NewProviderCallFinishedEvent(meta, gepevents.Correlation{}, "stop", "", &gepevents.Usage{InputTokens: in, OutputTokens: out}, nil, tools))
}

func str(p map[string]any, k string) string { v, _ := p[k].(string); return v }

// personMessage returns the index and text of the first user block of the
// turn's last run of user blocks: the person's message of the current run.
func personMessage(t *turns.Turn) (int, string) {
	i := len(t.Blocks) - 1
	for i >= 0 && t.Blocks[i].Kind != turns.BlockKindUser {
		i--
	}
	if i < 0 {
		return 0, ""
	}
	for i > 0 && t.Blocks[i-1].Kind == turns.BlockKindUser {
		i--
	}
	return i, strings.TrimSpace(str(t.Blocks[i].Payload, turns.PayloadKeyText))
}

func toolNameFor(t *turns.Turn, callID string) string {
	for _, b := range t.Blocks {
		if b.Kind == turns.BlockKindToolCall && str(b.Payload, turns.PayloadKeyID) == callID {
			return str(b.Payload, turns.PayloadKeyName)
		}
	}
	return "tool"
}

// countToolCalls counts calls (to name, or to any tool for "") from index from on.
func countToolCalls(t *turns.Turn, from int, name string) int {
	n := 0
	for _, b := range t.Blocks[from:] {
		if b.Kind == turns.BlockKindToolCall && (name == "" || str(b.Payload, turns.PayloadKeyName) == name) {
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
	rest := ""
	if parts := strings.SplitN(strings.TrimPrefix(user, "/tools "), " ", 3); len(parts) == 3 {
		rest = strings.TrimSpace(parts[2])
	}
	_, input := parseToolCommand(f[1] + " " + rest)
	return n, input, true
}

func parseToolCommand(s string) (string, map[string]any) {
	name, rest, _ := strings.Cut(strings.TrimSpace(s), " ")
	input := map[string]any{}
	if strings.TrimSpace(rest) != "" {
		if err := json.Unmarshal([]byte(rest), &input); err != nil {
			input = map[string]any{"text": rest}
		}
	}
	return name, input
}
