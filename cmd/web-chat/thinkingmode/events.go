package thinkingmode

import (
	"strings"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
)

// ThinkingModePayload represents the structured data payload for thinking mode events.
// This is a lightweight Go-native structure used by cmd/web-chat's app-owned module.
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

func registerThinkingModeEventFactories() {
	registerThinkingModeFactory("thinking.mode.started", func() gepevents.Event {
		return &EventThinkingModeStarted{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.started")}}
	})
	registerThinkingModeFactory("thinking.mode.update", func() gepevents.Event {
		return &EventThinkingModeUpdate{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.update")}}
	})
	registerThinkingModeFactory("thinking.mode.completed", func() gepevents.Event {
		return &EventThinkingModeCompleted{EventImpl: gepevents.EventImpl{Type_: gepevents.EventType("thinking.mode.completed")}}
	})
}

func registerThinkingModeFactory(typeName string, factory func() gepevents.Event) {
	if err := gepevents.RegisterEventFactory(typeName, factory); err != nil {
		// Registry is process-global and tests may call Register() repeatedly.
		if !strings.Contains(err.Error(), "already registered") {
			panic(err)
		}
	}
}
