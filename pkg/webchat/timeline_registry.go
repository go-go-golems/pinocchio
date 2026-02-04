package webchat

import (
	"context"
	"encoding/json"
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

var (
	timelineHandlersMu sync.RWMutex
	timelineHandlers   = map[string][]TimelineSemHandler{}
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

func handleTimelineHandlers(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) (bool, error) {
	if strings.TrimSpace(ev.Type) == "" {
		return false, nil
	}
	timelineHandlersMu.RLock()
	list := append([]TimelineSemHandler(nil), timelineHandlers[ev.Type]...)
	timelineHandlersMu.RUnlock()
	if len(list) == 0 {
		return false, nil
	}
	for _, h := range list {
		if h == nil {
			continue
		}
		if err := h(ctx, p, ev, now); err != nil {
			return true, err
		}
	}
	return true, nil
}

// ClearTimelineHandlers removes all registered handlers (useful for tests).
func ClearTimelineHandlers() {
	timelineHandlersMu.Lock()
	timelineHandlers = map[string][]TimelineSemHandler{}
	timelineHandlersMu.Unlock()
}
