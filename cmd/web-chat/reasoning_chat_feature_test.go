package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	appserver "github.com/go-go-golems/pinocchio/cmd/web-chat/app"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestReasoningChatFeatureHandleRuntimeEvent(t *testing.T) {
	feature := newReasoningChatFeature()
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

	handled, err := feature.HandleRuntimeEvent(context.Background(), ctx, gepevents.NewThinkingPartialEvent(gepevents.EventMetadata{SessionID: "sid"}, "why", "why because"))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 1)
	require.Equal(t, reasoningDeltaEventName, published[0].Name)
	require.Equal(t, "chat-msg-1:thinking", published[0].Payload.(*structpb.Struct).AsMap()["messageId"])

	handled, err = feature.HandleRuntimeEvent(context.Background(), ctx, gepevents.NewInfoEvent(gepevents.EventMetadata{SessionID: "sid"}, "reasoning-summary", map[string]interface{}{"text": "short summary"}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 2)
	require.Equal(t, reasoningFinishedEventName, published[1].Name)
	require.Equal(t, "short summary", published[1].Payload.(*structpb.Struct).AsMap()["content"])
}

func TestReasoningChatFeatureProjectsUIAndTimeline(t *testing.T) {
	feature := newReasoningChatFeature()

	deltaPayload, err := structpb.NewStruct(map[string]any{
		"messageId":       "chat-msg-2:thinking",
		"parentMessageId": "chat-msg-2",
		"content":         "thinking out loud",
		"status":          "streaming",
		"streaming":       true,
	})
	require.NoError(t, err)

	uiEvents, handled, err := feature.ProjectUI(context.Background(), sessionstream.Event{Name: reasoningDeltaEventName, SessionId: "sid", Ordinal: 7, Payload: deltaPayload}, nil, reasoningStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, uiEvents, 1)
	require.Equal(t, reasoningAppendedUIName, uiEvents[0].Name)

	entities, handled, err := feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: reasoningDeltaEventName, SessionId: "sid", Ordinal: 7, Payload: deltaPayload}, nil, reasoningStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entityPayload := entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "thinking", entityPayload["role"])
	require.Equal(t, "thinking out loud", entityPayload["content"])
	require.Equal(t, true, entityPayload["streaming"])

	finishedPayload, err := structpb.NewStruct(map[string]any{
		"messageId":       "chat-msg-2:thinking",
		"parentMessageId": "chat-msg-2",
		"status":          "finished",
		"streaming":       false,
	})
	require.NoError(t, err)

	view := reasoningStaticTimelineView{entities: map[string]sessionstream.TimelineEntity{
		"ChatMessage/chat-msg-2:thinking": {
			Kind: chatapp.TimelineEntityChatMessage,
			Id:   "chat-msg-2:thinking",
			Payload: mustStruct(t, map[string]any{
				"messageId": "chat-msg-2:thinking",
				"role":      "thinking",
				"content":   "kept content",
				"text":      "kept content",
				"streaming": true,
			}),
		},
	}}

	entities, handled, err = feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: reasoningFinishedEventName, SessionId: "sid", Ordinal: 8, Payload: finishedPayload}, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entityPayload = entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "kept content", entityPayload["content"])
	require.Equal(t, false, entityPayload["streaming"])

	summaryPayload, err := structpb.NewStruct(map[string]any{
		"messageId":       "chat-msg-2:thinking",
		"parentMessageId": "chat-msg-2",
		"content":         "summary wins",
		"source":          "summary",
		"status":          "finished",
		"streaming":       false,
	})
	require.NoError(t, err)
	entities, handled, err = feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: reasoningFinishedEventName, SessionId: "sid", Ordinal: 9, Payload: summaryPayload}, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entityPayload = entities[0].Payload.(*structpb.Struct).AsMap()
	require.Equal(t, "summary wins", entityPayload["content"])
	require.Equal(t, "summary", entityPayload["source"])
}

func TestReasoningChatFeatureServerSnapshotAndUIEvents(t *testing.T) {
	srv, httpSrv := newReasoningTestMux(t)
	defer func() {
		httpSrv.Close()
		_ = srv.Close()
	}()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var hello map[string]any
	require.NoError(t, json.Unmarshal(raw, &hello))
	require.Equal(t, "hello", hello["type"])

	require.NoError(t, conn.WriteJSON(map[string]any{
		"type":         "subscribe",
		"sessionId":    "sess-reasoning-1",
		"sinceOrdinal": "0",
	}))

	_, _, err = conn.ReadMessage() // snapshot
	require.NoError(t, err)
	_, _, err = conn.ReadMessage() // subscribed
	require.NoError(t, err)

	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-reasoning-1/messages", "application/json", bytes.NewReader([]byte(`{"prompt":"Show your reasoning summary"}`)))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	seenReasoningAppend := false
	seenReasoningFinish := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, raw, err = conn.ReadMessage()
		require.NoError(t, err)
		var frame map[string]any
		require.NoError(t, json.Unmarshal(raw, &frame))
		if frame["type"] != "ui-event" {
			continue
		}
		if frame["name"] == reasoningAppendedUIName {
			seenReasoningAppend = true
		}
		if frame["name"] == reasoningFinishedUIName {
			seenReasoningFinish = true
		}
		if seenReasoningAppend && seenReasoningFinish {
			break
		}
	}
	require.True(t, seenReasoningAppend)
	require.True(t, seenReasoningFinish)

	deadline = time.Now().Add(3 * time.Second)
	for {
		snapResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-reasoning-1")
		require.NoError(t, err)
		var snap appserver.SessionSnapshotResponse
		require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
		_ = snapResp.Body.Close()
		roles := map[string]map[string]any{}
		for _, entity := range snap.Entities {
			payload, ok := entity.Payload.(map[string]any)
			require.True(t, ok)
			if role, ok := payload["role"].(string); ok {
				roles[role] = payload
			}
		}
		if _, ok := roles["user"]; ok {
			if thinking, ok := roles["thinking"]; ok {
				if _, ok := roles["assistant"]; ok {
					require.Equal(t, "high level plan", thinking["content"])
					require.Equal(t, false, thinking["streaming"])
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for reasoning snapshot; last status=%q", snap.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type reasoningRuntimeTestEngine struct{}

func (reasoningRuntimeTestEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	meta := gepevents.EventMetadata{}
	gepevents.PublishEventToContext(ctx, gepevents.NewStartEvent(meta))
	gepevents.PublishEventToContext(ctx, gepevents.NewInfoEvent(meta, "thinking-started", nil))
	gepevents.PublishEventToContext(ctx, gepevents.NewThinkingPartialEvent(meta, "draft", "draft plan"))
	gepevents.PublishEventToContext(ctx, gepevents.NewInfoEvent(meta, "reasoning-summary", map[string]interface{}{"text": "high level plan"}))
	gepevents.PublishEventToContext(ctx, gepevents.NewInfoEvent(meta, "thinking-ended", nil))
	gepevents.PublishEventToContext(ctx, gepevents.NewPartialCompletionEvent(meta, "Answer: ready", "Answer: ready"))
	gepevents.PublishEventToContext(ctx, gepevents.NewFinalEvent(meta, "Answer: ready"))
	return t, nil
}

type reasoningRuntimeResolver struct{}

func (reasoningRuntimeResolver) Resolve(context.Context, *http.Request, string, string) (*infruntime.ComposedRuntime, error) {
	return &infruntime.ComposedRuntime{Engine: reasoningRuntimeTestEngine{}}, nil
}

func newReasoningTestMux(t *testing.T) (*appserver.Server, *httptest.Server) {
	t.Helper()
	srv, err := appserver.NewServer(
		appserver.WithDefaultProfile("gpt-5-low"),
		appserver.WithChunkDelay(time.Millisecond),
		appserver.WithRuntimeResolver(reasoningRuntimeResolver{}),
		appserver.WithChatFeatureSets(newReasoningChatFeature()),
	)
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat/sessions", srv.HandleCreateSession)
	mux.HandleFunc("/api/chat/sessions/", srv.HandleSessionRoutes)
	mux.HandleFunc("/api/chat/ws", srv.HandleWS)

	httpSrv := httptest.NewServer(mux)
	return srv, httpSrv
}

type reasoningStaticTimelineView struct {
	entities map[string]sessionstream.TimelineEntity
}

func (v reasoningStaticTimelineView) Get(kind, id string) (sessionstream.TimelineEntity, bool) {
	if v.entities == nil {
		return sessionstream.TimelineEntity{}, false
	}
	entity, ok := v.entities[kind+"/"+id]
	return entity, ok
}

func (v reasoningStaticTimelineView) List(string) []sessionstream.TimelineEntity { return nil }
func (v reasoningStaticTimelineView) Ordinal() uint64                            { return 0 }

func mustStruct(t *testing.T, payload map[string]any) *structpb.Struct {
	t.Helper()
	pb, err := structpb.NewStruct(payload)
	require.NoError(t, err)
	return pb
}
