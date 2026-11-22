package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-go-golems/pinocchio/pkg/chatapp/serverkit"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// SSEConfig tunes the server-sent events transport.
type SSEConfig struct {
	// KeepAlive is how often an idle stream gets a ": ping" comment (default 15s).
	KeepAlive time.Duration
	// QueueSize is how many UI batches a live stream may fall behind before it
	// is closed (default 256). A closed client reconnects and hydrates again.
	QueueSize int
	// HydrationBufferLimit is how many UI batches are kept for a stream while
	// its snapshot is read (default 1024).
	HydrationBufferLimit int
	// WriteTimeout bounds each write to the client (default 10s). It replaces
	// the http.Server's WriteTimeout for the stream.
	WriteTimeout time.Duration
}

// DefaultSSEConfig returns the defaults described on SSEConfig.
func DefaultSSEConfig() SSEConfig {
	return SSEConfig{KeepAlive: 15 * time.Second, QueueSize: 256, HydrationBufferLimit: 1024, WriteTimeout: 10 * time.Second}
}

func (c SSEConfig) withDefaults() SSEConfig {
	d := DefaultSSEConfig()
	if c.KeepAlive <= 0 {
		c.KeepAlive = d.KeepAlive
	}
	if c.QueueSize <= 0 {
		c.QueueSize = d.QueueSize
	}
	if c.HydrationBufferLimit <= 0 {
		c.HydrationBufferLimit = d.HydrationBufferLimit
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = d.WriteTimeout
	}
	return c
}

// UIEventMessage is the data of an SSE "ui" event.
type UIEventMessage struct {
	SessionID string `json:"sessionId"`
	Ordinal   string `json:"ordinal"`
	Name      string `json:"name"`
	Payload   any    `json:"payload,omitempty"`
}

// StreamError is the data of an SSE "error" event; the stream ends after it.
type StreamError struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

type uiBatch struct {
	ord    uint64
	events []sessionstream.UIEvent
}

// sseHub is the UIFanout of the SSE streams. A stream registers before its
// snapshot is read; batches published meanwhile are buffered, and those the
// snapshot does not contain (ordinal above the snapshot's) are written before
// the live ones. Nothing is lost between snapshot and live, nothing is sent
// twice.
type sseHub struct {
	cfg  SSEConfig
	mu   sync.Mutex
	subs map[sessionstream.SessionId]map[*sseSub]struct{}
}

type sseSub struct {
	mu     sync.Mutex
	live   bool
	buffer []uiBatch
	queue  chan uiBatch
	done   chan struct{}
	reason StreamError
	closed bool
	limit  int
}

func newSSEHub(cfg SSEConfig) *sseHub {
	return &sseHub{cfg: cfg, subs: map[sessionstream.SessionId]map[*sseSub]struct{}{}}
}

func (h *sseHub) subscribe(sid sessionstream.SessionId) *sseSub {
	sub := &sseSub{queue: make(chan uiBatch, h.cfg.QueueSize), done: make(chan struct{}), limit: h.cfg.HydrationBufferLimit}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[sid] == nil {
		h.subs[sid] = map[*sseSub]struct{}{}
	}
	h.subs[sid][sub] = struct{}{}
	return sub
}

func (h *sseHub) unsubscribe(sid sessionstream.SessionId, sub *sseSub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs[sid], sub)
	if len(h.subs[sid]) == 0 {
		delete(h.subs, sid)
	}
}

func (h *sseHub) targets(sid sessionstream.SessionId) []*sseSub {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*sseSub, 0, len(h.subs[sid]))
	for sub := range h.subs[sid] {
		out = append(out, sub)
	}
	return out
}

func (h *sseHub) PublishUI(_ context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	if len(events) == 0 {
		return nil
	}
	for _, sub := range h.targets(sid) {
		sub.offer(uiBatch{ord: ord, events: events})
	}
	return nil
}

var _ sessionstream.UIFanout = (*sseHub)(nil)

func (h *sseHub) closeSession(sid sessionstream.SessionId) {
	for _, sub := range h.targets(sid) {
		sub.close(StreamError{Code: "session_deleted", Error: "the session was deleted"})
	}
}

func (h *sseHub) closeAll() {
	h.mu.Lock()
	all := []*sseSub{}
	for _, subs := range h.subs {
		for sub := range subs {
			all = append(all, sub)
		}
	}
	h.mu.Unlock()
	for _, sub := range all {
		sub.close(StreamError{Code: "server_closed", Error: "the server is shutting down"})
	}
}

// offer never blocks: it runs inside the hub's publish.
func (sub *sseSub) offer(b uiBatch) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed {
		return
	}
	if !sub.live {
		if len(sub.buffer) >= sub.limit {
			sub.closeLocked(StreamError{Code: "hydration_buffer_overflow", Error: "too many events while the snapshot was read; reconnect"})
			return
		}
		sub.buffer = append(sub.buffer, b)
		return
	}
	select {
	case sub.queue <- b:
	default:
		sub.closeLocked(StreamError{Code: "stream_overflow", Error: "the client fell too far behind; reconnect"})
	}
}

// goLive returns the buffered batches after the snapshot and switches the
// subscription to live delivery.
func (sub *sseSub) goLive(snapshotOrdinal uint64) []uiBatch {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	pending := make([]uiBatch, 0, len(sub.buffer))
	for _, b := range sub.buffer {
		if b.ord > snapshotOrdinal {
			pending = append(pending, b)
		}
	}
	sub.buffer = nil
	sub.live = true
	return pending
}

func (sub *sseSub) close(reason StreamError) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	sub.closeLocked(reason)
}

func (sub *sseSub) closeLocked(reason StreamError) {
	if sub.closed {
		return
	}
	sub.closed = true
	sub.reason = reason
	close(sub.done)
}

func (sub *sseSub) closeReason() StreamError {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.reason
}

// handleEvents answers GET /sessions/{id}/events with a server-sent event
// stream: one "snapshot" event (a serverkit.SessionSnapshotResponse), then a
// "ui" event (UIEventMessage) per UI event, with the ordinal as the SSE id.
// A reconnecting client hydrates again from a new snapshot; Last-Event-ID is
// ignored.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, c *call) {
	sub := s.sse.subscribe(c.sid)
	defer s.sse.unsubscribe(c.sid, sub)
	snap, err := s.service.Snapshot(r.Context(), c.sid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	send := func(write func(io.Writer) error) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(s.sseConfig.WriteTimeout))
		if err := write(w); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	sendBatch := func(b uiBatch) bool {
		return send(func(out io.Writer) error {
			for _, ev := range b.events {
				if err := writeSSE(out, "ui", strconv.FormatUint(b.ord, 10), UIEventMessage{
					SessionID: string(c.sid), Ordinal: strconv.FormatUint(b.ord, 10), Name: ev.Name, Payload: serverkit.EncodeProtoJSON(ev.Payload),
				}); err != nil {
					return err
				}
			}
			return nil
		})
	}

	if !send(func(out io.Writer) error { return writeSSE(out, "snapshot", "", encodeSnapshotResponse(snap)) }) {
		return
	}
	for _, b := range sub.goLive(snap.SnapshotOrdinal) {
		if !sendBatch(b) {
			return
		}
	}
	ping := time.NewTicker(s.sseConfig.KeepAlive)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-sub.queue:
			if !sendBatch(b) {
				return
			}
		case <-sub.done:
			// Deliver what was queued before the close, then say why it ended.
			for drained := false; !drained; {
				select {
				case b := <-sub.queue:
					if !sendBatch(b) {
						return
					}
				default:
					drained = true
				}
			}
			reason := sub.closeReason()
			send(func(out io.Writer) error { return writeSSE(out, "error", "", reason) })
			return
		case <-ping.C:
			if !send(func(out io.Writer) error { _, err := io.WriteString(out, ": ping\n\n"); return err }) {
				return
			}
		}
	}
}

func writeSSE(w io.Writer, event, id string, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
	return err
}
