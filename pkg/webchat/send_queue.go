package webchat

import (
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

func (c *Conversation) ensureQueueInitLocked() {
	if c.requests == nil {
		c.requests = map[string]*chatRequestRecord{}
	}
}

func (c *Conversation) isBusyLocked() bool {
	if c == nil {
		return false
	}
	if c.runningKey != "" {
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
