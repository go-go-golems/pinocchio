---
Title: Router/Subscriber Redesign for Consumer Groups (Avoiding Handler Competition)
Slug: router-consumer-group-redesign
Short: Problem statement and API design proposals to allow distinct consumer groups per handler/concern in Geppetto’s EventRouter, without changing engine publishing via Watermill sink.
Topics:
- geppetto
- events
- watermill
- redis
- router
- consumer-groups
Date: 2025-08-20
SectionType: Design
---

## Overview

Today a single `events.EventRouter` wraps one Watermill `Publisher` and one `Subscriber`. Every `AddHandler(name, topic, f)` attaches a handler on the same `Subscriber`. For Redis Streams (consumer groups), this means each handler becomes a separate consumer in the SAME group, so entries are load-balanced across handlers. When multiple handlers are registered for the same topic (e.g., UI forwarder + logger), the `start` event that creates the timeline entity can be consumed by the logger instead of the UI, causing the UI to receive only later `partial` events.

We need to preserve the engine publishing path (Watermill sink + publisher) and update the router to support distinct consumer groups or dedicated subscribers per handler/concern.

## Current API (unchanged parts)

- Publishing (keep as-is):
  - `middleware.NewWatermillSink(router.Publisher, "chat")`
  - Engines publish `start`/`partial`/`final`/… to `router.Publisher`.
- Event model (unchanged): `geppetto/pkg/events/chat-events.go`

## Problem

- One shared `Subscriber` inside `EventRouter` → multiple `AddHandler` calls attach multiple consumers to the SAME consumer group.
- With Redis Streams, entries are delivered to only one consumer per group. Different handlers see disjoint subsets.
- UI forwarder sometimes misses `start`, so the timeline entity isn’t created before partials arrive.

## Goals

1) Allow separate consumer groups (or separate subscribers) per handler/concern.
2) Keep engine publishing unchanged.
3) Maintain simple default behavior for in-memory dev (gochannel remains default).

## Design Proposals

### Design A: Group-aware Router Handlers

Extend router to support group-scoped handlers by introducing a HandlerOption that carries a dedicated `message.Subscriber` (or a group name resolved by `EventRouter`).

API sketch:

```go
// New option types
type HandlerOption func(*handlerConfig)
func WithHandlerSubscriber(sub message.Subscriber) HandlerOption
func WithHandlerConsumerGroup(group string) HandlerOption
func WithHandlerConsumerName(consumer string) HandlerOption

// New method
func (e *EventRouter) AddHandlerWithOptions(name, topic string, f func(*message.Message) error, opts ...HandlerOption)

// Backward-compatible shim
func (e *EventRouter) AddHandler(name, topic string, f func(*message.Message) error) {
    e.AddHandlerWithOptions(name, topic, f)
}
```

Behavior:
- If a `HandlerOption` supplies a dedicated subscriber (configured to a specific consumer group), the handler uses it. Otherwise, the router falls back to its default `Subscriber`.
- For Redis Streams, we can pass a subscriber configured with `ConsumerGroup: "ui"` for the UI forwarder, while logging/persist can use `ConsumerGroup: "logs"`.

Pros:
- Minimal disruption; handlers remain simple.
- Clear typing; explicit per-handler subscriber/group.

Cons:
- Slightly more plumbing at call sites to construct per-group subscribers when desired.

### Design B: Multi-Subscriber Router Registry

Allow the router to manage multiple named subscribers internally and route handlers to a named subscriber.

API sketch:

```go
func (e *EventRouter) RegisterSubscriber(name string, sub message.Subscriber)
func (e *EventRouter) AddHandlerOn(name string, subscriberName string, topic string, f func(*message.Message) error)
```

Usage:
- `router.RegisterSubscriber("ui", uiSub)` and `router.RegisterSubscriber("logs", logsSub)`.
- `router.AddHandlerOn("ui-forward", "ui", "chat", uiHandler)`.

Pros:
- Centralized management; call sites don’t hold raw subscribers.

Cons:
- Slightly heavier router API; still need to create subscribers externally.

### Design C: Single Handler Fan-out (In-Process Demux)

Register exactly one Watermill handler per topic that consumes ALL entries; inside that handler, demultiplex to in-process listeners (UI, logger, persistence) via channels.

API impact:
- No router API change. Introduce a small demux component that dispatches to local consumers based on type.

Pros:
- Simple, preserves exactly-once per topic.
- Avoids consumer-group complexities.

Cons:
- All concerns must live in the same process.
- Failure isolation is weaker; a slow sink can affect others unless carefully buffered.

### Design D: RouterFactory Per Concern

Provide a `RouterFactory` util that creates small `EventRouter` instances per concern with dedicated subscriber configs. The publishing `Publisher` may be shared (or separate), but subscribers differ.

Sketch:

```go
type RouterFactory interface {
    NewRouterWithGroup(group string, consumer string) (*events.EventRouter, error)
}
```

Pros:
- Very explicit; isolation by design.

Cons:
- More router instances and goroutines; slightly higher overhead.

## Recommendation

Adopt Design A (Handler options) as the primary API. It’s explicit, minimally invasive, and lets callers opt into per-handler consumer groups while retaining existing usage.

Implementation steps:
1) Add `AddHandlerWithOptions` and the `HandlerOption` types to `geppetto/pkg/events/event-router.go`.
2) For Redis Streams, expose a helper to construct a group-specific Watermill subscriber (either in a separate package like `pinocchio/pkg/redisstream` or a small helper in Geppetto for common backends).
3) Migrate critical handlers (timeline UI forwarder) to a dedicated consumer group (e.g., `ui`). Keep loggers/persistence on another group (`logs`) or use Design C inside-process fan-out for those.
4) Add safety checks: when the default `Subscriber` is a Redis Streams subscriber and multiple handlers for the same topic are added without options, log a warning about potential load-balancing across handlers.

## Backwards Compatibility

- `AddHandler` continues to work as-is (uses the router’s default `Subscriber`).
- In-memory default (gochannel) behavior unchanged.
- Engine publishing via `WatermillSink` unchanged.

## Example (UI with dedicated group)

```go
// Construct group-specific subscriber (UI)
uiSub := redisstream.NewSubscriber(redisstream.SubscriberConfig{
    Client:        client,
    Unmarshaller:  marshaler,
    ConsumerGroup: "ui",
    Consumer:      "ui-1",
}, logger)

// Register UI handler on its own subscriber
router.AddHandlerWithOptions("ui-forward", "chat", backend.MakeUIForwarder(p),
    events.WithHandlerSubscriber(uiSub),
)

// Optional: logs on a separate group
logsSub := redisstream.NewSubscriber(redisstream.SubscriberConfig{ /* group: "logs" */ }, logger)
router.AddHandlerWithOptions("logs", "chat", logHandler,
    events.WithHandlerSubscriber(logsSub),
)
```

## Testing Plan

- Unit-test: ensure handlers on distinct subscribers each receive the full stream (no load-balancing across them).
- Integration-test with Redis: register UI handler on `ui` group and a logger on `logs` group; verify both receive `start`/`partial`/`final` for the same message IDs.
- Regression-test in in-memory mode: multiple handlers must all see the same events.

## Notes

- If we later support other backends (NATS/Kafka), the same handler-level subscriber approach applies.
- We can provide convenience constructors for common backends/groups to reduce boilerplate at call sites.


