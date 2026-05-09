package plugins

import (
	"context"
	"fmt"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

const (
	// EventToolCallStarted is published when the model starts a tool call.
	EventToolCallStarted = chatapp.EventChatToolCallStarted
	// EventToolCallArgumentsDelta is published for streamed tool-call arguments.
	EventToolCallArgumentsDelta = chatapp.EventChatToolCallArgumentsDelta
	// EventToolCallRequested is published when the model has fully requested a tool call.
	EventToolCallRequested = chatapp.EventChatToolCallRequested
	// EventToolExecutionStarted is published when host-side tool execution begins.
	EventToolExecutionStarted = chatapp.EventChatToolExecutionStarted
	// EventToolResultReady is published when a tool result is available.
	EventToolResultReady = chatapp.EventChatToolResultReady
	// EventToolCallFinished is published when a tool call completes.
	EventToolCallFinished = chatapp.EventChatToolCallFinished

	// TimelineEntityToolCall is the timeline entity kind for tool calls.
	TimelineEntityToolCall = "ChatToolCall"
	// TimelineEntityToolResult is the timeline entity kind for tool results.
	TimelineEntityToolResult = "ChatToolResult"
)

// ToolCallPlugin translates canonical Geppetto tool lifecycle events into
// canonical Pinocchio backend/UI events and timeline entities.
type ToolCallPlugin struct{}

// NewToolCallPlugin creates a new ToolCallPlugin.
func NewToolCallPlugin() chatapp.ChatPlugin { return &ToolCallPlugin{} }

// RegisterSchemas registers canonical tool event, UI event, and timeline entity schemas.
func (p *ToolCallPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(EventToolCallStarted, &chatappv1.ChatToolCallStarted{}),
		reg.RegisterEvent(EventToolCallArgumentsDelta, &chatappv1.ChatToolCallArgumentsDelta{}),
		reg.RegisterEvent(EventToolCallRequested, &chatappv1.ChatToolCallRequested{}),
		reg.RegisterEvent(EventToolExecutionStarted, &chatappv1.ChatToolExecutionStarted{}),
		reg.RegisterEvent(EventToolResultReady, &chatappv1.ChatToolResultReady{}),
		reg.RegisterEvent(EventToolCallFinished, &chatappv1.ChatToolCallFinished{}),
		reg.RegisterUIEvent(EventToolCallStarted, &chatappv1.ChatToolCallStarted{}),
		reg.RegisterUIEvent(EventToolCallArgumentsDelta, &chatappv1.ChatToolCallArgumentsDelta{}),
		reg.RegisterUIEvent(EventToolCallRequested, &chatappv1.ChatToolCallRequested{}),
		reg.RegisterUIEvent(EventToolExecutionStarted, &chatappv1.ChatToolExecutionStarted{}),
		reg.RegisterUIEvent(EventToolResultReady, &chatappv1.ChatToolResultReady{}),
		reg.RegisterUIEvent(EventToolCallFinished, &chatappv1.ChatToolCallFinished{}),
		reg.RegisterTimelineEntity(TimelineEntityToolCall, &chatappv1.ToolCallEntity{}),
		reg.RegisterTimelineEntity(TimelineEntityToolResult, &chatappv1.ToolResultEntity{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// HandleRuntimeEvent handles canonical tool call events from the Geppetto engine.
func (p *ToolCallPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	switch ev := event.(type) {
	case *gepevents.EventToolCallStarted:
		return true, runtime.Publish(ctx, EventToolCallStarted, &chatappv1.ChatToolCallStarted{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName, Status: "pending", Correlation: chatapp.CorrelationInfoFromEvent(ev)})
	case *gepevents.EventToolCallArgumentsDelta:
		return true, runtime.Publish(ctx, EventToolCallArgumentsDelta, &chatappv1.ChatToolCallArgumentsDelta{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ArgumentsDelta: ev.Delta, Input: ev.Arguments, Status: "streaming_args", Correlation: chatapp.CorrelationInfoFromEvent(ev)})
	case *gepevents.EventToolCallRequested:
		return true, runtime.Publish(ctx, EventToolCallRequested, &chatappv1.ChatToolCallRequested{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName, Input: ev.Input, Status: "pending", Correlation: chatapp.CorrelationInfoFromEvent(ev)})
	case *gepevents.EventToolExecutionStarted:
		return true, runtime.Publish(ctx, EventToolExecutionStarted, &chatappv1.ChatToolExecutionStarted{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName, Input: ev.Input, Executing: true, Status: "executing", Correlation: chatapp.CorrelationInfoFromEvent(ev)})
	case *gepevents.EventToolResultReady:
		return true, runtime.Publish(ctx, EventToolResultReady, &chatappv1.ChatToolResultReady{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName, Result: ev.Result, Status: firstNonEmptyString(ev.Status, "success"), Correlation: chatapp.CorrelationInfoFromEvent(ev)})
	case *gepevents.EventToolCallFinished:
		return true, runtime.Publish(ctx, EventToolCallFinished, &chatappv1.ChatToolCallFinished{MessageId: runtime.MessageID, ToolCallId: ev.ToolCallID, ToolName: ev.ToolName, Status: firstNonEmptyString(ev.Status, "completed"), Correlation: chatapp.CorrelationInfoFromEvent(ev)})
	default:
		return false, nil
	}
}

// ProjectUI forwards canonical tool backend events as canonical UI events.
func (p *ToolCallPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case EventToolCallStarted, EventToolCallArgumentsDelta, EventToolCallRequested, EventToolExecutionStarted, EventToolResultReady, EventToolCallFinished:
		if ev.Payload == nil {
			return nil, true, fmt.Errorf("tool payload must be proto message, got %T", ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: ev.Name, Payload: proto.Clone(ev.Payload)}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects tool call backend events into timeline entities.
func (p *ToolCallPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	switch ev.Name {
	case EventToolCallStarted, EventToolCallArgumentsDelta, EventToolCallRequested, EventToolExecutionStarted, EventToolCallFinished:
		payload, ok := toolCallFieldsFromCanonical(ev)
		if !ok {
			return nil, true, fmt.Errorf("unexpected tool call payload %T", ev.Payload)
		}
		existing := mergeToolCallFields(currentToolCallEntity(view, payload.ToolCallID), payload)
		if ev.Name == EventToolCallFinished {
			existing.Status = "completed"
			existing.Executing = false
		}
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityToolCall, Id: payload.ToolCallID, Payload: existing}}, true, nil
	case EventToolResultReady:
		payload, ok := ev.Payload.(*chatappv1.ChatToolResultReady)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("tool result payload must be %T, got %T", &chatappv1.ChatToolResultReady{}, ev.Payload)
		}
		return []sessionstream.TimelineEntity{{
			Kind: TimelineEntityToolResult,
			Id:   payload.GetToolCallId() + ":result",
			Payload: &chatappv1.ToolResultEntity{
				MessageId:   payload.GetMessageId(),
				ToolCallId:  payload.GetToolCallId(),
				ToolName:    payload.GetToolName(),
				Result:      payload.GetResult(),
				Status:      payload.GetStatus(),
				Correlation: chatapp.CloneCorrelationInfo(payload.GetCorrelation()),
			},
		}}, true, nil
	default:
		return nil, false, nil
	}
}

type toolCallFields struct {
	MessageID   string
	ToolCallID  string
	ToolName    string
	Input       string
	Executing   bool
	Status      string
	Correlation *chatappv1.CorrelationInfo
}

func toolCallFieldsFromCanonical(ev sessionstream.Event) (toolCallFields, bool) {
	switch payload := ev.Payload.(type) {
	case *chatappv1.ChatToolCallStarted:
		return toolCallFields{MessageID: payload.GetMessageId(), ToolCallID: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Executing: payload.GetExecuting(), Status: firstNonEmptyString(payload.GetStatus(), "pending"), Correlation: payload.GetCorrelation()}, true
	case *chatappv1.ChatToolCallArgumentsDelta:
		return toolCallFields{MessageID: payload.GetMessageId(), ToolCallID: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Status: firstNonEmptyString(payload.GetStatus(), "streaming_args"), Correlation: payload.GetCorrelation()}, true
	case *chatappv1.ChatToolCallRequested:
		return toolCallFields{MessageID: payload.GetMessageId(), ToolCallID: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Executing: payload.GetExecuting(), Status: firstNonEmptyString(payload.GetStatus(), "pending"), Correlation: payload.GetCorrelation()}, true
	case *chatappv1.ChatToolExecutionStarted:
		return toolCallFields{MessageID: payload.GetMessageId(), ToolCallID: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Executing: payload.GetExecuting(), Status: firstNonEmptyString(payload.GetStatus(), "executing"), Correlation: payload.GetCorrelation()}, true
	case *chatappv1.ChatToolCallFinished:
		return toolCallFields{MessageID: payload.GetMessageId(), ToolCallID: payload.GetToolCallId(), ToolName: payload.GetToolName(), Executing: false, Status: firstNonEmptyString(payload.GetStatus(), "completed"), Correlation: payload.GetCorrelation()}, true
	default:
		return toolCallFields{}, false
	}
}

func currentToolCallEntity(view sessionstream.TimelineView, id string) *chatappv1.ToolCallEntity {
	if view == nil {
		return &chatappv1.ToolCallEntity{}
	}
	entity, ok := view.Get(TimelineEntityToolCall, id)
	if !ok || entity.Payload == nil {
		return &chatappv1.ToolCallEntity{}
	}
	pb, ok := entity.Payload.(*chatappv1.ToolCallEntity)
	if !ok || pb == nil {
		return &chatappv1.ToolCallEntity{}
	}
	return proto.Clone(pb).(*chatappv1.ToolCallEntity)
}

func cloneCorrelationInfo(corr *chatappv1.CorrelationInfo) *chatappv1.CorrelationInfo {
	return chatapp.CloneCorrelationInfo(corr)
}

func mergeToolCallFields(entity *chatappv1.ToolCallEntity, update toolCallFields) *chatappv1.ToolCallEntity {
	if entity == nil {
		entity = &chatappv1.ToolCallEntity{}
	}
	if update.MessageID != "" {
		entity.MessageId = update.MessageID
	}
	if update.ToolCallID != "" {
		entity.ToolCallId = update.ToolCallID
	}
	if update.ToolName != "" {
		entity.ToolName = update.ToolName
	}
	if update.Input != "" {
		entity.Input = update.Input
	}
	entity.Executing = update.Executing
	if update.Status != "" {
		entity.Status = update.Status
	}
	entity.Correlation = chatapp.MergeCorrelationInfo(entity.GetCorrelation(), update.Correlation)
	return entity
}

// Ensure ToolCallPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ToolCallPlugin)(nil)
