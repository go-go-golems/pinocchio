package events

import (
	gepevents "github.com/go-go-golems/geppetto/pkg/events"
)

// ThinkingModePayload represents the structured data payload for thinking mode events.
// This is a lightweight Go-native structure; the SEM translator will map it into
// protobuf-authored `sem.middleware.thinking_mode` payloads at the boundary.
type ThinkingModePayload struct {
	Mode      string         `json:"mode" yaml:"mode"`
	Phase     string         `json:"phase" yaml:"phase"`
	Reasoning string         `json:"reasoning" yaml:"reasoning"`
	ExtraData map[string]any `json:"extra_data,omitempty" yaml:"extra_data,omitempty"`
}

type EventThinkingModeStarted struct {
	gepevents.EventImpl
	ItemID string               `json:"item_id"`
	Data   *ThinkingModePayload `json:"data,omitempty"`
}

func NewThinkingModeStarted(metadata gepevents.EventMetadata, itemID string, data *ThinkingModePayload) *EventThinkingModeStarted {
	return &EventThinkingModeStarted{
		EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.started"), Metadata_: metadata},
		ItemID:    itemID,
		Data:      data,
	}
}

var _ gepevents.Event = &EventThinkingModeStarted{}

type EventThinkingModeUpdate struct {
	gepevents.EventImpl
	ItemID string               `json:"item_id"`
	Data   *ThinkingModePayload `json:"data,omitempty"`
}

func NewThinkingModeUpdate(metadata gepevents.EventMetadata, itemID string, data *ThinkingModePayload) *EventThinkingModeUpdate {
	return &EventThinkingModeUpdate{
		EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.update"), Metadata_: metadata},
		ItemID:    itemID,
		Data:      data,
	}
}

var _ gepevents.Event = &EventThinkingModeUpdate{}

type EventThinkingModeCompleted struct {
	gepevents.EventImpl
	ItemID  string               `json:"item_id"`
	Data    *ThinkingModePayload `json:"data,omitempty"`
	Success bool                 `json:"success"`
	Error   string               `json:"error,omitempty"`
}

func NewThinkingModeCompleted(metadata gepevents.EventMetadata, itemID string, data *ThinkingModePayload, success bool, errStr string) *EventThinkingModeCompleted {
	return &EventThinkingModeCompleted{
		EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.completed"), Metadata_: metadata},
		ItemID:    itemID,
		Data:      data,
		Success:   success,
		Error:     errStr,
	}
}

var _ gepevents.Event = &EventThinkingModeCompleted{}

func init() {
	_ = gepevents.RegisterEventFactory("thinking.mode.started", func() gepevents.Event {
		return &EventThinkingModeStarted{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.started")}}
	})
	_ = gepevents.RegisterEventFactory("thinking.mode.update", func() gepevents.Event {
		return &EventThinkingModeUpdate{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.update")}}
	})
	_ = gepevents.RegisterEventFactory("thinking.mode.completed", func() gepevents.Event {
		return &EventThinkingModeCompleted{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.completed")}}
	})
}
