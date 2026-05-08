package chatapp

import (
	"context"
	"strings"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

func baseUIProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, error) {
	payload := chatMessageUpdateFromEvent(ev)
	if payload == nil {
		return nil, nil
	}
	cloned := proto.Clone(payload)
	switch ev.Name {
	case EventUserMessageAccepted:
		return []sessionstream.UIEvent{{Name: UIMessageAccepted, Payload: cloned}}, nil
	case EventChatTextSegmentStarted:
		return []sessionstream.UIEvent{{Name: UIMessageStarted, Payload: cloned}}, nil
	case EventChatTextDelta:
		return []sessionstream.UIEvent{{Name: UIMessageAppended, Payload: cloned}}, nil
	case EventChatTextSegmentFinished:
		return []sessionstream.UIEvent{{Name: UIMessageFinished, Payload: cloned}}, nil
	case EventChatRunStopped, EventChatRunFailed:
		return []sessionstream.UIEvent{{Name: UIMessageStopped, Payload: cloned}}, nil
	default:
		return nil, nil
	}
}

func baseTimelineProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, error) {
	payload := chatMessageUpdateFromEvent(ev)
	if payload == nil {
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
	case EventChatTextSegmentStarted:
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
	case EventChatTextDelta:
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
	case EventChatTextSegmentFinished:
		content := firstNonEmpty(payload.GetContent(), payload.GetText(), entity.GetContent(), entity.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = firstNonEmpty(payload.GetStatus(), "finished")
		entity.Streaming = false
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
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

func chatMessageUpdateFromEvent(ev sessionstream.Event) *chatappv1.ChatMessageUpdate {
	switch payload := ev.Payload.(type) {
	case *chatappv1.ChatMessageUpdate:
		return payload
	case *chatappv1.ChatTextSegmentStarted:
		segment := payload.GetCorrelation().GetSegmentIndex()
		return &chatappv1.ChatMessageUpdate{
			MessageId:       payload.GetMessageId(),
			Role:            payload.GetRole(),
			Prompt:          payload.GetPrompt(),
			Status:          payload.GetStatus(),
			Streaming:       payload.GetStreaming(),
			ParentMessageId: parentMessageIDFromSegmentMessageID(payload.GetMessageId()),
			Segment:         segment,
			SegmentType:     payload.GetCorrelation().GetSegmentType(),
		}
	case *chatappv1.ChatTextDelta:
		segment := payload.GetCorrelation().GetSegmentIndex()
		return &chatappv1.ChatMessageUpdate{
			MessageId:       payload.GetMessageId(),
			Role:            payload.GetRole(),
			Prompt:          payload.GetPrompt(),
			Chunk:           payload.GetChunk(),
			Text:            payload.GetText(),
			Content:         payload.GetContent(),
			Status:          payload.GetStatus(),
			Streaming:       payload.GetStreaming(),
			ParentMessageId: parentMessageIDFromSegmentMessageID(payload.GetMessageId()),
			Segment:         segment,
			SegmentType:     payload.GetCorrelation().GetSegmentType(),
		}
	case *chatappv1.ChatTextSegmentFinished:
		segment := payload.GetCorrelation().GetSegmentIndex()
		return &chatappv1.ChatMessageUpdate{
			MessageId:       payload.GetMessageId(),
			Role:            payload.GetRole(),
			Prompt:          payload.GetPrompt(),
			Text:            payload.GetText(),
			Content:         payload.GetContent(),
			Status:          payload.GetStatus(),
			Streaming:       payload.GetStreaming(),
			Final:           payload.GetFinal(),
			ParentMessageId: parentMessageIDFromSegmentMessageID(payload.GetMessageId()),
			Segment:         segment,
			SegmentType:     payload.GetCorrelation().GetSegmentType(),
		}
	case *chatappv1.ChatRunStopped:
		return newChatMessageUpdate(payload.GetMessageId(), "assistant", "", "", "", "stopped", false, payload.GetError())
	case *chatappv1.ChatRunFailed:
		return newChatMessageUpdate(payload.GetMessageId(), "assistant", "", "", "", "stopped", false, payload.GetError())
	default:
		return nil
	}
}

func parentMessageIDFromSegmentMessageID(messageID string) string {
	messageID = strings.TrimSpace(messageID)
	idx := strings.LastIndex(messageID, ":text:")
	if idx <= 0 {
		return ""
	}
	return messageID[:idx]
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
