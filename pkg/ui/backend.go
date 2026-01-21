package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// EngineBackend provides a Backend implementation using the Engine-first architecture.
type EngineBackend struct {
	engine  engine.Engine
	builder *session.ToolLoopEngineBuilder

	sessMu sync.RWMutex
	sess   *session.Session

	program *tea.Program

	emittedMu sync.Mutex
	emitted   map[string]struct{}
}

var _ boba_chat.Backend = &EngineBackend{}

// NewEngineBackend creates a new EngineBackend with the given engine and event sink.
// The eventSink is used to publish events during inference for UI updates.
func NewEngineBackend(engine engine.Engine, sinks ...events.EventSink) *EngineBackend {
	builder := &session.ToolLoopEngineBuilder{
		Base:       engine,
		EventSinks: append([]events.EventSink(nil), sinks...),
	}
	return &EngineBackend{
		engine:  engine,
		builder: builder,
		sess: &session.Session{
			Builder: builder,
		},
		emitted: make(map[string]struct{}),
	}
}

// AttachProgram registers the UI program to allow emitting initial timeline entities
// when seeding history. If not attached, seeding will only populate backend state.
func (e *EngineBackend) AttachProgram(p *tea.Program) {
	e.program = p
}

// Start executes inference using the engine and publishes events through the sink.
// This method implements the boba_chat.Backend interface with a plain prompt string.
func (e *EngineBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
	log.Debug().Str("component", "engine_backend").Str("method", "Start").Msg("Start called")

	e.sessMu.RLock()
	sess := e.sess
	e.sessMu.RUnlock()
	if sess == nil {
		return nil, errors.New("session is nil")
	}
	if sess.IsRunning() {
		log.Debug().Str("component", "engine_backend").Msg("Start rejected: already running")
		return nil, errors.New("Engine is already running")
	}

	log.Debug().Str("component", "engine_backend").Msg("Building seed turn, appending user block, starting inference")
	seed := e.snapshotForPrompt(prompt)
	sess.Append(seed)

	handle, err := sess.StartInference(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "start inference")
	}

	return func() tea.Msg {
		updated, err := handle.Wait()
		if err != nil {
			log.Error().Err(err).Msg("Engine inference failed")
			log.Error().Err(err).Str("component", "engine_backend").Msg("RunInference failed")
		}
		if updated != nil {
			log.Debug().Str("component", "engine_backend").Int("turn_blocks", len(updated.Blocks)).Msg("Updated conversation state from inference")
		}
		log.Debug().Str("component", "engine_backend").Msg("Returning BackendFinishedMsg")
		return boba_chat.BackendFinishedMsg{}
	}, nil
}

// SetSeedTurn sets the seed Turn directly
func (e *EngineBackend) SetSeedTurn(t *turns.Turn) {
	if t == nil {
		return
	}
	seed := cloneTurn(t)

	e.sessMu.Lock()
	runID := ""
	if seed.RunID != "" {
		runID = seed.RunID
	}
	e.sess = &session.Session{
		SessionID: runID,
		Builder:   e.builder,
		Turns:     []*turns.Turn{seed},
	}
	e.sessMu.Unlock()

	e.emittedMu.Lock()
	e.emitted = make(map[string]struct{})
	e.emittedMu.Unlock()

	log.Debug().Str("component", "engine_backend").Int("seed_blocks", len(t.Blocks)).Msg("Seed Turn loaded into conversation state")
	e.emitInitialEntities(t)
}

func (e *EngineBackend) snapshotForPrompt(prompt string) *turns.Turn {
	e.sessMu.RLock()
	sess := e.sess
	e.sessMu.RUnlock()

	var (
		base  *turns.Turn
		runID string
	)
	if sess != nil {
		base = sess.Latest()
		runID = sess.SessionID
	}

	seed := &turns.Turn{RunID: runID}
	if base != nil {
		seed = cloneTurn(base)
		if seed.RunID == "" {
			seed.RunID = runID
		}
	}
	if prompt != "" {
		turns.AppendBlock(seed, turns.NewUserTextBlock(prompt))
	}
	return seed
}

func cloneTurn(t *turns.Turn) *turns.Turn {
	if t == nil {
		return nil
	}
	return &turns.Turn{
		ID:       t.ID,
		RunID:    t.RunID,
		Blocks:   append([]turns.Block(nil), t.Blocks...),
		Metadata: t.Metadata.Clone(),
		Data:     t.Data.Clone(),
	}
}

// emitInitialEntities sends UI entities for existing blocks (system/user/assistant text)
// so the chat timeline reflects prior context when entering chat mode.
func (e *EngineBackend) emitInitialEntities(t *turns.Turn) {
	if e.program == nil || t == nil || len(t.Blocks) == 0 {
		return
	}
	for _, b := range t.Blocks {
		var role string
		var text string
		switch b.Kind {
		case turns.BlockKindUser:
			role = "user"
		case turns.BlockKindLLMText:
			role = "assistant"
		case turns.BlockKindSystem:
			continue
		case turns.BlockKindReasoning:
			continue
		case turns.BlockKindToolCall:
			continue
		case turns.BlockKindToolUse:
			continue
		case turns.BlockKindOther:
			continue
		}
		if s, ok := b.Payload[turns.PayloadKeyText].(string); ok {
			text = s
		}
		if role == "" || text == "" {
			continue
		}
		id := b.ID
		// Deduplicate entity emissions by block ID
		e.emittedMu.Lock()
		if _, seen := e.emitted[id]; seen {
			e.emittedMu.Unlock()
			continue
		}
		e.emitted[id] = struct{}{}
		e.emittedMu.Unlock()
		e.program.Send(timeline.UIEntityCreated{
			ID:        timeline.EntityID{LocalID: id, Kind: "llm_text"},
			Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
			Props:     map[string]any{"role": role, "text": text},
			StartedAt: time.Now(),
		})
		e.program.Send(timeline.UIEntityCompleted{
			ID:     timeline.EntityID{LocalID: id, Kind: "llm_text"},
			Result: map[string]any{"text": text},
		})
	}
}

// Interrupt attempts to cancel the current inference operation.
func (e *EngineBackend) Interrupt() {
	e.sessMu.RLock()
	sess := e.sess
	e.sessMu.RUnlock()
	if sess == nil {
		log.Warn().Msg("Engine is not running")
		return
	}
	if err := sess.CancelActive(); err != nil {
		log.Warn().Err(err).Msg("Engine is not running")
	}
}

// Kill forcefully cancels the current inference operation.
func (e *EngineBackend) Kill() {
	e.sessMu.RLock()
	sess := e.sess
	e.sessMu.RUnlock()
	if sess == nil {
		return
	}
	if err := sess.CancelActive(); err != nil {
		log.Debug().Err(err).Msg("Engine is not running")
	}
}

// IsFinished returns whether the engine is currently running an inference operation.
func (e *EngineBackend) IsFinished() bool {
	e.sessMu.RLock()
	sess := e.sess
	e.sessMu.RUnlock()
	return sess == nil || !sess.IsRunning()
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
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
				Props:     map[string]any{"role": "assistant", "text": "", "metadata": md.LLMInferenceData, "streaming": true},
				StartedAt: time.Now(),
			})
		case *events.EventPartialCompletion:
			// Update accumulated assistant text using the Completion field
			log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Int("delta_len", len(e_.Delta)).Int("completion_len", len(e_.Completion)).Msg("UIEntityUpdated (llm_text)")
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Patch:     map[string]any{"text": e_.Completion, "metadata": md.LLMInferenceData, "streaming": true},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventFinal:
			log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Int("text_len", len(e_.Text)).Msg("UIEntityCompleted (final)")
			p.Send(timeline.UIEntityCompleted{
				ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Result: map[string]any{"text": e_.Text, "metadata": md.LLMInferenceData},
			})
			// Mark streaming=false on completion by sending a final props update before BackendFinished
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Patch:     map[string]any{"streaming": false},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
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
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
			p.Send(boba_chat.BackendFinishedMsg{})
		case *events.EventError:
			log.Debug().Str("component", "step_forward").Str("entity_id", entityID).Msg("UIEntityCompleted (error)")
			p.Send(timeline.UIEntityCompleted{
				ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Result: map[string]any{"text": "**Error**\n\n" + e_.ErrorString},
			})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
			p.Send(boba_chat.BackendFinishedMsg{})
			// Tool-related events can be mapped to dedicated tool_call entities if desired
		case *events.EventInfo:
			if e_.Message == "thinking-started" {
				thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
				log.Debug().Str("component", "step_forward").Str("entity_id", thinkID.LocalID).Msg("UIEntityCreated (thinking)")
				p.Send(timeline.UIEntityCreated{
					ID:        thinkID,
					Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
					Props:     map[string]any{"role": "thinking", "text": "", "streaming": true},
					StartedAt: time.Now(),
				})
			}
			if e_.Message == "thinking-ended" {
				thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
				log.Debug().Str("component", "step_forward").Str("entity_id", thinkID.LocalID).Msg("UIEntityCompleted (thinking)")
				p.Send(timeline.UIEntityUpdated{
					ID:        thinkID,
					Patch:     map[string]any{"streaming": false},
					Version:   time.Now().UnixNano(),
					UpdatedAt: time.Now(),
				})
				p.Send(timeline.UIEntityCompleted{ID: thinkID})
			}
		case *events.EventThinkingPartial:
			thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
			log.Debug().Str("component", "step_forward").Str("entity_id", thinkID.LocalID).Int("completion_len", len(e_.Completion)).Msg("UIEntityUpdated (thinking)")
			p.Send(timeline.UIEntityUpdated{
				ID:        thinkID,
				Patch:     map[string]any{"text": e_.Completion, "streaming": true},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		}

		return nil
	}
}
