package main

import (
	"context"
	"fmt"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	agentModePreviewEventName   = "ChatAgentModePreviewUpdated"
	agentModeCommittedEventName = "ChatAgentModeCommitted"
	agentModePreviewUIName      = "ChatAgentModePreviewUpdated"
	agentModeCommittedUIName    = "ChatAgentModeCommitted"
	agentModePreviewClearUIName = "ChatAgentModePreviewCleared"
	agentModeTimelineEntityKind = "AgentMode"
)

type agentModePlugin struct{}

func newAgentModePlugin() chatapp.ChatPlugin {
	return agentModePlugin{}
}

func (agentModePlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(agentModePreviewEventName, &structpb.Struct{}),
		reg.RegisterEvent(agentModeCommittedEventName, &structpb.Struct{}),
		reg.RegisterUIEvent(agentModePreviewUIName, &structpb.Struct{}),
		reg.RegisterUIEvent(agentModeCommittedUIName, &structpb.Struct{}),
		reg.RegisterUIEvent(agentModePreviewClearUIName, &structpb.Struct{}),
		reg.RegisterTimelineEntity(agentModeTimelineEntityKind, &structpb.Struct{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

func (agentModePlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	switch ev := event.(type) {
	case *agentmode.EventModeSwitchPreview:
		pb, err := structpb.NewStruct(map[string]any{
			"messageId":     runtime.MessageID,
			"candidateMode": ev.CandidateMode,
			"analysis":      ev.Analysis,
			"parseState":    ev.ParseState,
			"preview":       true,
		})
		if err != nil {
			return true, err
		}
		return true, runtime.Publish(ctx, agentModePreviewEventName, pb)
	case *gepevents.EventAgentModeSwitch:
		payload := map[string]any{
			"messageId": runtime.MessageID,
			"title":     ev.Message,
			"preview":   false,
		}
		for k, v := range ev.Data {
			payload[k] = v
		}
		pb, err := structpb.NewStruct(payload)
		if err != nil {
			return true, err
		}
		return true, runtime.Publish(ctx, agentModeCommittedEventName, pb)
	default:
		return false, nil
	}
}

func (agentModePlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	payload := payloadWithOrdinal(ev)
	switch ev.Name {
	case agentModePreviewEventName:
		pb, err := structpb.NewStruct(payload)
		if err != nil {
			return nil, true, err
		}
		return []sessionstream.UIEvent{{Name: agentModePreviewUIName, Payload: pb}}, true, nil
	case agentModeCommittedEventName:
		pb, err := structpb.NewStruct(payload)
		if err != nil {
			return nil, true, err
		}
		clearPB, err := previewClearPayload(ev)
		if err != nil {
			return nil, true, err
		}
		return []sessionstream.UIEvent{{Name: agentModeCommittedUIName, Payload: pb}, {Name: agentModePreviewClearUIName, Payload: clearPB}}, true, nil
	case chatapp.EventInferenceFinished, chatapp.EventInferenceStopped:
		clearPB, err := previewClearPayload(ev)
		if err != nil {
			return nil, true, err
		}
		return []sessionstream.UIEvent{{Name: agentModePreviewClearUIName, Payload: clearPB}}, true, nil
	default:
		return nil, false, nil
	}
}

func (agentModePlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	if ev.Name != agentModeCommittedEventName {
		return nil, false, nil
	}
	payload := toMap(ev.Payload)
	entity := currentKindEntity(view, agentModeTimelineEntityKind, "session")
	entity["messageId"] = asString(payload["messageId"])
	entity["title"] = asString(payload["title"])
	entity["preview"] = false
	data := map[string]any{}
	for k, v := range payload {
		switch k {
		case "messageId", "title", "ordinal", "preview":
			continue
		default:
			data[k] = v
		}
	}
	entity["data"] = data
	pb, err := structpb.NewStruct(entity)
	if err != nil {
		return nil, true, err
	}
	return []sessionstream.TimelineEntity{{Kind: agentModeTimelineEntityKind, Id: "session", Payload: pb}}, true, nil
}

func payloadWithOrdinal(ev sessionstream.Event) map[string]any {
	payload := toMap(ev.Payload)
	payload["ordinal"] = fmt.Sprintf("%d", ev.Ordinal)
	return payload
}

func previewClearPayload(ev sessionstream.Event) (*structpb.Struct, error) {
	payload := toMap(ev.Payload)
	return structpb.NewStruct(map[string]any{
		"messageId": asString(payload["messageId"]),
		"ordinal":   fmt.Sprintf("%d", ev.Ordinal),
	})
}

func toMap(msg any) map[string]any {
	if pb, ok := msg.(*structpb.Struct); ok && pb != nil {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func currentKindEntity(view sessionstream.TimelineView, kind, id string) map[string]any {
	entity, ok := view.Get(kind, id)
	if !ok || entity.Payload == nil {
		return map[string]any{}
	}
	if pb, ok := entity.Payload.(*structpb.Struct); ok {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}
