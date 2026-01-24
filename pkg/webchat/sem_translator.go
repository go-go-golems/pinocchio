package webchat

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/events"
	sempb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/base"
	semregistry "github.com/go-go-golems/pinocchio/pkg/sem/registry"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type cachedToolCall struct {
	Name     string
	RawInput string
	InputObj map[string]any
}

// EventTranslator converts Geppetto events into SEM frames for the UI.
// It owns small caches used to produce stable IDs across streaming events.
type EventTranslator struct {
	toolCallCache sync.Map
	messageIDs    sync.Map // key: inferenceID/turnID/sessionID, value: string
}

func NewEventTranslator() *EventTranslator { return &EventTranslator{} }

var defaultEventTranslator = NewEventTranslator()

func init() {
	defaultEventTranslator.RegisterDefaultHandlers()
}

func wrapSem(ev map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"sem": true, "event": ev})
	return b
}

func protoToRaw(m proto.Message) (json.RawMessage, error) {
	if m == nil {
		return nil, nil
	}
	b, err := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false, // camelCase JSON names
	}.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func mapToStruct(m map[string]any) (*structpb.Struct, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return structpb.NewStruct(m)
}

func buildLlmInferenceMetadata(md events.LLMInferenceData) *sempb.LlmInferenceMetadataV1 {
	var meta sempbbuilder
	meta.Model = md.Model
	if md.Temperature != nil {
		meta.Temperature = md.Temperature
	}
	if md.TopP != nil {
		meta.TopP = md.TopP
	}
	if md.MaxTokens != nil {
		v := int32(*md.MaxTokens)
		meta.MaxTokens = &v
	}
	if md.StopReason != nil {
		meta.StopReason = md.StopReason
	}
	if md.Usage != nil {
		meta.Usage = &sempb.UsageV1{
			InputTokens:              int32(md.Usage.InputTokens),
			OutputTokens:             int32(md.Usage.OutputTokens),
			CachedTokens:             int32(md.Usage.CachedTokens),
			CacheCreationInputTokens: int32(md.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int32(md.Usage.CacheReadInputTokens),
		}
	}
	if md.DurationMs != nil {
		meta.DurationMs = md.DurationMs
	}
	if meta.isEmpty() {
		return nil
	}
	return meta.asProto()
}

type sempbbuilder struct {
	Model       string
	Temperature *float64
	TopP        *float64
	MaxTokens   *int32
	StopReason  *string
	Usage       *sempb.UsageV1
	DurationMs  *int64
}

func (b sempbbuilder) isEmpty() bool {
	return b.Model == "" &&
		b.Temperature == nil &&
		b.TopP == nil &&
		b.MaxTokens == nil &&
		b.StopReason == nil &&
		b.Usage == nil &&
		b.DurationMs == nil
}

func (b sempbbuilder) asProto() *sempb.LlmInferenceMetadataV1 {
	return &sempb.LlmInferenceMetadataV1{
		Model:       b.Model,
		Temperature: b.Temperature,
		TopP:        b.TopP,
		MaxTokens:   b.MaxTokens,
		StopReason:  b.StopReason,
		Usage:       b.Usage,
		DurationMs:  b.DurationMs,
	}
}

// SemanticEventsFromEvent converts a Geppetto event into SEM frames for the UI.
func SemanticEventsFromEvent(e events.Event) [][]byte {
	return defaultEventTranslator.Translate(e)
}

// Translate converts a Geppetto event into SEM frames.
// This implementation is registry-only (no fallback switch).
func (et *EventTranslator) Translate(e events.Event) [][]byte {
	if et == nil || e == nil {
		return nil
	}
	md := e.Metadata()
	log.Debug().
		Str("component", "web_forwarder").
		Str("event_type", fmt.Sprintf("%T", e)).
		Str("event_id", md.ID.String()).
		Str("session_id", md.SessionID).
		Str("inference_id", md.InferenceID).
		Str("turn_id", md.TurnID).
		Msg("received event (SEM)")

	frames, found, err := semregistry.Handle(e)
	if !found {
		log.Debug().
			Str("component", "web_forwarder").
			Str("event_type", fmt.Sprintf("%T", e)).
			Msg("no semantic mapping for event; dropping")
		return nil
	}
	if err != nil {
		log.Warn().
			Str("component", "web_forwarder").
			Str("event_type", fmt.Sprintf("%T", e)).
			Err(err).
			Msg("registry handler returned error; dropping event")
		return nil
	}
	return frames
}

func messageKey(md events.EventMetadata) string {
	if md.InferenceID != "" {
		return "inference:" + md.InferenceID
	}
	if md.TurnID != "" {
		return "turn:" + md.TurnID
	}
	if md.SessionID != "" {
		return "session:" + md.SessionID
	}
	return ""
}

func fallbackMessageID(md events.EventMetadata) string {
	if md.InferenceID != "" {
		return "llm-" + md.InferenceID
	}
	if md.TurnID != "" {
		return "llm-" + md.TurnID
	}
	if md.SessionID != "" {
		return "llm-" + md.SessionID
	}
	return "llm-" + uuid.NewString()
}

func (et *EventTranslator) resolveMessageID(md events.EventMetadata) string {
	if md.ID != uuid.Nil {
		id := md.ID.String()
		if key := messageKey(md); key != "" {
			et.messageIDs.Store(key, id)
		}
		return id
	}

	if key := messageKey(md); key != "" {
		if v, ok := et.messageIDs.Load(key); ok {
			if cached, ok2 := v.(string); ok2 && cached != "" {
				return cached
			}
		}
	}

	fallback := fallbackMessageID(md)
	if key := messageKey(md); key != "" {
		et.messageIDs.Store(key, fallback)
	}
	return fallback
}

func (et *EventTranslator) clearMessageID(md events.EventMetadata) {
	if et == nil {
		return
	}
	if key := messageKey(md); key != "" {
		et.messageIDs.Delete(key)
	}
}

func (et *EventTranslator) RegisterDefaultHandlers() {
	if et == nil {
		return
	}

	semregistry.RegisterByType[*events.EventLog](func(ev *events.EventLog) ([][]byte, error) {
		md := ev.Metadata()
		lvl := ev.Level
		if lvl == "" {
			lvl = "info"
		}
		id := md.ID.String()
		if md.ID == uuid.Nil {
			id = "log-" + uuid.NewString()
		}
		fields, err := mapToStruct(ev.Fields)
		if err != nil {
			return nil, err
		}
		data, err := protoToRaw(&sempb.LogV1{Id: id, Level: lvl, Message: ev.Message, Fields: fields})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "log", "id": id, "data": data})}, nil
	})

	semregistry.RegisterByType[*events.EventPartialCompletionStart](func(ev *events.EventPartialCompletionStart) ([][]byte, error) {
		md := ev.Metadata()
		id := et.resolveMessageID(md)
		data, err := protoToRaw(&sempb.LlmStart{Id: id, Role: "assistant"})
		if err != nil {
			return nil, err
		}
		out := map[string]any{"type": "llm.start", "id": id, "data": data}
		if metaPB := buildLlmInferenceMetadata(md.LLMInferenceData); metaPB != nil {
			if metaRaw, err := protoToRaw(metaPB); err == nil && metaRaw != nil {
				out["metadata"] = metaRaw
			}
		}
		return [][]byte{wrapSem(out)}, nil
	})

	semregistry.RegisterByType[*events.EventPartialCompletion](func(ev *events.EventPartialCompletion) ([][]byte, error) {
		md := ev.Metadata()
		id := et.resolveMessageID(md)
		data, err := protoToRaw(&sempb.LlmDelta{Id: id, Delta: ev.Delta, Cumulative: ev.Completion})
		if err != nil {
			return nil, err
		}
		out := map[string]any{"type": "llm.delta", "id": id, "data": data}
		if metaPB := buildLlmInferenceMetadata(md.LLMInferenceData); metaPB != nil {
			if metaRaw, err := protoToRaw(metaPB); err == nil && metaRaw != nil {
				out["metadata"] = metaRaw
			}
		}
		return [][]byte{wrapSem(out)}, nil
	})

	semregistry.RegisterByType[*events.EventFinal](func(ev *events.EventFinal) ([][]byte, error) {
		md := ev.Metadata()
		id := et.resolveMessageID(md)
		data, err := protoToRaw(&sempb.LlmFinal{Id: id, Text: ev.Text})
		if err != nil {
			return nil, err
		}
		sem := map[string]any{"type": "llm.final", "id": id, "data": data}
		if metaPB := buildLlmInferenceMetadata(md.LLMInferenceData); metaPB != nil {
			if metaRaw, err := protoToRaw(metaPB); err == nil && metaRaw != nil {
				sem["metadata"] = metaRaw
			}
		}
		et.clearMessageID(md)
		return [][]byte{wrapSem(sem)}, nil
	})

	semregistry.RegisterByType[*events.EventInfo](func(ev *events.EventInfo) ([][]byte, error) {
		md := ev.Metadata()
		baseID := et.resolveMessageID(md)
		switch ev.Message {
		case "thinking-started":
			id := baseID + ":thinking"
			data, err := protoToRaw(&sempb.LlmStart{Id: id, Role: "thinking"})
			if err != nil {
				return nil, err
			}
			out := map[string]any{"type": "llm.thinking.start", "id": id, "data": data}
			if metaPB := buildLlmInferenceMetadata(md.LLMInferenceData); metaPB != nil {
				if metaRaw, err := protoToRaw(metaPB); err == nil && metaRaw != nil {
					out["metadata"] = metaRaw
				}
			}
			return [][]byte{wrapSem(out)}, nil
		case "thinking-ended":
			id := baseID + ":thinking"
			data, err := protoToRaw(&sempb.LlmDone{Id: id})
			if err != nil {
				return nil, err
			}
			return [][]byte{wrapSem(map[string]any{"type": "llm.thinking.final", "id": id, "data": data})}, nil
		case "reasoning-summary":
			if text, ok := ev.Data["text"].(string); ok && text != "" {
				id := baseID + ":thinking"
				data, err := protoToRaw(&sempb.LlmDelta{Id: id, Delta: text, Cumulative: text})
				if err != nil {
					return nil, err
				}
				out := map[string]any{"type": "llm.thinking.delta", "id": id, "data": data}
				if metaPB := buildLlmInferenceMetadata(md.LLMInferenceData); metaPB != nil {
					if metaRaw, err := protoToRaw(metaPB); err == nil && metaRaw != nil {
						out["metadata"] = metaRaw
					}
				}
				return [][]byte{wrapSem(out)}, nil
			}
		}
		return nil, nil
	})

	semregistry.RegisterByType[*events.EventThinkingPartial](func(ev *events.EventThinkingPartial) ([][]byte, error) {
		md := ev.Metadata()
		baseID := et.resolveMessageID(md)
		id := baseID + ":thinking"
		data, err := protoToRaw(&sempb.LlmDelta{Id: id, Delta: ev.Delta, Cumulative: ev.Completion})
		if err != nil {
			return nil, err
		}
		out := map[string]any{"type": "llm.thinking.delta", "id": id, "data": data}
		if metaPB := buildLlmInferenceMetadata(md.LLMInferenceData); metaPB != nil {
			if metaRaw, err := protoToRaw(metaPB); err == nil && metaRaw != nil {
				out["metadata"] = metaRaw
			}
		}
		return [][]byte{wrapSem(out)}, nil
	})

	semregistry.RegisterByType[*events.EventInterrupt](func(ev *events.EventInterrupt) ([][]byte, error) {
		md := ev.Metadata()
		id := et.resolveMessageID(md)
		data, err := protoToRaw(&sempb.LlmFinal{Id: id, Text: ev.Text})
		if err != nil {
			return nil, err
		}
		sem := map[string]any{"type": "llm.final", "id": id, "data": data}
		et.clearMessageID(md)
		return [][]byte{wrapSem(sem)}, nil
	})

	semregistry.RegisterByType[*events.EventToolCall](func(ev *events.EventToolCall) ([][]byte, error) {
		var inputObj map[string]any
		if ev.ToolCall.Input != "" {
			_ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
		}
		et.toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
		input, err := mapToStruct(inputObj)
		if err != nil {
			return nil, err
		}
		data, err := protoToRaw(&sempb.ToolStart{Id: ev.ToolCall.ID, Name: ev.ToolCall.Name, Input: input})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "tool.start", "id": ev.ToolCall.ID, "data": data})}, nil
	})

	semregistry.RegisterByType[*events.EventToolCallExecute](func(ev *events.EventToolCallExecute) ([][]byte, error) {
		var inputObj map[string]any
		if ev.ToolCall.Input != "" {
			_ = json.Unmarshal([]byte(ev.ToolCall.Input), &inputObj)
		}
		et.toolCallCache.Store(ev.ToolCall.ID, cachedToolCall{Name: ev.ToolCall.Name, RawInput: ev.ToolCall.Input, InputObj: inputObj})
		patch, err := mapToStruct(map[string]any{"exec": true, "input": inputObj})
		if err != nil {
			return nil, err
		}
		data, err := protoToRaw(&sempb.ToolDelta{Id: ev.ToolCall.ID, Patch: patch})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "tool.delta", "id": ev.ToolCall.ID, "data": data})}, nil
	})

	semregistry.RegisterByType[*events.EventToolResult](func(ev *events.EventToolResult) ([][]byte, error) {
		var frames [][]byte
		if v, ok := et.toolCallCache.Load(ev.ToolResult.ID); ok {
			if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
				resultData, err := protoToRaw(&sempb.ToolResult{Id: ev.ToolResult.ID, Result: ev.ToolResult.Result, CustomKind: "calc_result"})
				if err != nil {
					return nil, err
				}
				doneData, err := protoToRaw(&sempb.ToolDone{Id: ev.ToolResult.ID})
				if err != nil {
					return nil, err
				}
				frames = append(frames, wrapSem(map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "data": resultData}))
				frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID, "data": doneData}))
				return frames, nil
			}
		}
		resultData, err := protoToRaw(&sempb.ToolResult{Id: ev.ToolResult.ID, Result: ev.ToolResult.Result})
		if err != nil {
			return nil, err
		}
		doneData, err := protoToRaw(&sempb.ToolDone{Id: ev.ToolResult.ID})
		if err != nil {
			return nil, err
		}
		frames = append(frames, wrapSem(map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "data": resultData}))
		frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID, "data": doneData}))
		return frames, nil
	})

	semregistry.RegisterByType[*events.EventToolCallExecutionResult](func(ev *events.EventToolCallExecutionResult) ([][]byte, error) {
		var frames [][]byte
		if v, ok := et.toolCallCache.Load(ev.ToolResult.ID); ok {
			if ctc, ok2 := v.(cachedToolCall); ok2 && ctc.Name == "calc" {
				resultData, err := protoToRaw(&sempb.ToolResult{Id: ev.ToolResult.ID, Result: ev.ToolResult.Result, CustomKind: "calc_result"})
				if err != nil {
					return nil, err
				}
				doneData, err := protoToRaw(&sempb.ToolDone{Id: ev.ToolResult.ID})
				if err != nil {
					return nil, err
				}
				frames = append(frames, wrapSem(map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "data": resultData}))
				frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID, "data": doneData}))
				return frames, nil
			}
		}
		resultData, err := protoToRaw(&sempb.ToolResult{Id: ev.ToolResult.ID, Result: ev.ToolResult.Result})
		if err != nil {
			return nil, err
		}
		doneData, err := protoToRaw(&sempb.ToolDone{Id: ev.ToolResult.ID})
		if err != nil {
			return nil, err
		}
		frames = append(frames, wrapSem(map[string]any{"type": "tool.result", "id": ev.ToolResult.ID, "data": resultData}))
		frames = append(frames, wrapSem(map[string]any{"type": "tool.done", "id": ev.ToolResult.ID, "data": doneData}))
		return frames, nil
	})

	semregistry.RegisterByType[*events.EventAgentModeSwitch](func(ev *events.EventAgentModeSwitch) ([][]byte, error) {
		md := ev.Metadata()
		props := map[string]any{}
		for k, v := range ev.Data {
			props[k] = v
		}
		localID := "agentmode-" + md.TurnID + "-" + uuid.NewString()
		dataStruct, err := mapToStruct(props)
		if err != nil {
			return nil, err
		}
		data, err := protoToRaw(&sempb.AgentModeV1{Id: localID, Title: ev.Message, Data: dataStruct})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "agent.mode", "id": localID, "data": data})}, nil
	})

	semregistry.RegisterByType[*events.EventDebuggerPause](func(ev *events.EventDebuggerPause) ([][]byte, error) {
		extra, err := mapToStruct(ev.Extra)
		if err != nil {
			return nil, err
		}
		data, err := protoToRaw(&sempb.DebuggerPauseV1{
			Id:         ev.PauseID,
			PauseId:    ev.PauseID,
			Phase:      ev.Phase,
			Summary:    ev.Summary,
			DeadlineMs: ev.DeadlineMs,
			Extra:      extra,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "debugger.pause", "id": ev.PauseID, "data": data})}, nil
	})
}
