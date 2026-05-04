package main

import (
	"context"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
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

type reasoningPlugin struct{}

func newReasoningPlugin() chatapp.ChatPlugin {
	return reasoningPlugin{}
}

func (reasoningPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
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

func (reasoningPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	reasoningMessageID := reasoningEntityID(runtime.MessageID)
	if reasoningMessageID == "" {
		return false, nil
	}

	switch ev := event.(type) {
	case *gepevents.EventThinkingPartial:
		pb, err := structpb.NewStruct(map[string]any{
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
		if err != nil {
			return true, err
		}
		return true, runtime.Publish(ctx, reasoningDeltaEventName, pb)
	case *gepevents.EventInfo:
		switch ev.Message {
		case "thinking-started":
			pb, err := structpb.NewStruct(map[string]any{
				"messageId":       reasoningMessageID,
				"parentMessageId": runtime.MessageID,
				"role":            "thinking",
				"status":          "streaming",
				"streaming":       true,
				"source":          "thinking",
			})
			if err != nil {
				return true, err
			}
			return true, runtime.Publish(ctx, reasoningStartedEventName, pb)
		case "thinking-ended":
			pb, err := structpb.NewStruct(map[string]any{
				"messageId":       reasoningMessageID,
				"parentMessageId": runtime.MessageID,
				"role":            "thinking",
				"status":          "finished",
				"streaming":       false,
				"source":          "thinking",
			})
			if err != nil {
				return true, err
			}
			return true, runtime.Publish(ctx, reasoningFinishedEventName, pb)
		case "reasoning-summary":
			pb, err := structpb.NewStruct(map[string]any{
				"messageId":       reasoningMessageID,
				"parentMessageId": runtime.MessageID,
				"role":            "thinking",
				"content":         infoText(ev.Data),
				"text":            infoText(ev.Data),
				"status":          "finished",
				"streaming":       false,
				"source":          "summary",
			})
			if err != nil {
				return true, err
			}
			return true, runtime.Publish(ctx, reasoningFinishedEventName, pb)
		default:
			return false, nil
		}
	default:
		return false, nil
	}
}

func (reasoningPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
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

func (reasoningPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	messageID := asString(payload["messageId"])
	if messageID == "" {
		return nil, true, nil
	}
	entity, hadEntity := currentReasoningEntity(view, messageID)
	content := asString(payload["content"])
	if content == "" {
		content = entity.GetContent()
		if content == "" {
			content = entity.GetText()
		}
	}
	if content == "" && !hadEntity {
		return nil, true, nil
	}

	entity.MessageId = messageID
	entity.Role = "thinking"
	entity.Content = content
	entity.Text = content

	switch ev.Name {
	case reasoningStartedEventName, reasoningDeltaEventName:
		entity.Status = "streaming"
		entity.Streaming = true
	case reasoningFinishedEventName:
		entity.Status = "finished"
		entity.Streaming = false
	default:
		return nil, false, nil
	}

	return []sessionstream.TimelineEntity{{Kind: chatapp.TimelineEntityChatMessage, Id: messageID, Payload: entity}}, true, nil
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
				current, _ := currentReasoningEntity(view, messageID)
				if currentContent := current.GetContent(); asString(payload["content"]) == "" && currentContent != "" {
					payload["content"] = currentContent
					payload["text"] = currentContent
				} else if currentText := current.GetText(); asString(payload["content"]) == "" && currentText != "" {
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

func currentReasoningEntity(view sessionstream.TimelineView, id string) (*chatappv1.ChatMessageEntity, bool) {
	if view == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	entity, ok := view.Get(chatapp.TimelineEntityChatMessage, id)
	if !ok || entity.Payload == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	pb, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
	if !ok || pb == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	return &chatappv1.ChatMessageEntity{
		MessageId: pb.GetMessageId(),
		Role:      pb.GetRole(),
		Prompt:    pb.GetPrompt(),
		Text:      pb.GetText(),
		Content:   pb.GetContent(),
		Status:    pb.GetStatus(),
		Streaming: pb.GetStreaming(),
		Error:     pb.GetError(),
	}, true
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
