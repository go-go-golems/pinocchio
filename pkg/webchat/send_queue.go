package webchat

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

type queuedChat struct {
	IdempotencyKey string
	ProfileSlug    string
	Overrides      map[string]any
	Prompt         string
	EnqueuedAt     time.Time
}

type chatRequestRecord struct {
	IdempotencyKey string
	Status         string // queued|running|completed|error

	EnqueuedAt  time.Time
	StartedAt   time.Time
	CompletedAt time.Time

	Response map[string]any
	Error    string
}

// SessionPreparation describes what to do with an incoming chat request.
type SessionPreparation struct {
	Start      bool
	HTTPStatus int
	Response   map[string]any
}

func (c *Conversation) ensureQueueInitLocked() {
	if c.requests == nil {
		c.requests = map[string]*chatRequestRecord{}
	}
}

func (c *Conversation) isBusyLocked() bool {
	if c == nil {
		return false
	}
	if c.activeRequestKey != "" {
		return true
	}
	if c.Sess != nil && c.Sess.IsRunning() {
		return true
	}
	return false
}

func (c *Conversation) getRecordLocked(idempotencyKey string) (*chatRequestRecord, bool) {
	if c == nil || idempotencyKey == "" || c.requests == nil {
		return nil, false
	}
	rec, ok := c.requests[idempotencyKey]
	return rec, ok
}

func (c *Conversation) upsertRecordLocked(rec *chatRequestRecord) {
	if c == nil || rec == nil || rec.IdempotencyKey == "" {
		return
	}
	c.ensureQueueInitLocked()
	c.requests[rec.IdempotencyKey] = rec
}

func (c *Conversation) enqueueLocked(q queuedChat) int {
	if c == nil {
		return -1
	}
	c.queue = append(c.queue, q)
	return len(c.queue)
}

func (c *Conversation) dequeueLocked() (queuedChat, bool) {
	if c == nil || len(c.queue) == 0 {
		return queuedChat{}, false
	}
	q := c.queue[0]
	c.queue = c.queue[1:]
	return q, true
}

// PrepareSessionInference applies idempotency + queue logic and indicates whether inference should start now.
func (c *Conversation) PrepareSessionInference(idempotencyKey, profileSlug string, overrides map[string]any, prompt string) (SessionPreparation, error) {
	if c == nil {
		return SessionPreparation{}, errors.New("conversation is nil")
	}
	if idempotencyKey == "" {
		return SessionPreparation{}, errors.New("idempotency key is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.touchLocked(time.Now())
	c.ensureQueueInitLocked()
	if rec, ok := c.getRecordLocked(idempotencyKey); ok && rec != nil && rec.Response != nil {
		status := strings.ToLower(strings.TrimSpace(rec.Status))
		resp := cloneResponse(rec.Response)
		httpStatus := http.StatusOK
		if status == "queued" {
			httpStatus = http.StatusAccepted
		}
		return SessionPreparation{Start: false, HTTPStatus: httpStatus, Response: resp}, nil
	}

	if c.isBusyLocked() {
		pos := c.enqueueLocked(queuedChat{
			IdempotencyKey: idempotencyKey,
			ProfileSlug:    profileSlug,
			Overrides:      overrides,
			Prompt:         prompt,
			EnqueuedAt:     time.Now(),
		})
		resp := map[string]any{
			"status":          "queued",
			"queue_position":  pos,
			"queue_depth":     len(c.queue),
			"idempotency_key": idempotencyKey,
			"conv_id":         c.ID,
			"session_id":      c.SessionID,
		}
		c.upsertRecordLocked(&chatRequestRecord{
			IdempotencyKey: idempotencyKey,
			Status:         "queued",
			EnqueuedAt:     time.Now(),
			Response:       resp,
		})
		return SessionPreparation{Start: false, HTTPStatus: http.StatusAccepted, Response: resp}, nil
	}

	c.activeRequestKey = idempotencyKey
	resp := map[string]any{
		"status":          "running",
		"idempotency_key": idempotencyKey,
		"conv_id":         c.ID,
		"session_id":      c.SessionID,
	}
	c.upsertRecordLocked(&chatRequestRecord{
		IdempotencyKey: idempotencyKey,
		Status:         "running",
		StartedAt:      time.Now(),
		Response:       resp,
	})
	return SessionPreparation{Start: true, HTTPStatus: http.StatusOK, Response: resp}, nil
}

// ClaimNextQueued pops the next queued request and marks it running.
func (c *Conversation) ClaimNextQueued() (queuedChat, bool) {
	if c == nil {
		return queuedChat{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.touchLocked(time.Now())
	if c.isBusyLocked() {
		return queuedChat{}, false
	}
	q, ok := c.dequeueLocked()
	if !ok {
		return queuedChat{}, false
	}
	c.activeRequestKey = q.IdempotencyKey
	c.ensureQueueInitLocked()
	if rec, ok := c.getRecordLocked(q.IdempotencyKey); ok && rec != nil {
		rec.Status = "running"
		rec.StartedAt = time.Now()
	} else {
		c.upsertRecordLocked(&chatRequestRecord{IdempotencyKey: q.IdempotencyKey, Status: "running", StartedAt: time.Now()})
	}
	return q, true
}

func cloneResponse(resp map[string]any) map[string]any {
	if resp == nil {
		return nil
	}
	out := make(map[string]any, len(resp))
	for k, v := range resp {
		out[k] = v
	}
	return out
}
