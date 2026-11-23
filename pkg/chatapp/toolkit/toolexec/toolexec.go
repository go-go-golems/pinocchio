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
	"sync"
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
// wait for a person); a limit <= 0 means none for that call. Options add a
// limit for the time a call waits for a person (WithPersonLimit, AwaitPerson)
// and a callback when that starts and ends (OnState).
func DeadlineBy(limit func(ctx context.Context, call geptools.ToolCall) time.Duration, message DeadlineMessage, opts ...DeadlineOption) Middleware {
	if message == nil {
		message = DefaultDeadlineMessage
	}
	var o deadlineOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return func(next geptools.ToolExecutor) geptools.ToolExecutor {
		return &deadlineExecutor{next: next, limit: limit, message: message, opts: o}
	}
}

// DeadlineOption configures DeadlineBy.
type DeadlineOption func(*deadlineOptions)

type deadlineOptions struct {
	person  func(ctx context.Context, call geptools.ToolCall) time.Duration
	onState func(ctx context.Context, call geptools.ToolCall, state string, limit time.Duration)
}

// WithPersonLimit is the limit while a call waits for a person (between
// AwaitPerson and its release). Without it, waiting does not change the limit.
func WithPersonLimit(limit func(ctx context.Context, call geptools.ToolCall) time.Duration) DeadlineOption {
	return func(o *deadlineOptions) { o.person = limit }
}

// OnState is called when a call starts waiting for a person (state
// "awaiting-person") and when it stops ("running"), with the limit that now
// applies. agentserver publishes it as ChatToolCallProgress.
func OnState(fn func(ctx context.Context, call geptools.ToolCall, state string, limit time.Duration)) DeadlineOption {
	return func(o *deadlineOptions) { o.onState = fn }
}

// States reported to OnState.
const (
	StateAwaitingPerson = "awaiting-person"
	StateRunning        = "running"
)

// AwaitPerson says that the tool call running in ctx now waits for a person
// (an approval, say). Until release is called, the call's limit is the person
// limit (WithPersonLimit); after it, the ordinary limit starts again from
// zero. Waits may nest; only the outermost changes the limit. Without a
// deadline in ctx it does nothing, so a tool may call it unconditionally.
func AwaitPerson(ctx context.Context) func() {
	c, _ := ctx.Value(callControlKey{}).(*callControl)
	if c == nil {
		return func() {}
	}
	c.await()
	var once sync.Once
	return func() { once.Do(c.release) }
}

type callControlKey struct{}

// callControl is one call's timer: the ordinary limit, or the person limit
// while the call waits for a person.
type callControl struct {
	mu      sync.Mutex
	timer   *time.Timer
	waits   int
	ctx     context.Context
	call    geptools.ToolCall
	limit   time.Duration
	person  time.Duration
	onState func(ctx context.Context, call geptools.ToolCall, state string, limit time.Duration)
}

func (c *callControl) await() {
	c.mu.Lock()
	c.waits++
	first := c.waits == 1
	if first {
		c.timer.Reset(c.person)
	}
	c.mu.Unlock()
	if first && c.onState != nil {
		c.onState(c.ctx, c.call, StateAwaitingPerson, c.person)
	}
}

func (c *callControl) release() {
	c.mu.Lock()
	c.waits--
	last := c.waits == 0
	if last {
		c.timer.Reset(c.limit)
	}
	c.mu.Unlock()
	if last && c.onState != nil {
		c.onState(c.ctx, c.call, StateRunning, c.limit)
	}
}

type deadlineExecutor struct {
	next    geptools.ToolExecutor
	limit   func(ctx context.Context, call geptools.ToolCall) time.Duration
	message DeadlineMessage
	opts    deadlineOptions
}

func (e *deadlineExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, registry geptools.ToolRegistry) (*geptools.ToolResult, error) {
	d := e.limit(ctx, call)
	if d <= 0 {
		return e.next.ExecuteToolCall(ctx, call, registry)
	}
	person := d
	if e.opts.person != nil {
		if p := e.opts.person(ctx, call); p > 0 {
			person = p
		}
	}
	started := time.Now()
	callCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	ctl := &callControl{ctx: ctx, call: call, limit: d, person: person, onState: e.opts.onState}
	ctl.timer = time.AfterFunc(d, func() { cancel(errDeadline) })
	defer ctl.timer.Stop()
	res, err := e.next.ExecuteToolCall(context.WithValue(callCtx, callControlKey{}, ctl), call, registry)
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
