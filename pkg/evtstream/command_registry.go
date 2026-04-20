package evtstream

import (
	"fmt"
	"sync"
)

type commandRegistry struct {
	mu       sync.RWMutex
	handlers map[string]CommandHandler
}

func newCommandRegistry() *commandRegistry {
	return &commandRegistry{handlers: map[string]CommandHandler{}}
}

func (r *commandRegistry) Register(name string, handler CommandHandler) error {
	if name == "" {
		return fmt.Errorf("command name is empty")
	}
	if handler == nil {
		return fmt.Errorf("command %q handler is nil", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.handlers[name]; ok {
		return fmt.Errorf("command %q already registered", name)
	}
	r.handlers[name] = handler
	return nil
}

func (r *commandRegistry) Lookup(name string) (CommandHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}
