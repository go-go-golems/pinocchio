# Phase 3 — Hydration and Reconnect

## What this chapter is about

Phase 2 showed you how the consumer assigns ordinals and how the framework establishes consumed order. Phase 1 showed you how commands become events and how projections derive state. But neither phase answered a practical question: what happens when a client connects, leaves, and comes back?

This chapter answers that question. By the end, you should understand why snapshot-before-live is non-negotiable, how ConnectionId and SessionId work together, and what the framework guarantees when a client reconnects.

---

## 1. Why reconnect is a framework problem

Many systems treat reconnect as a frontend concern. The browser dropped, so the frontend reconnects. The socket reopened, so the UI asks for state.

That is not wrong, but it is incomplete.

Reconnect is a framework problem because the framework owns:

- current durable state (the hydration store)
- live event delivery (the UIFanout)
- session routing (by SessionId)
- transport identity (by ConnectionId)

If the framework has no coherent answer to reconnect, the frontend can reconnect all it wants and still end up with duplicated updates, missing updates, or inconsistent state.

Phase 3 comes after hydration and ordering because reconnect only makes sense when the system already knows what state is current and how event order is defined.

---

## 2. The central rule: snapshot before live

> A reconnecting client should receive a coherent snapshot first, and only then continue with live UI events.

This is the most important sentence in Phase 3.

Consider what happens if this rule is violated. A client reconnects. It receives live events. Then it receives a snapshot. The snapshot contradicts the live events. The client must reconcile them, or it ends up wrong.

Now consider what happens when the rule holds. A client reconnects. It receives a snapshot. That snapshot represents the current state. Then it receives live events that continue from that state. No reconciliation needed. The client just continues.

The framework prevents an entire category of bugs by sequencing correctly.

---

## 3. What a subscribe looks like

Here is the sequence when a client subscribes to a session:

```text
Client connects
    -> receives ConnectionId
    ->
Client subscribes to SessionId
    ->
Framework loads snapshot from HydrationStore
    ->
Framework sends snapshot message (first)
    ->
Framework sends live UI events (after snapshot)
```

The snapshot arrives before any live events. That is the rule.

---

## 4. ConnectionId vs SessionId

Phase 0 introduced these concepts. Phase 3 makes them operational.

**SessionId** is the business-level routing key. It identifies the unit of work. All events for a session share the same SessionId. The hydration store tracks state per SessionId.

**ConnectionId** is the transport-level identity. It identifies one socket connection. One session can have multiple connections over time (reconnect), or multiple simultaneous connections (multiple tabs watching the same session).

```text
SessionId: "session-abc"     <- business identity, stable
ConnectionId: "conn-123"     <- transport identity, changes on reconnect
ConnectionId: "conn-456"     <- different connection, same session
```

This distinction matters because:

- A client disconnects and reconnects. The SessionId stays the same. The ConnectionId changes.
- Multiple tabs can watch the same session. Each has its own ConnectionId.
- The framework tracks subscriptions by SessionId. It tracks presence by ConnectionId.

---

## 5. The transport architecture

The websocket transport sits downstream of the consumer:

```text
Commands
   |
   v
Handlers
   |
   v
Canonical backend events
   |
   v
Consumer
   |------------------> TimelineProjection -> HydrationStore
   |
   +------------------> UIProjection -> UIFanout -> Websocket transport
                                                   |
                                                   v
                                              Client connections
```

The transport is not the source of truth. It is the live-delivery mechanism for one derived view of the truth.

The transport should:
- accept connections and assign ConnectionIds
- track subscriptions by SessionId
- deliver snapshots on subscribe
- deliver live UI events after snapshots
- stay unaware of application semantics

The transport should not:
- invent application semantics
- assign ordinals
- interpret command meanings
- become the place where business logic lives

---

## 6. Why this matters for correctness

Here is what the framework guarantees when a client subscribes:

1. The snapshot reflects the current hydration store state.
2. The snapshot arrives before any live events.
3. Live events continue from where the snapshot left off.
4. Events arrive in ordinal order.

Here is what the framework does not guarantee:

- That multiple connections see identical delivery timing.
- That the client never misses an event (the client must handle that).

The framework establishes the correct sequence. The transport delivers it. The client handles delivery confirmation.

---

## 7. The Phase 3 page

The Phase 3 page simulates two clients to make reconnect semantics visible.

**Client A** and **Client B** each have their own connection lifecycle. They can subscribe to the same session or different sessions. They can disconnect and reconnect independently.

This forces you to think about:
- independent connection lifecycle
- shared session state
- different subscribe timings
- reconvergence toward the same final session view

---

## 8. Things to try

**Connect Client A, subscribe to a session.** The client connects. A snapshot arrives. Live events continue from there.

**Generate activity while Client A is connected.** Client A receives live events as they happen.

**Disconnect Client A.** The connection closes. The framework stops sending to that ConnectionId.

**Reconnect Client A.** The client reconnects. It subscribes. It receives a snapshot of current state. Then it receives live events. Notice: the live events continue naturally from where the snapshot left off.

**Connect Client B to the same session while activity is ongoing.** Client B subscribes. It receives a snapshot of current state. Then it receives live events. Notice: both clients converge to the same final session view.

**Disconnect Client A, keep Client B connected, generate more activity.** Client A misses the activity. Client B receives it.

**Reconnect Client A.** Client A receives a new snapshot (current state). Then it receives live events. Notice: Client A and Client B are back in sync.

---

## 9. What the checks prove

| Check | What it proves |
|-------|----------------|
| `snapshotBeforeLive` | The snapshot arrived before any live events |
| `convergence` | Multiple clients converged to the same session state |
| `connectionIsolation` | Connection lifecycle does not affect session state |
| `ordinalOrder` | Events arrived in correct ordinal order |

---

## 10. Common mistakes

**Mistake: live before snapshot.** A client receives live events before it has a coherent base state. The framework must sequence snapshot before live.

**Mistake: transport owning business semantics.** If websocket code interprets application event meanings, the framework boundary is polluted. The transport should only deliver what the UIProjection produces.

**Mistake: one connection equals one session.** One session can have multiple connections over time (reconnect) or simultaneously (multiple tabs). The framework must handle this.

---

## Key Points

- Reconnect is a framework problem, not just a frontend concern. The framework owns the relationship between durable state and live delivery.
- Snapshot before live is non-negotiable. The framework must establish the correct state before delivering live events.
- SessionId is the business routing key. ConnectionId is the transport identity.
- One session can have multiple connections over time (reconnect) or simultaneously (multiple tabs).
- The transport sits downstream of the consumer. It delivers UI events; it does not interpret them.
- Clients with different live histories should converge to the same session truth.

---

## API Reference

- **`Subscribe(sessionId, connectionId)`**: Subscribe a connection to a session.
- **`Unsubscribe(sessionId, connectionId)`**: Remove a subscription.
- **`DeliverSnapshot(connectionId, snapshot)`**: Deliver the current state to a reconnecting client.
- **`DeliverEvent(connectionId, event)`**: Deliver a live UI event.
- **`HydrationStore.Snapshot(sessionId)`**: Load current state for a session.

---

## File References

### Framework files

- `pkg/evtstream/transport/transport.go` — transport interface
- `pkg/evtstream/fanout.go` — UI event fanout
- `pkg/evtstream/hydration.go` — hydration store interface
- `pkg/evtstream/hydration/memory/store.go` — in-memory store

### Systemlab files

- `cmd/evtstream-systemlab/phase3_lab.go` — Phase 3 lab setup
- `cmd/evtstream-systemlab/static/partials/phase3.html` — page layout
- `cmd/evtstream-systemlab/static/js/pages/phase3.js` — page behavior

### Tests

- `pkg/evtstream/transport/transport_test.go`
- `pkg/evtstream/hydration/memory/store_test.go`