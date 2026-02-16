package webchat

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	rediscfg "github.com/go-go-golems/pinocchio/pkg/redisstream"
)

// StreamBackend wraps transport setup concerns (in-memory or redis) and
// exposes publisher/subscriber construction for conversation streams.
type StreamBackend interface {
	EventRouter() *events.EventRouter
	Publisher() message.Publisher
	BuildSubscriber(ctx context.Context, convID string) (message.Subscriber, bool, error)
	Close() error
}

type eventRouterStreamBackend struct {
	router       *events.EventRouter
	redisEnabled bool
	redisAddr    string
}

func NewStreamBackendFromValues(ctx context.Context, parsed *values.Values) (StreamBackend, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	if parsed == nil {
		return nil, errors.New("parsed values are nil")
	}
	rs := rediscfg.Settings{}
	_ = parsed.DecodeSectionInto("redis", &rs)
	router, err := rediscfg.BuildRouter(rs, true)
	if err != nil {
		return nil, errors.Wrap(err, "build event router")
	}
	return &eventRouterStreamBackend{
		router:       router,
		redisEnabled: rs.Enabled,
		redisAddr:    rs.Addr,
	}, nil
}

func (b *eventRouterStreamBackend) EventRouter() *events.EventRouter {
	if b == nil {
		return nil
	}
	return b.router
}

func (b *eventRouterStreamBackend) Publisher() message.Publisher {
	if b == nil || b.router == nil {
		return nil
	}
	return b.router.Publisher
}

func (b *eventRouterStreamBackend) BuildSubscriber(ctx context.Context, convID string) (message.Subscriber, bool, error) {
	if b == nil || b.router == nil {
		return nil, false, errors.New("stream backend is not initialized")
	}
	if convID == "" {
		return nil, false, errors.New("convID is empty")
	}
	if b.redisEnabled {
		if ctx == nil {
			return nil, false, errors.New("ctx is nil")
		}
		_ = rediscfg.EnsureGroupAtTail(ctx, b.redisAddr, topicForConv(convID), "ui")
		sub, err := rediscfg.BuildGroupSubscriber(b.redisAddr, "ui", "ws-forwarder:"+convID)
		if err != nil {
			return nil, false, err
		}
		return sub, true, nil
	}
	return b.router.Subscriber, false, nil
}

func (b *eventRouterStreamBackend) Close() error {
	if b == nil || b.router == nil {
		return nil
	}
	return b.router.Close()
}
