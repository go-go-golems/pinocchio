package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatexport "github.com/go-go-golems/pinocchio/pkg/chatapp/export"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	storesqlite "github.com/go-go-golems/sessionstream/pkg/sessionstream/hydration/sqlite"
	wstransport "github.com/go-go-golems/sessionstream/pkg/sessionstream/transport/ws"
)

type Option func(*Server)

type Server struct {
	service             *chatapp.Service
	ws                  *wstransport.Server
	defaultProfile      string
	chunkDelay          time.Duration
	sqliteDSN           string
	sqliteDBPath        string
	runtimeResolver     RuntimeResolver
	turnStore           chatstore.TurnStore
	turnsDBPath         string
	exportService       *chatexport.Service
	chatPlugins         []chatapp.ChatPlugin
	frontendToolManager *frontendtools.Manager
	closeFn             func() error
}

func WithDefaultProfile(profile string) Option {
	return func(s *Server) {
		s.defaultProfile = strings.TrimSpace(profile)
	}
}

func WithChunkDelay(delay time.Duration) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.chunkDelay = delay
	}
}

func WithSQLiteDSN(dsn string) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.sqliteDSN = strings.TrimSpace(dsn)
	}
}

func WithSQLiteDBPath(path string) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.sqliteDBPath = strings.TrimSpace(path)
	}
}

func WithRuntimeResolver(resolver RuntimeResolver) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.runtimeResolver = resolver
	}
}

func WithTurnStore(store chatstore.TurnStore) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.turnStore = store
	}
}

func WithTurnsDBPath(path string) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.turnsDBPath = strings.TrimSpace(path)
	}
}

func WithChatPlugins(features ...chatapp.ChatPlugin) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		for _, feature := range features {
			if feature != nil {
				s.chatPlugins = append(s.chatPlugins, feature)
			}
		}
	}
}

func WithFrontendToolManager(manager *frontendtools.Manager) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		s.frontendToolManager = manager
	}
}

func NewServer(opts ...Option) (*Server, error) {
	s := &Server{chunkDelay: 20 * time.Millisecond}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	reg := sessionstream.NewSchemaRegistry()
	if err := chatapp.RegisterSchemas(reg, s.chatPlugins...); err != nil {
		return nil, err
	}
	store, cleanup, err := newHydrationStore(s, reg)
	if err != nil {
		return nil, err
	}
	provider := &hydrationSnapshotProvider{server: s}
	ws, err := wstransport.NewServer(provider)
	if err != nil {
		return nil, err
	}
	engine := chatapp.NewEngine(chatapp.WithChunkDelay(s.chunkDelay), chatapp.WithPlugins(s.chatPlugins...), chatapp.WithTurnStore(s.turnStore))
	hubOptions := []sessionstream.HubOption{
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
		sessionstream.WithUIFanout(ws),
	}
	hub, err := sessionstream.NewHub(hubOptions...)
	if err != nil {
		return nil, err
	}
	if err := chatapp.Install(hub, engine); err != nil {
		return nil, err
	}
	if s.frontendToolManager != nil {
		if err := s.frontendToolManager.Install(hub); err != nil {
			return nil, err
		}
	}
	service, err := chatapp.NewService(hub, engine)
	if err != nil {
		return nil, err
	}

	s.service = service
	s.exportService = chatexport.NewService(service, chatexport.WithTurnStore(s.turnStore), chatexport.WithTurnsDBPath(s.turnsDBPath))
	s.ws = ws
	s.closeFn = cleanup
	return s, nil
}

func newHydrationStore(s *Server, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if s == nil || reg == nil {
		return nil, nil, fmt.Errorf("app server or schema registry is nil")
	}
	if s.sqliteDSN == "" && s.sqliteDBPath == "" {
		store, err := storesqlite.NewInMemory(reg)
		if err != nil {
			return nil, nil, err
		}
		return store, store.Close, nil
	}
	dsn := s.sqliteDSN
	if dsn == "" {
		if dir := filepath.Dir(s.sqliteDBPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, nil, err
			}
		}
		var err error
		dsn, err = storesqlite.FileDSN(s.sqliteDBPath)
		if err != nil {
			return nil, nil, err
		}
	}
	store, err := storesqlite.New(dsn, reg)
	if err != nil {
		return nil, nil, err
	}
	return store, store.Close, nil
}

func (s *Server) Close() error {
	if s == nil || s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}

type hydrationSnapshotProvider struct {
	server *Server
}

func (p *hydrationSnapshotProvider) Snapshot(_ context.Context, sid sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if p == nil || p.server == nil {
		return sessionstream.Snapshot{}, fmt.Errorf("snapshot provider is not initialized")
	}
	return p.server.Snapshot(sid)
}

func (s *Server) Snapshot(sessionID sessionstream.SessionId) (sessionstream.Snapshot, error) {
	if s == nil || s.service == nil {
		return sessionstream.Snapshot{}, fmt.Errorf("server is not initialized")
	}
	return s.service.Snapshot(context.Background(), sessionID)
}

func (s *Server) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in CreateSessionRequest
	if err := serverkit.DecodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	profile := strings.TrimSpace(in.Profile)
	if profile == "" {
		profile = s.defaultProfile
	}
	writeJSON(w, http.StatusOK, CreateSessionResponse{SessionID: uuid.NewString(), Profile: profile})
}

func (s *Server) HandleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	sessionID, action, ok := parseWebChatSessionPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	sid := sessionstream.SessionId(sessionID)
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing session id"})
		return
	}
	if action == "" {
		s.handleSessionSnapshot(w, r, sid)
		return
	}
	if action == "messages" {
		s.handleSubmitMessage(w, r, sid)
		return
	}
	if action == "timeline" {
		s.handleTimelineExport(w, r, sid)
		return
	}
	if action == "turns" {
		s.handleTurnsExport(w, r, sid)
		return
	}
	if action == "export" {
		s.handleFullExport(w, r, sid)
		return
	}
	if action == "tools/manifest" {
		s.handleFrontendToolManifest(w, r, sid)
		return
	}
	if action == "tools/results" {
		s.handleFrontendToolResult(w, r, sid)
		return
	}
	writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.ws == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "websocket transport not initialized"})
		return
	}
	s.ws.ServeHTTP(w, r)
}

func (s *Server) handleSubmitMessage(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var in SubmitMessageRequest
	if err := serverkit.DecodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	if strings.TrimSpace(in.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing prompt"})
		return
	}
	var runtime *infruntime.ComposedRuntime
	if s.runtimeResolver != nil {
		resolved, err := s.runtimeResolver.Resolve(r.Context(), r, string(sid), in.Profile, in.Registry)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		runtime = resolved
	}
	if err := s.service.SubmitPromptRequest(r.Context(), sid, chatapp.PromptRequest{
		Prompt:         in.Prompt,
		IdempotencyKey: in.IdempotencyKey,
		Runtime:        runtime,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	profile := strings.TrimSpace(in.Profile)
	if profile == "" {
		profile = s.defaultProfile
	}
	writeJSON(w, http.StatusOK, SubmitMessageResponse{SessionID: string(sid), Accepted: true, Status: "running", Profile: profile})
}

func (s *Server) handleSessionSnapshot(w http.ResponseWriter, r *http.Request, sid sessionstream.SessionId) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	snap, err := s.service.Snapshot(r.Context(), sid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, encodeSnapshotResponse(snap))
}

func encodeSnapshotResponse(snap sessionstream.Snapshot) SessionSnapshotResponse {
	return serverkit.EncodeSnapshotResponse(snap, snapshotStatus)
}

func snapshotStatus(entities []SnapshotEntity) string {
	hasUser := false
	hasNonUserStatus := false
	fallbackStatus := ""
	for i := len(entities) - 1; i >= 0; i-- {
		payload, ok := entities[i].Payload.(map[string]any)
		if !ok {
			continue
		}
		role, _ := payload["role"].(string)
		status, _ := payload["status"].(string)
		if role == "assistant" && status != "" {
			return status
		}
		if role == "user" {
			hasUser = true
			continue
		}
		if status == "streaming" {
			return status
		}
		if status != "" {
			hasNonUserStatus = true
			if fallbackStatus == "" {
				fallbackStatus = status
			}
		}
	}
	if hasUser && hasNonUserStatus {
		return "streaming"
	}
	if fallbackStatus != "" {
		return fallbackStatus
	}
	if hasUser {
		return "streaming"
	}
	return "idle"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	serverkit.WriteJSON(w, status, payload)
}
