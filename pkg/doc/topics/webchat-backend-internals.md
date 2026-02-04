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

## Key Files

| File | Purpose |
|------|---------|
| `pinocchio/pkg/webchat/stream_coordinator.go` | StreamCoordinator implementation |
| `pinocchio/pkg/webchat/connection_pool.go` | ConnectionPool implementation |
| `pinocchio/pkg/webchat/sem_translator.go` | Event to SEM translation |
| `pinocchio/pkg/webchat/timeline_projector.go` | Timeline hydration/projection |

## See Also

- [Backend Reference](webchat-backend-reference.md) — API contracts and usage patterns
- [Webchat Framework Guide](webchat-framework-guide.md) — End-to-end usage
- [Debugging and Ops](webchat-debugging-and-ops.md) — Operational procedures
