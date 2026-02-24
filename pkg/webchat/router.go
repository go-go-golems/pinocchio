package webchat

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
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

// NewRouter creates webchat core plus optional HTTP utility handlers (UI + core API).
// It does not register app-owned transport routes such as /chat or /ws.
func NewRouter(ctx context.Context, parsed *values.Values, staticFS fs.FS, opts ...RouterOption) (*Router, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	streamBackend, err := NewStreamBackendFromValues(ctx, parsed)
	if err != nil {
		return nil, err
	}
	r := &Router{
		baseCtx:       ctx,
		parsed:        parsed,
		mux:           http.NewServeMux(),
		staticFS:      staticFS,
		router:        streamBackend.EventRouter(),
		streamBackend: streamBackend,
		mwFactories:   map[string]MiddlewareBuilder{},
		toolFactories: map[string]infruntime.ToolRegistrar{},
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
	r.timelineService = NewTimelineService(r.timelineStore)

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
	if r.runtimeComposer == nil {
		return nil, errors.New("runtime composer is not configured")
	}
	if r.timelineService == nil {
		r.timelineService = NewTimelineService(r.timelineStore)
	}

	if r.stepCtrl == nil {
		r.stepCtrl = toolloop.NewStepController()
	}
	if r.cm == nil {
		r.cm = NewConvManager(ConvManagerOptions{
			BaseCtx:            ctx,
			StepController:     r.stepCtrl,
			RuntimeComposer:    r.convRuntimeComposer(),
			BuildSubscriber:    r.BuildSubscriber,
			TimelineUpsertHook: r.TimelineUpsertHook,
		})
	} else {
		r.cm.SetRuntimeComposer(r.convRuntimeComposer())
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

	svc, err := NewConversationService(ConversationServiceConfig{
		BaseCtx:            ctx,
		ConvManager:        r.cm,
		StepController:     r.stepCtrl,
		TimelineStore:      r.timelineStore,
		TurnStore:          r.turnStore,
		SEMPublisher:       r.streamBackend.Publisher(),
		TimelineUpsertHook: r.timelineUpsertHookOverride,
		ToolFactories:      r.toolFactories,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new conversation service")
	}
	r.chatService = NewChatServiceFromConversation(svc)
	r.streamHub = svc.StreamHub()

	r.registerHTTPHandlers()
	return r, nil
}

// RegisterMiddleware adds a named middleware factory to the router.
func (r *Router) RegisterMiddleware(name string, f MiddlewareBuilder) {
	r.mwFactories[name] = f
}

// RegisterTool adds a named tool factory to the router.
func (r *Router) RegisterTool(name string, f infruntime.ToolRegistrar) {
	r.toolFactories[name] = f
	if r.chatService != nil {
		r.chatService.RegisterTool(name, f)
	}
}

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

// Handle attaches an extra handler to the router utility mux.
// This is optional convenience for app composition, not a central route-ownership mechanism.
func (r *Router) Handle(pattern string, h http.Handler) { r.mux.Handle(pattern, h) }

// HandleFunc attaches an extra handler to the router utility mux.
// This is optional convenience for app composition, not a central route-ownership mechanism.
func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.mux.HandleFunc(pattern, handler)
}

// Handler returns the router utility mux (UI + core API + any explicitly attached extras).
// Applications should still own and mount /chat and /ws themselves.
func (r *Router) Handler() http.Handler { return r.mux }

// ChatService returns the chat-focused service surface (queue/idempotency/inference).
func (r *Router) ChatService() *ChatService { return r.chatService }

// StreamHub returns the stream lifecycle service used by websocket helpers.
func (r *Router) StreamHub() *StreamHub {
	if r == nil {
		return nil
	}
	return r.streamHub
}

// TimelineService returns the timeline hydration service.
func (r *Router) TimelineService() *TimelineService {
	if r == nil {
		return nil
	}
	return r.timelineService
}

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

// registerHTTPHandlers sets up UI and core API utility handlers.
func (r *Router) registerHTTPHandlers() {
	r.registerUIHandlers(r.mux)
	r.registerAPIHandlers(r.mux)
}

// APIHandler returns an http.Handler that only exposes core API utilities (timeline/debug).
// It intentionally does not expose app-owned /chat or /ws routes.
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
	// Timeline hydration is part of core webchat and available independent of debug routes.
	r.registerTimelineAPIHandlers(mux)

	if r.debugRoutesEnabled() {
		r.registerDebugAPIHandlers(mux)
	}
}

// helpers
func fsSub(staticFS fs.FS, path string) (fs.FS, error) { return fs.Sub(staticFS, path) }

// runtime wiring bits
var (
	_ http.Handler
)

func (r *Router) convRuntimeComposer() infruntime.RuntimeBuilder {
	return infruntime.RuntimeBuilderFunc(func(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		if r == nil {
			return infruntime.ComposedRuntime{}, errors.New("router is nil")
		}
		if r.runtimeComposer == nil {
			return infruntime.ComposedRuntime{}, errors.New("runtime composer is not configured")
		}
		artifacts, err := r.runtimeComposer.Compose(ctx, req)
		if err != nil {
			return infruntime.ComposedRuntime{}, err
		}
		if artifacts.Engine == nil {
			return infruntime.ComposedRuntime{}, errors.New("runtime composer returned nil engine")
		}
		if artifacts.Sink == nil {
			artifacts.Sink = middleware.NewWatermillSink(r.router.Publisher, topicForConv(req.ConvID))
		}
		if r.eventSinkWrapper != nil {
			wrapped, err := r.eventSinkWrapper(req.ConvID, req, artifacts.Sink)
			if err != nil {
				return infruntime.ComposedRuntime{}, err
			}
			artifacts.Sink = wrapped
		}
		if strings.TrimSpace(artifacts.RuntimeKey) == "" {
			artifacts.RuntimeKey = strings.TrimSpace(req.ProfileKey)
		}
		if strings.TrimSpace(artifacts.RuntimeFingerprint) == "" {
			artifacts.RuntimeFingerprint = artifacts.RuntimeKey
		}
		return artifacts, nil
	})
}

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
	if r.streamBackend == nil {
		return nil, false, errors.New("stream backend is nil")
	}
	return r.streamBackend.BuildSubscriber(r.baseCtx, convID)
}

// private state fields appended to Router
// (declared here for proximity to logic, defined in types.go)
