package export

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestExportTimelineEntities(t *testing.T) {
	payload, err := structpb.NewStruct(map[string]any{"text": "hello", "count": float64(3)})
	require.NoError(t, err)
	provider := fakeSnapshotProvider{snap: sessionstream.Snapshot{
		SessionId:       "session-1",
		SnapshotOrdinal: 42,
		Entities: []sessionstream.TimelineEntity{{
			Kind:             "ChatMessage",
			Id:               "msg-1",
			CreatedOrdinal:   1,
			LastEventOrdinal: 2,
			Payload:          payload,
		}},
	}}
	svc := NewService(provider, WithClock(fixedClock))

	exported, err := svc.ExportTimeline(context.Background(), "session-1", Options{View: TimelineViewEntities})
	require.NoError(t, err)
	require.Equal(t, "session-1", exported.SessionID)
	require.Equal(t, uint64(42), exported.SnapshotOrdinal)
	require.Equal(t, "2026-05-06T12:00:00Z", exported.ExportedAt)
	require.Len(t, exported.Entities, 1)
	require.Equal(t, "ChatMessage", exported.Entities[0].Kind)
	require.Equal(t, "msg-1", exported.Entities[0].ID)
	require.Equal(t, uint64(1), exported.Entities[0].CreatedOrdinal)
	require.Equal(t, uint64(2), exported.Entities[0].LastEventOrdinal)
	require.Equal(t, map[string]any{"count": float64(3), "text": "hello"}, exported.Entities[0].Payload)
}

func TestExportTurnsSortsOldestFirst(t *testing.T) {
	store := &fakeTurnStore{items: []chatstore.TurnSnapshot{
		{ConvID: "session-1", SessionID: "session-1", TurnID: "turn-new", Phase: "final", RuntimeKey: "runtime-b", CreatedAtMs: 2000, Payload: "id: turn-new\n"},
		{ConvID: "session-1", SessionID: "session-1", TurnID: "turn-old", Phase: "final", RuntimeKey: "runtime-a", CreatedAtMs: 1000, Payload: "id: turn-old\n"},
	}}
	svc := NewService(nil, WithTurnStore(store), WithClock(fixedClock))

	exported, err := svc.ExportTurns(context.Background(), "session-1", Options{})
	require.NoError(t, err)
	require.Equal(t, chatstore.TurnQuery{ConvID: "session-1", Phase: "final", Limit: 1000}, store.lastQuery)
	require.Equal(t, "session-1", exported.SessionID)
	require.Equal(t, "final", exported.Phase)
	require.Len(t, exported.Turns, 2)
	require.Equal(t, "turn-old", exported.Turns[0].TurnID)
	require.Equal(t, "1970-01-01T00:00:01Z", exported.Turns[0].CreatedAt)
	require.Equal(t, "turn-new", exported.Turns[1].TurnID)
}

func TestExportTurnsRequiresStore(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.ExportTurns(context.Background(), "session-1", Options{})
	require.ErrorIs(t, err, ErrTurnStoreUnavailable)
}

func TestExportTurnsMinitraceRequiresFileBackedDBPath(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.ExportTurnsMinitrace(context.Background(), "session-1", Options{})
	require.ErrorIs(t, err, ErrTurnsDBPathRequired)
}

func TestExportTurnsMinitraceFromFileBackedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "turns.db")
	store, err := chatstore.NewSQLiteTurnStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hello minitrace"))
	turns.AppendBlock(turn, turns.NewAssistantTextBlock("hello back"))
	payload, err := serde.ToYAML(turn, serde.Options{})
	require.NoError(t, err)
	require.NoError(t, store.Save(context.Background(), "session-1", "session-1", "turn-1", "final", 1000, string(payload), chatstore.TurnSaveOptions{RuntimeKey: "gpt-5-mini"}))

	svc := NewService(nil, WithTurnsDBPath(dbPath), WithClock(fixedClock))
	raw, err := svc.ExportTurnsMinitrace(context.Background(), "session-1", Options{})
	require.NoError(t, err)
	session, ok := raw.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "session-1", session["id"])
	require.Equal(t, "minitrace-v0.2.0", session["schema_version"])
	require.Equal(t, "B", session["quality"])
	require.Equal(t, "hello minitrace", session["title"])
	require.Equal(t, "pinocchio-turns-sqlite-v1", session["provenance"].(map[string]any)["source_format"])
	require.Equal(t, "pinocchio", session["environment"].(map[string]any)["agent_framework"])
	require.Equal(t, "openai", session["environment"].(map[string]any)["provider_hint"])
	require.Len(t, session["turns"], 2)
	require.Equal(t, 2, session["metrics"].(map[string]any)["turn_count"])
}

func TestRenderJSONAndYAML(t *testing.T) {
	value := map[string]any{"session_id": "session-1"}

	jsonRendered, err := Render(value, FormatJSON)
	require.NoError(t, err)
	require.Equal(t, "application/json", jsonRendered.ContentType)
	require.Equal(t, ".json", jsonRendered.Extension)
	require.Contains(t, string(jsonRendered.Body), `"session_id": "session-1"`)

	yamlRendered, err := Render(value, FormatYAML)
	require.NoError(t, err)
	require.Equal(t, "application/x-yaml", yamlRendered.ContentType)
	require.Equal(t, ".yaml", yamlRendered.Extension)
	require.Contains(t, string(yamlRendered.Body), "session_id: session-1")
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
}

type fakeSnapshotProvider struct {
	snap sessionstream.Snapshot
	err  error
}

func (f fakeSnapshotProvider) Snapshot(context.Context, sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if f.err != nil {
		return sessionstream.Snapshot{}, f.err
	}
	return f.snap, nil
}

type fakeTurnStore struct {
	items     []chatstore.TurnSnapshot
	err       error
	lastQuery chatstore.TurnQuery
}

func (f *fakeTurnStore) Save(context.Context, string, string, string, string, int64, string, chatstore.TurnSaveOptions) error {
	return nil
}

func (f *fakeTurnStore) List(_ context.Context, q chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	f.lastQuery = q
	if f.err != nil {
		return nil, f.err
	}
	return append([]chatstore.TurnSnapshot(nil), f.items...), nil
}

func (f *fakeTurnStore) LoadLatestTurn(context.Context, string, string) (*chatstore.TurnSnapshot, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeTurnStore) Close() error { return nil }
