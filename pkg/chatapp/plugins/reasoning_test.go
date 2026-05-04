package plugins

import (
	"context"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestReasoningPluginAllocatesDistinctThinkingSegments(t *testing.T) {
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
	events := []gepevents.Event{
		gepevents.NewInfoEvent(meta, "thinking-started", nil),
		gepevents.NewThinkingPartialEvent(meta, "a", "alpha"),
		gepevents.NewInfoEvent(meta, "thinking-ended", nil),
		gepevents.NewInfoEvent(meta, "thinking-started", nil),
		gepevents.NewThinkingPartialEvent(meta, "b", "beta"),
		gepevents.NewInfoEvent(meta, "thinking-ended", nil),
	}
	for _, event := range events {
		handled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, event)
		require.NoError(t, err)
		require.True(t, handled)
	}

	require.Len(t, published, 6)
	ids := make([]string, 0, len(published))
	for _, event := range published {
		payload := event.Payload.(*structpb.Struct).AsMap()
		ids = append(ids, payload["messageId"].(string))
	}
	require.Equal(t, []string{
		"chat-msg-1:thinking:1",
		"chat-msg-1:thinking:1",
		"chat-msg-1:thinking:1",
		"chat-msg-1:thinking:2",
		"chat-msg-1:thinking:2",
		"chat-msg-1:thinking:2",
	}, ids)
}
