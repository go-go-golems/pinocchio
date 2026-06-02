package agentmodeplugin

import (
	"context"
	"fmt"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

const (
	agentModePreviewEventName   = "ChatAgentModePreviewUpdated"
	agentModeCommittedEventName = "ChatAgentModeCommitted"
	agentModePreviewUIName      = "ChatAgentModePreviewUpdated"
	agentModeCommittedUIName    = "ChatAgentModeCommitted"
	agentModePreviewClearUIName = "ChatAgentModePreviewCleared"
	agentModeTimelineEntityKind = "AgentMode"
)

// agentModePlugin is web-chat-app-local glue for the demo/product-specific
// agent-mode middleware. Keep it out of reusable chatapp plugin packages unless
// the protobuf and UI contracts become a shared Pinocchio feature.
type agentModePlugin struct{}

func NewPlugin() chatapp.ChatPlugin {
	return agentModePlugin{}
}

func (agentModePlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(agentModePreviewEventName, &chatappv1.AgentModePreviewUpdate{}),
		reg.RegisterEvent(agentModeCommittedEventName, &chatappv1.AgentModeCommittedUpdate{}),
		reg.RegisterUIEvent(agentModePreviewUIName, &chatappv1.AgentModePreviewUpdate{}),
		reg.RegisterUIEvent(agentModeCommittedUIName, &chatappv1.AgentModeCommittedUpdate{}),
		reg.RegisterUIEvent(agentModePreviewClearUIName, &chatappv1.AgentModePreviewCleared{}),
		reg.RegisterTimelineEntity(agentModeTimelineEntityKind, &chatappv1.AgentModeEntity{}),
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
		return true, runtime.Publish(ctx, agentModePreviewEventName, &chatappv1.AgentModePreviewUpdate{
			MessageId:     runtime.MessageID,
			CandidateMode: ev.CandidateMode,
			Analysis:      ev.Analysis,
			ParseState:    ev.ParseState,
			Preview:       true,
		})
	case *gepevents.EventAgentModeSwitch:
		return true, runtime.Publish(ctx, agentModeCommittedEventName, &chatappv1.AgentModeCommittedUpdate{
			MessageId: runtime.MessageID,
			Title:     ev.Message,
			From:      eventStringData(ev.Data, "from"),
			To:        eventStringData(ev.Data, "to"),
			Analysis:  eventStringData(ev.Data, "analysis"),
			Preview:   false,
		})
	default:
		return false, nil
	}
}

func (agentModePlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case agentModePreviewEventName:
		payload, ok := ev.Payload.(*chatappv1.AgentModePreviewUpdate)
		if !ok || payload == nil {
			return nil, true, unexpectedAgentModePayload(&chatappv1.AgentModePreviewUpdate{}, ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: agentModePreviewUIName, Payload: payload}}, true, nil
	case agentModeCommittedEventName:
		payload, ok := ev.Payload.(*chatappv1.AgentModeCommittedUpdate)
		if !ok || payload == nil {
			return nil, true, unexpectedAgentModePayload(&chatappv1.AgentModeCommittedUpdate{}, ev.Payload)
		}
		clearPB := &chatappv1.AgentModePreviewCleared{MessageId: payload.GetMessageId()}
		return []sessionstream.UIEvent{{Name: agentModeCommittedUIName, Payload: payload}, {Name: agentModePreviewClearUIName, Payload: clearPB}}, true, nil
	case chatapp.EventChatTextSegmentFinished:
		payload, ok := ev.Payload.(*chatappv1.ChatTextSegmentFinished)
		if !ok || payload == nil {
			return nil, true, unexpectedAgentModePayload(&chatappv1.ChatTextSegmentFinished{}, ev.Payload)
		}
		clearPB := &chatappv1.AgentModePreviewCleared{MessageId: payload.GetMessageId()}
		return []sessionstream.UIEvent{{Name: agentModePreviewClearUIName, Payload: clearPB}}, true, nil
	case chatapp.EventChatRunStopped, chatapp.EventChatRunFailed:
		payload, ok := ev.Payload.(interface{ GetMessageId() string })
		if !ok || payload == nil {
			return nil, true, unexpectedAgentModePayload(&chatappv1.ChatRunStopped{}, ev.Payload)
		}
		clearPB := &chatappv1.AgentModePreviewCleared{MessageId: payload.GetMessageId()}
		return []sessionstream.UIEvent{{Name: agentModePreviewClearUIName, Payload: clearPB}}, true, nil
	default:
		return nil, false, nil
	}
}

func (agentModePlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	if ev.Name != agentModeCommittedEventName {
		return nil, false, nil
	}
	payload, ok := ev.Payload.(*chatappv1.AgentModeCommittedUpdate)
	if !ok || payload == nil {
		return nil, true, unexpectedAgentModePayload(&chatappv1.AgentModeCommittedUpdate{}, ev.Payload)
	}
	entity := &chatappv1.AgentModeEntity{
		MessageId: payload.GetMessageId(),
		Title:     payload.GetTitle(),
		From:      payload.GetFrom(),
		To:        payload.GetTo(),
		Analysis:  payload.GetAnalysis(),
		Preview:   false,
	}
	return []sessionstream.TimelineEntity{{Kind: agentModeTimelineEntityKind, Id: payload.GetMessageId(), Payload: entity}}, true, nil
}

func eventStringData(data map[string]interface{}, key string) string {
	if s, ok := data[key].(string); ok {
		return s
	}
	return ""
}

func unexpectedAgentModePayload(want any, got any) error {
	return fmt.Errorf("agent mode payload must be %T, got %T", want, got)
}
