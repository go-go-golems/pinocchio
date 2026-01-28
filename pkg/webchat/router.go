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
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	inevents "github.com/go-go-golems/pinocchio/pkg/inference/events"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
	sempb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/base"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
	"google.golang.org/protobuf/encoding/protojson"
)

// RouterSettings are exposed via parameter layers (addr, agent, idle timeout, etc.).
type RouterSettings struct {
	Addr               string `glazed.parameter:"addr"`
	IdleTimeoutSeconds int    `glazed.parameter:"idle-timeout-seconds"`
	// Durable timeline projection store configuration (optional).
	// Use either:
	// - timeline-dsn (preferred; full sqlite DSN)
	// - timeline-db (file path; DSN derived)
	TimelineDSN string `glazed.parameter:"timeline-dsn"`
	TimelineDB  string `glazed.parameter:"timeline-db"`
	// In-memory timeline store sizing (used when no timeline DB is configured).
	TimelineInMemoryMaxEntities int `glazed.parameter:"timeline-inmem-max-entities"`
	// Optional: emit stub "agentic" planning/thinking events so the UI can render
	// planning widgets even when no real planning middleware is configured.
	EmitPlanningStubs bool `glazed.parameter:"emit-planning-stubs"`
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

	// Timeline store for hydration (SQLite when configured, in-memory otherwise).
	s := &RouterSettings{}
	if err := parsed.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return nil, errors.Wrap(err, "parse router settings")
	}
	r.emitPlanningStubs = s.EmitPlanningStubs
	if dsn := strings.TrimSpace(s.TimelineDSN); dsn != "" {
		store, err := NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open timeline store (dsn)")
		}
		r.timelineStore = store
	} else if p := strings.TrimSpace(s.TimelineDB); p != "" {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		dsn, err := SQLiteTimelineDSNForFile(p)
		if err != nil {
			return nil, errors.Wrap(err, "build timeline DSN")
		}
		store, err := NewSQLiteTimelineStore(dsn)
		if err != nil {
			return nil, errors.Wrap(err, "open timeline store (file)")
		}
		r.timelineStore = store
	} else {
		r.timelineStore = NewInMemoryTimelineStore(s.TimelineInMemoryMaxEntities)
	}

	r.registerHTTPHandlers()
	return r, nil
}

// Allow setting optional shared DB for middlewares that need it (e.g., sqlite tool)
func (r *Router) WithDB(db *sql.DB) *Router                 { r.db = db; return r }
func (r *Router) WithTimelineStore(s TimelineStore) *Router { r.timelineStore = s; return r }

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

	// get/set current profile (cookie-backed)
	r.mux.HandleFunc("/api/chat/profile", func(w http.ResponseWriter, r0 *http.Request) {
		type profilePayload struct {
			Slug    string `json:"slug"`
			Profile string `json:"profile"`
		}
		writeJSON := func(payload any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		}
		resolveDefault := func() string {
			if p, ok := r.profiles.Get("default"); ok && p != nil {
				return p.Slug
			}
			list := r.profiles.List()
			if len(list) > 0 && list[0] != nil {
				return list[0].Slug
			}
			return "default"
		}
		switch r0.Method {
		case http.MethodGet:
			slug := ""
			if ck, err := r0.Cookie("chat_profile"); err == nil && ck != nil {
				slug = strings.TrimSpace(ck.Value)
			}
			if slug == "" {
				slug = resolveDefault()
			} else if _, ok := r.profiles.Get(slug); !ok {
				slug = resolveDefault()
			}
			writeJSON(profilePayload{Slug: slug})
			return
		case http.MethodPost:
			var body profilePayload
			if err := json.NewDecoder(r0.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			slug := strings.TrimSpace(body.Slug)
			if slug == "" {
				slug = strings.TrimSpace(body.Profile)
			}
			if slug == "" {
				http.Error(w, "missing profile slug", http.StatusBadRequest)
				return
			}
			if _, ok := r.profiles.Get(slug); !ok {
				http.Error(w, "profile not found", http.StatusNotFound)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "chat_profile", Value: slug, Path: "/", SameSite: http.SameSiteLaxMode})
			writeJSON(profilePayload{Slug: slug})
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
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

	timelineHandler := func(w http.ResponseWriter, r0 *http.Request) {
		if r0.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.timelineStore == nil {
			http.Error(w, "timeline store not enabled", http.StatusNotFound)
			return
		}

		convID := strings.TrimSpace(r0.URL.Query().Get("conv_id"))
		if convID == "" {
			http.Error(w, "missing conv_id", http.StatusBadRequest)
			return
		}

		var sinceVersion uint64
		if s := strings.TrimSpace(r0.URL.Query().Get("since_version")); s != "" {
			_, _ = fmt.Sscanf(s, "%d", &sinceVersion)
		}
		limit := 0
		if s := strings.TrimSpace(r0.URL.Query().Get("limit")); s != "" {
			var v int
			_, _ = fmt.Sscanf(s, "%d", &v)
			if v > 0 {
				limit = v
			}
		}

		snap, err := r.timelineStore.GetSnapshot(r0.Context(), convID, sinceVersion, limit)
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Msg("timeline snapshot failed")
			http.Error(w, "timeline snapshot failed", http.StatusInternalServerError)
			return
		}
		out, err := protojson.MarshalOptions{
			EmitUnpopulated: false,
			UseProtoNames:   false,
		}.Marshal(snap)
		if err != nil {
			logger.Error().Err(err).Str("conv_id", convID).Msg("timeline marshal failed")
			http.Error(w, "timeline marshal failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}
	r.mux.HandleFunc("/timeline", timelineHandler)
	r.mux.HandleFunc("/timeline/", timelineHandler)

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
		idempotencyKey := idempotencyKeyFromRequest(r0, body)

		// Fast idempotency path (returns the previously computed response).
		conv.mu.Lock()
		conv.ensureQueueInitLocked()
		if rec, ok := conv.getRecordLocked(idempotencyKey); ok && rec != nil && rec.Response != nil {
			status := strings.ToLower(strings.TrimSpace(rec.Status))
			resp := make(map[string]any, len(rec.Response))
			for k, v := range rec.Response {
				resp[k] = v
			}
			conv.mu.Unlock()
			switch status {
			case "queued":
				w.WriteHeader(http.StatusAccepted)
			default:
				w.WriteHeader(http.StatusOK)
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Busy -> enqueue and return 202 Accepted.
		if conv.isBusyLocked() {
			pos := conv.enqueueLocked(queuedChat{
				IdempotencyKey: idempotencyKey,
				ProfileSlug:    profileSlug,
				Overrides:      input.Overrides,
				Prompt:         body.Prompt,
				EnqueuedAt:     time.Now(),
			})
			resp := map[string]any{
				"status":          "queued",
				"queue_position":  pos,
				"queue_depth":     len(conv.queue),
				"idempotency_key": idempotencyKey,
				"conv_id":         conv.ID,
				"run_id":          conv.RunID, // legacy
				"session_id":      conv.RunID,
			}
			conv.upsertRecordLocked(&chatRequestRecord{
				IdempotencyKey: idempotencyKey,
				Status:         "queued",
				EnqueuedAt:     time.Now(),
				Response:       resp,
			})
			conv.mu.Unlock()

			runLog.Info().
				Str("idempotency_key", idempotencyKey).
				Int("queue_position", pos).
				Msg("run in progress; queued prompt")

			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Not busy -> claim running slot before starting inference (prevents concurrent starts).
		conv.runningKey = idempotencyKey
		conv.upsertRecordLocked(&chatRequestRecord{
			IdempotencyKey: idempotencyKey,
			Status:         "running",
			StartedAt:      time.Now(),
			Response: map[string]any{
				"status":          "running",
				"idempotency_key": idempotencyKey,
				"conv_id":         conv.ID,
				"run_id":          conv.RunID, // legacy
				"session_id":      conv.RunID,
			},
		})
		conv.mu.Unlock()

		resp, err := r.startRunForPrompt(conv, profileSlug, input.Overrides, body.Prompt, idempotencyKey)
		if err != nil {
			runLog.Error().Err(err).Msg("start run failed")
			http.Error(w, "start run failed", http.StatusInternalServerError)
			return
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

func (r *Router) startRunForPrompt(conv *Conversation, profileSlug string, overrides map[string]any, prompt string, idempotencyKey string) (map[string]any, error) {
	if r == nil || conv == nil || conv.Sess == nil {
		return nil, errors.New("invalid conversation")
	}

	runLog := log.With().Str("component", "webchat").Str("conv_id", conv.ID).Str("run_id", conv.RunID).Str("session_id", conv.RunID).Logger()

	// Ensure the conversation stream is running so SEM frames are produced even without an attached WS client.
	conv.mu.Lock()
	stream := conv.stream
	baseCtx := conv.baseCtx
	conv.mu.Unlock()
	if stream != nil && !stream.IsRunning() {
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		_ = stream.Start(baseCtx)
	}

	cfg, err := r.BuildConfig(profileSlug, overrides)
	if err != nil {
		r.finishRun(conv, idempotencyKey, "", "", err)
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

	hook := snapshotHookForConv(conv, os.Getenv("PINOCCHIO_WEBCHAT_TURN_SNAPSHOTS_DIR"))

	seed, err := conv.Sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		r.finishRun(conv, idempotencyKey, "", "", err)
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
		if v, err := r.timelineStore.Upsert(r.baseCtx, conv.ID, entity); err == nil {
			r.emitTimelineUpsert(conv, entity, v)
		}
	}

	if stepModeFromOverrides(overrides) && r.stepCtrl != nil {
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

	runLog.Info().Str("idempotency_key", idempotencyKey).Msg("starting run loop")

	handle, err := conv.Sess.StartInference(r.baseCtx)
	if err != nil {
		r.finishRun(conv, idempotencyKey, "", turnID, err)
		return nil, err
	}
	if handle == nil {
		err := errors.New("start inference returned nil handle")
		r.finishRun(conv, idempotencyKey, "", turnID, err)
		return nil, err
	}

	agenticRunID := handle.InferenceID
	if agenticRunID == "" {
		agenticRunID = uuid.NewString()
	}
	agenticDirective := fmt.Sprintf("Respond to the user prompt (turn %s).", turnID)
	if r.emitPlanningStubs {
		r.emitAgenticPlanningAndThinking(runLog, conv, cfg, agenticRunID, turnID, handle.InferenceID, agenticDirective)
	}

	resp := map[string]any{
		"status":          "started",
		"idempotency_key": idempotencyKey,
		"conv_id":         conv.ID,
		"run_id":          conv.RunID, // legacy
		"session_id":      conv.RunID,
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
		var finalTurnID string
		if v, ok := resp["turn_id"].(string); ok {
			finalTurnID = v
		}
		if finalTurnID == "" {
			finalTurnID = turnID
		}
		if r.emitPlanningStubs {
			r.emitAgenticExecutionComplete(runLog, conv, agenticRunID, finalTurnID, handle.InferenceID, waitErr)
		}
		r.finishRun(conv, idempotencyKey, handle.InferenceID, turnID, waitErr)
		if waitErr != nil {
			runLog.Error().Err(waitErr).Str("inference_id", handle.InferenceID).Msg("run loop error")
		}
		runLog.Info().Str("inference_id", handle.InferenceID).Msg("run loop finished")
		r.tryDrainQueue(conv)
	}()

	return resp, nil
}

func middlewareEnabled(mws []MiddlewareUse, name string) bool {
	for _, mw := range mws {
		if mw.Name == name {
			return true
		}
	}
	return false
}

func (r *Router) emitAgenticPlanningAndThinking(
	logger zerolog.Logger,
	conv *Conversation,
	cfg EngineConfig,
	agenticRunID string,
	turnID string,
	inferenceID string,
	directive string,
) {
	if conv == nil || conv.Sink == nil {
		return
	}

	provider := ""
	model := ""
	if cfg.StepSettings != nil && cfg.StepSettings.Chat != nil {
		if cfg.StepSettings.Chat.ApiType != nil {
			provider = string(*cfg.StepSettings.Chat.ApiType)
		}
		if cfg.StepSettings.Chat.Engine != nil {
			model = *cfg.StepSettings.Chat.Engine
		}
	}

	md := events.EventMetadata{
		ID:          uuid.New(),
		SessionID:   conv.RunID,
		InferenceID: inferenceID,
		TurnID:      turnID,
	}

	// Thinking mode: emit a simple selection/completion pair so the UI can render.
	itemID := agenticRunID + ":thinking-mode"
	thinking := &inevents.ThinkingModePayload{
		Mode:      "deep",
		Phase:     "selected",
		Reasoning: "Selected thinking mode for this run.",
	}
	_ = conv.Sink.PublishEvent(inevents.NewThinkingModeStarted(md, itemID, thinking))
	_ = conv.Sink.PublishEvent(inevents.NewThinkingModeCompleted(md, itemID, thinking, true, ""))

	// Planning: minimal, single-iteration plan that leads directly into execution.
	started := time.Now().UnixMilli()
	_ = conv.Sink.PublishEvent(inevents.NewPlanningStart(md, agenticRunID, provider, model, 1, started))

	iter := inevents.NewPlanningIteration(md, agenticRunID, 1, "respond", "Direct response", "Executing")
	iter.Provider = provider
	iter.PlannerModel = model
	iter.MaxIterations = 1
	iter.Reasoning = "Proceed directly to execution for this prompt."
	iter.EmittedAtUnixMs = time.Now().UnixMilli()
	iter.ReflectionText = "No additional planning required."
	_ = conv.Sink.PublishEvent(iter)

	done := inevents.NewPlanningComplete(md, agenticRunID, 1, "execute")
	done.Provider = provider
	done.PlannerModel = model
	done.MaxIterations = 1
	done.StatusReason = "auto"
	done.FinalDirective = directive
	_ = conv.Sink.PublishEvent(done)

	_ = conv.Sink.PublishEvent(inevents.NewExecutionStart(md, agenticRunID, model, directive))

	logger.Debug().Str("agentic_run_id", agenticRunID).Msg("emitted planning/thinking-mode semantic events")
}

func (r *Router) emitAgenticExecutionComplete(
	logger zerolog.Logger,
	conv *Conversation,
	agenticRunID string,
	turnID string,
	inferenceID string,
	waitErr error,
) {
	if conv == nil || conv.Sink == nil {
		return
	}
	md := events.EventMetadata{
		ID:          uuid.New(),
		SessionID:   conv.RunID,
		InferenceID: inferenceID,
		TurnID:      turnID,
	}
	status := "completed"
	errMsg := ""
	if waitErr != nil {
		status = "error"
		errMsg = waitErr.Error()
	}
	_ = conv.Sink.PublishEvent(inevents.NewExecutionComplete(md, agenticRunID, status, errMsg))
	logger.Debug().Str("agentic_run_id", agenticRunID).Str("status", status).Msg("emitted execution.complete semantic event")
}

func (r *Router) finishRun(conv *Conversation, idempotencyKey string, inferenceID string, turnID string, err error) {
	if conv == nil {
		return
	}
	conv.mu.Lock()
	defer conv.mu.Unlock()

	if conv.runningKey == idempotencyKey {
		conv.runningKey = ""
	}
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
		conv.mu.Lock()
		if conv.isBusyLocked() {
			conv.mu.Unlock()
			return
		}
		q, ok := conv.dequeueLocked()
		if !ok {
			conv.mu.Unlock()
			return
		}
		conv.runningKey = q.IdempotencyKey
		conv.ensureQueueInitLocked()
		if rec, ok := conv.getRecordLocked(q.IdempotencyKey); ok && rec != nil {
			rec.Status = "running"
			rec.StartedAt = time.Now()
		} else {
			conv.upsertRecordLocked(&chatRequestRecord{IdempotencyKey: q.IdempotencyKey, Status: "running", StartedAt: time.Now()})
		}
		conv.mu.Unlock()

		_, err := r.startRunForPrompt(conv, q.ProfileSlug, q.Overrides, q.Prompt, q.IdempotencyKey)
		if err != nil {
			r.finishRun(conv, q.IdempotencyKey, "", "", err)
			// Continue draining so later queued items can still run.
			continue
		}
		// Successfully started one run; subsequent items are handled when it finishes.
		return
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
