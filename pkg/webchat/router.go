package webchat

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	sempb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/base"
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
		stepCtrl:      toolloop.NewStepController(),
	}
	r.engineFromReqBuilder = NewDefaultEngineFromReqBuilder(r.profiles, r.cm)
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
	logger := log.With().Str("component", "webchat").Logger()
	logger.Info().Msg("starting event router loop")
	err := r.router.Run(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("event router exited with error")
		return err
	}
	logger.Info().Msg("event router loop exited")
	return nil
}

// registerHTTPHandlers sets up static, API and websockets.
func (r *Router) registerHTTPHandlers() {
	logger := log.With().Str("component", "webchat").Logger()

	// static assets
	if staticSub, err := fsSub(r.staticFS, "static"); err == nil {
		r.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
		logger.Info().Msg("mounted /static/ asset handler")
	} else {
		logger.Warn().Err(err).Msg("failed to mount /static/ asset handler")
	}
	if distAssets, err := fsSub(r.staticFS, "static/dist/assets"); err == nil {
		r.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(distAssets))))
		logger.Info().Msg("mounted /assets/ handler for built dist assets")
	} else {
		logger.Warn().Err(err).Msg("no built dist assets found under static/dist/assets")
	}
	// index
	r.mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if b, err := r.staticFS.ReadFile("static/dist/index.html"); err == nil {
			logger.Debug().Msg("serving built index (static/dist/index.html)")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		if b, err := r.staticFS.ReadFile("static/index.html"); err == nil {
			logger.Debug().Msg("serving dev index (static/index.html)")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		logger.Error().Msg("index not found in embedded FS")
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

	// debug endpoints (dev-gated via PINOCCHIO_WEBCHAT_DEBUG=1)
	r.mux.HandleFunc("/debug/step/enable", func(w http.ResponseWriter, r0 *http.Request) {
		if os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") != "1" {
			http.NotFound(w, r0)
			return
		}
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ConvID    string `json:"conv_id"`
			SessionID string `json:"session_id"`
			Owner     string `json:"owner"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		sessionID := strings.TrimSpace(body.SessionID)
		convID := strings.TrimSpace(body.ConvID)
		if sessionID == "" && convID != "" {
			r.cm.mu.Lock()
			if c, ok := r.cm.conns[convID]; ok && c != nil {
				sessionID = c.RunID
			}
			r.cm.mu.Unlock()
		}
		if sessionID == "" {
			http.Error(w, "missing session_id (or unknown conv_id)", http.StatusBadRequest)
			return
		}
		if r.stepCtrl == nil {
			http.Error(w, "step controller not initialized", http.StatusInternalServerError)
			return
		}
		r.stepCtrl.Enable(toolloop.StepScope{SessionID: sessionID, ConversationID: convID, Owner: strings.TrimSpace(body.Owner)})
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "session_id": sessionID, "conv_id": convID})
	})

	r.mux.HandleFunc("/debug/step/disable", func(w http.ResponseWriter, r0 *http.Request) {
		if os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") != "1" {
			http.NotFound(w, r0)
			return
		}
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			SessionID string `json:"session_id"`
			ConvID    string `json:"conv_id"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		sessionID := strings.TrimSpace(body.SessionID)
		convID := strings.TrimSpace(body.ConvID)
		if sessionID == "" && convID != "" {
			r.cm.mu.Lock()
			if c, ok := r.cm.conns[convID]; ok && c != nil {
				sessionID = c.RunID
			}
			r.cm.mu.Unlock()
		}
		if sessionID == "" {
			http.Error(w, "missing session_id (or unknown conv_id)", http.StatusBadRequest)
			return
		}
		if r.stepCtrl != nil {
			r.stepCtrl.DisableSession(sessionID)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "session_id": sessionID})
	})

	r.mux.HandleFunc("/debug/continue", func(w http.ResponseWriter, r0 *http.Request) {
		if os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") != "1" {
			http.NotFound(w, r0)
			return
		}
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			PauseID string `json:"pause_id"`
			ConvID  string `json:"conv_id,omitempty"`
		}
		if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		pauseID := strings.TrimSpace(body.PauseID)
		if pauseID == "" {
			http.Error(w, "missing pause_id", http.StatusBadRequest)
			return
		}
		if r.stepCtrl == nil {
			http.Error(w, "step controller not initialized", http.StatusInternalServerError)
			return
		}
		if convID := strings.TrimSpace(body.ConvID); convID != "" {
			if meta, ok := r.stepCtrl.Lookup(pauseID); ok {
				if meta.Scope.ConversationID != "" && meta.Scope.ConversationID != convID {
					http.Error(w, "pause does not belong to this conversation", http.StatusForbidden)
					return
				}
			}
		}
		meta, ok := r.stepCtrl.Continue(pauseID)
		if !ok {
			http.Error(w, "unknown pause_id", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "pause": meta})
	})

	// websocket join: /ws?conv_id=...&profile=slug (falls back to chat_profile cookie)
	r.mux.HandleFunc("/ws", func(w http.ResponseWriter, r0 *http.Request) {
		conn, err := r.upgrader.Upgrade(w, r0, nil)
		if err != nil {
			logger.Error().Err(err).Msg("websocket upgrade failed")
			return
		}
		b := r.engineFromReqBuilder
		if b == nil {
			b = NewDefaultEngineFromReqBuilder(r.profiles, r.cm)
		}
		input, _, err := b.BuildEngineFromReq(r0)
		if err != nil {
			msg := err.Error()
			var rbe *RequestBuildError
			if stderrors.As(err, &rbe) && rbe != nil && strings.TrimSpace(rbe.ClientMsg) != "" {
				msg = rbe.ClientMsg
			}
			wsLog := logger.With().Str("remote", r0.RemoteAddr).Logger()
			wsLog.Warn().Err(err).Msg("ws request policy failed")
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+msg+`"}`))
			_ = conn.Close()
			return
		}
		convID := input.ConvID
		profileSlug := input.ProfileSlug
		wsLog := logger.With().
			Str("remote", r0.RemoteAddr).
			Str("conv_id", convID).
			Str("profile", profileSlug).
			Logger()
		wsLog.Info().Msg("ws connect request")
		wsLog.Info().Msg("ws joining conversation")
		conv, err := r.getOrCreateConv(convID, profileSlug, nil)
		if err != nil {
			wsLog.Error().Err(err).Msg("failed to join conversation")
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
			_ = conn.Close()
			return
		}
		r.addConn(conv, conn)
		wsLog.Info().Msg("ws connected")

		// Send a greeting frame to the newly connected client (mirrors moments/go-go-mento behavior).
		if conv != nil && conv.pool != nil {
			ts := time.Now().UnixMilli()
			data, _ := protoToRaw(&sempb.WsHelloV1{ConvId: convID, Profile: profileSlug, ServerTime: ts})
			hello := map[string]any{
				"sem": true,
				"event": map[string]any{
					"type": "ws.hello",
					"id":   fmt.Sprintf("ws.hello:%s:%d", convID, ts),
					"data": data,
				},
			}
			if b, err := json.Marshal(hello); err == nil {
				conv.pool.SendToOne(conn, b)
			}
		}

		go func() {
			defer r.removeConn(conv, conn)
			defer wsLog.Info().Msg("ws disconnected")
			for {
				msgType, data, err := conn.ReadMessage()
				if err != nil {
					wsLog.Debug().Err(err).Msg("ws read loop end")
					return
				}

				// Lightweight ping/pong protocol (mirrors moments/go-go-mento behavior)
				if msgType == websocket.TextMessage && len(data) > 0 && conv != nil && conv.pool != nil {
					s := strings.TrimSpace(strings.ToLower(string(data)))
					isPing := s == "ping"
					if !isPing {
						var v map[string]any
						if err := json.Unmarshal(data, &v); err == nil && v != nil {
							if t, ok := v["type"].(string); ok && strings.EqualFold(t, "ws.ping") {
								isPing = true
							} else if sem, ok := v["sem"].(bool); ok && sem {
								if ev, ok := v["event"].(map[string]any); ok {
									if t2, ok := ev["type"].(string); ok && strings.EqualFold(t2, "ws.ping") {
										isPing = true
									}
								}
							}
						}
					}
					if isPing {
						ts := time.Now().UnixMilli()
						data, _ := protoToRaw(&sempb.WsPongV1{ConvId: convID, ServerTime: ts})
						pong := map[string]any{
							"sem": true,
							"event": map[string]any{
								"type": "ws.pong",
								"id":   fmt.Sprintf("ws.pong:%s:%d", convID, ts),
								"data": data,
							},
						}
						if b, err := json.Marshal(pong); err == nil {
							conv.pool.SendToOne(conn, b)
						}
					}
				}
			}
		}()
	})

	handleChatRequest := func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		b := r.engineFromReqBuilder
		if b == nil {
			b = NewDefaultEngineFromReqBuilder(r.profiles, r.cm)
		}
		input, body, err := b.BuildEngineFromReq(r0)
		if err != nil {
			status := http.StatusInternalServerError
			msg := "failed to resolve request"
			var rbe *RequestBuildError
			if stderrors.As(err, &rbe) && rbe != nil {
				if rbe.Status > 0 {
					status = rbe.Status
				}
				if strings.TrimSpace(rbe.ClientMsg) != "" {
					msg = rbe.ClientMsg
				}
			}
			logger.Warn().Err(err).Msg("chat request policy failed")
			http.Error(w, msg, status)
			return
		}
		if body == nil {
			logger.Warn().Msg("chat request policy missing parsed body")
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		convID := input.ConvID
		profileSlug := input.ProfileSlug
		chatReqLog := logger.With().Str("conv_id", convID).Str("profile", profileSlug).Logger()
		chatReqLog.Info().Int("prompt_len", len(body.Prompt)).Msg("/chat received")

		conv, err := r.getOrCreateConv(convID, profileSlug, input.Overrides)
		if err != nil {
			chatReqLog.Error().Err(err).Msg("failed to create conversation")
			http.Error(w, "failed to create conversation", http.StatusInternalServerError)
			return
		}
		if conv.Sess == nil {
			http.Error(w, "conversation session not initialized", http.StatusInternalServerError)
			return
		}

		runLog := logger.With().Str("conv_id", conv.ID).Str("run_id", conv.RunID).Str("session_id", conv.RunID).Logger()
		if conv.Sess.IsRunning() {
			runLog.Warn().Msg("run in progress")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":      "run in progress",
				"conv_id":    conv.ID,
				"run_id":     conv.RunID, // legacy
				"session_id": conv.RunID,
			})
			return
		}

		cfg, err := r.BuildConfig(profileSlug, input.Overrides)
		if err != nil {
			runLog.Error().Err(err).Msg("build config failed")
			http.Error(w, "build config failed", http.StatusInternalServerError)
			return
		}

		tmpReg := geptools.NewInMemoryToolRegistry()
		for _, tf := range r.toolFactories {
			_ = tf(tmpReg)
		}
		registry := geptools.NewInMemoryToolRegistry()
		if len(cfg.Tools) == 0 {
			for _, td := range tmpReg.ListTools() {
				_ = registry.RegisterTool(td.Name, td)
			}
		} else {
			allowed := map[string]struct{}{}
			for _, n := range cfg.Tools {
				if s := strings.TrimSpace(n); s != "" {
					allowed[s] = struct{}{}
				}
			}
			for _, td := range tmpReg.ListTools() {
				if _, ok := allowed[td.Name]; ok {
					_ = registry.RegisterTool(td.Name, td)
				}
			}
		}

		// Ensure router is running before we start inference (best-effort).
		select {
		case <-r.router.Running():
		case <-time.After(2 * time.Second):
		}

		hook := snapshotHookForConv(conv, os.Getenv("PINOCCHIO_WEBCHAT_TURN_SNAPSHOTS_DIR"))
		runLog.Info().Msg("starting run loop")

		seed, err := conv.Sess.AppendNewTurnFromUserPrompt(body.Prompt)
		if err != nil {
			runLog.Error().Err(err).Msg("append prompt turn failed")
			http.Error(w, "append prompt turn failed", http.StatusInternalServerError)
			return
		}

		if stepModeFromOverrides(body.Overrides) && r.stepCtrl != nil {
			r.stepCtrl.Enable(toolloop.StepScope{SessionID: conv.RunID, ConversationID: conv.ID})
		}

		loopCfg := toolloop.NewLoopConfig().WithMaxIterations(5)
		toolCfg := geptools.DefaultToolConfig().WithExecutionTimeout(60 * time.Second)
		conv.Sess.Builder = &enginebuilder.Builder{
			Base:             conv.Eng,
			Registry:         registry,
			LoopConfig:       &loopCfg,
			ToolConfig:       &toolCfg,
			EventSinks:       []events.EventSink{conv.Sink},
			SnapshotHook:     hook,
			StepController:   r.stepCtrl,
			StepPauseTimeout: 30 * time.Second,
		}

		handle, err := conv.Sess.StartInference(r.baseCtx)
		if err != nil {
			runLog.Error().Err(err).Msg("start inference failed")
		} else {
			go func() {
				_, waitErr := handle.Wait()
				if waitErr != nil {
					runLog.Error().
						Err(waitErr).
						Str("inference_id", handle.InferenceID).
						Msg("run loop error")
				}
				runLog.Info().
					Str("inference_id", handle.InferenceID).
					Msg("run loop finished")
			}()
		}

		resp := map[string]string{
			"conv_id":    conv.ID,
			"run_id":     conv.RunID, // legacy
			"session_id": conv.RunID,
		}
		if seed != nil && seed.ID != "" {
			resp["turn_id"] = seed.ID
		}
		if handle != nil {
			if handle.InferenceID != "" {
				resp["inference_id"] = handle.InferenceID
			}
			if handle.Input != nil && handle.Input.ID != "" {
				resp["turn_id"] = handle.Input.ID
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	r.mux.HandleFunc("/chat", func(w http.ResponseWriter, r0 *http.Request) { handleChatRequest(w, r0) })
	r.mux.HandleFunc("/chat/", func(w http.ResponseWriter, r0 *http.Request) { handleChatRequest(w, r0) })
}

// helpers
func fsSub(staticFS embed.FS, path string) (fs.FS, error) { return fs.Sub(staticFS, path) }

// runtime wiring bits
var (
	_ http.Handler
)

func stepModeFromOverrides(overrides map[string]any) bool {
	if overrides == nil {
		return false
	}
	if v, ok := overrides["step_mode"].(bool); ok {
		return v
	}
	if v, ok := overrides["step_mode"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		default:
			return false
		}
	}
	return false
}

func snapshotHookForConv(conv *Conversation, dir string) toolloop.SnapshotHook {
	if conv == nil || dir == "" {
		return nil
	}
	snapLog := log.With().
		Str("component", "webchat").
		Str("conv_id", conv.ID).
		Str("run_id", conv.RunID).
		Logger()
	return func(ctx context.Context, t *turns.Turn, phase string) {
		if t == nil {
			return
		}
		subdir := filepath.Join(dir, conv.ID, conv.RunID)
		if err := os.MkdirAll(subdir, 0755); err != nil {
			snapLog.Warn().Err(err).Str("dir", subdir).Msg("webchat snapshot: mkdir failed")
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
			snapLog.Warn().Err(err).Str("path", path).Msg("webchat snapshot: serialize failed")
			return
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			snapLog.Warn().Err(err).Str("path", path).Msg("webchat snapshot: write failed")
			return
		}
		snapLog.Debug().Str("path", path).Str("phase", phase).Msg("webchat snapshot: saved turn")
	}
}

func (r *Router) buildSubscriber(convID string) (message.Subscriber, bool, error) {
	if r == nil {
		return nil, false, errors.New("router is nil")
	}
	if convID == "" {
		return nil, false, errors.New("convID is empty")
	}
	// subscriber/publisher
	if r.usesRedis {
		_ = rediscfg.EnsureGroupAtTail(context.Background(), r.redisAddr, topicForConv(convID), "ui")
		sub, err := rediscfg.BuildGroupSubscriber(r.redisAddr, "ui", "ws-forwarder:"+convID)
		if err != nil {
			return nil, false, err
		}
		return sub, true, nil
	}
	return r.router.Subscriber, false, nil
}

// private state fields appended to Router
// (declared here for proximity to logic, defined in types.go)
