package backend

import (
    "encoding/json"
    "fmt"
    "time"
    "sync"

    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/google/uuid"
    "github.com/rs/zerolog/log"
)

// TimelineEvent is a web UI timeline lifecycle message
type TimelineEvent struct {
    Type      string                 `json:"type"`
    EntityID  string                 `json:"entityId"`
    Kind      string                 `json:"kind,omitempty"`
    Renderer  map[string]string      `json:"renderer,omitempty"`
    Props     map[string]any         `json:"props,omitempty"`
    Patch     map[string]any         `json:"patch,omitempty"`
    Result    map[string]any         `json:"result,omitempty"`
    StartedAt int64                  `json:"startedAt,omitempty"`
    UpdatedAt int64                  `json:"updatedAt,omitempty"`
    Version   int64                  `json:"version,omitempty"`
    Flags     map[string]any         `json:"flags,omitempty"`
}

// cache tool call inputs by ID to enrich tool results
type cachedToolCall struct {
    Name     string
    RawInput string
    InputObj map[string]any
}

var toolCallCache sync.Map // key: ToolCall.ID -> cachedToolCall

// wrapSem wraps a semantic event payload into the SEM envelope { sem: true, event: {...} }
func wrapSem(ev map[string]any) []byte {
    payload := map[string]any{"sem": true, "event": ev}
    b, _ := json.Marshal(payload)
    return b
}

// SemanticEventsFromEvent converts a Geppetto event into one or multiple semantic messages (encoded as JSON ready to send)
func SemanticEventsFromEvent(e events.Event) [][]byte {
    md := e.Metadata()

    // Debug: received event
    log.Debug().
        Str("component", "web_forwarder").
        Str("event_type", fmt.Sprintf("%T", e)).
        Str("event_id", md.ID.String()).
        Str("run_id", md.RunID).
        Str("turn_id", md.TurnID).
        Msg("received event (SEM)")

    switch ev := e.(type) {
    case *events.EventLog:
        lvl := ev.Level
        if lvl == "" { lvl = "info" }
        id := md.ID.String()
        if md.ID == uuid.Nil { id = "log-" + uuid.NewString() }
        sem := map[string]any{
            "type":    "log",
            "id":      id,
            "level":   lvl,
            "message": ev.Message,
        }
        if len(ev.Fields) > 0 { sem["fields"] = ev.Fields }
        return [][]byte{ wrapSem(sem) }

    case *events.EventPartialCompletionStart:
        id := md.ID.String()
        if md.ID == uuid.Nil { id = "llm-" + uuid.NewString() }
        return [][]byte{ wrapSem(map[string]any{
            "type":     "llm.start",
            "id":       id,
            "role":     "assistant",
            "metadata": md.LLMInferenceData,
        }) }

    case *events.EventPartialCompletion:
        id := md.ID.String()
        if md.ID == uuid.Nil { id = "llm-" + uuid.NewString() }
        return [][]byte{ wrapSem(map[string]any{
            "type":       "llm.delta",
            "id":         id,
            "delta":      ev.Delta,
            "cumulative": ev.Completion,
            "metadata":   md.LLMInferenceData,
        }) }

    case *events.EventFinal:
        id := md.ID.String()
        if md.ID == uuid.Nil { id = "llm-" + uuid.NewString() }
        return [][]byte{ wrapSem(map[string]any{
            "type":     "llm.final",
            "id":       id,
            "text":     ev.Text,
            "metadata": md.LLMInferenceData,
        }) }

    case *events.EventInterrupt:
        if intr, ok := events.ToTypedEvent[events.EventInterrupt](e); ok {
            id := md.ID.String()
            if md.ID == uuid.Nil { id = "llm-" + uuid.NewString() }
            return [][]byte{ wrapSem(map[string]any{
                "type": "llm.final",
                "id":   id,
                "text": intr.Text,
            }) }
        }

    case *events.EventToolCall:
        // cache input for future result enrichment
        var inputObj map[string]any
        if ev.ToolCall.Input != "" {
            _ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
        }
        toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
        return [][]byte{ wrapSem(map[string]any{
            "type":  "tool.start",
            "id":    ev.ToolCall.ID,
            "name":  ev.ToolCall.Name,
            "input": inputObj,
        }) }

    case *events.EventToolCallExecute:
        var inputObj map[string]any
        if ev.ToolCall.Input != "" { _ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj) }
        toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
        return [][]byte{ wrapSem(map[string]any{
            "type":  "tool.delta",
            "id":    ev.ToolCall.ID,
            "patch": map[string]any{"exec": true, "input": inputObj},
        }) }

    case *events.EventToolResult:
        var frames [][]byte
        if v, ok := toolCallCache.Load(ev.ToolResult.ID); ok {
            if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
                frames = append(frames, wrapSem(map[string]any{
                    "type":       "tool.result",
                    "id":         ev.ToolResult.ID,
                    "result":     ev.ToolResult.Result,
                    "customKind": "calc_result",
                }))
                frames = append(frames, wrapSem(map[string]any{
                    "type": "tool.done",
                    "id":   ev.ToolResult.ID,
                }))
                return frames
            }
        }
        frames = append(frames, wrapSem(map[string]any{
            "type":   "tool.result",
            "id":     ev.ToolResult.ID,
            "result": ev.ToolResult.Result,
        }))
        frames = append(frames, wrapSem(map[string]any{
            "type": "tool.done",
            "id":   ev.ToolResult.ID,
        }))
        return frames

    case *events.EventToolCallExecutionResult:
        var frames [][]byte
        if v, ok := toolCallCache.Load(ev.ToolResult.ID); ok {
            if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
                frames = append(frames, wrapSem(map[string]any{
                    "type":       "tool.result",
                    "id":         ev.ToolResult.ID,
                    "result":     ev.ToolResult.Result,
                    "customKind": "calc_result",
                }))
                frames = append(frames, wrapSem(map[string]any{
                    "type": "tool.done",
                    "id":   ev.ToolResult.ID,
                }))
                return frames
            }
        }
        frames = append(frames, wrapSem(map[string]any{
            "type":   "tool.result",
            "id":     ev.ToolResult.ID,
            "result": ev.ToolResult.Result,
        }))
        frames = append(frames, wrapSem(map[string]any{
            "type": "tool.done",
            "id":   ev.ToolResult.ID,
        }))
        return frames

    case *events.EventAgentModeSwitch:
        props := map[string]any{"title": ev.Message}
        for k, v := range ev.Data { props[k] = v }
        localID := "agentmode-" + md.TurnID + "-" + uuid.NewString()
        return [][]byte{
            wrapSem(map[string]any{"type": "agent.mode", "id": localID, "title": ev.Message, "data": props}),
        }
    }
    log.Debug().Str("component", "web_forwarder").Msg("no semantic mapping for event; dropping")
    return nil
}

// TimelineEventsFromEvent converts a Geppetto event into one or multiple timeline lifecycle messages (encoded as JSON ready to send)
func TimelineEventsFromEvent(e events.Event) [][]byte {
    md := e.Metadata()
    now := time.Now()
    wrap := func(te TimelineEvent) []byte {
        payload := map[string]any{"tl": true, "event": te}
        b, _ := json.Marshal(payload)
        return b
    }

    // Debug: received event
    log.Debug().
        Str("component", "web_forwarder").
        Str("event_type", fmt.Sprintf("%T", e)).
        Str("event_id", md.ID.String()).
        Str("run_id", md.RunID).
        Str("turn_id", md.TurnID).
        Msg("received event")

    switch ev := e.(type) {
    case *events.EventLog:
        log.Debug().Str("component", "web_forwarder").Str("kind", "log_event").Str("level", ev.Level).Msg("mapping to timeline created+completed")
        localID := md.ID.String()
        if md.ID == uuid.Nil {
            localID = "log-" + uuid.NewString()
            log.Warn().Str("component", "web_forwarder").Msg("log event has zero UUID; generating local timeline id")
        }
        props := map[string]any{"level": ev.Level, "message": ev.Message}
        if len(ev.Fields) > 0 { props["fields"] = ev.Fields }
        return [][]byte{
            wrap(TimelineEvent{Type: "created", EntityID: localID, Kind: "log_event", Renderer: map[string]string{"kind":"log_event"}, Props: props, StartedAt: now.UnixMilli()}),
            wrap(TimelineEvent{Type: "completed", EntityID: localID, Result: map[string]any{"message": ev.Message}}),
        }
    case *events.EventPartialCompletionStart:
        log.Debug().Str("component", "web_forwarder").Str("kind", "llm_text").Msg("mapping start to timeline created")
        idStr := md.ID.String()
        if md.ID == uuid.Nil {
            idStr = "llm-" + uuid.NewString()
            log.Warn().Str("component", "web_forwarder").Msg("llm start event has zero UUID; generating local timeline id")
        }
        return [][]byte{
            wrap(TimelineEvent{Type: "created", EntityID: idStr, Kind: "llm_text", Renderer: map[string]string{"kind":"llm_text"}, Props: map[string]any{"role":"assistant", "text":"", "metadata": md.LLMInferenceData, "streaming": true}, StartedAt: now.UnixMilli()}),
        }
    case *events.EventPartialCompletion:
        log.Debug().Str("component", "web_forwarder").Str("kind", "llm_text").Int("delta_len", len(ev.Delta)).Msg("mapping partial to timeline updated")
        idStr := md.ID.String()
        if md.ID == uuid.Nil {
            idStr = "llm-" + uuid.NewString()
            log.Warn().Str("component", "web_forwarder").Msg("llm partial event has zero UUID; generating local timeline id")
        }
        return [][]byte{
            wrap(TimelineEvent{Type: "updated", EntityID: idStr, Patch: map[string]any{"text": ev.Completion, "metadata": md.LLMInferenceData, "streaming": true}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
        }
    case *events.EventFinal:
        log.Debug().Str("component", "web_forwarder").Str("kind", "llm_text").Int("text_len", len(ev.Text)).Msg("mapping final to timeline completed+updated")
        idStr := md.ID.String()
        if md.ID == uuid.Nil {
            idStr = "llm-" + uuid.NewString()
            log.Warn().Str("component", "web_forwarder").Msg("llm final event has zero UUID; generating local timeline id")
        }
        return [][]byte{
            wrap(TimelineEvent{Type: "completed", EntityID: idStr, Result: map[string]any{"text": ev.Text, "metadata": md.LLMInferenceData}}),
            wrap(TimelineEvent{Type: "updated", EntityID: idStr, Patch: map[string]any{"streaming": false}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
        }
    case *events.EventInterrupt:
        intr, ok := events.ToTypedEvent[events.EventInterrupt](e)
        if ok {
            log.Debug().Str("component", "web_forwarder").Str("kind", "llm_text").Msg("mapping interrupt to timeline completed+updated")
            idStr := md.ID.String()
            if md.ID == uuid.Nil {
                idStr = "llm-" + uuid.NewString()
                log.Warn().Str("component", "web_forwarder").Msg("llm interrupt event has zero UUID; generating local timeline id")
            }
            return [][]byte{
                wrap(TimelineEvent{Type: "completed", EntityID: idStr, Result: map[string]any{"text": intr.Text}}),
                wrap(TimelineEvent{Type: "updated", EntityID: idStr, Patch: map[string]any{"streaming": false}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
            }
        }
    case *events.EventToolCall:
        log.Debug().Str("component", "web_forwarder").Str("kind", "tool_call").Str("name", ev.ToolCall.Name).Msg("mapping tool_call to timeline created")
        // cache input for future result enrichment
        var inputObj map[string]any
        if ev.ToolCall.Input != "" {
            _ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
        }
        toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
        log.Info().Str("component", "web_forwarder").Str("tool_id", ev.ToolCall.ID).Str("tool_name", ev.ToolCall.Name).Interface("input", inputObj).Msg("cached tool_call input")
        return [][]byte{
            wrap(TimelineEvent{Type: "created", EntityID: ev.ToolCall.ID, Kind: "tool_call", Renderer: map[string]string{"kind":"tool_call"}, Props: map[string]any{"name": ev.ToolCall.Name, "input": ev.ToolCall.Input}, StartedAt: now.UnixMilli()}),
        }
    case *events.EventToolCallExecute:
        log.Debug().Str("component", "web_forwarder").Str("kind", "tool_call").Str("name", ev.ToolCall.Name).Msg("mapping tool_exec to timeline updated")
        // update cache too
        var inputObj map[string]any
        if ev.ToolCall.Input != "" {
            _ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
        }
        toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
        return [][]byte{
            wrap(TimelineEvent{Type: "updated", EntityID: ev.ToolCall.ID, Patch: map[string]any{"exec": true, "input": ev.ToolCall.Input}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
        }
    case *events.EventToolResult:
        log.Debug().Str("component", "web_forwarder").Str("kind", "tool_call_result").Msg("mapping tool_result to timeline created+completed")
        // Check tool name to decide generic vs custom emission
        if v, ok := toolCallCache.Load(ev.ToolResult.ID); ok {
            if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
                // Emit ONLY custom calc_result and clear exec
                props := map[string]any{"name": ctc.Name, "result": ev.ToolResult.Result}
                if ctc.InputObj != nil { for k, vv := range ctc.InputObj { props[k] = vv } }
                customID := ev.ToolResult.ID + ":custom"
                return [][]byte{
                    wrap(TimelineEvent{Type: "created", EntityID: customID, Kind: "calc_result", Renderer: map[string]string{"kind": "calc_result"}, Props: props}),
                    wrap(TimelineEvent{Type: "completed", EntityID: customID}),
                    wrap(TimelineEvent{Type: "updated", EntityID: ev.ToolResult.ID, Patch: map[string]any{"exec": false}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
                }
            }
        }
        // Default: emit generic tool_call_result and clear exec
        return [][]byte{
            wrap(TimelineEvent{Type: "created", EntityID: ev.ToolResult.ID + ":result", Kind: "tool_call_result", Renderer: map[string]string{"kind":"tool_call_result"}, Props: map[string]any{"result": ev.ToolResult.Result}}),
            wrap(TimelineEvent{Type: "completed", EntityID: ev.ToolResult.ID + ":result"}),
            wrap(TimelineEvent{Type: "updated", EntityID: ev.ToolResult.ID, Patch: map[string]any{"exec": false}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
        }
    case *events.EventToolCallExecutionResult:
        log.Debug().Str("component", "web_forwarder").Str("kind", "tool_call_result").Msg("mapping tool_exec_result to timeline created+completed")
        if v, ok := toolCallCache.Load(ev.ToolResult.ID); ok {
            if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
                props := map[string]any{"name": ctc.Name, "result": ev.ToolResult.Result}
                if ctc.InputObj != nil { for k, vv := range ctc.InputObj { props[k] = vv } }
                customID := ev.ToolResult.ID + ":custom"
                return [][]byte{
                    wrap(TimelineEvent{Type: "created", EntityID: customID, Kind: "calc_result", Renderer: map[string]string{"kind": "calc_result"}, Props: props}),
                    wrap(TimelineEvent{Type: "completed", EntityID: customID}),
                    wrap(TimelineEvent{Type: "updated", EntityID: ev.ToolResult.ID, Patch: map[string]any{"exec": false}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
                }
            }
        }
        return [][]byte{
            wrap(TimelineEvent{Type: "created", EntityID: ev.ToolResult.ID + ":result", Kind: "tool_call_result", Renderer: map[string]string{"kind":"tool_call_result"}, Props: map[string]any{"result": ev.ToolResult.Result}}),
            wrap(TimelineEvent{Type: "completed", EntityID: ev.ToolResult.ID + ":result"}),
            wrap(TimelineEvent{Type: "updated", EntityID: ev.ToolResult.ID, Patch: map[string]any{"exec": false}, Version: now.UnixNano(), UpdatedAt: now.UnixMilli()}),
        }
    case *events.EventAgentModeSwitch:
        log.Debug().Str("component", "web_forwarder").Str("kind", "agent_mode").Msg("mapping agent_mode to timeline created+completed")
        props := map[string]any{"title": ev.Message}
        for k, v := range ev.Data { props[k] = v }
        localID := "agentmode-" + md.TurnID + "-" + uuid.NewString()
        return [][]byte{
            wrap(TimelineEvent{Type: "created", EntityID: localID, Kind: "agent_mode", Renderer: map[string]string{"kind":"agent_mode"}, Props: props}),
            wrap(TimelineEvent{Type: "completed", EntityID: localID}),
        }
    }
    log.Debug().Str("component", "web_forwarder").Msg("no timeline mapping for event; dropping")
    return nil
}


