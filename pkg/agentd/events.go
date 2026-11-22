package agentd

import (
	"sync"
	"time"
)

// Event types sent to the page. Names follow the HB-002 protocol.
const (
	EvRunStarted     = "run.started"
	EvRunFinished    = "run.finished"
	EvRunFailed      = "run.failed"
	EvRunCancelled   = "run.cancelled"
	EvTextDelta      = "text.delta"
	EvReasoningDelta = "reasoning.delta"
	EvToolRequested  = "tool.requested"
	EvToolFinished   = "tool.finished"
	EvToolProgress   = "tool.progress"
	EvToolsManifest  = "tools.manifest"
	EvUsage          = "usage"
	EvUserMessage    = "user.message"
)

// Event is one entry of a session's event log and one SSE message.
type Event struct {
	Seq     int64     `json:"seq"`
	Type    string    `json:"type"`
	Session string    `json:"session"`
	Run     string    `json:"run,omitempty"`
	At      time.Time `json:"at"`
	Data    any       `json:"data,omitempty"`
}

// EventLog is an append-only, bounded, sequence-numbered event list with
// subscribers. A reconnecting subscriber asks for events after a sequence
// number and receives the backlog it missed, then live events.
type EventLog struct {
	mu      sync.Mutex
	session string
	seq     int64
	events  []Event
	max     int
	subs    map[chan Event]struct{}
}

// NewEventLog keeps at most limit events in memory (older ones are dropped).
func NewEventLog(session string, limit int) *EventLog {
	if limit <= 0 {
		limit = 5000
	}
	return &EventLog{session: session, max: limit, subs: map[chan Event]struct{}{}}
}

// Append records an event, assigns its sequence number and fans it out.
// Slow subscribers lose live events; they recover by reconnecting with
// their last sequence number.
func (l *EventLog) Append(typ, run string, data any) Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	ev := Event{Seq: l.seq, Type: typ, Session: l.session, Run: run, At: time.Now().UTC(), Data: data}
	l.events = append(l.events, ev)
	if len(l.events) > l.max {
		l.events = append([]Event(nil), l.events[len(l.events)-l.max:]...)
	}
	for ch := range l.subs {
		select {
		case ch <- ev:
		default:
			close(ch)
			delete(l.subs, ch)
		}
	}
	return ev
}

// Seq returns the last assigned sequence number.
func (l *EventLog) Seq() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seq
}

// Since returns the retained events with Seq > after.
func (l *EventLog) Since(after int64) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return since(l.events, after)
}

func since(evs []Event, after int64) []Event {
	i := len(evs)
	for i > 0 && evs[i-1].Seq > after {
		i--
	}
	return append([]Event(nil), evs[i:]...)
}

// Subscribe returns the backlog after `after` and a channel of later events.
// The channel is closed when the subscriber falls behind or cancel is called.
func (l *EventLog) Subscribe(after int64) ([]Event, <-chan Event, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	backlog := since(l.events, after)
	ch := make(chan Event, 256)
	l.subs[ch] = struct{}{}
	cancel := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, ok := l.subs[ch]; ok {
			delete(l.subs, ch)
			close(ch)
		}
	}
	return backlog, ch, cancel
}

// restore replaces the log (used when a session is loaded from disk).
func (l *EventLog) restore(evs []Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append([]Event(nil), evs...)
	if n := len(l.events); n > 0 {
		l.seq = l.events[n-1].Seq
	}
}
