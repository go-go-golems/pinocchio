package evtstream

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
	"google.golang.org/protobuf/proto"
)

type eventConsumer struct {
	hub      *Hub
	ordinals *OrdinalAssigner
	ready    chan error
	done     chan error
}

func newEventConsumer(h *Hub) *eventConsumer {
	return &eventConsumer{
		hub: h,
		ordinals: NewOrdinalAssigner(func(ctx context.Context, sid SessionId) (uint64, error) {
			return h.store.Cursor(ctx, sid)
		}),
		ready: make(chan error, 1),
		done:  make(chan error, 1),
	}
}

func (c *eventConsumer) start(ctx context.Context) error {
	go c.consume(ctx)
	return <-c.ready
}

func (c *eventConsumer) wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case err := <-c.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *eventConsumer) consume(ctx context.Context) {
	if c.hub == nil || c.hub.bus == nil {
		c.ready <- fmt.Errorf("event bus is not configured")
		c.done <- fmt.Errorf("event bus is not configured")
		return
	}
	ch, err := c.hub.bus.subscriber.Subscribe(ctx, c.hub.bus.topic)
	if err != nil {
		c.ready <- err
		c.done <- err
		return
	}
	c.ready <- nil

	for {
		select {
		case <-ctx.Done():
			c.done <- nil
			return
		case msg, ok := <-ch:
			if !ok {
				c.done <- nil
				return
			}
			if err := c.handleMessage(ctx, msg); err != nil {
				msg.Nack()
				c.done <- err
				return
			}
			msg.Ack()
		}
	}
}

func (c *eventConsumer) handleMessage(ctx context.Context, msg *message.Message) error {
	ev, err := decodeEventEnvelope(c.hub.reg, msg.Payload)
	if err != nil {
		return nil
	}
	ord, err := c.ordinals.Next(ctx, ev.SessionId, msg.Metadata)
	if err != nil {
		return err
	}
	ev.Ordinal = ord
	if c.hub.bus != nil && c.hub.bus.observer != nil {
		c.hub.bus.observer.Consumed(ctx, Event{
			Name:      ev.Name,
			Payload:   proto.Clone(ev.Payload),
			SessionId: ev.SessionId,
			Ordinal:   ev.Ordinal,
		}, newBusRecord(msg, c.hub.bus.topic))
	}
	_, err = c.hub.projectAndApply(ctx, ev)
	return err
}
