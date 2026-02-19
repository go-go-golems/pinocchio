---
Title: Webchat Backend Internals
Slug: webchat-backend-internals
Short: Deep-dive into StreamCoordinator and ConnectionPool implementation, concurrency, and performance.
Topics:
- webchat
- streaming
- internals
- concurrency
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

This document provides implementation details for `StreamCoordinator` and `ConnectionPool`. For API reference and usage patterns, see [Backend Reference](webchat-backend-reference.md).

## StreamCoordinator Internals

### Event Subscription

`StreamCoordinator` uses [Watermill](https://watermill.io/) as an abstraction layer over Redis Streams (when enabled). The coordinator subscribes to a conversation-specific topic and receives events through a Go channel.

**Architecture:**

```
┌─────────────────────────────────────────────────────┐
│              StreamCoordinator                       │
│                                                      │
│  subscriber.Subscribe(ctx, "chat:abc123")           │
│       ↓                                              │
│  [Watermill message.Subscriber interface]           │
│       ↓                                              │
│  for msg := range ch {                              │
│    event := events.NewEventFromJson(msg.Payload)    │
│    cursor := derive StreamCursor (seq + stream_id)  │
│    onEvent(event, cursor)  // Synchronous hydration │
│    frames := SemanticEventsFromEventWithCursor(...) │
│    for _, frame := range frames {                   │
│      onFrame(event, cursor, frame) // broadcast     │
│    }                                                 │
│    msg.Ack()              // Confirm processed      │
│  }                                                   │
└─────────────────────────────────────────────────────┘
            ↓
  [Watermill Redis Subscriber or In-Memory]
            ↓
  Consumer Group semantics
```

### Redis Consumer Group Semantics

When using Redis Streams, consumer groups enable load balancing and fault tolerance.

**Key Properties:**

1. **Load Balancing**: Multiple servers for same conversation distribute messages.
2. **At-Least-Once Delivery**: Crashed consumer's pending messages are reassigned.
3. **ACK Required**: `msg.Ack()` tells Redis "message processed."
4. **Ordering**: Messages within a stream are FIFO ordered.

### Goroutine Lifecycle

**Entry Point: `Start()` spawns `consume()` goroutine**

```
Start()
  → Lock, check not already running
  → Create cancel context
  → Set running=true
  → Spawn consume(ctx) goroutine
```

**The `consume()` Loop:**

```
consume(ctx):
  1. Subscribe to topic
  2. Loop over messages from channel
  3. For each: decode → onEvent → translate → onFrame → ack
  4. On channel close: set running=false, clear cancel
```

**Stopping:**

- `Stop()`: Cancels context → Watermill stops → channel closes → loop exits
- `Close()`: Calls `Stop()` then closes subscriber

### Concurrency Model

Callbacks are synchronous to preserve event ordering:

- Events processed in arrival order
- `onEvent` runs before `onFrame` for each event
- Multiple frames from one event broadcast sequentially
- Database writes happen in order (no races)

**Goroutine Count Per Conversation:**

```
1. WebSocket read loop (router.go)
2. Tool loop (if inference running)
3. StreamCoordinator.consume()
4. One writer goroutine per active WebSocket connection (ConnectionPool)
```

Steady state: 3 long-lived goroutines per conversation.

### Performance Characteristics

| Operation | Cost | Notes |
|-----------|------|-------|
| Subscribe call | ~5-20ms | Redis XREADGROUP + channel setup |
| Message receive | ~1-5ms | Redis → Watermill → Go channel |
| Event deserialize | ~0.1-1ms | JSON unmarshal |
| Timeline projection | 10-100ms | SQLite INSERT (blocks consume loop) |
| WebSocket broadcast | 2-5ms per conn | Depends on connection count |
| msg.Ack() | ~1ms | Redis XACK command |

**Bottleneck**: Usually timeline projection or WebSocket writes, not Redis.

**Trade-offs**: Synchronous projection ensures ordering but can delay broadcasts if the database is slow.

### Sequence Derivation

`StreamCoordinator` derives `event.seq` from Redis stream IDs when metadata is present. If not present, it falls back to a time-based monotonic sequence so timeline versions stay comparable to user-message versions.

## ConnectionPool Internals

### Locking Strategy

`ConnectionPool` uses a mutex to protect the connection map and idle timer, while writes happen in per-connection goroutines. Broadcasts are non-blocking and enqueue to buffered channels.

**Lock Scope:**

- `Add()`: Locks to add connection and cancel timer
- `Remove()`: Locks to remove connection and schedule timer
- `Broadcast()`: Locks to snapshot clients, then enqueues without blocking
- `SendToOne()`: Locks to locate client, then enqueues without blocking

### Idle Timer Implementation

Uses `time.AfterFunc` to schedule callback when pool becomes empty.

**Timer Lifecycle:**

1. Pool becomes empty → `Remove()` schedules timer
2. Timer fires → Calls `onIdle` (stops StreamCoordinator)
3. Connection added → `Add()` cancels timer

The callback runs outside the mutex to prevent deadlocks.

### Error Handling

`Broadcast()` and `SendToOne()` handle backpressure and write failures by removing connections:

- **Send buffer full**: Log warning, close connection, remove from pool
- **Write error** (writer goroutine): Log warning, close connection, remove from pool
- **Empty payload**: Ignored

Dead connections are automatically pruned; clients can reconnect.

## Troubleshooting

### No frames reaching clients

- Verify `conv.stream.IsRunning()` returns true
- Check `pool.Count() > 0`
- Review logs for decode errors or SEM frame build warnings
- Confirm consumer group is active (when using Redis)

### Duplicate frames

- Confirm only one coordinator per conversation
- Check for multiple `Start()` calls
- Double-starts are prevented but verify no duplicate instances

### Projection blocking broadcasts

- Since projection is synchronous, slow writes delay broadcasts
- Monitor SQLite performance
- Consider database indexing for timeline tables

### Idle timer never fires

- Ensure `idleTimeout > 0` and `onIdle` is non-nil
- Check that `Remove()` is called on disconnect
- `CloseAll()` stops the timer; reinitialize for new connections

## Timeline Projector Internals

The `TimelineProjector` converts ephemeral SEM frames into durable, version-tracked timeline entities stored via a `TimelineStore`. It maintains in-memory state to reconstruct complete entities across streaming events.

### Architecture

```
SEM Frame (JSON)
    ↓
TimelineProjector.ApplySemFrame()
    ├── Parse envelope (type, id, seq, stream_id, data)
    ├── Switch on event type
    ├── Unmarshal protobuf data payload
    ├── Construct TimelineEntityV2 (kind + props)
    └── Upsert to TimelineStore with seq as version
```

The projector is per-conversation: each `Conversation` creates its own projector instance. The projector is called synchronously from the `StreamCoordinator.consume()` loop, which means projection happens in event order.

### SEM Frame to Entity Mapping

| SEM Event Type | Entity Kind | Props Contract Source | Notes |
|---------------|-------------|------------------------|-------|
| `llm.start` | `message` | `pinocchio/proto/sem/timeline/message.proto` | Sets role, marks streaming=true |
| `llm.delta` | `message` | `pinocchio/proto/sem/timeline/message.proto` | Cumulative content, throttled writes |
| `llm.final` | `message` | `pinocchio/proto/sem/timeline/message.proto` | Final text, streaming=false |
| `llm.thinking.start` | `message` | `pinocchio/proto/sem/timeline/message.proto` | role=thinking |
| `llm.thinking.delta` | `message` | `pinocchio/proto/sem/timeline/message.proto` | Thinking content, throttled |
| `llm.thinking.final` | `message` | `pinocchio/proto/sem/timeline/message.proto` | Thinking complete |
| `tool.start` | `tool_call` | `pinocchio/proto/sem/timeline/tool.proto` | name, input, status=running |
| `tool.done` | `tool_call` | `pinocchio/proto/sem/timeline/tool.proto` | status=completed, progress=1 |
| `tool.result` | `tool_result` | `pinocchio/proto/sem/timeline/tool.proto` | Result (structured or raw) |
| `thinking.mode.*` | `thinking_mode` | `pinocchio/cmd/web-chat/proto/sem/timeline/middleware.proto` | App-owned module projection |
| `planning.*` | `planning` | `pinocchio/proto/sem/timeline/tool.proto` + planning aggregate schema | Aggregated from multiple events |
| `execution.*` | (updates planning) | `pinocchio/proto/sem/timeline/tool.proto` + planning aggregate schema | Updates nested execution snapshot |

### Write Throttling

High-frequency `llm.delta` events would overwhelm the database during fast streaming. The projector throttles these writes:

```
if now - lastMsgWrite[id] < 250ms:
    skip DB write (return nil)
else:
    write to DB, update lastMsgWrite[id]
```

This means the DB state can lag up to 250ms behind the in-memory/WebSocket state during streaming. The `llm.final` event always writes (no throttling), so the final state is always persisted.

**Impact on hydration:** If the server crashes during streaming, the hydrated state may be up to 250ms stale. The `llm.final` event ensures the completed message is always durable.

### Role Memory

The projector stores the role from `llm.start` and applies it to all subsequent `llm.delta` events for the same message ID:

```
llm.start (id=abc, role=assistant)  → store msgRoles["abc"] = "assistant"
llm.delta (id=abc)                  → use msgRoles["abc"] for entity role
llm.final (id=abc)                  → use msgRoles["abc"] for entity role
```

If the `llm.start` event is missed (e.g., late WebSocket connection), deltas will have no role. The entity will still be created but the role field will be empty.

Thinking events use the same mechanism with `role=thinking` and append `:thinking` to the message ID to create a separate entity from the main assistant message.

### Stable ID Resolution

The SEM translator (not the projector, but closely related) resolves stable message IDs using a three-tier fallback:

1. **Metadata ID** — if the event has an explicit `metadata.ID`, use it
2. **Cached ID** — look up by inference ID → turn ID → session ID (first match wins)
3. **Generated fallback** — `"llm-" + uuid.New()`

IDs are cached per-translator to ensure streaming events for the same logical message use the same ID. The cache is cleared when `EventFinal` is processed to prevent memory leaks.

### Tool Result Handling

Each `tool.result` event creates **two** entities:

1. **Tool call completion** — updates the existing `tool_call` entity with `status=completed`, `progress=1.0`, cached name and input from `tool.start`
2. **Tool result entity** — new entity with ID `{toolCallID}:result` (or `{toolCallID}:custom` for custom kinds) containing the result data

The projector attempts to parse tool results as JSON. If successful and the result is an object, it stores the structured form. Otherwise it stores the raw string.

Special case: "calc" tool results get `CustomKind: "calc_result"` for specialized widget rendering.

### Planning Aggregation

Planning events are stateful — they build up over time. The projector maintains a `planningAgg` struct per `session_id`:

```
planning.start       → create planningAgg, set provider/model/maxIterations
planning.iteration   → add/update iteration in map[iterationIndex]
planning.reflection  → update iteration's reflection text
planning.complete    → set finalDecision, status=executing
execution.start      → set nested execution snapshot
execution.complete   → set execution status, overall status=completed
```

On **every** planning event, the projector:
1. Acquires the lock and updates the in-memory aggregate
2. Collects all iterations, sorts them by index
3. Clones the snapshot (to avoid holding the lock during DB write)
4. Releases the lock
5. Upserts the full snapshot to the store

This ensures the stored entity always reflects the complete planning state, not just the latest event.

### Version Semantics

The `version` parameter passed to `TimelineStore.Upsert()` is the SEM frame's `Seq` value — a monotonic number derived from Redis stream IDs (when available) or a time-based fallback. This enables:

- **Incremental hydration**: `GetSnapshot(sinceVersion=N)` returns only entities updated after version N
- **Conflict resolution**: Higher version wins during merge
- **Ordering**: Versions are comparable across entity types within a conversation

### Custom Timeline Handlers

The projector supports an extension point via `handleTimelineHandlers()` which allows external code to register handlers for specific event types. These run before the built-in switch statement, enabling applications to intercept and handle custom events without modifying the projector code.

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/stream_coordinator.go` | StreamCoordinator implementation |
| `pinocchio/pkg/webchat/connection_pool.go` | ConnectionPool implementation |
| `pinocchio/pkg/webchat/sem_translator.go` | Event to SEM translation |
| `pinocchio/pkg/webchat/timeline_projector.go` | Timeline hydration/projection |
| `pinocchio/pkg/webchat/timeline_store.go` | TimelineStore interface |
| `pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go` | SQLite implementation |
| `pinocchio/cmd/web-chat/thinkingmode/backend.go` | App-owned `thinking.mode.*` SEM + projection handlers |
| `pinocchio/cmd/web-chat/proto/` | App-owned middleware/timeline proto schemas + Buf module |

## See Also

- [Backend Reference](webchat-backend-reference.md) — API contracts and usage patterns
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
- [Debugging and Ops](webchat-debugging-and-ops.md) — Operational procedures
