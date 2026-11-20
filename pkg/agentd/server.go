package agentd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/modelsource"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools"
	toolv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/frontendtools/v1"
	"github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

// ModelSource lists and resolves models (modelsource.Set implements it).
type ModelSource interface {
	Models(ctx context.Context) ([]modelsource.Model, error)
	Resolve(ctx context.Context, ref string, o modelsource.Options) (*modelsource.Resolved, error)
}

// Options configure a Server.
type Options struct {
	Agents map[string]*AgentConfig
	Models ModelSource
	// Store persists sessions; nil keeps them in memory only.
	Store *Store
	// Engine, when set, replaces model resolution (tests and --engine fake).
	Engine func() (engine.Engine, error)
	// EngineModel describes Engine for clients.
	EngineModel modelsource.Model
	// Heartbeat is the SSE comment interval (default 15 s).
	Heartbeat time.Duration
}

// Server is the agent service.
type Server struct {
	opts   Options
	tools  *frontendtools.Manager
	store  *Store
	mu     sync.Mutex
	byID   map[string]*Session
	hbeat  time.Duration
	models ModelSource
}

// NewServer creates a server and loads saved sessions.
func NewServer(ctx context.Context, o Options) (*Server, error) {
	if len(o.Agents) == 0 {
		return nil, errors.New("agentd: no agents configured")
	}
	if o.Models == nil && o.Engine == nil {
		return nil, errors.New("agentd: no model source")
	}
	s := &Server{opts: o, tools: frontendtools.NewManager(), store: o.Store, byID: map[string]*Session{}, hbeat: o.Heartbeat, models: o.Models}
	if s.hbeat <= 0 {
		s.hbeat = 15 * time.Second
	}
	if o.Store != nil {
		recs, err := o.Store.loadAll()
		if err != nil {
			return nil, err
		}
		for _, rec := range recs {
			sess, err := s.restore(ctx, rec)
			if err != nil {
				log.Warn().Err(err).Str("session", rec.ID).Msg("agentd: skip saved session")
				continue
			}
			s.byID[sess.ID] = sess
		}
	}
	return s, nil
}

func (s *Server) engineFor(ctx context.Context, ref, thinking string, maxTokens int) (modelsource.Model, func() (engine.Engine, error), error) {
	if s.opts.Engine != nil {
		return s.opts.EngineModel, s.opts.Engine, nil
	}
	r, err := s.models.Resolve(ctx, ref, modelsource.Options{Thinking: thinking, MaxTokens: maxTokens})
	if err != nil {
		return modelsource.Model{}, nil, err
	}
	return r.Model, r.Engine, nil
}

func (s *Server) newSession(id string, a *AgentConfig, m modelsource.Model, eng func() (engine.Engine, error), system, thinking string) *Session {
	now := time.Now().UTC()
	return &Session{ID: id, Agent: a, Model: m, Created: now, updated: now, srv: s, newEng: eng,
		gs: gepsession.NewSessionWithID(id), log: NewEventLog(id, 5000), system: system, thinking: thinking}
}

func (s *Server) restore(ctx context.Context, rec storedSession) (*Session, error) {
	a := s.opts.Agents[rec.Agent]
	if a == nil {
		return nil, fmt.Errorf("agent %q is no longer configured", rec.Agent)
	}
	m, eng, err := s.engineFor(ctx, rec.Model.Ref, rec.Thinking, a.MaxTokens)
	if err != nil {
		return nil, err
	}
	sess := s.newSession(rec.ID, a, m, eng, rec.System, rec.Thinking)
	sess.Created, sess.updated, sess.title, sess.usage, sess.contextIDs = rec.Created, rec.Updated, rec.Title, rec.Usage, rec.ContextIDs
	if rec.TurnYAML != "" {
		t, err := serde.FromYAML([]byte(rec.TurnYAML))
		if err != nil {
			return nil, err
		}
		sess.gs.Append(t)
	}
	sess.log.restore(rec.Events)
	return sess, nil
}

func (s *Server) session(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byID[id]
}

// Handler returns the HTTP API, rooted at /v1.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "agents": len(s.opts.Agents), "sessions": len(s.list())})
	})
	mux.HandleFunc("GET /v1/agents", s.handleAgents)
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("GET /v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		out := []SessionView{}
		for _, ss := range s.list() {
			if a := r.URL.Query().Get("agent"); a == "" || a == ss.Agent.Name {
				out = append(out, ss.View(false))
			}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("POST /v1/sessions", s.handleCreate)
	mux.HandleFunc("GET /v1/sessions/{id}", s.withSession(func(w http.ResponseWriter, r *http.Request, ss *Session) {
		writeJSON(w, 200, ss.View(true))
	}))
	mux.HandleFunc("DELETE /v1/sessions/{id}", s.withSession(func(w http.ResponseWriter, r *http.Request, ss *Session) {
		ss.Cancel()
		s.mu.Lock()
		delete(s.byID, ss.ID)
		s.mu.Unlock()
		if s.store != nil {
			_ = s.store.Delete(ss.ID)
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /v1/sessions/{id}/tools/manifest", s.withSession(s.handleManifest))
	mux.HandleFunc("POST /v1/sessions/{id}/turns", s.withSession(s.handleTurn))
	mux.HandleFunc("GET /v1/sessions/{id}/events", s.withSession(s.handleEvents))
	mux.HandleFunc("POST /v1/sessions/{id}/tools/{callId}/result", s.withSession(s.handleResult))
	mux.HandleFunc("POST /v1/sessions/{id}/tools/{callId}/progress", s.withSession(func(w http.ResponseWriter, r *http.Request, ss *Session) {
		var in struct {
			State string `json:"state"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		if err := ss.SetCallState(r.PathValue("callId"), in.State); err != nil {
			fail(w, 409, "invalid_state", err.Error())
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /v1/sessions/{id}/cancel", s.withSession(func(w http.ResponseWriter, r *http.Request, ss *Session) {
		writeJSON(w, 202, map[string]any{"cancelled": ss.Cancel()})
	}))
	return mux
}

func (s *Server) list() []*Session {
	s.mu.Lock()
	out := make([]*Session, 0, len(s.byID))
	for _, ss := range s.byID {
		out = append(out, ss)
	}
	s.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func (s *Server) withSession(h func(http.ResponseWriter, *http.Request, *Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ss := s.session(r.PathValue("id"))
		if ss == nil {
			fail(w, 404, "not_found", "no session "+r.PathValue("id"))
			return
		}
		h(w, r, ss)
	}
}

func (s *Server) handleAgents(w http.ResponseWriter, _ *http.Request) {
	out := []map[string]any{}
	for _, a := range s.opts.Agents {
		out = append(out, map[string]any{"name": a.Name, "description": a.Description, "model": a.Model, "pageInstructions": a.PageInstructions,
			"limits": Limits{MaxRounds: a.MaxRounds, ToolTimeoutMs: a.ToolTimeoutDuration().Milliseconds(), ResultCut: a.ResultCut, TokenBudget: a.Budget.TokensPerSession}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
	writeJSON(w, 200, out)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if s.opts.Engine != nil {
		writeJSON(w, 200, map[string]any{"models": []modelsource.Model{s.opts.EngineModel}})
		return
	}
	ms, err := s.models.Models(r.Context())
	resp := map[string]any{"models": ms}
	if err != nil {
		resp["warning"] = err.Error()
	}
	writeJSON(w, 200, resp)
}

type createReq struct {
	Agent        string `json:"agent"`
	Model        string `json:"model,omitempty"`
	Thinking     string `json:"thinking,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in createReq
	if !readJSON(w, r, &in, 1<<20) {
		return
	}
	a := s.opts.Agents[in.Agent]
	if a == nil {
		fail(w, 400, "invalid_request", "unknown agent "+strconv.Quote(in.Agent))
		return
	}
	system := a.SystemText()
	if in.Instructions != "" {
		if !a.PageInstructions {
			fail(w, 400, "invalid_request", "agent "+a.Name+" does not accept page instructions")
			return
		}
		if len(in.Instructions) > a.MaxPageInstructionsBytes {
			fail(w, 400, "prompt_too_large", fmt.Sprintf("page instructions are at most %d bytes", a.MaxPageInstructionsBytes))
			return
		}
		system = strings.TrimRight(system, "\n") + "\n\n" + in.Instructions
	}
	ref, thinking := in.Model, in.Thinking
	if ref == "" {
		ref = a.Model
	}
	if thinking == "" {
		thinking = a.Thinking
	}
	m, eng, err := s.engineFor(r.Context(), ref, thinking, a.MaxTokens)
	if err != nil {
		if errors.Is(err, modelsource.ErrUnknownModel) {
			fail(w, 400, "invalid_request", err.Error())
			return
		}
		log.Warn().Err(err).Str("model", ref).Msg("agentd: resolve model")
		fail(w, 503, "authentication_error", "the model "+ref+" cannot be used with the server's credentials")
		return
	}
	ss := s.newSession(uuid.NewString(), a, m, eng, system, thinking)
	s.mu.Lock()
	s.byID[ss.ID] = ss
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.Save(ss)
	}
	writeJSON(w, 201, ss.View(false))
}

type manifestReq struct {
	Revision         uint64 `json:"revision"`
	ClientInstanceID string `json:"clientInstanceId"`
	ConnectionID     string `json:"connectionId"`
	Tools            []struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		InputSchema map[string]any `json:"inputSchema,omitempty"`
		Mode        string         `json:"mode,omitempty"`
	} `json:"tools"`
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request, ss *Session) {
	var in manifestReq
	if !readJSON(w, r, &in, 1<<20) {
		return
	}
	cmd := &toolv1.FrontendToolManifestCommand{Revision: in.Revision, ClientInstanceId: in.ClientInstanceID, ConnectionId: in.ConnectionID}
	for _, t := range in.Tools {
		var schema *structpb.Struct
		if t.InputSchema != nil {
			st, err := structpb.NewStruct(t.InputSchema)
			if err != nil {
				fail(w, 400, "invalid_request", "tool "+t.Name+": inputSchema: "+err.Error())
				return
			}
			schema = st
		}
		mode := toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_AUTO
		if strings.EqualFold(t.Mode, "human") || ss.Agent.ToolMode(t.Name) == "human" {
			mode = toolv1.ToolExecutionMode_TOOL_EXECUTION_MODE_FRONTEND_HUMAN
		}
		cmd.Tools = append(cmd.Tools, &toolv1.FrontendToolDescriptor{Name: t.Name, Description: t.Description, InputSchema: schema, Mode: mode, Available: true})
	}
	upd, err := s.tools.AcceptManifest(r.Context(), sessionstream.SessionId(ss.ID), toolPublisher{ss}, cmd)
	if err != nil {
		code := "invalid_request"
		if c, ok := frontendtools.ManifestErrorCodeOf(err); ok {
			code = string(c)
		}
		fail(w, 409, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"revision": upd.GetRevision(), "executor": executorJSON(upd.GetExecutor())})
}

func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request, ss *Session) {
	var in struct {
		Text    string `json:"text"`
		Context string `json:"context,omitempty"`
	}
	if !readJSON(w, r, &in, 4<<20) {
		return
	}
	run, err := ss.StartTurn(in.Text, in.Context)
	switch {
	case errors.Is(err, ErrRunActive):
		fail(w, 409, "run_active", err.Error())
	case errors.Is(err, ErrQuota):
		fail(w, 429, "quota_exceeded", err.Error())
	case err != nil:
		fail(w, 400, "invalid_request", err.Error())
	default:
		writeJSON(w, 202, map[string]any{"run": run, "seq": ss.log.Seq()})
	}
}

type resultReq struct {
	ToolName string         `json:"toolName,omitempty"`
	Status   string         `json:"status"`
	Value    any            `json:"value"`
	Error    string         `json:"error,omitempty"`
	Executor map[string]any `json:"executor"`
}

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request, ss *Session) {
	var in resultReq
	if !readJSON(w, r, &in, 1<<20) {
		return
	}
	value := map[string]any{"value": in.Value}
	if m, ok := in.Value.(map[string]any); ok {
		value = m
	}
	st, err := structpb.NewStruct(value)
	if err != nil {
		fail(w, 400, "invalid_request", "value: "+err.Error())
		return
	}
	str := func(k string) string { v, _ := in.Executor[k].(string); return v }
	cmd := &toolv1.FrontendToolResultCommand{ToolCallId: r.PathValue("callId"), ToolName: in.ToolName, Result: st, Status: in.Status, Error: in.Error,
		Executor: &toolv1.FrontendToolExecutor{ClientInstanceId: str("clientInstanceId"), ConnectionId: str("connectionId"), AssignmentId: str("assignmentId")}}
	err = s.tools.HandleResult(r.Context(), sessionstream.Command{Name: "ChatFrontendToolResult", SessionId: sessionstream.SessionId(ss.ID), Payload: cmd}, nil, toolPublisher{ss})
	if err != nil {
		code, status := "invalid_request", 400
		if c, ok := frontendtools.InvocationErrorCodeOf(err); ok {
			code, status = string(c), 409
		}
		fail(w, status, code, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, ss *Session) {
	fl, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "internal", "streaming unsupported")
		return
	}
	after := int64(0)
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		after, _ = strconv.ParseInt(v, 10, 64)
	} else if v := r.URL.Query().Get("after"); v != "" {
		after, _ = strconv.ParseInt(v, 10, 64)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	backlog, ch, cancel := ss.log.Subscribe(after)
	defer cancel()
	write := func(ev Event) bool {
		b, err := json.Marshal(ev)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, b); err != nil {
			return false
		}
		return true
	}
	for _, ev := range backlog {
		if !write(ev) {
			return
		}
	}
	fl.Flush()
	t := time.NewTicker(s.hbeat)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return // fell behind; the client reconnects with Last-Event-ID
			}
			if !write(ev) {
				return
			}
			fl.Flush()
		case <-t.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	if err := dec.Decode(v); err != nil {
		fail(w, 400, "invalid_request", "body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
