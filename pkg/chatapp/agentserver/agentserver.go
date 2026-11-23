// Package agentserver is the agent service behind `pinocchio agent serve`: a
// chatapp httpapi server whose sessions each run one preset (a model, a
// system prompt and run limits) with the page's browser tools.
//
// It is an application of chatapp, not part of it: the texts it gives the
// model (the system prompt, the final-round instruction, the tool time-limit
// and result-cut notes) are its own, and an embedding program can replace
// them (WithTexts).
//
// A session is created with labels and optional data:
//
//	POST /v1/sessions {"labels": {"preset": "coder", "model": "pi:anthropic/claude-sonnet-4-5", "thinking": "low"},
//	                   "data": {"instructions": "the page's API reference"}}
//
// "preset" defaults to WithDefaultPreset (or the only preset); "model" and
// "thinking" default to the preset's. Instructions are appended to the
// preset's system prompt when the preset sets pageInstructions: true; the
// system prompt is fixed for the session, so the prompt prefix stays the same
// from turn to turn (prompt caching).
//
// Besides httpapi's routes under the prefix (default "/v1"), the server
// answers GET /health, GET /presets and GET /models.
//
// A program that embeds the server adds what only it knows:
//
//	srv, _ := agentserver.New(presets,
//	    agentserver.WithToolRegistry(tools),            // in-process tools; a preset's tools: list picks from them
//	    agentserver.WithSessionInstructions(apiRef),    // once per session, after the preset's prompt
//	    agentserver.WithMessageContext(whatChanged),    // per message, after the page's context
//	    agentserver.WithStateDir(dir))
//
// An in-process tool finds its session with srv.Session(ctx) and calls
// toolexec.AwaitPerson(ctx) while a person decides: the preset's
// personTimeout applies instead of its toolTimeout, and the call's
// ChatToolCall entity says "awaiting-person" (ChatToolCallProgress).
package agentserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/plugins"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/budget"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/presets"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/toolexec"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/pkg/errors"
)

// Defaults for preset fields left at zero.
const (
	DefaultPrefix                   = "/v1"
	DefaultMaxRounds                = 40
	DefaultToolTimeout              = 120 * time.Second
	DefaultResultCut                = 9000
	DefaultMaxPageInstructionsBytes = 64 << 10
	DefaultMaxContextBytes          = 64 << 10
	DefaultPersonTimeout            = 10 * time.Minute
)

// Error codes of rejected sessions and messages.
const (
	CodeUnknownPreset          = "unknown_preset"
	CodeUnknownModel           = "unknown_model"
	CodeInstructionsNotAllowed = "instructions_not_allowed"
	CodePromptTooLarge         = "prompt_too_large"
)

// Texts are what the server tells the model in the situations its limits
// create.
type Texts struct {
	// FinalRound is the instruction of the final answer at the round limit;
	// %d is the number of rounds.
	FinalRound string
	// RefusedTool answers a tool call the model makes during the final answer.
	RefusedTool string
	// ToolTimeout is the tool error of a call that ran out of time.
	ToolTimeout toolexec.DeadlineMessage
	// ResultCut is appended to a tool result that was cut.
	ResultCut toolexec.CutNote
}

// DefaultTexts are agentd's texts.
func DefaultTexts() Texts {
	return Texts{
		FinalRound: "You have used all %d tool rounds of this turn. Do not call tools. Answer the person now: " +
			"what you found, what is still running (with ids), and what you would do next. Do not claim anything you did not check.",
		RefusedTool: "no more tool calls in this turn; answer the person",
		ToolTimeout: func(call geptools.ToolCall, ran time.Duration) string {
			return fmt.Sprintf("The %s step ran for %s and was stopped: that is the limit for one tool call. "+
				"Work it started in the page (jobs, timers) may still be running. Do not wait in loops: "+
				"start long work, tell the person what you started, and end the turn, or check progress with a short call.",
				call.Name, ran.Round(time.Second))
		},
		ResultCut: func(shown, total int) string {
			return fmt.Sprintf("\n… cut at %d of %d characters. Return less: pick fields, slice arrays, summarize.", shown, total)
		},
	}
}

// MessageExtra is the part of a message body this server reads besides the
// prompt.
type MessageExtra struct {
	// Context is appended as a user block after the person's message (e.g.
	// the page's state). It stays in the conversation: send what changed, not
	// the whole state every time.
	Context string `json:"context,omitempty"`
}

// SessionData is the session data this server reads (sessions.Info.Data).
type SessionData struct {
	// Instructions are appended to the preset's system prompt (presets with
	// pageInstructions: true).
	Instructions string `json:"instructions,omitempty"`
	// ServerInstructions are the embedding program's (WithSessionInstructions),
	// written when the session is created; a page cannot set them.
	ServerInstructions string `json:"serverInstructions,omitempty"`
}

// SessionInstructionsFunc gives a new session's instructions from the
// embedding program (e.g. an API reference); they follow the preset's system
// prompt for the session's life.
type SessionInstructionsFunc func(ctx context.Context, info sessions.Info) (string, error)

// MessageContextFunc gives a message's context from the embedding program
// (e.g. the state the agent works in); a non-empty text becomes a user block
// after the message and after the page's own context. The conversation is
// append-only: return what changed, not the whole state every time.
type MessageContextFunc func(ctx context.Context, info sessions.Info, msg httpapi.MessageRequest) (string, error)

// Server is the agent service.
type Server struct {
	api           *httpapi.Server
	presets       map[string]*presets.Preset
	registries    map[string]geptools.ToolRegistry
	models        *modelsource.Set
	fixedModel    modelsource.Model
	newEngine     func() (engine.Engine, error)
	defaultPreset string
	prefix        string
	texts         Texts
	stateDir      string
	tools         geptools.ToolRegistry
	browserTools  *frontendtools.Manager
	httpOptions   []httpapi.Option
	closers       []func() error

	sessionInstructions SessionInstructionsFunc
	messageContext      MessageContextFunc
}

// Option configures a Server.
type Option func(*Server)

// WithModels resolves each session's model from set.
func WithModels(set *modelsource.Set) Option { return func(s *Server) { s.models = set } }

// WithEngine runs every session on engines from newEngine, described by
// model (e.g. the scripted test engine); model labels are ignored.
func WithEngine(model modelsource.Model, newEngine func() (engine.Engine, error)) Option {
	return func(s *Server) { s.fixedModel, s.newEngine = model, newEngine }
}

// WithStateDir keeps sessions, conversations and timelines in SQLite files
// in dir (default: in memory).
func WithStateDir(dir string) Option { return func(s *Server) { s.stateDir = dir } }

// WithDefaultPreset is the preset of sessions created without one.
func WithDefaultPreset(name string) Option { return func(s *Server) { s.defaultPreset = name } }

// WithPrefix sets the route prefix (default DefaultPrefix).
func WithPrefix(prefix string) Option { return func(s *Server) { s.prefix = prefix } }

// WithTexts replaces the texts the server gives the model.
func WithTexts(t Texts) Option { return func(s *Server) { s.texts = t } }

// WithSessionInstructions sets instructions from the embedding program for
// every new session (SessionInstructionsFunc).
func WithSessionInstructions(fn SessionInstructionsFunc) Option {
	return func(s *Server) { s.sessionInstructions = fn }
}

// WithMessageContext sets context from the embedding program for every
// message (MessageContextFunc).
func WithMessageContext(fn MessageContextFunc) Option {
	return func(s *Server) { s.messageContext = fn }
}

// WithToolRegistry offers in-process tools: a preset's tools list picks from
// registry by name.
func WithToolRegistry(registry geptools.ToolRegistry) Option {
	return func(s *Server) { s.tools = registry }
}

// WithHTTPOptions adds httpapi options (authentication, plugins, ...).
func WithHTTPOptions(opts ...httpapi.Option) Option {
	return func(s *Server) { s.httpOptions = append(s.httpOptions, opts...) }
}

// New builds the server for the presets.
func New(ps map[string]*presets.Preset, opts ...Option) (*Server, error) {
	if len(ps) == 0 {
		return nil, errors.New("agentserver: no presets")
	}
	s := &Server{presets: ps, prefix: DefaultPrefix, texts: DefaultTexts(), registries: map[string]geptools.ToolRegistry{}, browserTools: frontendtools.NewManager()}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.models == nil && s.newEngine == nil {
		return nil, errors.New("agentserver: models or an engine are required (WithModels, WithEngine)")
	}
	if s.defaultPreset == "" && len(ps) == 1 {
		for name := range ps {
			s.defaultPreset = name
		}
	}
	if s.defaultPreset != "" && ps[s.defaultPreset] == nil {
		return nil, fmt.Errorf("agentserver: default preset %q is not defined", s.defaultPreset)
	}
	for name, p := range ps {
		if len(p.Tools) == 0 {
			continue
		}
		if s.tools == nil {
			return nil, fmt.Errorf("agentserver: preset %q names in-process tools %v, but the server has none", name, p.Tools)
		}
		reg := geptools.NewInMemoryToolRegistry()
		for _, tool := range p.Tools {
			def, err := s.tools.GetTool(tool)
			if err != nil {
				return nil, fmt.Errorf("agentserver: preset %q: tool %q: %w", name, tool, err)
			}
			if err := reg.RegisterTool(tool, *def); err != nil {
				return nil, err
			}
		}
		s.registries[name] = reg
	}

	httpOpts := []httpapi.Option{
		httpapi.WithPrefix(s.prefix),
		httpapi.WithRuntimeResolver(httpapi.RuntimeResolverFunc(s.resolve)),
		httpapi.WithCreateHook(s.completeSession),
		httpapi.WithBlocks(s.contextBlocks),
		httpapi.WithFrontendTools(s.browserTools),
		httpapi.WithChatPlugins(frontendtools.NewPlugin(), plugins.NewReasoningPlugin(), plugins.NewToolCallPlugin()),
		// in-process tools find their run (Session, tool progress) through the context
		httpapi.WithEngineOptions(chatapp.WithRunTools(keepRun)),
	}
	if s.stateDir != "" {
		httpOpts = append(httpOpts, httpapi.WithStateDir(s.stateDir))
	}
	api, err := httpapi.New(append(httpOpts, s.httpOptions...)...)
	if err != nil {
		_ = s.Close()
		return nil, err
	}
	s.api = api
	s.closers = append(s.closers, api.Close)
	return s, nil
}

// API is the httpapi server (sessions store, chatapp service).
func (s *Server) API() *httpapi.Server { return s.api }

// Handler serves httpapi's routes and /health, /presets, /models.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.api.Register(mux)
	p := "/" + strings.Trim(s.prefix, "/")
	if p == "/" {
		p = ""
	}
	mux.HandleFunc("GET "+p+"/health", func(w http.ResponseWriter, r *http.Request) {
		serverkit.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "presets": len(s.presets)})
	})
	mux.HandleFunc("GET "+p+"/presets", s.handlePresets)
	mux.HandleFunc("GET "+p+"/models", s.handleModels)
	return mux
}

// Close closes the stores.
func (s *Server) Close() error {
	var errs []error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i](); err != nil {
			errs = append(errs, err)
		}
	}
	s.closers = nil
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// PresetView is a preset as GET /presets lists it.
type PresetView struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Model            string `json:"model,omitempty"`
	Thinking         string `json:"thinking,omitempty"`
	MaxRounds        int    `json:"maxRounds"`
	PageInstructions bool   `json:"pageInstructions"`
	Default          bool   `json:"default,omitempty"`
}

func (s *Server) handlePresets(w http.ResponseWriter, _ *http.Request) {
	out := []PresetView{}
	for _, name := range presets.Names(s.presets) {
		p := s.presets[name]
		model := p.Model
		if s.newEngine != nil {
			model = s.fixedModel.Ref
		}
		out = append(out, PresetView{Name: name, Description: p.Description, Model: model, Thinking: p.Thinking,
			MaxRounds: orInt(p.MaxRounds, DefaultMaxRounds), PageInstructions: pageInstructions(p), Default: name == s.defaultPreset})
	}
	serverkit.WriteJSON(w, http.StatusOK, map[string]any{"presets": out})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if s.newEngine != nil {
		serverkit.WriteJSON(w, http.StatusOK, map[string]any{"models": []modelsource.Model{s.fixedModel}})
		return
	}
	models, err := s.models.Models(r.Context())
	if err != nil {
		log.Warn().Err(err).Msg("agentserver: some model sources failed")
	}
	if models == nil {
		models = []modelsource.Model{}
	}
	serverkit.WriteJSON(w, http.StatusOK, map[string]any{"models": models})
}

// completeSession is the create hook: it fills in the default preset and
// checks the preset, the model and the instructions.
func (s *Server) completeSession(ctx context.Context, _ httpapi.Principal, info *sessions.Info) error {
	if info.Labels == nil {
		info.Labels = map[string]string{}
	}
	name := info.Labels["preset"]
	if name == "" {
		name = s.defaultPreset
	}
	p := s.presets[name]
	if p == nil {
		if name == "" {
			return httpapi.Coded(CodeUnknownPreset, fmt.Sprintf("choose a preset (labels.preset): %s", strings.Join(presets.Names(s.presets), ", ")))
		}
		return httpapi.Coded(CodeUnknownPreset, fmt.Sprintf("unknown preset %q", name))
	}
	info.Labels["preset"] = name
	if ref := info.Labels["model"]; ref != "" && s.models != nil {
		if _, err := s.models.Resolve(ctx, ref, modelsource.Options{}); err != nil {
			return httpapi.Coded(CodeUnknownModel, err.Error())
		}
	}
	data, err := sessionData(*info)
	if err != nil {
		return httpapi.Coded(httpapi.CodeInvalidRequest, err.Error())
	}
	if err := s.setServerInstructions(ctx, info); err != nil {
		return err
	}
	if data.Instructions != "" {
		if !pageInstructions(p) {
			return httpapi.Coded(CodeInstructionsNotAllowed, fmt.Sprintf("preset %q does not accept page instructions", name))
		}
		if limit := maxPageInstructionsBytes(p); len(data.Instructions) > limit {
			return httpapi.Coded(CodePromptTooLarge, fmt.Sprintf("page instructions are at most %d bytes", limit))
		}
	}
	return nil
}

// resolve builds one message's runtime from the session's preset.
func (s *Server) resolve(ctx context.Context, _ *http.Request, info sessions.Info, _ httpapi.MessageRequest) (*infruntime.ComposedRuntime, error) {
	p := s.presets[info.Labels["preset"]]
	if p == nil {
		return nil, httpapi.Coded(CodeUnknownPreset, fmt.Sprintf("the session's preset %q is no longer defined", info.Labels["preset"]))
	}
	base, modelRef, err := s.engineFor(ctx, p, info)
	if err != nil {
		return nil, err
	}
	data, err := sessionData(info)
	if err != nil {
		return nil, err
	}
	system := p.SystemText()
	for _, extra := range []string{data.ServerInstructions, data.Instructions} {
		if strings.TrimSpace(extra) != "" {
			system = strings.TrimRight(system, "\n") + "\n\n" + extra
		}
	}
	mws := []middleware.Middleware{middleware.NewToolResultReorderMiddleware()}
	if strings.TrimSpace(system) != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(system))
	}

	rounds := orInt(p.MaxRounds, DefaultMaxRounds)
	loop := toolloop.DefaultLoopConfig().WithMaxIterations(rounds).WithFinalAnswer(toolloop.FinalAnswer{
		Prompt:            func(n int) turns.Block { return turns.NewUserTextBlock(fmt.Sprintf(s.texts.FinalRound, n)) },
		RefusedToolResult: s.texts.RefusedTool,
	})
	timeout := p.ToolTimeoutDuration()
	if timeout <= 0 {
		timeout = DefaultToolTimeout
	}
	toolCfg := geptools.DefaultToolConfig()
	toolCfg.ExecutionTimeout = timeout + 5*time.Second // the deadline below acts first
	toolCfg.MaxParallelTools = 1
	toolCfg.RetryConfig.MaxRetries = 0 // a retried browser call would run page code twice

	return &infruntime.ComposedRuntime{
		Engine:     engineWithMiddleware{handler: middleware.Chain(base.RunInference, mws...)},
		Registry:   s.registries[p.Name],
		Loop:       &loop,
		ToolConfig: &toolCfg,
		RoundGuard: budget.SessionTokens(s.api.Sessions(), p.Budget.TokensPerSession),
		WrapToolExecutor: toolexec.Compose(
			toolexec.DeadlineBy(s.callLimit(p, timeout), s.texts.ToolTimeout,
				toolexec.WithPersonLimit(func(context.Context, geptools.ToolCall) time.Duration {
					return extraDuration(p, "personTimeout", DefaultPersonTimeout)
				}),
				toolexec.OnState(publishToolState)),
			toolexec.CutResults(orInt(p.ResultCut, DefaultResultCut), s.texts.ResultCut),
		),
		RuntimeKey:         p.Name,
		RuntimeFingerprint: p.Name + ":" + modelRef,
		Model:              modelRef,
	}, nil
}

func (s *Server) engineFor(ctx context.Context, p *presets.Preset, info sessions.Info) (engine.Engine, string, error) {
	if s.newEngine != nil {
		eng, err := s.newEngine()
		return eng, s.fixedModel.Ref, err
	}
	ref := info.Labels["model"]
	if ref == "" {
		ref = p.Model
	}
	if ref == "" {
		return nil, "", httpapi.Coded(CodeUnknownModel, fmt.Sprintf("preset %q names no model and the session chose none", p.Name))
	}
	thinking := info.Labels["thinking"]
	if thinking == "" {
		thinking = p.Thinking
	}
	resolved, err := s.models.Resolve(ctx, ref, modelsource.Options{Thinking: thinking, MaxTokens: p.MaxTokens})
	if err != nil {
		return nil, "", httpapi.Coded(CodeUnknownModel, err.Error())
	}
	eng, err := resolved.Engine()
	return eng, resolved.Model.Ref, err
}

// callLimit is a call's time limit: the preset's personTimeout for browser
// tools the person approves (mode "human"), its toolTimeout for the others.
func (s *Server) callLimit(p *presets.Preset, timeout time.Duration) func(context.Context, geptools.ToolCall) time.Duration {
	person := extraDuration(p, "personTimeout", DefaultPersonTimeout)
	return func(ctx context.Context, call geptools.ToolCall) time.Duration {
		sid := sessionstream.SessionId(gepsession.SessionIDFromContext(ctx))
		if name := s.browserTools.ResolveProviderToolName(sid, call.Name); name != "" {
			if desc, ok := s.browserTools.Descriptor(sid, name); ok && desc.GetMode() == toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN {
				return person
			}
		}
		return timeout
	}
}

// contextBlocks is the BlocksFunc: a message's "context" (the page's) and the
// embedding program's context (WithMessageContext) become user blocks after
// the person's message, in that order.
func (s *Server) contextBlocks(ctx context.Context, info sessions.Info, msg httpapi.MessageRequest) ([]turns.Block, error) {
	limit := DefaultMaxContextBytes
	if p := s.presets[info.Labels["preset"]]; p != nil {
		if v, ok := p.Extra["maxContextBytes"].(int); ok && v > 0 {
			limit = v
		}
	}
	blocks, err := httpapi.ContextField(limit)(ctx, info, msg)
	if err != nil || s.messageContext == nil {
		return blocks, err
	}
	text, err := s.messageContext(ctx, info, msg)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) != "" {
		blocks = append(blocks, turns.NewUserTextBlock(text))
	}
	return blocks, nil
}

func extraDuration(p *presets.Preset, key string, def time.Duration) time.Duration {
	if s, ok := p.Extra[key].(string); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil && d > 0 {
			return d
		}
	}
	return def
}

type engineWithMiddleware struct{ handler middleware.HandlerFunc }

func (e engineWithMiddleware) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	return e.handler(ctx, t)
}

func sessionData(info sessions.Info) (SessionData, error) {
	var d SessionData
	if len(info.Data) == 0 {
		return d, nil
	}
	if err := json.Unmarshal(info.Data, &d); err != nil {
		return d, fmt.Errorf("session data: %w", err)
	}
	return d, nil
}

func pageInstructions(p *presets.Preset) bool {
	v, _ := p.Extra["pageInstructions"].(bool)
	return v
}

func maxPageInstructionsBytes(p *presets.Preset) int {
	if v, ok := p.Extra["maxPageInstructionsBytes"].(int); ok && v > 0 {
		return v
	}
	return DefaultMaxPageInstructionsBytes
}

func orInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
