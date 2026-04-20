package main

import (
	"context"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
)

type hydrationSnapshotProvider struct {
	store evtstream.HydrationStore
}

func (p hydrationSnapshotProvider) Snapshot(ctx context.Context, sid evtstream.SessionId) (evtstream.Snapshot, error) {
	return p.store.Snapshot(ctx, sid, 0)
}
