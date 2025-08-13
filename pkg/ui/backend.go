package ui

import (
    "context"
    "fmt"
    "sync"
    "time"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"

    "github.com/ThreeDotsLabs/watermill/message"
    tea "github.com/charmbracelet/bubbletea"
    boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
    "github.com/go-go-golems/bobatea/pkg/timeline"
    "github.com/go-go-golems/geppetto/pkg/events"
    conv "github.com/go-go-golems/geppetto/pkg/conversation"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
)

// EngineBackend provides a Backend implementation using the Engine-first architecture.
type EngineBackend struct {
	engine    engine.Engine
	isRunning bool
	cancel    context.CancelFunc
    historyMu sync.RWMutex
    history   []*turns.Turn
}

var _ boba_chat.Backend = &EngineBackend{}

// NewEngineBackend creates a new EngineBackend with the given engine and event sink.
// The eventSink is used to publish events during inference for UI updates.
func NewEngineBackend(engine engine.Engine) *EngineBackend {
	return &EngineBackend{
		engine:    engine,
		isRunning: false,
	}
}

// Start executes inference using the engine and publishes events through the sink.
// This method implements the boba_chat.Backend interface with a plain prompt string.
func (e *EngineBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
    log.Debug().Str("component", "engine_backend").Str("method", "Start").Bool("already_running", e.isRunning).Msg("Start called")
	if e.isRunning {
        log.Debug().Str("component", "engine_backend").Msg("Start rejected: already running")
		return nil, errors.New("Engine is already running")
	}

	// Create cancellable context for this inference run
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.isRunning = true

	// Create engine with the event sink if provided
	engine := e.engine

    return func() tea.Msg {
		if !e.isRunning {
			return nil
		}

        log.Debug().Str("component", "engine_backend").Msg("Reducing history, appending user block, running inference")
        // Reduce history into a seed Turn, append user Block, then run inference
        seed := e.reduceHistory()
        if prompt != "" {
            turns.AppendBlock(seed, turns.NewUserTextBlock(prompt))
        }
        updated, err := engine.RunInference(ctx, seed)

		// Mark as finished
		e.isRunning = false
		e.cancel = nil

        if err != nil {
			log.Error().Err(err).Msg("Engine inference failed")
            log.Error().Err(err).Str("component", "engine_backend").Msg("RunInference failed")
		}
        // Append updated Turn to history for cohesive continuation
        if updated != nil {
            e.historyMu.Lock()
            e.history = append(e.history, updated)
            e.historyMu.Unlock()
            log.Debug().Str("component", "engine_backend").Int("turn_blocks", len(updated.Blocks)).Int("history_len", len(e.history)).Msg("Appended updated Turn to history")
        }
        log.Debug().Str("component", "engine_backend").Msg("Returning BackendFinishedMsg")
        return boba_chat.BackendFinishedMsg{}
	}, nil
}

// SetSeedFromConversation populates the initial Turn with system and prior user messages
func (e *EngineBackend) SetSeedFromConversation(c conv.Conversation) {
    t := &turns.Turn{}
    // Convert existing conversation into blocks (system/user/assistant as available)
    turns.AppendBlocks(t, turns.BlocksFromConversationDelta(c, 0)...)
    e.historyMu.Lock()
    e.history = append(e.history, t)
    e.historyMu.Unlock()
    log.Debug().Str("component", "engine_backend").Int("seed_blocks", len(t.Blocks)).Int("history_len", len(e.history)).Msg("Seed Turn appended to history from conversation")
}

// SetSeedTurn sets the seed Turn directly
func (e *EngineBackend) SetSeedTurn(t *turns.Turn) {
    e.historyMu.Lock()
    e.history = append(e.history, t)
    e.historyMu.Unlock()
    log.Debug().Str("component", "engine_backend").Int("seed_blocks", len(t.Blocks)).Int("history_len", len(e.history)).Msg("Seed Turn appended to history")
}

// Interrupt attempts to cancel the current inference operation.
func (e *EngineBackend) Interrupt() {
	if e.cancel != nil {
		e.cancel()
	} else {
		log.Warn().Msg("Engine is not running")
	}
}

// Kill forcefully cancels the current inference operation.
func (e *EngineBackend) Kill() {
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
		e.isRunning = false
	} else {
		log.Debug().Msg("Engine is not running")
	}
}

// IsFinished returns whether the engine is currently running an inference operation.
func (e *EngineBackend) IsFinished() bool {
	return !e.isRunning
}

// reduceHistory flattens all prior Turns into a single Turn by concatenating Blocks
func (e *EngineBackend) reduceHistory() *turns.Turn {
    out := &turns.Turn{}
    e.historyMu.RLock()
    defer e.historyMu.RUnlock()
    for _, t := range e.history {
        if t == nil { continue }
        turns.AppendBlocks(out, t.Blocks...)
    }
    return out
}

// StepChatForwardFunc is a function that forwards watermill messages to the UI by
// trasnforming them into bubbletea messages and injecting them into the program `p`.
func StepChatForwardFunc(p *tea.Program) func(msg *message.Message) error {
	return func(msg *message.Message) error {
		msg.Ack()

		e, err := events.NewEventFromJson(msg.Payload)
		if err != nil {
			log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("Failed to parse event")
            log.Error().Err(err).Int("payload_len", len(msg.Payload)).Str("component", "step_forward").Msg("Failed to parse event from payload")
			return err
		}

        md := e.Metadata()
        entityID := md.ID.String()
        log.Debug().Interface("event", e).Str("event_type", fmt.Sprintf("%T", e)).Str("entity_id", entityID).Msg("Dispatching event to UI")

        switch e_ := e.(type) {
        case *events.EventPartialCompletionStart:
            // Create assistant message entity for this stream
            log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Msg("UIEntityCreated (llm_text)")
            p.Send(timeline.UIEntityCreated{
                ID:       timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Renderer: timeline.RendererDescriptor{Kind: "llm_text"},
                Props:    map[string]any{"role": "assistant", "text": ""},
                StartedAt: time.Now(),
            })
        case *events.EventPartialCompletion:
            // Update accumulated assistant text using the Completion field
            log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Int("delta_len", len(e_.Delta)).Int("completion_len", len(e_.Completion)).Msg("UIEntityUpdated (llm_text)")
            p.Send(timeline.UIEntityUpdated{
                ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Patch:     map[string]any{"text": e_.Completion},
                Version:   time.Now().UnixNano(),
                UpdatedAt: time.Now(),
            })
        case *events.EventFinal:
            log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Int("text_len", len(e_.Text)).Msg("UIEntityCompleted (final)")
            p.Send(timeline.UIEntityCompleted{
                ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Result: map[string]any{"text": e_.Text},
            })
            p.Send(boba_chat.BackendFinishedMsg{})
        case *events.EventInterrupt:
            intr, ok := events.ToTypedEvent[events.EventInterrupt](e)
            if !ok {
                log.Error().Str("component", "step_forward").Msg("EventInterrupt type assertion failed")
                return errors.New("payload is not of type EventInterrupt")
            }
            log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Int("text_len", len(intr.Text)).Msg("UIEntityCompleted (interrupt)")
            p.Send(timeline.UIEntityCompleted{
                ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Result: map[string]any{"text": intr.Text},
            })
            p.Send(boba_chat.BackendFinishedMsg{})
        case *events.EventError:
            log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Msg("UIEntityCompleted (error)")
            p.Send(timeline.UIEntityCompleted{
                ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
                Result: map[string]any{"text": "**Error**\n\n" + e_.ErrorString},
            })
            p.Send(boba_chat.BackendFinishedMsg{})
        // Tool-related events can be mapped to dedicated tool_call entities if desired
        }

		return nil
	}
}
