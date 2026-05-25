package agent

import (
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	"github.com/go-go-golems/geppetto/pkg/events"
)

// MakeUIForwarder returns a Watermill handler that forwards Geppetto events to the Bubble Tea program p
// without signaling backend finish on provider events (the tool loop backend is responsible for emitting
// BackendFinishedMsg when the loop completes).
func MakeUIForwarder(p *tea.Program) func(msg *message.Message) error {
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
		case *events.EventTextSegmentStarted:
			log.Debug().Str("event", "text_segment_started").Str("session_id", md.SessionID).Str("inference_id", md.InferenceID).Str("turn_id", md.TurnID).Str("message_id", md.ID.String()).Msg("forward: text segment start")
			p.Send(timeline.UIEntityCreated{
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
				Props:     map[string]any{"role": "assistant", "text": "", "metadata": md.LLMInferenceData, "streaming": true},
				StartedAt: time.Now(),
			})
			if entityID == "00000000-0000-0000-0000-000000000000" {
				log.Debug().Msg("forward: text segment start has zero message_id (check event metadata assignment)")
			}
		case *events.EventTextDelta:
			log.Debug().Str("event", "text_delta").Str("session_id", md.SessionID).Str("inference_id", md.InferenceID).Str("turn_id", md.TurnID).Int("delta_len", len(e_.Delta)).Int("text_len", len(e_.Text)).Msg("forward: text delta")
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Patch:     map[string]any{"text": e_.Text, "metadata": md.LLMInferenceData, "streaming": true},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventTextSegmentFinished:
			log.Debug().Str("event", "text_segment_finished").Str("session_id", md.SessionID).Str("inference_id", md.InferenceID).Str("turn_id", md.TurnID).Int("text_len", len(e_.Text)).Msg("forward: text segment finished")
			p.Send(timeline.UIEntityCompleted{
				ID:     timeline.EntityID{LocalID: entityID, Kind: "llm_text"},
				Result: map[string]any{"text": e_.Text, "metadata": md.LLMInferenceData},
			})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventReasoningSegmentStarted:
			// Render reasoning streams as their own timeline entity.
			thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
			p.Send(timeline.UIEntityCreated{
				ID:        thinkID,
				Renderer:  timeline.RendererDescriptor{Kind: "llm_text"},
				Props:     map[string]any{"role": "thinking", "text": "", "streaming": true},
				StartedAt: time.Now(),
			})
		case *events.EventReasoningDelta:
			thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
			p.Send(timeline.UIEntityUpdated{
				ID:        thinkID,
				Patch:     map[string]any{"text": e_.Text, "streaming": true},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventReasoningSegmentFinished:
			thinkID := timeline.EntityID{LocalID: entityID + ":thinking", Kind: "llm_text"}
			p.Send(timeline.UIEntityUpdated{ID: thinkID, Patch: map[string]any{"text": e_.Text, "streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
			p.Send(timeline.UIEntityCompleted{ID: thinkID})
		case *events.EventInterrupt:
			log.Debug().Str("event", "interrupt").Str("session_id", md.SessionID).Str("inference_id", md.InferenceID).Str("turn_id", md.TurnID).Msg("forward: interrupt")
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Result: map[string]any{"text": e_.Text}})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventError:
			log.Debug().Str("event", "error").Str("session_id", md.SessionID).Str("inference_id", md.InferenceID).Str("turn_id", md.TurnID).Str("err", e_.ErrorString).Msg("forward: error")
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Result: map[string]any{"text": "**Error**\n\n" + e_.ErrorString}})
			p.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: entityID, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		case *events.EventToolCallStarted:
			log.Debug().Str("event", "tool_call_started").Str("tool_id", e_.ToolCallID).Str("name", e_.ToolName).Msg("forward: tool_call_started")
			p.Send(timeline.UIEntityCreated{
				ID:        timeline.EntityID{LocalID: e_.ToolCallID, Kind: "tool_call"},
				Renderer:  timeline.RendererDescriptor{Kind: "tool_call"},
				Props:     map[string]any{"name": e_.ToolName},
				StartedAt: time.Now(),
			})
		case *events.EventToolCallRequested:
			log.Debug().Str("event", "tool_call_requested").Str("tool_id", e_.ToolCallID).Str("name", e_.ToolName).Int("input_len", len(e_.Input)).Msg("forward: tool_call_requested")
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: e_.ToolCallID, Kind: "tool_call"},
				Patch:     map[string]any{"name": e_.ToolName, "input": e_.Input},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventToolExecutionStarted:
			log.Debug().Str("event", "tool_exec").Str("tool_id", e_.ToolCallID).Str("name", e_.ToolName).Msg("forward: tool_exec")
			p.Send(timeline.UIEntityUpdated{
				ID:        timeline.EntityID{LocalID: e_.ToolCallID, Kind: "tool_call"},
				Patch:     map[string]any{"exec": true, "input": e_.Input},
				Version:   time.Now().UnixNano(),
				UpdatedAt: time.Now(),
			})
		case *events.EventToolResultReady:
			log.Debug().Str("event", "tool_result_ready").Str("tool_id", e_.ToolCallID).Int("result_len", len(e_.Result)).Msg("forward: tool_result_ready")
			p.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: e_.ToolCallID + ":result", Kind: "tool_call_result"}, Renderer: timeline.RendererDescriptor{Kind: "tool_call_result"}, Props: map[string]any{"result": e_.Result}})
			p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: e_.ToolCallID + ":result", Kind: "tool_call_result"}})
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
