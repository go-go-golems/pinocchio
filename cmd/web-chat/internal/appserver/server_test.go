package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/cmd/web-chat/internal/mockruntime"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstreamv1 "github.com/go-go-golems/sessionstream/pkg/sessionstream/pb/proto/sessionstream/v1"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

type runtimeBackedTestEngine struct {
	completion string
	seen       **turns.Turn
}

func (e runtimeBackedTestEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	if e.seen != nil && t != nil {
		*e.seen = t.Clone()
	}
	completion := strings.TrimSpace(e.completion)
	if completion == "" {
		completion = "runtime-backed response"
	}
	meta := gepevents.EventMetadata{}
	corr := gepevents.Correlation{RunID: "run-1", ProviderCallID: "provider-call-1", SegmentID: "segment-1"}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, completion, completion, 1))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, completion, "stop"))
	return t, nil
}

type mockRuntimeResolver struct{}

func (mockRuntimeResolver) Resolve(context.Context, *http.Request, string, string, string) (*infruntime.ComposedRuntime, error) {
	composed := mockruntime.NewComposedRuntime(mockruntime.Options{})
	return &composed, nil
}

type staticRuntimeResolver struct {
	completion    string
	seenSessionID *string
	seenTurn      **turns.Turn
}

func (r staticRuntimeResolver) Resolve(_ context.Context, _ *http.Request, sessionID string, _ string, _ string) (*infruntime.ComposedRuntime, error) {
	if r.seenSessionID != nil {
		*r.seenSessionID = sessionID
	}
	return &infruntime.ComposedRuntime{Engine: runtimeBackedTestEngine{completion: r.completion, seen: r.seenTurn}}, nil
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

func TestFrontendToolManifestEndpointPublishesTimelineEntity(t *testing.T) {
	manager := frontendtools.NewManager()
	_, httpSrv := newTestMux(t, WithFrontendToolManager(manager), WithChatPlugins(frontendtools.NewPlugin()))

	body := []byte(`{"revision":7,"tools":[{"name":"app.confirm_action","description":"Confirm an action","mode":"human","inputSchema":{"type":"object"},"available":true}]}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-tools/tools/manifest", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	desc, ok := manager.Descriptor("sess-tools", "app.confirm_action")
	require.True(t, ok)
	require.Equal(t, "app.confirm_action", desc.Name)
	require.Equal(t, toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN, desc.Mode)
	require.True(t, desc.Available)
}

func TestFrontendToolResultEndpointPublishesTimelineEntity(t *testing.T) {
	manager := frontendtools.NewManager()
	_, httpSrv := newTestMux(t, WithFrontendToolManager(manager), WithChatPlugins(frontendtools.NewPlugin()))

	body := []byte(`{"toolCallId":"call-1","toolName":"app.confirm_action","status":"success","result":{"approved":true,"decision":"approved"}}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-tools/tools/results", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	snapResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-tools")
	require.NoError(t, err)
	defer func() { _ = snapResp.Body.Close() }()
	var snap SessionSnapshotResponse
	require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
	require.Len(t, snap.Entities, 1)
	toolEntity := snap.Entities[0]
	require.Equal(t, "ChatFrontendToolCall", toolEntity.Kind)
	require.Equal(t, "call-1", toolEntity.ID)
	payload, ok := toolEntity.Payload.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "app.confirm_action", payload["toolName"])
	require.Equal(t, "success", payload["status"])
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
			require.NotEmpty(t, snap.SnapshotOrdinal)
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
	hello := readServerFrame(t, conn)
	require.NotNil(t, hello.GetHello())

	writeClientFrame(t, conn, map[string]any{
		"subscribe": map[string]any{
			"sessionId":            "sess-ws-1",
			"sinceSnapshotOrdinal": "0",
		},
	})

	snap := readServerFrame(t, conn)
	require.NotNil(t, snap.GetSnapshot())
	require.Equal(t, "sess-ws-1", snap.GetSnapshot().GetSessionId())

	subscribed := readServerFrame(t, conn)
	require.NotNil(t, subscribed.GetSubscribed())

	body := []byte(`{"prompt":"hello over websocket"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-ws-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	seenUIEvent := false
	deadline := time.Now().Add(2 * time.Second)
	for !seenUIEvent && time.Now().Before(deadline) {
		frame := readServerFrame(t, conn)
		if frame.GetUiEvent() != nil {
			seenUIEvent = true
			require.Equal(t, "sess-ws-1", frame.GetUiEvent().GetSessionId())
		}
	}
	require.True(t, seenUIEvent, "expected at least one ui-event frame")
}

func TestSubmitAndSnapshot_MockRuntimeProjectsDeterministicParityEntities(t *testing.T) {
	_, httpSrv := newTestMux(t,
		WithRuntimeResolver(mockRuntimeResolver{}),
		WithChatPlugins(plugins.NewReasoningPlugin(), plugins.NewToolCallPlugin()),
	)

	body := []byte(`{"prompt":"run deterministic parity","profile":"mock_parity"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-mock/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	snap := waitForFinishedSnapshot(t, httpSrv.URL, "sess-mock")
	kinds := map[string]bool{}
	texts := []string{}
	for _, entity := range snap.Entities {
		kinds[entity.Kind] = true
		if payload, ok := entity.Payload.(map[string]any); ok {
			if text, ok := payload["text"].(string); ok {
				texts = append(texts, text)
			}
			if content, ok := payload["content"].(string); ok {
				texts = append(texts, content)
			}
		}
	}
	require.True(t, kinds[chatapp.TimelineEntityChatMessage])
	require.True(t, kinds[plugins.TimelineEntityToolCall])
	require.Contains(t, strings.Join(texts, "\n"), "Mock parity run complete")
	require.Contains(t, strings.Join(texts, "\n"), "Inspecting deterministic inputs")
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

func TestSubmitAndSnapshot_WiresSessionIDAndTurnStoreIntoRuntime(t *testing.T) {
	prior := &turns.Turn{ID: "turn-prior"}
	turns.AppendBlock(prior, turns.NewUserTextBlock("previous question"))
	turns.AppendBlock(prior, turns.NewAssistantTextBlock("previous answer"))
	payload, err := serde.ToYAML(prior, serde.Options{})
	require.NoError(t, err)

	var seenSessionID string
	var seenTurn *turns.Turn
	_, httpSrv := newTestMux(t,
		WithTurnStore(&fakeTurnStore{snapshot: &chatstore.TurnSnapshot{
			ConvID:    "sess-history-app",
			SessionID: "sess-history-app",
			TurnID:    "turn-prior",
			Phase:     "final",
			Payload:   string(payload),
		}}),
		WithRuntimeResolver(staticRuntimeResolver{completion: "history-aware response", seenSessionID: &seenSessionID, seenTurn: &seenTurn}),
	)

	body := []byte(`{"prompt":"follow up","profile":"gpt-5-nano-low"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-history-app/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	deadline := time.Now().Add(2 * time.Second)
	for seenTurn == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.NotNil(t, seenTurn)
	require.Equal(t, "sess-history-app", seenSessionID)
	require.Len(t, seenTurn.Blocks, 3)
	require.Equal(t, "previous question", seenTurn.Blocks[0].Payload[turns.PayloadKeyText])
	require.Equal(t, "previous answer", seenTurn.Blocks[1].Payload[turns.PayloadKeyText])
	require.Equal(t, "follow up", seenTurn.Blocks[2].Payload[turns.PayloadKeyText])
}

func TestTimelineExportJSONDownload(t *testing.T) {
	_, httpSrv := newTestMux(t)

	body := []byte(`{"prompt":"export this timeline","profile":"gpt-5-nano-low"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-export-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	waitForFinishedSnapshot(t, httpSrv.URL, "sess-export-1")

	exportResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-export-1/timeline?format=json&view=entities&download=true")
	require.NoError(t, err)
	defer func() { _ = exportResp.Body.Close() }()
	require.Equal(t, http.StatusOK, exportResp.StatusCode)
	require.Equal(t, "application/json", exportResp.Header.Get("Content-Type"))
	require.Contains(t, exportResp.Header.Get("Content-Disposition"), "pinocchio-sess-export-1-timeline.json")

	var payload map[string]any
	require.NoError(t, json.NewDecoder(exportResp.Body).Decode(&payload))
	require.Equal(t, "sess-export-1", payload["session_id"])
	require.Equal(t, "entities", payload["view"])
	require.Len(t, payload["entities"], 2)
}

func TestTurnsExportMinitraceWithFileBackedDB(t *testing.T) {
	turnsDBPath := filepath.Join(t.TempDir(), "turns.db")
	turnStore, err := chatstore.NewSQLiteTurnStore(turnsDBPath)
	require.NoError(t, err)
	defer func() { _ = turnStore.Close() }()
	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("pinocchio minitrace export"))
	payload, err := serde.ToYAML(turn, serde.Options{})
	require.NoError(t, err)
	require.NoError(t, turnStore.Save(context.Background(), "sess-minitrace", "sess-minitrace", "turn-1", "final", 1000, string(payload), chatstore.TurnSaveOptions{RuntimeKey: "gpt-5-mini"}))

	_, httpSrv := newTestMux(t, WithTurnStore(turnStore), WithTurnsDBPath(turnsDBPath))
	resp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-minitrace/turns?format=minitrace&download=true")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Content-Disposition"), "pinocchio-sess-minitrace-turns.minitrace.json")
	var mt map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&mt))
	require.Equal(t, "minitrace-v0.2.0", mt["schema_version"])
	require.Equal(t, "pinocchio-turns-sqlite-v1", mt["provenance"].(map[string]any)["source_format"])
}

func TestTurnsExportYAMLAndMinitraceMissingPath(t *testing.T) {
	_, httpSrv := newTestMux(t, WithTurnStore(&fakeTurnStore{snapshot: &chatstore.TurnSnapshot{
		ConvID:      "sess-turn-export",
		SessionID:   "sess-turn-export",
		TurnID:      "turn-1",
		Phase:       "final",
		RuntimeKey:  "runtime-a",
		CreatedAtMs: 1000,
		Payload:     "id: turn-1\n",
	}}))

	yamlResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-turn-export/turns?format=yaml&download=true")
	require.NoError(t, err)
	defer func() { _ = yamlResp.Body.Close() }()
	require.Equal(t, http.StatusOK, yamlResp.StatusCode)
	require.Equal(t, "application/x-yaml", yamlResp.Header.Get("Content-Type"))
	require.Contains(t, yamlResp.Header.Get("Content-Disposition"), "pinocchio-sess-turn-export-turns.yaml")
	yamlBody, err := io.ReadAll(yamlResp.Body)
	require.NoError(t, err)
	require.Contains(t, string(yamlBody), "turn_id: turn-1")
	require.Contains(t, string(yamlBody), "runtime_key: runtime-a")

	minitraceResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-turn-export/turns?format=minitrace&download=true")
	require.NoError(t, err)
	defer func() { _ = minitraceResp.Body.Close() }()
	require.Equal(t, http.StatusConflict, minitraceResp.StatusCode)
}

func TestFullExportOmitsTurnsWhenStoreUnavailable(t *testing.T) {
	_, httpSrv := newTestMux(t)

	body := []byte(`{"prompt":"export bundle","profile":"gpt-5-nano-low"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-full-export/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	waitForFinishedSnapshot(t, httpSrv.URL, "sess-full-export")

	exportResp, err := http.Get(httpSrv.URL + "/api/chat/sessions/sess-full-export/export?format=json")
	require.NoError(t, err)
	defer func() { _ = exportResp.Body.Close() }()
	require.Equal(t, http.StatusOK, exportResp.StatusCode)
	var payload map[string]any
	require.NoError(t, json.NewDecoder(exportResp.Body).Decode(&payload))
	require.Equal(t, "sess-full-export", payload["session_id"])
	require.Contains(t, payload, "timeline")
	require.NotContains(t, payload, "turns")
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

func waitForFinishedSnapshot(t *testing.T, baseURL string, sessionID string) SessionSnapshotResponse {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		snapResp, err := http.Get(baseURL + "/api/chat/sessions/" + sessionID)
		require.NoError(t, err)
		var snap SessionSnapshotResponse
		require.NoError(t, json.NewDecoder(snapResp.Body).Decode(&snap))
		_ = snapResp.Body.Close()
		if snap.Status == "finished" {
			return snap
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for finished snapshot; last status=%q", snap.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type fakeTurnStore struct {
	snapshot *chatstore.TurnSnapshot
	err      error
}

func (s *fakeTurnStore) Save(context.Context, string, string, string, string, int64, string, chatstore.TurnSaveOptions) error {
	return nil
}

func (s *fakeTurnStore) List(context.Context, chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	if s.snapshot == nil {
		return nil, s.err
	}
	return []chatstore.TurnSnapshot{*s.snapshot}, s.err
}

func (s *fakeTurnStore) LoadLatestTurn(context.Context, string, string) (*chatstore.TurnSnapshot, error) {
	return s.snapshot, s.err
}

func (s *fakeTurnStore) Close() error { return nil }

func readServerFrame(t *testing.T, conn *websocket.Conn) *sessionstreamv1.ServerFrame {
	t.Helper()
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	frame := &sessionstreamv1.ServerFrame{}
	require.NoError(t, protojson.Unmarshal(raw, frame))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	return frame
}

func writeClientFrame(t *testing.T, conn *websocket.Conn, payload map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, body))
}
