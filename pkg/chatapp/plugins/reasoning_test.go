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
		SessionID:      "sid",
		RunID:          "run-1",
		ProviderCallID: "provider:1",
		SegmentID:      "reasoning-segment-1",
	}

	for _, event := range []gepevents.Event{
		gepevents.NewReasoningSegmentStartedEvent(meta, corr, "thinking"),
		gepevents.NewReasoningDeltaEventWithSource(meta, corr, "summary", "draft", "draft plan", 1),
		gepevents.NewReasoningSegmentFinishedEventWithSource(meta, corr, "summary", "draft plan", "stop"),
	} {
		handled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, event)
		require.NoError(t, err)
		require.True(t, handled)
	}

	require.Len(t, published, 3)
	require.Equal(t, chatapp.EventChatReasoningSegmentStarted, published[0].Name)
	started := published[0].Payload.(*chatappv1.ChatReasoningSegmentStarted)
	require.Equal(t, "chat-msg-1:thinking:reasoning-segment-1", started.GetMessageId())
	require.Equal(t, "chat-msg-1", started.GetParentMessageId())
	require.Equal(t, "reasoning-segment-1", started.GetCorrelation().GetSegmentId())

	require.Equal(t, chatapp.EventChatReasoningDelta, published[1].Name)
	delta := published[1].Payload.(*chatappv1.ChatReasoningDelta)
	require.Equal(t, "draft", delta.GetChunk())
	require.Equal(t, "draft plan", delta.GetText())
	require.Equal(t, "reasoning-segment-1", delta.GetCorrelation().GetSegmentId())
	require.Equal(t, "summary", delta.GetSource())

	require.Equal(t, chatapp.EventChatReasoningSegmentFinished, published[2].Name)
	finished := published[2].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, "draft plan", finished.GetText())
	require.Equal(t, "stop", finished.GetFinishReason())
	require.Equal(t, "summary", finished.GetSource())
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

	corr := gepevents.Correlation{SessionID: "sid", RunID: "run-1", ProviderCallID: "provider-call-1", SegmentID: "reasoning-rs-1"}
	meta := gepevents.EventMetadata{SessionID: "sid"}
	_, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewReasoningDeltaEvent(meta, corr, "draft", "draft", 1))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewReasoningSegmentFinishedEvent(meta, corr, "summary", "stop"))
	require.NoError(t, err)

	require.Len(t, published, 2)
	delta := published[0].Payload.(*chatappv1.ChatReasoningDelta)
	finished := published[1].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, delta.GetMessageId(), finished.GetMessageId())
	require.Equal(t, "chat-msg-1:thinking:reasoning-rs-1", finished.GetMessageId())
	require.Equal(t, "reasoning-rs-1", finished.GetCorrelation().GetSegmentId())
}

func TestReasoningPluginUsesCorrelationIdentityWhenSegmentIndexIsAbsent(t *testing.T) {
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

	first := gepevents.Correlation{SessionID: "sid", RunID: "run-1", ProviderCallID: "provider-1", SegmentID: "openai-chat:resp-1:choice:0:reasoning"}
	second := gepevents.Correlation{SessionID: "sid", RunID: "run-1", ProviderCallID: "provider-2", SegmentID: "openai-chat:resp-2:choice:0:reasoning"}

	for _, corr := range []gepevents.Correlation{first, second} {
		_, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewReasoningSegmentStartedEvent(meta, corr, "thinking"))
		require.NoError(t, err)
		_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewReasoningSegmentFinishedEvent(meta, corr, "done", "stop"))
		require.NoError(t, err)
	}

	require.Len(t, published, 4)
	firstStarted := published[0].Payload.(*chatappv1.ChatReasoningSegmentStarted)
	firstFinished := published[1].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	secondStarted := published[2].Payload.(*chatappv1.ChatReasoningSegmentStarted)
	secondFinished := published[3].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, "chat-msg-1:thinking:openai-chat:resp-1:choice:0:reasoning", firstStarted.GetMessageId())
	require.Equal(t, firstStarted.GetMessageId(), firstFinished.GetMessageId())
	require.Equal(t, "chat-msg-1:thinking:openai-chat:resp-2:choice:0:reasoning", secondStarted.GetMessageId())
	require.Equal(t, secondStarted.GetMessageId(), secondFinished.GetMessageId())
	require.NotEqual(t, firstStarted.GetMessageId(), secondStarted.GetMessageId())
}

func TestReasoningPluginProjectsCanonicalEventsToUIAndTimeline(t *testing.T) {
	plugin := NewReasoningPlugin()
	corr := &chatappv1.CorrelationInfo{SessionId: "sid", RunId: "run-1", ProviderCallId: "provider-call-1", SegmentId: "reasoning-segment-1"}
	backend := sessionstream.Event{Name: ReasoningFinishedEventName, SessionId: "sid", Ordinal: 10, Payload: &chatappv1.ChatReasoningSegmentFinished{
		MessageId:       "chat-msg-1:thinking:reasoning-segment-1",
		ParentMessageId: "chat-msg-1",
		Role:            "thinking",
		Text:            "summary text",
		Content:         "summary text",
		Status:          "finished",
		Streaming:       false,
		Source:          "thinking",
		Correlation:     corr,
	}}

	uiEvents, handled, err := plugin.ProjectUI(context.Background(), backend, nil, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, uiEvents, 1)
	require.Equal(t, ReasoningFinishedEventName, uiEvents[0].Name)
	uiPayload := uiEvents[0].Payload.(*chatappv1.ChatReasoningSegmentFinished)
	require.Equal(t, "summary text", uiPayload.GetText())
	require.Equal(t, "reasoning-segment-1", uiPayload.GetCorrelation().GetSegmentId())

	entities, handled, err := plugin.ProjectTimeline(context.Background(), backend, nil, reasoningStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entity := entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "chat-msg-1:thinking:reasoning-segment-1", entity.GetMessageId())
	require.Equal(t, "thinking", entity.GetRole())
	require.Equal(t, "summary text", entity.GetContent())
	require.Equal(t, "finished", entity.GetStatus())
	require.False(t, entity.GetStreaming())
	require.Equal(t, "reasoning-segment-1", entity.GetCorrelation().GetSegmentId())
}

func TestReasoningPluginSparseProjectionMatrix(t *testing.T) {
	plugin := NewReasoningPlugin()
	fullCorr := &chatappv1.CorrelationInfo{SessionId: "sid", RunId: "run-1", ProviderCallId: "provider-call-key", SegmentId: "reasoning-segment-1"}
	segmentOnlyCorr := &chatappv1.CorrelationInfo{SegmentId: "reasoning-segment-1"}

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
				MessageId:       "chat-msg-1:thinking:reasoning-segment-1",
				ParentMessageId: "chat-msg-1",
				Role:            "thinking",
				Content:         "partial plan",
				Text:            "partial plan",
				Status:          "streaming",
				Streaming:       true,
				Correlation:     fullCorr,
			}),
			event: sessionstream.Event{Name: ReasoningFinishedEventName, SessionId: "sid", Payload: &chatappv1.ChatReasoningSegmentFinished{
				MessageId:   "chat-msg-1:thinking:reasoning-segment-1",
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
				MessageId:       "chat-msg-1:thinking:reasoning-segment-1",
				ParentMessageId: "chat-msg-1",
				Role:            "thinking",
				Content:         "partial",
				Text:            "partial",
				Status:          "streaming",
				Streaming:       true,
				Correlation:     fullCorr,
			}),
			event: sessionstream.Event{Name: ReasoningDeltaEventName, SessionId: "sid", Payload: &chatappv1.ChatReasoningDelta{
				MessageId:   "chat-msg-1:thinking:reasoning-segment-1",
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
				MessageId:       "chat-msg-1:thinking:reasoning-segment-1",
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
	require.Equal(t, "sid", corr.GetSessionId())
	require.Equal(t, "run-1", corr.GetRunId())
	require.Equal(t, "provider-call-key", corr.GetProviderCallId())
	require.Equal(t, "reasoning-segment-1", corr.GetSegmentId())
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
