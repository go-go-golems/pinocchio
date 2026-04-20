package evtstream

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// UIEvent is what a client-facing UI consumes.
type UIEvent struct {
	Name    string
	Payload proto.Message
}

// TimelineEntity is the value type persisted in hydration state.
type TimelineEntity struct {
	Kind      string
	Id        string
	Payload   proto.Message
	Tombstone bool
}

// TimelineView is a read-only view of current timeline state.
type TimelineView interface {
	Get(kind, id string) (TimelineEntity, bool)
	List(kind string) []TimelineEntity
	Ordinal() uint64
}

// UIProjection turns one backend event into zero or more UI events.
type UIProjection interface {
	Project(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]UIEvent, error)
}

// UIProjectionFunc adapts a function to UIProjection.
type UIProjectionFunc func(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]UIEvent, error)

func (f UIProjectionFunc) Project(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]UIEvent, error) {
	return f(ctx, ev, sess, view)
}

// TimelineProjection turns one backend event into zero or more timeline entities.
type TimelineProjection interface {
	Project(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]TimelineEntity, error)
}

// TimelineProjectionFunc adapts a function to TimelineProjection.
type TimelineProjectionFunc func(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]TimelineEntity, error)

func (f TimelineProjectionFunc) Project(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]TimelineEntity, error) {
	return f(ctx, ev, sess, view)
}
