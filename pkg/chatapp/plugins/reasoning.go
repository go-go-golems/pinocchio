package plugins

import (
	"context"
	"fmt"
	"strings"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

const (
	// ReasoningStartedEventName is the canonical backend event published when a reasoning segment begins.
	ReasoningStartedEventName = chatapp.EventChatReasoningSegmentStarted
	// ReasoningDeltaEventName is the canonical backend event published for each reasoning token delta.
	ReasoningDeltaEventName = chatapp.EventChatReasoningDelta
	// ReasoningFinishedEventName is the canonical backend event published when a reasoning segment completes.
	ReasoningFinishedEventName = chatapp.EventChatReasoningSegmentFinished

	// TimelineEntityReasoning aliases the base chat message entity kind; reasoning blocks are messages with role=thinking.
	TimelineEntityReasoning = chatapp.TimelineEntityChatMessage
)

// ReasoningPlugin translates canonical Geppetto reasoning segment events into
// canonical Pinocchio reasoning events and timeline entities.
type ReasoningPlugin struct{}

// NewReasoningPlugin creates a new ReasoningPlugin.
func NewReasoningPlugin() chatapp.ChatPlugin { return &ReasoningPlugin{} }

// RegisterSchemas registers canonical reasoning event, UI event, and timeline payload schemas.
func (p *ReasoningPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(ReasoningStartedEventName, &chatappv1.ChatReasoningSegmentStarted{}),
		reg.RegisterEvent(ReasoningDeltaEventName, &chatappv1.ChatReasoningDelta{}),
		reg.RegisterEvent(ReasoningFinishedEventName, &chatappv1.ChatReasoningSegmentFinished{}),
		reg.RegisterUIEvent(ReasoningStartedEventName, &chatappv1.ChatReasoningSegmentStarted{}),
		reg.RegisterUIEvent(ReasoningDeltaEventName, &chatappv1.ChatReasoningDelta{}),
		reg.RegisterUIEvent(ReasoningFinishedEventName, &chatappv1.ChatReasoningSegmentFinished{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// HandleRuntimeEvent handles canonical Geppetto reasoning events.
func (p *ReasoningPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	parentMessageID := strings.TrimSpace(runtime.MessageID)
	if parentMessageID == "" {
		return false, nil
	}

	switch ev := event.(type) {
	case *gepevents.EventReasoningSegmentStarted:
		return true, runtime.Publish(ctx, ReasoningStartedEventName, &chatappv1.ChatReasoningSegmentStarted{
			MessageId:       reasoningMessageID(parentMessageID, ev.Correlation()),
			ParentMessageId: parentMessageID,
			Role:            "thinking",
			Status:          "streaming",
			Streaming:       true,
			Source:          firstNonEmptyString(ev.Source, "thinking"),
			Correlation:     chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventReasoningDelta:
		return true, runtime.Publish(ctx, ReasoningDeltaEventName, &chatappv1.ChatReasoningDelta{
			MessageId:       reasoningMessageID(parentMessageID, ev.Correlation()),
			ParentMessageId: parentMessageID,
			Role:            "thinking",
			Chunk:           ev.Delta,
			Text:            ev.Text,
			Content:         ev.Text,
			Status:          "streaming",
			Streaming:       true,
			Source:          reasoningSource(ev.Correlation()),
			Correlation:     chatapp.CorrelationInfoFromEvent(ev),
		})
	case *gepevents.EventReasoningSegmentFinished:
		return true, runtime.Publish(ctx, ReasoningFinishedEventName, &chatappv1.ChatReasoningSegmentFinished{
			MessageId:       reasoningMessageID(parentMessageID, ev.Correlation()),
			ParentMessageId: parentMessageID,
			Role:            "thinking",
			Text:            ev.Text,
			Content:         ev.Text,
			Status:          "finished",
			Streaming:       false,
			Source:          reasoningSource(ev.Correlation()),
			FinishReason:    ev.FinishReason,
			Correlation:     chatapp.CorrelationInfoFromEvent(ev),
		})
	default:
		return false, nil
	}
}

// ProjectUI forwards canonical reasoning backend events as canonical UI events.
func (p *ReasoningPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	switch ev.Name {
	case ReasoningStartedEventName, ReasoningDeltaEventName, ReasoningFinishedEventName:
		if ev.Payload == nil {
			return nil, true, fmt.Errorf("reasoning payload must be proto message, got %T", ev.Payload)
		}
		return []sessionstream.UIEvent{{Name: ev.Name, Payload: proto.Clone(ev.Payload)}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects reasoning backend events into ChatMessage timeline entities.
func (p *ReasoningPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	messageID, entity, ok, err := reasoningEntityFromEvent(ev, view)
	if err != nil || !ok || entity == nil || strings.TrimSpace(messageID) == "" {
		return nil, ok, err
	}
	return []sessionstream.TimelineEntity{{Kind: chatapp.TimelineEntityChatMessage, Id: messageID, Payload: entity}}, true, nil
}

// ReasoningEntityID returns the first thinking segment ID for a given parent message ID.
func ReasoningEntityID(messageID string) string { return ReasoningSegmentEntityID(messageID, 1) }

// ReasoningSegmentEntityID returns the thinking message ID for a specific parent
// assistant message and reasoning segment number.
func ReasoningSegmentEntityID(messageID string, segment int32) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || segment <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:thinking:%d", messageID, segment)
}

func reasoningMessageID(parentMessageID string, corr gepevents.Correlation) string {
	if corr.SegmentIndex > 0 {
		return ReasoningSegmentEntityID(parentMessageID, corr.SegmentIndex)
	}
	return ReasoningSegmentEntityID(parentMessageID, 1)
}

func reasoningSource(corr gepevents.Correlation) string {
	if corr.SummaryIndex != nil {
		return "summary"
	}
	return "thinking"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func reasoningEntityFromEvent(ev sessionstream.Event, view sessionstream.TimelineView) (string, *chatappv1.ChatMessageEntity, bool, error) {
	switch payload := ev.Payload.(type) {
	case *chatappv1.ChatReasoningSegmentStarted:
		return reasoningEntityFromFields(view, payload.GetMessageId(), payload.GetParentMessageId(), payload.GetRole(), "", "", payload.GetStatus(), payload.GetStreaming(), payload.GetCorrelation(), ev.Name)
	case *chatappv1.ChatReasoningDelta:
		return reasoningEntityFromFields(view, payload.GetMessageId(), payload.GetParentMessageId(), payload.GetRole(), payload.GetContent(), payload.GetText(), payload.GetStatus(), payload.GetStreaming(), payload.GetCorrelation(), ev.Name)
	case *chatappv1.ChatReasoningSegmentFinished:
		return reasoningEntityFromFields(view, payload.GetMessageId(), payload.GetParentMessageId(), payload.GetRole(), payload.GetContent(), payload.GetText(), payload.GetStatus(), payload.GetStreaming(), payload.GetCorrelation(), ev.Name)
	default:
		return "", nil, false, nil
	}
}

func reasoningEntityFromFields(view sessionstream.TimelineView, messageID, parentMessageID, role, content, text, status string, streaming bool, corr *chatappv1.CorrelationInfo, eventName string) (string, *chatappv1.ChatMessageEntity, bool, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "", nil, true, nil
	}
	entity, hadEntity := currentReasoningEntity(view, messageID)
	content = firstNonEmptyString(content, text)
	if content == "" {
		content = firstNonEmptyString(entity.GetContent(), entity.GetText())
	}
	if content == "" && !hadEntity {
		return "", nil, true, nil
	}
	entity.MessageId = messageID
	entity.Role = firstNonEmptyString(role, "thinking")
	entity.Content = content
	entity.Text = content
	entity.ParentMessageId = firstNonEmptyString(parentMessageID, entity.GetParentMessageId())
	entity.Correlation = chatapp.MergeCorrelationInfo(entity.GetCorrelation(), corr)
	entity.Segment = entity.GetCorrelation().GetSegmentIndex()
	entity.SegmentType = firstNonEmptyString(entity.GetCorrelation().GetSegmentType(), gepevents.SegmentTypeReasoning)
	switch eventName {
	case ReasoningStartedEventName, ReasoningDeltaEventName:
		entity.Status = firstNonEmptyString(status, "streaming")
		entity.Streaming = streaming
	case ReasoningFinishedEventName:
		entity.Status = firstNonEmptyString(status, "finished")
		entity.Streaming = false
	default:
		return "", nil, false, nil
	}
	return messageID, entity, true, nil
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
	return proto.Clone(pb).(*chatappv1.ChatMessageEntity), true
}

// Ensure ReasoningPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ReasoningPlugin)(nil)
