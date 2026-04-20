package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLabEnvironmentRunAndExportPhase1(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	resp, err := env.RunPhase1(context.Background(), phase1RunRequest{
		SessionID:   "lab-session-test",
		CommandName: phase1CommandName,
		Prompt:      "hello from systemlab",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["sessionExists"])
	require.True(t, resp.Checks["cursorAdvanced"])
	require.Len(t, resp.UIEvents, 4)

	filename, contentType, body, err := env.ExportPhase1("lab-session-test", "markdown")
	require.NoError(t, err)
	require.Equal(t, "phase1-transcript-lab-session-test.md", filename)
	require.Equal(t, "text/markdown; charset=utf-8", contentType)
	require.Contains(t, string(body), "# Phase 1 Transcript")
	require.Contains(t, string(body), "hello from systemlab")
}

func TestLabEnvironmentRunAndExportPhase2(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	resp, err := env.RunPhase2(context.Background(), phase2RunRequest{
		Action:     "publish-a",
		SessionA:   "s-a",
		SessionB:   "s-b",
		BurstCount: 3,
		StreamMode: "derived",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["publishOrdinalZero"])
	require.True(t, resp.Checks["monotonicPerSession"])
	require.Len(t, resp.MessageHistory, 1)
	require.NotEmpty(t, resp.PerSessionOrdinals["s-a"])

	resp, err = env.RunPhase2(context.Background(), phase2RunRequest{
		Action:     "burst-a",
		SessionA:   "s-a",
		SessionB:   "s-b",
		BurstCount: 3,
		StreamMode: "missing",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["monotonicPerSession"])
	require.Len(t, resp.PerSessionOrdinals["s-a"], 4)

	filename, contentType, body, err := env.ExportPhase2("markdown")
	require.NoError(t, err)
	require.Equal(t, "phase2-transcript.md", filename)
	require.Equal(t, "text/markdown; charset=utf-8", contentType)
	require.Contains(t, string(body), "# Phase 2 Transcript")
	require.Contains(t, string(body), "burst-a")
}

func TestLabEnvironmentRunPhase3(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	resp, err := env.RunPhase3(context.Background(), phase3RunRequest{
		Action:    "seed-session",
		SessionID: "reconnect-demo",
		Prompt:    "watch reconnect preserve a coherent snapshot",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["snapshotBeforeLive"])
	require.True(t, resp.Checks["sessionHydrated"])
	require.Equal(t, "4", resp.Snapshot["ordinal"])
}

func TestLabEnvironmentRunPhase4(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	_, err = env.RunPhase4(context.Background(), phase4RunRequest{
		Action:    "send",
		SessionID: "chat-demo",
		Prompt:    "Explain ordinals in plain language",
	})
	require.NoError(t, err)

	time.Sleep(80 * time.Millisecond)
	resp, err := env.RunPhase4(context.Background(), phase4RunRequest{
		Action:    "await-idle",
		SessionID: "chat-demo",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["hasChatEntity"])
	require.True(t, resp.Checks["timelineMatchesUI"])
}

func TestLabEnvironmentRunPhase5SQLRestart(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	_, err = env.RunPhase5(context.Background(), phase5RunRequest{
		Action:    "seed-session",
		Mode:      "sql",
		SessionID: "persist-demo",
		Text:      "persist this record",
	})
	require.NoError(t, err)

	resp, err := env.RunPhase5(context.Background(), phase5RunRequest{
		Action:    "restart-backend",
		Mode:      "sql",
		SessionID: "persist-demo",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["cursorPreserved"])
	require.True(t, resp.Checks["entitiesPreserved"])

	resp, err = env.RunPhase5(context.Background(), phase5RunRequest{
		Action:    "seed-session",
		Mode:      "sql",
		SessionID: "persist-demo",
		Text:      "persist this record again",
	})
	require.NoError(t, err)
	require.True(t, resp.Checks["resumeWithoutGaps"])
}

func TestLabEnvironmentRunPhase6(t *testing.T) {
	env, err := newLabEnvironment()
	require.NoError(t, err)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/chat/profiles":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"registry": "default", "slug": "gpt-5-nano-low"}})
		case r.Method == http.MethodPost && r.URL.Path == "/chat":
			http.NotFound(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/api/timeline":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat/sessions":
			_ = json.NewEncoder(w).Encode(map[string]any{"sessionId": "phase6-demo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat/sessions/phase6-demo/messages":
			_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/chat/sessions/phase6-demo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionId": "phase6-demo",
				"status":    "finished",
				"ordinal":   "7",
				"entities": []map[string]any{
					{
						"kind": "ChatMessage",
						"id":   "assistant-1",
						"payload": map[string]any{
							"role":    "assistant",
							"content": "Ordinals describe position in order.",
							"status":  "finished",
						},
					},
					{
						"kind": "ChatMessage",
						"id":   "user-1",
						"payload": map[string]any{
							"role":    "user",
							"content": "In one short sentence, explain ordinals.",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer testServer.Close()

	resp, err := env.RunPhase6(context.Background(), phase6RunRequest{
		Action:  "run",
		BaseURL: testServer.URL,
		Profile: "gpt-5-nano-low",
		Prompt:  "In one short sentence, explain ordinals.",
	})
	require.NoError(t, err)
	require.Equal(t, "phase6-demo", resp.SessionID)
	require.NotEmpty(t, resp.Trace)
	require.True(t, resp.Checks["profilesLoaded"])
	require.True(t, resp.Checks["targetProfilePresent"])
	require.True(t, resp.Checks["legacyRoutesRemoved"])
	require.True(t, resp.Checks["snapshotHasUser"])
	require.True(t, resp.Checks["snapshotHasAssistant"])
	require.True(t, resp.Checks["assistantCompleted"])
	require.True(t, resp.Checks["assistantGeneratedText"])
	require.True(t, resp.Checks["assistantIsNotEchoEngine"])
	require.Equal(t, http.StatusNotFound, resp.RouteStatuses["POST /chat"])
	require.Equal(t, http.StatusNotFound, resp.RouteStatuses["GET /api/timeline"])
	require.True(t, strings.Contains(toString(phase6MessageByRole(resp.Snapshot, "assistant")["content"]), "position") || strings.Contains(toString(phase6MessageByRole(resp.Snapshot, "assistant")["content"]), "order"))
}
