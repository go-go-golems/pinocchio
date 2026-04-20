package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	"google.golang.org/protobuf/proto"
)

type entityKey struct {
	kind string
	id   string
}

type sessionState struct {
	cursor   uint64
	entities map[entityKey]evtstream.TimelineEntity
}

// Store is an in-memory hydration store used for phase 1.
type Store struct {
	mu       sync.RWMutex
	sessions map[evtstream.SessionId]*sessionState
}

var _ evtstream.HydrationStore = (*Store)(nil)

func New() *Store {
	return &Store{sessions: map[evtstream.SessionId]*sessionState{}}
}

func (s *Store) Apply(_ context.Context, sid evtstream.SessionId, ord uint64, entities []evtstream.TimelineEntity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.getOrCreateLocked(sid)
	for _, entity := range entities {
		key := entityKey{kind: entity.Kind, id: entity.Id}
		if entity.Tombstone {
			delete(state.entities, key)
			continue
		}
		state.entities[key] = cloneEntity(entity)
	}
	if ord > state.cursor {
		state.cursor = ord
	}
	return nil
}

func (s *Store) Snapshot(_ context.Context, sid evtstream.SessionId, _ uint64) (evtstream.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.sessions[sid]
	if state == nil {
		return evtstream.Snapshot{SessionId: sid}, nil
	}
	entities := make([]evtstream.TimelineEntity, 0, len(state.entities))
	for _, entity := range state.entities {
		entities = append(entities, cloneEntity(entity))
	}
	sort.Slice(entities, func(i, j int) bool {
		if entities[i].Kind == entities[j].Kind {
			return entities[i].Id < entities[j].Id
		}
		return entities[i].Kind < entities[j].Kind
	})
	return evtstream.Snapshot{SessionId: sid, Ordinal: state.cursor, Entities: entities}, nil
}

func (s *Store) View(ctx context.Context, sid evtstream.SessionId) (evtstream.TimelineView, error) {
	snap, err := s.Snapshot(ctx, sid, 0)
	if err != nil {
		return nil, err
	}
	return newView(snap), nil
}

func (s *Store) Cursor(_ context.Context, sid evtstream.SessionId) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.sessions[sid]
	if state == nil {
		return 0, nil
	}
	return state.cursor, nil
}

func (s *Store) getOrCreateLocked(sid evtstream.SessionId) *sessionState {
	state := s.sessions[sid]
	if state == nil {
		state = &sessionState{entities: map[entityKey]evtstream.TimelineEntity{}}
		s.sessions[sid] = state
	}
	return state
}

type view struct {
	ordinal uint64
	index   map[entityKey]evtstream.TimelineEntity
}

func newView(snap evtstream.Snapshot) *view {
	index := map[entityKey]evtstream.TimelineEntity{}
	for _, entity := range snap.Entities {
		index[entityKey{kind: entity.Kind, id: entity.Id}] = cloneEntity(entity)
	}
	return &view{ordinal: snap.Ordinal, index: index}
}

func (v *view) Get(kind, id string) (evtstream.TimelineEntity, bool) {
	entity, ok := v.index[entityKey{kind: kind, id: id}]
	if !ok {
		return evtstream.TimelineEntity{}, false
	}
	return cloneEntity(entity), true
}

func (v *view) List(kind string) []evtstream.TimelineEntity {
	ret := make([]evtstream.TimelineEntity, 0)
	for _, entity := range v.index {
		if kind != "" && entity.Kind != kind {
			continue
		}
		ret = append(ret, cloneEntity(entity))
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Kind == ret[j].Kind {
			return ret[i].Id < ret[j].Id
		}
		return ret[i].Kind < ret[j].Kind
	})
	return ret
}

func (v *view) Ordinal() uint64 { return v.ordinal }

func cloneEntity(entity evtstream.TimelineEntity) evtstream.TimelineEntity {
	out := entity
	if entity.Payload != nil {
		out.Payload = proto.Clone(entity.Payload)
	}
	return out
}
