package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/go-go-golems/geppetto/pkg/turns"

	chatapp "github.com/go-go-golems/pinocchio/pkg/chatapp"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/httpapi/internal/sessionguard"
	"github.com/go-go-golems/pinocchio/pkg/chatapp/sessions"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// call is one authorized request on one session.
// call is one authorized request on a session: sess is what the guard
// opened for the route's action; info is its metadata.
type call struct {
	principal Principal
	sess      sessionguard.Session
	info      sessions.Info
	sid       sessionstream.SessionId
}

var errInvalidSessionID = errors.New("httpapi: session ids are 1-128 characters of [A-Za-z0-9._:-]")

func validSessionID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '_', c == ':', c == '-':
		default:
			return false
		}
	}
	return true
}

// session wraps a handler for /sessions/{id}/...: it authenticates, finds (or
// auto-creates) the session and authorizes the action.
func (s *Server) session(action Action, fn func(http.ResponseWriter, *http.Request, *call)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.authenticateRequest(w, r)
		if !ok {
			return
		}
		r = r.WithContext(withPrincipal(r.Context(), p))
		id := r.PathValue("id")
		if !validSessionID(id) {
			writeError(w, http.StatusBadRequest, CodeInvalidSessionID, errInvalidSessionID)
			return
		}
		sess, err := s.guard.For(p).Open(r.Context(), id, action)
		if err != nil {
			writeLookupError(w, err)
			return
		}
		fn(w, r, &call{principal: p, sess: sess, info: sess.Info(), sid: sessionstream.SessionId(id)})
	}
}

func writeLookupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sessions.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeSessionNotFound, err)
	case errors.As(err, new(sessionguard.ForbiddenError)):
		writeError(w, http.StatusForbidden, CodeForbidden, err)
	case errors.Is(err, errInvalidSessionID):
		writeError(w, http.StatusBadRequest, CodeInvalidSessionID, err)
	case errors.As(err, new(sessionguard.CreateRejectedError)):
		writeCodedError(w, http.StatusBadRequest, CodeInvalidRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
	}
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	ctx := withPrincipal(r.Context(), p)
	var in CreateSessionRequest
	if _, ok := s.decodeBody(w, r, &in); !ok {
		return
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.NewString()
	}
	if !validSessionID(id) {
		writeError(w, http.StatusBadRequest, CodeInvalidSessionID, errInvalidSessionID)
		return
	}
	sess, err := s.guard.For(p).Create(ctx, sessions.Info{ID: id, Title: strings.TrimSpace(in.Title), Labels: in.Labels, Data: in.Data})
	if errors.Is(err, sessions.ErrExists) {
		writeError(w, http.StatusConflict, CodeSessionExists, err)
		return
	}
	if err != nil {
		writeLookupError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, SessionResponse{SessionID: id, Session: sess.Info()})
}

// handleList answers GET /sessions. An authenticated principal sees only its
// own sessions. ?label=k:v (repeatable) filters on labels, ?limit=n limits.
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	p, ok := s.authenticateRequest(w, r)
	if !ok {
		return
	}
	ctx := withPrincipal(r.Context(), p)
	var filter sessions.Filter // the guard limits it to the principal's own sessions
	for _, raw := range r.URL.Query()["label"] {
		k, v, found := strings.Cut(raw, ":")
		if !found || strings.TrimSpace(k) == "" {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, errors.New("label filters are key:value"))
			return
		}
		if filter.Labels == nil {
			filter.Labels = map[string]string{}
		}
		filter.Labels[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, errors.New("limit must be a non-negative integer"))
			return
		}
		filter.Limit = n
	}
	list, err := s.guard.For(p).List(ctx, filter)
	if err != nil {
		writeLookupError(w, err)
		return
	}
	out := make([]SessionSummary, 0, len(list))
	for _, info := range list {
		out = append(out, SessionSummary{Info: info, Running: s.service.Running(sessionstream.SessionId(info.ID))})
	}
	writeJSON(w, http.StatusOK, SessionListResponse{Sessions: out})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, c *call) {
	snap, err := s.service.Snapshot(r.Context(), c.sid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	encoded := encodeSnapshotResponse(snap)
	writeJSON(w, http.StatusOK, SessionResponse{SessionID: string(c.sid), Session: c.info, Running: s.service.Running(c.sid), Snapshot: &encoded})
}

// handleDelete stops a running turn, waits for it to end, closes the
// session's event streams and deletes the metadata. The conversation in the
// turn store and the timeline in the hydration store remain; the session
// store keeps the id taken (a tombstone), so no one can create it again and
// read them.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, c *call) {
	if err := s.service.Stop(r.Context(), c.sid); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	waitCtx, cancel := context.WithTimeout(r.Context(), shutdownTimeout)
	defer cancel()
	if err := s.service.WaitIdle(waitCtx, c.sid); err != nil {
		log.Warn().Err(err).Str("session", string(c.sid)).Msg("httpapi: delete: run did not end in time")
	}
	s.sse.closeSession(c.sid)
	if err := c.sess.Delete(r.Context()); err != nil && !errors.Is(err, sessions.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request, c *call) {
	if err := s.service.Stop(r.Context(), c.sid); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	writeJSON(w, http.StatusAccepted, AcceptedResponse{SessionID: string(c.sid), Accepted: true})
}

// handleMessage starts a run: it resolves the runtime, asks the BlocksFunc
// for the application's blocks and submits the prompt. A session with a
// running turn answers 409 run_active (chatapp.BusyReject, the default), and
// nothing of the refused message is published.
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request, c *call) {
	var msg MessageRequest
	raw, ok := s.decodeBody(w, r, &msg)
	if !ok {
		return
	}
	msg.Raw = raw
	attachments := make([]chatapp.Attachment, 0, len(msg.Attachments))
	for _, ref := range msg.Attachments {
		id := strings.TrimSpace(ref.AttachmentID)
		if id == "" {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, errors.New("attachment_id must not be empty"))
			return
		}
		attachments = append(attachments, chatapp.Attachment{ID: id, Kind: chatapp.AttachmentKindImage})
	}
	if strings.TrimSpace(msg.Prompt) == "" && len(attachments) == 0 {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, errors.New("a message needs a prompt or attachments"))
		return
	}
	runtime, err := s.resolver.Resolve(r.Context(), r, c.info, msg)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, CodeRuntimeError, err)
		return
	}
	var blocks []turns.Block
	if s.blocks != nil {
		blocks, err = s.blocks(r.Context(), c.info, msg)
		if err != nil {
			writeCodedError(w, http.StatusBadRequest, CodeInvalidRequest, err)
			return
		}
	}
	runID, err := s.service.NewMessageID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	err = s.service.SubmitPromptRequest(r.Context(), c.sid, chatapp.PromptRequest{
		MessageID:      runID,
		Prompt:         msg.Prompt,
		Attachments:    attachments,
		IdempotencyKey: msg.IdempotencyKey,
		Runtime:        runtime,
		Blocks:         blocks,
	})
	switch {
	case errors.Is(err, chatapp.ErrRunActive):
		writeError(w, http.StatusConflict, CodeRunActive, err)
		return
	case errors.Is(err, chatapp.ErrNoRuntime):
		writeError(w, http.StatusInternalServerError, CodeNoRuntime, err)
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	if c.info.Title == "" && strings.TrimSpace(msg.Prompt) != "" {
		title := sessions.TitleFrom(msg.Prompt, 80)
		if err := c.sess.Update(r.Context(), func(info *sessions.Info) error {
			if info.Title == "" {
				info.Title = title
			}
			return nil
		}); err != nil {
			log.Warn().Err(err).Str("session", string(c.sid)).Msg("httpapi: set session title")
		}
	}
	writeJSON(w, http.StatusAccepted, AcceptedResponse{SessionID: string(c.sid), Accepted: true, MessageID: runID})
}

// writeCodedError answers with err's chatapp.CodedError code, or fallback.
func writeCodedError(w http.ResponseWriter, status int, fallback string, err error) {
	code := fallback
	var coded chatapp.CodedError
	if errors.As(err, &coded) {
		code = coded.Code()
		// a hook's code that has a status of its own answers with it
		switch code {
		case CodeSessionExists, CodeRunActive:
			status = http.StatusConflict
		case CodeSessionNotFound:
			status = http.StatusNotFound
		case CodeForbidden:
			status = http.StatusForbidden
		case CodeUnauthenticated:
			status = http.StatusUnauthorized
		}
	}
	writeError(w, status, code, err)
}

func readBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
}

func asMaxBytes(err error, target **http.MaxBytesError) bool {
	return errors.As(err, target)
}
