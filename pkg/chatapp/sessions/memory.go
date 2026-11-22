package sessions

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryStore keeps sessions in memory.
type MemoryStore struct {
	mu      sync.Mutex
	byID    map[string]Info
	deleted map[string]bool // tombstones: ids never given out again
	now     func() time.Time
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byID: map[string]Info{}, deleted: map[string]bool{}, now: func() time.Time { return time.Now().UTC() }}
}

func (s *MemoryStore) Create(_ context.Context, info Info) error {
	if strings.TrimSpace(info.ID) == "" {
		return errEmptyID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[info.ID]; ok || s.deleted[info.ID] {
		return ErrExists
	}
	now := s.now()
	if info.Created.IsZero() {
		info.Created = now
	}
	if info.Updated.IsZero() {
		info.Updated = info.Created
	}
	s.byID[info.ID] = cloneInfo(info)
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, ok := s.byID[id]
	if !ok {
		return Info{}, ErrNotFound
	}
	return cloneInfo(info), nil
}

func (s *MemoryStore) List(_ context.Context, f Filter) ([]Info, error) {
	s.mu.Lock()
	out := make([]Info, 0, len(s.byID))
	for _, info := range s.byID {
		if f.Matches(info) {
			out = append(out, cloneInfo(info))
		}
	}
	s.mu.Unlock()
	sortNewestFirst(out)
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (s *MemoryStore) Update(_ context.Context, id string, fn func(*Info) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, ok := s.byID[id]
	if !ok {
		return ErrNotFound
	}
	next := cloneInfo(info)
	if err := fn(&next); err != nil {
		return err
	}
	next.ID = id
	next.Updated = s.now()
	s.byID[id] = next
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return ErrNotFound
	}
	delete(s.byID, id)
	s.deleted[id] = true
	return nil
}

func sortNewestFirst(infos []Info) {
	sort.SliceStable(infos, func(i, j int) bool {
		if !infos[i].Created.Equal(infos[j].Created) {
			return infos[i].Created.After(infos[j].Created)
		}
		return infos[i].ID < infos[j].ID
	})
}
