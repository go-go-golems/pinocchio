package chatapp

import (
	"context"
	"strings"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

func baseUIProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, error) {
	if ev.Payload == nil {
		return nil, nil
	}
	switch ev.Name {
	case EventUserMessageAccepted,
		EventChatRunStarted, EventChatRunFinished, EventChatRunStopped, EventChatRunFailed,
		EventChatProviderCallStarted, EventChatProviderCallMetadataUpdated, EventChatProviderCallFinished,
		EventChatTextSegmentStarted, EventChatTextDelta, EventChatTextSegmentFinished:
		return []sessionstream.UIEvent{{Name: ev.Name, Payload: proto.Clone(ev.Payload)}}, nil
	default:
		return nil, nil
	}
}

func baseTimelineProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, error) {
	switch payload := ev.Payload.(type) {
	case *chatappv1.ChatUserMessageAccepted:
		messageID := strings.TrimSpace(payload.GetMessageId())
		if messageID == "" {
			return nil, nil
		}
		entity := &chatappv1.ChatMessageEntity{
			MessageId: messageID,
			Role:      firstNonEmpty(payload.GetRole(), "user"),
			Prompt:    payload.GetPrompt(),
			Content:   firstNonEmpty(payload.GetContent(), payload.GetText()),
			Status:    firstNonEmpty(payload.GetStatus(), "accepted"),
			Streaming: false,
		}
		entity.Text = entity.Content
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	case *chatappv1.ChatTextSegmentStarted:
		messageID := strings.TrimSpace(payload.GetMessageId())
		if messageID == "" {
			return nil, nil
		}
		entity, hadEntity := currentChatMessageEntity(view, messageID)
		content := firstNonEmpty(entity.GetContent(), entity.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Prompt = firstNonEmpty(payload.GetPrompt(), entity.GetPrompt())
		entity.Content = content
		entity.Text = content
		entity.Status = firstNonEmpty(payload.GetStatus(), "streaming")
		entity.Streaming = payload.GetStreaming()
		entity.ParentMessageId = parentMessageIDFromSegmentMessageID(messageID)
		entity.Segment = payload.GetCorrelation().GetSegmentIndex()
		entity.SegmentType = payload.GetCorrelation().GetSegmentType()
		entity.Correlation = cloneCorrelationInfo(payload.GetCorrelation())
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	case *chatappv1.ChatTextDelta:
		messageID := strings.TrimSpace(payload.GetMessageId())
		if messageID == "" {
			return nil, nil
		}
		content := firstNonEmpty(payload.GetContent(), payload.GetText())
		if content == "" {
			return nil, nil
		}
		entity, _ := currentChatMessageEntity(view, messageID)
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Prompt = firstNonEmpty(payload.GetPrompt(), entity.GetPrompt())
		entity.Content = content
		entity.Text = content
		entity.Status = firstNonEmpty(payload.GetStatus(), "streaming")
		entity.Streaming = payload.GetStreaming()
		entity.ParentMessageId = parentMessageIDFromSegmentMessageID(messageID)
		entity.Segment = payload.GetCorrelation().GetSegmentIndex()
		entity.SegmentType = payload.GetCorrelation().GetSegmentType()
		entity.Correlation = cloneCorrelationInfo(payload.GetCorrelation())
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	case *chatappv1.ChatTextSegmentFinished:
		messageID := strings.TrimSpace(payload.GetMessageId())
		if messageID == "" {
			return nil, nil
		}
		entity, hadEntity := currentChatMessageEntity(view, messageID)
		content := firstNonEmpty(payload.GetContent(), payload.GetText(), entity.GetContent(), entity.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Prompt = firstNonEmpty(payload.GetPrompt(), entity.GetPrompt())
		entity.Content = content
		entity.Text = content
		entity.Status = firstNonEmpty(payload.GetStatus(), "finished")
		entity.Streaming = payload.GetStreaming()
		entity.Final = payload.GetFinal()
		entity.ParentMessageId = parentMessageIDFromSegmentMessageID(messageID)
		entity.Segment = payload.GetCorrelation().GetSegmentIndex()
		entity.SegmentType = payload.GetCorrelation().GetSegmentType()
		entity.Correlation = cloneCorrelationInfo(payload.GetCorrelation())
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	default:
		return nil, nil
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

func cloneCorrelationInfo(corr *chatappv1.CorrelationInfo) *chatappv1.CorrelationInfo {
	if corr == nil {
		return nil
	}
	return proto.Clone(corr).(*chatappv1.CorrelationInfo)
}
