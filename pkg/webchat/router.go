package webchat

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolhelpers"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/pinocchio/pkg/inference/runner"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

// RouterSettings are exposed via parameter layers (addr, agent, idle timeout, etc.).
type RouterSettings struct {
	Addr               string `glazed.parameter:"addr"`
	IdleTimeoutSeconds int    `glazed.parameter:"idle-timeout-seconds"`
}

// RouterBuilder creates a new composable webchat router.
func NewRouter(ctx context.Context, parsed *layers.ParsedLayers, staticFS embed.FS) (*Router, error) {
	rs := rediscfg.Settings{}
	_ = parsed.InitializeStruct("redis", &rs)
	router, err := rediscfg.BuildRouter(rs, true)
	if err != nil {
		return nil, errors.Wrap(err, "build event router")
	}
	r := &Router{
		baseCtx:       ctx,
		parsed:        parsed,
		mux:           http.NewServeMux(),
		staticFS:      staticFS,
		router:        router,
		mwFactories:   map[string]MiddlewareFactory{},
		toolFactories: map[string]ToolFactory{},
		profiles:      newInMemoryProfileRegistry(),
		upgrader:      websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		cm:            &ConvManager{conns: map[string]*Conversation{}},
	}
	// set redis flags for ws reader
	if rs.Enabled {
		r.usesRedis = true
		r.redisAddr = rs.Addr
	}
	r.registerHTTPHandlers()
	return r, nil
}

// Allow setting optional shared DB for middlewares that need it (e.g., sqlite tool)
func (r *Router) WithDB(db *sql.DB) *Router { r.db = db; return r }

// RegisterMiddleware adds a named middleware factory to the router.
func (r *Router) RegisterMiddleware(name string, f MiddlewareFactory) { r.mwFactories[name] = f }

// RegisterTool adds a named tool factory to the router.
func (r *Router) RegisterTool(name string, f ToolFactory) { r.toolFactories[name] = f }

// AddProfile registers a chat profile.
func (r *Router) AddProfile(p *Profile) { _ = r.profiles.Add(p) }

// Mount attaches all handlers to a parent mux with the given prefix.
func (r *Router) Mount(mux *http.ServeMux, prefix string) { mux.Handle(prefix, r.mux) }

// Expose lightweight handler registration for external customization (e.g., profile switchers)
func (r *Router) Handle(pattern string, h http.Handler) { r.mux.Handle(pattern, h) }
func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.mux.HandleFunc(pattern, handler)
}

// Handler returns the internal mux as an http.Handler.
func (r *Router) Handler() http.Handler { return r.mux }

// BuildHTTPServer constructs an http.Server using settings from layers.
func (r *Router) BuildHTTPServer() (*http.Server, error) {
	s := &RouterSettings{}
	if err := r.parsed.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return nil, err
	}
	r.idleTimeoutSec = s.IdleTimeoutSeconds
	return &http.Server{
		Addr:              s.Addr,
		Handler:           r.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, nil
}

// RunEventRouter starts the underlying event router loop with the provided context.
// This is useful when integrating the webchat router into an existing HTTP server
// and you need the event router running independently.
func (r *Router) RunEventRouter(ctx context.Context) error {
	log.Info().Str("component", "webchat").Msg("starting event router loop")
	err := r.router.Run(ctx)
	if err != nil {
		log.Error().Err(err).Str("component", "webchat").Msg("event router exited with error")
		return err
	}
	log.Info().Str("component", "webchat").Msg("event router loop exited")
	return nil
}

// registerHTTPHandlers sets up static, API and websockets.
func (r *Router) registerHTTPHandlers() {
	// static assets
	if staticSub, err := fsSub(r.staticFS, "static"); err == nil {
		r.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
		log.Info().Str("component", "webchat").Msg("mounted /static/ asset handler")
	} else {
		log.Warn().Err(err).Str("component", "webchat").Msg("failed to mount /static/ asset handler")
	}
	if distAssets, err := fsSub(r.staticFS, "static/dist/assets"); err == nil {
		r.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(distAssets))))
		log.Info().Str("component", "webchat").Msg("mounted /assets/ handler for built dist assets")
	} else {
		log.Warn().Err(err).Str("component", "webchat").Msg("no built dist assets found under static/dist/assets")
	}
	// index
	r.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if b, err := r.staticFS.ReadFile("static/dist/index.html"); err == nil {
			log.Debug().Str("component", "webchat").Msg("serving built index (static/dist/index.html)")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		if b, err := r.staticFS.ReadFile("static/index.html"); err == nil {
			log.Debug().Str("component", "webchat").Msg("serving dev index (static/index.html)")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		log.Error().Str("component", "webchat").Msg("index not found in embedded FS")
		http.Error(w, "index not found", http.StatusInternalServerError)
	})

	// list profiles for UI
	r.mux.HandleFunc("/api/chat/profiles", func(w http.ResponseWriter, _ *http.Request) {
		type profileInfo struct {
			Slug          string `json:"slug"`
			DefaultPrompt string `json:"default_prompt"`
		}
		var out []profileInfo
		for _, p := range r.profiles.List() {
			out = append(out, profileInfo{Slug: p.Slug, DefaultPrompt: p.DefaultPrompt})
		}
		_ = json.NewEncoder(w).Encode(out)
	})

	// websocket join: /ws?conv_id=...&profile=slug (falls back to chat_profile cookie)
	r.mux.HandleFunc("/ws", func(w http.ResponseWriter, r0 *http.Request) {
		conn, err := r.upgrader.Upgrade(w, r0, nil)
		if err != nil {
			log.Error().Err(err).Msg("websocket upgrade failed")
			return
		}
		convID := r0.URL.Query().Get("conv_id")
		profileSlug := r0.URL.Query().Get("profile")
		if profileSlug == "" {
			if ck, err := r0.Cookie("chat_profile"); err == nil && ck != nil {
				profileSlug = ck.Value
			}
		}
		log.Info().Str("component", "webchat").Str("remote", r0.RemoteAddr).Str("conv_id", convID).Str("profile", profileSlug).Msg("ws connect request")
		if convID == "" {
			log.Warn().Str("component", "webchat").Msg("ws missing conv_id")
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing conv_id"}`))
			_ = conn.Close()
			return
		}
		if profileSlug == "" {
			profileSlug = "default"
		}
		log.Info().Str("component", "webchat").Str("conv_id", convID).Str("profile", profileSlug).Msg("ws joining conversation")
		build := func() (engine.Engine, *middleware.WatermillSink, message.Subscriber, error) {
			var (
				eng  engine.Engine
				sink *middleware.WatermillSink
				sub  message.Subscriber
				err  error
			)
			// subscriber/publisher
			if r.usesRedis {
				_ = rediscfg.EnsureGroupAtTail(context.Background(), r.redisAddr, topicForConv(convID), "ui")
			}
			if r.usesRedis {
				sub, err = rediscfg.BuildGroupSubscriber(r.redisAddr, "ui", "ws-forwarder:"+convID)
				if err != nil {
					return nil, nil, nil, err
				}
			} else {
				sub = r.router.Subscriber
			}
			sink = middleware.NewWatermillSink(r.router.Publisher, topicForConv(convID))
			// engine from profile + overrides are not needed for ws join
			p, _ := r.profiles.Get(profileSlug)
			stepSettings, _ := settings.NewStepSettingsFromParsedLayers(r.parsed)
			sys := p.DefaultPrompt
			eng, err = composeEngineFromSettings(stepSettings, sys, p.DefaultMws, r.mwFactories)
			return eng, sink, sub, err
		}
		conv, err := r.getOrCreateConv(convID, build)
		if err != nil {
			log.Error().Err(err).Str("component", "webchat").Str("conv_id", convID).Msg("failed to join conversation")
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
			_ = conn.Close()
			return
		}
		r.addConn(conv, conn)
		log.Info().Str("component", "webchat").Str("conv_id", convID).Msg("ws connected")
		go func() {
			defer r.removeConn(conv, conn)
			defer log.Info().Str("component", "webchat").Str("conv_id", convID).Msg("ws disconnected")
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					log.Debug().Err(err).Str("component", "webchat").Msg("ws read loop end")
					return
				}
			}
		}()
	})

	// start run: support both /chat (default/cookie/profile) and /chat/{profile}
	r.mux.HandleFunc("/chat", func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Prompt    string         `json:"prompt"`
			ConvID    string         `json:"conv_id"`
			Overrides map[string]any `json:"overrides"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			log.Warn().Err(err).Str("component", "webchat").Msg("bad /chat request body")
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		convID := body.ConvID
		if convID == "" {
			convID = uuid.NewString()
		}
		profileSlug := ""
		if ck, err := r0.Cookie("chat_profile"); err == nil && ck != nil {
			profileSlug = ck.Value
		}
		if profileSlug == "" {
			profileSlug = "default"
		}
		log.Info().Str("component", "webchat").Str("conv_id", convID).Str("profile", profileSlug).Int("prompt_len", len(body.Prompt)).Msg("/chat received")
		p, ok := r.profiles.Get(profileSlug)
		if !ok {
			log.Warn().Str("component", "webchat").Str("profile", profileSlug).Msg("unknown profile")
			http.Error(w, "unknown profile", http.StatusNotFound)
			return
		}

		build := func() (engine.Engine, *middleware.WatermillSink, message.Subscriber, error) {
			var (
				eng  engine.Engine
				sink *middleware.WatermillSink
				sub  message.Subscriber
				err  error
			)
			if r.usesRedis {
				_ = rediscfg.EnsureGroupAtTail(context.Background(), r.redisAddr, topicForConv(convID), "ui")
			}
			if r.usesRedis {
				sub, err = rediscfg.BuildGroupSubscriber(r.redisAddr, "ui", "ws-forwarder:"+convID)
				if err != nil {
					return nil, nil, nil, err
				}
			} else {
				sub = r.router.Subscriber
			}
			sink = middleware.NewWatermillSink(r.router.Publisher, topicForConv(convID))
			stepSettings, err2 := settings.NewStepSettingsFromParsedLayers(r.parsed)
			if err2 != nil {
				return nil, nil, nil, err2
			}
			sys := p.DefaultPrompt
			uses := append([]MiddlewareUse{}, p.DefaultMws...)
			if body.Overrides != nil {
				if v, ok := body.Overrides["system_prompt"].(string); ok && v != "" {
					sys = v
				}
				if arr, ok := body.Overrides["middlewares"].([]any); ok {
					uses = make([]MiddlewareUse, 0, len(arr))
					for _, it := range arr {
						if m, ok2 := it.(map[string]any); ok2 {
							name, _ := m["name"].(string)
							cfg := m["config"]
							if name != "" {
								uses = append(uses, MiddlewareUse{Name: name, Config: cfg})
							}
						}
					}
				}
			}
			eng, err = composeEngineFromSettings(stepSettings, sys, uses, r.mwFactories)
			return eng, sink, sub, err
		}
		conv, err := r.getOrCreateConv(convID, build)
		if err != nil {
			log.Error().Err(err).Str("component", "webchat").Str("conv_id", convID).Msg("failed to create conversation")
			http.Error(w, "failed to create conversation", http.StatusInternalServerError)
			return
		}
		conv.mu.Lock()
		if conv.running {
			conv.mu.Unlock()
			log.Warn().Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("run in progress")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "run in progress", "conv_id": conv.ID, "run_id": conv.RunID})
			return
		}
		conv.running = true
		conv.mu.Unlock()
		registry := geptools.NewInMemoryToolRegistry()
		for name, tf := range r.toolFactories {
			_ = tf(registry)
			_ = name
		}

		go func(conv *Conversation) {
			<-r.router.Running()
			runCtx, runCancel := context.WithCancel(r.baseCtx)
			conv.mu.Lock()
			conv.cancel = runCancel
			conv.mu.Unlock()
			hook := snapshotHookForConv(conv, os.Getenv("PINOCCHIO_WEBCHAT_TURN_SNAPSHOTS_DIR"))
			log.Info().Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("starting run loop")
			cfg := toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60 * time.Second)
			_, err := runner.Run(runCtx, conv.Eng, &conv.State, body.Prompt, runner.RunOptions{
				ToolRegistry: registry,
				ToolConfig:   &cfg,
				SnapshotHook: hook,
				EventSinks:   []events.EventSink{conv.Sink},
				Update: runner.UpdateOptions{
					FilterBlocks: filterSystemPromptBlocks,
				},
			})
			if err != nil {
				log.Error().Err(err).Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("run loop error")
			}
			runCancel()
			conv.mu.Lock()
			conv.running = false
			conv.cancel = nil
			conv.mu.Unlock()
			log.Info().Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("run loop finished")
		}(conv)

		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": conv.RunID, "conv_id": conv.ID})
	})

	// start run: /chat/{profile}
	r.mux.HandleFunc("/chat/", func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// extract profile from path: /chat/{profile}[/...]
		path := r0.URL.Path
		var profileSlug string
		if strings.HasPrefix(path, "/chat/") {
			rest := path[len("/chat/"):]
			if i := strings.Index(rest, "/"); i >= 0 {
				profileSlug = rest[:i]
			} else {
				profileSlug = rest
			}
		}
		if profileSlug == "" {
			profileSlug = "default"
		}
		var body struct {
			Prompt    string         `json:"prompt"`
			ConvID    string         `json:"conv_id"`
			Overrides map[string]any `json:"overrides"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		convID := body.ConvID
		if convID == "" {
			convID = uuid.NewString()
		}
		p, ok := r.profiles.Get(profileSlug)
		if !ok {
			http.Error(w, "unknown profile", http.StatusNotFound)
			return
		}

		// Build or reuse conversation with correct engine (consider overrides)
		build := func() (engine.Engine, *middleware.WatermillSink, message.Subscriber, error) {
			var (
				eng  engine.Engine
				sink *middleware.WatermillSink
				sub  message.Subscriber
				err  error
			)
			if r.usesRedis {
				_ = rediscfg.EnsureGroupAtTail(context.Background(), r.redisAddr, topicForConv(convID), "ui")
			}
			if r.usesRedis {
				sub, err = rediscfg.BuildGroupSubscriber(r.redisAddr, "ui", "ws-forwarder:"+convID)
				if err != nil {
					return nil, nil, nil, err
				}
			} else {
				sub = r.router.Subscriber
			}
			sink = middleware.NewWatermillSink(r.router.Publisher, topicForConv(convID))
			// step settings from layers and apply overrides if provided
			stepSettings, err2 := settings.NewStepSettingsFromParsedLayers(r.parsed)
			if err2 != nil {
				return nil, nil, nil, err2
			}
			sys := p.DefaultPrompt
			uses := append([]MiddlewareUse{}, p.DefaultMws...)
			// apply overrides: system_prompt, middlewares
			if body.Overrides != nil {
				if v, ok := body.Overrides["system_prompt"].(string); ok && v != "" {
					sys = v
				}
				if arr, ok := body.Overrides["middlewares"].([]any); ok {
					uses = make([]MiddlewareUse, 0, len(arr))
					for _, it := range arr {
						if m, ok2 := it.(map[string]any); ok2 {
							name, _ := m["name"].(string)
							cfg := m["config"]
							if name != "" {
								uses = append(uses, MiddlewareUse{Name: name, Config: cfg})
							}
						}
					}
				}
				// TODO: tools override can be applied via registry decision in loop
			}
			eng, err = composeEngineFromSettings(stepSettings, sys, uses, r.mwFactories)
			return eng, sink, sub, err
		}
		conv, err := r.getOrCreateConv(convID, build)
		if err != nil {
			http.Error(w, "failed to create conversation", http.StatusInternalServerError)
			return
		}
		conv.mu.Lock()
		if conv.running {
			conv.mu.Unlock()
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "run in progress", "conv_id": conv.ID, "run_id": conv.RunID})
			return
		}
		conv.running = true
		conv.mu.Unlock()
		// Build registry for this run from default tools (and optional overrides later)
		registry := geptools.NewInMemoryToolRegistry()
		for name, tf := range r.toolFactories {
			_ = tf(registry)
			_ = name
		}

		go func(conv *Conversation) {
			<-r.router.Running()
			runCtx, runCancel := context.WithCancel(r.baseCtx)
			conv.mu.Lock()
			conv.cancel = runCancel
			conv.mu.Unlock()
			hook := snapshotHookForConv(conv, os.Getenv("PINOCCHIO_WEBCHAT_TURN_SNAPSHOTS_DIR"))
			log.Info().Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("starting run loop")
			cfg := toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60 * time.Second)
			_, err := runner.Run(runCtx, conv.Eng, &conv.State, body.Prompt, runner.RunOptions{
				ToolRegistry: registry,
				ToolConfig:   &cfg,
				SnapshotHook: hook,
				EventSinks:   []events.EventSink{conv.Sink},
				Update: runner.UpdateOptions{
					FilterBlocks: filterSystemPromptBlocks,
				},
			})
			if err != nil {
				log.Error().Err(err).Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("run loop error")
			}
			runCancel()
			conv.mu.Lock()
			conv.running = false
			conv.cancel = nil
			conv.mu.Unlock()
			log.Info().Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Msg("run loop finished")
		}(conv)

		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": conv.RunID, "conv_id": conv.ID})
	})
}

// helpers
func fsSub(staticFS embed.FS, path string) (fs.FS, error) { return fs.Sub(staticFS, path) }

// runtime wiring bits
var (
	_ http.Handler
)

func snapshotHookForConv(conv *Conversation, dir string) toolhelpers.SnapshotHook {
	if conv == nil || dir == "" {
		return nil
	}
	return func(ctx context.Context, t *turns.Turn, phase string) {
		if t == nil {
			return
		}
		subdir := filepath.Join(dir, conv.ID, conv.RunID)
		if err := os.MkdirAll(subdir, 0755); err != nil {
			log.Warn().Err(err).Str("dir", subdir).Msg("webchat snapshot: mkdir failed")
			return
		}
		ts := time.Now().UTC().Format("20060102-150405.000000000")
		turnID := t.ID
		if turnID == "" {
			turnID = "turn"
		}
		name := fmt.Sprintf("%s-%s-%s.yaml", ts, phase, turnID)
		path := filepath.Join(subdir, name)
		data, err := serde.ToYAML(t, serde.Options{})
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("webchat snapshot: serialize failed")
			return
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			log.Warn().Err(err).Str("path", path).Msg("webchat snapshot: write failed")
			return
		}
		log.Debug().Str("path", path).Str("phase", phase).Msg("webchat snapshot: saved turn")
	}
}

// private state fields appended to Router
// (declared here for proximity to logic, defined in types.go)
