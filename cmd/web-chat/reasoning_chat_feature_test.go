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
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	sessionstreamv1 "github.com/go-go-golems/sessionstream/pkg/sessionstream/pb/proto/sessionstream/v1"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestReasoningChatFeatureHandleRuntimeEvent(t *testing.T) {
	feature := plugins.NewReasoningPlugin()
	var published []sessionstream.Event
	ctx := chatapp.RuntimeEventContext{
		SessionID: "sid",
		MessageID: "chat-msg-1",
		Publish: func(_ context.Context, eventName string, payload proto.Message) error {
			published = append(published, sessionstream.Event{Name: eventName, SessionId: "sid", Payload: payload})
			return nil
		},
	}

	handled, err := feature.HandleRuntimeEvent(context.Background(), ctx, gepevents.NewThinkingPartialEvent(gepevents.EventMetadata{SessionID: "sid"}, "why", "why because"))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 1)
	require.Equal(t, plugins.ReasoningDeltaEventName, published[0].Name)
	require.Equal(t, "chat-msg-1:thinking", published[0].Payload.(*structpb.Struct).AsMap()["messageId"])

	handled, err = feature.HandleRuntimeEvent(context.Background(), ctx, gepevents.NewInfoEvent(gepevents.EventMetadata{SessionID: "sid"}, "reasoning-summary", map[string]interface{}{"text": "short summary"}))
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, published, 2)
	require.Equal(t, plugins.ReasoningFinishedEventName, published[1].Name)
	require.Equal(t, "short summary", published[1].Payload.(*structpb.Struct).AsMap()["content"])
}

func TestReasoningChatFeatureProjectsUIAndTimeline(t *testing.T) {
	feature := plugins.NewReasoningPlugin()

	deltaPayload, err := structpb.NewStruct(map[string]any{
		"messageId":       "chat-msg-2:thinking",
		"parentMessageId": "chat-msg-2",
		"content":         "thinking out loud",
		"status":          "streaming",
		"streaming":       true,
	})
	require.NoError(t, err)

	uiEvents, handled, err := feature.ProjectUI(context.Background(), sessionstream.Event{Name: plugins.ReasoningDeltaEventName, SessionId: "sid", Ordinal: 7, Payload: deltaPayload}, nil, reasoningStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, uiEvents, 1)
	require.Equal(t, plugins.ReasoningAppendedUIName, uiEvents[0].Name)

	entities, handled, err := feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: plugins.ReasoningDeltaEventName, SessionId: "sid", Ordinal: 7, Payload: deltaPayload}, nil, reasoningStaticTimelineView{})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entityPayload := entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "thinking", entityPayload.GetRole())
	require.Equal(t, "thinking out loud", entityPayload.GetContent())
	require.Equal(t, true, entityPayload.GetStreaming())

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
			Payload: &chatappv1.ChatMessageEntity{
				MessageId: "chat-msg-2:thinking",
				Role:      "thinking",
				Content:   "kept content",
				Text:      "kept content",
				Streaming: true,
			},
		},
	}}

	entities, handled, err = feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: plugins.ReasoningFinishedEventName, SessionId: "sid", Ordinal: 8, Payload: finishedPayload}, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entityPayload = entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "kept content", entityPayload.GetContent())
	require.Equal(t, false, entityPayload.GetStreaming())

	summaryPayload, err := structpb.NewStruct(map[string]any{
		"messageId":       "chat-msg-2:thinking",
		"parentMessageId": "chat-msg-2",
		"content":         "summary wins",
		"source":          "summary",
		"status":          "finished",
		"streaming":       false,
	})
	require.NoError(t, err)
	entities, handled, err = feature.ProjectTimeline(context.Background(), sessionstream.Event{Name: plugins.ReasoningFinishedEventName, SessionId: "sid", Ordinal: 9, Payload: summaryPayload}, nil, view)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, entities, 1)
	entityPayload = entities[0].Payload.(*chatappv1.ChatMessageEntity)
	require.Equal(t, "summary wins", entityPayload.GetContent())
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
	hello := readReasoningServerFrame(t, conn)
	require.NotNil(t, hello.GetHello())

	writeReasoningClientFrame(t, conn, map[string]any{
		"subscribe": map[string]any{
			"sessionId":            "sess-reasoning-1",
			"sinceSnapshotOrdinal": "0",
		},
	})

	_ = readReasoningServerFrame(t, conn) // snapshot
	_ = readReasoningServerFrame(t, conn) // subscribed

	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-reasoning-1/messages", "application/json", bytes.NewReader([]byte(`{"prompt":"Show your reasoning summary"}`)))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	deadline := time.Now().Add(3 * time.Second)
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
					if streaming, ok := thinking["streaming"]; ok {
						require.Equal(t, false, streaming)
					}
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
		appserver.WithChatPlugins(plugins.NewReasoningPlugin()),
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

func readReasoningServerFrame(t *testing.T, conn *websocket.Conn) *sessionstreamv1.ServerFrame {
	t.Helper()
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	frame := &sessionstreamv1.ServerFrame{}
	require.NoError(t, protojson.Unmarshal(raw, frame))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	return frame
}

func writeReasoningClientFrame(t *testing.T, conn *websocket.Conn, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, body))
}
