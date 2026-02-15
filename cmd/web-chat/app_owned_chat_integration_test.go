package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/stretchr/testify/require"
)

type integrationNoopEngine struct{}

func (integrationNoopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type integrationNoopSink struct{}

func (integrationNoopSink) PublishEvent(events.Event) error { return nil }

func TestAppOwnedChatHandler_Integration_DefaultProfilePath(t *testing.T) {
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

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.Handle("/api/", r.APIHandler())
	appMux.Handle("/", r.UIHandler())

	srv := httptest.NewServer(appMux)
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
