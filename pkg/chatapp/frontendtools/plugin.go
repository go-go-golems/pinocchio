package frontendtools

import (
	"context"
	"fmt"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

type Plugin struct{}

func NewPlugin() chatapp.ChatPlugin { return &Plugin{} }

func (p *Plugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	return RegisterSchemas(reg)
}

func (p *Plugin) HandleRuntimeEvent(_ context.Context, _ chatapp.RuntimeEventContext, _ gepevents.Event) (bool, error) {
	return false, nil
}

func (p *Plugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case EventCallRequested, EventResultReceived:
		if ev.Payload == nil {
			return nil, true, fmt.Errorf("frontend tool payload must be proto message, got nil")
		}
		return []sessionstream.UIEvent{{Name: ev.Name, Payload: proto.Clone(ev.Payload)}}, true, nil
	default:
		return nil, false, nil
	}
}

func (p *Plugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	switch ev.Name {
	case EventCallRequested:
		payload, ok := ev.Payload.(*toolv1.FrontendToolCallRequested)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("unexpected FrontendToolCallRequested payload %T", ev.Payload)
		}
		return []sessionstream.TimelineEntity{{
			Kind: TimelineEntityFrontendToolCall,
			Id:   payload.ToolCallId,
			Payload: &toolv1.FrontendToolCallEntity{
				ToolCallId:      payload.ToolCallId,
				ToolName:        payload.ToolName,
				ParentMessageId: payload.MessageId,
				Mode:            payload.Mode,
				Status:          firstNonEmpty(payload.Status, "requested"),
				Input:           payload.Input,
			},
		}}, true, nil

	case EventResultReceived:
		payload, ok := ev.Payload.(*toolv1.FrontendToolResultReceived)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("unexpected FrontendToolResultReceived payload %T", ev.Payload)
		}
		entity := currentToolEntity(view, payload.ToolCallId)
		entity.ToolCallId = payload.ToolCallId
		if payload.ToolName != "" {
			entity.ToolName = payload.ToolName
		}
		if payload.MessageId != "" {
			entity.ParentMessageId = payload.MessageId
		}
		entity.Result = payload.Result
		entity.Status = firstNonEmpty(payload.Status, "success")
		entity.Error = payload.Error
		return []sessionstream.TimelineEntity{{
			Kind:    TimelineEntityFrontendToolCall,
			Id:      payload.ToolCallId,
			Payload: entity,
		}}, true, nil

	default:
		return nil, false, nil
	}
}

func currentToolEntity(view sessionstream.TimelineView, id string) *toolv1.FrontendToolCallEntity {
	if view == nil {
		return &toolv1.FrontendToolCallEntity{ToolCallId: id}
	}
	entity, ok := view.Get(TimelineEntityFrontendToolCall, id)
	if !ok || entity.Payload == nil {
		return &toolv1.FrontendToolCallEntity{ToolCallId: id}
	}
	pb, ok := entity.Payload.(*toolv1.FrontendToolCallEntity)
	if !ok || pb == nil {
		return &toolv1.FrontendToolCallEntity{ToolCallId: id}
	}
	return proto.Clone(pb).(*toolv1.FrontendToolCallEntity)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var _ chatapp.ChatPlugin = (*Plugin)(nil)
