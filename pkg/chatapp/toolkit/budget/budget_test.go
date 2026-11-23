package budget_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/budget"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func ctxFor(sid string) context.Context {
	return gepsession.WithSessionMeta(context.Background(), sid, "inference-1")
}

func TestSessionTokens(t *testing.T) {
	store := sessions.NewMemoryStore()
	require.NoError(t, store.Create(context.Background(), sessions.Info{ID: "s", Usage: sessions.Usage{Input: 90, Output: 9}}))
	guard := budget.SessionTokens(store, 100)
	require.NoError(t, guard(ctxFor("s"), nil, 0), "99 of 100")
	require.NoError(t, store.Update(context.Background(), "s", func(i *sessions.Info) error { i.Usage.Output = 10; return nil }))
	err := guard(ctxFor("s"), nil, 1)
	var quota budget.QuotaExceededError
	require.ErrorAs(t, err, &quota)
	require.Equal(t, budget.QuotaExceededError{Used: 100, Limit: 100}, quota)
	var coded chatapp.CodedError
	require.ErrorAs(t, err, &coded)
	require.Equal(t, budget.CodeQuotaExceeded, coded.Code())

	require.NoError(t, guard(ctxFor("unknown"), nil, 0), "a session missing from the store is not limited")
	require.NoError(t, guard(context.Background(), nil, 0), "no session id")
	require.NoError(t, budget.SessionTokens(store, 0)(ctxFor("s"), nil, 0), "no limit")
}

func TestAllReturnsTheFirstError(t *testing.T) {
	stop := errors.New("stop")
	calls := 0
	count := func(context.Context, *turns.Turn, int) error { calls++; return nil }
	fail := func(context.Context, *turns.Turn, int) error { return stop }
	require.ErrorIs(t, budget.All(count, nil, fail, count)(context.Background(), nil, 0), stop)
	require.Equal(t, 1, calls)
}

// spendingEngine asks for a tool on every call and reports 60 tokens of usage.
type spendingEngine struct {
	mu    sync.Mutex
	calls int
}

func (e *spendingEngine) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	e.mu.Lock()
	e.calls++
	n := e.calls
	e.mu.Unlock()
	usage := &gepevents.Usage{InputTokens: 50, OutputTokens: 10}
	gepevents.PublishEventToContext(ctx, gepevents.NewProviderCallFinishedEvent(gepevents.EventMetadata{}, gepevents.Correlation{RunID: "r", ProviderCallID: fmt.Sprintf("p%d", n)}, "tool_use", "", usage, nil, true))
	turns.AppendBlock(t, turns.NewToolCallBlock(fmt.Sprintf("call-%d", n), "noop", map[string]any{}))
	return t, nil
}

func TestABudgetStopsAChatappRunBetweenRounds(t *testing.T) {
	ctx := context.Background()
	store := sessions.NewMemoryStore()
	require.NoError(t, store.Create(ctx, sessions.Info{ID: "s1"}))
	var failed []map[string]any
	var mu sync.Mutex
	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{EngineOptions: []chatapp.Option{
		chatapp.WithUsageObserver(sessions.RecordUsage(store)),
		chatapp.WithHooks(chatapp.Hooks{OnBackendEvent: func(_, name string, payload map[string]any) {
			if name == chatapp.EventChatRunFailed {
				mu.Lock()
				failed = append(failed, payload)
				mu.Unlock()
			}
		}}),
	}})
	require.NoError(t, err)
	defer func() { _ = runner.Close() }()

	reg := geptools.NewInMemoryToolRegistry()
	def, err := geptools.NewToolFromFunc("noop", "does nothing", func(struct{}) (string, error) { return "ok", nil })
	require.NoError(t, err)
	require.NoError(t, reg.RegisterTool("noop", *def))
	eng := &spendingEngine{}
	require.NoError(t, runner.Service.SubmitPromptRequest(ctx, "s1", chatapp.PromptRequest{Prompt: "spend", Runtime: &infruntime.ComposedRuntime{
		Engine:     eng,
		Registry:   reg,
		Loop:       &toolloop.LoopConfig{MaxIterations: 10},
		RoundGuard: budget.SessionTokens(store, 100),
	}}))
	require.NoError(t, runner.Service.WaitIdle(ctx, sessionstream.SessionId("s1")))

	// Round 0: 0 used. Round 1: 60 used. Round 2: 120 used, stopped before the call.
	require.Equal(t, 2, eng.calls)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, failed, 1)
	require.Equal(t, budget.CodeQuotaExceeded, failed[0]["code"])
}
