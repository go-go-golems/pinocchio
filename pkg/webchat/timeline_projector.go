package webchat

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	sempb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/base"
	semMw "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/middleware"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type semEnvelope struct {
	Sem   bool `json:"sem"`
	Event struct {
		Type     string          `json:"type"`
		ID       string          `json:"id"`
		Seq      uint64          `json:"seq"`
		StreamID string          `json:"stream_id"`
		Data     json.RawMessage `json:"data"`
	} `json:"event"`
}

// TimelineProjector converts SEM frames into sem.timeline.* projection snapshots and persists them via a TimelineStore.
//
// It is per-conversation and keeps small in-memory caches to:
// - remember message roles (assistant vs thinking) across llm.delta events
// - throttle high-frequency writes (llm.delta)
type TimelineProjector struct {
	convID   string
	store    TimelineStore
	onUpsert func(entity *timelinepb.TimelineEntityV1, version uint64)

	mu           sync.Mutex
	msgRoles     map[string]string
	lastMsgWrite map[string]int64
	toolNames    map[string]string
	toolInputs   map[string]*structpb.Struct
}

func NewTimelineProjector(convID string, store TimelineStore, onUpsert func(entity *timelinepb.TimelineEntityV1, version uint64)) *TimelineProjector {
	return &TimelineProjector{
		convID:       convID,
		store:        store,
		onUpsert:     onUpsert,
		msgRoles:     map[string]string{},
		lastMsgWrite: map[string]int64{},
		toolNames:    map[string]string{},
		toolInputs:   map[string]*structpb.Struct{},
	}
}

func (p *TimelineProjector) upsert(ctx context.Context, version uint64, entity *timelinepb.TimelineEntityV1) error {
	if p == nil || p.store == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := p.store.Upsert(ctx, p.convID, version, entity); err != nil {
		return err
	}
	if p.onUpsert != nil {
		p.onUpsert(entity, version)
	}
	return nil
}

// Upsert exposes timeline writes for custom SEM handlers.
func (p *TimelineProjector) Upsert(ctx context.Context, version uint64, entity *timelinepb.TimelineEntityV1) error {
	return p.upsert(ctx, version, entity)
}

func (p *TimelineProjector) ApplySemFrame(ctx context.Context, frame []byte) error {
	if p == nil || p.store == nil {
		return nil
	}
	if len(frame) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var env semEnvelope
	if err := json.Unmarshal(frame, &env); err != nil {
		return nil
	}
	if !env.Sem {
		return nil
	}
	if strings.TrimSpace(env.Event.Type) == "" || strings.TrimSpace(env.Event.ID) == "" {
		return nil
	}
	seq := env.Event.Seq
	if seq == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	customEvent := TimelineSemEvent{
		Type:     env.Event.Type,
		ID:       env.Event.ID,
		Seq:      env.Event.Seq,
		StreamID: env.Event.StreamID,
		Data:     env.Event.Data,
	}
	if handled, err := handleTimelineHandlers(ctx, p, customEvent, now); handled {
		return err
	}
	switch env.Event.Type {
	case "llm.start", "llm.thinking.start":
		var pb sempb.LlmStart
		if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
			return nil
		}
		role := strings.TrimSpace(pb.Role)
		if role == "" {
			if env.Event.Type == "llm.thinking.start" {
				role = "thinking"
			} else {
				role = "assistant"
			}
		}
		p.mu.Lock()
		p.msgRoles[env.Event.ID] = role
		p.mu.Unlock()
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   env.Event.ID,
			Kind: "message",
			Snapshot: &timelinepb.TimelineEntityV1_Message{
				Message: &timelinepb.MessageSnapshotV1{
					SchemaVersion: 1,
					Role:          role,
					Content:       "",
					Streaming:     true,
				},
			},
		})
		return err

	case "llm.delta", "llm.thinking.delta":
		var pb sempb.LlmDelta
		if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
			return nil
		}
		cum := pb.Cumulative
		if cum == "" {
			return nil
		}

		// Throttle writes: keep the DB churn bounded during token streaming.
		p.mu.Lock()
		last := p.lastMsgWrite[env.Event.ID]
		role := p.msgRoles[env.Event.ID]
		if role == "" {
			role = "assistant"
		}
		if now-last < 250 {
			p.mu.Unlock()
			return nil
		}
		p.lastMsgWrite[env.Event.ID] = now
		p.mu.Unlock()

		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   env.Event.ID,
			Kind: "message",
			Snapshot: &timelinepb.TimelineEntityV1_Message{
				Message: &timelinepb.MessageSnapshotV1{
					SchemaVersion: 1,
					Role:          role,
					Content:       cum,
					Streaming:     true,
				},
			},
		})
		return err

	case "llm.final":
		var pb sempb.LlmFinal
		if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
			return nil
		}
		p.mu.Lock()
		role := p.msgRoles[env.Event.ID]
		if role == "" {
			role = "assistant"
		}
		delete(p.lastMsgWrite, env.Event.ID)
		p.mu.Unlock()
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   env.Event.ID,
			Kind: "message",
			Snapshot: &timelinepb.TimelineEntityV1_Message{
				Message: &timelinepb.MessageSnapshotV1{
					SchemaVersion: 1,
					Role:          role,
					Content:       pb.Text,
					Streaming:     false,
				},
			},
		})
		return err

	case "llm.thinking.final":
		// sem.base.llm.LlmDone contains only an id; treat as "stop streaming".
		p.mu.Lock()
		role := p.msgRoles[env.Event.ID]
		if role == "" {
			role = "thinking"
		}
		delete(p.lastMsgWrite, env.Event.ID)
		p.mu.Unlock()
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   env.Event.ID,
			Kind: "message",
			Snapshot: &timelinepb.TimelineEntityV1_Message{
				Message: &timelinepb.MessageSnapshotV1{
					SchemaVersion: 1,
					Role:          role,
					Streaming:     false,
				},
			},
		})
		return err

	case "tool.start":
		var pb sempb.ToolStart
		if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
			return nil
		}
		p.mu.Lock()
		p.toolNames[env.Event.ID] = pb.Name
		p.toolInputs[env.Event.ID] = pb.Input
		p.mu.Unlock()
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   env.Event.ID,
			Kind: "tool_call",
			Snapshot: &timelinepb.TimelineEntityV1_ToolCall{
				ToolCall: &timelinepb.ToolCallSnapshotV1{
					SchemaVersion: 1,
					Name:          pb.Name,
					Input:         pb.Input,
					Status:        "running",
					Progress:      0,
					Done:          false,
				},
			},
		})
		return err

	case "tool.done":
		p.mu.Lock()
		name := p.toolNames[env.Event.ID]
		input := p.toolInputs[env.Event.ID]
		p.mu.Unlock()
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   env.Event.ID,
			Kind: "tool_call",
			Snapshot: &timelinepb.TimelineEntityV1_ToolCall{
				ToolCall: &timelinepb.ToolCallSnapshotV1{
					SchemaVersion: 1,
					Name:          name,
					Input:         input,
					Status:        "completed",
					Progress:      1,
					Done:          true,
				},
			},
		})
		return err

	case "tool.result":
		var pb sempb.ToolResult
		if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
			return nil
		}
		resultEntityID := env.Event.ID + ":result"
		if strings.TrimSpace(pb.CustomKind) != "" {
			resultEntityID = env.Event.ID + ":custom"
		}
		resultStruct, _ := structpb.NewStruct(map[string]any{"raw": pb.Result})
		// Best-effort: if the tool result is JSON, store it structurally as well.
		var obj any
		if strings.HasPrefix(strings.TrimSpace(pb.Result), "{") {
			_ = json.Unmarshal([]byte(pb.Result), &obj)
		}
		if m, ok := obj.(map[string]any); ok {
			if st, err := structpb.NewStruct(m); err == nil {
				resultStruct = st
			}
		}
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   resultEntityID,
			Kind: "tool_result",
			Snapshot: &timelinepb.TimelineEntityV1_ToolResult{
				ToolResult: &timelinepb.ToolResultSnapshotV1{
					SchemaVersion: 1,
					ToolCallId:    env.Event.ID,
					Result:        resultStruct,
					ResultRaw:     pb.Result,
					CustomKind:    pb.CustomKind,
				},
			},
		})
		return err

	case "thinking.mode.started", "thinking.mode.update", "thinking.mode.completed":
		var (
			itemID  string
			mode    string
			phase   string
			reason  string
			success bool
			errStr  string
		)
		switch env.Event.Type {
		case "thinking.mode.started":
			var pb semMw.ThinkingModeStarted
			if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
				return nil
			}
			itemID = pb.ItemId
			if pb.Data != nil {
				mode, phase, reason = pb.Data.Mode, pb.Data.Phase, pb.Data.Reasoning
			}
			success = true
		case "thinking.mode.update":
			var pb semMw.ThinkingModeUpdate
			if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
				return nil
			}
			itemID = pb.ItemId
			if pb.Data != nil {
				mode, phase, reason = pb.Data.Mode, pb.Data.Phase, pb.Data.Reasoning
			}
			success = true
		case "thinking.mode.completed":
			var pb semMw.ThinkingModeCompleted
			if err := protojson.Unmarshal(env.Event.Data, &pb); err != nil {
				return nil
			}
			itemID = pb.ItemId
			if pb.Data != nil {
				mode, phase, reason = pb.Data.Mode, pb.Data.Phase, pb.Data.Reasoning
			}
			success = pb.Success
			errStr = pb.Error
		}
		if strings.TrimSpace(itemID) == "" {
			itemID = env.Event.ID
		}
		status := "active"
		if env.Event.Type == "thinking.mode.completed" {
			if success {
				status = "completed"
			} else {
				status = "error"
			}
		}
		if errStr != "" {
			status = "error"
		}
		err := p.upsert(ctx, seq, &timelinepb.TimelineEntityV1{
			Id:   itemID,
			Kind: "thinking_mode",
			Snapshot: &timelinepb.TimelineEntityV1_ThinkingMode{
				ThinkingMode: &timelinepb.ThinkingModeSnapshotV1{
					SchemaVersion: 1,
					Status:        status,
					Mode:          mode,
					Phase:         phase,
					Reasoning:     reason,
					Success:       success,
					Error:         errStr,
				},
			},
		})
		return err

	}

	return nil
}
