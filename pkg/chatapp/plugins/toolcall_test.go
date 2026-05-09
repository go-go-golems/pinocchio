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

func TestToolCallPluginProjectsCanonicalEventsToUIAndTimeline(t *testing.T) {
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
	require.Equal(t, EventToolCallRequested, uiEvents[0].Name)
	uiPayload := uiEvents[0].Payload.(*chatappv1.ChatToolCallRequested)
	require.Equal(t, "call-2", uiPayload.GetToolCallId())
	require.Equal(t, "inventory", uiPayload.GetToolName())
	require.Equal(t, `{"coin":"ETH"}`, uiPayload.GetInput())
	require.Equal(t, "tool:call-2", uiPayload.GetCorrelation().GetCorrelationKey())

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
	require.Equal(t, "tool:call-2", entityPayload.GetCorrelation().GetCorrelationKey())

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
	require.Equal(t, "tool:call-2", resultEntity.GetCorrelation().GetCorrelationKey())
}

func TestToolCallPluginSparseProjectionMatrix(t *testing.T) {
	plugin := NewToolCallPlugin()
	fullCorr := &chatappv1.CorrelationInfo{
		Provider:             "openai-responses",
		Model:                "gpt-test",
		ResponseId:           "resp_tool",
		ItemId:               "fc_1",
		OutputIndex:          int32Ptr(0),
		StreamKind:           gepevents.StreamKindToolCall,
		ToolCallId:           "call-sparse",
		ToolCallIndex:        int32Ptr(0),
		CorrelationKey:       "tool:call-sparse",
		ParentCorrelationKey: "provider-call-key",
	}
	segmentOnlyCorr := &chatappv1.CorrelationInfo{ToolCallId: "call-sparse", ToolCallIndex: int32Ptr(0)}

	tests := []struct {
		name  string
		view  sessionstream.TimelineView
		event sessionstream.Event
		check func(t *testing.T, entity *chatappv1.ToolCallEntity)
	}{
		{
			name: "TOOL-PROJECTION-01 sparse finish preserves name input and correlation",
			view: toolCallStaticTimelineView{entities: map[string]sessionstream.TimelineEntity{
				TimelineEntityToolCall + "/call-sparse": {
					Kind: TimelineEntityToolCall,
					Id:   "call-sparse",
					Payload: &chatappv1.ToolCallEntity{
						MessageId:   "chat-msg-tool",
						ToolCallId:  "call-sparse",
						ToolName:    "inventory",
						Input:       `{"coin":"ETH"}`,
						Executing:   true,
						Status:      "executing",
						Correlation: fullCorr,
					},
				},
			}},
			event: sessionstream.Event{Name: EventToolCallFinished, SessionId: "sid", Payload: &chatappv1.ChatToolCallFinished{
				MessageId:   "chat-msg-tool",
				ToolCallId:  "call-sparse",
				Status:      "completed",
				Correlation: segmentOnlyCorr,
			}},
			check: func(t *testing.T, entity *chatappv1.ToolCallEntity) {
				t.Helper()
				require.Equal(t, "inventory", entity.GetToolName())
				require.Equal(t, `{"coin":"ETH"}`, entity.GetInput())
				require.False(t, entity.GetExecuting())
				require.Equal(t, "completed", entity.GetStatus())
				requireToolProjectionFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "TOOL-PROJECTION-02 sparse argument delta preserves known tool name",
			view: toolCallStaticTimelineView{entities: map[string]sessionstream.TimelineEntity{
				TimelineEntityToolCall + "/call-sparse": {
					Kind: TimelineEntityToolCall,
					Id:   "call-sparse",
					Payload: &chatappv1.ToolCallEntity{
						MessageId:   "chat-msg-tool",
						ToolCallId:  "call-sparse",
						ToolName:    "inventory",
						Status:      "pending",
						Correlation: fullCorr,
					},
				},
			}},
			event: sessionstream.Event{Name: EventToolCallArgumentsDelta, SessionId: "sid", Payload: &chatappv1.ChatToolCallArgumentsDelta{
				MessageId:      "chat-msg-tool",
				ToolCallId:     "call-sparse",
				ArgumentsDelta: `"ETH"}`,
				Input:          `{"coin":"ETH"}`,
				Status:         "streaming_args",
				Correlation:    segmentOnlyCorr,
			}},
			check: func(t *testing.T, entity *chatappv1.ToolCallEntity) {
				t.Helper()
				require.Equal(t, "inventory", entity.GetToolName())
				require.Equal(t, `{"coin":"ETH"}`, entity.GetInput())
				require.Equal(t, "streaming_args", entity.GetStatus())
				requireToolProjectionFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "TOOL-PROJECTION-03 empty tool name stays empty instead of persisting display fallback",
			view: toolCallStaticTimelineView{},
			event: sessionstream.Event{Name: EventToolCallStarted, SessionId: "sid", Payload: &chatappv1.ChatToolCallStarted{
				MessageId:  "chat-msg-tool",
				ToolCallId: "call-sparse",
				Status:     "pending",
			}},
			check: func(t *testing.T, entity *chatappv1.ToolCallEntity) {
				t.Helper()
				require.Empty(t, entity.GetToolName())
				require.NotEqual(t, "tool", entity.GetToolName())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities, handled, err := plugin.ProjectTimeline(context.Background(), tt.event, nil, tt.view)
			require.NoError(t, err)
			require.True(t, handled)
			require.Len(t, entities, 1)
			require.Equal(t, TimelineEntityToolCall, entities[0].Kind)
			payload := entities[0].Payload.(*chatappv1.ToolCallEntity)
			if tt.check != nil {
				tt.check(t, payload)
			}
		})
	}
}

func requireToolProjectionFullCorrelation(t *testing.T, corr *chatappv1.CorrelationInfo) {
	t.Helper()
	require.NotNil(t, corr)
	require.Equal(t, "openai-responses", corr.GetProvider())
	require.Equal(t, "gpt-test", corr.GetModel())
	require.Equal(t, "resp_tool", corr.GetResponseId())
	require.Equal(t, "fc_1", corr.GetItemId())
	require.NotNil(t, corr.OutputIndex)
	require.Equal(t, int32(0), corr.GetOutputIndex())
	require.Equal(t, gepevents.StreamKindToolCall, corr.GetStreamKind())
	require.Equal(t, "call-sparse", corr.GetToolCallId())
	require.NotNil(t, corr.ToolCallIndex)
	require.Equal(t, int32(0), corr.GetToolCallIndex())
	require.Equal(t, "tool:call-sparse", corr.GetCorrelationKey())
	require.Equal(t, "provider-call-key", corr.GetParentCorrelationKey())
}

func TestToolCallPluginIgnoresUnrelatedEvents(t *testing.T) {
	plugin := NewToolCallPlugin()
	handled, err := plugin.HandleRuntimeEvent(context.Background(), chatapp.RuntimeEventContext{SessionID: "sid", MessageID: "chat-msg-1"}, gepevents.NewErrorEvent(gepevents.EventMetadata{SessionID: "sid"}, context.Canceled))
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
