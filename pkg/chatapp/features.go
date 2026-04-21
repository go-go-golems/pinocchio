package chatapp

import (
	"context"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	sessionstream "github.com/go-go-golems/sessionstream"
)

type FeatureSet interface {
	RegisterSchemas(reg *sessionstream.SchemaRegistry) error
	HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (handled bool, err error)
	ProjectUI(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error)
	ProjectTimeline(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error)
}

type RuntimeEventContext struct {
	SessionID sessionstream.SessionId
	MessageID string
	Publish   func(ctx context.Context, eventName string, payload map[string]any) error
}

func WithFeatureSets(features ...FeatureSet) Option {
	return func(e *Engine) {
		for _, feature := range features {
			if feature != nil {
				e.features = append(e.features, feature)
			}
		}
	}
}

func (e *Engine) activeFeatures() []FeatureSet {
	if e == nil || len(e.features) == 0 {
		return nil
	}
	out := make([]FeatureSet, 0, len(e.features))
	for _, feature := range e.features {
		if feature != nil {
			out = append(out, feature)
		}
	}
	return out
}

func (e *Engine) handleFeatureRuntimeEvent(ctx context.Context, sid sessionstream.SessionId, messageID string, pub sessionstream.EventPublisher, event gepevents.Event) error {
	for _, feature := range e.activeFeatures() {
		handled, err := feature.HandleRuntimeEvent(ctx, RuntimeEventContext{
			SessionID: sid,
			MessageID: messageID,
			Publish: func(ctx context.Context, eventName string, payload map[string]any) error {
				return e.publish(ctx, sid, pub, eventName, payload)
			},
		}, event)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	return nil
}

func (e *Engine) uiProjection(ctx context.Context, ev sessionstream.Event, sess *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, error) {
	base, err := baseUIProjection(ctx, ev, sess, view)
	if err != nil {
		return nil, err
	}
	if base != nil {
		return base, nil
	}
	for _, feature := range e.activeFeatures() {
		projected, handled, err := feature.ProjectUI(ctx, ev, sess, view)
		if err != nil {
			return nil, err
		}
		if handled {
			return projected, nil
		}
	}
	return nil, nil
}

func (e *Engine) timelineProjection(ctx context.Context, ev sessionstream.Event, sess *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, error) {
	base, err := baseTimelineProjection(ctx, ev, sess, view)
	if err != nil {
		return nil, err
	}
	if base != nil {
		return base, nil
	}
	for _, feature := range e.activeFeatures() {
		projected, handled, err := feature.ProjectTimeline(ctx, ev, sess, view)
		if err != nil {
			return nil, err
		}
		if handled {
			return projected, nil
		}
	}
	return nil, nil
}
