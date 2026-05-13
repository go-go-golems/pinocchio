package chatapp

import (
	"context"
	"testing"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestBaseTimelineProjectionSparseTextMatrix(t *testing.T) {
	fullCorr := projectionFullTextCorrelation()
	segmentOnlyCorr := &chatappv1.CorrelationInfo{SegmentId: "segment-1"}

	tests := []struct {
		name    string
		view    sessionstream.TimelineView
		event   sessionstream.Event
		wantNil bool
		check   func(t *testing.T, entity *chatappv1.ChatMessageEntity)
	}{
		{
			name: "PROJECTION-01 sparse finished preserves existing content and correlation",
			view: projectionTimelineViewWithMessage(&chatappv1.ChatMessageEntity{
				MessageId:   "message-1:text:segment-1",
				Role:        "assistant",
				Content:     "partial answer",
				Text:        "partial answer",
				Status:      "streaming",
				Streaming:   true,
				Correlation: fullCorr,
			}),
			event: sessionstream.Event{Name: EventChatTextSegmentFinished, SessionId: "session-1", Payload: &chatappv1.ChatTextSegmentFinished{
				MessageId: "message-1:text:segment-1",
				Status:    "failed",
				Streaming: false,
				Final:     true,
			}},
			check: func(t *testing.T, entity *chatappv1.ChatMessageEntity) {
				t.Helper()
				require.Equal(t, "partial answer", entity.GetContent())
				require.Equal(t, "failed", entity.GetStatus())
				require.True(t, entity.GetFinal())
				requireProjectionFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "PROJECTION-02 sparse finished correlation merges instead of clearing provider identity",
			view: projectionTimelineViewWithMessage(&chatappv1.ChatMessageEntity{
				MessageId:   "message-1:text:segment-1",
				Role:        "assistant",
				Content:     "partial answer",
				Text:        "partial answer",
				Status:      "streaming",
				Streaming:   true,
				Correlation: fullCorr,
			}),
			event: sessionstream.Event{Name: EventChatTextSegmentFinished, SessionId: "session-1", Payload: &chatappv1.ChatTextSegmentFinished{
				MessageId:   "message-1:text:segment-1",
				Status:      "finished",
				Streaming:   false,
				Final:       true,
				Correlation: segmentOnlyCorr,
			}},
			check: func(t *testing.T, entity *chatappv1.ChatMessageEntity) {
				t.Helper()
				require.Equal(t, "partial answer", entity.GetContent())
				requireProjectionFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "PROJECTION-03 sparse delta correlation merges while updating content",
			view: projectionTimelineViewWithMessage(&chatappv1.ChatMessageEntity{
				MessageId:   "message-1:text:segment-1",
				Role:        "assistant",
				Content:     "partial",
				Text:        "partial",
				Status:      "streaming",
				Streaming:   true,
				Correlation: fullCorr,
			}),
			event: sessionstream.Event{Name: EventChatTextPatch, SessionId: "session-1", Payload: &chatappv1.ChatTextPatch{
				MessageId:   "message-1:text:segment-1",
				Text:        " answer",
				Mode:        chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND,
				Status:      "streaming",
				Correlation: segmentOnlyCorr,
			}},
			check: func(t *testing.T, entity *chatappv1.ChatMessageEntity) {
				t.Helper()
				require.Equal(t, "partial answer", entity.GetContent())
				require.Equal(t, "streaming", entity.GetStatus())
				requireProjectionFullCorrelation(t, entity.GetCorrelation())
			},
		},
		{
			name: "PROJECTION-04 empty started without existing content still creates no placeholder",
			view: projectionTimelineView{},
			event: sessionstream.Event{Name: EventChatTextSegmentStarted, SessionId: "session-1", Payload: &chatappv1.ChatTextSegmentStarted{
				MessageId:   "message-1:text:segment-1",
				Status:      "streaming",
				Streaming:   true,
				Correlation: segmentOnlyCorr,
			}},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities, err := baseTimelineProjection(context.Background(), tt.event, nil, tt.view)
			require.NoError(t, err)
			if tt.wantNil {
				require.Nil(t, entities)
				return
			}
			require.Len(t, entities, 1)
			require.Equal(t, TimelineEntityChatMessage, entities[0].Kind)
			payload := entities[0].Payload.(*chatappv1.ChatMessageEntity)
			if tt.check != nil {
				tt.check(t, payload)
			}
		})
	}
}

type projectionTimelineView struct {
	entities map[string]sessionstream.TimelineEntity
}

func projectionTimelineViewWithMessage(message *chatappv1.ChatMessageEntity) projectionTimelineView {
	return projectionTimelineView{entities: map[string]sessionstream.TimelineEntity{
		TimelineEntityChatMessage + "/" + message.GetMessageId(): {
			Kind:    TimelineEntityChatMessage,
			Id:      message.GetMessageId(),
			Payload: message,
		},
	}}
}

func (v projectionTimelineView) Get(kind, id string) (sessionstream.TimelineEntity, bool) {
	if v.entities == nil {
		return sessionstream.TimelineEntity{}, false
	}
	entity, ok := v.entities[kind+"/"+id]
	return entity, ok
}

func (v projectionTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (v projectionTimelineView) Ordinal() uint64                            { return 0 }

func projectionFullTextCorrelation() *chatappv1.CorrelationInfo {
	return &chatappv1.CorrelationInfo{
		SessionId:      "session-1",
		RunId:          "message-1",
		TurnId:         "turn-1",
		ProviderCallId: "provider-call-1",
		SegmentId:      "segment-1",
	}
}

func requireProjectionFullCorrelation(t *testing.T, corr *chatappv1.CorrelationInfo) {
	t.Helper()
	require.NotNil(t, corr)
	require.Equal(t, "session-1", corr.GetSessionId())
	require.Equal(t, "message-1", corr.GetRunId())
	require.Equal(t, "turn-1", corr.GetTurnId())
	require.Equal(t, "provider-call-1", corr.GetProviderCallId())
	require.Equal(t, "segment-1", corr.GetSegmentId())
}
