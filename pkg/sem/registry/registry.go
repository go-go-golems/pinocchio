package registry

import (
	"reflect"
	"sync"

	"github.com/go-go-golems/geppetto/pkg/events"
)

// Handler maps a Geppetto event to zero or more SEM frames (each a JSON message).
// A handler should be idempotent and safe to call multiple times for the same
// logical event stream.
type Handler func(e events.Event) ([][]byte, error)

var (
	mu       sync.RWMutex
	handlers = map[reflect.Type][]Handler{}
)

// RegisterByType registers a handler for a specific event type T (usually a pointer type).
// The handler will be invoked only when the incoming event is assignable to T.
func RegisterByType[T any](fn func(T) ([][]byte, error)) {
	mu.Lock()
	defer mu.Unlock()

	typ := reflect.TypeOf((*T)(nil)).Elem()
	wrapped := func(e events.Event) ([][]byte, error) {
		if v, ok := any(e).(T); ok {
			return fn(v)
		}
		return nil, nil
	}

	handlers[typ] = append(handlers[typ], wrapped)
}

// Handle attempts to process an event using registered handlers.
// Returns frames, whether a handler was found for the event type, and any error.
func Handle(e events.Event) ([][]byte, bool, error) {
	if e == nil {
		return nil, false, nil
	}

	mu.RLock()
	hlist := append([]Handler(nil), handlers[reflect.TypeOf(e)]...)
	mu.RUnlock()

	if len(hlist) == 0 {
		return nil, false, nil
	}

	for _, h := range hlist {
		if h == nil {
			continue
		}
		frames, err := h(e)
		if frames != nil || err != nil {
			return frames, true, err
		}
	}

	return nil, true, nil
}

// Clear removes all registered handlers (useful for tests).
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	handlers = map[reflect.Type][]Handler{}
}
