package sessions_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func stores(t *testing.T) map[string]sessions.Store {
	t.Helper()
	sq, err := sessions.NewSQLiteStore("file:" + filepath.Join(t.TempDir(), "sessions.db") + "?_busy_timeout=5000")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sq.Close() })
	return map[string]sessions.Store{"memory": sessions.NewMemoryStore(), "sqlite": sq}
}

func TestStoreContract(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			base := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
			require.NoError(t, store.Create(ctx, sessions.Info{ID: "a", Owner: "alice", Labels: map[string]string{"room": "main"}, Created: base, Data: []byte(`{"instructions":"be brief"}`)}))
			require.NoError(t, store.Create(ctx, sessions.Info{ID: "b", Owner: "bob", Labels: map[string]string{"room": "main"}, Created: base.Add(time.Minute)}))
			require.NoError(t, store.Create(ctx, sessions.Info{ID: "c", Owner: "alice", Labels: map[string]string{"room": "t-1"}, Created: base.Add(2 * time.Minute)}))
			require.ErrorIs(t, store.Create(ctx, sessions.Info{ID: "a"}), sessions.ErrExists)

			got, err := store.Get(ctx, "a")
			require.NoError(t, err)
			require.Equal(t, "alice", got.Owner)
			require.Equal(t, "main", got.Labels["room"])
			require.True(t, got.Created.Equal(base))
			require.JSONEq(t, `{"instructions":"be brief"}`, string(got.Data))
			_, err = store.Get(ctx, "missing")
			require.ErrorIs(t, err, sessions.ErrNotFound)

			all, err := store.List(ctx, sessions.Filter{})
			require.NoError(t, err)
			require.Equal(t, []string{"c", "b", "a"}, ids(all), "newest first")
			mine, err := store.List(ctx, sessions.Filter{Owner: "alice"})
			require.NoError(t, err)
			require.Equal(t, []string{"c", "a"}, ids(mine))
			main, err := store.List(ctx, sessions.Filter{Labels: map[string]string{"room": "main"}})
			require.NoError(t, err)
			require.Equal(t, []string{"b", "a"}, ids(main))
			one, err := store.List(ctx, sessions.Filter{Limit: 1})
			require.NoError(t, err)
			require.Equal(t, []string{"c"}, ids(one))

			require.NoError(t, store.Update(ctx, "a", func(info *sessions.Info) error {
				info.Title = "first"
				info.Usage = info.Usage.Add(sessions.Usage{Input: 10, Output: 2, Calls: 1})
				info.ID = "renamed" // ignored
				return nil
			}))
			got, err = store.Get(ctx, "a")
			require.NoError(t, err)
			require.Equal(t, "first", got.Title)
			require.Equal(t, 12, got.Usage.Tokens())
			require.JSONEq(t, `{"instructions":"be brief"}`, string(got.Data), "data survives updates")
			require.True(t, got.Updated.After(got.Created) || got.Updated.Equal(got.Created))
			require.ErrorIs(t, store.Update(ctx, "missing", func(*sessions.Info) error { return nil }), sessions.ErrNotFound)

			require.NoError(t, store.Delete(ctx, "a"))
			require.ErrorIs(t, store.Delete(ctx, "a"), sessions.ErrNotFound)
		})
	}
}

func ids(infos []sessions.Info) []string {
	out := make([]string, 0, len(infos))
	for _, info := range infos {
		out = append(out, info.ID)
	}
	return out
}

func TestTitleFrom(t *testing.T) {
	require.Equal(t, "hello there", sessions.TitleFrom("\n  hello there  \nsecond line", 80))
	require.Equal(t, "héllo…", sessions.TitleFrom("héllo world", 5))
}

// usageEngine reports one provider call with usage and answers.
type usageEngine struct{}

func (usageEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	meta := gepevents.EventMetadata{SessionID: "sid"}
	usage := &gepevents.Usage{InputTokens: 100, OutputTokens: 7, CachedTokens: 80}
	gepevents.PublishEventToContext(ctx, gepevents.NewProviderCallFinishedEvent(meta, gepevents.Correlation{SessionID: "sid", RunID: "r", ProviderCallID: "p"}, "stop", "", usage, nil, false))
	turns.AppendBlock(t, turns.NewAssistantTextBlock("ok"))
	return t, nil
}

func TestRecordUsageAddsEachProviderCall(t *testing.T) {
	ctx := context.Background()
	store := sessions.NewMemoryStore()
	require.NoError(t, store.Create(ctx, sessions.Info{ID: "s1"}))
	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{EngineOptions: []chatapp.Option{chatapp.WithUsageObserver(sessions.RecordUsage(store))}})
	require.NoError(t, err)
	defer func() { _ = runner.Close() }()

	for i := 0; i < 2; i++ {
		require.NoError(t, runner.Service.SubmitPromptRequest(ctx, "s1", chatapp.PromptRequest{Prompt: "hi", Runtime: &infruntime.ComposedRuntime{Engine: usageEngine{}}}))
		require.NoError(t, runner.Service.WaitIdle(ctx, sessionstream.SessionId("s1")))
	}

	info, err := store.Get(ctx, "s1")
	require.NoError(t, err)
	require.Equal(t, sessions.Usage{Input: 200, Output: 14, Cached: 160, Calls: 2}, info.Usage)
}
