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

	// UIToolCallStarted is the UI event for tool call start.
	UIToolCallStarted = "ChatToolCallStarted"
	// UIToolCallUpdated is the UI event for tool call execution or argument updates.
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

// ToolCallPlugin translates canonical Geppetto tool lifecycle events into
// canonical Pinocchio backend events and compatibility UI events.
type ToolCallPlugin struct{}

// NewToolCallPlugin creates a new ToolCallPlugin.
func NewToolCallPlugin() chatapp.ChatPlugin { return &ToolCallPlugin{} }

// RegisterSchemas registers the tool call event names, UI events, and timeline entity kinds.
func (p *ToolCallPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(EventToolCallStarted, &chatappv1.ChatToolCallStarted{}),
		reg.RegisterEvent(EventToolCallArgumentsDelta, &chatappv1.ChatToolCallArgumentsDelta{}),
		reg.RegisterEvent(EventToolCallRequested, &chatappv1.ChatToolCallRequested{}),
		reg.RegisterEvent(EventToolExecutionStarted, &chatappv1.ChatToolExecutionStarted{}),
		reg.RegisterEvent(EventToolResultReady, &chatappv1.ChatToolResultReady{}),
		reg.RegisterEvent(EventToolCallFinished, &chatappv1.ChatToolCallFinished{}),
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

// HandleRuntimeEvent handles canonical tool call events from the Geppetto engine.
func (p *ToolCallPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	switch ev := event.(type) {
	case *gepevents.EventToolCallStarted:
		return true, runtime.Publish(ctx, EventToolCallStarted, &chatappv1.ChatToolCallStarted{
			MessageId:   runtime.MessageID,
			ToolCallId:  ev.ToolCallID,
			ToolName:    ev.ToolName,
			Status:      "pending",
			Correlation: chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventToolCallArgumentsDelta:
		return true, runtime.Publish(ctx, EventToolCallArgumentsDelta, &chatappv1.ChatToolCallArgumentsDelta{
			MessageId:      runtime.MessageID,
			ToolCallId:     ev.ToolCallID,
			ArgumentsDelta: ev.Delta,
			Input:          ev.Arguments,
			Status:         "streaming_args",
			Correlation:    chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventToolCallRequested:
		return true, runtime.Publish(ctx, EventToolCallRequested, &chatappv1.ChatToolCallRequested{
			MessageId:   runtime.MessageID,
			ToolCallId:  ev.ToolCallID,
			ToolName:    ev.ToolName,
			Input:       ev.Input,
			Status:      "pending",
			Correlation: chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventToolExecutionStarted:
		return true, runtime.Publish(ctx, EventToolExecutionStarted, &chatappv1.ChatToolExecutionStarted{
			MessageId:   runtime.MessageID,
			ToolCallId:  ev.ToolCallID,
			ToolName:    ev.ToolName,
			Input:       ev.Input,
			Executing:   true,
			Status:      "executing",
			Correlation: chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventToolResultReady:
		return true, runtime.Publish(ctx, EventToolResultReady, &chatappv1.ChatToolResultReady{
			MessageId:   runtime.MessageID,
			ToolCallId:  ev.ToolCallID,
			ToolName:    ev.ToolName,
			Result:      ev.Result,
			Status:      firstNonEmptyString(ev.Status, "success"),
			Correlation: chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventToolCallFinished:
		return true, runtime.Publish(ctx, EventToolCallFinished, &chatappv1.ChatToolCallFinished{
			MessageId:   runtime.MessageID,
			ToolCallId:  ev.ToolCallID,
			ToolName:    ev.ToolName,
			Status:      firstNonEmptyString(ev.Status, "completed"),
			Correlation: chatapp.CorrelationInfoFromEvent(ev),
		})
	default:
		return false, nil
	}
}

// ProjectUI projects tool call backend events into compatibility UI events.
func (p *ToolCallPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case EventToolCallStarted:
		payload, ok := toolCallUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ChatToolCallStarted{}, ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: UIToolCallStarted, Payload: payload}}, true, nil
	case EventToolCallArgumentsDelta, EventToolExecutionStarted:
		payload, ok := toolCallUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ToolCallUpdate{}, ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: UIToolCallUpdated, Payload: payload}}, true, nil
	case EventToolCallRequested:
		payload, ok := toolCallUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ChatToolCallRequested{}, ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: UIToolCallStarted, Payload: payload}}, true, nil
	case EventToolCallFinished:
		payload, ok := toolCallUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ChatToolCallFinished{}, ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: UIToolCallFinished, Payload: payload}}, true, nil
	case EventToolResultReady:
		payload, ok := toolResultUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ChatToolResultReady{}, ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: UIToolResultReady, Payload: payload}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects tool call backend events into timeline entities.
func (p *ToolCallPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	switch ev.Name {
	case EventToolCallStarted, EventToolCallArgumentsDelta, EventToolCallRequested, EventToolExecutionStarted, EventToolCallFinished:
		payload, ok := toolCallUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ToolCallUpdate{}, ev.Payload)
		}
		existing := mergeToolCallUpdate(currentToolCallEntity(view, payload.GetToolCallId()), payload)
		existing.Executing = payload.GetExecuting()
		if payload.GetStatus() != "" {
			existing.Status = payload.GetStatus()
		}
		if ev.Name == EventToolCallFinished {
			existing.Status = "completed"
			existing.Executing = false
		}
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityToolCall, Id: payload.GetToolCallId(), Payload: existing}}, true, nil
	case EventToolResultReady:
		payload, ok := toolResultUpdateFromCanonical(ev)
		if !ok {
			return nil, true, unexpectedToolPayload(&chatappv1.ChatToolResultReady{}, ev.Payload)
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

func mergeToolCallUpdate(entity *chatappv1.ToolCallEntity, update *chatappv1.ToolCallUpdate) *chatappv1.ToolCallEntity {
	if entity == nil {
		entity = &chatappv1.ToolCallEntity{}
	}
	if update == nil {
		return entity
	}
	if update.GetMessageId() != "" {
		entity.MessageId = update.GetMessageId()
	}
	if update.GetToolCallId() != "" {
		entity.ToolCallId = update.GetToolCallId()
	}
	if update.GetToolName() != "" {
		entity.ToolName = update.GetToolName()
	}
	if update.GetInput() != "" {
		entity.Input = update.GetInput()
	}
	return entity
}

func toolCallUpdateFromCanonical(ev sessionstream.Event) (*chatappv1.ToolCallUpdate, bool) {
	switch payload := ev.Payload.(type) {
	case *chatappv1.ChatToolCallStarted:
		return toolCallUpdateFromCorrelation(&chatappv1.ToolCallUpdate{MessageId: payload.GetMessageId(), ToolCallId: payload.GetToolCallId(), ToolName: payload.GetToolName(), Status: firstNonEmptyString(payload.GetStatus(), "pending")}, payload.GetCorrelation()), true
	case *chatappv1.ChatToolCallArgumentsDelta:
		return toolCallUpdateFromCorrelation(&chatappv1.ToolCallUpdate{MessageId: payload.GetMessageId(), ToolCallId: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Status: firstNonEmptyString(payload.GetStatus(), "streaming_args")}, payload.GetCorrelation()), true
	case *chatappv1.ChatToolCallRequested:
		return toolCallUpdateFromCorrelation(&chatappv1.ToolCallUpdate{MessageId: payload.GetMessageId(), ToolCallId: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Executing: payload.GetExecuting(), Status: firstNonEmptyString(payload.GetStatus(), "pending")}, payload.GetCorrelation()), true
	case *chatappv1.ChatToolExecutionStarted:
		return toolCallUpdateFromCorrelation(&chatappv1.ToolCallUpdate{MessageId: payload.GetMessageId(), ToolCallId: payload.GetToolCallId(), ToolName: payload.GetToolName(), Input: payload.GetInput(), Executing: payload.GetExecuting(), Status: firstNonEmptyString(payload.GetStatus(), "executing")}, payload.GetCorrelation()), true
	case *chatappv1.ChatToolCallFinished:
		return toolCallUpdateFromCorrelation(&chatappv1.ToolCallUpdate{MessageId: payload.GetMessageId(), ToolCallId: payload.GetToolCallId(), ToolName: payload.GetToolName(), Executing: false, Status: firstNonEmptyString(payload.GetStatus(), "completed")}, payload.GetCorrelation()), true
	default:
		return nil, false
	}
}

func toolResultUpdateFromCanonical(ev sessionstream.Event) (*chatappv1.ToolResultUpdate, bool) {
	payload, ok := ev.Payload.(*chatappv1.ChatToolResultReady)
	if !ok || payload == nil {
		return nil, false
	}
	return toolResultUpdateFromCorrelation(&chatappv1.ToolResultUpdate{MessageId: payload.GetMessageId(), ToolCallId: payload.GetToolCallId(), ToolName: payload.GetToolName(), Result: payload.GetResult(), Status: firstNonEmptyString(payload.GetStatus(), "success")}, payload.GetCorrelation()), true
}

func toolCallUpdateFromCorrelation(update *chatappv1.ToolCallUpdate, corr *chatappv1.CorrelationInfo) *chatappv1.ToolCallUpdate {
	if update == nil || corr == nil {
		return update
	}
	if update.ToolCallId == "" {
		update.ToolCallId = corr.GetToolCallId()
	}
	update.Provider = corr.GetProvider()
	update.ResponseId = corr.GetResponseId()
	update.ChoiceIndex = cloneInt32Ptr(corr.ChoiceIndex)
	update.StreamKind = firstNonEmptyString(corr.GetStreamKind(), "tool_call")
	update.CorrelationKey = corr.GetCorrelationKey()
	update.ToolCallIndex = cloneInt32Ptr(corr.ToolCallIndex)
	return update
}

func toolResultUpdateFromCorrelation(update *chatappv1.ToolResultUpdate, corr *chatappv1.CorrelationInfo) *chatappv1.ToolResultUpdate {
	if update == nil || corr == nil {
		return update
	}
	if update.ToolCallId == "" {
		update.ToolCallId = corr.GetToolCallId()
	}
	update.Provider = corr.GetProvider()
	update.ResponseId = corr.GetResponseId()
	update.ChoiceIndex = cloneInt32Ptr(corr.ChoiceIndex)
	update.StreamKind = firstNonEmptyString(corr.GetStreamKind(), "tool_call")
	update.CorrelationKey = corr.GetCorrelationKey()
	update.ToolCallIndex = cloneInt32Ptr(corr.ToolCallIndex)
	return update
}

func unexpectedToolPayload(expected proto.Message, actual any) error {
	return fmt.Errorf("tool payload must be %T, got %T", expected, actual)
}

// Ensure ToolCallPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ToolCallPlugin)(nil)
