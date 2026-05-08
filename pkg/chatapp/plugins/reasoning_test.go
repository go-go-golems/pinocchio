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

func TestReasoningPluginIgnoresUnrelatedEvents(t *testing.T) {
	plugin := NewReasoningPlugin()
	handled, err := plugin.HandleRuntimeEvent(context.Background(), chatapp.RuntimeEventContext{SessionID: "sid", MessageID: "chat-msg-1"}, gepevents.NewErrorEvent(gepevents.EventMetadata{SessionID: "sid"}, context.Canceled))
	require.NoError(t, err)
	require.False(t, handled)
}

type reasoningStaticTimelineView struct{}

func (reasoningStaticTimelineView) Get(string, string) (sessionstream.TimelineEntity, bool) {
	return sessionstream.TimelineEntity{}, false
}
func (reasoningStaticTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (reasoningStaticTimelineView) Ordinal() uint64                            { return 0 }

func int32Ptr(v int32) *int32 { return &v }
