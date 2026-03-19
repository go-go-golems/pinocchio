package webchat

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
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
// Deprecated: use BuildRouterDepsFromValues plus NewRouterFromDeps, or call NewRouterFromDeps directly with explicit dependencies.
func NewRouter(ctx context.Context, parsed ParsedRouterInputs, staticFS fs.FS, opts ...RouterOption) (*Router, error) {
	deps, err := BuildRouterDepsFromValues(ctx, parsed, staticFS)
	if err != nil {
		return nil, err
	}
	return NewRouterFromDeps(ctx, deps, opts...)
}

// NewRouterFromDeps creates webchat core from already resolved infrastructure inputs.
// It does not register app-owned transport routes such as /chat or /ws.
func NewRouterFromDeps(ctx context.Context, deps RouterDeps, opts ...RouterOption) (*Router, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	if deps.StreamBackend == nil {
		return nil, errors.New("stream backend is nil")
	}
	eventRouter := deps.StreamBackend.EventRouter()
	if eventRouter == nil {
		return nil, errors.New("stream backend event router is nil")
	}
	r := &Router{
		baseCtx:       ctx,
		mux:           http.NewServeMux(),
		staticFS:      deps.StaticFS,
		settings:      deps.Settings,
		router:        eventRouter,
		streamBackend: deps.StreamBackend,
		timelineStore: deps.TimelineStore,
		turnStore:     deps.TurnStore,
		toolFactories: map[string]infruntime.ToolRegistrar{},
	}
	if r.timelineStore == nil {
		r.timelineStore = NewDefaultTimelineStore(deps.Settings)
	}
	if r.cm != nil {
		r.cm.SetTimelineStore(r.timelineStore)
	}
	r.timelineService = NewTimelineService(r.timelineStore)

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
		r.cm.SetIdleTimeoutSeconds(r.settings.IdleTimeoutSeconds)
		r.cm.SetEvictionConfig(
			time.Duration(r.settings.EvictIdleSeconds)*time.Second,
			time.Duration(r.settings.EvictIntervalSeconds)*time.Second,
		)
	}
	r.idleTimeoutSec = r.settings.IdleTimeoutSeconds

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

// RegisterTool adds a named tool factory to the router.
func (r *Router) RegisterTool(name string, f infruntime.ToolRegistrar) {
	r.toolFactories[name] = f
	if r.chatService != nil {
		r.chatService.RegisterTool(name, f)
	}
}

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
	s := r.settings
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
