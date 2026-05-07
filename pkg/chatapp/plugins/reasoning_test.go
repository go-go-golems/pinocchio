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
		payload := event.Payload.(*chatappv1.ReasoningUpdate)
		ids = append(ids, payload.GetMessageId())
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

func TestReasoningPluginCarriesProviderIDsOnReasoningUpdates(t *testing.T) {
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
	startHandled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(meta, "thinking-started", map[string]interface{}{
		"provider":     "openai_responses",
		"response_id":  "resp_1",
		"item_id":      "rs_1",
		"output_index": 0,
	}))
	require.NoError(t, err)
	require.True(t, startHandled)

	summaryStartedHandled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(meta, "reasoning-summary-started", map[string]interface{}{
		"provider":      "openai_responses",
		"response_id":   "resp_1",
		"item_id":       "rs_1",
		"output_index":  0,
		"summary_index": 0,
	}))
	require.NoError(t, err)
	require.False(t, summaryStartedHandled)

	deltaHandled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewThinkingPartialEvent(meta, "a", "alpha"))
	require.NoError(t, err)
	require.True(t, deltaHandled)

	summaryHandled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(meta, "reasoning-summary", map[string]interface{}{
		"text":          "alpha summary",
		"provider":      "openai_responses",
		"response_id":   "resp_1",
		"item_id":       "rs_1",
		"output_index":  0,
		"summary_index": 0,
	}))
	require.NoError(t, err)
	require.True(t, summaryHandled)

	require.Len(t, published, 3)
	for _, event := range published {
		payload := event.Payload.(*chatappv1.ReasoningUpdate)
		require.Equal(t, "openai_responses", payload.GetProvider())
		require.Equal(t, "resp_1", payload.GetResponseId())
		require.Equal(t, "rs_1", payload.GetItemId())
		require.Equal(t, int32(0), payload.GetOutputIndex())
		require.NotNil(t, payload.OutputIndex)
	}
	for _, event := range published[1:] {
		payload := event.Payload.(*chatappv1.ReasoningUpdate)
		require.Equal(t, int32(0), payload.GetSummaryIndex())
		require.NotNil(t, payload.SummaryIndex)
	}
}

func TestReasoningPluginSummaryUpdatesCompletedSegment(t *testing.T) {
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
		gepevents.NewInfoEvent(meta, "reasoning-summary", map[string]interface{}{"text": "alpha summary"}),
	}
	for _, event := range events {
		handled, err := plugin.HandleRuntimeEvent(context.Background(), runtime, event)
		require.NoError(t, err)
		require.True(t, handled)
	}

	require.Len(t, published, 4)
	for _, event := range published {
		payload := event.Payload.(*chatappv1.ReasoningUpdate)
		require.Equal(t, "chat-msg-1:thinking:1", payload.GetMessageId())
		require.Equal(t, int32(1), payload.GetSegment())
	}
	last := published[len(published)-1].Payload.(*chatappv1.ReasoningUpdate)
	require.Equal(t, "finished", last.GetStatus())
	require.False(t, last.GetStreaming())
	require.Equal(t, "summary", last.GetSource())
	require.Equal(t, "alpha summary", last.GetText())
}

func TestReasoningPluginRoutesInterleavedProviderItemsByMetadata(t *testing.T) {
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

	providerData := func(itemID string) map[string]interface{} {
		return map[string]interface{}{
			"provider":     "openai_responses",
			"response_id":  "resp_1",
			"item_id":      itemID,
			"output_index": 0,
		}
	}
	providerMeta := func(itemID string) gepevents.EventMetadata {
		return gepevents.EventMetadata{SessionID: "sid", Extra: providerData(itemID)}
	}

	_, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(providerMeta("rs_a"), "thinking-started", providerData("rs_a")))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(providerMeta("rs_b"), "thinking-started", providerData("rs_b")))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewThinkingPartialEvent(providerMeta("rs_a"), "a", "alpha"))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewThinkingPartialEvent(providerMeta("rs_b"), "b", "bravo"))
	require.NoError(t, err)

	require.Len(t, published, 4)
	payloadA := published[2].Payload.(*chatappv1.ReasoningUpdate)
	require.Equal(t, ReasoningDeltaEventName, published[2].Name)
	require.Equal(t, "chat-msg-1:thinking:1", payloadA.GetMessageId())
	require.Equal(t, int32(1), payloadA.GetSegment())
	require.Equal(t, "rs_a", payloadA.GetItemId())
	require.Equal(t, "alpha", payloadA.GetContent())

	payloadB := published[3].Payload.(*chatappv1.ReasoningUpdate)
	require.Equal(t, ReasoningDeltaEventName, published[3].Name)
	require.Equal(t, "chat-msg-1:thinking:2", payloadB.GetMessageId())
	require.Equal(t, int32(2), payloadB.GetSegment())
	require.Equal(t, "rs_b", payloadB.GetItemId())
	require.Equal(t, "bravo", payloadB.GetContent())
}

func TestReasoningPluginRoutesSummaryToCompletedProviderSegment(t *testing.T) {
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

	providerData := func(itemID string) map[string]interface{} {
		return map[string]interface{}{
			"provider":     "openai_responses",
			"response_id":  "resp_1",
			"item_id":      itemID,
			"output_index": 0,
		}
	}
	providerMeta := func(itemID string) gepevents.EventMetadata {
		return gepevents.EventMetadata{SessionID: "sid", Extra: providerData(itemID)}
	}

	_, err := plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(providerMeta("rs_a"), "thinking-started", providerData("rs_a")))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewThinkingPartialEvent(providerMeta("rs_a"), "a", "alpha"))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(providerMeta("rs_a"), "thinking-ended", providerData("rs_a")))
	require.NoError(t, err)
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(providerMeta("rs_b"), "thinking-started", providerData("rs_b")))
	require.NoError(t, err)

	summaryData := providerData("rs_a")
	summaryData["summary_index"] = 0
	summaryData["text"] = "alpha summary"
	_, err = plugin.HandleRuntimeEvent(context.Background(), runtime, gepevents.NewInfoEvent(providerMeta("rs_a"), "reasoning-summary", summaryData))
	require.NoError(t, err)

	require.Len(t, published, 5)
	summary := published[4].Payload.(*chatappv1.ReasoningUpdate)
	require.Equal(t, ReasoningFinishedEventName, published[4].Name)
	require.Equal(t, "chat-msg-1:thinking:1", summary.GetMessageId())
	require.Equal(t, int32(1), summary.GetSegment())
	require.Equal(t, "rs_a", summary.GetItemId())
	require.Equal(t, "summary", summary.GetSource())
	require.Equal(t, "alpha summary", summary.GetText())
	require.NotNil(t, summary.SummaryIndex)
	require.Equal(t, int32(0), summary.GetSummaryIndex())
}

func TestReasoningInt32FromAnyRejectsUnsignedOverflow(t *testing.T) {
	v, ok := int32FromAny(uint64(1 << 31))
	require.False(t, ok)
	require.Equal(t, int32(0), v)

	v, ok = int32FromAny(uint32(1<<31 - 1))
	require.True(t, ok)
	require.Equal(t, int32(1<<31-1), v)
}
