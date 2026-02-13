package webchat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/stretchr/testify/require"
)

type stubTimelineStore struct {
	snapshot *timelinepb.TimelineSnapshotV1
}

func (s *stubTimelineStore) Upsert(context.Context, string, uint64, *timelinepb.TimelineEntityV1) error {
	return nil
}

func (s *stubTimelineStore) GetSnapshot(context.Context, string, uint64, int) (*timelinepb.TimelineSnapshotV1, error) {
	if s.snapshot == nil {
		return &timelinepb.TimelineSnapshotV1{}, nil
	}
	return s.snapshot, nil
}

func (s *stubTimelineStore) Close() error { return nil }

type stubTurnStore struct {
	items []chatstore.TurnSnapshot
}

func (s *stubTurnStore) Save(context.Context, string, string, string, string, int64, string) error {
	return nil
}

func (s *stubTurnStore) List(context.Context, chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	return s.items, nil
}

func (s *stubTurnStore) Close() error { return nil }

func TestAPIHandler_DebugTimelineParity(t *testing.T) {
	r := &Router{
		profiles:      newInMemoryProfileRegistry(),
		cm:            &ConvManager{conns: map[string]*Conversation{}},
		timelineStore: &stubTimelineStore{snapshot: sampleTimelineSnapshot()},
	}
	h := r.APIHandler()

	oldStatus, oldBody := runRequest(t, h, http.MethodGet, "/timeline?conv_id=conv-1", nil)
	newStatus, newBody := runRequest(t, h, http.MethodGet, "/api/debug/timeline?conv_id=conv-1", nil)

	require.Equal(t, oldStatus, newStatus)
	require.Equal(t, oldBody, newBody)
}

func TestAPIHandler_DebugTurnsParity(t *testing.T) {
	items := []chatstore.TurnSnapshot{
		{ConvID: "conv-1", SessionID: "session-1", TurnID: "turn-1", Phase: "final", CreatedAtMs: 101, Payload: "payload-1"},
	}
	r := &Router{
		profiles:  newInMemoryProfileRegistry(),
		cm:        &ConvManager{conns: map[string]*Conversation{}},
		turnStore: &stubTurnStore{items: items},
	}
	h := r.APIHandler()

	query := "/turns?conv_id=conv-1&session_id=session-1&phase=final&since_ms=100&limit=1"
	oldStatus, oldBody := runRequest(t, h, http.MethodGet, query, nil)
	newStatus, newBody := runRequest(t, h, http.MethodGet, "/api/debug"+query, nil)

	require.Equal(t, oldStatus, newStatus)
	require.Equal(t, oldBody, newBody)
}

func TestAPIHandler_DebugTurnsEnvelopeMetadata(t *testing.T) {
	items := []chatstore.TurnSnapshot{
		{ConvID: "conv-1", SessionID: "session-1", TurnID: "turn-1", Phase: "draft", CreatedAtMs: 100, Payload: "p1"},
		{ConvID: "conv-1", SessionID: "session-1", TurnID: "turn-2", Phase: "final", CreatedAtMs: 200, Payload: "p2"},
	}
	r := &Router{
		profiles:  newInMemoryProfileRegistry(),
		cm:        &ConvManager{conns: map[string]*Conversation{}},
		turnStore: &stubTurnStore{items: items},
	}
	h := r.APIHandler()

	status, body := runRequest(t, h, http.MethodGet, "/api/debug/turns?conv_id=conv-1&session_id=session-1&phase=final&since_ms=42&limit=2", nil)
	require.Equal(t, http.StatusOK, status)

	resp := map[string]any{}
	require.NoError(t, json.Unmarshal(body, &resp))
	require.Equal(t, "conv-1", resp["conv_id"])
	require.Equal(t, "session-1", resp["session_id"])
	require.Equal(t, "final", resp["phase"])
	require.Equal(t, float64(42), resp["since_ms"])

	rawItems, ok := resp["items"].([]any)
	require.True(t, ok)
	require.Len(t, rawItems, 2)
}

func TestAPIHandler_DebugStepRoutes(t *testing.T) {
	t.Setenv("PINOCCHIO_WEBCHAT_DEBUG", "1")

	stepCtrl := toolloop.NewStepController()
	r := &Router{
		profiles: newInMemoryProfileRegistry(),
		cm:       &ConvManager{conns: map[string]*Conversation{}},
		stepCtrl: stepCtrl,
	}
	h := r.APIHandler()

	enableStatus, enableBody := runRequest(t, h, http.MethodPost, "/api/debug/step/enable", map[string]any{
		"session_id": "session-1",
		"owner":      "debug-ui",
	})
	require.Equal(t, http.StatusOK, enableStatus)
	enableResp := map[string]any{}
	require.NoError(t, json.Unmarshal(enableBody, &enableResp))
	require.Equal(t, true, enableResp["ok"])
	require.Equal(t, "session-1", enableResp["session_id"])

	meta, ok := stepCtrl.Pause(toolloop.PauseMeta{
		SessionID: "session-1",
		Summary:   "pause for handler test",
		Phase:     toolloop.StepPhaseAfterInference,
	})
	require.True(t, ok)
	require.NotEmpty(t, meta.PauseID)

	continueStatus, continueBody := runRequest(t, h, http.MethodPost, "/api/debug/continue", map[string]any{
		"pause_id": meta.PauseID,
	})
	require.Equal(t, http.StatusOK, continueStatus)
	continueResp := map[string]any{}
	require.NoError(t, json.Unmarshal(continueBody, &continueResp))
	require.Equal(t, true, continueResp["ok"])

	disableStatus, disableBody := runRequest(t, h, http.MethodPost, "/api/debug/step/disable", map[string]any{
		"session_id": "session-1",
	})
	require.Equal(t, http.StatusOK, disableStatus)
	disableResp := map[string]any{}
	require.NoError(t, json.Unmarshal(disableBody, &disableResp))
	require.Equal(t, true, disableResp["ok"])
	require.Equal(t, "session-1", disableResp["session_id"])
}

func runRequest(t *testing.T, h http.Handler, method string, path string, payload map[string]any) (int, []byte) {
	t.Helper()

	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, "http://example.com"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func sampleTimelineSnapshot() *timelinepb.TimelineSnapshotV1 {
	return &timelinepb.TimelineSnapshotV1{
		ConvId:       "conv-1",
		Version:      3,
		ServerTimeMs: 1000,
		Entities: []*timelinepb.TimelineEntityV1{
			{
				Id:          "msg-1",
				Kind:        "message",
				CreatedAtMs: 900,
				UpdatedAtMs: 1000,
				Snapshot: &timelinepb.TimelineEntityV1_Message{
					Message: &timelinepb.MessageSnapshotV1{
						SchemaVersion: 1,
						Role:          "assistant",
						Content:       "hello",
						Streaming:     false,
					},
				},
			},
		},
	}
}
