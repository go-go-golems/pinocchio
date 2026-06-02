package frontendtools

import (
	"context"
	"testing"

	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestPluginProjectsFrontendToolTimeline(t *testing.T) {
	plugin := &Plugin{}
	input, err := structpb.NewStruct(map[string]any{"sku": "retro-boot"})
	if err != nil {
		t.Fatalf("input struct: %v", err)
	}
	entities, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventCallRequested, Payload: &toolv1.FrontendToolCallRequested{MessageId: "msg-1", ToolCallId: "call-1", ToolName: "cart.add", Input: input, Mode: toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_AUTO, Status: "requested"}}, nil, nil)
	if err != nil || !handled {
		t.Fatalf("ProjectTimeline request handled=%v err=%v", handled, err)
	}
	if len(entities) != 1 || entities[0].Kind != TimelineEntityFrontendToolCall || entities[0].Id != "call-1" {
		t.Fatalf("unexpected request entities: %#v", entities)
	}
	payload, ok := entities[0].Payload.(*toolv1.FrontendToolCallEntity)
	if !ok || payload.GetToolName() != "cart.add" || payload.GetStatus() != "requested" {
		t.Fatalf("unexpected request payload: %#v", entities[0].Payload)
	}

	result, err := structpb.NewStruct(map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("result struct: %v", err)
	}
	view := fakeTimelineView{entity: entities[0]}
	updated, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventResultReceived, Payload: &toolv1.FrontendToolResultReceived{MessageId: "msg-1", ToolCallId: "call-1", ToolName: "cart.add", Result: result, Status: "success"}}, nil, view)
	if err != nil || !handled {
		t.Fatalf("ProjectTimeline result handled=%v err=%v", handled, err)
	}
	updatedPayload, ok := updated[0].Payload.(*toolv1.FrontendToolCallEntity)
	if !ok || updatedPayload.GetStatus() != "success" || updatedPayload.GetResult() == nil {
		t.Fatalf("unexpected result payload: %#v", updated[0].Payload)
	}
}

type fakeTimelineView struct {
	entity sessionstream.TimelineEntity
}

func (v fakeTimelineView) Get(kind, id string) (sessionstream.TimelineEntity, bool) {
	if v.entity.Kind == kind && v.entity.Id == id {
		return v.entity, true
	}
	return sessionstream.TimelineEntity{}, false
}

func (v fakeTimelineView) List(kind string) []sessionstream.TimelineEntity {
	if v.entity.Kind == kind {
		return []sessionstream.TimelineEntity{v.entity}
	}
	return nil
}

func (v fakeTimelineView) Ordinal() uint64 { return 0 }
