package webchat

import (
	"context"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	semMw "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/middleware"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

func registerThinkingModeTimelineHandlers() {
	RegisterTimelineHandler("thinking.mode.started", thinkingModeTimelineHandler)
	RegisterTimelineHandler("thinking.mode.update", thinkingModeTimelineHandler)
	RegisterTimelineHandler("thinking.mode.completed", thinkingModeTimelineHandler)
}

func thinkingModeTimelineHandler(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, _ int64) error {
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
	return p.Upsert(ctx, ev.Seq, timelineEntityV2FromProtoMessage(itemID, "thinking_mode", &timelinepb.ThinkingModeSnapshotV1{
		SchemaVersion: 1,
		Status:        status,
		Mode:          mode,
		Phase:         phase,
		Reasoning:     reason,
		Success:       success,
		Error:         errStr,
	}))
}
