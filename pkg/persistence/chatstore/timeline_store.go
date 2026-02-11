package chatstore

import (
	"context"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// TimelineStore is the durable "actual hydration" projection store.
//
// It stores the canonical timeline entity set for a conversation and supports
// snapshot retrieval by a per-conversation monotonic version.
type TimelineStore interface {
	Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV1) error
	GetSnapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, error)
	Close() error
}
