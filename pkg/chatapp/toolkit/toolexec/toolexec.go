// Package toolexec has optional wrappers for geppetto tool executors: a
// per-call deadline and a cap on result size. They are mechanism only; the
// texts the model reads when a wrapper acts are the caller's (with short
// neutral defaults).
//
// Wrap a run's final executor with RunTools, so the wrappers also cover
// browser tools (frontendtools routes those before the runtime's executor):
//
//	chatapp.WithRunTools(
//	    manager.RunTools(), // browser tools first
//	    toolexec.RunTools(toolexec.Deadline(2*time.Minute, nil), toolexec.CutResults(9000, nil)),
//	)
//
// With httpapi, pass the second function through httpapi.WithEngineOptions;
// httpapi.WithFrontendTools installs the first.
package toolexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
)

// Middleware wraps a tool executor.
type Middleware func(next geptools.ToolExecutor) geptools.ToolExecutor

// Chain wraps next with the middlewares; the first one is the outermost.
func Chain(next geptools.ToolExecutor, mws ...Middleware) geptools.ToolExecutor {
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] != nil {
			next = mws[i](next)
		}
	}
	return next
}

// Compose returns one middleware applying mws (the first is the outermost);
// it fits infruntime.ComposedRuntime.WrapToolExecutor.
func Compose(mws ...Middleware) Middleware {
	return func(next geptools.ToolExecutor) geptools.ToolExecutor { return Chain(next, mws...) }
}

// RunTools returns a chatapp.RunToolsFunc that wraps each run's executor (or
// geppetto's default one, built from the run's tool config) with mws.
func RunTools(mws ...Middleware) chatapp.RunToolsFunc {
	return func(ctx context.Context, _ chatapp.RunScope, tools chatapp.RunTools) (context.Context, chatapp.RunTools, error) {
		tools.Executor = Chain(tools.ExecutorOrDefault(), mws...)
		return ctx, tools, nil
	}
}

// DeadlineMessage is the tool error the model reads when a call ran out of
// time.
type DeadlineMessage func(call geptools.ToolCall, ran time.Duration) string

// DefaultDeadlineMessage says which tool was stopped and after how long.
func DefaultDeadlineMessage(call geptools.ToolCall, ran time.Duration) string {
	return fmt.Sprintf("the %s tool call was stopped after %s (time limit)", call.Name, ran.Round(time.Second))
}

var errDeadline = errors.New("toolexec: tool call deadline")

// Deadline cancels a call's context after d and replaces its result with a
// tool error (message, or DefaultDeadlineMessage). A call stopped because the
// run was stopped keeps the executor's result. A batch runs one call at a
// time, each with its own deadline.
func Deadline(d time.Duration, message DeadlineMessage) Middleware {
	return DeadlineBy(func(context.Context, geptools.ToolCall) time.Duration { return d }, message)
}

// DeadlineBy is Deadline with a limit per call (e.g. longer for tools that
// wait for a person); a limit <= 0 means none for that call.
func DeadlineBy(limit func(ctx context.Context, call geptools.ToolCall) time.Duration, message DeadlineMessage) Middleware {
	if message == nil {
		message = DefaultDeadlineMessage
	}
	return func(next geptools.ToolExecutor) geptools.ToolExecutor {
		return &deadlineExecutor{next: next, limit: limit, message: message}
	}
}

type deadlineExecutor struct {
	next    geptools.ToolExecutor
	limit   func(ctx context.Context, call geptools.ToolCall) time.Duration
	message DeadlineMessage
}

func (e *deadlineExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, registry geptools.ToolRegistry) (*geptools.ToolResult, error) {
	d := e.limit(ctx, call)
	if d <= 0 {
		return e.next.ExecuteToolCall(ctx, call, registry)
	}
	started := time.Now()
	callCtx, cancel := context.WithTimeoutCause(ctx, d, errDeadline)
	defer cancel()
	res, err := e.next.ExecuteToolCall(callCtx, call, registry)
	if ctx.Err() == nil && errors.Is(context.Cause(callCtx), errDeadline) {
		if res == nil {
			res = &geptools.ToolResult{ID: call.ID}
		}
		res.Result, res.Error = nil, e.message(call, time.Since(started))
		return res, nil
	}
	return res, err
}

func (e *deadlineExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, registry geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	out := make([]*geptools.ToolResult, 0, len(calls))
	for _, call := range calls {
		res, err := e.ExecuteToolCall(ctx, call, registry)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	return out, nil
}

// CutNote is appended to a result that was cut.
type CutNote func(shown, total int) string

// DefaultCutNote says how much of the result is shown.
func DefaultCutNote(shown, total int) string {
	return fmt.Sprintf("\n[result cut: %d of %d characters shown]", shown, total)
}

// CutResults limits a result to maxChars characters: a longer result becomes
// a string with its first maxChars characters and note (or DefaultCutNote).
// A string result is cut as text; any other value is cut as its JSON.
func CutResults(maxChars int, note CutNote) Middleware {
	if note == nil {
		note = DefaultCutNote
	}
	return func(next geptools.ToolExecutor) geptools.ToolExecutor {
		return &cutExecutor{next: next, max: maxChars, note: note}
	}
}

type cutExecutor struct {
	next geptools.ToolExecutor
	max  int
	note CutNote
}

func (e *cutExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, registry geptools.ToolRegistry) (*geptools.ToolResult, error) {
	res, err := e.next.ExecuteToolCall(ctx, call, registry)
	e.cut(res)
	return res, err
}

func (e *cutExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, registry geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	results, err := e.next.ExecuteToolCalls(ctx, calls, registry)
	for _, res := range results {
		e.cut(res)
	}
	return results, err
}

func (e *cutExecutor) cut(res *geptools.ToolResult) {
	if res == nil || res.Result == nil || e.max <= 0 {
		return
	}
	text, ok := res.Result.(string)
	if !ok {
		b, err := json.Marshal(res.Result)
		if err != nil {
			return
		}
		text = string(b)
	}
	total := utf8.RuneCountInString(text)
	if total <= e.max {
		return
	}
	runes := []rune(text)
	res.Result = string(runes[:e.max]) + e.note(e.max, total)
}
