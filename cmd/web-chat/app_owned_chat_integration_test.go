package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-go-golems/geppetto/pkg/events"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

type integrationNoopEngine struct{}

func (integrationNoopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type integrationNoopSink struct{}

func (integrationNoopSink) PublishEvent(events.Event) error { return nil }

func newAppOwnedIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()

	parsed := values.New()
	staticFS := fstest.MapFS{
		"static/index.html": {Data: []byte("<html><body>ok</body></html>")},
	}
	runtimeComposer := infruntime.RuntimeComposerFunc(func(_ context.Context, req infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
		runtimeKey := strings.TrimSpace(req.RuntimeKey)
		if runtimeKey == "" {
			runtimeKey = "default"
		}
		return infruntime.RuntimeArtifacts{
			Engine:             integrationNoopEngine{},
			Sink:               integrationNoopSink{},
			RuntimeKey:         runtimeKey,
			RuntimeFingerprint: "fp-" + runtimeKey,
			SeedSystemPrompt:   "seed",
		}, nil
	})

	webchatSrv, err := webchat.NewServer(context.Background(), parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
	require.NoError(t, err)

	profileRegistry, err := newInMemoryProfileRegistry(
		"default",
		&gepprofiles.Profile{Slug: gepprofiles.MustProfileSlug("default"), Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are default"}},
		&gepprofiles.Profile{
			Slug:    gepprofiles.MustProfileSlug("agent"),
			Runtime: gepprofiles.RuntimeSpec{SystemPrompt: "You are agent"},
			Policy:  gepprofiles.PolicySpec{AllowOverrides: true},
		},
	)
	require.NoError(t, err)
	requestResolver := newWebChatProfileResolver(profileRegistry, gepprofiles.MustRegistrySlug(defaultWebChatRegistrySlug))
	chatHandler := webhttp.NewChatHandler(webchatSrv.ChatService(), requestResolver)
	wsHandler := webhttp.NewWSHandler(
		webchatSrv.StreamHub(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	timelineLogger := log.With().Str("component", "webchat-test").Str("route", "/api/timeline").Logger()
	timelineHandler := webhttp.NewTimelineHandler(webchatSrv.TimelineService(), timelineLogger)
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	appMux.Handle("/api/", webchatSrv.APIHandler())
	appMux.Handle("/", webchatSrv.UIHandler())

	httpSrv := httptest.NewServer(appMux)
	return httpSrv
}

func TestAppOwnedChatHandler_Integration_DefaultProfilePath(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	reqBody := []byte(`{"prompt":"hello from integration test","conv_id":"conv-chat-1"}`)
	resp, err := http.Post(srv.URL+"/chat/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "started", payload["status"])
	require.Equal(t, "conv-chat-1", payload["conv_id"])
	require.NotEmpty(t, payload["session_id"])
	require.NotEmpty(t, payload["idempotency_key"])
}

func TestAppOwnedWSHandler_Integration_HelloAndPong(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?conv_id=conv-ws-1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, helloFrame, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "ws.hello", integrationSemEventType(helloFrame))

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("ping")))

	seenPong := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !seenPong {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
		_, frame, readErr := conn.ReadMessage()
		if readErr != nil {
			if ne, ok := readErr.(net.Error); ok && ne.Timeout() {
				continue
			}
			require.NoError(t, readErr)
		}
		if integrationSemEventType(frame) == "ws.pong" {
			seenPong = true
		}
	}
	require.True(t, seenPong, "expected ws.pong response to ping")
}

func TestAppOwnedChatHandler_Integration_UserMessageProjectedViaStream(t *testing.T) {
	srv := newAppOwnedIntegrationServer(t)
	defer srv.Close()

	convID := "conv-chat-proj-1"
	prompt := "hello from chat.message projection"
	reqBody := []byte(`{"prompt":"` + prompt + `","conv_id":"` + convID + `"}`)
	resp, err := http.Post(srv.URL+"/chat/default", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	turnID, _ := payload["turn_id"].(string)
	require.NotEmpty(t, turnID)
	expectedID := "user-" + turnID

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		timelineResp, err := http.Get(srv.URL + "/api/timeline?conv_id=" + convID)
		require.NoError(t, err)
		var snap map[string]any
		require.NoError(t, json.NewDecoder(timelineResp.Body).Decode(&snap))
		_ = timelineResp.Body.Close()

		entities, _ := snap["entities"].([]any)
		for _, raw := range entities {
			entity, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := entity["id"].(string)
			if id != expectedID {
				continue
			}
			props, _ := entity["props"].(map[string]any)
			if props == nil {
				continue
			}
			role, _ := props["role"].(string)
			content, _ := props["content"].(string)
			streaming, _ := props["streaming"].(bool)
			if role == "user" && content == prompt && !streaming {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(50 * time.Millisecond)
		}
	}
	require.True(t, found, "expected user timeline entity projected from chat.message stream event")
}

func integrationSemEventType(frame []byte) string {
	var env struct {
		Event struct {
			Type string `json:"type"`
		} `json:"event"`
	}
	if err := json.Unmarshal(frame, &env); err != nil {
		return ""
	}
	return env.Event.Type
}
