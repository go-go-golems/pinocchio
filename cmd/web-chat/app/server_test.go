package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	geppettoobs "github.com/go-go-golems/geppetto/pkg/observability"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstreamv1 "github.com/go-go-golems/sessionstream/pkg/sessionstream/pb/proto/sessionstream/v1"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
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
	corr := gepevents.Correlation{SegmentID: "segment-1", SegmentIndex: 1, SegmentType: "text", StreamKind: "content", CorrelationKey: "text:1"}
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentStartedEvent(meta, corr, "assistant"))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextDeltaEvent(meta, corr, completion, completion, 1))
	gepevents.PublishEventToContext(ctx, gepevents.NewTextSegmentFinishedEvent(meta, corr, completion, "stop"))
	return t, nil
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
	if srv.debugRecorder != nil {
		mux.HandleFunc("/api/debug/sessions/", srv.HandleDebugRoutes)
	}

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

func TestDebugRecorderEndpointsExposePipelineAndTransportRecords(t *testing.T) {
	recorder := NewStreamDebugRecorder(1000)
	_, httpSrv := newTestMux(t, WithDebugRecorder(recorder), WithRuntimeResolver(staticRuntimeResolver{completion: "debug response"}))

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_ = readServerFrame(t, conn) // hello
	writeClientFrame(t, conn, map[string]any{"subscribe": map[string]any{"sessionId": "sess-debug-1"}})
	_ = readServerFrame(t, conn) // snapshot
	_ = readServerFrame(t, conn) // subscribed

	body := []byte(`{"prompt":"collect debug records"}`)
	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-debug-1/messages", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		frame := readServerFrame(t, conn)
		if frame.GetUiEvent() != nil && frame.GetUiEvent().GetName() == "ChatMessageFinished" {
			break
		}
	}

	pipelineResp, err := http.Get(httpSrv.URL + "/api/debug/sessions/sess-debug-1/pipeline")
	require.NoError(t, err)
	defer func() { _ = pipelineResp.Body.Close() }()
	require.Equal(t, http.StatusOK, pipelineResp.StatusCode)
	var pipeline debugRecordsResponse
	require.NoError(t, json.NewDecoder(pipelineResp.Body).Decode(&pipeline))
	require.Equal(t, "sess-debug-1", pipeline.SessionID)
	require.Equal(t, string(DebugRecordKindPipeline), pipeline.Kind)
	require.NotEmpty(t, pipeline.Records)
	require.Equal(t, DebugRecordKindPipeline, pipeline.Records[0].Kind)

	transportResp, err := http.Get(httpSrv.URL + "/api/debug/sessions/sess-debug-1/transport")
	require.NoError(t, err)
	defer func() { _ = transportResp.Body.Close() }()
	require.Equal(t, http.StatusOK, transportResp.StatusCode)
	var transport debugRecordsResponse
	require.NoError(t, json.NewDecoder(transportResp.Body).Decode(&transport))
	require.Equal(t, string(DebugRecordKindTransport), transport.Kind)
	require.NotEmpty(t, transport.Records)
	foundFanout := false
	for _, rec := range transport.Records {
		if rec.Transport != nil && rec.Transport.Stage == string(wstransport.TransportStageFanoutStarted) {
			foundFanout = true
		}
	}
	require.True(t, foundFanout)

	reconcileResp, err := http.Get(httpSrv.URL + "/api/debug/sessions/sess-debug-1/reconcile")
	require.NoError(t, err)
	defer func() { _ = reconcileResp.Body.Close() }()
	require.Equal(t, http.StatusOK, reconcileResp.StatusCode)
	var reconcile DebugReconcileResponse
	require.NoError(t, json.NewDecoder(reconcileResp.Body).Decode(&reconcile))
	require.Equal(t, "sess-debug-1", reconcile.SessionID)
	require.NotZero(t, reconcile.PipelineRecordCount)
	require.NotZero(t, reconcile.TransportRecordCount)
}

func TestDebugRecorderEndpointExposesGeppettoRecords(t *testing.T) {
	recorder := NewStreamDebugRecorder(1000)
	_, httpSrv := newTestMux(t, WithDebugRecorder(recorder), WithRuntimeResolver(staticRuntimeResolver{completion: "debug response"}))

	recorder.OnGeppettoRecord(context.Background(), geppettoobs.Record{
		Timestamp:    time.Now().UTC(),
		SessionID:    "sess-geppetto-1",
		Stage:        geppettoobs.StageProviderRoutedEvent,
		Provider:     "openai_responses",
		EventType:    "response.reasoning_summary_text.delta",
		ResponseID:   "resp_1",
		ItemID:       "rs_1",
		ObjectJSON:   json.RawMessage(`{"item_id":"rs_1","delta":"thinking"}`),
		EventJSON:    json.RawMessage(`{"message":"reasoning-summary"}`),
		MetadataJSON: json.RawMessage(`{"turn_id":"turn_1"}`),
	})

	geppettoResp, err := http.Get(httpSrv.URL + "/api/debug/sessions/sess-geppetto-1/geppetto")
	require.NoError(t, err)
	defer func() { _ = geppettoResp.Body.Close() }()
	require.Equal(t, http.StatusOK, geppettoResp.StatusCode)
	var out debugRecordsResponse
	require.NoError(t, json.NewDecoder(geppettoResp.Body).Decode(&out))
	require.Equal(t, "sess-geppetto-1", out.SessionID)
	require.Equal(t, string(DebugRecordKindGeppetto), out.Kind)
	require.Len(t, out.Records, 1)
	require.Equal(t, DebugRecordKindGeppetto, out.Records[0].Kind)
	require.NotNil(t, out.Records[0].Geppetto)
	require.Equal(t, "rs_1", out.Records[0].Geppetto.ItemID)
	require.Equal(t, "response.reasoning_summary_text.delta", out.Records[0].Geppetto.EventType)
	require.NotNil(t, out.Records[0].Geppetto.ObjectJSON)
	require.NotNil(t, out.Records[0].Geppetto.EventJSON)
	require.NotNil(t, out.Records[0].Geppetto.MetadataJSON)
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

func TestDebugReconcileUploadReturnsSQLiteDatabase(t *testing.T) {
	recorder := NewStreamDebugRecorder(1000)
	_, httpSrv := newTestMux(t, WithDebugRecorder(recorder), WithRuntimeResolver(staticRuntimeResolver{completion: "sqlite debug response"}))

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/api/chat/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	_ = readServerFrame(t, conn)
	writeClientFrame(t, conn, map[string]any{"subscribe": map[string]any{"sessionId": "sess-sqlite-debug"}})
	_ = readServerFrame(t, conn)
	_ = readServerFrame(t, conn)

	resp, err := http.Post(httpSrv.URL+"/api/chat/sessions/sess-sqlite-debug/messages", "application/json", bytes.NewReader([]byte(`{"prompt":"make records"}`)))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = readServerFrame(t, conn)

	recorder.OnGeppettoRecord(context.Background(), geppettoobs.Record{
		Timestamp:    time.Now().UTC(),
		SessionID:    "sess-sqlite-debug",
		InferenceID:  "inf_1",
		TurnID:       "turn_1",
		MessageID:    "msg_1",
		Stage:        geppettoobs.StageProviderRoutedEvent,
		Provider:     "openai_responses",
		EventType:    "response.reasoning_summary_text.delta",
		ResponseID:   "resp_1",
		ItemID:       "rs_1",
		SummaryIndex: ptr(0),
		ObjectJSON:   json.RawMessage(`{"item_id":"rs_1","summary_index":0,"delta":"thinking"}`),
	})
	recorder.OnGeppettoRecord(context.Background(), geppettoobs.Record{
		Timestamp:    time.Now().UTC(),
		SessionID:    "sess-sqlite-debug",
		InferenceID:  "inf_1",
		TurnID:       "turn_1",
		MessageID:    "msg_1",
		Stage:        geppettoobs.StageGeppettoPublishDone,
		Provider:     "openai_responses",
		EventType:    string(gepevents.EventTypeInfo),
		InfoMessage:  "reasoning-summary",
		ResponseID:   "resp_1",
		ItemID:       "rs_1",
		SummaryIndex: ptr(0),
		EventJSON:    json.RawMessage(`{"message":"reasoning-summary","data":{"item_id":"rs_1"}}`),
		MetadataJSON: json.RawMessage(`{"turn_id":"turn_1","inference_id":"inf_1"}`),
	})

	uploadBody := `{"records":[{"id":1,"timestamp":1770000000000,"type":"parsed-frame","sessionId":"sess-sqlite-debug","ordinal":"1","frameType":"ui-event","name":"ChatMessageAppended","frame":{"type":"ui-event"}},{"id":2,"timestamp":1770000000001,"type":"ui-event","sessionId":"sess-sqlite-debug","ordinal":"1","name":"ChatMessageAppended","messageId":"chat-msg-1","mutation":{"upsert":{"id":"chat-msg-1"}}}]}`
	uploadResp, err := http.Post(httpSrv.URL+"/api/debug/sessions/sess-sqlite-debug/reconcile/upload", "application/json", strings.NewReader(uploadBody))
	require.NoError(t, err)
	defer func() { _ = uploadResp.Body.Close() }()
	require.Equal(t, http.StatusOK, uploadResp.StatusCode)
	require.Equal(t, "application/vnd.sqlite3", uploadResp.Header.Get("Content-Type"))
	body, err := io.ReadAll(uploadResp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, body)

	dbPath := filepath.Join(t.TempDir(), "debug.sqlite")
	require.NoError(t, os.WriteFile(dbPath, body, 0o644))
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	assertTableCount(t, db, "backend_records")
	assertTableCount(t, db, "backend_pipeline")
	assertTableCount(t, db, "backend_transport")
	assertTableCount(t, db, "frontend_records")
	assertTableCount(t, db, "frontend_parsed_frames")
	assertTableCount(t, db, "frontend_ui_events")

	assertTableCount(t, db, "geppetto_records")
	assertTableCount(t, db, "geppetto_provider_events")
	assertTableCount(t, db, "geppetto_emitted_events")
	var geppettoMeta string
	require.NoError(t, db.QueryRow("SELECT value FROM meta WHERE key='geppetto_record_count'").Scan(&geppettoMeta))
	assert.Equal(t, "2", geppettoMeta)
	var itemID, objectJSON string
	require.NoError(t, db.QueryRow("SELECT item_id, object_json FROM geppetto_provider_events WHERE provider_event_type=?", "response.reasoning_summary_text.delta").Scan(&itemID, &objectJSON))
	assert.Equal(t, "rs_1", itemID)
	assert.Contains(t, objectJSON, "thinking")
	var eventJSON, metadataJSON string
	require.NoError(t, db.QueryRow("SELECT event_json, metadata_json FROM geppetto_emitted_events WHERE info_message=?", "reasoning-summary").Scan(&eventJSON, &metadataJSON))
	assert.Contains(t, eventJSON, "reasoning-summary")
	assert.Contains(t, metadataJSON, "turn_1")
	assertViewExists(t, db, "geppetto_reasoning_sequence")
	assertViewExists(t, db, "geppetto_summary_without_item_id")
	assertViewExists(t, db, "geppetto_publish_errors")
	assertViewExists(t, db, "geppetto_reasoning_to_frontend")

	// Verify timeline_entities and turns tables exist (may be empty without turn store).
	assertTableExists(t, db, "timeline_entities")
	assertTableExists(t, db, "turns")
}

func TestDebugReconcileUploadIncludesTimelineAndTurns(t *testing.T) {
	recorder := NewStreamDebugRecorder(1000)
	mockProvider := &mockDebugDataProvider{
		timelineEntities: []DebugTimelineEntity{
			{Kind: "message", ID: "msg-1", CreatedOrdinal: 1, LastEventOrdinal: 3, Tombstone: false, PayloadType: "ChatMessage", Payload: `{"text":"hello"}`},
			{Kind: "message", ID: "msg-2", CreatedOrdinal: 2, LastEventOrdinal: 4, Tombstone: true, PayloadType: "ChatMessage", Payload: `{"text":"deleted"}`},
		},
		turns: []DebugTurn{
			{ConvID: "sess-tl-turns", SessionID: "sess-tl-turns", TurnID: "turn-1", Phase: "final", CreatedAtMs: 1700000000000, Payload: `{"blocks":[{"id":"b1"}]}`},
			{ConvID: "sess-tl-turns", SessionID: "sess-tl-turns", TurnID: "turn-2", Phase: "streaming", CreatedAtMs: 1700000001000, Payload: `{"blocks":[{"id":"b2"}]}`},
		},
	}
	uploadBody := `{"records":[]}`
	body, err := recorder.BuildSQLiteReconcileDB(context.Background(), "sess-tl-turns", strings.NewReader(uploadBody), mockProvider)
	require.NoError(t, err)
	require.NotEmpty(t, body)

	dbPath := filepath.Join(t.TempDir(), "debug-tl.sqlite")
	require.NoError(t, os.WriteFile(dbPath, body, 0o644))
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Verify timeline entities.
	var tlCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM timeline_entities").Scan(&tlCount))
	assert.Equal(t, 2, tlCount)

	var kind, eid, pt string
	var co, lo int
	var tomb int
	require.NoError(t, db.QueryRow("SELECT kind, entity_id, created_ordinal, last_event_ordinal, tombstone, payload_type FROM timeline_entities WHERE entity_id=?", "msg-1").Scan(&kind, &eid, &co, &lo, &tomb, &pt))
	assert.Equal(t, "message", kind)
	assert.Equal(t, "msg-1", eid)
	assert.Equal(t, 1, co)
	assert.Equal(t, 3, lo)
	assert.Equal(t, 0, tomb)
	assert.Equal(t, "ChatMessage", pt)

	// Verify tombstone entity.
	require.NoError(t, db.QueryRow("SELECT tombstone FROM timeline_entities WHERE entity_id=?", "msg-2").Scan(&tomb))
	assert.Equal(t, 1, tomb)

	// Verify turns.
	var turnCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM turns").Scan(&turnCount))
	assert.Equal(t, 2, turnCount)

	var phase, payload string
	var createdAtMs int64
	require.NoError(t, db.QueryRow("SELECT phase, created_at_ms, payload_json FROM turns WHERE turn_id=?", "turn-1").Scan(&phase, &createdAtMs, &payload))
	assert.Equal(t, "final", phase)
	assert.Equal(t, int64(1700000000000), createdAtMs)
	assert.Contains(t, payload, "b1")

	// Meta should reference the session.
	var metaSession string
	require.NoError(t, db.QueryRow("SELECT value FROM meta WHERE key='session_id'").Scan(&metaSession))
	assert.Equal(t, "sess-tl-turns", metaSession)

	// Verify SQL views were created.
	var viewCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='view'").Scan(&viewCount))
	assert.Greater(t, viewCount, 0, "expected SQL views to be created")

	// Spot-check a view query works.
	var deliveryRows int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM delivery_chain").Scan(&deliveryRows))
	assert.GreaterOrEqual(t, deliveryRows, 0, "delivery_chain view should be queryable")
}

type mockDebugDataProvider struct {
	timelineEntities []DebugTimelineEntity
	turns            []DebugTurn
}

func (m *mockDebugDataProvider) ExportTimelineEntities(_ context.Context, _ string) ([]DebugTimelineEntity, error) {
	return m.timelineEntities, nil
}

func (m *mockDebugDataProvider) ExportTurnsList(_ context.Context, _ string) ([]DebugTurn, error) {
	return m.turns, nil
}

func ptr[T any](v T) *T { return &v }

func assertTableCount(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
	require.Greater(t, count, 0, "expected rows in %s", table)
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var name string
	require.NoError(t, db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name))
	require.Equal(t, table, name, "expected table %s to exist", table)
}

func assertViewExists(t *testing.T, db *sql.DB, view string) {
	t.Helper()
	var name string
	require.NoError(t, db.QueryRow("SELECT name FROM sqlite_master WHERE type='view' AND name=?", view).Scan(&name))
	require.Equal(t, view, name, "expected view %s to exist", view)
}
