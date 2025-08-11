package eval

import (
	"context"
	"strings"
	"time"

	"github.com/go-go-golems/bobatea/pkg/repl"
	"github.com/go-go-golems/geppetto/pkg/conversation"
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
	manager  conversation.Manager
	registry *tools.InMemoryToolRegistry
	sink     *middleware.WatermillSink

    snapshotHook toolhelpers.SnapshotHook
}

func NewChatEvaluator(
	eng engine.Engine,
	manager conversation.Manager,
	registry *tools.InMemoryToolRegistry,
    sink *middleware.WatermillSink,
    hook toolhelpers.SnapshotHook,
) *ChatEvaluator {
    return &ChatEvaluator{eng: eng, manager: manager, registry: registry, sink: sink, snapshotHook: hook}
}

func (e *ChatEvaluator) Evaluate(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", nil
	}
	if err := e.manager.AppendMessages(conversation.NewChatMessage(conversation.RoleUser, code)); err != nil {
		return "", err
	}
    conv := e.manager.GetConversation()
    runCtx := events.WithEventSinks(ctx, e.sink)
    if e.snapshotHook != nil {
        runCtx = toolhelpers.WithTurnSnapshotHook(runCtx, e.snapshotHook)
    }
    // Build a Turn from the conversation and run the tool-calling loop on Turns
    t := &turns.Turn{Data: map[string]any{}}
    blocks := turns.BlocksFromConversationDelta(conv, 0)
    turns.AppendBlocks(t, blocks...)
    t2, err := toolhelpers.RunToolCallingLoop(
        runCtx, e.eng, t, e.registry,
        toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second),
    )
	if err != nil {
		return "", err
	}
    updated := turns.BuildConversationFromTurn(t2)
    for _, m := range updated[len(conv):] {
		if err := e.manager.AppendMessages(m); err != nil {
			return "", err
		}
	}
	var last string
	for i := len(updated) - 1; i >= 0; i-- {
		msg := updated[i]
		if msg.Content.ContentType() == conversation.ContentTypeChatMessage {
			c := msg.Content.(*conversation.ChatMessageContent)
			if c.Role == conversation.RoleAssistant {
				last = c.Text
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
