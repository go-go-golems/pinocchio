package evtstream

import "context"

// HydrationStore is the substrate-owned persistence seam.
type HydrationStore interface {
	Apply(ctx context.Context, sid SessionId, ord uint64, entities []TimelineEntity) error
	Snapshot(ctx context.Context, sid SessionId, asOf uint64) (Snapshot, error)
	View(ctx context.Context, sid SessionId) (TimelineView, error)
	Cursor(ctx context.Context, sid SessionId) (uint64, error)
}

// Snapshot is the reconnect payload returned by the store.
type Snapshot struct {
	SessionId SessionId        `json:"sessionId"`
	Ordinal   uint64           `json:"ordinal"`
	Entities  []TimelineEntity `json:"entities"`
}
