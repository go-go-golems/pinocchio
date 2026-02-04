package webchat

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// InMemoryTimelineStore is a size-limited, in-memory TimelineStore implementation.
// It mirrors the ordering semantics of the SQLite store to keep hydration behavior consistent.
type InMemoryTimelineStore struct {
	mu                 sync.Mutex
	maxEntitiesPerConv int
	convs              map[string]*inMemTimeline
}

type inMemTimeline struct {
	version       uint64
	entities      map[string]*timelinepb.TimelineEntityV1
	entityVersion map[string]uint64
}

var _ TimelineStore = &InMemoryTimelineStore{}

func NewInMemoryTimelineStore(maxEntitiesPerConv int) *InMemoryTimelineStore {
	if maxEntitiesPerConv <= 0 {
		maxEntitiesPerConv = 5000
	}
	return &InMemoryTimelineStore{
		maxEntitiesPerConv: maxEntitiesPerConv,
		convs:              map[string]*inMemTimeline{},
	}
}

func (s *InMemoryTimelineStore) Close() error { return nil }

func (s *InMemoryTimelineStore) Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV1) error {
	if s == nil {
		return errors.New("in-memory timeline store: nil store")
	}
	if convID == "" {
		return errors.New("in-memory timeline store: convID is empty")
	}
	if version == 0 {
		return errors.New("in-memory timeline store: version is 0")
	}
	if entity == nil {
		return errors.New("in-memory timeline store: entity is nil")
	}
	if entity.Id == "" {
		return errors.New("in-memory timeline store: entity.id is empty")
	}
	if entity.Kind == "" {
		return errors.New("in-memory timeline store: entity.kind is empty")
	}
	_ = ctx

	s.mu.Lock()
	defer s.mu.Unlock()

	conv := s.convs[convID]
	if conv == nil {
		conv = &inMemTimeline{
			version:       0,
			entities:      map[string]*timelinepb.TimelineEntityV1{},
			entityVersion: map[string]uint64{},
		}
		s.convs[convID] = conv
	}

	newVersion := version

	now := time.Now().UnixMilli()
	createdAt := entity.CreatedAtMs
	if existing, ok := conv.entities[entity.Id]; ok && existing != nil && existing.CreatedAtMs > 0 {
		createdAt = existing.CreatedAtMs
	}
	if createdAt == 0 {
		createdAt = now
	}

	clone := proto.Clone(entity).(*timelinepb.TimelineEntityV1)
	clone.CreatedAtMs = createdAt
	clone.UpdatedAtMs = now

	conv.entities[entity.Id] = clone
	conv.entityVersion[entity.Id] = newVersion
	if newVersion > conv.version {
		conv.version = newVersion
	}

	// Enforce per-conversation size limit by evicting the oldest versioned entities.
	if s.maxEntitiesPerConv > 0 && len(conv.entities) > s.maxEntitiesPerConv {
		type pair struct {
			id      string
			version uint64
		}
		pairs := make([]pair, 0, len(conv.entityVersion))
		for id, v := range conv.entityVersion {
			pairs = append(pairs, pair{id: id, version: v})
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].version == pairs[j].version {
				return pairs[i].id < pairs[j].id
			}
			return pairs[i].version < pairs[j].version
		})
		toDrop := len(conv.entities) - s.maxEntitiesPerConv
		for i := 0; i < toDrop && i < len(pairs); i++ {
			delete(conv.entities, pairs[i].id)
			delete(conv.entityVersion, pairs[i].id)
		}
	}

	return nil
}

func (s *InMemoryTimelineStore) GetSnapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, error) {
	if s == nil {
		return nil, errors.New("in-memory timeline store: nil store")
	}
	if convID == "" {
		return nil, errors.New("in-memory timeline store: convID is empty")
	}
	_ = ctx
	if limit <= 0 {
		limit = 5000
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	conv := s.convs[convID]
	if conv == nil {
		return &timelinepb.TimelineSnapshotV1{
			ConvId:       convID,
			Version:      0,
			ServerTimeMs: time.Now().UnixMilli(),
			Entities:     nil,
		}, nil
	}

	type pair struct {
		entity  *timelinepb.TimelineEntityV1
		version uint64
	}
	pairs := make([]pair, 0, len(conv.entities))
	for id, e := range conv.entities {
		v := conv.entityVersion[id]
		if sinceVersion > 0 && v <= sinceVersion {
			continue
		}
		pairs = append(pairs, pair{entity: e, version: v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].version == pairs[j].version {
			if pairs[i].entity == nil || pairs[j].entity == nil {
				return pairs[i].version < pairs[j].version
			}
			return pairs[i].entity.Id < pairs[j].entity.Id
		}
		return pairs[i].version < pairs[j].version
	})

	if len(pairs) > limit {
		pairs = pairs[:limit]
	}

	entities := make([]*timelinepb.TimelineEntityV1, 0, len(pairs))
	for _, p := range pairs {
		if p.entity == nil {
			continue
		}
		clone := proto.Clone(p.entity).(*timelinepb.TimelineEntityV1)
		entities = append(entities, clone)
	}

	return &timelinepb.TimelineSnapshotV1{
		ConvId:       convID,
		Version:      conv.version,
		ServerTimeMs: time.Now().UnixMilli(),
		Entities:     entities,
	}, nil
}
