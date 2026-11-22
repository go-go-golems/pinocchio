// Package httpapi serves chatapp sessions over HTTP.
//
// A Server builds the whole chatapp stack (schema registry, hydration store,
// engine, sessionstream hub, service) from its options and mounts these routes
// under a prefix (default "/api/chat"):
//
//	POST   /sessions                      create a session → 201 SessionResponse
//	GET    /sessions?label=k:v&limit=n    list sessions → SessionListResponse
//	GET    /sessions/{id}                 metadata and timeline snapshot → SessionResponse
//	DELETE /sessions/{id}                 stop the run, delete the metadata → 204
//	POST   /sessions/{id}/messages        start a run → 202; 409 run_active
//	POST   /sessions/{id}/stop            stop the running turn → 202
//	GET    /sessions/{id}/events          server-sent events: snapshot, then live UI events
//	POST   /sessions/{id}/tools/manifest  register the page's browser tools
//	POST   /sessions/{id}/tools/results   post a browser tool's result
//	GET    /sessions/{id}/timeline|turns|export   exports
//	GET    /ws                            websocket (sessionstream protocol)
//
// What a run does (model, system prompt, tools, limits) is decided by the
// application's RuntimeResolver for every message. The server never writes
// content into the context window itself; WithBlocks lets the application add
// its own blocks after the person's message.
package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/turns"
	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatexport "github.com/go-go-golems/pinocchio/pkg/chatapp/export"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
)

// DefaultPrefix is where the routes are mounted unless WithPrefix says otherwise.
const DefaultPrefix = "/api/chat"

// RuntimeResolver decides the runtime (engine, system prompt, tools, loop
// limits, round guard) of one message. It is called for every message, so the
// runtime may depend on the session, the request or the message itself.
// Returning an error that implements chatapp.CodedError answers 400 with that
// code; any other error answers 400 with code "runtime_error".
type RuntimeResolver interface {
	Resolve(ctx context.Context, r *http.Request, s sessions.Info, msg MessageRequest) (*infruntime.ComposedRuntime, error)
}

// RuntimeResolverFunc adapts a function to RuntimeResolver.
type RuntimeResolverFunc func(ctx context.Context, r *http.Request, s sessions.Info, msg MessageRequest) (*infruntime.ComposedRuntime, error)

func (f RuntimeResolverFunc) Resolve(ctx context.Context, r *http.Request, s sessions.Info, msg MessageRequest) (*infruntime.ComposedRuntime, error) {
	return f(ctx, r, s, msg)
}

// BlocksFunc returns blocks to append after the person's message in the turn
// (chatapp.PromptRequest.Blocks), e.g. the current state of the page. The
// blocks stay in the conversation; they are never removed later.
type BlocksFunc func(ctx context.Context, s sessions.Info, msg MessageRequest) ([]turns.Block, error)

// HydrationStoreFactory constructs the timeline store after the server has
// registered all protobuf schemas.
type HydrationStoreFactory func(context.Context, *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error)

// Server is the HTTP API over one chatapp engine.
type Server struct {
	prefix           string
	resolver         RuntimeResolver
	sessionStore     sessions.Store
	autoCreate       bool
	hydrationSpec    serverkit.StoreSpec
	hydrationFactory HydrationStoreFactory
	turnStore        chatstore.TurnStore
	turnsDBPath      string
	plugins          []chatapp.ChatPlugin
	frontendTools    *frontendtools.Manager
	engineOptions    []chatapp.Option
	authenticate     Authenticator
	authorize        Authorizer
	blocks           BlocksFunc
	sseConfig        SSEConfig
	maxBodyBytes     int64

	service       *chatapp.Service
	hub           *sessionstream.Hub
	ws            *wstransport.Server
	sse           *sseHub
	exportService *chatexport.Service
	closeFn       func() error
}

// Option configures a Server.
type Option func(*Server)

// WithPrefix sets the path prefix of every route (default DefaultPrefix).
func WithPrefix(prefix string) Option {
	return func(s *Server) { s.prefix = prefix }
}

// WithRuntimeResolver sets the resolver that decides each message's runtime. Required.
func WithRuntimeResolver(resolver RuntimeResolver) Option {
	return func(s *Server) { s.resolver = resolver }
}

// WithSessionStore sets where session metadata is kept (default: in memory).
func WithSessionStore(store sessions.Store) Option {
	return func(s *Server) { s.sessionStore = store }
}

// WithAutoCreateSessions makes a request for an unknown session id create the
// session (owned by the requesting principal) instead of answering 404. Delete
// and stop never create. Clients that keep a session id across server restarts
// need this when the session store is in memory.
func WithAutoCreateSessions(enabled bool) Option {
	return func(s *Server) { s.autoCreate = enabled }
}

// WithHydrationStoreSpec selects the timeline store (default: in memory).
func WithHydrationStoreSpec(spec serverkit.StoreSpec) Option {
	return func(s *Server) { s.hydrationSpec = spec }
}

// WithHydrationStoreFactory overrides WithHydrationStoreSpec with a constructor.
func WithHydrationStoreFactory(factory HydrationStoreFactory) Option {
	return func(s *Server) { s.hydrationFactory = factory }
}

// WithTurnStore sets where conversations are kept (chatapp.WithTurnStore).
func WithTurnStore(store chatstore.TurnStore) Option {
	return func(s *Server) { s.turnStore = store }
}

// WithTurnsDBPath names the SQLite turn database for minitrace exports.
func WithTurnsDBPath(path string) Option {
	return func(s *Server) { s.turnsDBPath = strings.TrimSpace(path) }
}

// WithChatPlugins adds chatapp plugins (projections and schemas).
func WithChatPlugins(plugins ...chatapp.ChatPlugin) Option {
	return func(s *Server) {
		for _, p := range plugins {
			if p != nil {
				s.plugins = append(s.plugins, p)
			}
		}
	}
}

// WithFrontendTools enables browser tools: the manifest and result routes,
// and every run gets the tools its session's page registered
// (frontendtools.Manager.RunTools). Add frontendtools.NewPlugin() to project
// browser tool calls into the timeline.
func WithFrontendTools(manager *frontendtools.Manager) Option {
	return func(s *Server) { s.frontendTools = manager }
}

// WithEngineOptions passes options to chatapp.NewEngine (busy policy, error
// classifier and text, ...). They are applied after the server's own options
// (plugins, turn store, usage recording), so they can override them.
func WithEngineOptions(opts ...chatapp.Option) Option {
	return func(s *Server) { s.engineOptions = append(s.engineOptions, opts...) }
}

// WithAuthenticator sets how a request becomes a Principal (default: anonymous).
func WithAuthenticator(fn Authenticator) Option {
	return func(s *Server) { s.authenticate = fn }
}

// WithAuthorizer sets the per-action check (default: OwnerOnly).
func WithAuthorizer(fn Authorizer) Option {
	return func(s *Server) { s.authorize = fn }
}

// WithBlocks sets the hook that adds the application's blocks to each message.
func WithBlocks(fn BlocksFunc) Option {
	return func(s *Server) { s.blocks = fn }
}

// WithSSEConfig tunes the server-sent events transport.
func WithSSEConfig(cfg SSEConfig) Option {
	return func(s *Server) { s.sseConfig = cfg }
}

// WithMaxBodyBytes limits JSON request bodies (default 1 MiB).
func WithMaxBodyBytes(n int64) Option {
	return func(s *Server) { s.maxBodyBytes = n }
}

// New builds the server and the chatapp stack behind it.
func New(opts ...Option) (*Server, error) {
	s := &Server{
		prefix:        DefaultPrefix,
		hydrationSpec: serverkit.StoreSpec{Backend: serverkit.StoreBackendMemory},
		sseConfig:     DefaultSSEConfig(),
		maxBodyBytes:  1 << 20,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.resolver == nil {
		return nil, errors.New("httpapi: a runtime resolver is required (WithRuntimeResolver)")
	}
	s.prefix = "/" + strings.Trim(strings.TrimSpace(s.prefix), "/")
	if s.prefix == "/" {
		s.prefix = ""
	}
	if s.sessionStore == nil {
		s.sessionStore = sessions.NewMemoryStore()
	}
	if s.authorize == nil {
		s.authorize = OwnerOnly
	}
	s.sseConfig = s.sseConfig.withDefaults()

	reg := sessionstream.NewSchemaRegistry()
	if err := chatapp.RegisterSchemas(reg, s.plugins...); err != nil {
		return nil, err
	}
	store, cleanup, err := s.openHydrationStore(reg)
	if err != nil {
		return nil, err
	}
	closeStore := func() error {
		if cleanup != nil {
			return cleanup()
		}
		return nil
	}
	ws, err := wstransport.NewServer(&hydrationSnapshotProvider{server: s}, wstransport.WithSubscribeAuthorizer(s.authorizeSubscribe))
	if err != nil {
		_ = closeStore()
		return nil, err
	}
	s.ws = ws
	s.sse = newSSEHub(s.sseConfig)

	engineOptions := []chatapp.Option{
		chatapp.WithPlugins(s.plugins...),
		chatapp.WithTurnStore(s.turnStore),
		chatapp.WithUsageObserver(sessions.RecordUsage(s.sessionStore)),
	}
	if s.frontendTools != nil {
		engineOptions = append(engineOptions, chatapp.WithRunTools(s.frontendTools.RunTools()))
	}
	engine := chatapp.NewEngine(append(engineOptions, s.engineOptions...)...)
	hub, err := sessionstream.NewHub(
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
		sessionstream.WithUIFanout(transports{ws, s.sse}),
	)
	if err != nil {
		_ = closeStore()
		return nil, err
	}
	if err := chatapp.Install(hub, engine); err != nil {
		_ = closeStore()
		return nil, err
	}
	if s.frontendTools != nil {
		if err := s.frontendTools.Install(hub); err != nil {
			_ = closeStore()
			return nil, err
		}
	}
	service, err := chatapp.NewService(hub, engine)
	if err != nil {
		_ = closeStore()
		return nil, err
	}
	s.hub = hub
	s.service = service
	s.exportService = chatexport.NewService(service, chatexport.WithTurnStore(s.turnStore), chatexport.WithTurnsDBPath(s.turnsDBPath))
	s.closeFn = func() error {
		s.sse.closeAll()
		return closeStore()
	}
	return s, nil
}

func (s *Server) openHydrationStore(reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if s.hydrationFactory != nil {
		return s.hydrationFactory(context.Background(), reg)
	}
	return serverkit.OpenHydrationStore(context.Background(), s.hydrationSpec, reg)
}

// Register adds the server's routes to mux.
func (s *Server) Register(mux *http.ServeMux) {
	p := s.prefix
	mux.HandleFunc("POST "+p+"/sessions", s.handleCreate)
	mux.HandleFunc("GET "+p+"/sessions", s.handleList)
	mux.HandleFunc("GET "+p+"/sessions/{id}", s.session(ActionRead, s.handleGet))
	mux.HandleFunc("DELETE "+p+"/sessions/{id}", s.session(ActionDelete, s.handleDelete))
	mux.HandleFunc("POST "+p+"/sessions/{id}/messages", s.session(ActionSend, s.handleMessage))
	mux.HandleFunc("POST "+p+"/sessions/{id}/stop", s.session(ActionStop, s.handleStop))
	mux.HandleFunc("GET "+p+"/sessions/{id}/events", s.session(ActionRead, s.handleEvents))
	mux.HandleFunc("POST "+p+"/sessions/{id}/tools/manifest", s.session(ActionTools, s.handleFrontendToolManifest))
	mux.HandleFunc("POST "+p+"/sessions/{id}/tools/results", s.session(ActionTools, s.handleFrontendToolResult))
	mux.HandleFunc("GET "+p+"/sessions/{id}/timeline", s.session(ActionRead, s.handleTimelineExport))
	mux.HandleFunc("GET "+p+"/sessions/{id}/turns", s.session(ActionRead, s.handleTurnsExport))
	mux.HandleFunc("GET "+p+"/sessions/{id}/export", s.session(ActionRead, s.handleFullExport))
	mux.HandleFunc("GET "+p+"/ws", s.handleWS)
}

// Handler returns a handler serving only the server's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.Register(mux)
	return mux
}

// Service is the chatapp service behind the server, for applications that
// submit prompts or read snapshots without HTTP.
func (s *Server) Service() *chatapp.Service { return s.service }

// Sessions is the session metadata store.
func (s *Server) Sessions() sessions.Store { return s.sessionStore }

// Hub is the sessionstream hub.
func (s *Server) Hub() *sessionstream.Hub { return s.hub }

// Close ends the event streams and closes the hydration store.
func (s *Server) Close() error {
	if s == nil || s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	p, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	s.ws.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
}

// authorizeSubscribe runs when a websocket client subscribes to a session; the
// context derives from the upgrade request and carries the principal.
func (s *Server) authorizeSubscribe(ctx context.Context, sid sessionstream.SessionId) error {
	p, _ := PrincipalFromContext(ctx)
	if !validSessionID(string(sid)) {
		return errInvalidSessionID
	}
	_, err := s.lookup(ctx, p, string(sid), ActionRead)
	return err
}

// transports delivers UI events to every transport. A delivery failure is a
// matter of one connection (the transport closes it and the client hydrates
// again); the event is already stored, so it must not fail the publish that
// produced it.
type transports []sessionstream.UIFanout

func (t transports) PublishUI(ctx context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	for _, tr := range t {
		if err := tr.PublishUI(ctx, sid, ord, events); err != nil {
			log.Warn().Err(err).Str("session", string(sid)).Uint64("ordinal", ord).Msg("httpapi: deliver ui events")
		}
	}
	return nil
}

var _ sessionstream.UIFanout = transports{}

// shutdownTimeout bounds how long DELETE waits for a stopped run to end.
const shutdownTimeout = 10 * time.Second
