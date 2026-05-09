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

func TestReasoningPluginPublishesCanonicalReasoningEvents(t *testing.T) {
	plugin := NewReasoningPlugin()
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
		SessionID:            "sid",
		Provider:             "openai_responses",
		ResponseID:           "resp_1",
		ItemID:               "rs_1",
		OutputIndex:          int32Ptr(0),
		SegmentID:            "reasoning-segment-1",
		SegmentIndex:         2,
		SegmentType:          gepevents.SegmentTypeReasoning,
		StreamKind:           gepevents.StreamKindReasoning,
		CorrelationKey:       "reasoning:rs_1",
		ParentCorrelationKey: "provider:1",
	}

	for _, event := range []gepevents.Event{
		gepevents.NewReasoningSegmentStartedEvent(meta, corr, "thinking"),
		gepevents.NewReasoningDeltaEvent(meta, corr, "draft", "draft plan", 1),
		gepevents.NewReasoningSegmentFinishedEvent(meta, corr, "draft plan", "stop"),
	} {
		handled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, event)
		require.NoError(t, err)
		require.True(t, handled)
	}

	require.Len(t, published, 3)
	require.Equal(t, chatapp.EventChatReasoningSegmentStarted, published[0].Name)
	started := published[0].Payload.(*chatappv1.ChatReasoningSegmentStarted)
	require.Equal(t, "chat-msg-1:thinking:2", started.GetMessageId())
	require.Equal(t, "chat-msg-1", started.GetParentMessageId())
	require.Equal(t, "openai_responses", started.GetCorrelation().GetProvider())
	require.Equal(t, "reasoning:rs_1", started.GetCorrelation().GetCorrelationKey())

	require.Equal(t, chatapp.EventChatReasoningDelta, published[1].Name)
	delta := published[1].Payload.(*chatappv1.ChatReasoningDelta)
	require.Equal(t, "draft", delta.GetChunk())
	require.Equal(t, "draft plan", delta.GetText())
	require.Equal(t, int32(2), delta.GetCorrelation().GetSegmentIndex())

	require.Equal(t, chatapp.EventChatReasoningSegmentFinished, published[2].Name)
	finished := published[2].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, "draft plan", finished.GetText())
	require.Equal(t, "stop", finished.GetFinishReason())
}

func TestReasoningPluginRoutesReasoningByStableSegmentIdentity(t *testing.T) {
	plugin := NewReasoningPlugin()
	var published []sessionstream.Event
	runtime := chatapp.RuntimeEventContext{
		SessionID: "sid",
		MessageID: "chat-msg-1",
		Publish: func(_ context.Context, eventName string, payload proto.Message) error {
			published = append(published, sessionstream.Event{Name: eventName, SessionId: "sid", Payload: payload})
			return nil
		},
	}

	baseCorr := gepevents.Correlation{
		SessionID:      "sid",
		Provider:       "openai_responses",
		ResponseID:     "resp_1",
		ItemID:         "rs_1",
		OutputIndex:    int32Ptr(0),
		SegmentIndex:   1,
		SegmentType:    gepevents.SegmentTypeReasoning,
		StreamKind:     gepevents.StreamKindReasoning,
		CorrelationKey: "reasoning:rs_1:full",
		ProviderCallID: "provider-call-1",
	}
	meta := gepevents.EventMetadata{SessionID: "sid"}
	_, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewReasoningDeltaEvent(meta, baseCorr, "draft", "draft", 1))
	require.NoError(t, err)

	summaryCorr := baseCorr
	summaryCorr.SummaryIndex = int32Ptr(0)
	summaryCorr.StreamKind = "reasoning-summary"
	summaryCorr.CorrelationKey = "reasoning:rs_1:summary:0"
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewReasoningSegmentFinishedEvent(meta, summaryCorr, "summary", "stop"))
	require.NoError(t, err)

	require.Len(t, published, 2)
	delta := published[0].Payload.(*chatappv1.ChatReasoningDelta)
	finished := published[1].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, delta.GetMessageId(), finished.GetMessageId())
	require.Equal(t, "chat-msg-1:thinking:1", finished.GetMessageId())
	require.Equal(t, "reasoning:rs_1:summary:0", finished.GetCorrelation().GetCorrelationKey())
	require.Equal(t, "reasoning-summary", finished.GetCorrelation().GetStreamKind())
}

func TestReasoningPluginProjectsCanonicalEventsToUIAndTimeline(t *testing.T) {
	plugin := NewReasoningPlugin()
	corr := &chatappv1.CorrelationInfo{
		Provider:       "openai_responses",
		ResponseId:     "resp_1",
		ItemId:         "rs_1",
		OutputIndex:    int32Ptr(0),
		SummaryIndex:   int32Ptr(0),
		SegmentIndex:   1,
		SegmentType:    gepevents.SegmentTypeReasoning,
		StreamKind:     gepevents.StreamKindReasoning,
		CorrelationKey: "reasoning:rs_1",
	}
	backend := sessionstream.Event{Name: ReasoningFinishedEventName, SessionId: "sid", Ordinal: 10, Payload: &chatappv1.ChatReasoningSegmentFinished{
		MessageId:       "chat-msg-1:thinking:1",
		ParentMessageId: "chat-msg-1",
		Role:            "thinking",
		Text:            "summary text",
		Content:         "summary text",
		Status:          "finished",
		Streaming:       false,
		Source:          "summary",
		Correlation:     corr,
	}}

	uiEvents, handled, err := plugin.ProjectUI(context.Background(), backend, nil, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, uiEvents, 1)
	require.Equal(t, ReasoningFinishedEventName, uiEvents[0].Name)
	uiPayload := uiEvents[0].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, "summary text", uiPayload.GetText())
	require.Equal(t, "openai_responses", uiPayload.GetCorrelation().GetProvider())
	require.Equal(t, int32(0), uiPayload.GetCorrelation().GetSummaryIndex())
	require.NotNil(t, uiPayload.GetCorrelation().SummaryIndex)
	require.Equal(t, "reasoning:rs_1", uiPayload.GetCorrelation().GetCorrelationKey())

	entities, handled, err := plugin.ProjectTimeline(context.Background(), backend, nil, reasoningStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entity := entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "chat-msg-1:thinking:1", entity.GetMessageId())
	require.Equal(t, "thinking", entity.GetRole())
	require.Equal(t, "summary text", entity.GetContent())
	require.Equal(t, "finished", entity.GetStatus())
	require.False(t, entity.GetStreaming())
	require.Equal(t, "reasoning:rs_1", entity.GetCorrelation().GetCorrelationKey())
}

func TestReasoningPluginSparseProjectionMatrix(t *testing.T) {
	plugin := NewReasoningPlugin()
	fullCorr := &chatappv1.CorrelationInfo{
		Provider:             "openai-responses",
		Model:                "gpt-test",
		ResponseId:           "resp_reason",
		ItemId:               "rs_1",
		OutputIndex:          int32Ptr(0),
		SummaryIndex:         int32Ptr(0),
		SegmentId:            "reasoning-segment-1",
		SegmentIndex:         1,
		SegmentType:          gepevents.SegmentTypeReasoning,
		StreamKind:           gepevents.StreamKindReasoning,
		CorrelationKey:       "reasoning:rs_1",
		ParentCorrelationKey: "provider-call-key",
	}
	segmentOnlyCorr := &chatappv1.CorrelationInfo{SegmentIndex: 1, SegmentType: gepevents.SegmentTypeReasoning}

	tests := []struct {
		name    string
		view    sessionstream.TimelineView
		event   sessionstream.Event
		wantNil bool
		check   func(t *testing.T, entity *chatappv1.ChatMessageEntity)
	}{
		{
			name: "REASONING-PROJECTION-01 sparse finish preserves content and correlation",
			view: reasoningTimelineViewWithMessage(&chatappv1.ChatMessageEntity{
				MessageId:       "chat-msg-1:thinking:1",
				ParentMessageId: "chat-msg-1",
				Role:            "thinking",
				Content:         "partial plan",
				Text:            "partial plan",
				Status:          "streaming",
				Streaming:       true,
				Correlation:     fullCorr,
			}),
			event: sessionstream.Event{Name: ReasoningFinishedEventName, SessionId: "sid", Payload: &chatappv1.ChatReasoningSegmentFinished{
				MessageId:   "chat-msg-1:thinking:1",
				Status:      "finished",
				Streaming:   false,
				Correlation: segmentOnlyCorr,
			}},
			check: func(t *testing.T, entity *chatappv1.ChatMessageEntity) {
				t.Helper()
				require.Equal(t, "partial plan", entity.GetContent())
				require.Equal(t, "chat-msg-1", entity.GetParentMessageId())
				require.Equal(t, "finished", entity.GetStatus())
				require.False(t, entity.GetStreaming())
				requireReasoningFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "REASONING-PROJECTION-02 sparse delta preserves provider identity while updating content",
			view: reasoningTimelineViewWithMessage(&chatappv1.ChatMessageEntity{
				MessageId:       "chat-msg-1:thinking:1",
				ParentMessageId: "chat-msg-1",
				Role:            "thinking",
				Content:         "partial",
				Text:            "partial",
				Status:          "streaming",
				Streaming:       true,
				Correlation:     fullCorr,
			}),
			event: sessionstream.Event{Name: ReasoningDeltaEventName, SessionId: "sid", Payload: &chatappv1.ChatReasoningDelta{
				MessageId:   "chat-msg-1:thinking:1",
				Content:     "partial plan",
				Text:        "partial plan",
				Status:      "streaming",
				Streaming:   true,
				Correlation: segmentOnlyCorr,
			}},
			check: func(t *testing.T, entity *chatappv1.ChatMessageEntity) {
				t.Helper()
				require.Equal(t, "partial plan", entity.GetContent())
				require.Equal(t, "chat-msg-1", entity.GetParentMessageId())
				requireReasoningFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "REASONING-PROJECTION-03 empty start without existing content creates no entity",
			view: reasoningStaticTimelineView{},
			event: sessionstream.Event{Name: ReasoningStartedEventName, SessionId: "sid", Payload: &chatappv1.ChatReasoningSegmentStarted{
				MessageId:       "chat-msg-1:thinking:1",
				ParentMessageId: "chat-msg-1",
				Status:          "streaming",
				Streaming:       true,
				Correlation:     segmentOnlyCorr,
			}},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities, handled, err := plugin.ProjectTimeline(context.Background(), tt.event, nil, tt.view)
			require.NoError(t, err)
			require.True(t, handled)
			if tt.wantNil {
				require.Nil(t, entities)
				return
			}
			require.Len(t, entities, 1)
			require.Equal(t, chatapp.TimelineEntityChatMessage, entities[0].Kind)
			payload := entities[0].Payload.(*chatappv1.ChatMessageEntity)
			if tt.check != nil {
				tt.check(t, payload)
			}
		})
	}
}

func requireReasoningFullCorrelation(t *testing.T, corr *chatappv1.CorrelationInfo) {
	t.Helper()
	require.NotNil(t, corr)
	require.Equal(t, "openai-responses", corr.GetProvider())
	require.Equal(t, "gpt-test", corr.GetModel())
	require.Equal(t, "resp_reason", corr.GetResponseId())
	require.Equal(t, "rs_1", corr.GetItemId())
	require.NotNil(t, corr.OutputIndex)
	require.Equal(t, int32(0), corr.GetOutputIndex())
	require.NotNil(t, corr.SummaryIndex)
	require.Equal(t, int32(0), corr.GetSummaryIndex())
	require.Equal(t, "reasoning-segment-1", corr.GetSegmentId())
	require.Equal(t, int32(1), corr.GetSegmentIndex())
	require.Equal(t, gepevents.SegmentTypeReasoning, corr.GetSegmentType())
	require.Equal(t, gepevents.StreamKindReasoning, corr.GetStreamKind())
	require.Equal(t, "reasoning:rs_1", corr.GetCorrelationKey())
	require.Equal(t, "provider-call-key", corr.GetParentCorrelationKey())
}

func TestReasoningPluginIgnoresUnrelatedEvents(t *testing.T) {
	plugin := NewReasoningPlugin()
	handled, err := plugin.HandleRuntimeEvent(context.Background(), chatapp.RuntimeEventContext{SessionID: "sid", MessageID: "chat-msg-1"}, gepevents.NewErrorEvent(gepevents.EventMetadata{SessionID: "sid"}, context.Canceled))
	require.NoError(t, err)
	require.False(t, handled)
}

type reasoningStaticTimelineView struct {
	entities map[string]sessionstream.TimelineEntity
}

func reasoningTimelineViewWithMessage(message *chatappv1.ChatMessageEntity) reasoningStaticTimelineView {
	return reasoningStaticTimelineView{entities: map[string]sessionstream.TimelineEntity{
		chatapp.TimelineEntityChatMessage + "/" + message.GetMessageId(): {
			Kind:    chatapp.TimelineEntityChatMessage,
			Id:      message.GetMessageId(),
			Payload: message,
		},
	}}
}

func (v reasoningStaticTimelineView) Get(kind, id string) (sessionstream.TimelineEntity, bool) {
	if v.entities == nil {
		return sessionstream.TimelineEntity{}, false
	}
	entity, ok := v.entities[kind+"/"+id]
	return entity, ok
}
func (reasoningStaticTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (reasoningStaticTimelineView) Ordinal() uint64                            { return 0 }

func int32Ptr(v int32) *int32 { return &v }
