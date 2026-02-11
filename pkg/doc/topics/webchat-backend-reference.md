---
Title: Webchat Backend Reference
Slug: webchat-backend-reference
Short: API reference for StreamCoordinator, ConnectionPool, and core backend webchat components.
Topics:
- webchat
- streaming
- websocket
- backend
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This document provides API reference for `StreamCoordinator` and `ConnectionPool`, the core backend components that bridge event streams to WebSocket connections. For implementation details and troubleshooting, see [Backend Internals](webchat-backend-internals.md).

## StreamCoordinator

### Purpose

`StreamCoordinator` bridges event topics and WebSocket connections. It owns a subscriber, converts inbound `events.Event` payloads into SEM frames, stamps them with a `StreamCursor` (stream ID + sequence), and fans out frames through callbacks so conversations can log emissions and broadcast bytes to every `ConnectionPool` member. The sequence is derived from Redis stream IDs when available; otherwise it falls back to a time-based monotonic sequence.

The coordinator never writes directly to WebSockets—it relies entirely on callbacks, keeping the component reusable and testable.

### Constructor

```go
NewStreamCoordinator(
    convID string,
    subscriber message.Subscriber,
    onEvent func(events.Event, StreamCursor),
    onFrame func(events.Event, StreamCursor, []byte),
) *StreamCoordinator
```

Creates a coordinator for the given conversation ID, wiring the subscriber and callbacks. `onEvent` is optional for custom bookkeeping; `onFrame` handles logging, WebSocket broadcast, and timeline projection from SEM frames.

### Methods

| Method | Description |
|--------|-------------|
| `Start(ctx context.Context) error` | Begins the reader loop once; re-entrant calls ignored. Stores a cancel function derived from the context. |
| `Stop()` | Cancels the reader loop without closing the subscriber, allowing future `Start` calls. Safe to call multiple times. |
| `Close()` | Calls `Stop()` then closes the underlying subscriber. |
| `IsRunning() bool` | Reports whether the reader goroutine is active. |

### Callbacks

**`onEvent` (Synchronous)**

Called synchronously in the consume goroutine to preserve event ordering and avoid races in downstream persistence. The `StreamCursor` includes a monotonic `Seq` and the upstream stream ID (if available).

```go
onEvent := func(ev events.Event, cur StreamCursor) {
    // Optional: metrics/logging per event
    log.Debug().Str("type", ev.Type()).Uint64("seq", cur.Seq).Msg("event received")
}
```

**`onFrame` (Synchronous)**

Called synchronously after translation for each SEM frame.

```go
onFrame := func(ev events.Event, cur StreamCursor, frame []byte) {
    log.Debug().Str("sem_type", ev.Type).Uint64("seq", cur.Seq).Msg("frame ready")
    conv.pool.Broadcast(frame)
    // Optional: timeline projection from SEM frames
    // _ = conv.timelineProj.ApplySemFrame(ctx, frame)
}
```

### Internal Flow

The `consume` goroutine:

1. Subscribes to the conversation topic once and logs start/stop.
2. For each message: decode JSON into `events.Event`, derive a `StreamCursor` (stream-ID-derived when available, time-based otherwise), call `onEvent` synchronously, build SEM frames with cursor metadata (`event.seq`, `event.stream_id`), invoke `onFrame` for each frame, then ack.
3. When the channel closes, mark `running=false` and clear the cancel handle.

### Lifecycle

**Creation:**

```go
onEvent := r.streamOnEvent(conv)
onFrame := r.streamOnFrame(conv)
conv.stream = NewStreamCoordinator(conv.ID, subscriber, onEvent, onFrame)
```

**Best practices:**

- Close previous coordinators before swapping in a new subscriber.
- Handle nil subscribers when routing is disabled.

**Starting / Stopping:**

- Call `conv.stream.Start(router.baseCtx)` immediately after attaching.
- Use idle timers in `ConnectionPool` to call `conv.stream.Stop()`.
- When evicting a conversation, call `conv.stream.Close()`.

### Error Handling

- **Subscription failures**: Logged, triggers `Stop()`. Next `Start` retries.
- **JSON decode failures**: Log warning, ack message to avoid stalling.
- **SEM frame build errors**: Log warning, drop event without crashing.

## ConnectionPool

### Purpose

`ConnectionPool` centralizes WebSocket bookkeeping for a single conversation. It owns add/remove/broadcast semantics, fans out frames through per-connection writers, and raises idle callbacks so the router can stop `StreamCoordinator` instances when no clients remain.

### Constructor

```go
NewConnectionPool(
    convID string,
    idleTimeout time.Duration,
    onIdle func(),
) *ConnectionPool
```

Creates a pool for the conversation ID with an idle timeout and callback for when the pool becomes empty.

### Methods

| Method | Description |
|--------|-------------|
| `Add(conn wsConn)` | Registers a connection and cancels any pending idle timer. |
| `Remove(conn wsConn)` | Removes the socket, schedules idle timer if empty, closes connection. Safe to call multiple times. |
| `Broadcast(data []byte)` | Enqueues frames to every connection; full buffers cause drop. |
| `SendToOne(conn wsConn, data []byte)` | Enqueues to a single connection; full buffers cause drop. |
| `Count() int` | Number of active sockets. |
| `IsEmpty() bool` | Whether the pool has zero connections. |
| `CloseAll()` | Closes every socket, clears set, cancels idle timer. |
| `CancelIdleTimer()` | Stops pending idle timer without closing connections. |

`wsConn` is any connection implementing `WriteMessage`, `Close`, and `SetWriteDeadline` (e.g., `*websocket.Conn`).

### Idle Timer Behavior

- Timer only runs when **all** connections removed and `idleTimeout > 0`.
- Callback runs outside the mutex to avoid deadlocks.
- `Add` cancels the timer immediately.
- Empty detection is driven by `Add`/`Remove`, not by broadcasts.

**Typical `onIdle` implementation:**

```go
pool := NewConnectionPool(conv.ID, 30*time.Second, func() {
    if conv.stream != nil {
        conv.stream.Stop()
    }
})
```

### Error Handling

- `Broadcast` logs warning with `conv_id` when the send buffer is full, then closes/removes.
- Writer goroutines log and drop connections on `WriteMessage` failure.
- Idle callbacks should be idempotent.

## EngineBuilder

### Purpose

`EngineBuilder` is the composition hub for webchat conversations. It turns profile metadata plus request overrides into an `EngineConfig`, materializes inference engines, and wraps Watermill sinks with profile-specific extractors. By centralizing this logic, Router handlers stay lean and recomposition happens deterministically.

### Interface

```go
type EngineBuilder interface {
    BuildConfig(profileSlug string, overrides map[string]any) (EngineConfig, error)
    BuildFromConfig(convID string, config EngineConfig) (engine.Engine, events.EventSink, error)
}
```

`Router` implements this interface directly.

### Methods

| Method | Description |
|--------|-------------|
| `BuildConfig(profileSlug, overrides)` | Parses overrides and builds a config without allocating engines. Enables signature comparisons before recomposition. |
| `BuildFromConfig(convID, config)` | Materializes the engine and sink from a ready config. Requires `config.StepSettings` to be non-nil. |

### EngineConfig

`EngineConfig` captures all inputs that influence engine composition:

```go
type EngineConfig struct {
    ProfileSlug  string
    SystemPrompt string
    Middlewares  []MiddlewareUse
    Tools        []string
    StepSettings *settings.StepSettings
}
```

`Signature()` returns a deterministic JSON representation (not a hash) so it's debuggable. Comparison of signatures determines whether the engine needs recomposition.

### Override Parsing

Request overrides are validated and merged with profile defaults:

| Override | Type | Behavior |
|----------|------|----------|
| `system_prompt` | `string` | Replaces profile default prompt. Must be non-empty string. |
| `middlewares` | `[{ name, config }]` | Replaces profile default middleware list. Each entry must have a `name`. |
| `tools` | `[string]` | Replaces profile default tools. Entries trimmed and validated. |

Profiles can disable overrides via `AllowOverrides: false`, which causes any override attempt to return an error.

### Typical Flow

```
1. Router receives chat request with profileSlug + overrides
2. BuildConfig(profileSlug, overrides) → EngineConfig
3. Compare config.Signature() with conv.EngConfigSig
4. If different: BuildFromConfig(convID, config) → engine + sink
5. Store engine, sink, and new signature on Conversation
```

This ensures engines are only recomposed when inputs actually change.

### Sink Wrapping

`BuildFromConfig` creates a `WatermillSink` for the conversation topic and optionally wraps it through an `eventSinkWrapper` hook. This hook allows applications to layer extractors (timeline hydration, structured data extraction, etc.) without modifying the builder.

### Error Handling

| Error | Cause | HTTP Status |
|-------|-------|-------------|
| "profile not found" | Unknown profile slug | 400 |
| "profile does not allow overrides" | Overrides sent to locked profile | 400 |
| "engine config missing step settings" | Config not produced by BuildConfig | 500 |
| "router is nil" | Nil receiver (shouldn't happen) | 500 |

## Conversation Lifecycle

### Purpose

`Conversation` holds per-conversation state: the engine, sink, session, stream coordinator, connection pool, and request queue. `ConvManager` manages all live conversations and centralizes lifecycle wiring (idle eviction, timeline store, builder injection).

### Conversation Struct

```go
type Conversation struct {
    ID           string
    SessionID    string
    Sess         *session.Session
    Eng          engine.Engine
    Sink         events.EventSink
    ProfileSlug  string
    EngConfigSig string       // Signature for rebuild detection

    pool         *ConnectionPool
    stream       *StreamCoordinator
    timelineProj *TimelineProjector  // Optional durable projection
}
```

### Request Queue

Conversations serialize chat requests through a queue:

- `activeRequestKey` tracks the currently executing request
- `queue` holds pending requests waiting for the current inference to complete
- `requests` maps request IDs to their records (for cancellation, status)

This prevents concurrent inferences on the same conversation, which would corrupt the session state.

### ConvManager

`ConvManager` creates, retrieves, and evicts conversations:

| Responsibility | Mechanism |
|----------------|-----------|
| **Creation** | `getOrCreate(convID)` — builds engine, subscriber, coordinator, pool |
| **Rebuild detection** | Compares `EngineConfig.Signature()` — only rebuilds when inputs change |
| **Idle eviction** | Configurable `evictIdle` duration, periodic scan via `evictInterval` |
| **Timeline store** | Optional — when set, enables durable projection per conversation |

### Lifecycle Flow

```
New WebSocket connection
    │
    ├─ ConvManager.getOrCreate(convID)
    │   ├─ BuildConfig + BuildFromConfig (if new or signature changed)
    │   ├─ Create StreamCoordinator + ConnectionPool
    │   ├─ Start coordinator
    │   └─ Return Conversation
    │
    ├─ pool.Add(wsConn)
    │
    ├─ ... inference runs ...
    │
    ├─ WebSocket disconnect
    │   └─ pool.Remove(wsConn)
    │       └─ If empty → idle timer starts
    │           └─ On idle → stream.Stop()
    │
    └─ Eviction (if idle too long)
        ├─ stream.Close()
        ├─ pool.CloseAll()
        └─ Remove from ConvManager
```

### Cancellation

Cancellation uses context-based control:

```go
// Starting a run
ctx, cancel := context.WithCancel(baseCtx)
// Store cancel function for external cancellation
// ...
// Cancelling externally
cancel() // Signals the inference goroutine to stop
```

The tool loop and engine both respect context cancellation, so calling `cancel()` stops inference gracefully.

## Related Components

- **SEM translation helpers**: `SemanticEventsFromEvent*` converts `events.Event` into SEM frames.
- **`Router`**: Orchestrates coordinator and pool creation, implements `EngineBuilder`.
- **`Conversation`**: Owns both `StreamCoordinator` and `ConnectionPool`.
- **`ConvManager`**: Manages conversation lifecycle, idle eviction, and rebuild detection.

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/stream_coordinator.go` | StreamCoordinator implementation |
| `pinocchio/pkg/webchat/connection_pool.go` | ConnectionPool implementation |
| `pinocchio/pkg/webchat/conversation.go` | Conversation and ConvManager lifecycle |
| `pinocchio/pkg/webchat/engine_builder.go` | EngineBuilder interface and Router implementation |
| `pinocchio/pkg/webchat/engine_config.go` | EngineConfig struct and signature generation |
| `pinocchio/pkg/webchat/sem_translator.go` | Event to SEM frame translation |

## See Also

- [Backend Internals](webchat-backend-internals.md) — Implementation details and troubleshooting
- [Adding a New Event Type](webchat-adding-event-types.md) — End-to-end custom event tutorial
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage guide
- [Debugging and Ops](webchat-debugging-and-ops.md) — Operational troubleshooting
