package webchat

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
)

// TimelineSemEvent is a simplified SEM envelope passed to custom timeline handlers.
type TimelineSemEvent struct {
	Type     string          `json:"type"`
	ID       string          `json:"id"`
	Seq      uint64          `json:"seq"`
	StreamID string          `json:"stream_id"`
	Data     json.RawMessage `json:"data"`
}

// TimelineSemHandler converts a SEM event into timeline snapshots.
// Returning an error stops processing for that event.
type TimelineSemHandler func(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) error

// TimelineSemRuntime handles SEM events through a pluggable runtime (for example JS reducers/handlers).
// Returning handled=true consumes the event and skips builtin projection.
type TimelineSemRuntime interface {
	HandleSemEvent(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) (handled bool, err error)
}

var (
	timelineHandlersMu sync.RWMutex
	timelineHandlers   = map[string][]TimelineSemHandler{}
	timelineRuntime    TimelineSemRuntime
)

// RegisterTimelineHandler registers a handler for a SEM event type.
func RegisterTimelineHandler(eventType string, handler TimelineSemHandler) {
	if strings.TrimSpace(eventType) == "" || handler == nil {
		return
	}
	timelineHandlersMu.Lock()
	timelineHandlers[eventType] = append(timelineHandlers[eventType], handler)
	timelineHandlersMu.Unlock()
}

// SetTimelineRuntime installs an optional runtime bridge.
func SetTimelineRuntime(runtime TimelineSemRuntime) {
	timelineHandlersMu.Lock()
	prev := timelineRuntime
	timelineRuntime = runtime
	timelineHandlersMu.Unlock()
	closeTimelineRuntime(prev)
}

// ClearTimelineRuntime removes the runtime bridge.
func ClearTimelineRuntime() {
	timelineHandlersMu.Lock()
	prev := timelineRuntime
	timelineRuntime = nil
	timelineHandlersMu.Unlock()
	closeTimelineRuntime(prev)
}

func handleTimelineHandlers(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) (bool, error) {
	if strings.TrimSpace(ev.Type) == "" {
		return false, nil
	}
	timelineHandlersMu.RLock()
	list := append([]TimelineSemHandler(nil), timelineHandlers[ev.Type]...)
	runtime := timelineRuntime
	timelineHandlersMu.RUnlock()

	// Runtime runs before list handlers so consume=true can suppress handler-backed builtin projection.
	if runtime != nil {
		handled, err := runtime.HandleSemEvent(ctx, p, ev, now)
		if err != nil {
			// Treat runtime execution failures as handled to force propagation from ApplySemFrame.
			return true, err
		}
		if handled {
			return true, nil
		}
	}

	handledByList := false
	for _, h := range list {
		if h == nil {
			continue
		}
		handledByList = true
		if err := h(ctx, p, ev, now); err != nil {
			return true, err
		}
	}

	if handledByList {
		return true, nil
	}
	return false, nil
}

// ClearTimelineHandlers removes all registered handlers (useful for tests).
func ClearTimelineHandlers() {
	timelineHandlersMu.Lock()
	prev := timelineRuntime
	timelineHandlers = map[string][]TimelineSemHandler{}
	timelineRuntime = nil
	timelineHandlersMu.Unlock()
	closeTimelineRuntime(prev)
}

func closeTimelineRuntime(runtime TimelineSemRuntime) {
	if runtime == nil {
		return
	}
	if closer, ok := runtime.(interface{ Close(context.Context) error }); ok {
		_ = closer.Close(context.Background())
		return
	}
	if closer, ok := runtime.(io.Closer); ok {
		_ = closer.Close()
	}
}
