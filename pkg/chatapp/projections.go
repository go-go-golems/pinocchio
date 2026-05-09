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
		entity.Correlation = mergeCorrelationInfo(entity.GetCorrelation(), payload.GetCorrelation())
		entity.Segment = entity.GetCorrelation().GetSegmentIndex()
		entity.SegmentType = entity.GetCorrelation().GetSegmentType()
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
		entity.Correlation = mergeCorrelationInfo(entity.GetCorrelation(), payload.GetCorrelation())
		entity.Segment = entity.GetCorrelation().GetSegmentIndex()
		entity.SegmentType = entity.GetCorrelation().GetSegmentType()
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
		entity.Correlation = mergeCorrelationInfo(entity.GetCorrelation(), payload.GetCorrelation())
		entity.Segment = entity.GetCorrelation().GetSegmentIndex()
		entity.SegmentType = entity.GetCorrelation().GetSegmentType()
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

func mergeCorrelationInfo(existing, update *chatappv1.CorrelationInfo) *chatappv1.CorrelationInfo {
	if existing == nil {
		return cloneCorrelationInfo(update)
	}
	if update == nil {
		return cloneCorrelationInfo(existing)
	}
	out := cloneCorrelationInfo(existing)
	if update.SessionId != "" {
		out.SessionId = update.SessionId
	}
	if update.RunId != "" {
		out.RunId = update.RunId
	}
	if update.InferenceId != "" {
		out.InferenceId = update.InferenceId
	}
	if update.TurnId != "" {
		out.TurnId = update.TurnId
	}
	if update.ProviderCallId != "" {
		out.ProviderCallId = update.ProviderCallId
	}
	if update.ProviderCallIndex != 0 {
		out.ProviderCallIndex = update.ProviderCallIndex
	}
	if update.Provider != "" {
		out.Provider = update.Provider
	}
	if update.Model != "" {
		out.Model = update.Model
	}
	if update.ResponseId != "" {
		out.ResponseId = update.ResponseId
	}
	if update.ItemId != "" {
		out.ItemId = update.ItemId
	}
	if update.OutputIndex != nil {
		out.OutputIndex = cloneInt32Ptr(update.OutputIndex)
	}
	if update.SummaryIndex != nil {
		out.SummaryIndex = cloneInt32Ptr(update.SummaryIndex)
	}
	if update.ChoiceIndex != nil {
		out.ChoiceIndex = cloneInt32Ptr(update.ChoiceIndex)
	}
	if update.ContentBlockIndex != nil {
		out.ContentBlockIndex = cloneInt32Ptr(update.ContentBlockIndex)
	}
	if update.SegmentId != "" {
		out.SegmentId = update.SegmentId
	}
	if update.SegmentIndex != 0 {
		out.SegmentIndex = update.SegmentIndex
	}
	if update.SegmentType != "" {
		out.SegmentType = update.SegmentType
	}
	if update.StreamKind != "" {
		out.StreamKind = update.StreamKind
	}
	if update.ToolCallId != "" {
		out.ToolCallId = update.ToolCallId
	}
	if update.ToolCallIndex != nil {
		out.ToolCallIndex = cloneInt32Ptr(update.ToolCallIndex)
	}
	if update.CorrelationKey != "" {
		out.CorrelationKey = update.CorrelationKey
	}
	if update.ParentCorrelationKey != "" {
		out.ParentCorrelationKey = update.ParentCorrelationKey
	}
	return out
}
