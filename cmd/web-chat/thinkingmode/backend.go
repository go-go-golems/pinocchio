package thinkingmode

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	semregistry "github.com/go-go-golems/pinocchio/pkg/sem/registry"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"google.golang.org/protobuf/types/known/structpb"
)

var registerOnce sync.Once

// Register wires thinking-mode integration for cmd/web-chat.
// It is safe to call multiple times.
func Register() {
	registerOnce.Do(func() {
		registerThinkingModeEventFactories()
		registerSemTranslatorHandlers()
		registerTimelineProjectionHandlers()
	})
}

func resetForTests() {
	registerOnce = sync.Once{}
}

func registerTimelineProjectionHandlers() {
	webchat.RegisterTimelineHandler("thinking.mode.started", thinkingModeTimelineHandler)
	webchat.RegisterTimelineHandler("thinking.mode.update", thinkingModeTimelineHandler)
	webchat.RegisterTimelineHandler("thinking.mode.completed", thinkingModeTimelineHandler)
}

func thinkingModeTimelineHandler(ctx context.Context, p *webchat.TimelineProjector, ev webchat.TimelineSemEvent, _ int64) error {
	var (
		itemID  string
		mode    string
		phase   string
		reason  string
		success bool
		errStr  string
	)

	switch ev.Type {
	case "thinking.mode.started":
		var payload semThinkingModeStarted
		if err := json.Unmarshal(ev.Data, &payload); err != nil {
			return nil
		}
		itemID = payload.ItemID
		if payload.Data != nil {
			mode, phase, reason = payload.Data.Mode, payload.Data.Phase, payload.Data.Reasoning
		}
		success = true

	case "thinking.mode.update":
		var payload semThinkingModeUpdate
		if err := json.Unmarshal(ev.Data, &payload); err != nil {
			return nil
		}
		itemID = payload.ItemID
		if payload.Data != nil {
			mode, phase, reason = payload.Data.Mode, payload.Data.Phase, payload.Data.Reasoning
		}
		success = true

	case "thinking.mode.completed":
		var payload semThinkingModeCompleted
		if err := json.Unmarshal(ev.Data, &payload); err != nil {
			return nil
		}
		itemID = payload.ItemID
		if payload.Data != nil {
			mode, phase, reason = payload.Data.Mode, payload.Data.Phase, payload.Data.Reasoning
		}
		success = payload.Success
		errStr = payload.Error

	default:
		return nil
	}

	if strings.TrimSpace(itemID) == "" {
		itemID = ev.ID
	}

	status := "active"
	if ev.Type == "thinking.mode.completed" {
		if success {
			status = "completed"
		} else {
			status = "error"
		}
	}
	if errStr != "" {
		status = "error"
	}

	return p.Upsert(ctx, ev.Seq, timelineEntityFromMap(itemID, "thinking_mode", map[string]any{
		"schemaVersion": 1,
		"status":        status,
		"mode":          mode,
		"phase":         phase,
		"reasoning":     reason,
		"success":       success,
		"error":         errStr,
	}))
}

func registerSemTranslatorHandlers() {
	semregistry.RegisterByType[*EventThinkingModeStarted](func(ev *EventThinkingModeStarted) ([][]byte, error) {
		payload := &semThinkingModeStarted{
			ItemID: ev.ItemID,
			Data:   semPayloadFromEventData(ev.Data),
		}
		return [][]byte{wrapSem(map[string]any{"type": "thinking.mode.started", "id": ev.ItemID, "data": payload})}, nil
	})

	semregistry.RegisterByType[*EventThinkingModeUpdate](func(ev *EventThinkingModeUpdate) ([][]byte, error) {
		payload := &semThinkingModeUpdate{
			ItemID: ev.ItemID,
			Data:   semPayloadFromEventData(ev.Data),
		}
		return [][]byte{wrapSem(map[string]any{"type": "thinking.mode.update", "id": ev.ItemID, "data": payload})}, nil
	})

	semregistry.RegisterByType[*EventThinkingModeCompleted](func(ev *EventThinkingModeCompleted) ([][]byte, error) {
		payload := &semThinkingModeCompleted{
			ItemID:  ev.ItemID,
			Data:    semPayloadFromEventData(ev.Data),
			Success: ev.Success,
			Error:   ev.Error,
		}
		return [][]byte{wrapSem(map[string]any{"type": "thinking.mode.completed", "id": ev.ItemID, "data": payload})}, nil
	})
}

func wrapSem(ev map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"sem": true, "event": ev})
	return b
}

func timelineEntityFromMap(id, kind string, props map[string]any) *timelinepb.TimelineEntityV2 {
	st, err := structpb.NewStruct(props)
	if err != nil {
		st = &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return &timelinepb.TimelineEntityV2{
		Id:    strings.TrimSpace(id),
		Kind:  strings.TrimSpace(kind),
		Props: st,
	}
}

func semPayloadFromEventData(in *ThinkingModePayload) *semThinkingModePayload {
	if in == nil {
		return nil
	}
	return &semThinkingModePayload{
		Mode:      in.Mode,
		Phase:     in.Phase,
		Reasoning: in.Reasoning,
		ExtraData: in.ExtraData,
	}
}

type semThinkingModePayload struct {
	Mode      string         `json:"mode,omitempty"`
	Phase     string         `json:"phase,omitempty"`
	Reasoning string         `json:"reasoning,omitempty"`
	ExtraData map[string]any `json:"extraData,omitempty"`
}

type semThinkingModeStarted struct {
	ItemID string                  `json:"itemId,omitempty"`
	Data   *semThinkingModePayload `json:"data,omitempty"`
}

type semThinkingModeUpdate struct {
	ItemID string                  `json:"itemId,omitempty"`
	Data   *semThinkingModePayload `json:"data,omitempty"`
}

type semThinkingModeCompleted struct {
	ItemID  string                  `json:"itemId,omitempty"`
	Data    *semThinkingModePayload `json:"data,omitempty"`
	Success bool                    `json:"success"`
	Error   string                  `json:"error,omitempty"`
}
