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
	// EventToolCallStarted is published when the model requests a tool call.
	EventToolCallStarted = "ChatToolCallStarted"
	// EventToolCallUpdated is published when tool execution begins.
	EventToolCallUpdated = "ChatToolCallUpdated"
	// EventToolCallFinished is published when a tool call completes.
	EventToolCallFinished = "ChatToolCallFinished"
	// EventToolResultReady is published when a tool result is available.
	EventToolResultReady = "ChatToolResultReady"

	// UIToolCallStarted is the UI event for tool call start.
	UIToolCallStarted = "ChatToolCallStarted"
	// UIToolCallUpdated is the UI event for tool call execution.
	UIToolCallUpdated = "ChatToolCallUpdated"
	// UIToolCallFinished is the UI event for tool call completion.
	UIToolCallFinished = "ChatToolCallFinished"
	// UIToolResultReady is the UI event for tool result.
	UIToolResultReady = "ChatToolResultReady"

	// TimelineEntityToolCall is the timeline entity kind for tool calls.
	TimelineEntityToolCall = "ChatToolCall"
	// TimelineEntityToolResult is the timeline entity kind for tool results.
	TimelineEntityToolResult = "ChatToolResult"
)

// ToolCallPlugin is a ChatPlugin that handles tool call lifecycle events from
// geppetto inference engines. It translates EventToolCall, EventToolCallExecute,
// EventToolResult, and EventToolCallExecutionResult into sessionstream events,
// and projects them into ChatToolCall and ChatToolResult timeline entities.
type ToolCallPlugin struct{}

// NewToolCallPlugin creates a new ToolCallPlugin.
func NewToolCallPlugin() chatapp.ChatPlugin {
	return &ToolCallPlugin{}
}

// RegisterSchemas registers the tool call event names, UI events, and timeline entity kinds.
func (p *ToolCallPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(EventToolCallStarted, &chatappv1.ToolCallUpdate{}),
		reg.RegisterEvent(EventToolCallUpdated, &chatappv1.ToolCallUpdate{}),
		reg.RegisterEvent(EventToolCallFinished, &chatappv1.ToolCallUpdate{}),
		reg.RegisterEvent(EventToolResultReady, &chatappv1.ToolResultUpdate{}),
		reg.RegisterUIEvent(UIToolCallStarted, &chatappv1.ToolCallUpdate{}),
		reg.RegisterUIEvent(UIToolCallUpdated, &chatappv1.ToolCallUpdate{}),
		reg.RegisterUIEvent(UIToolCallFinished, &chatappv1.ToolCallUpdate{}),
		reg.RegisterUIEvent(UIToolResultReady, &chatappv1.ToolResultUpdate{}),
		reg.RegisterTimelineEntity(TimelineEntityToolCall, &chatappv1.ToolCallEntity{}),
		reg.RegisterTimelineEntity(TimelineEntityToolResult, &chatappv1.ToolResultEntity{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// HandleRuntimeEvent handles tool call events from the geppetto engine.
func (p *ToolCallPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	switch ev := event.(type) {
	case *gepevents.EventToolCall:
		return true, runtime.Publish(ctx, EventToolCallStarted, &chatappv1.ToolCallUpdate{
			MessageId:  runtime.MessageID,
			ToolCallId: ev.ToolCall.ID,
			ToolName:   ev.ToolCall.Name,
			Input:      ev.ToolCall.Input,
			Status:     "pending",
		})

	case *gepevents.EventToolCallExecute:
		return true, runtime.Publish(ctx, EventToolCallUpdated, &chatappv1.ToolCallUpdate{
			MessageId:  runtime.MessageID,
			ToolCallId: ev.ToolCall.ID,
			ToolName:   ev.ToolCall.Name,
			Input:      ev.ToolCall.Input,
			Executing:  true,
			Status:     "executing",
		})

	case *gepevents.EventToolResult:
		_ = runtime.Publish(ctx, EventToolResultReady, &chatappv1.ToolResultUpdate{
			MessageId:  runtime.MessageID,
			ToolCallId: ev.ToolResult.ID,
			ToolName:   ev.ToolResult.Name,
			Result:     ev.ToolResult.Result,
			Status:     "success",
		})
		return true, runtime.Publish(ctx, EventToolCallFinished, &chatappv1.ToolCallUpdate{
			MessageId:  runtime.MessageID,
			ToolCallId: ev.ToolResult.ID,
			Status:     "completed",
		})

	case *gepevents.EventToolCallExecutionResult:
		_ = runtime.Publish(ctx, EventToolResultReady, &chatappv1.ToolResultUpdate{
			MessageId:  runtime.MessageID,
			ToolCallId: ev.ToolResult.ID,
			ToolName:   ev.ToolResult.Name,
			Result:     ev.ToolResult.Result,
			Status:     "success",
		})
		return true, runtime.Publish(ctx, EventToolCallFinished, &chatappv1.ToolCallUpdate{
			MessageId:  runtime.MessageID,
			ToolCallId: ev.ToolResult.ID,
			Status:     "completed",
		})

	default:
		return false, nil
	}
}

// ProjectUI projects tool call backend events into UI events.
func (p *ToolCallPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case EventToolCallStarted, EventToolCallUpdated,
		EventToolCallFinished, EventToolResultReady:
		return []sessionstream.UIEvent{{
			Name:    ev.Name,
			Payload: proto.Clone(ev.Payload),
		}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects tool call backend events into timeline entities.
func (p *ToolCallPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	switch ev.Name {
	case EventToolCallStarted:
		payload, ok := ev.Payload.(*chatappv1.ToolCallUpdate)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("tool call started payload must be %T, got %T", &chatappv1.ToolCallUpdate{}, ev.Payload)
		}
		return []sessionstream.TimelineEntity{{
			Kind: TimelineEntityToolCall,
			Id:   payload.GetToolCallId(),
			Payload: &chatappv1.ToolCallEntity{
				MessageId:  payload.GetMessageId(),
				ToolCallId: payload.GetToolCallId(),
				ToolName:   payload.GetToolName(),
				Input:      payload.GetInput(),
				Status:     "pending",
			},
		}}, true, nil

	case EventToolCallUpdated:
		payload, ok := ev.Payload.(*chatappv1.ToolCallUpdate)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("tool call updated payload must be %T, got %T", &chatappv1.ToolCallUpdate{}, ev.Payload)
		}
		existing := currentToolCallEntity(view, payload.GetToolCallId())
		existing.Executing = payload.GetExecuting()
		existing.Status = "executing"
		return []sessionstream.TimelineEntity{{
			Kind:    TimelineEntityToolCall,
			Id:      payload.GetToolCallId(),
			Payload: existing,
		}}, true, nil

	case EventToolCallFinished:
		payload, ok := ev.Payload.(*chatappv1.ToolCallUpdate)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("tool call finished payload must be %T, got %T", &chatappv1.ToolCallUpdate{}, ev.Payload)
		}
		existing := currentToolCallEntity(view, payload.GetToolCallId())
		existing.Status = "completed"
		return []sessionstream.TimelineEntity{{
			Kind:    TimelineEntityToolCall,
			Id:      payload.GetToolCallId(),
			Payload: existing,
		}}, true, nil

	case EventToolResultReady:
		payload, ok := ev.Payload.(*chatappv1.ToolResultUpdate)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("tool result payload must be %T, got %T", &chatappv1.ToolResultUpdate{}, ev.Payload)
		}
		return []sessionstream.TimelineEntity{{
			Kind: TimelineEntityToolResult,
			Id:   payload.GetToolCallId() + ":result",
			Payload: &chatappv1.ToolResultEntity{
				MessageId:  payload.GetMessageId(),
				ToolCallId: payload.GetToolCallId(),
				ToolName:   payload.GetToolName(),
				Result:     payload.GetResult(),
				Status:     payload.GetStatus(),
			},
		}}, true, nil

	default:
		return nil, false, nil
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
	return &chatappv1.ToolCallEntity{
		MessageId:  pb.GetMessageId(),
		ToolCallId: pb.GetToolCallId(),
		ToolName:   pb.GetToolName(),
		Input:      pb.GetInput(),
		Executing:  pb.GetExecuting(),
		Status:     pb.GetStatus(),
	}
}

// Ensure ToolCallPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ToolCallPlugin)(nil)
