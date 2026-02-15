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
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/gorilla/websocket"
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
	runtimeComposer := webchat.RuntimeComposerFunc(func(_ context.Context, req webchat.RuntimeComposeRequest) (webchat.RuntimeArtifacts, error) {
		runtimeKey := strings.TrimSpace(req.RuntimeKey)
		if runtimeKey == "" {
			runtimeKey = "default"
		}
		return webchat.RuntimeArtifacts{
			Engine:             integrationNoopEngine{},
			Sink:               integrationNoopSink{},
			RuntimeKey:         runtimeKey,
			RuntimeFingerprint: "fp-" + runtimeKey,
			SeedSystemPrompt:   "seed",
		}, nil
	})

	r, err := webchat.NewRouter(context.Background(), parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
	require.NoError(t, err)

	profiles := newChatProfileRegistry(
		"default",
		&chatProfile{Slug: "default", DefaultPrompt: "You are default"},
		&chatProfile{Slug: "agent", DefaultPrompt: "You are agent", AllowOverrides: true},
	)
	requestResolver := newWebChatProfileResolver(profiles)
	chatHandler := webchat.NewChatHandler(r.ConversationService(), requestResolver)
	wsHandler := webchat.NewWSHandler(
		r.ConversationService(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	appMux.Handle("/api/", r.APIHandler())
	appMux.Handle("/", r.UIHandler())

	srv := httptest.NewServer(appMux)
	return srv
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
