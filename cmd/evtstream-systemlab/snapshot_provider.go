package main

import (
	"context"

	sessionstream "github.com/go-go-golems/sessionstream"
)

type hydrationSnapshotProvider struct {
	store sessionstream.HydrationStore
}

func (p hydrationSnapshotProvider) Snapshot(ctx context.Context, sid sessionstream.SessionId) (sessionstream.Snapshot, error) {
	return p.store.Snapshot(ctx, sid, 0)
}
