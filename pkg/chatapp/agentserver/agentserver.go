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
package agentserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/budget"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/presets"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/toolkit/toolexec"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/pkg/errors"
)

// Defaults for preset fields left at zero.
const (
	DefaultPrefix                   = "/v1"
	DefaultMaxRounds                = 40
	DefaultToolTimeout              = 120 * time.Second
	DefaultResultCut                = 9000
	DefaultMaxPageInstructionsBytes = 64 << 10
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

// SessionData is the session data this server reads (sessions.Info.Data).
type SessionData struct {
	// Instructions are appended to the preset's system prompt (presets with
	// pageInstructions: true).
	Instructions string `json:"instructions,omitempty"`
}

type codedError struct{ code, msg string }

func (e codedError) Error() string { return e.msg }
func (e codedError) Code() string  { return e.code }

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
	httpOptions   []httpapi.Option
	closers       []func() error
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
	s := &Server{presets: ps, prefix: DefaultPrefix, texts: DefaultTexts(), registries: map[string]geptools.ToolRegistry{}}
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
		httpapi.WithFrontendTools(frontendtools.NewManager()),
		httpapi.WithChatPlugins(frontendtools.NewPlugin()),
	}
	if s.stateDir != "" {
		stores, err := s.openStores()
		if err != nil {
			_ = s.Close()
			return nil, err
		}
		httpOpts = append(httpOpts, stores...)
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

func (s *Server) openStores() ([]httpapi.Option, error) {
	sessionStore, err := sessions.NewSQLiteStore("file:" + filepath.Join(s.stateDir, "sessions.db") + "?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, errors.Wrap(err, "open session store")
	}
	s.closers = append(s.closers, sessionStore.Close)
	turnStore, err := chatstore.NewSQLiteTurnStore(filepath.Join(s.stateDir, "turns.db"))
	if err != nil {
		return nil, errors.Wrap(err, "open turn store")
	}
	s.closers = append(s.closers, turnStore.Close)
	return []httpapi.Option{
		httpapi.WithSessionStore(sessionStore),
		httpapi.WithTurnStore(turnStore),
		httpapi.WithTurnsDBPath(filepath.Join(s.stateDir, "turns.db")),
		httpapi.WithHydrationStoreSpec(serverkit.StoreSpec{Backend: serverkit.StoreBackendSQLite, Path: filepath.Join(s.stateDir, "timeline.db")}),
	}, nil
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
			return codedError{CodeUnknownPreset, fmt.Sprintf("choose a preset (labels.preset): %s", strings.Join(presets.Names(s.presets), ", "))}
		}
		return codedError{CodeUnknownPreset, fmt.Sprintf("unknown preset %q", name)}
	}
	info.Labels["preset"] = name
	if ref := info.Labels["model"]; ref != "" && s.models != nil {
		if _, err := s.models.Resolve(ctx, ref, modelsource.Options{}); err != nil {
			return codedError{CodeUnknownModel, err.Error()}
		}
	}
	data, err := sessionData(*info)
	if err != nil {
		return codedError{httpapi.CodeInvalidRequest, err.Error()}
	}
	if data.Instructions != "" {
		if !pageInstructions(p) {
			return codedError{CodeInstructionsNotAllowed, fmt.Sprintf("preset %q does not accept page instructions", name)}
		}
		if limit := maxPageInstructionsBytes(p); len(data.Instructions) > limit {
			return codedError{CodePromptTooLarge, fmt.Sprintf("page instructions are at most %d bytes", limit)}
		}
	}
	return nil
}

// resolve builds one message's runtime from the session's preset.
func (s *Server) resolve(ctx context.Context, _ *http.Request, info sessions.Info, _ httpapi.MessageRequest) (*infruntime.ComposedRuntime, error) {
	p := s.presets[info.Labels["preset"]]
	if p == nil {
		return nil, codedError{CodeUnknownPreset, fmt.Sprintf("the session's preset %q is no longer defined", info.Labels["preset"])}
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
	if data.Instructions != "" {
		system = strings.TrimRight(system, "\n") + "\n\n" + data.Instructions
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
			toolexec.Deadline(timeout, s.texts.ToolTimeout),
			toolexec.CutResults(orInt(p.ResultCut, DefaultResultCut), s.texts.ResultCut),
		),
		RuntimeKey:         p.Name,
		RuntimeFingerprint: p.Name + ":" + modelRef,
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
		return nil, "", codedError{CodeUnknownModel, fmt.Sprintf("preset %q names no model and the session chose none", p.Name)}
	}
	thinking := info.Labels["thinking"]
	if thinking == "" {
		thinking = p.Thinking
	}
	resolved, err := s.models.Resolve(ctx, ref, modelsource.Options{Thinking: thinking, MaxTokens: p.MaxTokens})
	if err != nil {
		return nil, "", codedError{CodeUnknownModel, err.Error()}
	}
	eng, err := resolved.Engine()
	return eng, resolved.Model.Ref, err
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
