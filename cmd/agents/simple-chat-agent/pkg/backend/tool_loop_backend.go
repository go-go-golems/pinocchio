package backend

import (
    "context"
    "fmt"
    "sync/atomic"
    "time"

    "github.com/ThreeDotsLabs/watermill/message"
    tea "github.com/charmbracelet/bubbletea"
    boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
    "github.com/go-go-golems/bobatea/pkg/timeline"
    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
    "github.com/go-go-golems/geppetto/pkg/inference/tools"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
)

// ToolLoopBackend runs the tool-calling loop across turns and emits BackendFinishedMsg when done.
type ToolLoopBackend struct {
    eng  engine.Engine
    reg  *tools.InMemoryToolRegistry
    sink *middleware.WatermillSink
    hook toolhelpers.SnapshotHook

    turn   *turns.Turn
    cancel context.CancelFunc
    running atomic.Bool
}

func NewToolLoopBackend(eng engine.Engine, reg *tools.InMemoryToolRegistry, sink *middleware.WatermillSink, hook toolhelpers.SnapshotHook) *ToolLoopBackend {
    return &ToolLoopBackend{eng: eng, reg: reg, sink: sink, hook: hook, turn: &turns.Turn{Data: map[string]any{}}}
}

func (b *ToolLoopBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
    if !b.running.CompareAndSwap(false, true) {
        return nil, errors.New("already running")
    }
    if b.turn == nil {
        b.turn = &turns.Turn{Data: map[string]any{}}
    }
    if prompt != "" {
        turns.AppendBlock(b.turn, turns.NewUserTextBlock(prompt))
    }

    ctx, b.cancel = context.WithCancel(ctx)
    runCtx := events.WithEventSinks(ctx, b.sink)
    if b.hook != nil {
        runCtx = toolhelpers.WithTurnSnapshotHook(runCtx, b.hook)
    }

    return func() tea.Msg {
        updated, err := toolhelpers.RunToolCallingLoop(
            runCtx,
            b.eng,
            b.turn,
            b.reg,
            toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second),
        )
        if err != nil {
            log.Error().Err(err).Msg("tool loop failed")
        }
        if updated != nil {
            b.turn = updated
        }
        b.running.Store(false)
        b.cancel = nil
        return boba_chat.BackendFinishedMsg{}
    }, nil
}

func (b *ToolLoopBackend) Interrupt() {
    if b.cancel != nil {
        b.cancel()
    }
}

func (b *ToolLoopBackend) Kill() {
    if b.cancel != nil {
        b.cancel()
        b.cancel = nil
        b.running.Store(false)
    }
}

func (b *ToolLoopBackend) IsFinished() bool {
    return !b.running.Load()
}

// MakeUIForwarder returns a Watermill handler that forwards geppetto events to the Bubble Tea program p
// without signaling backend finish on provider events. The backend itself will emit BackendFinishedMsg
// when the tool loop completes.
func (b *ToolLoopBackend) MakeUIForwarder(p *tea.Program) func(msg *message.Message) error {
    return func(msg *message.Message) error {
        msg.Ack()
        e, err := events.NewEventFromJson(msg.Payload)
        if err != nil {
            log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("agent forwarder: parse error")
            return err
        }
        md := e.Metadata()
        entityID := md.ID.String()
        log.Debug().Interface("event", e).Str("event_type", fmt.Sprintf("%T", e)).Str("entity_id", entityID).Msg("agent forwarder: dispatch")

        switch e_ := e.(type) {
        case *events.EventPartialCompletionStart:
            p.Send(timeline.UIEntityCreated{
                ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
                Props:     map[string]any{"role": "assistant", "text": "", "metadata": md.LLMInferenceData, "streaming": true},
                StartedAt: time.Now(),
            })
        case *events.EventPartialCompletion:
            p.Send(timeline.UIEntityUpdated{
                ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Patch:     map[string]any{"text": e_.Completion, "metadata": md.LLMInferenceData, "streaming": true},
                Version:   time.Now().UnixNano(),
                UpdatedAt: time.Now(),
            })
        case *events.EventFinal:
            p.Send(timeline.UIEntityCompleted{
                ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Result: map[string]any{"text": e_.Text, "metadata": md.LLMInferenceData},
            })
            p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
        case *events.EventInterrupt:
            intr, ok := events.ToTypedEvent[events.EventInterrupt](e)
            if !ok {
                return errors.New("payload is not of type EventInterrupt")
            }
            p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Result: map[string]any{"text": intr.Text}})
            p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
        case *events.EventError:
            p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Result: map[string]any{"text": "**Error**\n\n" + e_.ErrorString}})
            p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
        case *events.EventToolCall:
            p.Send(timeline.UIEntityCreated{
                ID:        timeline.EntityID{LocalID: e_.ToolCall.ID, Kind: "tool_call"},
                Renderer:  timeline.RendererDescriptor{Kind: "tool_call"},
                Props:     map[string]any{"name": e_.ToolCall.Name, "input": e_.ToolCall.Input},
                StartedAt: time.Now(),
            })
        case *events.EventToolCallExecute:
            p.Send(timeline.UIEntityUpdated{
                ID:        timeline.EntityID{LocalID: e_.ToolCall.ID, Kind: "tool_call"},
                Patch:     map[string]any{"exec": true, "input": e_.ToolCall.Input},
                Version:   time.Now().UnixNano(),
                UpdatedAt: time.Now(),
            })
        case *events.EventToolResult:
            p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: e_.ToolResult.ID, Kind: "tool_call"}, Patch: map[string]any{"result": e_.ToolResult.Result}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
            p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: e_.ToolResult.ID, Kind: "tool_call"}})
        case *events.EventToolCallExecutionResult:
            p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: e_.ToolResult.ID, Kind: "tool_call"}, Patch: map[string]any{"result": e_.ToolResult.Result}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
            p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: e_.ToolResult.ID, Kind: "tool_call"}})
        case *events.EventAgentModeSwitch:
            // Render a plain entity summarizing the mode change and analysis
            props := map[string]any{"title": e_.Message}
            for k, v := range e_.Data { props[k] = v }
            p.Send(timeline.UIEntityCreated{
                ID:       timeline.EntityID{LocalID: fmt.Sprintf("agentmode-%d", time.Now().UnixNano()), Kind: "plain"},
                Renderer: timeline.RendererDescriptor{Kind: "plain"},
                Props:    props,
            })
            p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: fmt.Sprintf("agentmode-%d", time.Now().UnixNano()), Kind: "plain"}})
        }
        return nil
    }
}


