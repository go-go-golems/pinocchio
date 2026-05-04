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
	"google.golang.org/protobuf/types/known/structpb"
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
// The thinking message ID is derived from the parent message ID by appending
// ":thinking" (e.g., "chat-msg-5:thinking").
type ReasoningPlugin struct{}

// NewReasoningPlugin creates a new ReasoningPlugin.
func NewReasoningPlugin() chatapp.ChatPlugin {
	return &ReasoningPlugin{}
}

// RegisterSchemas registers the reasoning event names, UI events with structpb.Struct payloads.
func (p *ReasoningPlugin) RegisterSchemas(reg *sessionstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterEvent(ReasoningStartedEventName, &structpb.Struct{}),
		reg.RegisterEvent(ReasoningDeltaEventName, &structpb.Struct{}),
		reg.RegisterEvent(ReasoningFinishedEventName, &structpb.Struct{}),
		reg.RegisterUIEvent(ReasoningStartedUIName, &structpb.Struct{}),
		reg.RegisterUIEvent(ReasoningAppendedUIName, &structpb.Struct{}),
		reg.RegisterUIEvent(ReasoningFinishedUIName, &structpb.Struct{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

// HandleRuntimeEvent handles EventThinkingPartial and EventInfo events.
func (p *ReasoningPlugin) HandleRuntimeEvent(ctx context.Context, runtime chatapp.RuntimeEventContext, event gepevents.Event) (bool, error) {
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
		return true, runtime.Publish(ctx, ReasoningDeltaEventName, pb)
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
			return true, runtime.Publish(ctx, ReasoningStartedEventName, pb)
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
			return true, runtime.Publish(ctx, ReasoningFinishedEventName, pb)
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
			return true, runtime.Publish(ctx, ReasoningFinishedEventName, pb)
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
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, true, err
	}
	switch ev.Name {
	case ReasoningStartedEventName:
		return []sessionstream.UIEvent{{Name: ReasoningStartedUIName, Payload: pb}}, true, nil
	case ReasoningDeltaEventName:
		return []sessionstream.UIEvent{{Name: ReasoningAppendedUIName, Payload: pb}}, true, nil
	case ReasoningFinishedEventName:
		return []sessionstream.UIEvent{{Name: ReasoningFinishedUIName, Payload: pb}}, true, nil
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

// ReasoningEntityID returns the thinking message ID for a given parent message ID.
func ReasoningEntityID(messageID string) string {
	return reasoningEntityID(messageID)
}

func reasoningEntityID(messageID string) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ""
	}
	return messageID + ":thinking"
}

func reasoningProjectedPayload(ev sessionstream.Event, view sessionstream.TimelineView) (map[string]any, bool) {
	switch ev.Name {
	case ReasoningStartedEventName, ReasoningDeltaEventName, ReasoningFinishedEventName:
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

func payloadWithOrdinal(ev sessionstream.Event) map[string]any {
	payload := toMap(ev.Payload)
	payload["ordinal"] = fmt.Sprintf("%d", ev.Ordinal)
	return payload
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

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toMap(msg any) map[string]any {
	if pb, ok := msg.(*structpb.Struct); ok && pb != nil {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Ensure ReasoningPlugin implements ChatPlugin.
var _ chatapp.ChatPlugin = (*ReasoningPlugin)(nil)

// compile-time check for proto usage
var _ proto.Message = (*chatappv1.ChatMessageEntity)(nil)
