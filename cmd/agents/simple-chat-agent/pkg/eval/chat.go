package eval

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/bobatea/pkg/repl"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
    	"github.com/go-go-golems/geppetto/pkg/turns"
)

// Ensure ChatEvaluator implements repl.Evaluator
var _ repl.Evaluator = (*ChatEvaluator)(nil)

type ChatEvaluator struct {
	eng      engine.Engine
	turn     *turns.Turn
	registry *tools.InMemoryToolRegistry
	sink     *middleware.WatermillSink

	snapshotHook toolhelpers.SnapshotHook
}

func NewChatEvaluator(
	eng engine.Engine,
	registry *tools.InMemoryToolRegistry,
    	sink *middleware.WatermillSink,
    	hook toolhelpers.SnapshotHook,
) *ChatEvaluator {
    	return &ChatEvaluator{eng: eng, turn: &turns.Turn{Data: map[string]any{}}, registry: registry, sink: sink, snapshotHook: hook}
}

func (e *ChatEvaluator) Evaluate(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", nil
	}
	// Append user message to ongoing Turn
	if e.turn == nil {
		e.turn = &turns.Turn{Data: map[string]any{}}
	}
	prevLen := len(e.turn.Blocks)
	turns.AppendBlock(e.turn, turns.NewUserTextBlock(code))

	runCtx := events.WithEventSinks(ctx, e.sink)
	if e.snapshotHook != nil {
		runCtx = toolhelpers.WithTurnSnapshotHook(runCtx, e.snapshotHook)
	}
	// Run tool-calling loop on the current Turn
	t2, err := toolhelpers.RunToolCallingLoop(
		runCtx, e.eng, e.turn, e.registry,
		toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second),
	)
	if err != nil {
		return "", err
	}
	// Update state
	e.turn = t2

	// Return last assistant text added in this evaluation step
	var last string
	for i := len(e.turn.Blocks) - 1; i >= prevLen; i-- {
		b := e.turn.Blocks[i]
		if b.Kind == turns.BlockKindLLMText {
			if txt, ok := b.Payload[turns.PayloadKeyText].(string); ok {
				last = txt
				break
			}
		}
	}
	return last, nil
}

func (e *ChatEvaluator) GetPrompt() string        { return "> " }
func (e *ChatEvaluator) GetName() string          { return "Chat" }
func (e *ChatEvaluator) SupportsMultiline() bool  { return true }
func (e *ChatEvaluator) GetFileExtension() string { return ".txt" }
