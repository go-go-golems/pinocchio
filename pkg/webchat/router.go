package webchat

import (
	"context"
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
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	sempb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/base"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// RouterSettings are exposed via parameter layers (addr, agent, idle timeout, etc.).
type RouterSettings struct {
	Addr                 string `glazed:"addr"`
	IdleTimeoutSeconds   int    `glazed:"idle-timeout-seconds"`
	EvictIdleSeconds     int    `glazed:"evict-idle-seconds"`
	EvictIntervalSeconds int    `glazed:"evict-interval-seconds"`
	// Durable timeline projection store configuration (optional).
	// Use either:
	// - timeline-dsn (preferred; full sqlite DSN)
	// - timeline-db (file path; DSN derived)
	TimelineDSN string `glazed:"timeline-dsn"`
	TimelineDB  string `glazed:"timeline-db"`
	// Durable turn snapshot store configuration (optional).
	// Use either:
	// - turns-dsn (preferred; full sqlite DSN)
	// - turns-db (file path; DSN derived)
	TurnsDSN string `glazed:"turns-dsn"`
	TurnsDB  string `glazed:"turns-db"`
	// In-memory timeline store sizing (used when no timeline DB is configured).
	TimelineInMemoryMaxEntities int `glazed:"timeline-inmem-max-entities"`
}

// RouterBuilder creates a new composable webchat router.
func NewRouter(ctx context.Context, parsed *values.Values, staticFS fs.FS, opts ...RouterOption) (*Router, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	rs := rediscfg.Settings{}
	_ = parsed.DecodeSectionInto("redis", &rs)
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
		upgrader:      websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
	// set redis flags for ws reader
	if rs.Enabled {
		r.usesRedis = true
		r.redisAddr = rs.Addr
	}

	// Timeline store for hydration (SQLite when configured, in-memory otherwise).
	s := &RouterSettings{}
	if err := parsed.DecodeSectionInto(values.DefaultSlug, s); err != nil {
		return nil, errors.Wrap(err, "parse router settings")
	}
	if dsn := strings.TrimSpace(s.TimelineDSN); dsn != "" {
		store, err := chatstore.NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open timeline store (dsn)")
		}
		r.timelineStore = store
	} else if p := strings.TrimSpace(s.TimelineDB); p != "" {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		dsn, err := chatstore.SQLiteTimelineDSNForFile(p)
		if err != nil {
			return nil, errors.Wrap(err, "build timeline DSN")
		}
		store, err := chatstore.NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open timeline store (file)")
		}
		r.timelineStore = store
	} else {
		r.timelineStore = chatstore.NewInMemoryTimelineStore(s.TimelineInMemoryMaxEntities)
	}
	if r.cm != nil {
		r.cm.SetTimelineStore(r.timelineStore)
	}

	// Optional turn snapshot store (SQLite when configured).
	if dsn := strings.TrimSpace(s.TurnsDSN); dsn != "" {
		store, err := chatstore.NewSQLiteTurnStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open turn store (dsn)")
		}
		r.turnStore = store
	} else if p := strings.TrimSpace(s.TurnsDB); p != "" {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		dsn, err := chatstore.SQLiteTurnDSNForFile(p)
		if err != nil {
			return nil, errors.Wrap(err, "build turn DSN")
		}
		store, err := chatstore.NewSQLiteTurnStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open turn store (file)")
		}
		r.turnStore = store
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	if r.stepCtrl == nil {
		r.stepCtrl = toolloop.NewStepController()
	}
	if r.cm == nil {
		r.cm = NewConvManager(ConvManagerOptions{
			BaseCtx:            ctx,
			StepController:     r.stepCtrl,
			BuildConfig:        r.BuildConfig,
			BuildFromConfig:    r.BuildFromConfig,
			BuildSubscriber:    r.BuildSubscriber,
			TimelineUpsertHook: r.TimelineUpsertHook,
		})
	}
	if r.requestResolver == nil {
		r.requestResolver = NewDefaultConversationRequestResolver(r.cm)
	}

	if r.cm != nil {
		r.cm.SetTimelineStore(r.timelineStore)
		r.cm.SetIdleTimeoutSeconds(s.IdleTimeoutSeconds)
		r.cm.SetEvictionConfig(
			time.Duration(s.EvictIdleSeconds)*time.Second,
			time.Duration(s.EvictIntervalSeconds)*time.Second,
		)
	}
	r.idleTimeoutSec = s.IdleTimeoutSeconds

	r.registerHTTPHandlers()
	return r, nil
}

// RegisterMiddleware adds a named middleware factory to the router.
func (r *Router) RegisterMiddleware(name string, f MiddlewareFactory) { r.mwFactories[name] = f }

// RegisterTool adds a named tool factory to the router.
func (r *Router) RegisterTool(name string, f ToolFactory) { r.toolFactories[name] = f }

// Mount attaches all handlers to a parent mux with the given prefix.
// http.ServeMux does not strip prefixes, so we must use StripPrefix explicitly.
func (r *Router) Mount(mux *http.ServeMux, prefix string) {
	if prefix == "" || prefix == "/" {
		mux.Handle("/", r.mux)
		return
	}
	prefix = strings.TrimRight(prefix, "/")
	mux.Handle(prefix+"/", http.StripPrefix(prefix, r.mux))
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r0 *http.Request) {
		http.Redirect(w, r0, prefix+"/", http.StatusPermanentRedirect)
	})
}

// Expose lightweight handler registration for external customization (e.g., runtime policy handlers).
func (r *Router) Handle(pattern string, h http.Handler) { r.mux.Handle(pattern, h) }
func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.mux.HandleFunc(pattern, handler)
}

// Handler returns the internal mux as an http.Handler.
func (r *Router) Handler() http.Handler { return r.mux }

// BuildHTTPServer constructs an http.Server using settings from layers.
func (r *Router) BuildHTTPServer() (*http.Server, error) {
	s := &RouterSettings{}
	if err := r.parsed.DecodeSectionInto(values.DefaultSlug, s); err != nil {
		return nil, err
	}
	r.idleTimeoutSec = s.IdleTimeoutSeconds
	if r.cm != nil {
		r.cm.SetIdleTimeoutSeconds(s.IdleTimeoutSeconds)
		r.cm.SetEvictionConfig(
			time.Duration(s.EvictIdleSeconds)*time.Second,
			time.Duration(s.EvictIntervalSeconds)*time.Second,
		)
	}
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
	if r.cm != nil {
		r.cm.StartEvictionLoop(ctx)
	}
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
	r.registerUIHandlers(r.mux)
	r.registerAPIHandlers(r.mux)
}

// APIHandler returns an http.Handler that only exposes API + websocket routes.
func (r *Router) APIHandler() http.Handler {
	mux := http.NewServeMux()
	r.registerAPIHandlers(mux)
	return mux
}

// UIHandler returns an http.Handler that only serves the embedded web UI assets.
func (r *Router) UIHandler() http.Handler {
	mux := http.NewServeMux()
	r.registerUIHandlers(mux)
	return mux
}

func (r *Router) registerUIHandlers(mux *http.ServeMux) {
	logger := log.With().Str("component", "webchat").Logger()

	if r.staticFS == nil {
		logger.Warn().Msg("static FS not configured; UI handler disabled")
		return
	}

	// static assets
	if staticSub, err := fsSub(r.staticFS, "static"); err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
		logger.Info().Msg("mounted /static/ asset handler")
	} else {
		logger.Warn().Err(err).Msg("failed to mount /static/ asset handler")
	}
	if distAssets, err := fsSub(r.staticFS, "static/dist/assets"); err == nil {
		mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(distAssets))))
		logger.Info().Msg("mounted /assets/ handler for built dist assets")
	} else {
		logger.Warn().Err(err).Msg("no built dist assets found under static/dist/assets")
	}
	// index
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if b, err := fs.ReadFile(r.staticFS, "static/dist/index.html"); err == nil {
			logger.Debug().Msg("serving built index (static/dist/index.html)")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		if b, err := fs.ReadFile(r.staticFS, "static/index.html"); err == nil {
			logger.Debug().Msg("serving dev index (static/index.html)")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		logger.Error().Msg("index not found in embedded FS")
		http.Error(w, "index not found", http.StatusInternalServerError)
	})
}

func (r *Router) registerAPIHandlers(mux *http.ServeMux) {
	logger := log.With().Str("component", "webchat").Logger()

	if r.debugRoutesEnabled() {
		r.registerDebugAPIHandlers(mux)
	}

	// websocket join: /ws?conv_id=...&runtime=key
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r0 *http.Request) {
		queryConv := strings.TrimSpace(r0.URL.Query().Get("conv_id"))
		queryRuntime := strings.TrimSpace(r0.URL.Query().Get("runtime"))
		logger.Debug().
			Str("conv_id_query", queryConv).
			Str("runtime_query", queryRuntime).
			Str("remote", r0.RemoteAddr).
			Msg("ws request query")
		conn, err := r.upgrader.Upgrade(w, r0, nil)
		if err != nil {
			logger.Error().Err(err).Msg("websocket upgrade failed")
			return
		}
		resolver := r.requestResolver
		if resolver == nil {
			resolver = NewDefaultConversationRequestResolver(r.cm)
		}
		plan, err := resolver.Resolve(r0)
		if err != nil {
			msg := err.Error()
			var rbe *RequestResolutionError
			if stderrors.As(err, &rbe) && rbe != nil && strings.TrimSpace(rbe.ClientMsg) != "" {
				msg = rbe.ClientMsg
			}
			wsLog := logger.With().Str("remote", r0.RemoteAddr).Logger()
			wsLog.Warn().Err(err).Msg("ws request policy failed")
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+msg+`"}`))
			_ = conn.Close()
			return
		}
		convID := plan.ConvID
		runtimeKey := plan.RuntimeKey
		wsLog := logger.With().
			Str("remote", r0.RemoteAddr).
			Str("conv_id", convID).
			Str("runtime_key", runtimeKey).
			Logger()
		wsLog.Debug().
			Str("conv_id_query", queryConv).
			Str("runtime_query", queryRuntime).
			Msg("ws resolved request")
		wsLog.Info().Msg("ws connect request")
		wsLog.Info().Msg("ws joining conversation")
		conv, err := r.cm.GetOrCreate(convID, runtimeKey, nil)
		if err != nil {
			wsLog.Error().Err(err).Msg("failed to join conversation")
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to join conversation"}`))
			_ = conn.Close()
			return
		}
		r.cm.AddConn(conv, conn)
		wsLog.Info().Msg("ws connected")

		// Send a greeting frame to the newly connected client (mirrors moments/go-go-mento behavior).
		if conv != nil && conv.pool != nil {
			ts := time.Now().UnixMilli()
			data, _ := protoToRaw(&sempb.WsHelloV1{ConvId: convID, RuntimeKey: runtimeKey, ServerTime: ts})
			hello := map[string]any{
				"sem": true,
				"event": map[string]any{
					"type": "ws.hello",
					"id":   fmt.Sprintf("ws.hello:%s:%d", convID, ts),
					"data": data,
				},
			}
			if b, err := json.Marshal(hello); err == nil {
				wsLog.Debug().Msg("ws sending hello")
				conv.pool.SendToOne(conn, b)
			}
		}

		go func() {
			defer r.cm.RemoveConn(conv, conn)
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
							wsLog.Debug().Msg("ws sending pong")
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

		resolver := r.requestResolver
		if resolver == nil {
			resolver = NewDefaultConversationRequestResolver(r.cm)
		}
		plan, err := resolver.Resolve(r0)
		if err != nil {
			status := http.StatusInternalServerError
			msg := "failed to resolve request"
			var rbe *RequestResolutionError
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
		if strings.TrimSpace(plan.Prompt) == "" {
			logger.Warn().Msg("chat request resolver missing prompt")
			http.Error(w, "missing prompt", http.StatusBadRequest)
			return
		}

		convID := plan.ConvID
		runtimeKey := plan.RuntimeKey
		chatReqLog := logger.With().Str("conv_id", convID).Str("runtime_key", runtimeKey).Logger()
		chatReqLog.Info().Int("prompt_len", len(plan.Prompt)).Msg("/chat received")
		if plan.Overrides != nil {
			chatReqLog.Debug().Interface("overrides", plan.Overrides).Msg("/chat overrides")
		} else {
			chatReqLog.Debug().Msg("/chat overrides empty")
		}

		conv, err := r.cm.GetOrCreate(convID, runtimeKey, plan.Overrides)
		if err != nil {
			chatReqLog.Error().Err(err).Msg("failed to create conversation")
			http.Error(w, "failed to create conversation", http.StatusInternalServerError)
			return
		}
		if conv.Sess == nil {
			http.Error(w, "conversation session not initialized", http.StatusInternalServerError)
			return
		}

		sessionLog := logger.With().Str("conv_id", conv.ID).Str("session_id", conv.SessionID).Logger()
		idempotencyKey := strings.TrimSpace(plan.IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = idempotencyKeyFromRequest(r0, nil)
		}

		prep, err := conv.PrepareSessionInference(idempotencyKey, runtimeKey, plan.Overrides, plan.Prompt)
		if err != nil {
			sessionLog.Error().Err(err).Msg("prepare session inference failed")
			http.Error(w, "prepare session inference failed", http.StatusInternalServerError)
			return
		}
		if !prep.Start {
			if status, ok := prep.Response["status"].(string); ok && strings.EqualFold(status, "queued") {
				if pos, ok := prep.Response["queue_position"].(int); ok {
					sessionLog.Info().
						Str("idempotency_key", idempotencyKey).
						Int("queue_position", pos).
						Msg("session inference in progress; queued prompt")
				}
			}
			if prep.HTTPStatus > 0 {
				w.WriteHeader(prep.HTTPStatus)
			}
			_ = json.NewEncoder(w).Encode(prep.Response)
			return
		}

		resp, err := r.startInferenceForPrompt(conv, runtimeKey, plan.Overrides, plan.Prompt, idempotencyKey)
		if err != nil {
			sessionLog.Error().Err(err).Msg("start session inference failed")
			http.Error(w, "start session inference failed", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	mux.HandleFunc("/chat", func(w http.ResponseWriter, r0 *http.Request) { handleChatRequest(w, r0) })
	mux.HandleFunc("/chat/", func(w http.ResponseWriter, r0 *http.Request) { handleChatRequest(w, r0) })
}

// helpers
func fsSub(staticFS fs.FS, path string) (fs.FS, error) { return fs.Sub(staticFS, path) }

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

func snapshotHookForConv(conv *Conversation, dir string, store chatstore.TurnStore) toolloop.SnapshotHook {
	if conv == nil || (dir == "" && store == nil) {
		return nil
	}
	snapLog := log.With().
		Str("component", "webchat").
		Str("conv_id", conv.ID).
		Str("session_id", conv.SessionID).
		Logger()
	return func(ctx context.Context, t *turns.Turn, phase string) {
		if t == nil {
			return
		}
		if store != nil {
			turnID := t.ID
			if turnID == "" {
				turnID = "turn"
			}
			sessionID := conv.SessionID
			if sessionID == "" {
				if v, ok, err := turns.KeyTurnMetaSessionID.Get(t.Metadata); err == nil && ok {
					sessionID = v
				}
			}
			if sessionID != "" {
				payload, err := serde.ToYAML(t, serde.Options{})
				if err != nil {
					snapLog.Warn().Err(err).Str("phase", phase).Msg("webchat snapshot: serialize failed (store)")
				} else if err := store.Save(ctx, conv.ID, sessionID, turnID, phase, time.Now().UnixMilli(), string(payload)); err != nil {
					snapLog.Warn().Err(err).Str("phase", phase).Msg("webchat snapshot: store save failed")
				}
			}
		}
		if dir == "" {
			return
		}
		subdir := filepath.Join(dir, conv.ID, conv.SessionID)
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

func idempotencyKeyFromRequest(r *http.Request, body *ChatRequestBody) string {
	var key string
	if r != nil {
		key = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			key = strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
		}
	}
	if key == "" && body != nil {
		key = strings.TrimSpace(body.IdempotencyKey)
	}
	if key == "" {
		key = uuid.NewString()
	}
	return key
}

func (r *Router) startInferenceForPrompt(conv *Conversation, runtimeKey string, overrides map[string]any, prompt string, idempotencyKey string) (map[string]any, error) {
	if r == nil || conv == nil || conv.Sess == nil {
		return nil, errors.New("invalid conversation")
	}

	sessionLog := log.With().Str("component", "webchat").Str("conv_id", conv.ID).Str("session_id", conv.SessionID).Logger()

	// Ensure the conversation stream is running so SEM frames are produced even without an attached WS client.
	conv.mu.Lock()
	stream := conv.stream
	baseCtx := conv.baseCtx
	conv.mu.Unlock()
	if baseCtx == nil {
		baseCtx = r.baseCtx
	}
	if baseCtx == nil {
		return nil, errors.New("conversation context is nil")
	}
	if stream != nil && !stream.IsRunning() {
		_ = stream.Start(baseCtx)
	}

	cfg, err := r.BuildConfig(runtimeKey, overrides)
	if err != nil {
		r.finishSessionInference(conv, idempotencyKey, "", "", err)
		return nil, err
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

	hook := snapshotHookForConv(conv, os.Getenv("PINOCCHIO_WEBCHAT_TURN_SNAPSHOTS_DIR"), r.turnStore)

	seed, err := conv.Sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		r.finishSessionInference(conv, idempotencyKey, "", "", err)
		return nil, err
	}
	turnID := ""
	if seed != nil && seed.ID != "" {
		turnID = seed.ID
	}
	if r.timelineStore != nil && turnID != "" && strings.TrimSpace(prompt) != "" {
		entity := &timelinepb.TimelineEntityV1{
			Id:   "user-" + turnID,
			Kind: "message",
			Snapshot: &timelinepb.TimelineEntityV1_Message{
				Message: &timelinepb.MessageSnapshotV1{
					SchemaVersion: 1,
					Role:          "user",
					Content:       prompt,
					Streaming:     false,
				},
			},
		}
		version := uint64(time.Now().UnixMilli()) * 1_000_000
		if r.baseCtx == nil {
			return nil, errors.New("router context is nil")
		}
		if err := r.timelineStore.Upsert(r.baseCtx, conv.ID, version, entity); err == nil {
			r.emitTimelineUpsert(conv, entity, version)
		}
	}

	if stepModeFromOverrides(overrides) && r.stepCtrl != nil {
		r.stepCtrl.Enable(toolloop.StepScope{SessionID: conv.SessionID, ConversationID: conv.ID})
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
		Persister:        newTurnStorePersister(r.turnStore, conv, "final"),
	}

	sessionLog.Info().Str("idempotency_key", idempotencyKey).Msg("starting inference loop")

	if r.baseCtx == nil {
		return nil, errors.New("router context is nil")
	}
	handle, err := conv.Sess.StartInference(r.baseCtx)
	if err != nil {
		r.finishSessionInference(conv, idempotencyKey, "", turnID, err)
		return nil, err
	}
	if handle == nil {
		err := errors.New("start inference returned nil handle")
		r.finishSessionInference(conv, idempotencyKey, "", turnID, err)
		return nil, err
	}

	resp := map[string]any{
		"status":          "started",
		"idempotency_key": idempotencyKey,
		"conv_id":         conv.ID,
		"session_id":      conv.SessionID,
	}
	if turnID != "" {
		resp["turn_id"] = turnID
	}
	if handle.InferenceID != "" {
		resp["inference_id"] = handle.InferenceID
	}
	if handle.Input != nil && handle.Input.ID != "" {
		resp["turn_id"] = handle.Input.ID
	}

	conv.mu.Lock()
	conv.ensureQueueInitLocked()
	if rec, ok := conv.getRecordLocked(idempotencyKey); ok && rec != nil {
		rec.Status = "running"
		rec.StartedAt = time.Now()
		rec.Response = resp
	} else {
		conv.upsertRecordLocked(&chatRequestRecord{IdempotencyKey: idempotencyKey, Status: "running", StartedAt: time.Now(), Response: resp})
	}
	conv.mu.Unlock()

	go func() {
		_, waitErr := handle.Wait()
		r.finishSessionInference(conv, idempotencyKey, handle.InferenceID, turnID, waitErr)
		if waitErr != nil {
			sessionLog.Error().Err(waitErr).Str("inference_id", handle.InferenceID).Msg("inference loop error")
		}
		sessionLog.Info().Str("inference_id", handle.InferenceID).Msg("inference loop finished")
		r.tryDrainQueue(conv)
	}()

	return resp, nil
}

func (r *Router) finishSessionInference(conv *Conversation, idempotencyKey string, inferenceID string, turnID string, err error) {
	if conv == nil {
		return
	}
	conv.mu.Lock()
	defer conv.mu.Unlock()

	if conv.activeRequestKey == idempotencyKey {
		conv.activeRequestKey = ""
	}
	conv.touchLocked(time.Now())
	conv.ensureQueueInitLocked()
	if rec, ok := conv.getRecordLocked(idempotencyKey); ok && rec != nil {
		if err != nil {
			rec.Status = "error"
			rec.Error = err.Error()
		} else if rec.Status == "running" {
			rec.Status = "completed"
		}
		rec.CompletedAt = time.Now()
		if rec.Response == nil {
			rec.Response = map[string]any{}
		}
		if inferenceID != "" {
			rec.Response["inference_id"] = inferenceID
		}
		if turnID != "" {
			rec.Response["turn_id"] = turnID
		}
		rec.Response["status"] = rec.Status
	}
}

func (r *Router) tryDrainQueue(conv *Conversation) {
	if r == nil || conv == nil {
		return
	}
	for {
		q, ok := conv.ClaimNextQueued()
		if !ok {
			return
		}
		_, err := r.startInferenceForPrompt(conv, q.RuntimeKey, q.Overrides, q.Prompt, q.IdempotencyKey)
		if err != nil {
			r.finishSessionInference(conv, q.IdempotencyKey, "", "", err)
			// Continue draining so later queued items can still execute.
			continue
		}
		// Successfully started one inference; subsequent items are handled when it finishes.
		return
	}
}

// BuildSubscriber exposes the subscriber builder for external use.
func (r *Router) BuildSubscriber(convID string) (message.Subscriber, bool, error) {
	if r != nil && r.buildSubscriberOverride != nil {
		return r.buildSubscriberOverride(convID)
	}
	return r.buildSubscriberDefault(convID)
}

func (r *Router) buildSubscriberDefault(convID string) (message.Subscriber, bool, error) {
	if r == nil {
		return nil, false, errors.New("router is nil")
	}
	if convID == "" {
		return nil, false, errors.New("convID is empty")
	}
	// subscriber/publisher
	if r.usesRedis {
		if r.baseCtx == nil {
			return nil, false, errors.New("router context is nil")
		}
		_ = rediscfg.EnsureGroupAtTail(r.baseCtx, r.redisAddr, topicForConv(convID), "ui")
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
