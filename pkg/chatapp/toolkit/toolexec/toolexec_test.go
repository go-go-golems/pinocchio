package toolexec

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/stretchr/testify/require"
)

// fakeExecutor returns result, or blocks until its context ends when block is set.
type fakeExecutor struct {
	result any
	block  bool
}

func (f fakeExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, _ geptools.ToolRegistry) (*geptools.ToolResult, error) {
	if f.block {
		<-ctx.Done()
		return &geptools.ToolResult{ID: call.ID, Error: "execution cancelled"}, nil
	}
	return &geptools.ToolResult{ID: call.ID, Result: f.result}, nil
}

func (f fakeExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, reg geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	out := []*geptools.ToolResult{}
	for _, c := range calls {
		r, _ := f.ExecuteToolCall(ctx, c, reg)
		out = append(out, r)
	}
	return out, nil
}

func TestDeadlineStopsASlowCallWithTheCallersMessage(t *testing.T) {
	exec := Deadline(20*time.Millisecond, func(call geptools.ToolCall, ran time.Duration) string {
		return "too slow: " + call.Name
	})(fakeExecutor{block: true})
	res, err := exec.ExecuteToolCall(context.Background(), geptools.ToolCall{ID: "c1", Name: "run_tests"}, nil)
	require.NoError(t, err)
	require.Equal(t, "too slow: run_tests", res.Error)
	require.Nil(t, res.Result)
}

func TestDeadlineKeepsTheResultWhenTheRunIsStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	exec := Deadline(time.Minute, nil)(fakeExecutor{block: true})
	res, err := exec.ExecuteToolCall(ctx, geptools.ToolCall{ID: "c1", Name: "run_tests"}, nil)
	require.NoError(t, err)
	require.Equal(t, "execution cancelled", res.Error, "a stop is not a deadline")
}

func TestDefaultDeadlineMessage(t *testing.T) {
	require.Equal(t, "the eval tool call was stopped after 2m0s (time limit)", DefaultDeadlineMessage(geptools.ToolCall{Name: "eval"}, 2*time.Minute))
}

func TestCutResults(t *testing.T) {
	cut := CutResults(5, func(shown, total int) string { return "|" + strings.Repeat("+", total-shown) })
	res, err := cut(fakeExecutor{result: "héllo world"}).ExecuteToolCall(context.Background(), geptools.ToolCall{ID: "c"}, nil)
	require.NoError(t, err)
	require.Equal(t, "héllo|++++++", res.Result, "strings are cut as text, by characters")

	res, err = cut(fakeExecutor{result: map[string]any{"a": 1}}).ExecuteToolCall(context.Background(), geptools.ToolCall{ID: "c"}, nil)
	require.NoError(t, err)
	require.Equal(t, `{"a":|++`, res.Result, "other values are cut as JSON")

	res, err = cut(fakeExecutor{result: "short"}).ExecuteToolCall(context.Background(), geptools.ToolCall{ID: "c"}, nil)
	require.NoError(t, err)
	require.Equal(t, "short", res.Result)

	results, err := CutResults(3, nil)(fakeExecutor{result: "abcdef"}).ExecuteToolCalls(context.Background(), []geptools.ToolCall{{ID: "a"}, {ID: "b"}}, nil)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "abc\n[result cut: 3 of 6 characters shown]", results[1].Result)
}

func TestRunToolsWrapsTheRunsExecutorOrTheDefault(t *testing.T) {
	ctx := context.Background()
	_, wrapped, err := RunTools(CutResults(2, nil))(ctx, chatapp.RunScope{}, chatapp.RunTools{Executor: fakeExecutor{result: "abcd"}})
	require.NoError(t, err)
	res, err := wrapped.Executor.ExecuteToolCall(ctx, geptools.ToolCall{ID: "c"}, nil)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(res.Result.(string), "ab\n"))

	_, defaulted, err := RunTools(Deadline(time.Second, nil))(ctx, chatapp.RunScope{}, chatapp.RunTools{Config: geptools.DefaultToolConfig()})
	require.NoError(t, err)
	require.NotNil(t, defaulted.Executor, "a nil executor is replaced by geppetto's default, wrapped")
}

func TestChainOrder(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next geptools.ToolExecutor) geptools.ToolExecutor {
			return recordingExecutor{name: name, next: next, order: &order}
		}
	}
	_, err := Chain(fakeExecutor{}, mw("outer"), mw("inner")).ExecuteToolCall(context.Background(), geptools.ToolCall{}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"outer", "inner"}, order)
}

type recordingExecutor struct {
	name  string
	next  geptools.ToolExecutor
	order *[]string
}

func (r recordingExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, reg geptools.ToolRegistry) (*geptools.ToolResult, error) {
	*r.order = append(*r.order, r.name)
	return r.next.ExecuteToolCall(ctx, call, reg)
}

func (r recordingExecutor) ExecuteToolCalls(context.Context, []geptools.ToolCall, geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	return nil, errors.New("unused")
}

// waitingExecutor waits `wait` inside AwaitPerson, then answers "ok" (or the
// context's error).
type waitingExecutor struct{ wait time.Duration }

func (w waitingExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, _ geptools.ToolRegistry) (*geptools.ToolResult, error) {
	release := AwaitPerson(ctx)
	defer release()
	select {
	case <-time.After(w.wait):
		return &geptools.ToolResult{ID: call.ID, Result: "ok"}, nil
	case <-ctx.Done():
		return &geptools.ToolResult{ID: call.ID, Error: "cancelled"}, nil
	}
}

func (w waitingExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, reg geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	return nil, nil
}

func personDeadline(limit, person time.Duration, states *[]string) Middleware {
	var mu sync.Mutex
	return DeadlineBy(func(context.Context, geptools.ToolCall) time.Duration { return limit }, func(call geptools.ToolCall, _ time.Duration) string { return "too slow" },
		WithPersonLimit(func(context.Context, geptools.ToolCall) time.Duration { return person }),
		OnState(func(_ context.Context, call geptools.ToolCall, state string, d time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			*states = append(*states, call.ID+" "+state+" "+d.String())
		}))
}

func TestACallThatWaitsForAPersonGetsThePersonLimit(t *testing.T) {
	var states []string
	exec := personDeadline(20*time.Millisecond, time.Second, &states)(waitingExecutor{wait: 80 * time.Millisecond})
	res, err := exec.ExecuteToolCall(context.Background(), geptools.ToolCall{ID: "c1", Name: "eval"}, nil)
	require.NoError(t, err)
	require.Equal(t, "ok", res.Result, "waiting longer than the ordinary limit is fine while a person decides")
	require.Equal(t, []string{"c1 awaiting-person 1s", "c1 running 20ms"}, states)
}

func TestAPersonWaitHasItsOwnLimit(t *testing.T) {
	var states []string
	exec := personDeadline(time.Second, 20*time.Millisecond, &states)(waitingExecutor{wait: time.Second})
	res, err := exec.ExecuteToolCall(context.Background(), geptools.ToolCall{ID: "c2", Name: "eval"}, nil)
	require.NoError(t, err)
	require.Equal(t, "too slow", res.Error)
	require.Nil(t, res.Result)
}

func TestAwaitPersonWithoutADeadlineDoesNothing(t *testing.T) {
	release := AwaitPerson(context.Background())
	release()
	release()
}
