package evtstream

import (
	"context"
	"fmt"
	"sync"
)

// SessionMetadataFactory creates metadata for a session on first reference.
type SessionMetadataFactory func(ctx context.Context, sid SessionId) (any, error)

type sessionRegistry struct {
	mu      sync.RWMutex
	factory SessionMetadataFactory
	byID    map[SessionId]*Session
}

func newSessionRegistry(factory SessionMetadataFactory) *sessionRegistry {
	return &sessionRegistry{
		factory: factory,
		byID:    map[SessionId]*Session{},
	}
}

func (r *sessionRegistry) Get(sid SessionId) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byID[sid]
	return s, ok
}

func (r *sessionRegistry) GetOrCreate(ctx context.Context, sid SessionId) (*Session, error) {
	if sid == "" {
		return nil, fmt.Errorf("session id is empty")
	}
	if s, ok := r.Get(sid); ok {
		return s, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.byID[sid]; ok {
		return s, nil
	}
	var metadata any
	var err error
	if r.factory != nil {
		metadata, err = r.factory(ctx, sid)
		if err != nil {
			return nil, fmt.Errorf("build session metadata for %q: %w", sid, err)
		}
	}
	s := &Session{Id: sid, Metadata: metadata}
	r.byID[sid] = s
	return s, nil
}
