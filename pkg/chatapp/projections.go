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
		EventChatTextSegmentStarted, EventChatTextPatch, EventChatTextSegmentFinished:
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
		if atts := payload.GetAttachments(); len(atts) > 0 {
			entity.Attachments = make([]*chatappv1.ChatAttachment, 0, len(atts))
			for _, a := range atts {
				entity.Attachments = append(entity.Attachments, proto.Clone(a).(*chatappv1.ChatAttachment))
			}
		}
		entity.Text = entity.Content
		entity.StartedAtMs, entity.FinishedAtMs = payload.GetAtMs(), payload.GetAtMs()
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	case *chatappv1.ChatRunStarted:
		return runEntity(view, payload.GetMessageId(), func(r *chatappv1.ChatRunEntity) {
			r.Status, r.Prompt = "running", payload.GetPrompt()
			r.Model, r.StartedAtMs = payload.GetModel(), payload.GetStartedAtMs()
		}), nil
	case *chatappv1.ChatRunFinished:
		return runEntity(view, payload.GetMessageId(), func(r *chatappv1.ChatRunEntity) {
			r.Status, r.Truncated, r.Reason = "finished", payload.GetTruncated(), payload.GetReason()
			r.FinishedAtMs = payload.GetFinishedAtMs()
		}), nil
	case *chatappv1.ChatRunStopped:
		return runEntity(view, payload.GetMessageId(), func(r *chatappv1.ChatRunEntity) {
			r.Status, r.FinishedAtMs = "stopped", payload.GetFinishedAtMs()
		}), nil
	case *chatappv1.ChatProviderCallFinished:
		// A run's usage is the sum of its provider calls; a call without a run
		// (published by an older sink) changes nothing.
		if strings.TrimSpace(payload.GetMessageId()) == "" {
			return nil, nil
		}
		return runEntity(view, payload.GetMessageId(), func(r *chatappv1.ChatRunEntity) {
			r.Usage = AddUsage(r.GetUsage(), payload.GetUsage())
			r.ProviderCalls++
		}), nil
	case *chatappv1.ChatRunFailed:
		messageID := strings.TrimSpace(payload.GetMessageId())
		if messageID == "" {
			return nil, nil
		}
		run := runEntity(view, messageID, func(r *chatappv1.ChatRunEntity) {
			r.Status, r.Code, r.Error = "failed", payload.GetCode(), payload.GetError()
			r.FinishedAtMs = payload.GetFinishedAtMs()
		})
		content := firstNonEmpty(payload.GetError(), "Chat run failed")
		entity := &chatappv1.ChatMessageEntity{
			MessageId:    messageID,
			Role:         "error",
			Content:      content,
			Text:         content,
			Status:       firstNonEmpty(payload.GetStatus(), "failed"),
			Streaming:    false,
			Final:        true,
			Correlation:  payload.GetCorrelation(),
			StartedAtMs:  payload.GetFinishedAtMs(),
			FinishedAtMs: payload.GetFinishedAtMs(),
		}
		return append(run, sessionstream.TimelineEntity{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}), nil
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
		StartAt(entity, payload.GetAtMs())
		entity.ParentMessageId = parentMessageIDFromSegmentMessageID(messageID)
		entity.Correlation = MergeCorrelationInfo(entity.GetCorrelation(), payload.GetCorrelation())
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	case *chatappv1.ChatTextPatch:
		messageID := strings.TrimSpace(payload.GetMessageId())
		if messageID == "" {
			return nil, nil
		}
		entity, _ := currentChatMessageEntity(view, messageID)
		content := ApplyStreamPatch(firstNonEmpty(entity.GetContent(), entity.GetText()), payload.GetText(), payload.GetMode())
		if content == "" {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Prompt = firstNonEmpty(payload.GetPrompt(), entity.GetPrompt())
		entity.Content = content
		entity.Text = content
		entity.Status = firstNonEmpty(payload.GetStatus(), "streaming")
		entity.Streaming = !payload.GetFinal()
		entity.Final = payload.GetFinal()
		StartAt(entity, payload.GetAtMs())
		entity.ParentMessageId = parentMessageIDFromSegmentMessageID(messageID)
		entity.Correlation = MergeCorrelationInfo(entity.GetCorrelation(), payload.GetCorrelation())
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
		StartAt(entity, payload.GetAtMs())
		entity.FinishedAtMs = payload.GetAtMs()
		entity.ParentMessageId = parentMessageIDFromSegmentMessageID(messageID)
		entity.Correlation = MergeCorrelationInfo(entity.GetCorrelation(), payload.GetCorrelation())
		return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
	default:
		return nil, nil
	}
}

// StartAt sets a message entity's start time once: from the first event of
// the segment that carries a time (the segment's start, or its first patch
// when the start made no entity).
func StartAt(e *chatappv1.ChatMessageEntity, atMs int64) {
	if e != nil && e.GetStartedAtMs() == 0 && atMs != 0 {
		e.StartedAtMs = atMs
	}
}

// AddUsage returns a + b (nil counts as zero).
func AddUsage(a, b *chatappv1.UsageInfo) *chatappv1.UsageInfo {
	return &chatappv1.UsageInfo{
		InputTokens:              a.GetInputTokens() + b.GetInputTokens(),
		OutputTokens:             a.GetOutputTokens() + b.GetOutputTokens(),
		CachedTokens:             a.GetCachedTokens() + b.GetCachedTokens(),
		CacheCreationInputTokens: a.GetCacheCreationInputTokens() + b.GetCacheCreationInputTokens(),
		CacheReadInputTokens:     a.GetCacheReadInputTokens() + b.GetCacheReadInputTokens(),
	}
}

// runEntity is the run's entity after fn, starting from its current state.
func runEntity(view sessionstream.TimelineView, messageID string, fn func(*chatappv1.ChatRunEntity)) []sessionstream.TimelineEntity {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil
	}
	run := &chatappv1.ChatRunEntity{MessageId: messageID}
	if e, ok := view.Get(TimelineEntityChatRun, messageID); ok && e.Payload != nil {
		if pb, ok := e.Payload.(*chatappv1.ChatRunEntity); ok && pb != nil {
			run = proto.Clone(pb).(*chatappv1.ChatRunEntity)
		}
	}
	fn(run)
	return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatRun, Id: messageID, Payload: run}}
}

func parentMessageIDFromSegmentMessageID(messageID string) string {
	messageID = strings.TrimSpace(messageID)
	idx := strings.LastIndex(messageID, textMessageIDDelimiter)
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

func ApplyStreamPatch(current, patch string, mode chatappv1.ChatStreamPatchMode) string {
	switch mode {
	case chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_SNAPSHOT,
		chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_REPLACE:
		return patch
	case chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND,
		chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_UNSPECIFIED:
		fallthrough
	default:
		return current + patch
	}
}
