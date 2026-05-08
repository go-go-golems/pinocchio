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

func TestToolCallPluginPublishesCanonicalToolEvents(t *testing.T) {
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
	meta := gepevents.EventMetadata{SessionID: "sid"}
	corr := gepevents.Correlation{
		SessionID:      "sid",
		Provider:       "openai",
		ResponseID:     "resp_1",
		ChoiceIndex:    int32Ptr(0),
		StreamKind:     gepevents.StreamKindToolCall,
		ToolCallID:     "call-1",
		ToolCallIndex:  int32Ptr(0),
		CorrelationKey: "tool:call-1",
	}

	for _, event := range []gepevents.Event{
		gepevents.NewToolCallStartedEvent(meta, corr, "call-1", "lookup"),
		gepevents.NewToolCallArgumentsDeltaEvent(meta, corr, "call-1", `{"symbol"`, `{"symbol":"BTC"}`, 1),
		gepevents.NewToolCallRequestedEvent(meta, corr, "call-1", "lookup", `{"symbol":"BTC"}`),
		gepevents.NewToolExecutionStartedEvent(meta, corr, "call-1", "lookup", `{"symbol":"BTC"}`),
		gepevents.NewToolResultReadyEvent(meta, corr, "call-1", "lookup", `{"price":123}`, "success"),
		gepevents.NewToolCallFinishedEvent(meta, corr, "call-1", "lookup", "completed"),
	} {
		handled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, event)
		require.NoError(t, err)
		require.True(t, handled)
	}

	require.Len(t, published, 6)
	require.Equal(t, chatapp.EventChatToolCallStarted, published[0].Name)
	started := published[0].Payload.(*chatappv1.ChatToolCallStarted)
	require.Equal(t, "chat-msg-1", started.GetMessageId())
	require.Equal(t, "call-1", started.GetToolCallId())
	require.Equal(t, "lookup", started.GetToolName())
	require.Equal(t, "tool:call-1", started.GetCorrelation().GetCorrelationKey())

	require.Equal(t, chatapp.EventChatToolCallArgumentsDelta, published[1].Name)
	args := published[1].Payload.(*chatappv1.ChatToolCallArgumentsDelta)
	require.Equal(t, `{"symbol"`, args.GetArgumentsDelta())
	require.Equal(t, `{"symbol":"BTC"}`, args.GetInput())

	require.Equal(t, chatapp.EventChatToolResultReady, published[4].Name)
	result := published[4].Payload.(*chatappv1.ChatToolResultReady)
	require.Equal(t, `{"price":123}`, result.GetResult())
}

func TestToolCallPluginProjectsCanonicalEventsToCompatibilityUIAndTimeline(t *testing.T) {
	plugin := NewToolCallPlugin()
	corr := &chatappv1.CorrelationInfo{
		Provider:       "openai",
		ResponseId:     "resp_1",
		ChoiceIndex:    int32Ptr(0),
		StreamKind:     gepevents.StreamKindToolCall,
		ToolCallId:     "call-2",
		ToolCallIndex:  int32Ptr(0),
		CorrelationKey: "tool:call-2",
	}
	requested := sessionstream.Event{Name: EventToolCallRequested, SessionId: "sid", Ordinal: 10, Payload: &chatappv1.ChatToolCallRequested{
		MessageId:   "chat-msg-2",
		ToolCallId:  "call-2",
		ToolName:    "inventory",
		Input:       `{"coin":"ETH"}`,
		Status:      "pending",
		Correlation: corr,
	}}

	uiEvents, handled, err := plugin.ProjectUI(context.Background(), requested, nil, toolCallStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, uiEvents, 1)
	require.Equal(t, UIToolCallStarted, uiEvents[0].Name)
	uiPayload := uiEvents[0].Payload.(*chatappv1.ToolCallUpdate)
	require.Equal(t, "call-2", uiPayload.GetToolCallId())
	require.Equal(t, "inventory", uiPayload.GetToolName())
	require.Equal(t, `{"coin":"ETH"}`, uiPayload.GetInput())
	require.Equal(t, "tool:call-2", uiPayload.GetCorrelationKey())

	entities, handled, err := plugin.ProjectTimeline(context.Background(), requested, nil, toolCallStaticTimelineView{})
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
	execution := sessionstream.Event{Name: EventToolExecutionStarted, SessionId: "sid", Ordinal: 11, Payload: &chatappv1.ChatToolExecutionStarted{MessageId: "chat-msg-2", ToolCallId: "call-2", Executing: true, Status: "executing", Correlation: corr}}
	entities, handled, err = plugin.ProjectTimeline(context.Background(), execution, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	entityPayload = entities[0].Payload.(*chatappv1.ToolCallEntity)
	require.Equal(t, "inventory", entityPayload.GetToolName())
	require.Equal(t, `{"coin":"ETH"}`, entityPayload.GetInput())
	require.True(t, entityPayload.GetExecuting())
	require.Equal(t, "executing", entityPayload.GetStatus())

	resultEvent := sessionstream.Event{Name: EventToolResultReady, SessionId: "sid", Ordinal: 12, Payload: &chatappv1.ChatToolResultReady{MessageId: "chat-msg-2", ToolCallId: "call-2", ToolName: "inventory", Result: `{"ok":true}`, Status: "success", Correlation: corr}}
	entities, handled, err = plugin.ProjectTimeline(context.Background(), resultEvent, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	require.Equal(t, TimelineEntityToolResult, entities[0].Kind)
	resultEntity := entities[0].Payload.(*chatappv1.ToolResultEntity)
	require.Equal(t, "call-2", resultEntity.GetToolCallId())
	require.Equal(t, "inventory", resultEntity.GetToolName())
	require.Equal(t, `{"ok":true}`, resultEntity.GetResult())
}

func TestToolCallPluginIgnoresLegacyToolEvents(t *testing.T) {
	plugin := NewToolCallPlugin()
	handled, err := plugin.HandleRuntimeEvent(context.Background(), chatapp.RuntimeEventContext{SessionID: "sid", MessageID: "chat-msg-1"}, gepevents.NewToolCallEvent(gepevents.EventMetadata{SessionID: "sid"}, gepevents.ToolCall{ID: "call-1", Name: "lookup"}))
	require.NoError(t, err)
	require.False(t, handled)
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
