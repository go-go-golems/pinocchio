package agentd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/google/uuid"
)

// ContextPrefix starts the text of a turn's context block.
const ContextPrefix = "[context]\n"

// Session is one conversation with one agent and one model.
type Session struct {
	ID      string
	Labels  map[string]string
	Agent   *AgentConfig
	Model   modelsource.Model
	Created time.Time

	srv      *Server
	newEng   func() (engine.Engine, error)
	gs       *gepsession.Session
	log      *EventLog
	system   string
	thinking string

	mu         sync.Mutex
	title      string
	updated    time.Time
	usage      Usage
	contextIDs []string
	active     *activeRun
	calls      map[string]*callDeadline
}

type activeRun struct {
	id        string
	handle    *gepsession.ExecutionHandle
	cancelled bool
}

// FinalRoundInstruction is added (as a hidden context block) when a turn has
// used all its rounds; the model then answers once more with tools disabled
// (geppetto's toolloop FinalAnswer).
const FinalRoundInstruction = "You have used all %d tool rounds of this turn. Do not call tools. Answer the person now: " +
	"what you found, what is still running (with ids), and what you would do next. Do not claim anything you did not check."

// errStepTimeout is the cancel cause of a tool call that ran out of time. It
// wraps context.DeadlineExceeded so that code below the executor that checks
// context.Cause (frontendtools reports status "timeout") sees a deadline, even
// though the resettable limit is a cancel and ctx.Err() is context.Canceled.
var errStepTimeout = fmt.Errorf("tool step timed out: %w", context.DeadlineExceeded)

// callDeadline is the resettable time limit of one running tool call.
type callDeadline struct {
	mu      sync.Mutex
	timer   *time.Timer
	state   string // running, awaiting-person
	started time.Time
}

// SessionView is the JSON description of a session.
type SessionView struct {
	ID       string            `json:"id"`
	Labels   map[string]string `json:"labels,omitempty"`
	Agent    string            `json:"agent"`
	Model    modelsource.Model `json:"model"`
	Title    string            `json:"title,omitempty"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
	Running  string            `json:"running,omitempty"`
	Usage    Usage             `json:"usage"`
	Seq      int64             `json:"seq"`
	Limits   Limits            `json:"limits"`
	Messages []MessageView     `json:"messages,omitempty"`
}

// Limits are the agent limits a page should know.
type Limits struct {
	MaxRounds       int   `json:"maxRounds"`
	ToolTimeoutMs   int64 `json:"toolTimeoutMs"`
	PersonTimeoutMs int64 `json:"personTimeoutMs"`
	ResultCut       int   `json:"resultCut"`
	TokenBudget     int   `json:"tokenBudget,omitempty"`
}

// MessageView is one block of the conversation, for hydrating a page.
type MessageView struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"` // user, assistant, reasoning, tool_call, tool_result, context
	Text   string `json:"text,omitempty"`
	CallID string `json:"callId,omitempty"`
	Name   string `json:"name,omitempty"`
	Input  any    `json:"input,omitempty"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *Session) limits() Limits {
	return Limits{MaxRounds: s.Agent.MaxRounds, ToolTimeoutMs: s.Agent.ToolTimeoutDuration().Milliseconds(), PersonTimeoutMs: s.Agent.PersonTimeoutDuration().Milliseconds(), ResultCut: s.Agent.ResultCut, TokenBudget: s.Agent.Budget.TokensPerSession}
}

// View describes the session; withMessages includes the conversation.
func (s *Session) View(withMessages bool) SessionView {
	s.mu.Lock()
	v := SessionView{ID: s.ID, Labels: s.Labels, Agent: s.Agent.Name, Model: s.Model, Title: s.title, Created: s.Created, Updated: s.updated, Usage: s.usage, Limits: s.limits()}
	if s.active != nil {
		v.Running = s.active.id
	}
	s.mu.Unlock()
	v.Seq = s.log.Seq()
	if withMessages {
		v.Messages = messagesOf(s.gs.Latest())
	}
	return v
}

func messagesOf(t *turns.Turn) []MessageView {
	if t == nil {
		return nil
	}
	out := []MessageView{}
	for _, b := range t.Blocks {
		m := MessageView{ID: b.ID}
		switch b.Kind {
		case turns.BlockKindSystem:
			continue
		case turns.BlockKindUser:
			m.Kind, m.Text = "user", str(b.Payload, turns.PayloadKeyText)
			if strings.HasPrefix(m.Text, ContextPrefix) {
				m.Kind, m.Text = "context", strings.TrimPrefix(m.Text, ContextPrefix)
			}
		case turns.BlockKindLLMText:
			m.Kind, m.Text = "assistant", str(b.Payload, turns.PayloadKeyText)
		case turns.BlockKindReasoning:
			m.Kind, m.Text = "reasoning", str(b.Payload, turns.PayloadKeyText)
		case turns.BlockKindToolCall:
			m.Kind, m.CallID, m.Name, m.Input = "tool_call", str(b.Payload, turns.PayloadKeyID), str(b.Payload, turns.PayloadKeyName), b.Payload[turns.PayloadKeyArgs]
		case turns.BlockKindToolUse:
			m.Kind, m.CallID, m.Result, m.Error = "tool_result", str(b.Payload, turns.PayloadKeyID), b.Payload[turns.PayloadKeyResult], str(b.Payload, turns.PayloadKeyError)
		case turns.BlockKindOther:
			continue
		default:
			continue
		}
		out = append(out, m)
	}
	return out
}

func (s *Session) addUsage(u *gepevents.Usage) (Usage, Usage) {
	var round Usage
	round.add(u)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage.add(u)
	if c := s.Model.Cost; c != nil {
		usd := costUSD(s.usage, c.Input, c.Output, c.CacheRead, c.CacheWrite)
		s.usage.USD = &usd
	}
	return round, s.usage
}

// costUSD prices usage per million tokens. OpenAI-style providers report
// cached prompt tokens inside Input (as Cached); Claude reports cache reads
// and writes separately from Input.
func costUSD(u Usage, in, out, cacheRead, cacheWrite float64) float64 {
	uncached := u.Input - u.Cached
	if uncached < 0 {
		uncached = 0
	}
	return (float64(uncached)*in + float64(u.Cached+u.CacheRead)*cacheRead + float64(u.CacheCreation)*cacheWrite + float64(u.Output)*out) / 1e6
}

// ErrRunActive is returned when a turn is sent while a run is in progress.
var ErrRunActive = errors.New("a run is already active in this session")

// ErrQuota is returned when the session's token budget is used up.
var ErrQuota = errors.New("the session's token budget is used up")

// StartTurn appends the user's text (and the page's context) and starts a run.
func (s *Session) StartTurn(text, contextText string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("text is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return "", ErrRunActive
	}
	if b := s.Agent.Budget.TokensPerSession; b > 0 && s.usage.Input+s.usage.Output >= b {
		return "", ErrQuota
	}
	eng, err := s.newEng()
	if err != nil {
		return "", err
	}
	reg := geptools.NewInMemoryToolRegistry()
	if err := s.srv.tools.RegisterManifestTools(sessionstream.SessionId(s.ID), reg); err != nil {
		return "", err
	}
	if err := s.srv.registerHostTools(s.Agent, reg); err != nil {
		return "", err
	}
	var registry geptools.ToolRegistry
	if len(reg.ListTools()) > 0 {
		registry = reg
	}
	loopCfg := toolloop.DefaultLoopConfig().WithMaxIterations(s.Agent.MaxRounds).
		WithFinalAnswer(toolloop.FinalAnswer{Prompt: s.finalPrompt, RefusedToolResult: RefusedToolMessage})
	toolCfg := geptools.DefaultToolConfig()
	toolCfg.ExecutionTimeout = s.Agent.ToolTimeoutDuration() + 5*time.Second
	toolCfg.MaxParallelTools = 1
	toolCfg.RetryConfig.MaxRetries = 0
	runID := uuid.NewString()
	sink := &runSink{s: s, run: runID}
	s.gs.Builder = &enginebuilder.Builder{
		Base:         eng,
		Registry:     registry,
		LoopConfig:   &loopCfg,
		ToolConfig:   &toolCfg,
		ToolExecutor: &boundedExecutor{s: s, inner: &hostExecutor{s: s, next: frontendtools.NewBridgeExecutor(s.srv.tools, missingTool{})}, cut: s.Agent.ResultCut, timeout: s.Agent.ToolTimeoutDuration()},
		EventSinks:   []gepevents.EventSink{sink},
		RoundGuard:   s.budgetGuard,
	}
	s.appendTurnLocked(text, contextText)
	if s.title == "" {
		s.title = firstLine(text, 80)
	}
	ctx := frontendtools.WithBridgeContext(context.Background(), frontendtools.BridgeContext{
		SessionID: sessionstream.SessionId(s.ID), MessageID: runID, Publisher: toolPublisher{s},
	})
	s.log.Append(EvUserMessage, runID, map[string]any{"text": text})
	handle, err := s.gs.StartInference(ctx)
	if err != nil {
		return "", err
	}
	s.active = &activeRun{id: runID, handle: handle}
	s.updated = time.Now().UTC()
	s.log.Append(EvRunStarted, runID, map[string]any{"model": s.Model.Ref, "tools": toolNames(registry)})
	go s.wait(runID, handle, sink)
	return runID, nil
}

func toolNames(r geptools.ToolRegistry) []string {
	if r == nil {
		return []string{}
	}
	out := []string{}
	for _, t := range r.ListTools() {
		out = append(out, t.Name)
	}
	return out
}

// appendTurnLocked clones the latest turn (the whole conversation), drops the
// previous turns' context blocks, and appends the user text and new context.
func (s *Session) appendTurnLocked(text, contextText string) {
	seed := &turns.Turn{}
	if base := s.gs.Latest(); base != nil {
		seed = base.Clone()
	} else if strings.TrimSpace(s.system) != "" {
		turns.AppendBlock(seed, turns.NewSystemTextBlock(s.system))
	}
	if len(s.contextIDs) > 0 {
		stale := map[string]bool{}
		for _, id := range s.contextIDs {
			stale[id] = true
		}
		kept := seed.Blocks[:0]
		for _, b := range seed.Blocks {
			if !stale[b.ID] {
				kept = append(kept, b)
			}
		}
		seed.Blocks = kept
		s.contextIDs = nil
	}
	turns.AppendBlock(seed, turns.NewUserTextBlock(text))
	if c := strings.TrimSpace(contextText); c != "" {
		b := turns.NewUserTextBlock(ContextPrefix + c)
		turns.AppendBlock(seed, b)
		s.contextIDs = append(s.contextIDs, b.ID)
	}
	seed.ID = uuid.NewString()
	s.gs.Append(seed)
}

func (s *Session) wait(runID string, h *gepsession.ExecutionHandle, sink *runSink) {
	out, err := h.Wait()
	s.mu.Lock()
	cancelled := s.active != nil && s.active.cancelled
	s.active = nil
	s.updated = time.Now().UTC()
	s.mu.Unlock()
	text := sink.answer()
	if text == "" {
		text = lastAssistantText(out)
	}
	ev, data := runOutcome(err, cancelled)
	data["text"] = text
	if ev == EvRunFailed {
		log.Warn().Err(err).Str("session", s.ID).Str("run", runID).Any("code", data["code"]).Msg("agentd run failed")
	}
	s.log.Append(ev, runID, data)
	if s.srv.store != nil {
		if err := s.srv.store.Save(s); err != nil {
			log.Warn().Err(err).Str("session", s.ID).Msg("agentd: save session")
		}
	}
}

// runOutcome is the one place that decides how a run ended, from the tool
// loop's error: the terminal event and its data (without the answer text).
//
//   - cancelled (by the client, or the context): run.cancelled
//   - no error: run.finished
//   - the token budget stopped the loop between rounds: run.failed quota_exceeded
//   - all rounds used and the final answer given: run.finished, truncated, max_rounds
//   - anything else, including a failed final answer: run.failed with the
//     provider error's code
func runOutcome(err error, cancelled bool) (string, map[string]any) {
	var limit *toolloop.MaxIterationsError
	switch {
	case cancelled || errors.Is(err, context.Canceled):
		return EvRunCancelled, map[string]any{}
	case err == nil:
		return EvRunFinished, map[string]any{"truncated": false}
	case errors.Is(err, ErrQuota):
		return EvRunFailed, map[string]any{"code": "quota_exceeded", "message": ErrQuota.Error() + "; the answer so far is above."}
	case errors.As(err, &limit) && limit.Answered:
		return EvRunFinished, map[string]any{"truncated": true, "reason": "max_rounds"}
	default:
		code, msg := classify(err)
		return EvRunFailed, map[string]any{"code": code, "message": msg}
	}
}

// finalPrompt builds the hidden instruction of the final answer at the round
// limit; it is a context block, so the next turn drops it.
func (s *Session) finalPrompt(maxRounds int) turns.Block {
	b := turns.NewUserTextBlock(ContextPrefix + fmt.Sprintf(FinalRoundInstruction, maxRounds))
	s.mu.Lock()
	s.contextIDs = append(s.contextIDs, b.ID)
	s.mu.Unlock()
	return b
}

// budgetGuard stops a run before its next model call once the session's
// token budget is used up (checked between rounds, not only at turn start).
func (s *Session) budgetGuard(context.Context, *turns.Turn, int) error {
	b := s.Agent.Budget.TokensPerSession
	if b <= 0 {
		return nil
	}
	s.mu.Lock()
	used := s.usage.Input + s.usage.Output
	s.mu.Unlock()
	if used >= b {
		return ErrQuota
	}
	return nil
}

// RefusedToolMessage is the tool error for calls the model makes in the final answer.
const RefusedToolMessage = "no more tool calls in this turn; answer the person"

// SetCallState changes a running tool call's time limit: "awaiting-person"
// pauses the step limit (up to the agent's person timeout) while the page
// waits for a decision; "running" restarts the step limit from now.
func (s *Session) SetCallState(callID, state string) error {
	s.mu.Lock()
	d := s.calls[callID]
	s.mu.Unlock()
	if d == nil {
		return fmt.Errorf("no running tool call %q", callID)
	}
	var limit time.Duration
	switch state {
	case "awaiting-person":
		limit = s.Agent.PersonTimeoutDuration()
	case "running":
		limit = s.Agent.ToolTimeoutDuration()
	default:
		return fmt.Errorf("state must be awaiting-person or running, got %q", state)
	}
	d.mu.Lock()
	d.state = state
	d.timer.Reset(limit)
	d.mu.Unlock()
	s.log.Append(EvToolProgress, s.runID(), map[string]any{"callId": callID, "state": state, "limitMs": limit.Milliseconds()})
	return nil
}

func (s *Session) runID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ""
	}
	return s.active.id
}

func (s *Session) trackCall(id string, d *callDeadline) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = map[string]*callDeadline{}
	}
	s.calls[id] = d
}

func (s *Session) untrackCall(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.calls, id)
}

// Cancel stops the active run, if any.
func (s *Session) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return false
	}
	s.active.cancelled = true
	s.active.handle.Cancel()
	return true
}

func lastAssistantText(t *turns.Turn) string {
	if t == nil {
		return ""
	}
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		b := t.Blocks[i]
		if b.Kind == turns.BlockKindUser {
			break
		}
		if b.Kind == turns.BlockKindLLMText {
			if v, ok := b.Payload[turns.PayloadKeyText].(string); ok {
				return v
			}
		}
	}
	return ""
}

// classify maps an engine error to a protocol code and a fixed message;
// provider diagnostics stay in the server log.
func classify(err error) (string, string) {
	m := strings.ToLower(err.Error())
	switch {
	case strings.Contains(m, "401") || strings.Contains(m, "403") || strings.Contains(m, "unauthorized") || strings.Contains(m, "credential") || strings.Contains(m, "authentication"):
		return "authentication_error", "The model provider rejected the server's credential. Log in again (pi) or fix the profile, then restart the agent service."
	case strings.Contains(m, "429") || strings.Contains(m, "rate limit") || strings.Contains(m, "rate_limit"):
		return "rate_limited", "The model provider's rate limit was reached. Wait briefly and send again."
	case strings.Contains(m, "context length") || strings.Contains(m, "too long") || strings.Contains(m, "too large") || strings.Contains(m, "maximum context"):
		return "prompt_too_large", "The conversation no longer fits the model's context. Start a new conversation."
	}
	return "upstream_error", "The model call failed. Send again, or choose another model."
}

func firstLine(s string, n int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}

// boundedExecutor adds a resettable per-call time limit and cuts large results.
type boundedExecutor struct {
	s       *Session
	inner   geptools.ToolExecutor
	cut     int
	timeout time.Duration
}

// TimeoutMessage is the tool error when a call runs out of time.
const TimeoutMessage = "The %s step ran for %s and was stopped: that is the limit for one tool call. " +
	"Work it started in the page (jobs, timers) may still be running. Do not wait in loops: " +
	"start long work, tell the person what you started, and end the turn, or check progress with a short call."

func (e *boundedExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, reg geptools.ToolRegistry) (*geptools.ToolResult, error) {
	cctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	d := &callDeadline{state: "running", started: time.Now()}
	d.timer = time.AfterFunc(e.timeout, func() { cancel(errStepTimeout) })
	defer d.timer.Stop()
	if e.s != nil {
		e.s.trackCall(call.ID, d)
		defer e.s.untrackCall(call.ID)
	}
	res, err := e.inner.ExecuteToolCall(cctx, call, reg)
	if err != nil {
		return res, err
	}
	if res == nil {
		res = &geptools.ToolResult{ID: call.ID}
	}
	if errors.Is(context.Cause(cctx), errStepTimeout) && ctx.Err() == nil {
		ran := time.Since(d.started).Round(time.Second)
		res.Result, res.Error = nil, fmt.Sprintf(TimeoutMessage, call.Name, ran)
	}
	// Pages wrap non-object results as {value: x}; the model gets x.
	res.Result = unwrapValue(res.Result)
	if res.Result != nil && e.cut > 0 {
		var text string
		if s, ok := unwrapValue(res.Result).(string); ok {
			text = s
		} else if b, err := json.Marshal(res.Result); err == nil {
			text = string(b)
		}
		if len(text) > e.cut {
			res.Result = text[:e.cut] + fmt.Sprintf("\n… cut at %d of %d characters. Return less: pick fields, slice arrays, summarize.", e.cut, len(text))
		}
	}
	return res, nil
}

func (e *boundedExecutor) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, reg geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	out := make([]*geptools.ToolResult, 0, len(calls))
	for _, c := range calls {
		r, err := e.ExecuteToolCall(ctx, c, reg)
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

// unwrapValue returns v["value"] for the {value: x} wrapper pages use for
// non-object results, else v.
func unwrapValue(v any) any {
	if m, ok := v.(map[string]any); ok && len(m) == 1 {
		if x, ok := m["value"]; ok {
			return x
		}
	}
	return v
}

// missingTool answers calls to tools the page did not register.
type missingTool struct{}

func (missingTool) ExecuteToolCall(_ context.Context, call geptools.ToolCall, _ geptools.ToolRegistry) (*geptools.ToolResult, error) {
	return &geptools.ToolResult{ID: call.ID, Error: "no tool named " + call.Name + " is available in this page"}, nil
}

func (m missingTool) ExecuteToolCalls(ctx context.Context, calls []geptools.ToolCall, reg geptools.ToolRegistry) ([]*geptools.ToolResult, error) {
	out := make([]*geptools.ToolResult, 0, len(calls))
	for _, c := range calls {
		r, _ := m.ExecuteToolCall(ctx, c, reg)
		out = append(out, r)
	}
	return out, nil
}
