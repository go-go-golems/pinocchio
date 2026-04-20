package evtstream

import "context"

type noopHydrationStore struct{}

func newNoopHydrationStore() HydrationStore { return noopHydrationStore{} }

func (noopHydrationStore) Apply(_ context.Context, sid SessionId, ord uint64, entities []TimelineEntity) error {
	_ = sid
	_ = ord
	_ = entities
	return nil
}

func (noopHydrationStore) Snapshot(_ context.Context, sid SessionId, _ uint64) (Snapshot, error) {
	return Snapshot{SessionId: sid}, nil
}

func (noopHydrationStore) View(_ context.Context, _ SessionId) (TimelineView, error) {
	return emptyTimelineView{}, nil
}

func (noopHydrationStore) Cursor(_ context.Context, _ SessionId) (uint64, error) {
	return 0, nil
}

type emptyTimelineView struct{}

func (emptyTimelineView) Get(kind, id string) (TimelineEntity, bool) {
	_ = kind
	_ = id
	return TimelineEntity{}, false
}

func (emptyTimelineView) List(kind string) []TimelineEntity {
	_ = kind
	return nil
}

func (emptyTimelineView) Ordinal() uint64 { return 0 }
