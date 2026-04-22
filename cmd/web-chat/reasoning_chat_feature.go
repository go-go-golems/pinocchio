package main

import (
	"context"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	reasoningStartedEventName  = "ChatReasoningStarted"
	reasoningDeltaEventName    = "ChatReasoningDelta"
	reasoningFinishedEventName = "ChatReasoningFinished"

	reasoningStartedUIName  = "ChatReasoningStarted"
	reasoningAppendedUIName = "ChatReasoningAppended"
	reasoningFinishedUIName = "ChatReasoningFinished"
)

type reasoningChatFeature struct{}

func newReasoningChatFeature() chatapp.FeatureSet {
	return reasoningChatFeature{}
}

func (reasoningChatFeature) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(reasoningStartedEventName, &structpb.Struct{}),
		reg.RegisterEvent(reasoningDeltaEventName, &structpb.Struct{}),
		reg.RegisterEvent(reasoningFinishedEventName, &structpb.Struct{}),
		reg.RegisterUIEvent(reasoningStartedUIName, &structpb.Struct{}),
		reg.RegisterUIEvent(reasoningAppendedUIName, &structpb.Struct{}),
		reg.RegisterUIEvent(reasoningFinishedUIName, &structpb.Struct{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

func (reasoningChatFeature) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	reasoningMessageID := reasoningEntityID(runtime.MessageID)
	if reasoningMessageID == "" {
		return false, nil
	}

	switch ev := event.(type) {
	case *gepevents.EventThinkingPartial:
		return true, runtime.Publish(ctx, reasoningDeltaEventName, map[string]any{
			"messageId":       reasoningMessageID,
			"parentMessageId": runtime.MessageID,
			"role":            "thinking",
			"chunk":           ev.Delta,
			"content":         ev.Completion,
			"text":            ev.Completion,
			"status":          "streaming",
			"streaming":       true,
			"source":          "thinking",
		})
	case *gepevents.EventInfo:
		switch ev.Message {
		case "thinking-started":
			return true, runtime.Publish(ctx, reasoningStartedEventName, map[string]any{
				"messageId":       reasoningMessageID,
				"parentMessageId": runtime.MessageID,
				"role":            "thinking",
				"status":          "streaming",
				"streaming":       true,
				"source":          "thinking",
			})
		case "thinking-ended":
			return true, runtime.Publish(ctx, reasoningFinishedEventName, map[string]any{
				"messageId":       reasoningMessageID,
				"parentMessageId": runtime.MessageID,
				"role":            "thinking",
				"status":          "finished",
				"streaming":       false,
				"source":          "thinking",
			})
		case "reasoning-summary":
			return true, runtime.Publish(ctx, reasoningFinishedEventName, map[string]any{
				"messageId":       reasoningMessageID,
				"parentMessageId": runtime.MessageID,
				"role":            "thinking",
				"content":         infoText(ev.Data),
				"text":            infoText(ev.Data),
				"status":          "finished",
				"streaming":       false,
				"source":          "summary",
			})
		default:
			return false, nil
		}
	default:
		return false, nil
	}
}

func (reasoningChatFeature) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, true, err
	}
	switch ev.Name {
	case reasoningStartedEventName:
		return []sessionstream.UIEvent{{Name: reasoningStartedUIName, Payload: pb}}, true, nil
	case reasoningDeltaEventName:
		return []sessionstream.UIEvent{{Name: reasoningAppendedUIName, Payload: pb}}, true, nil
	case reasoningFinishedEventName:
		return []sessionstream.UIEvent{{Name: reasoningFinishedUIName, Payload: pb}}, true, nil
	default:
		return nil, false, nil
	}
}

func (reasoningChatFeature) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	messageID := asString(payload["messageId"])
	if messageID == "" {
		return nil, true, nil
	}
	entity := currentKindEntity(view, chatapp.TimelineEntityChatMessage, messageID)
	content := asString(payload["content"])
	if content == "" {
		content = asString(entity["content"])
		if content == "" {
			content = asString(entity["text"])
		}
	}
	if content == "" && len(entity) == 0 {
		return nil, true, nil
	}

	entity["messageId"] = messageID
	entity["parentMessageId"] = asString(payload["parentMessageId"])
	entity["role"] = "thinking"
	entity["content"] = content
	entity["text"] = content
	if source := asString(payload["source"]); source != "" {
		entity["source"] = source
	}

	switch ev.Name {
	case reasoningStartedEventName, reasoningDeltaEventName:
		entity["status"] = "streaming"
		entity["streaming"] = true
	case reasoningFinishedEventName:
		entity["status"] = "finished"
		entity["streaming"] = false
	default:
		return nil, false, nil
	}

	pb, err := structpb.NewStruct(entity)
	if err != nil {
		return nil, true, err
	}
	return []sessionstream.TimelineEntity{{Kind: chatapp.TimelineEntityChatMessage, Id: messageID, Payload: pb}}, true, nil
}

func reasoningProjectedPayload(ev sessionstream.Event, view sessionstream.TimelineView) (map[string]any, bool) {
	switch ev.Name {
	case reasoningStartedEventName, reasoningDeltaEventName, reasoningFinishedEventName:
		payload := payloadWithOrdinal(ev)
		if asString(payload["role"]) == "" {
			payload["role"] = "thinking"
		}
		if view != nil {
			messageID := asString(payload["messageId"])
			if messageID != "" {
				current := currentKindEntity(view, chatapp.TimelineEntityChatMessage, messageID)
				if currentContent := asString(current["content"]); asString(payload["content"]) == "" && currentContent != "" {
					payload["content"] = currentContent
					payload["text"] = currentContent
				} else if currentText := asString(current["text"]); asString(payload["content"]) == "" && currentText != "" {
					payload["content"] = currentText
					payload["text"] = currentText
				}
			}
		}
		return payload, true
	default:
		return nil, false
	}
}

func reasoningEntityID(messageID string) string {
	messageID = asString(messageID)
	if messageID == "" {
		return ""
	}
	return messageID + ":thinking"
}

func infoText(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}
	if s, ok := data["text"].(string); ok {
		return s
	}
	return ""
}
