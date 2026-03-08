package webchat

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

type queuedChat struct {
	IdempotencyKey string
	Payload        any
	Metadata       map[string]any
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

func ensurePromptQueueInitLocked(c *Conversation) {
	if c.requests == nil {
		c.requests = map[string]*chatRequestRecord{}
	}
}

func isPromptBusyLocked(c *Conversation) bool {
	if c == nil {
		return false
	}
	if c.activeRequestKey != "" {
		return true
	}
	return false
}

func getPromptRecordLocked(c *Conversation, idempotencyKey string) (*chatRequestRecord, bool) {
	if c == nil || idempotencyKey == "" || c.requests == nil {
		return nil, false
	}
	rec, ok := c.requests[idempotencyKey]
	return rec, ok
}

func upsertPromptRecordLocked(c *Conversation, rec *chatRequestRecord) {
	if c == nil || rec == nil || rec.IdempotencyKey == "" {
		return
	}
	ensurePromptQueueInitLocked(c)
	c.requests[rec.IdempotencyKey] = rec
}

func enqueuePromptLocked(c *Conversation, q queuedChat) int {
	if c == nil {
		return -1
	}
	c.queue = append(c.queue, q)
	return len(c.queue)
}

func dequeuePromptLocked(c *Conversation) (queuedChat, bool) {
	if c == nil || len(c.queue) == 0 {
		return queuedChat{}, false
	}
	q := c.queue[0]
	c.queue = c.queue[1:]
	return q, true
}

// preparePromptSubmission applies idempotency + queue logic and indicates whether the next runner should start now.
func preparePromptSubmission(c *Conversation, idempotencyKey string, payload any, metadata map[string]any) (SessionPreparation, error) {
	if c == nil {
		return SessionPreparation{}, errors.New("conversation is nil")
	}
	if idempotencyKey == "" {
		return SessionPreparation{}, errors.New("idempotency key is empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.touchLocked(time.Now())
	ensurePromptQueueInitLocked(c)
	if rec, ok := getPromptRecordLocked(c, idempotencyKey); ok && rec != nil && rec.Response != nil {
		status := strings.ToLower(strings.TrimSpace(rec.Status))
		resp := cloneResponse(rec.Response)
		httpStatus := http.StatusOK
		if status == "queued" {
			httpStatus = http.StatusAccepted
		}
		return SessionPreparation{Start: false, HTTPStatus: httpStatus, Response: resp}, nil
	}

	if isPromptBusyLocked(c) {
		pos := enqueuePromptLocked(c, queuedChat{
			IdempotencyKey: idempotencyKey,
			Payload:        payload,
			Metadata:       cloneResponse(metadata),
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
		upsertPromptRecordLocked(c, &chatRequestRecord{
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
	upsertPromptRecordLocked(c, &chatRequestRecord{
		IdempotencyKey: idempotencyKey,
		Status:         "running",
		StartedAt:      time.Now(),
		Response:       resp,
	})
	return SessionPreparation{Start: true, HTTPStatus: http.StatusOK, Response: resp}, nil
}

// claimNextQueuedPrompt pops the next queued request and marks it running.
func claimNextQueuedPrompt(c *Conversation) (queuedChat, bool) {
	if c == nil {
		return queuedChat{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.touchLocked(time.Now())
	if isPromptBusyLocked(c) {
		return queuedChat{}, false
	}
	q, ok := dequeuePromptLocked(c)
	if !ok {
		return queuedChat{}, false
	}
	c.activeRequestKey = q.IdempotencyKey
	ensurePromptQueueInitLocked(c)
	if rec, ok := getPromptRecordLocked(c, q.IdempotencyKey); ok && rec != nil {
		rec.Status = "running"
		rec.StartedAt = time.Now()
	} else {
		upsertPromptRecordLocked(c, &chatRequestRecord{IdempotencyKey: q.IdempotencyKey, Status: "running", StartedAt: time.Now()})
	}
	return q, true
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneResponse(resp map[string]any) map[string]any {
	return cloneStringAnyMap(resp)
}
