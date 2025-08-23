---
Title: Redis Streaming and Event Routing with Geppetto and Pinocchio
Slug: redis-streaming-event-routing
Short: A practical guide to streaming LLM events over Redis Streams, configuring Watermill routing, and building UIs with per-consumer-group handlers.
Topics:
- geppetto
- pinocchio
- events
- watermill
- redis
- consumer-groups
- ui
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
Date: 2025-08-20
---

## Overview

This guide explains, from first principles, how streaming LLM events flow through Geppetto and Pinocchio, how to publish and consume those events via Redis Streams using Watermill, and how to configure the router so UI handlers receive every event they need. We start with the “why” behind the design: streaming is critical for responsive UIs and fine-grained telemetry; decoupling producers from consumers lets engines focus on inference while UIs, loggers and storage systems can evolve independently. We then introduce the event model, the Watermill abstraction that gives us transport-agnostic pub/sub, and Redis Streams as a concrete backend with consumer groups. Finally, we walk through the new per-handler subscriber API that prevents consumer-group competition so each concern (UI, logs, persistence) can see the full event stream.

## Event Model and Publishing

At the heart of the system is a simple, typed event model. Geppetto engines emit streaming events like “start”, “partial”, and “final” while producing LLM output. Tool-calling helpers emit “tool-call”, “tool-call-execute”, and “tool-result” to reflect structured operations during a turn. This rich stream allows UIs to render partial text in real-time, show tool invocations, and aggregate usage metrics. All event types are defined in `geppetto/pkg/events/chat-events.go` and are serialized to JSON so any subscriber can parse them consistently.

Key constructors and helpers:

- Types: `EventPartialCompletionStart`, `EventPartialCompletion`, `EventFinal`, `EventToolCall`, `EventToolResult`, `EventLog`, `EventInfo`.
- Metadata: `EventMetadata` with `message_id`, `run_id`, `turn_id`, `Usage`, `Extra`.
- Context sinks: `events.WithEventSinks`, `events.PublishEventToContext`.

Publishing path (unchanged):

```go
// Configure engine to publish to Watermill
sink := middleware.NewWatermillSink(router.Publisher, "chat")
eng, _ := factory.NewEngineFromParsedLayers(parsed, engine.WithSink(sink))
```

## Redis Transport and Router Construction

Watermill provides a clean publisher/subscriber abstraction. We keep the publishing path inside engines unchanged and switch transports by changing the router’s `Publisher`/`Subscriber`. Redis Streams offers a great fit for streaming because it provides ordered append-only logs (streams) and consumer groups for horizontal scaling while preserving at-least-once delivery semantics.

To make this easy, we introduce a small package to configure Redis Streams as a Watermill transport:

- `pinocchio/pkg/redisstream/redis_layer.go` defines a `redis` layer and settings:
  - `redis-enabled`, `redis-addr`, `redis-group`, `redis-consumer`.
- `pinocchio/pkg/redisstream/router.go` provides:
  - `BuildRouter(Settings, verbose) (*events.EventRouter, error)`
  - `BuildGroupSubscriber(addr, group, consumer) (message.Subscriber, error)`

When enabled, `BuildRouter` returns an `events.EventRouter` wired to Redis Streams; otherwise, it defaults to in-memory gochannel. Example (high level):

```go
rs := rediscfg.Settings{}
_ = parsedLayers.InitializeStruct("redis", &rs)
router, _ := rediscfg.BuildRouter(rs, false)
sink := middleware.NewWatermillSink(router.Publisher, "chat")
```

## Per-Handler Subscribers (Consumer Groups)

Background: Redis Streams deliver each entry to exactly one consumer within a consumer group. If you attach multiple handlers to the same `Subscriber` (and therefore the same group), the entries get load-balanced among those handlers. For our use case, this is undesirable: the timeline UI must see every `start`, `partial`, and `final` to create and complete UI entities, while loggers and persistence should also see the full stream independently. If they are all in the same group, one may consume an event that another depends on.

Problem: A single subscriber means multiple handlers on the same topic compete in one consumer group. With Redis Streams, events are load-balanced across consumers, so the UI may miss `start` events if a logger consumes them first and the UI only sees later `partial` events.

Solution (Design A): Extend the router with per-handler subscriber options so different concerns can consume the full stream independently. Implemented in `geppetto/pkg/events/event-router.go`:

- `AddHandlerWithOptions(name, topic string, f func(*message.Message) error, opts ...HandlerOption)`
- `WithHandlerSubscriber(sub message.Subscriber)`
- `AddHandler(...)` remains and delegates to `AddHandlerWithOptions` for backward compatibility.

Example (UI vs. logs):

```go
// UI on group "ui"
uiSub, _ := rediscfg.BuildGroupSubscriber(rs.Addr, "ui", "ui-1")
router.AddHandlerWithOptions("ui-forward", "chat", backend.MakeUIForwarder(p),
    events.WithHandlerSubscriber(uiSub),
)

// Logs/persistence on group "logs"
logsSub, _ := rediscfg.BuildGroupSubscriber(rs.Addr, "logs", "logs-1")
router.AddHandlerWithOptions("tool-logger", "chat", logHandler,
    events.WithHandlerSubscriber(logsSub),
)
```

Recommended groups:
- UI: group `ui` (consumer per process, e.g., `ui-1`)
- Logs/persistence: group `logs` (consumers `logs-1`, `persist-1`)

Practical tips:
- Give each process a unique consumer name (e.g., `ui-2`, `logs-3`) so Redis can track pending entries and allow robust recovery.
- Keep UI on its own group so it never competes with background consumers.
- If you add a new concern (e.g., metrics aggregation), prefer a new group so it sees all events.

## Where Handlers Are Registered (Current Code)

- Agent (`pinocchio/cmd/agents/simple-chat-agent/main.go`):
  - `ui-forward` (UI timeline entity creation)
  - `tool-logger` (logs tool events)
  - `event-sql-logger` (persists events to SQLite)
- CLI examples (printers):
  - `geppetto/cmd/examples/simple-streaming-inference/main.go`: `StepPrinterFunc`
  - `pinocchio/cmd/examples/simple-chat/main.go`: `StepPrinterFunc`
- Redis example (debug):
  - `pinocchio/cmd/examples/simple-redis-streaming-inference/main.go`: `debug-raw`, `debug-events`

In Redis-enabled runs, the agent now assigns UI, logging, and persistence to separate consumers/groups to avoid competition.

How this maps to UX: the UI always receives the `start` event first, creates a timeline entity, applies `partial` deltas to keep the view responsive, then marks the entity complete on `final`. The logger and persistence layers concurrently receive the same events in their groups without interfering with the UI’s ordering.

## End-to-End Flow

1) Router and sink:
```go
router, _ := rediscfg.BuildRouter(rs, false)  // Redis-backed if enabled
sink := middleware.NewWatermillSink(router.Publisher, "chat")
```
2) Engine and middleware emit events to `chat` topic.
3) Handlers:
```go
// UI
uiSub, _ := rediscfg.BuildGroupSubscriber(rs.Addr, "ui", "ui-1")
router.AddHandlerWithOptions("ui-forward", "chat", backend.MakeUIForwarder(p), events.WithHandlerSubscriber(uiSub))

// Logging
logsSub, _ := rediscfg.BuildGroupSubscriber(rs.Addr, "logs", "logs-1")
router.AddHandlerWithOptions("tool-logger", "chat", logHandler, events.WithHandlerSubscriber(logsSub))
```
4) Run router and UI in parallel; wait for readiness with `<-router.Running()>`.

End-to-end, this approach separates concerns clearly:
- The engine focuses solely on generating events.
- The router handles transport, allowing you to swap in-memory vs. Redis without code changes in the engine.
- Handlers render UIs, write logs, and persist data independently, thanks to consumer-group isolation.

## CLI Flags and Layers

Enable Redis with the `redis` layer flags:

- `--redis-enabled=true`
- `--redis-addr=localhost:6379`
- `--redis-group=chat-ui` (default consumer group used for default subscriber)
- `--redis-consumer=ui-1`

Note: UI/log/persist use `BuildGroupSubscriber` and do not rely on the default group when isolating concerns.

For non-Redis development, leave `--redis-enabled=false` and the router will use an in-memory bus; all handlers will receive every event within the same process.

## Inspecting Streams with redis-cli

- Live tail:
```bash
redis-cli --raw XREAD BLOCK 0 STREAMS chat $
```
- Show recent entries:
```bash
redis-cli --raw XRANGE chat - + COUNT 10
```
- Stream info and groups:
```bash
redis-cli XINFO STREAM chat
redis-cli XINFO GROUPS chat
```

Reading output:
- For our events, the JSON payload contains fields like `type`, `id`, `delta`, and `text`. You can pretty-print JSON externally (e.g., `| jq .`) if needed.
- If a consumer group seems stuck, inspect `XINFO GROUPS chat` to see pending counts and last delivered IDs.

## Troubleshooting

- UI misses `start` event:
  - Ensure UI uses a dedicated subscriber on group `ui` via `AddHandlerWithOptions(..., WithHandlerSubscriber(uiSub))`.
- No events in Redis:
  - Verify `--redis-enabled`, address, and the example is running concurrently while tailing.
- Multiple processes competing in one group:
  - Assign unique consumer names per process (e.g., `ui-2`, `logs-2`).

Additional checks:
- Engine not publishing? Confirm `engine.WithSink(watermillSink)` is set when you create the engine (or sinks attached via context for helpers).
- Handlers not attached? Ensure `router.AddHandler...` calls happen before `router.Run` and after building any per-group subscribers.
- Ordering concerns? Redis Streams preserve stream order, but separate groups each track their own offsets. This is expected and safe.

Operational guidance:
- Use short consumer names and meaningful group names (e.g., `ui`, `logs`).
- For durability, consider creating the consumer groups up front with `XGROUP CREATE chat ui $ MKSTREAM` to start reading near real-time, or `0` to read from the beginning.

## References

- Event types: `geppetto/pkg/events/chat-events.go`
- Router options: `geppetto/pkg/events/event-router.go`
- Redis transport: `pinocchio/pkg/redisstream/`
- Agent wiring: `pinocchio/cmd/agents/simple-chat-agent/main.go`
- Example (Redis): `pinocchio/cmd/examples/simple-redis-streaming-inference/main.go`

---

## Appendix: Frequently Asked Questions

### Why not use a single handler and fan out in-process?
You can. It’s simpler but ties all concerns to one process. Using consumer groups enables multiple processes (e.g., a web UI service and a background logger) to receive the same events independently.

### Are events delivered exactly once?
Redis Streams provide at-least-once delivery per group. Handlers should be idempotent where necessary. For UI rendering, repeating a `partial` update that sets the latest completion string is naturally idempotent.

### How do I add a new consumer (e.g., metrics)?
Create a new subscriber via `BuildGroupSubscriber(addr, "metrics", "metrics-1")` and register a handler with `AddHandlerWithOptions(..., WithHandlerSubscriber(metricsSub))`.

### Can I switch transports?
Yes. The engine code is transport-agnostic. Swap the router’s publisher/subscriber (e.g., NATS, Kafka) and keep the same event model and handlers.


