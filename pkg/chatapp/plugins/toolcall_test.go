package plugins

import (
	"context"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestToolCallPluginHandleRuntimeEvent(t *testing.T) {
	plugin := NewToolCallPlugin()
	var published []sessionstream.Event
	runtime := chatapp.RuntimeEventContext{
		SessionID: "sid",
		MessageID: "chat-msg-1",
		Publish: func(_ context.Context, eventName string, payload proto.Message) error {
			published = append(published, sessionstream.Event{Name: eventName, SessionId: "sid", Payload: payload})
			return nil
		},
	}

	handled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewToolCallEvent(gepevents.EventMetadata{SessionID: "sid"}, gepevents.ToolCall{ID: "call-1", Name: "lookup", Input: `{"symbol":"BTC"}`}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 1)
	require.Equal(t, EventToolCallStarted, published[0].Name)
	callPayload := published[0].Payload.(*chatappv1.ToolCallUpdate)
	require.Equal(t, "chat-msg-1", callPayload.GetMessageId())
	require.Equal(t, "call-1", callPayload.GetToolCallId())
	require.Equal(t, "lookup", callPayload.GetToolName())
	require.Equal(t, `{"symbol":"BTC"}`, callPayload.GetInput())
	require.Equal(t, "pending", callPayload.GetStatus())

	handled, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewToolResultEvent(gepevents.EventMetadata{SessionID: "sid"}, gepevents.ToolResult{ID: "call-1", Name: "lookup", Result: `{"price":123}`}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 3)
	require.Equal(t, EventToolResultReady, published[1].Name)
	require.Equal(t, EventToolCallFinished, published[2].Name)
	resultPayload := published[1].Payload.(*chatappv1.ToolResultUpdate)
	require.Equal(t, "call-1", resultPayload.GetToolCallId())
	require.Equal(t, "lookup", resultPayload.GetToolName())
	require.Equal(t, `{"price":123}`, resultPayload.GetResult())
}

func TestToolCallPluginProjectsUIAndTimeline(t *testing.T) {
	plugin := NewToolCallPlugin()
	callPayload := &chatappv1.ToolCallUpdate{
		MessageId:  "chat-msg-2",
		ToolCallId: "call-2",
		ToolName:   "inventory",
		Input:      `{"coin":"ETH"}`,
		Status:     "pending",
	}

	uiEvents, handled, err := plugin.ProjectUI(context.Background(), sessionstream.Event{Name: EventToolCallStarted, SessionId: "sid", Ordinal: 10, Payload: callPayload}, nil, toolCallStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, uiEvents, 1)
	require.Equal(t, UIToolCallStarted, uiEvents[0].Name)

	entities, handled, err := plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventToolCallStarted, SessionId: "sid", Ordinal: 10, Payload: callPayload}, nil, toolCallStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	require.Equal(t, TimelineEntityToolCall, entities[0].Kind)
	entityPayload := entities[0].Payload.(*chatappv1.ToolCallEntity)
	require.Equal(t, "chat-msg-2", entityPayload.GetMessageId())
	require.Equal(t, "call-2", entityPayload.GetToolCallId())
	require.Equal(t, "inventory", entityPayload.GetToolName())
	require.Equal(t, `{"coin":"ETH"}`, entityPayload.GetInput())
	require.Equal(t, "pending", entityPayload.GetStatus())

	view := toolCallStaticTimelineView{entities: map[string]sessionstream.TimelineEntity{
		TimelineEntityToolCall + "/call-2": {
			Kind: TimelineEntityToolCall,
			Id:   "call-2",
			Payload: &chatappv1.ToolCallEntity{
				MessageId:  "chat-msg-2",
				ToolCallId: "call-2",
				ToolName:   "inventory",
				Input:      `{"coin":"ETH"}`,
				Status:     "pending",
			},
		},
	}}
	entities, handled, err = plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventToolCallUpdated, SessionId: "sid", Ordinal: 11, Payload: &chatappv1.ToolCallUpdate{MessageId: "chat-msg-2", ToolCallId: "call-2", Executing: true}}, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	entityPayload = entities[0].Payload.(*chatappv1.ToolCallEntity)
	require.Equal(t, "inventory", entityPayload.GetToolName())
	require.Equal(t, `{"coin":"ETH"}`, entityPayload.GetInput())
	require.True(t, entityPayload.GetExecuting())
	require.Equal(t, "executing", entityPayload.GetStatus())

	resultPayload := &chatappv1.ToolResultUpdate{MessageId: "chat-msg-2", ToolCallId: "call-2", ToolName: "inventory", Result: `{"ok":true}`, Status: "success"}
	entities, handled, err = plugin.ProjectTimeline(context.Background(), sessionstream.Event{Name: EventToolResultReady, SessionId: "sid", Ordinal: 12, Payload: resultPayload}, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	require.Equal(t, TimelineEntityToolResult, entities[0].Kind)
	resultEntity := entities[0].Payload.(*chatappv1.ToolResultEntity)
	require.Equal(t, "call-2", resultEntity.GetToolCallId())
	require.Equal(t, "inventory", resultEntity.GetToolName())
	require.Equal(t, `{"ok":true}`, resultEntity.GetResult())
}

type toolCallStaticTimelineView struct {
	entities map[string]sessionstream.TimelineEntity
}

func (v toolCallStaticTimelineView) Get(kind, id string) (sessionstream.TimelineEntity, bool) {
	if v.entities == nil {
		return sessionstream.TimelineEntity{}, false
	}
	entity, ok := v.entities[kind+"/"+id]
	return entity, ok
}

func (v toolCallStaticTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (v toolCallStaticTimelineView) Ordinal() uint64                            { return 0 }
