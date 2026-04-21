package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream"
	storememory "github.com/go-go-golems/sessionstream/hydration/memory"
	storesqlite "github.com/go-go-golems/sessionstream/hydration/sqlite"
	wstransport "github.com/go-go-golems/sessionstream/transport/ws"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Option func(*Server)

type Server struct {
	service         *chatapp.Service
	ws              *wstransport.Server
	defaultProfile  string
	chunkDelay      time.Duration
	sqliteDSN       string
	sqliteDBPath    string
	runtimeResolver RuntimeResolver
	chatFeatures    []chatapp.FeatureSet
	closeFn         func() error
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

func WithChatFeatureSets(features ...chatapp.FeatureSet) Option {
	return func(s *Server) {
		if s == nil {
			return
		}
		for _, feature := range features {
			if feature != nil {
				s.chatFeatures = append(s.chatFeatures, feature)
			}
		}
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
	if err := chatapp.RegisterSchemas(reg, s.chatFeatures...); err != nil {
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
	engine := chatapp.NewEngine(chatapp.WithChunkDelay(s.chunkDelay), chatapp.WithFeatureSets(s.chatFeatures...))
	hub, err := sessionstream.NewHub(
		sessionstream.WithSchemaRegistry(reg),
		sessionstream.WithHydrationStore(store),
		sessionstream.WithUIFanout(ws),
	)
	if err != nil {
		return nil, err
	}
	if err := chatapp.Install(hub, engine); err != nil {
		return nil, err
	}
	service, err := chatapp.NewService(hub, engine)
	if err != nil {
		return nil, err
	}

	s.service = service
	s.ws = ws
	s.closeFn = cleanup
	return s, nil
}

func newHydrationStore(s *Server, reg *sessionstream.SchemaRegistry) (sessionstream.HydrationStore, func() error, error) {
	if s == nil || reg == nil {
		return nil, nil, fmt.Errorf("app server or schema registry is nil")
	}
	if s.sqliteDSN == "" && s.sqliteDBPath == "" {
		return storememory.New(), nil, nil
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
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && err.Error() != "EOF" {
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
	sessionID, action, ok := parseSessionPath(r.URL.Path)
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
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad request"})
		return
	}
	if strings.TrimSpace(in.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing prompt"})
		return
	}
	var runtime *infruntime.ComposedRuntime
	if s.runtimeResolver != nil {
		resolved, err := s.runtimeResolver.Resolve(r.Context(), r, in.Profile, in.Registry)
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
	resp := SessionSnapshotResponse{
		SessionID: string(snap.SessionId),
		Ordinal:   fmt.Sprintf("%d", snap.Ordinal),
		Entities:  make([]SnapshotEntity, 0, len(snap.Entities)),
	}
	for _, entity := range snap.Entities {
		resp.Entities = append(resp.Entities, SnapshotEntity{
			Kind:      entity.Kind,
			ID:        entity.Id,
			Tombstone: entity.Tombstone,
			Payload:   encodeProtoJSON(entity.Payload),
		})
	}
	resp.Status = snapshotStatus(resp.Entities)
	return resp
}

func snapshotStatus(entities []SnapshotEntity) string {
	for i := len(entities) - 1; i >= 0; i-- {
		payload, ok := entities[i].Payload.(map[string]any)
		if !ok {
			continue
		}
		status, _ := payload["status"].(string)
		if status != "" {
			return status
		}
	}
	return "idle"
}

func encodeProtoJSON(msg proto.Message) any {
	if msg == nil {
		return nil
	}
	body, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(msg)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return string(body)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseSessionPath(path string) (sessionID string, action string, ok bool) {
	const prefix = "/api/chat/sessions/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		return parts[0], "", true
	}
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}
