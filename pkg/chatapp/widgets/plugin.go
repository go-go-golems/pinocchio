package widgets

import (
	"context"
	"fmt"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	widgetv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/widgets/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// Backend events (published by the mock engine).
	EventWidgetInstanceStarted   = "ChatWidgetInstanceStarted"
	EventWidgetInstancePatched   = "ChatWidgetInstancePatched"
	EventWidgetInstanceCompleted = "ChatWidgetInstanceCompleted"
	EventWidgetInstanceRemoved   = "ChatWidgetInstanceRemoved"

	// Client command (sent from frontend when user interacts with a widget).
	CommandWidgetAction = "ChatWidgetAction"

	// Timeline entity kind.
	TimelineEntityWidgetInstance = "ChatWidgetInstance"
)

// WidgetPlugin implements chatapp.ChatPlugin to handle widget events.
type WidgetPlugin struct{}

// NewWidgetPlugin creates a new WidgetPlugin.
func NewWidgetPlugin() chatapp.ChatPlugin { return &WidgetPlugin{} }

// RegisterSchemas registers widget event, UI event, and timeline entity schemas.
func (p *WidgetPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		// Backend events
		reg.RegisterEvent(EventWidgetInstanceStarted, &widgetv1.WidgetInstanceStarted{}),
		reg.RegisterEvent(EventWidgetInstancePatched, &widgetv1.WidgetInstancePatched{}),
		reg.RegisterEvent(EventWidgetInstanceCompleted, &widgetv1.WidgetInstanceCompleted{}),
		reg.RegisterEvent(EventWidgetInstanceRemoved, &widgetv1.WidgetInstanceRemoved{}),
		// UI events (same payloads as backend events, delivered live to WebSocket clients)
		reg.RegisterUIEvent(EventWidgetInstanceStarted, &widgetv1.WidgetInstanceStarted{}),
		reg.RegisterUIEvent(EventWidgetInstancePatched, &widgetv1.WidgetInstancePatched{}),
		reg.RegisterUIEvent(EventWidgetInstanceCompleted, &widgetv1.WidgetInstanceCompleted{}),
		reg.RegisterUIEvent(EventWidgetInstanceRemoved, &widgetv1.WidgetInstanceRemoved{}),
		// Timeline entity
		reg.RegisterTimelineEntity(TimelineEntityWidgetInstance, &widgetv1.WidgetInstanceEntity{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// HandleRuntimeEvent is not used for widgets — widget events are published
// directly by the engine, not translated from Geppetto events.
func (p *WidgetPlugin) HandleRuntimeEvent(_ context.Context, _ chatapp.RuntimeEventContext, _ gepevents.Event) (bool, error) {
	return false, nil
}

// ProjectUI forwards widget backend events as live UI events.
func (p *WidgetPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case EventWidgetInstanceStarted, EventWidgetInstancePatched, EventWidgetInstanceCompleted, EventWidgetInstanceRemoved:
		if ev.Payload == nil {
			return nil, true, fmt.Errorf("widget payload must be proto message, got nil")
		}
		return []sessionstream.UIEvent{{Name: ev.Name, Payload: proto.Clone(ev.Payload)}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects widget backend events into durable timeline entities.
func (p *WidgetPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	switch ev.Name {
	case EventWidgetInstanceStarted:
		payload, ok := ev.Payload.(*widgetv1.WidgetInstanceStarted)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("unexpected WidgetInstanceStarted payload %T", ev.Payload)
		}
		return []sessionstream.TimelineEntity{{
			Kind: TimelineEntityWidgetInstance,
			Id:   payload.InstanceId,
			Payload: &widgetv1.WidgetInstanceEntity{
				InstanceId:      payload.InstanceId,
				WidgetName:      payload.WidgetName,
				ParentMessageId: payload.ParentMessageId,
				Status:          payload.Status,
				Props:           payload.Props,
			},
		}}, true, nil

	case EventWidgetInstancePatched:
		payload, ok := ev.Payload.(*widgetv1.WidgetInstancePatched)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("unexpected WidgetInstancePatched payload %T", ev.Payload)
		}
		// Merge patch into existing entity
		existing := currentWidgetEntity(view, payload.InstanceId)
		if existing != nil && payload.Patch != nil {
			mergeStructPatch(existing, payload.Patch, payload.PatchPaths)
		}
		if payload.Status != widgetv1.WidgetStatus_WIDGET_STATUS_UNSPECIFIED {
			existing.Status = payload.Status
		}
		return []sessionstream.TimelineEntity{{
			Kind:    TimelineEntityWidgetInstance,
			Id:      payload.InstanceId,
			Payload: existing,
		}}, true, nil

	case EventWidgetInstanceCompleted:
		payload, ok := ev.Payload.(*widgetv1.WidgetInstanceCompleted)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("unexpected WidgetInstanceCompleted payload %T", ev.Payload)
		}
		existing := currentWidgetEntity(view, payload.InstanceId)
		if payload.Status != widgetv1.WidgetStatus_WIDGET_STATUS_UNSPECIFIED {
			existing.Status = payload.Status
		} else {
			existing.Status = widgetv1.WidgetStatus_WIDGET_STATUS_READY
		}
		return []sessionstream.TimelineEntity{{
			Kind:    TimelineEntityWidgetInstance,
			Id:      payload.InstanceId,
			Payload: existing,
		}}, true, nil

	case EventWidgetInstanceRemoved:
		payload, ok := ev.Payload.(*widgetv1.WidgetInstanceRemoved)
		if !ok || payload == nil {
			return nil, true, fmt.Errorf("unexpected WidgetInstanceRemoved payload %T", ev.Payload)
		}
		return []sessionstream.TimelineEntity{{
			Kind:      TimelineEntityWidgetInstance,
			Id:        payload.InstanceId,
			Tombstone: true,
		}}, true, nil

	default:
		return nil, false, nil
	}
}

// currentWidgetEntity returns the current widget entity from the timeline view,
// or a fresh entity if none exists.
func currentWidgetEntity(view sessionstream.TimelineView, id string) *widgetv1.WidgetInstanceEntity {
	if view == nil {
		return &widgetv1.WidgetInstanceEntity{InstanceId: id}
	}
	entity, ok := view.Get(TimelineEntityWidgetInstance, id)
	if !ok || entity.Payload == nil {
		return &widgetv1.WidgetInstanceEntity{InstanceId: id}
	}
	pb, ok := entity.Payload.(*widgetv1.WidgetInstanceEntity)
	if !ok || pb == nil {
		return &widgetv1.WidgetInstanceEntity{InstanceId: id}
	}
	return proto.Clone(pb).(*widgetv1.WidgetInstanceEntity)
}

// mergeStructPatch merges a patch Struct into the existing props Struct.
// If patchPaths is specified, only those paths are updated.
// Otherwise, all fields from patch are merged into existing.
func mergeStructPatch(existing *widgetv1.WidgetInstanceEntity, patch *structpb.Struct, paths []string) {
	if existing.Props == nil {
		existing.Props = patch
		return
	}
	if len(paths) == 0 {
		// Full merge: copy all patch fields over existing
		for k, v := range patch.Fields {
			existing.Props.Fields[k] = v
		}
	} else {
		// Selective merge
		for _, path := range paths {
			if v, ok := patch.Fields[path]; ok {
				existing.Props.Fields[path] = v
			}
		}
	}
}

// Ensure WidgetPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*WidgetPlugin)(nil)
