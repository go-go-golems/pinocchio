package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func newTestMux(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	srv, err := NewServer(WithDefaultProfile("gpt-5-nano-low"), WithChunkDelay(time.Millisecond))
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat/sessions", srv.HandleCreateSession)
	mux.HandleFunc("/api/chat/sessions/", srv.HandleSessionRoutes)
	mux.HandleFunc("/api/chat/ws", srv.HandleWS)

	httpSrv := httptest.NewServer(mux)
	t.Cleanup(httpSrv.Close)
	return srv, httpSrv
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
			require.Len(t, snap.Entities, 1)
			payload, ok := snap.Entities[0].Payload.(map[string]any)
			require.True(t, ok)
			require.Equal(t, "Answer: Explain ordinals in plain language", payload["text"])
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
