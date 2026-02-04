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

`StreamCoordinator` bridges event topics and WebSocket connections. It owns a subscriber, converts inbound `events.Event` payloads into SEM frames, stamps them with a `StreamCursor` (stream ID + sequence), and fans out frames through callbacks so conversations can log emissions and broadcast bytes to every `ConnectionPool` member.

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

Creates a coordinator for the given conversation ID, wiring the subscriber and callbacks. `onEvent` handles timeline hydration; `onFrame` handles logging and WebSocket broadcast.

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
    // Hydrate timeline from event
    projector.HandleEvent(ctx, convID, ev)
}
```

**`onFrame` (Synchronous)**

Called synchronously after translation for each SEM frame.

```go
onFrame := func(ev events.Event, cur StreamCursor, frame []byte) {
    log.Debug().Str("sem_type", ev.Type).Uint64("seq", cur.Seq).Msg("frame ready")
    conv.pool.Broadcast(frame)
}
```

### Internal Flow

The `consume` goroutine:

1. Subscribes to the conversation topic once and logs start/stop.
2. For each message: decode JSON into `events.Event`, derive a `StreamCursor`, call `onEvent` synchronously, build SEM frames with cursor metadata (`event.seq`, `event.stream_id`), invoke `onFrame` for each frame, then ack.
3. When the channel closes, mark `running=false` and clear the cancel handle.

### Lifecycle

**Creation:**

```go
onEvent := r.streamOnEvent(conv)
onFrame := r.streamOnFrame(conv)
conv.stream = NewStreamCoordinator(conv.ID, subscriber, onEvent, onFrame)
```

**Best practices:**

- Create one translator per conversation to isolate tool-call caches.
- Close previous coordinators before swapping in a new subscriber.
- Handle nil subscribers when routing is disabled.

**Starting / Stopping:**

- Call `conv.stream.Start(router.baseCtx)` immediately after attaching.
- Use idle timers in `ConnectionPool` to call `conv.stream.Stop()`.
- When evicting a conversation, call `conv.stream.Close()`.

### Error Handling

- **Subscription failures**: Logged, triggers `Stop()`. Next `Start` retries.
- **JSON decode failures**: Log warning, ack message to avoid stalling.
- **Translator errors**: Log warning, drop event without crashing.

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

## Related Components

- **`SEMTranslator`**: Converts `events.Event` into SEM frames; scoped per coordinator.
- **`Router`**: Orchestrates coordinator and pool creation.
- **`Conversation`**: Owns both `StreamCoordinator` and `ConnectionPool`.

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/stream_coordinator.go` | StreamCoordinator implementation |
| `pinocchio/pkg/webchat/connection_pool.go` | ConnectionPool implementation |
| `pinocchio/pkg/webchat/conversation.go` | Conversation lifecycle, owns stream + pool |
| `pinocchio/pkg/webchat/sem_translator.go` | Event to SEM frame translation |

## See Also

- [Backend Internals](webchat-backend-internals.md) — Implementation details and troubleshooting
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage guide
- [Debugging and Ops](webchat-debugging-and-ops.md) — Operational troubleshooting
