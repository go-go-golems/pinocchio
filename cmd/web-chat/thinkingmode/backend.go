package thinkingmode

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	pinevents "github.com/go-go-golems/pinocchio/pkg/inference/events"
	semMw "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/middleware"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	semregistry "github.com/go-go-golems/pinocchio/pkg/sem/registry"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var registerOnce sync.Once

// Register wires thinking-mode integration for cmd/web-chat.
// It is safe to call multiple times.
func Register() {
	registerOnce.Do(func() {
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
		var pb semMw.ThinkingModeStarted
		if err := protojson.Unmarshal(ev.Data, &pb); err != nil {
			return nil
		}
		itemID = pb.ItemId
		if pb.Data != nil {
			mode, phase, reason = pb.Data.Mode, pb.Data.Phase, pb.Data.Reasoning
		}
		success = true

	case "thinking.mode.update":
		var pb semMw.ThinkingModeUpdate
		if err := protojson.Unmarshal(ev.Data, &pb); err != nil {
			return nil
		}
		itemID = pb.ItemId
		if pb.Data != nil {
			mode, phase, reason = pb.Data.Mode, pb.Data.Phase, pb.Data.Reasoning
		}
		success = true

	case "thinking.mode.completed":
		var pb semMw.ThinkingModeCompleted
		if err := protojson.Unmarshal(ev.Data, &pb); err != nil {
			return nil
		}
		itemID = pb.ItemId
		if pb.Data != nil {
			mode, phase, reason = pb.Data.Mode, pb.Data.Phase, pb.Data.Reasoning
		}
		success = pb.Success
		errStr = pb.Error

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

	return p.Upsert(ctx, ev.Seq, timelineEntityFromProtoMessage(itemID, "thinking_mode", &timelinepb.ThinkingModeSnapshotV1{
		SchemaVersion: 1,
		Status:        status,
		Mode:          mode,
		Phase:         phase,
		Reasoning:     reason,
		Success:       success,
		Error:         errStr,
	}))
}

func registerSemTranslatorHandlers() {
	semregistry.RegisterByType[*pinevents.EventThinkingModeStarted](func(ev *pinevents.EventThinkingModeStarted) ([][]byte, error) {
		var payload *semMw.ThinkingModePayload
		if ev.Data != nil {
			extra, err := mapToStruct(ev.Data.ExtraData)
			if err != nil {
				return nil, err
			}
			payload = &semMw.ThinkingModePayload{
				Mode:      ev.Data.Mode,
				Phase:     ev.Data.Phase,
				Reasoning: ev.Data.Reasoning,
				ExtraData: extra,
			}
		}
		data, err := protoToRaw(&semMw.ThinkingModeStarted{ItemId: ev.ItemID, Data: payload})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "thinking.mode.started", "id": ev.ItemID, "data": data})}, nil
	})

	semregistry.RegisterByType[*pinevents.EventThinkingModeUpdate](func(ev *pinevents.EventThinkingModeUpdate) ([][]byte, error) {
		var payload *semMw.ThinkingModePayload
		if ev.Data != nil {
			extra, err := mapToStruct(ev.Data.ExtraData)
			if err != nil {
				return nil, err
			}
			payload = &semMw.ThinkingModePayload{
				Mode:      ev.Data.Mode,
				Phase:     ev.Data.Phase,
				Reasoning: ev.Data.Reasoning,
				ExtraData: extra,
			}
		}
		data, err := protoToRaw(&semMw.ThinkingModeUpdate{ItemId: ev.ItemID, Data: payload})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "thinking.mode.update", "id": ev.ItemID, "data": data})}, nil
	})

	semregistry.RegisterByType[*pinevents.EventThinkingModeCompleted](func(ev *pinevents.EventThinkingModeCompleted) ([][]byte, error) {
		var payload *semMw.ThinkingModePayload
		if ev.Data != nil {
			extra, err := mapToStruct(ev.Data.ExtraData)
			if err != nil {
				return nil, err
			}
			payload = &semMw.ThinkingModePayload{
				Mode:      ev.Data.Mode,
				Phase:     ev.Data.Phase,
				Reasoning: ev.Data.Reasoning,
				ExtraData: extra,
			}
		}
		data, err := protoToRaw(&semMw.ThinkingModeCompleted{
			ItemId:  ev.ItemID,
			Data:    payload,
			Success: ev.Success,
			Error:   ev.Error,
		})
		if err != nil {
			return nil, err
		}
		return [][]byte{wrapSem(map[string]any{"type": "thinking.mode.completed", "id": ev.ItemID, "data": data})}, nil
	})
}

func wrapSem(ev map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{"sem": true, "event": ev})
	return b
}

func protoToRaw(m proto.Message) (json.RawMessage, error) {
	if m == nil {
		return nil, nil
	}
	b, err := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false,
	}.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func mapToStruct(m map[string]any) (*structpb.Struct, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return structpb.NewStruct(m)
}

func timelineEntityFromProtoMessage(id, kind string, msg proto.Message) *timelinepb.TimelineEntityV2 {
	return &timelinepb.TimelineEntityV2{
		Id:    strings.TrimSpace(id),
		Kind:  strings.TrimSpace(kind),
		Props: timelineStructFromProtoMessage(msg),
	}
}

func timelineStructFromProtoMessage(msg proto.Message) *structpb.Struct {
	if msg == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	raw, err := protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   false,
	}.Marshal(msg)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	st, err := structpb.NewStruct(m)
	if err != nil || st == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return st
}
