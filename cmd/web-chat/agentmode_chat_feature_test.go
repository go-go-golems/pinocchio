package main

import (
	"context"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestAgentModeChatFeatureHandleRuntimeEvent(t *testing.T) {
	feature := newAgentModePlugin()
	var published []sessionstream.Event
	ctx := chatapp.RuntimeEventContext{
		SessionID: "sid",
		MessageID: "chat-msg-1",
		Publish: func(_ context.Context, eventName string, payload map[string]any) error {
			pb, err := structpb.NewStruct(payload)
			require.NoError(t, err)
			published = append(published, sessionstream.Event{Name: eventName, SessionId: "sid", Payload: pb})
			return nil
		},
	}

	handled, err := feature.HandleRuntimeEvent(context.Background(), ctx, agentmode.NewModeSwitchPreviewEvent(gepevents.EventMetadata{SessionID: "sid"}, "item-1", "reviewer", "hello", "candidate"))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 1)
	require.Equal(t, agentModePreviewEventName, published[0].Name)

	handled, err = feature.HandleRuntimeEvent(context.Background(), ctx, gepevents.NewAgentModeSwitchEvent(gepevents.EventMetadata{SessionID: "sid"}, "analyst", "reviewer", "hello"))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 2)
	require.Equal(t, agentModeCommittedEventName, published[1].Name)
}

func TestAgentModeChatFeatureProjectsUIAndTimeline(t *testing.T) {
	feature := newAgentModePlugin()
	previewPayload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-2", "candidateMode": "reviewer", "analysis": "hello"})
	require.NoError(t, err)
	previewEvents, handled, err := feature.ProjectUI(context.Background(), sessionstream.Event{Name: agentModePreviewEventName, SessionId: "sid", Ordinal: 7, Payload: previewPayload}, nil, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, previewEvents, 1)
	require.Equal(t, agentModePreviewUIName, previewEvents[0].Name)

	commitPayload, err := structpb.NewStruct(map[string]any{"messageId": "chat-msg-3", "title": "agentmode: mode switched", "from": "analyst", "to": "reviewer", "analysis": "hello"})
	require.NoError(t, err)
	commitEvents, handled, err := feature.ProjectUI(context.Background(), sessionstream.Event{Name: agentModeCommittedEventName, SessionId: "sid", Ordinal: 8, Payload: commitPayload}, nil, nil)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, commitEvents, 2)
	require.Equal(t, agentModeCommittedUIName, commitEvents[0].Name)
	require.Equal(t, agentModePreviewClearUIName, commitEvents[1].Name)

	entities, handled, err := feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: agentModeCommittedEventName, SessionId: "sid", Ordinal: 9, Payload: commitPayload}, nil, agentmodeStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	require.Equal(t, agentModeTimelineEntityKind, entities[0].Kind)
	entityPayload := entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "agentmode: mode switched", entityPayload["title"])
}

type agentmodeStaticTimelineView struct{}

func (agentmodeStaticTimelineView) Get(string, string) (sessionstream.TimelineEntity, bool) {
	return sessionstream.TimelineEntity{}, false
}

func (agentmodeStaticTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (agentmodeStaticTimelineView) Ordinal() uint64                            { return 0 }
