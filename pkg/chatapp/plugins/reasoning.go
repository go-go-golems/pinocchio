package plugins

import (
	"context"
	"fmt"
	"strings"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

const (
	// ReasoningStartedEventName is the canonical backend event published when a reasoning segment begins.
	ReasoningStartedEventName = chatapp.EventChatReasoningSegmentStarted
	// ReasoningDeltaEventName is the canonical backend event published for each reasoning token delta.
	ReasoningDeltaEventName = chatapp.EventChatReasoningDelta
	// ReasoningFinishedEventName is the canonical backend event published when a reasoning segment completes.
	ReasoningFinishedEventName = chatapp.EventChatReasoningSegmentFinished

	// ReasoningStartedUIName is the UI event emitted when a thinking stream begins.
	ReasoningStartedUIName = "ChatReasoningStarted"
	// ReasoningAppendedUIName is the UI event emitted for each thinking token delta.
	ReasoningAppendedUIName = "ChatReasoningAppended"
	// ReasoningFinishedUIName is the UI event emitted when thinking ends.
	ReasoningFinishedUIName = "ChatReasoningFinished"
)

// ReasoningPlugin translates canonical Geppetto reasoning segment events into
// canonical Pinocchio backend events, then projects them into the existing
// reasoning UI event vocabulary for frontend compatibility.
type ReasoningPlugin struct{}

// NewReasoningPlugin creates a new ReasoningPlugin.
func NewReasoningPlugin() chatapp.ChatPlugin { return &ReasoningPlugin{} }

// RegisterSchemas registers the reasoning event and UI event payload schemas.
func (p *ReasoningPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(ReasoningStartedEventName, &chatappv1.ChatReasoningSegmentStarted{}),
		reg.RegisterEvent(ReasoningDeltaEventName, &chatappv1.ChatReasoningDelta{}),
		reg.RegisterEvent(ReasoningFinishedEventName, &chatappv1.ChatReasoningSegmentFinished{}),
		reg.RegisterUIEvent(ReasoningStartedUIName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterUIEvent(ReasoningAppendedUIName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterUIEvent(ReasoningFinishedUIName, &chatappv1.ReasoningUpdate{}),
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

// ProjectUI projects canonical reasoning backend events into compatibility UI events.
func (p *ReasoningPlugin) ProjectUI(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	switch ev.Name {
	case ReasoningStartedEventName:
		return []sessionstream.UIEvent{{Name: ReasoningStartedUIName, Payload: payload}}, true, nil
	case ReasoningDeltaEventName:
		return []sessionstream.UIEvent{{Name: ReasoningAppendedUIName, Payload: payload}}, true, nil
	case ReasoningFinishedEventName:
		return []sessionstream.UIEvent{{Name: ReasoningFinishedUIName, Payload: payload}}, true, nil
	default:
		return nil, false, nil
	}
}

// ProjectTimeline projects reasoning backend events into ChatMessage timeline entities.
func (p *ReasoningPlugin) ProjectTimeline(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error) {
	payload, ok := reasoningProjectedPayload(ev, view)
	if !ok {
		return nil, false, nil
	}
	messageID := payload.GetMessageId()
	if messageID == "" {
		return nil, true, nil
	}
	entity, hadEntity := currentReasoningEntity(view, messageID)
	content := payload.GetContent()
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
	entity.ParentMessageId = payload.GetParentMessageId()
	entity.Segment = payload.GetSegment()
	entity.SegmentType = "thinking"

	switch ev.Name {
	case ReasoningStartedEventName, ReasoningDeltaEventName:
		entity.Status = "streaming"
		entity.Streaming = true
	case ReasoningFinishedEventName:
		entity.Status = "finished"
		entity.Streaming = false
	default:
		return nil, false, nil
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

func reasoningUpdateFromCorrelation(base *chatappv1.ReasoningUpdate, corr *chatappv1.CorrelationInfo) *chatappv1.ReasoningUpdate {
	if base == nil {
		return nil
	}
	if corr == nil {
		return base
	}
	base.Segment = corr.GetSegmentIndex()
	base.SegmentType = firstNonEmptyString(corr.GetSegmentType(), "thinking")
	base.Provider = corr.GetProvider()
	base.ResponseId = corr.GetResponseId()
	base.ItemId = corr.GetItemId()
	base.OutputIndex = cloneInt32Ptr(corr.OutputIndex)
	base.SummaryIndex = cloneInt32Ptr(corr.SummaryIndex)
	base.ChoiceIndex = cloneInt32Ptr(corr.ChoiceIndex)
	base.StreamKind = corr.GetStreamKind()
	base.CorrelationKey = corr.GetCorrelationKey()
	return base
}

func reasoningProjectedPayload(ev sessionstream.Event, view sessionstream.TimelineView) (*chatappv1.ReasoningUpdate, bool) {
	var payload *chatappv1.ReasoningUpdate
	switch ev.Name {
	case ReasoningStartedEventName:
		p, ok := ev.Payload.(*chatappv1.ChatReasoningSegmentStarted)
		if !ok || p == nil {
			return nil, false
		}
		payload = reasoningUpdateFromCorrelation(&chatappv1.ReasoningUpdate{
			MessageId:       p.GetMessageId(),
			ParentMessageId: p.GetParentMessageId(),
			Role:            firstNonEmptyString(p.GetRole(), "thinking"),
			Status:          firstNonEmptyString(p.GetStatus(), "streaming"),
			Streaming:       p.GetStreaming(),
			Source:          firstNonEmptyString(p.GetSource(), "thinking"),
		}, p.GetCorrelation())
	case ReasoningDeltaEventName:
		p, ok := ev.Payload.(*chatappv1.ChatReasoningDelta)
		if !ok || p == nil {
			return nil, false
		}
		payload = reasoningUpdateFromCorrelation(&chatappv1.ReasoningUpdate{
			MessageId:       p.GetMessageId(),
			ParentMessageId: p.GetParentMessageId(),
			Role:            firstNonEmptyString(p.GetRole(), "thinking"),
			Chunk:           p.GetChunk(),
			Content:         p.GetContent(),
			Text:            p.GetText(),
			Status:          firstNonEmptyString(p.GetStatus(), "streaming"),
			Streaming:       p.GetStreaming(),
			Source:          firstNonEmptyString(p.GetSource(), "thinking"),
		}, p.GetCorrelation())
	case ReasoningFinishedEventName:
		p, ok := ev.Payload.(*chatappv1.ChatReasoningSegmentFinished)
		if !ok || p == nil {
			return nil, false
		}
		payload = reasoningUpdateFromCorrelation(&chatappv1.ReasoningUpdate{
			MessageId:       p.GetMessageId(),
			ParentMessageId: p.GetParentMessageId(),
			Role:            firstNonEmptyString(p.GetRole(), "thinking"),
			Content:         p.GetContent(),
			Text:            p.GetText(),
			Status:          firstNonEmptyString(p.GetStatus(), "finished"),
			Streaming:       p.GetStreaming(),
			Source:          firstNonEmptyString(p.GetSource(), "thinking"),
		}, p.GetCorrelation())
	default:
		return nil, false
	}
	if payload.Role == "" {
		payload.Role = "thinking"
	}
	if payload.SegmentType == "" {
		payload.SegmentType = "thinking"
	}
	if view != nil && payload.GetMessageId() != "" && payload.GetContent() == "" {
		current, _ := currentReasoningEntity(view, payload.GetMessageId())
		if currentContent := current.GetContent(); currentContent != "" {
			payload.Content = currentContent
			payload.Text = currentContent
		} else if currentText := current.GetText(); currentText != "" {
			payload.Content = currentText
			payload.Text = currentText
		}
	}
	return payload, true
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
		MessageId:       pb.GetMessageId(),
		Role:            pb.GetRole(),
		Prompt:          pb.GetPrompt(),
		Text:            pb.GetText(),
		Content:         pb.GetContent(),
		Status:          pb.GetStatus(),
		Streaming:       pb.GetStreaming(),
		Error:           pb.GetError(),
		ParentMessageId: pb.GetParentMessageId(),
		Segment:         pb.GetSegment(),
		SegmentType:     pb.GetSegmentType(),
		Final:           pb.GetFinal(),
	}, true
}

func cloneInt32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	vv := *v
	return &vv
}

// Ensure ReasoningPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ReasoningPlugin)(nil)
