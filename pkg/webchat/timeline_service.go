package webchat

import (
	"context"

	"github.com/pkg/errors"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// TimelineService exposes timeline hydration reads independent of Router wiring.
type TimelineService struct {
	store chatstore.TimelineStore
}

func NewTimelineService(store chatstore.TimelineStore) *TimelineService {
	return &TimelineService{store: store}
}

func (s *TimelineService) Snapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV2, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("timeline service not enabled")
	}
	return s.store.GetSnapshot(ctx, convID, sinceVersion, limit)
}
