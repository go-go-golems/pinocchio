---
Title: Designing a Redis-backed Streaming Chat Web Agent
Slug: redis-chat-web-agent-design
Short: Architecture survey and design to stream Geppetto LLM events via Redis, consume them in Pinocchio/Bobatea, and add a WebSocket web UI.
Topics:
- geppetto
- events
- watermill
- redis
- pinocchio
- bobatea
- websocket
- web-ui
SectionType: Design
Date: 2025-08-20
---

## Overview

This document surveys the relevant architecture in the repository and proposes a concrete design to:

- Stream Geppetto LLM inference events over Redis.
- Consume those events and display them with the existing Pinocchio/Bobatea UI patterns (see `pinocchio/cmd/agents/simple-chat-agent`).
- Add a simple web UI that uses WebSockets to stream the same events to the browser, following the established event/timeline patterns.

The current system uses an engine-first design where inference engines emit typed events during streaming. These events are published through `events.EventSink` implementations, most commonly a Watermill-backed sink, and routed by an `events.EventRouter`. Tool-calling orchestration is separate from engines and can also emit events. The TUI uses Bobatea’s timeline controller to render entities (assistant text, tool calls, log/info messages) in real time. Our proposal keeps these patterns and swaps the in-memory bus with a Redis-backed Watermill transport, plus a small HTTP/WebSocket layer for the web UI.

### Key building blocks (files, packages, symbols)

- Geppetto events and context sinks
  - Package: `github.com/go-go-golems/geppetto/pkg/events`
  - Files: `geppetto/pkg/events/chat-events.go`, `geppetto/pkg/events/context.go`
  - Symbols: `Event`, `EventType*` constants, `NewEventFromJson`, `EventMetadata`, `PublishEventToContext`, `WithEventSinks`
  
- Watermill integration and routing
  - Packages: `github.com/go-go-golems/geppetto/pkg/events`, `github.com/go-go-golems/geppetto/pkg/inference/middleware`
  - Files: `geppetto/pkg/inference/middleware/sink_watermill.go`, `geppetto/pkg/events/event-router.go`
  - Symbols: `middleware.WatermillSink` (publisher), `events.EventRouter`, `AddHandler`, `Run`, `WithPublisher`, `WithSubscriber`

- Engines and tool orchestration
  - Packages: `github.com/go-go-golems/geppetto/pkg/inference/engine`, `.../factory`, `.../toolhelpers`, `.../tools`
  - Files (examples): `geppetto/cmd/examples/simple-streaming-inference/main.go`
  - Symbols: `factory.NewEngineFromParsedLayers`, `engine.WithSink`, `toolhelpers.RunToolCallingLoop`

- Pinocchio simple chat agent (TUI) wiring and UI forwarding
  - Package: `pinocchio/cmd/agents/simple-chat-agent`
  - Files: `pinocchio/cmd/agents/simple-chat-agent/main.go`, `.../pkg/backend/tool_loop_backend.go`, `.../pkg/xevents/events.go`
  - Symbols: `eventspkg.AddUIForwarder`, `backend.ToolLoopBackend`, `MakeUIForwarder`, Bubble Tea model setup with timeline renderers

- Bobatea chat/timeline UI
  - Packages: `github.com/go-go-golems/bobatea/pkg/chat`, `.../pkg/timeline`, `.../pkg/timeline/renderers`
  - Symbols (usage in agent): `timeline.Registry`, `RegisterModelFactory`, `renderers.NewLLMTextFactory`, `renderers.NewToolCallFactory`, etc.

The following sections dive deeper into each topic with references and sketches, then present feature-specific designs.

---

## Topics

### Topic: Geppetto Chat Events and Context-Carried Sinks

---
Title: Geppetto Chat Events and Context-Carried Sinks
Topics:
- geppetto
- events
Files:
- geppetto/pkg/events/chat-events.go
- geppetto/pkg/events/context.go
Packages:
- github.com/go-go-golems/geppetto/pkg/events
Symbols:
- Event, EventType, EventMetadata, NewEventFromJson
- WithEventSinks, PublishEventToContext, GetEventSinks
---

Geppetto defines a typed event model for streaming LLM inference. The core types live in `geppetto/pkg/events/chat-events.go` and include start, partial, final, tool-call, tool-result, error, interrupt, and auxiliary log/info events. Each event carries an `EventMetadata` that supplies correlation IDs (`run_id`, `turn_id`), a stable `message_id`, model information, and typed usage.

Event sinks are abstracted via the `events.EventSink` interface. Downstream publishers (engines, helpers, tools) can emit events in two ways: by configuring engine-level sinks (e.g., `engine.WithSink`) or by using context-attached sinks with `events.WithEventSinks(ctx, sinks...)` and `events.PublishEventToContext(ctx, e)`. The latter is commonly used by tool helpers to avoid plumbing sinks through call stacks.

Illustrative snippet (types and helpers):

```go
// geppetto/pkg/events/chat-events.go
type Event interface { Type() EventType; Metadata() EventMetadata; Payload() []byte }
const (
  EventTypeStart EventType = "start"
  EventTypePartialCompletion EventType = "partial"
  EventTypeFinal EventType = "final"
  EventTypeToolCall EventType = "tool-call"
  EventTypeToolResult EventType = "tool-result"
  EventTypeError EventType = "error"
  EventTypeInterrupt EventType = "interrupt"
  EventTypeLog EventType = "log"
  EventTypeInfo EventType = "info"
)

// geppetto/pkg/events/context.go
func WithEventSinks(ctx context.Context, sinks ...EventSink) context.Context
func PublishEventToContext(ctx context.Context, event Event)
```

These are the same typed events we will serialize over Redis for web consumption and the Bobatea UI.

### Topic: Watermill Sink and Event Router

---
Title: Watermill Sink and Event Router
Topics:
- watermill
- pubsub
Files:
- geppetto/pkg/inference/middleware/sink_watermill.go
- geppetto/pkg/events/event-router.go
Packages:
- github.com/go-go-golems/geppetto/pkg/inference/middleware
- github.com/go-go-golems/geppetto/pkg/events
Symbols:
- middleware.WatermillSink
- events.EventRouter, WithPublisher, WithSubscriber, AddHandler, Run
---

`middleware.WatermillSink` serializes events to JSON and publishes them to a Watermill `message.Publisher` on a topic (commonly `"chat"`). The `events.EventRouter` wraps a Watermill router, defaulting to an in-memory `gochannel` publisher/subscriber, and provides `AddHandler(name, topic, func(*message.Message) error)` to attach consumers. Handlers typically parse messages with `events.NewEventFromJson` and then render, forward, or persist.

To adopt Redis, we’ll replace the default in-memory pub/sub with Watermill’s Redis Publisher/Subscriber via `WithPublisher` and `WithSubscriber` options on `EventRouter` (details in the feature section below).

Example (current pattern):

```go
// Create router and sink
router, _ := events.NewEventRouter()
sink := middleware.NewWatermillSink(router.Publisher, "chat")

// Add a handler that parses typed events
router.AddHandler("pretty", "chat", events.StepPrinterFunc("", os.Stdout))

// Engine with sink (engines publish start/partial/final/...)
eng, _ := factory.NewEngineFromParsedLayers(parsed, engine.WithSink(sink))
```

### Topic: Tool Orchestration with Helpers

---
Title: Tool Orchestration with Helpers
Topics:
- tools
- orchestration
Files:
- geppetto/pkg/inference/toolhelpers/helpers.go (and related)
- pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go
Packages:
- github.com/go-go-golems/geppetto/pkg/inference/toolhelpers
- github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent/pkg/backend
Symbols:
- toolhelpers.RunToolCallingLoop
- backend.ToolLoopBackend
---

Tool calling is not embedded in engines. Instead, `toolhelpers.RunToolCallingLoop` detects tool calls in model outputs, invokes registered tools, appends results, and may iterate until resolution. It publishes its own execution-phase events (`tool-call-execute`, `tool-call-execution-result`) via context sinks.

The agent wraps this in `backend.ToolLoopBackend`, which attaches the sink to the context and fronts a Bubble Tea chat model. This backend is a good reuse point for both TUI and web workflows.

```go
// pinocchio/.../tool_loop_backend.go
runCtx := events.WithEventSinks(ctx, b.sink)
updated, err := toolhelpers.RunToolCallingLoop(runCtx, b.eng, b.turn, b.reg, ...)
```

### Topic: Pinocchio Simple Chat Agent Wiring

---
Title: Pinocchio Simple Chat Agent Wiring
Topics:
- pinocchio
- tui
Files:
- pinocchio/cmd/agents/simple-chat-agent/main.go
- pinocchio/cmd/agents/simple-chat-agent/pkg/xevents/events.go
Packages:
- github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent
Symbols:
- eventspkg.AddUIForwarder
- backend.ToolLoopBackend.MakeUIForwarder
---

The `simple-chat-agent` command wires an `events.EventRouter`, a `WatermillSink`, an engine, and a Bubble Tea chat UI. It attaches handlers that log tool events, persist events to SQLite, and forward typed events to the UI.

```go
// pinocchio/.../main.go
router, _ := events.NewEventRouter()
sink := middleware.NewWatermillSink(router.Publisher, "chat")
backend := backendpkg.NewToolLoopBackend(eng, registry, sink, hook)
router.AddHandler("ui-forward", "chat", backend.MakeUIForwarder(p))
```

For the web UI, we’ll reuse this forwarding concept but target WebSocket connections instead of a Bubble Tea program, and we’ll subscribe via Redis.

### Topic: Bobatea Timeline Chat UI

---
Title: Bobatea Timeline Chat UI
Topics:
- bobatea
- timeline
Files:
- usage in pinocchio/cmd/agents/simple-chat-agent/main.go
Packages:
- github.com/go-go-golems/bobatea/pkg/chat
- github.com/go-go-golems/bobatea/pkg/timeline
- github.com/go-go-golems/bobatea/pkg/timeline/renderers
Symbols:
- timeline.Registry, RegisterModelFactory
- renderers.NewLLMTextFactory, NewToolCallFactory, ToolCallResultFactory
---

Bobatea provides a timeline-first UI. The agent registers model factories that know how to render and update entities as events arrive. The event forwarder translates typed events into timeline lifecycle messages: create/update/complete for assistant text blocks; create/complete for tool calls and results; and also lightweight log/info entities. This structure is equally suitable for a web UI—our web server will stream the same event representations to the browser for rendering on the client.

---

## Feature Designs

### 1) Stream Geppetto LLM Inference Events over Redis

We will plug a Redis-backed Watermill Publisher/Subscriber into the existing `events.EventRouter` so engines and helpers can publish/consume events across processes.

- Reuse
  - `middleware.WatermillSink` for publishing typed events.
  - `events.EventRouter` for adding handlers; switch its transport from in-memory to Redis.
  - Existing typed event model in `geppetto/pkg/events/chat-events.go`.

- Add
  - Dependency: Watermill Redis (e.g., `github.com/ThreeDotsLabs/watermill-redisstream` for Redis Streams, or `watermill-redis` depending on choice).
  - A small factory to create a Redis Publisher/Subscriber from settings (host, DB, stream name, consumer group).

- Changes
  - Construct router with explicit publisher/subscriber:

```go
// sketch: configure EventRouter with Redis transport
pub := redisstream.NewPublisher(redisstream.PublisherConfig{ /* addr, marshaler, stream */ }, logger)
sub := redisstream.NewSubscriber(redisstream.SubscriberConfig{ /* addr, group, consumer */ }, logger)
router, _ := events.NewEventRouter(
  events.WithPublisher(pub),
  events.WithSubscriber(sub),
)

// sink publishes to Redis-backed topic
sink := middleware.NewWatermillSink(router.Publisher, "chat")
```

Alternative: keep the `EventRouter` for handlers only and directly pass the Redis publisher to `NewWatermillSink` without creating a router for the producer process.

Configuration notes:

- Use `RunID`/`TurnID` in `EventMetadata` to filter on the consumer side. We’ll keep a single topic (`"chat"`) and apply filtering in handlers.
- For dev, continue using `gochannel` in-memory by default; enable Redis via flags or profiles.

### 2) Receive Events over Redis and Display in Pinocchio/Bobatea

We will run a consumer process that attaches handlers to the Redis-backed `EventRouter` and forwards events to the UI.

- Reuse
  - `backend.ToolLoopBackend.MakeUIForwarder` to translate events to Bobatea timeline messages for TUI.
  - `eventspkg.AddUIForwarder` utility for channel-based forwarding if needed.
  - `events.NewEventFromJson` for parsing.

- Add
  - CLI flags (or layered profiles) to select Redis subscriber configuration when launching the UI-only client.

- Changes
  - The UI-only client (no local engine) should not configure `engine.WithSink`. It simply subscribes to `"chat"` over Redis and renders.

Sketch (UI client):

```go
// setup router with Redis subscriber
router, _ := events.NewEventRouter(events.WithSubscriber(redisSub), events.WithPublisher(redisPub))
// forward to Bubble Tea program
router.AddHandler("ui-forward", "chat", backend.MakeUIForwarder(p))
go router.Run(ctx)
```

Alternative: For web-only consumption (no Bubble Tea), attach a handler that writes to connected WebSocket clients (see next section).

### 3) Add a Simple Web UI with WebSocket Streaming

We will build a small HTTP server that serves a static chat page (plain HTML + vanilla JavaScript, no frameworks) and a `/ws` endpoint streaming typed events as JSON. The server subscribes to the same `"chat"` topic via Redis, filters by `run_id` (and optionally `turn_id`), and forwards events to the WebSocket.

- Reuse
  - Typed events and `NewEventFromJson` for consistent payloads.
  - The event-to-UI mapping already implemented in `backend.ToolLoopBackend.MakeUIForwarder`; we can mirror it for the web by sending minimal JSON patches to the client representing timeline lifecycle events.

- Add
  - Package `pinocchio/pkg/ui/web` (server) or a new command under `pinocchio/cmd/web-agent`.
  - Static HTML + vanilla JS assets for the chat page; client applies streaming updates.
  - WebSocket endpoint using `net/http` and an upgrader (gorilla/websocket is fine) that:
    - Creates a per-connection Watermill subscription to `"chat"` via Redis.
    - Filters messages by `EventMetadata.RunID` and forwards as JSON.
  - A small on-the-wire schema for timeline lifecycle messages (created/updated/completed), mirroring Bobatea’s concepts.

- Changes
  - None to engine/tooling: the engine continues to publish to Redis through `WatermillSink`.

Server sketch:

```go
// pseudo-code
type WSHandler struct { sub message.Subscriber }

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  conn := upgrader.Upgrade(w, r, nil)
  ctx := r.Context()
  ch, _ := h.sub.Subscribe(ctx, "chat")
  runID := r.URL.Query().Get("run_id")

  go func() {
    defer conn.Close()
    for msg := range ch {
      e, err := events.NewEventFromJson(msg.Payload)
      if err != nil { msg.Ack(); continue }
      if runID != "" && e.Metadata().RunID != runID { msg.Ack(); continue }
      _ = conn.WriteJSON(mapEventToTimelineDelta(e))
      msg.Ack()
    }
  }()
}
```

Client sketch (plain HTML + JS):

```html
<div id="timeline"></div>
<script>
  const ws = new WebSocket(`wss://${location.host}/ws?run_id=${encodeURIComponent(runId)}`)
  ws.onmessage = (ev) => {
    const delta = JSON.parse(ev.data)
    applyTimelineDelta(document.getElementById('timeline'), delta)
  }
  ws.onclose = () => {/* show disconnected */}
  ws.onerror = () => {/* show error */}
</script>
```

Alternative: SSE instead of WebSocket (simpler backpressure and infra), but the request explicitly prefers WebSockets.

---

## Integration Considerations

- Topics and filtering
  - Keep a single `"chat"` topic and filter by `EventMetadata.RunID`/`TurnID` in consumers. This mirrors the existing forwarders and avoids topic proliferation.

- Avoid duplicate publishing
  - Either configure `engine.WithSink(sink)` or attach sinks via context (`WithEventSinks`), not both, to avoid duplicates (see `simple-chat-agent.md`).

- Lifecycle and cleanup
  - For web sockets, use `context.Context` to cancel subscriptions on disconnect.
  - Use `errgroup` when running router + server concurrently, following patterns in `pinocchio/cmd/agents/simple-chat-agent/main.go`.

- Error handling and logging
  - Continue using `github.com/pkg/errors` for wrapping.
  - Maintain zerolog event logs; optionally forward `EventLog` to the web client for visibility.

---

## Alternatives and Trade-offs

- Transport choice
  - Redis Streams via Watermill: straightforward, durable, consumer groups; requires Redis infra.
  - In-memory `gochannel`: best for local dev, not cross-process.
  - Other Watermill backends (NATS, Kafka): swap without touching event model.

- Browser streaming protocol
  - WebSocket: bi-directional, fits future interactive tool UIs.
  - SSE: uni-directional, simpler; adequate for output-only streaming.

---

## Minimal End-to-End Flow (Putting It Together)

1) Producer (engine/tool loop) process:

```go
router, _ := events.NewEventRouter(events.WithPublisher(redisPub))
sink := middleware.NewWatermillSink(router.Publisher, "chat")
eng, _ := factory.NewEngineFromParsedLayers(parsed, engine.WithSink(sink))
// run inference/tool loop; engines and helpers publish to Redis
```

2) TUI consumer process:

```go
router, _ := events.NewEventRouter(events.WithSubscriber(redisSub))
router.AddHandler("ui-forward", "chat", backend.MakeUIForwarder(p))
_ = router.Run(ctx)
```

3) Web consumer process:

```go
// HTTP server exposes /ws; handler subscribes to redisSub.Subscribe(ctx, "chat")
// and forwards filtered events to the socket.
```

This preserves the engine-first/event-sink architecture, enables cross-process streaming via Redis, and adds a web presentation layer without perturbing the core inference and tool-calling logic.


