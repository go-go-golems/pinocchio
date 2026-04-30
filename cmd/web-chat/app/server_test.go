package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type runtimeBackedTestEngine struct {
	completion string
}

func (e runtimeBackedTestEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	completion := strings.TrimSpace(e.completion)
	if completion == "" {
		completion = "runtime-backed response"
	}
	meta := gepevents.EventMetadata{}
	gepevents.PublishEventToContext(ctx, gepevents.NewStartEvent(meta))
	gepevents.PublishEventToContext(ctx, gepevents.NewPartialCompletionEvent(meta, completion, completion))
	return t, nil
}

type staticRuntimeResolver struct {
	completion string
}

func (r staticRuntimeResolver) Resolve(context.Context, *http.Request, string, string) (*infruntime.ComposedRuntime, error) {
	return &infruntime.ComposedRuntime{Engine: runtimeBackedTestEngine(r)}, nil
}

func newTestMux(t *testing.T, opts ...Option) (*Server, *httptest.Server) {
	t.Helper()
	baseOpts := []Option{WithDefaultProfile("gpt-5-nano-low"), WithChunkDelay(time.Millisecond)}
	baseOpts = append(baseOpts, opts...)
	srv, err := NewServer(baseOpts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat/sessions", srv.HandleCreateSession)
	mux.HandleFunc("/api/chat/sessions/", srv.HandleSessionRoutes)
	mux.HandleFunc("/api/chat/ws", srv.HandleWS)

	httpSrv := httptest.NewServer(mux)
	t.Cleanup(httpSrv.Close)
	return srv, httpSrv
}

func TestSnapshotStatusDoesNotFinishBeforeAssistant(t *testing.T) {
	entities := []SnapshotEntity{
		{Payload: map[string]any{"role": "user", "status": ""}},
		{Payload: map[string]any{"role": "thinking", "status": "finished"}},
	}
	require.Equal(t, "streaming", snapshotStatus(entities))

	entities = append(entities, SnapshotEntity{Payload: map[string]any{"role": "assistant", "status": "finished"}})
	require.Equal(t, "finished", snapshotStatus(entities))
}

func TestCreateSession(t *testing.T) {
	_, httpSrv := newTestMux(t)

	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions", "application/json", strings.NewReader(`{"profile":"gpt-5-nano-low"}`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out CreateSessionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.SessionID)
	require.Equal(t, "gpt-5-nano-low", out.Profile)
}

func TestSubmitAndSnapshot(t *testing.T) {
	_, httpSrv := newTestMux(t)

	body := []byte(`{"prompt":"Explain ordinals in plain language","profile":"gpt-5-nano-low"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var submit SubmitMessageResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&submit))
	require.Equal(t, "sess-1", submit.SessionID)
	require.True(t, submit.Accepted)

	deadline := time.Now().Add(2 * time.Second)
	for {
		snapResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-1")
		require.NoError(t, err)
		var snap SessionSnapshotResponse
		require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
		_ = snapResp.Body.Close()
		if snap.Status == "finished" {
			require.Equal(t, "sess-1", snap.SessionID)
			require.NotEmpty(t, snap.Ordinal)
			require.Len(t, snap.Entities, 2)
			foundAssistant := false
			foundUser := false
			for _, entity := range snap.Entities {
				payload, ok := entity.Payload.(map[string]any)
				require.True(t, ok)
				switch payload["role"] {
				case "assistant":
					foundAssistant = payload["text"] == "Answer: Explain ordinals in plain language"
				case "user":
					foundUser = payload["content"] == "Explain ordinals in plain language"
				}
			}
			require.True(t, foundAssistant)
			require.True(t, foundUser)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for finished snapshot; last status=%q", snap.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWebSocketSnapshotAndLiveEvent(t *testing.T) {
	_, httpSrv := newTestMux(t)

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var hello map[string]any
	require.NoError(t, json.Unmarshal(raw, &hello))
	require.Equal(t, "hello", hello["type"])

	require.NoError(t, conn.WriteJSON(map[string]any{
		"type":         "subscribe",
		"sessionId":    "sess-ws-1",
		"sinceOrdinal": "0",
	}))

	_, raw, err = conn.ReadMessage()
	require.NoError(t, err)
	var snap map[string]any
	require.NoError(t, json.Unmarshal(raw, &snap))
	require.Equal(t, "snapshot", snap["type"])
	require.Equal(t, "sess-ws-1", snap["sessionId"])

	_, raw, err = conn.ReadMessage()
	require.NoError(t, err)
	var subscribed map[string]any
	require.NoError(t, json.Unmarshal(raw, &subscribed))
	require.Equal(t, "subscribed", subscribed["type"])

	body := []byte(`{"prompt":"hello over websocket"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-ws-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	seenUIEvent := false
	deadline := time.Now().Add(2 * time.Second)
	for !seenUIEvent && time.Now().Before(deadline) {
		_, raw, err = conn.ReadMessage()
		require.NoError(t, err)
		var frame map[string]any
		require.NoError(t, json.Unmarshal(raw, &frame))
		if frame["type"] == "ui-event" {
			seenUIEvent = true
			require.Equal(t, "sess-ws-1", frame["sessionId"])
		}
	}
	require.True(t, seenUIEvent, "expected at least one ui-event frame")
}

func TestSubmitAndSnapshot_UsesResolvedRuntimeWhenConfigured(t *testing.T) {
	_, httpSrv := newTestMux(t, WithRuntimeResolver(staticRuntimeResolver{completion: "hello from runtime"}))

	body := []byte(`{"prompt":"ignored by fake runtime","profile":"gpt-5-nano-low"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-runtime-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	deadline := time.Now().Add(2 * time.Second)
	for {
		snapResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-runtime-1")
		require.NoError(t, err)
		var snap SessionSnapshotResponse
		require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
		_ = snapResp.Body.Close()
		if snap.Status == "finished" {
			require.Len(t, snap.Entities, 2)
			foundAssistant := false
			for _, entity := range snap.Entities {
				payload, ok := entity.Payload.(map[string]any)
				require.True(t, ok)
				if payload["role"] == "assistant" && payload["text"] == "hello from runtime" {
					foundAssistant = true
				}
			}
			require.True(t, foundAssistant)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for finished runtime-backed snapshot; last status=%q", snap.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSQLiteSnapshotPersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "evtstream-web-chat.db")
	serverA, httpSrvA := newTestMux(t, WithSQLiteDBPath(dbPath))

	body := []byte(`{"prompt":"persist across restart","profile":"gpt-5-nano-low"}`)
	resp, err := http.Post(httpSrvA.URL+"/api/chat/sessions/sess-sql-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	deadline := time.Now().Add(2 * time.Second)
	for {
		snapResp, err := http.Get(httpSrvA.URL + "/api/chat/sessions/sess-sql-1")
		require.NoError(t, err)
		var snap SessionSnapshotResponse
		require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
		_ = snapResp.Body.Close()
		if snap.Status == "finished" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for sqlite snapshot before restart")
		}
		time.Sleep(10 * time.Millisecond)
	}

	httpSrvA.Close()
	require.NoError(t, serverA.Close())

	serverB, httpSrvB := newTestMux(t, WithSQLiteDBPath(dbPath))
	defer func() {
		httpSrvB.Close()
		_ = serverB.Close()
	}()

	snapResp, err := http.Get(httpSrvB.URL + "/api/chat/sessions/sess-sql-1")
	require.NoError(t, err)
	var snap SessionSnapshotResponse
	require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
	_ = snapResp.Body.Close()
	require.Equal(t, "finished", snap.Status)
	require.Len(t, snap.Entities, 2)
	foundAssistant := false
	foundUser := false
	for _, entity := range snap.Entities {
		payload, ok := entity.Payload.(map[string]any)
		require.True(t, ok)
		switch payload["role"] {
		case "assistant":
			foundAssistant = payload["text"] == "Answer: persist across restart"
		case "user":
			foundUser = payload["content"] == "persist across restart"
		}
	}
	require.True(t, foundAssistant)
	require.True(t, foundUser)
}
