package plugins

import (
	"context"
	"fmt"
	"strings"
	"sync"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

const (
	// ReasoningStartedEventName is the backend event published when a thinking stream begins.
	ReasoningStartedEventName = "ChatReasoningStarted"
	// ReasoningDeltaEventName is the backend event published for each thinking token delta.
	ReasoningDeltaEventName = "ChatReasoningDelta"
	// ReasoningFinishedEventName is the backend event published when thinking ends or a reasoning summary arrives.
	ReasoningFinishedEventName = "ChatReasoningFinished"

	// ReasoningStartedUIName is the UI event emitted when a thinking stream begins.
	ReasoningStartedUIName = "ChatReasoningStarted"
	// ReasoningAppendedUIName is the UI event emitted for each thinking token delta.
	ReasoningAppendedUIName = "ChatReasoningAppended"
	// ReasoningFinishedUIName is the UI event emitted when thinking ends.
	ReasoningFinishedUIName = "ChatReasoningFinished"
)

// ReasoningPlugin is a ChatPlugin that handles thinking/reasoning streams from
// geppetto inference engines. It translates EventThinkingPartial and EventInfo
// (thinking-started, thinking-ended, reasoning-summary) into sessionstream
// events, and projects them into ChatMessage timeline entities with role "thinking".
//
// Each contiguous thinking phase gets its own segment ID derived from the
// parent assistant message ID (for example, "chat-msg-5:thinking:1",
// "chat-msg-5:thinking:2").
type ReasoningPlugin struct {
	mu       sync.Mutex
	segments map[string]reasoningSegmentState
}

type reasoningSegmentState struct {
	Current int
	Active  bool
}

// NewReasoningPlugin creates a new ReasoningPlugin.
func NewReasoningPlugin() chatapp.ChatPlugin {
	return &ReasoningPlugin{segments: map[string]reasoningSegmentState{}}
}

// RegisterSchemas registers the reasoning event and UI event payload schemas.
func (p *ReasoningPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(ReasoningStartedEventName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterEvent(ReasoningDeltaEventName, &chatappv1.ReasoningUpdate{}),
		reg.RegisterEvent(ReasoningFinishedEventName, &chatappv1.ReasoningUpdate{}),
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

// HandleRuntimeEvent handles EventThinkingPartial and EventInfo events.
func (p *ReasoningPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
	parentMessageID := strings.TrimSpace(runtime.MessageID)
	if parentMessageID == "" {
		return false, nil
	}

	switch ev := event.(type) {
	case *gepevents.EventThinkingPartial:
		reasoningMessageID, segment := p.ensureReasoningSegment(parentMessageID)
		return true, runtime.Publish(ctx, ReasoningDeltaEventName, &chatappv1.ReasoningUpdate{
			MessageId:       reasoningMessageID,
			ParentMessageId: parentMessageID,
			Segment:         int32(segment),
			Role:            "thinking",
			Chunk:           ev.Delta,
			Content:         ev.Completion,
			Text:            ev.Completion,
			Status:          "streaming",
			Streaming:       true,
			Source:          "thinking",
			SegmentType:     "thinking",
		})
	case *gepevents.EventInfo:
		switch ev.Message {
		case "thinking-started":
			reasoningMessageID, segment := p.startReasoningSegment(parentMessageID)
			return true, runtime.Publish(ctx, ReasoningStartedEventName, &chatappv1.ReasoningUpdate{
				MessageId:       reasoningMessageID,
				ParentMessageId: parentMessageID,
				Segment:         int32(segment),
				Role:            "thinking",
				Status:          "streaming",
				Streaming:       true,
				Source:          "thinking",
				SegmentType:     "thinking",
			})
		case "thinking-ended":
			reasoningMessageID, segment, ok := p.currentReasoningSegment(parentMessageID)
			if !ok {
				return false, nil
			}
			p.finishReasoningSegment(parentMessageID)
			return true, runtime.Publish(ctx, ReasoningFinishedEventName, &chatappv1.ReasoningUpdate{
				MessageId:       reasoningMessageID,
				ParentMessageId: parentMessageID,
				Segment:         int32(segment),
				Role:            "thinking",
				Status:          "finished",
				Streaming:       false,
				Source:          "thinking",
				SegmentType:     "thinking",
			})
		case "reasoning-summary":
			reasoningMessageID, segment := p.ensureReasoningSegment(parentMessageID)
			p.finishReasoningSegment(parentMessageID)
			text := infoText(ev.Data)
			return true, runtime.Publish(ctx, ReasoningFinishedEventName, &chatappv1.ReasoningUpdate{
				MessageId:       reasoningMessageID,
				ParentMessageId: parentMessageID,
				Segment:         int32(segment),
				Role:            "thinking",
				Content:         text,
				Text:            text,
				Status:          "finished",
				Streaming:       false,
				Source:          "summary",
				SegmentType:     "thinking",
			})
		default:
			return false, nil
		}
	default:
		return false, nil
	}
}

// ProjectUI projects reasoning backend events into UI events.
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
// Thinking entities use role "thinking" and accumulate content across deltas.
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
func ReasoningEntityID(messageID string) string {
	return ReasoningSegmentEntityID(messageID, 1)
}

// ReasoningSegmentEntityID returns the thinking message ID for a specific parent
// assistant message and reasoning segment number.
func ReasoningSegmentEntityID(messageID string, segment int) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || segment <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:thinking:%d", messageID, segment)
}

func (p *ReasoningPlugin) startReasoningSegment(parentMessageID string) (string, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.segments == nil {
		p.segments = map[string]reasoningSegmentState{}
	}
	state := p.segments[parentMessageID]
	if !state.Active {
		state.Current++
		state.Active = true
	}
	p.segments[parentMessageID] = state
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current
}

func (p *ReasoningPlugin) ensureReasoningSegment(parentMessageID string) (string, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.segments == nil {
		p.segments = map[string]reasoningSegmentState{}
	}
	state := p.segments[parentMessageID]
	if !state.Active {
		state.Current++
		state.Active = true
	}
	p.segments[parentMessageID] = state
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current
}

func (p *ReasoningPlugin) currentReasoningSegment(parentMessageID string) (string, int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.segments[parentMessageID]
	if !state.Active || state.Current <= 0 {
		return "", 0, false
	}
	return ReasoningSegmentEntityID(parentMessageID, state.Current), state.Current, true
}

func (p *ReasoningPlugin) finishReasoningSegment(parentMessageID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.segments == nil {
		return
	}
	state := p.segments[parentMessageID]
	state.Active = false
	p.segments[parentMessageID] = state
}

func reasoningProjectedPayload(ev sessionstream.Event, view sessionstream.TimelineView) (*chatappv1.ReasoningUpdate, bool) {
	switch ev.Name {
	case ReasoningStartedEventName, ReasoningDeltaEventName, ReasoningFinishedEventName:
		payload, ok := ev.Payload.(*chatappv1.ReasoningUpdate)
		if !ok || payload == nil {
			return nil, false
		}
		payload = proto.Clone(payload).(*chatappv1.ReasoningUpdate)
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

func infoText(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}
	if s, ok := data["text"].(string); ok {
		return s
	}
	return ""
}

// Ensure ReasoningPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ReasoningPlugin)(nil)

// compile-time check for proto usage
var _ proto.Message = (*chatappv1.ChatMessageEntity)(nil)
