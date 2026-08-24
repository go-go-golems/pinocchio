package frontendtools

import (
	"container/list"
	"time"
)

type terminalOrigin uint8

const (
	terminalOriginResult terminalOrigin = iota + 1
	terminalOriginContext
)

type terminalCall struct {
	key         pendingKey
	toolName    string
	digest      [32]byte
	origin      terminalOrigin
	completedAt time.Time
}

type terminalEntry struct {
	key    pendingKey
	record *terminalCall
}

// boundedTerminalStore is guarded by Manager.mu. It uses insertion order for
// deterministic age/count eviction; terminal entries are never refreshed by a
// retry, so repeated delivery cannot extend retention indefinitely.
type boundedTerminalStore struct {
	maxEntries int
	ttl        time.Duration
	byKey      map[pendingKey]*list.Element
	order      *list.List
}

func newBoundedTerminalStore(maxEntries int, ttl time.Duration) *boundedTerminalStore {
	return &boundedTerminalStore{
		maxEntries: maxEntries,
		ttl:        ttl,
		byKey:      make(map[pendingKey]*list.Element, maxEntries),
		order:      list.New(),
	}
}

func (s *boundedTerminalStore) get(key pendingKey, now time.Time) (*terminalCall, bool) {
	if s == nil {
		return nil, false
	}
	s.evictExpired(now)
	element := s.byKey[key]
	if element == nil {
		return nil, false
	}
	return element.Value.(terminalEntry).record, true
}

func (s *boundedTerminalStore) add(record *terminalCall, now time.Time) {
	if s == nil || record == nil {
		return
	}
	s.evictExpired(now)
	if existing := s.byKey[record.key]; existing != nil {
		s.order.Remove(existing)
		delete(s.byKey, record.key)
	}
	element := s.order.PushBack(terminalEntry{key: record.key, record: record})
	s.byKey[record.key] = element
	for s.order.Len() > s.maxEntries {
		s.removeOldest()
	}
}

func (s *boundedTerminalStore) hasToolCallInOtherSession(key pendingKey, now time.Time) bool {
	if s == nil {
		return false
	}
	s.evictExpired(now)
	for candidate := range s.byKey {
		if candidate.sessionID != key.sessionID && candidate.toolCallID == key.toolCallID {
			return true
		}
	}
	return false
}

func (s *boundedTerminalStore) len(now time.Time) int {
	if s == nil {
		return 0
	}
	s.evictExpired(now)
	return len(s.byKey)
}

func (s *boundedTerminalStore) evictExpired(now time.Time) {
	if s == nil {
		return
	}
	for {
		oldest := s.order.Front()
		if oldest == nil {
			return
		}
		record := oldest.Value.(terminalEntry).record
		if now.Sub(record.completedAt) < s.ttl {
			return
		}
		s.removeOldest()
	}
}

func (s *boundedTerminalStore) removeOldest() {
	oldest := s.order.Front()
	if oldest == nil {
		return
	}
	entry := oldest.Value.(terminalEntry)
	delete(s.byKey, entry.key)
	s.order.Remove(oldest)
}
