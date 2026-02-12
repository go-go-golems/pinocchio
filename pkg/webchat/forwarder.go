package webchat

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	gcompat "github.com/go-go-golems/pinocchio/pkg/geppettocompat"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type TimelineEvent struct {
	Type      string            `json:"type"`
	EntityID  string            `json:"entityId"`
	Kind      string            `json:"kind,omitempty"`
	Renderer  map[string]string `json:"renderer,omitempty"`
	Props     map[string]any    `json:"props,omitempty"`
	Patch     map[string]any    `json:"patch,omitempty"`
	Result    map[string]any    `json:"result,omitempty"`
	StartedAt int64             `json:"startedAt,omitempty"`
	UpdatedAt int64             `json:"updatedAt,omitempty"`
	Version   int64             `json:"version,omitempty"`
	Flags     map[string]any    `json:"flags,omitempty"`
}

type cachedToolCall struct {
	Name     string
	RawInput string
	InputObj map[string]any
}

var toolCallCache sync.Map

func wrapSem(ev map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"sem": true, "event": ev})
	return b
}

// SemanticEventsFromEvent converts a Geppetto event into SEM frames for the UI.
func SemanticEventsFromEvent(e events.Event) [][]byte {
	md := e.Metadata()
	log.Debug().Str("component", "web_forwarder").Str("event_type", fmt.Sprintf("%T", e)).Str("event_id", md.ID.String()).Str("run_id", gcompat.EventSessionID(md)).Str("turn_id", md.TurnID).Msg("received event (SEM)")
	switch ev := e.(type) {
	case *events.EventLog:
		lvl := ev.Level
		if lvl == "" {
			lvl = "info"
		}
		id := md.ID.String()
		if md.ID == uuid.Nil {
			id = "log-" + uuid.NewString()
		}
		sem := map[string]any{"type": "log", "id": id, "level": lvl, "message": ev.Message}
		if len(ev.Fields) > 0 {
			sem["fields"] = ev.Fields
		}
		b := wrapSem(sem)
		return [][]byte{b}
	case *events.EventPartialCompletionStart:
		id := md.ID.String()
		if md.ID == uuid.Nil {
			id = "llm-" + uuid.NewString()
		}
		sem := map[string]any{"type": "llm.start", "id": id, "role": "assistant", "metadata": md.LLMInferenceData}
		return [][]byte{wrapSem(sem)}
	case *events.EventPartialCompletion:
		id := md.ID.String()
		if md.ID == uuid.Nil {
			id = "llm-" + uuid.NewString()
		}
		sem := map[string]any{"type": "llm.delta", "id": id, "delta": ev.Delta, "cumulative": ev.Completion, "metadata": md.LLMInferenceData}
		return [][]byte{wrapSem(sem)}
	case *events.EventFinal:
		id := md.ID.String()
		if md.ID == uuid.Nil {
			id = "llm-" + uuid.NewString()
		}
		sem := map[string]any{"type": "llm.final", "id": id, "text": ev.Text, "metadata": md.LLMInferenceData}
		return [][]byte{wrapSem(sem)}
	case *events.EventInterrupt:
		if intr, ok := events.ToTypedEvent[events.EventInterrupt](e); ok {
			id := md.ID.String()
			if md.ID == uuid.Nil {
				id = "llm-" + uuid.NewString()
			}
			sem := map[string]any{"type": "llm.final", "id": id, "text": intr.Text}
			return [][]byte{wrapSem(sem)}
		}
	case *events.EventToolCall:
		var inputObj map[string]any
		if ev.ToolCall.Input != "" {
			_ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
		}
		toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
		sem := map[string]any{"type": "tool.start", "id": ev.ToolCall.ID, "name": ev.ToolCall.Name, "input": inputObj}
		return [][]byte{wrapSem(sem)}
	case *events.EventToolCallExecute:
		var inputObj map[string]any
		if ev.ToolCall.Input != "" {
			_ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
		}
		toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
		sem := map[string]any{"type": "tool.delta", "id": ev.ToolCall.ID, "patch": map[string]any{"exec": true, "input": inputObj}}
		return [][]byte{wrapSem(sem)}
	case *events.EventToolResult:
		var frames [][]byte
		if v, ok := toolCallCache.Load(ev.ToolResult.ID); ok {
			if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
				sem := map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "result": ev.ToolResult.Result, "customKind": "calc_result"}
				frames = append(frames, wrapSem(sem))
				frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID}))
				return frames
			}
		}
		frames = append(frames, wrapSem(map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "result": ev.ToolResult.Result}))
		frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID}))
		return frames
	case *events.EventToolCallExecutionResult:
		var frames [][]byte
		if v, ok := toolCallCache.Load(ev.ToolResult.ID); ok {
			if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
				sem := map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "result": ev.ToolResult.Result, "customKind": "calc_result"}
				frames = append(frames, wrapSem(sem))
				frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID}))
				return frames
			}
		}
		frames = append(frames, wrapSem(map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "result": ev.ToolResult.Result}))
		frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID}))
		return frames
	case *events.EventAgentModeSwitch:
		props := map[string]any{"title": ev.Message}
		for k, v := range ev.Data {
			props[k] = v
		}
		localID := "agentmode-" + md.TurnID + "-" + uuid.NewString()
		return [][]byte{wrapSem(map[string]any{"type": "agent.mode", "id": localID, "title": ev.Message, "data": props})}
	}
	log.Debug().Str("component", "web_forwarder").Msg("no semantic mapping for event; dropping")
	return nil
}

// TimelineEventsFromEvent retained for compatibility if needed by UI
func TimelineEventsFromEvent(e events.Event) [][]byte {
	md := e.Metadata()
	now := time.Now()
	wrap := func(te TimelineEvent) []byte { b, _ := json.Marshal(map[string]any{"tl": true, "event": te}); return b }
	switch ev := e.(type) {
	case *events.EventLog:
		localID := md.ID.String()
		if md.ID == uuid.Nil {
			localID = "log-" + uuid.NewString()
		}
		props := map[string]any{"level": ev.Level, "message": ev.Message}
		if len(ev.Fields) > 0 {
			props["fields"] = ev.Fields
		}
		return [][]byte{wrap(TimelineEvent{Type: "created", EntityID: localID, Kind: "log_event", Renderer: map[string]string{"kind": "log_event"}, Props: props, StartedAt: now.UnixMilli()}), wrap(TimelineEvent{Type: "completed", EntityID: localID, Result: map[string]any{"message": ev.Message}})}
	}
	return nil
}
