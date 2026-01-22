package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
	"github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// ToolLoopBackend runs the tool-calling loop across turns and emits BackendFinishedMsg when done.
type ToolLoopBackend struct {
	reg  *tools.InMemoryToolRegistry
	sink events.EventSink
	hook toolhelpers.SnapshotHook

	sess *session.Session
}

func NewToolLoopBackend(eng engine.Engine, reg *tools.InMemoryToolRegistry, sink events.EventSink, hook toolhelpers.SnapshotHook) *ToolLoopBackend {
	cfg := toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60 * time.Second)
	sess := session.NewSession()
	sess.Builder = &session.ToolLoopEngineBuilder{
		Base:         eng,
		Registry:     reg,
		ToolConfig:   &cfg,
		EventSinks:   []events.EventSink{sink},
		SnapshotHook: hook,
	}
	return &ToolLoopBackend{reg: reg, sink: sink, hook: hook, sess: sess}
}

func (b *ToolLoopBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
	if b == nil || b.sess == nil {
		return nil, errors.New("backend not initialized")
	}
	if b.sess.IsRunning() {
		return nil, errors.New("already running")
	}

	seed := snapshotForPrompt(b.sess, prompt)
	b.sess.Append(seed)

	handle, err := b.sess.StartInference(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "start inference")
	}

	return func() tea.Msg {
		_, waitErr := handle.Wait()
		if waitErr != nil {
			log.Error().Err(waitErr).Msg("tool loop failed")
		}
		return boba_chat.BackendFinishedMsg{}
	}, nil
}

func (b *ToolLoopBackend) Interrupt() {
	if b != nil && b.sess != nil {
		_ = b.sess.CancelActive()
	}
}

func (b *ToolLoopBackend) Kill() {
	if b != nil && b.sess != nil {
		_ = b.sess.CancelActive()
	}
}

func (b *ToolLoopBackend) IsFinished() bool {
	return b == nil || b.sess == nil || !b.sess.IsRunning()
}

// CurrentTurn returns the latest in-memory turn snapshot for this backend.
// Callers may mutate the returned Turn (e.g. seed Turn.Data) before starting inference.
func (b *ToolLoopBackend) CurrentTurn() *turns.Turn {
	if b == nil || b.sess == nil {
		return nil
	}
	return b.sess.Latest()
}

func snapshotForPrompt(sess *session.Session, prompt string) *turns.Turn {
	if sess == nil {
		t := &turns.Turn{}
		if prompt != "" {
			turns.AppendBlock(t, turns.NewUserTextBlock(prompt))
		}
		return t
	}
	base := sess.Latest()
	seed := &turns.Turn{}
	if base != nil {
		seed = cloneTurn(base)
	}
	if sess.SessionID != "" {
		if _, ok, err := turns.KeyTurnMetaSessionID.Get(seed.Metadata); err != nil || !ok {
			_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sess.SessionID)
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
		Blocks:   append([]turns.Block(nil), t.Blocks...),
		Metadata: t.Metadata.Clone(),
		Data:     t.Data.Clone(),
	}
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
		case *events.EventLog:
			// Render logs as dedicated timeline entries with unobtrusive gray styling
			log.Debug().Str("event", "log").Str("level", e_.Level).Str("message", e_.Message).Msg("forward: log")
			localID := fmt.Sprintf("log-%s-%d", md.TurnID, time.Now().UnixNano())
			props := map[string]any{"level": e_.Level, "message": e_.Message, "metadata": md}
			if len(e_.Fields) > 0 {
				props["fields"] = e_.Fields
			}
			p.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: localID, Kind: "log_event"}, Renderer: timeline.RendererDescriptor{Kind: "log_event"}, Props: props})
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: localID, Kind: "log_event"}})
		case *events.EventPartialCompletionStart:
			log.Debug().Str("event", "partial_start").Str("run_id", md.RunID).Str("turn_id", md.TurnID).Str("message_id", md.ID.String()).Msg("forward: start")
			p.Send(timeline.UIEntityCreated{
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
				Props:     map[string]any{"role": "assistant", "text": "", "metadata": md.LLMInferenceData, "streaming": true},
				StartedAt: time.Now(),
			})
			if entityID == "00000000-0000-0000-0000-000000000000" {
				log.Debug().Msg("forward: start has zero message_id (check event metadata assignment)")
			}
		case *events.EventPartialCompletion:
			log.Debug().Str("event", "partial").Str("run_id", md.RunID).Str("turn_id", md.TurnID).Int("delta_len", len(e_.Delta)).Int("completion_len", len(e_.Completion)).Msg("forward: partial")
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Patch:     map[string]any{"text": e_.Completion, "metadata": md.LLMInferenceData, "streaming": true},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventFinal:
			log.Debug().Str("event", "final").Str("run_id", md.RunID).Str("turn_id", md.TurnID).Int("text_len", len(e_.Text)).Msg("forward: final")
			p.Send(timeline.UIEntityCompleted{
				ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Result: map[string]any{"text": e_.Text, "metadata": md.LLMInferenceData},
			})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventInfo:
			// Render thinking streams as their own timeline entity
			if e_.Message == "thinking-started" {
				thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
				p.Send(timeline.UIEntityCreated{
					ID:        thinkID,
					Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
					Props:     map[string]any{"role": "thinking", "text": "", "streaming": true},
					StartedAt: time.Now(),
				})
			}
			if e_.Message == "thinking-ended" {
				thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
				p.Send(timeline.UIEntityUpdated{ID: thinkID, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
				p.Send(timeline.UIEntityCompleted{ID: thinkID})
			}
		case *events.EventThinkingPartial:
			// Stream reasoning summary deltas into the thinking entity
			thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
			p.Send(timeline.UIEntityUpdated{
				ID:        thinkID,
				Patch:     map[string]any{"text": e_.Completion, "streaming": true},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventInterrupt:
			log.Debug().Str("event", "interrupt").Str("run_id", md.RunID).Str("turn_id", md.TurnID).Msg("forward: interrupt")
			intr, ok := events.ToTypedEvent[events.EventInterrupt](e)
			if !ok {
				return errors.New("payload is not of type EventInterrupt")
			}
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Result: map[string]any{"text": intr.Text}})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventError:
			log.Debug().Str("event", "error").Str("run_id", md.RunID).Str("turn_id", md.TurnID).Str("err", e_.ErrorString).Msg("forward: error")
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Result: map[string]any{"text": "**Error**\n\n" + e_.ErrorString}})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventToolCall:
			log.Debug().Str("event", "tool_call").Str("tool_id", e_.ToolCall.ID).Str("name", e_.ToolCall.Name).Int("input_len", len(e_.ToolCall.Input)).Msg("forward: tool_call")
			// Render tool call using dedicated tool_call renderer
			p.Send(timeline.UIEntityCreated{
				ID:        timeline.EntityID{LocalID: e_.ToolCall.ID, Kind: "tool_call"},
				Renderer:  timeline.RendererDescriptor{Kind: "tool_call"},
				Props:     map[string]any{"name": e_.ToolCall.Name, "input": e_.ToolCall.Input},
				StartedAt: time.Now(),
			})
		case *events.EventToolCallExecute:
			log.Debug().Str("event", "tool_exec").Str("tool_id", e_.ToolCall.ID).Str("name", e_.ToolCall.Name).Msg("forward: tool_exec")
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: e_.ToolCall.ID, Kind: "tool_call"},
				Patch:     map[string]any{"exec": true, "input": e_.ToolCall.Input},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventToolResult:
			log.Debug().Str("event", "tool_result").Str("tool_id", e_.ToolResult.ID).Int("result_len", len(e_.ToolResult.Result)).Msg("forward: tool_result")
			p.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: e_.ToolResult.ID + ":result", Kind: "tool_call_result"}, Renderer: timeline.RendererDescriptor{Kind: "tool_call_result"}, Props: map[string]any{"result": e_.ToolResult.Result}})
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: e_.ToolResult.ID + ":result", Kind: "tool_call_result"}})
		case *events.EventToolCallExecutionResult:
			log.Debug().Str("event", "tool_exec_result").Str("tool_id", e_.ToolResult.ID).Int("result_len", len(e_.ToolResult.Result)).Msg("forward: tool_exec_result")
			p.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: e_.ToolResult.ID + ":result", Kind: "tool_call_result"}, Renderer: timeline.RendererDescriptor{Kind: "tool_call_result"}, Props: map[string]any{"result": e_.ToolResult.Result}})
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: e_.ToolResult.ID + ":result", Kind: "tool_call_result"}})
		case *events.EventAgentModeSwitch:
			// Expect Data to contain keys: from, to, analysis
			log.Debug().Str("event", "agent_mode").Interface("data", e_.Data).Msg("forward: agent_mode")
			props := map[string]any{"title": e_.Message}
			for k, v := range e_.Data {
				props[k] = v
			}
			// Generate unique local id to avoid collisions when message_id is zero or reused
			localID := fmt.Sprintf("agentmode-%s-%d", md.TurnID, time.Now().UnixNano())
			p.Send(timeline.UIEntityCreated{
				ID:       timeline.EntityID{LocalID: localID, Kind: "agent_mode"},
				Renderer: timeline.RendererDescriptor{Kind: "agent_mode"},
				Props:    props,
			})
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: localID, Kind: "agent_mode"}})
			// Backend-driven deletion example (unused by default):
			// Emit timeline.UIEntityDeleted{ID: timeline.EntityID{LocalID: someID, Kind: someKind}} to remove an entity.
			// The controller will adjust selection accordingly.
		case *events.EventWebSearchStarted:
			// Aggregate web_search events into a single entity per ItemID
			id := timeline.EntityID{LocalID: e_.ItemID, Kind: "web_search"}
			props := map[string]any{"status": "searching", "opened_urls": []string{}, "results": []map[string]any{}}
			if e_.Query != "" {
				props["query"] = e_.Query
			}
			p.Send(timeline.UIEntityCreated{ID: id, Renderer: timeline.RendererDescriptor{Kind: "web_search"}, Props: props, StartedAt: time.Now()})
		case *events.EventWebSearchSearching:
			id := timeline.EntityID{LocalID: e_.ItemID, Kind: "web_search"}
			p.Send(timeline.UIEntityUpdated{ID: id, Patch: map[string]any{"status": "searching"}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventWebSearchOpenPage:
			id := timeline.EntityID{LocalID: e_.ItemID, Kind: "web_search"}
			// Use append semantic; renderer will merge
			p.Send(timeline.UIEntityUpdated{ID: id, Patch: map[string]any{"opened_urls.append": e_.URL}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventWebSearchDone:
			id := timeline.EntityID{LocalID: e_.ItemID, Kind: "web_search"}
			p.Send(timeline.UIEntityUpdated{ID: id, Patch: map[string]any{"status": "completed"}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
			p.Send(timeline.UIEntityCompleted{ID: id})
		case *events.EventToolSearchResults:
			if e_.Tool == "web_search" {
				id := timeline.EntityID{LocalID: e_.ItemID, Kind: "web_search"}
				// Results is []events.SearchResult; convert to []map[string]any for renderer
				conv := make([]map[string]any, 0, len(e_.Results))
				for _, r := range e_.Results {
					m := map[string]any{"url": r.URL, "title": r.Title, "snippet": r.Snippet}
					if len(r.Extensions) > 0 {
						m["ext"] = r.Extensions
					}
					conv = append(conv, m)
				}
				p.Send(timeline.UIEntityUpdated{ID: id, Patch: map[string]any{"results.append": conv}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
			}
		}
		return nil
	}
}
