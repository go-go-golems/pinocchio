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
}

type activeRun struct {
	id        string
	handle    *gepsession.ExecutionHandle
	cancelled bool
}

// SessionView is the JSON description of a session.
type SessionView struct {
	ID       string            `json:"id"`
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
	MaxRounds     int   `json:"maxRounds"`
	ToolTimeoutMs int64 `json:"toolTimeoutMs"`
	ResultCut     int   `json:"resultCut"`
	TokenBudget   int   `json:"tokenBudget,omitempty"`
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
	return Limits{MaxRounds: s.Agent.MaxRounds, ToolTimeoutMs: s.Agent.ToolTimeoutDuration().Milliseconds(), ResultCut: s.Agent.ResultCut, TokenBudget: s.Agent.Budget.TokensPerSession}
}

// View describes the session; withMessages includes the conversation.
func (s *Session) View(withMessages bool) SessionView {
	s.mu.Lock()
	v := SessionView{ID: s.ID, Agent: s.Agent.Name, Model: s.Model, Title: s.title, Created: s.Created, Updated: s.updated, Usage: s.usage, Limits: s.limits()}
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
		str := func(k string) string { v, _ := b.Payload[k].(string); return v }
		m := MessageView{ID: b.ID}
		switch b.Kind {
		case turns.BlockKindSystem:
			continue
		case turns.BlockKindUser:
			m.Kind, m.Text = "user", str(turns.PayloadKeyText)
			if strings.HasPrefix(m.Text, ContextPrefix) {
				m.Kind, m.Text = "context", strings.TrimPrefix(m.Text, ContextPrefix)
			}
		case turns.BlockKindLLMText:
			m.Kind, m.Text = "assistant", str(turns.PayloadKeyText)
		case turns.BlockKindReasoning:
			m.Kind, m.Text = "reasoning", str(turns.PayloadKeyText)
		case turns.BlockKindToolCall:
			m.Kind, m.CallID, m.Name, m.Input = "tool_call", str(turns.PayloadKeyID), str(turns.PayloadKeyName), b.Payload[turns.PayloadKeyArgs]
		case turns.BlockKindToolUse:
			m.Kind, m.CallID, m.Result, m.Error = "tool_result", str(turns.PayloadKeyID), b.Payload[turns.PayloadKeyResult], str(turns.PayloadKeyError)
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
		usd := (float64(s.usage.Input)*c.Input + float64(s.usage.Output)*c.Output + float64(s.usage.CacheRead)*c.CacheRead + float64(s.usage.CacheCreation)*c.CacheWrite) / 1e6
		s.usage.USD = &usd
	}
	return round, s.usage
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
	var registry geptools.ToolRegistry
	if len(reg.ListTools()) > 0 {
		registry = reg
	}
	loopCfg := toolloop.DefaultLoopConfig().WithMaxIterations(s.Agent.MaxRounds)
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
		ToolExecutor: &boundedExecutor{inner: frontendtools.NewBridgeExecutor(s.srv.tools, missingTool{}), cut: s.Agent.ResultCut, timeout: s.Agent.ToolTimeoutDuration()},
		EventSinks:   []gepevents.EventSink{sink},
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
	switch {
	case cancelled || errors.Is(err, context.Canceled):
		s.log.Append(EvRunCancelled, runID, map[string]any{"text": text})
	case err != nil && strings.Contains(err.Error(), "max iterations"):
		s.log.Append(EvRunFinished, runID, map[string]any{"text": text, "truncated": true, "reason": "max_rounds"})
	case err != nil:
		code, msg := classify(err)
		log.Warn().Err(err).Str("session", s.ID).Str("run", runID).Str("code", code).Msg("agentd run failed")
		s.log.Append(EvRunFailed, runID, map[string]any{"code": code, "message": msg, "text": text})
	default:
		s.log.Append(EvRunFinished, runID, map[string]any{"text": text, "truncated": false})
	}
	if s.srv.store != nil {
		if err := s.srv.store.Save(s); err != nil {
			log.Warn().Err(err).Str("session", s.ID).Msg("agentd: save session")
		}
	}
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

// boundedExecutor adds a per-call timeout and cuts large results.
type boundedExecutor struct {
	inner   geptools.ToolExecutor
	cut     int
	timeout time.Duration
}

func (e *boundedExecutor) ExecuteToolCall(ctx context.Context, call geptools.ToolCall, reg geptools.ToolRegistry) (*geptools.ToolResult, error) {
	cctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	res, err := e.inner.ExecuteToolCall(cctx, call, reg)
	if err != nil {
		return res, err
	}
	if res == nil {
		res = &geptools.ToolResult{ID: call.ID}
	}
	if errors.Is(cctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		res.Result, res.Error = nil, fmt.Sprintf("The tool %s did not answer within %s; the page may be busy or closed. Ask for less, or try again.", call.Name, e.timeout)
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
