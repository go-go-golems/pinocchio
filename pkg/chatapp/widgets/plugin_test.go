package widgets

import (
	"context"
	"testing"

	widgetv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/widgets/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestWidgetPluginProjectsLifecycle(t *testing.T) {
	plugin := &WidgetPlugin{}
	props, err := structpb.NewStruct(map[string]any{"title": "Boots", "count": 1})
	if err != nil {
		t.Fatalf("props struct: %v", err)
	}
	started, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventWidgetInstanceStarted, Payload: &widgetv1.WidgetInstanceStarted{InstanceId: "widget-1", WidgetName: "ProductCarousel", ParentMessageId: "msg-1", Status: widgetv1.WidgetStatus_WIDGET_STATUS_STREAMING, Props: props}}, nil, nil)
	if err != nil || !handled {
		t.Fatalf("started handled=%v err=%v", handled, err)
	}
	if len(started) != 1 || started[0].Kind != TimelineEntityWidgetInstance || started[0].Id != "widget-1" {
		t.Fatalf("unexpected started entity: %#v", started)
	}
	startedPayload, ok := started[0].Payload.(*widgetv1.WidgetInstanceEntity)
	if !ok || startedPayload.GetWidgetName() != "ProductCarousel" || startedPayload.GetStatus() != widgetv1.WidgetStatus_WIDGET_STATUS_STREAMING {
		t.Fatalf("unexpected started payload: %#v", started[0].Payload)
	}

	patch, err := structpb.NewStruct(map[string]any{"count": 2, "ignored": true})
	if err != nil {
		t.Fatalf("patch struct: %v", err)
	}
	patched, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventWidgetInstancePatched, Payload: &widgetv1.WidgetInstancePatched{InstanceId: "widget-1", Status: widgetv1.WidgetStatus_WIDGET_STATUS_STREAMING, Patch: patch, PatchPaths: []string{"count"}}}, nil, fakeTimelineView{entity: started[0]})
	if err != nil || !handled {
		t.Fatalf("patched handled=%v err=%v", handled, err)
	}
	patchedPayload, ok := patched[0].Payload.(*widgetv1.WidgetInstanceEntity)
	if !ok {
		t.Fatalf("unexpected patched payload: %#v", patched[0].Payload)
	}
	if got := patchedPayload.GetProps().AsMap()["count"]; got != float64(2) {
		t.Fatalf("expected patched count=2, got %#v", got)
	}
	if _, ok := patchedPayload.GetProps().AsMap()["ignored"]; ok {
		t.Fatalf("selective patch should not include ignored field: %#v", patchedPayload.GetProps().AsMap())
	}

	completed, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventWidgetInstanceCompleted, Payload: &widgetv1.WidgetInstanceCompleted{InstanceId: "widget-1"}}, nil, fakeTimelineView{entity: patched[0]})
	if err != nil || !handled {
		t.Fatalf("completed handled=%v err=%v", handled, err)
	}
	completedPayload, ok := completed[0].Payload.(*widgetv1.WidgetInstanceEntity)
	if !ok || completedPayload.GetStatus() != widgetv1.WidgetStatus_WIDGET_STATUS_READY {
		t.Fatalf("unexpected completed payload: %#v", completed[0].Payload)
	}
}

func TestWidgetPluginProjectsRemovedAsTombstone(t *testing.T) {
	plugin := &WidgetPlugin{}
	removed, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventWidgetInstanceRemoved, Payload: &widgetv1.WidgetInstanceRemoved{InstanceId: "widget-1"}}, nil, nil)
	if err != nil || !handled {
		t.Fatalf("removed handled=%v err=%v", handled, err)
	}
	if len(removed) != 1 || !removed[0].Tombstone || removed[0].Id != "widget-1" {
		t.Fatalf("unexpected removed entity: %#v", removed)
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
