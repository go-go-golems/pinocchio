package webchat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"github.com/stretchr/testify/require"
)

func TestAPIHandler_OfflineRunsAndArtifactDetail(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "run-1")
	require.NoError(t, os.MkdirAll(runDir, 0o755))

	yamlPayload := "id: turn-1\nblocks:\n  - kind: llm_text\n    role: assistant\n    payload:\n      text: hello\n"
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "input_turn.yaml"), []byte(yamlPayload), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "final_turn.yaml"), []byte(yamlPayload), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "events.ndjson"), []byte("{\"type\":\"x\",\"event\":{\"type\":\"chat.message\"}}\n"), 0o644))

	r := &Router{
		cm:                &ConvManager{conns: map[string]*Conversation{}},
		enableDebugRoutes: true,
	}
	h := r.APIHandler()

	listStatus, listBody := runRequest(t, h, http.MethodGet, "/api/debug/runs?artifacts_root="+url.QueryEscape(root), nil)
	require.Equal(t, http.StatusOK, listStatus)

	listResp := map[string]any{}
	require.NoError(t, json.Unmarshal(listBody, &listResp))
	items, ok := listResp["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, items)

	runID := items[0].(map[string]any)["run_id"].(string)
	detailPath := "/api/debug/runs/" + url.PathEscape(runID) + "?artifacts_root=" + url.QueryEscape(root)
	detailStatus, detailBody := runRequest(t, h, http.MethodGet, detailPath, nil)
	require.Equal(t, http.StatusOK, detailStatus)

	detailResp := map[string]any{}
	require.NoError(t, json.Unmarshal(detailBody, &detailResp))
	require.Equal(t, runID, detailResp["run_id"])
	require.Equal(t, "artifact", detailResp["kind"])
	detail := detailResp["detail"].(map[string]any)
	turns, ok := detail["turns"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, turns)
	events, ok := detail["events"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, events)
}

func TestAPIHandler_OfflineRunsAndTurnsSQLiteDetail(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "turns.db")
	dsn, err := chatstore.SQLiteTurnDSNForFile(dbPath)
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	payload := "id: turn-1\nblocks:\n  - id: b1\n    kind: llm_text\n    role: assistant\n    payload:\n      text: hello\n"
	require.NoError(t, store.Save(context.Background(), "conv-1", "session-1", "turn-1", "draft", 100, payload))
	require.NoError(t, store.Save(context.Background(), "conv-1", "session-1", "turn-1", "final", 200, payload))

	r := &Router{
		cm:                &ConvManager{conns: map[string]*Conversation{}},
		enableDebugRoutes: true,
	}
	h := r.APIHandler()

	listStatus, listBody := runRequest(t, h, http.MethodGet, "/api/debug/runs?turns_db="+url.QueryEscape(dbPath), nil)
	require.Equal(t, http.StatusOK, listStatus)

	listResp := map[string]any{}
	require.NoError(t, json.Unmarshal(listBody, &listResp))
	items, ok := listResp["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, items)

	runID := items[0].(map[string]any)["run_id"].(string)
	detailPath := "/api/debug/runs/" + url.PathEscape(runID) + "?turns_db=" + url.QueryEscape(dbPath)
	detailStatus, detailBody := runRequest(t, h, http.MethodGet, detailPath, nil)
	require.Equal(t, http.StatusOK, detailStatus)

	detailResp := map[string]any{}
	require.NoError(t, json.Unmarshal(detailBody, &detailResp))
	require.Equal(t, "turns", detailResp["kind"])
	detail := detailResp["detail"].(map[string]any)
	require.Equal(t, "conv-1", detail["conv_id"])
	require.Equal(t, "session-1", detail["session_id"])
	rows, ok := detail["items"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 2)
}

func TestAPIHandler_OfflineRunsAndTimelineSQLiteDetail(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "timeline.db")
	dsn, err := chatstore.SQLiteTimelineDSNForFile(dbPath)
	require.NoError(t, err)
	store, err := chatstore.NewSQLiteTimelineStore(dsn)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	err = store.Upsert(context.Background(), "conv-1", 3, &timelinepb.TimelineEntityV1{
		Id:   "msg-1",
		Kind: "message",
		Snapshot: &timelinepb.TimelineEntityV1_Message{
			Message: &timelinepb.MessageSnapshotV1{
				SchemaVersion: 1,
				Role:          "assistant",
				Content:       "hello",
				Streaming:     false,
			},
		},
	})
	require.NoError(t, err)

	r := &Router{
		cm:                &ConvManager{conns: map[string]*Conversation{}},
		enableDebugRoutes: true,
	}
	h := r.APIHandler()

	listStatus, listBody := runRequest(t, h, http.MethodGet, "/api/debug/runs?timeline_db="+url.QueryEscape(dbPath), nil)
	require.Equal(t, http.StatusOK, listStatus)

	listResp := map[string]any{}
	require.NoError(t, json.Unmarshal(listBody, &listResp))
	items, ok := listResp["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, items)

	runID := items[0].(map[string]any)["run_id"].(string)
	detailPath := "/api/debug/runs/" + url.PathEscape(runID) + "?timeline_db=" + url.QueryEscape(dbPath)
	detailStatus, detailBody := runRequest(t, h, http.MethodGet, detailPath, nil)
	require.Equal(t, http.StatusOK, detailStatus)

	detailResp := map[string]any{}
	require.NoError(t, json.Unmarshal(detailBody, &detailResp))
	require.Equal(t, "timeline", detailResp["kind"])
	detail := detailResp["detail"].(map[string]any)
	snapshot, ok := detail["snapshot"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "conv-1", snapshot["convId"])
}
