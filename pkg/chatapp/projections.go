package chatapp

import (
	"context"
	"strings"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

func baseUIProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, error) {
	payload, ok := ev.Payload.(*chatappv1.ChatMessageUpdate)
	if !ok || payload == nil {
		return nil, nil
	}
	cloned := proto.Clone(payload)
	switch ev.Name {
	case EventUserMessageAccepted:
		return []sessionstream.UIEvent{{Name: UIMessageAccepted, Payload: cloned}}, nil
	case EventInferenceStarted:
		return []sessionstream.UIEvent{{Name: UIMessageStarted, Payload: cloned}}, nil
	case EventTokensDelta:
		return []sessionstream.UIEvent{{Name: UIMessageAppended, Payload: cloned}}, nil
	case EventInferenceFinished:
		return []sessionstream.UIEvent{{Name: UIMessageFinished, Payload: cloned}}, nil
	case EventInferenceStopped:
		return []sessionstream.UIEvent{{Name: UIMessageStopped, Payload: cloned}}, nil
	default:
		return nil, nil
	}
}

func baseTimelineProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, error) {
	payload, ok := ev.Payload.(*chatappv1.ChatMessageUpdate)
	if !ok || payload == nil {
		return nil, nil
	}
	messageID := strings.TrimSpace(payload.GetMessageId())
	if messageID == "" {
		return nil, nil
	}
	entity, hadEntity := currentChatMessageEntity(view, messageID)
	switch ev.Name {
	case EventUserMessageAccepted:
		entity.MessageId = messageID
		entity.Role = "user"
		entity.Content = firstNonEmpty(payload.GetContent(), payload.GetText())
		entity.Text = entity.Content
		entity.Streaming = false
	case EventInferenceStarted:
		content := firstNonEmpty(payload.GetContent(), payload.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Status = "streaming"
		entity.Streaming = true
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
		if content != "" {
			entity.Content = content
			entity.Text = content
		}
	case EventTokensDelta:
		content := firstNonEmpty(payload.GetContent(), payload.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = "streaming"
		entity.Streaming = true
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
	case EventInferenceFinished:
		content := firstNonEmpty(payload.GetContent(), payload.GetText(), entity.GetContent(), entity.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = "finished"
		entity.Streaming = false
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
	case EventInferenceStopped:
		content := firstNonEmpty(payload.GetContent(), payload.GetText(), entity.GetContent(), entity.GetText())
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = "stopped"
		entity.Streaming = false
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
		if errText := payload.GetError(); errText != "" {
			entity.Error = errText
		}
	default:
		return nil, nil
	}
	entity.ParentMessageId = payload.GetParentMessageId()
	entity.Segment = payload.GetSegment()
	entity.SegmentType = payload.GetSegmentType()
	entity.Final = payload.GetFinal()
	return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
}

func currentChatMessageEntity(view sessionstream.TimelineView, id string) (*chatappv1.ChatMessageEntity, bool) {
	entity, ok := view.Get(TimelineEntityChatMessage, id)
	if !ok || entity.Payload == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	pb, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
	if !ok || pb == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	return proto.Clone(pb).(*chatappv1.ChatMessageEntity), true
}
