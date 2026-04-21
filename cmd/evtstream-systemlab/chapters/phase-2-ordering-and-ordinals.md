# Phase 2 — Ordering and Ordinals

## What this chapter is about

Phase 1 showed you a command entering the Hub, a handler publishing events, and projections running. Everything happened in one process, immediately. That is a useful simplification, but it hides a question that becomes critical in distributed systems: who decides the order?

This chapter answers that question. By the end, you should understand what happens when publishing and consuming are separate moments, why the consumer assigns ordinals instead of the handler, and how per-session ordering works.

---

## 1. The Phase 1 assumption

Here is what Phase 1 showed you:

```go
func (h *Hub) Submit(cmd) {
    handler := h.commands.Lookup(cmd.Name)
    handler(cmd, session, publisher)
    // projections run immediately, synchronously
}
```

The handler calls `publisher.Publish(Event{...})`. Immediately after, the framework runs projections. The handler and projections share a process and a call stack.

This means one thing that is easy to miss: **publish and consume happened at the same time.**

In the same process, by the time the handler returns, the consumer has already processed the event. There is no gap.

---

## 2. What a bus changes

Now introduce a message broker. The handler publishes to a bus. The bus delivers the event to a consumer running in a different process—or maybe on a different machine.

Two things are now true:

1. The handler publishes and returns without waiting for consumption.
2. The consumer receives the event later, in a different context.

```text
Handler process                    Consumer process
┌──────────────────┐              ┌──────────────────┐
│                  │   network   │                  │
│  pub.Publish(e)  │────────────▶│  handleMessage() │
│                  │              │                  │
└──────────────────┘              └──────────────────┘
      returns immediately              runs later
```

This changes who can speak honestly about order.

---

## 3. Who knows the order?

In Phase 1, the handler knows the order because it runs before everything else. It publishes event 1, then event 2, then event 3. The handler knows its own sequence.

In Phase 2, the handler publishes to a bus. It sends event 1, then event 2, then event 3. But the bus might deliver them out of order. The bus might batch them. The bus might deliver event 3 before event 1.

The handler does not know what the consumer will actually observe.

The consumer, however, receives events in the order they arrive from the bus. The consumer is the first place where the framework can speak honestly about what happened.

This is why the consumer assigns ordinals. Not the handler.

---

## 4. The handler publishes with ordinal zero

When the handler publishes, it sets `ordinal = 0`.

```go
pub.Publish(Event{
    Name:    "LabStarted",
    SessionId: cmd.SessionId,
    Ordinal:   0,  // placeholder—I don't know the real order yet
    Payload:   &LabStarted{MsgId: msgId},
})
```

Zero means: "I know what happened. I am not claiming where this belongs in the sequence."

This is not a placeholder in the sloppy sense. It is a deliberate statement of scope. The handler's job is to describe what happened. Ordering is not the handler's job.

---

## 5. The consumer assigns the real ordinal

When the consumer receives the event, it assigns the real ordinal.

```go
func handleMessage(msg Message) {
    ev := decode(msg)
    
    // Consumer assigns the ordinal here—not the handler
    ord := ordinals.Next(ev.SessionId, msg.Metadata)
    ev.Ordinal = ord
    
    // Now projections run with the real ordinal
    entities := timelineProjection.Project(ev, store.View(ev.SessionId))
    store.Apply(ev.SessionId, ord, entities)
}
```

The ordinal is what the framework uses for:
- **Order.** If you know ordinals 1 through 5, you know the sequence.
- **Reconnect.** A client that disconnected at ordinal 3 can ask for everything after ordinal 3.
- **Hydration.** The store tracks the latest ordinal per session.

---

## 6. How ordinals are derived

In this lab, the consumer tries to derive ordinals from stream metadata.

Stream IDs look like this:

```
1713560000123-0
1713560000123-1
1713560000124-0
```

The framework parses this into a comparable ordinal. The timestamp and sequence parts are preserved.

If metadata is missing or invalid, the framework falls back to a monotonic counter:

```go
func (a *OrdinalAssigner) Next(sessionId string, meta Metadata) Ordinal {
    if streamId := meta["stream_id"]; streamId != "" {
        if o, ok := parseStreamId(streamId); ok {
            return o
        }
    }
    return a.counter.Next(sessionId)  // fallback: monotonic local counter
}
```

The system never fails to assign an ordinal.

---

## 7. Per-session ordering

The framework orders events per session, not globally.

Session A and Session B can have interleaved publishes on the same topic. Session A's ordinal sequence and Session B's ordinal sequence are independent.

| Session | Ordinals |
|---------|----------|
| session-a | 1, 2, 3 |
| session-b | 1, 2 |

Session A has three events. Session B has two. They do not interfere.

This matters because the framework's unit of work is the session. Hydration tracks per session. Reconnect resumes per session.

---

## 8. The pseudocode comparison

Here is Phase 1:

```go
func (h *Hub) Submit(cmd) {
    handler := h.commands.Lookup(cmd.Name)
    handler(cmd, session, publisher)
    // projections run here, immediately
}
```

Here is Phase 2:

```go
// Hub: publish only
func (h *Hub) Submit(cmd) {
    handler := h.commands.Lookup(cmd.Name)
    handler(cmd, session, publisher)
    // done—consumer handles projections asynchronously
}

// Consumer: project asynchronously
func handleMessage(msg Message) {
    ev := decode(msg)
    ord := ordinals.Next(ev.SessionId, msg.Metadata)
    ev.Ordinal = ord

    view := store.View(ev.SessionId)
    uiEvents := uiProjection.Project(ev, view)
    entities := timelineProjection.Project(ev, view)

    store.Apply(ev.SessionId, ord, entities)
    fanout.PublishUI(ev.SessionId, uiEvents)
}
```

The hub is now thin. The consumer is the orchestration center.

---

## 9. The UIFanout seam

The consumer projects UI events. But the consumer should not know about websockets, browser tabs, or connection lifetimes.

```go
type UIFanout interface {
    PublishUI(sessionId string, ordinal Ordinal, events []UIEvent)
}
```

The consumer calls `fanout.PublishUI(...)` and is done. What happens next is the fanout's problem.

This separation means Phase 3 can add websocket transport without changing the consumer.

---

## 10. Reading the page

The Phase 2 page shows the publish/consume split explicitly.

Look at the **Message History** panel. Two columns:

- `publishedOrdinal`: always 0
- `consumedOrdinal`: the real ordinal assigned by the consumer

The framework uses the consumed value. That is the truth.

The **Bus Trace** shows what happened in order. The **Session Ordinals** shows per-session sequences. The **Checks** prove the invariants held.

---

## 11. Things to try

**Publish A.** Look at the message history. `publishedOrdinal` is 0. `consumedOrdinal` is a large value. The framework believes the consumed value.

**Publish B.** Session B gets its own ordinal sequence. Session A is unaffected. Sessions are isolated.

**Burst A.** Multiple publishes, each with ordinal 0. Multiple consumes, each with a real ordinal. The ordinals are monotonic.

Note: the burst order and displayed order may not match perfectly. The framework guarantees monotonic ordinals, not visual order.

**Stream Mode: missing.** The framework falls back to a monotonic counter. Everything still works.

**Restart Consumer.** The consumer restarts and continues processing. The lab remains responsive.

**Export.** Ordinals render as strings. This is intentional. JavaScript cannot represent large stream IDs precisely as numbers.

---

## 12. What the checks prove

| Check | What it proves |
|-------|----------------|
| `publishOrdinalZero` | Handler published with ordinal 0, not the real ordinal |
| `monotonicPerSession` | Consumed ordinals increase monotonically within each session |
| `sessionIsolation` | Session A and Session B have independent ordinal sequences |
| `messagesConsumed` | The consumer processed messages, not just the page reflecting publish intent |

---

## Key Points

- Phase 1 assumed publish and consume happen at the same time. Phase 2 separates them.
- The handler publishes with ordinal 0. The handler knows what happened, not where it belongs.
- The consumer assigns the real ordinal. The consumer is the first place that knows the actual order.
- Ordinals are monotonic per session. Sessions are independent.
- The ordinal assigner prefers stream metadata when available, falls back to a counter otherwise.
- `UIFanout` separates the consumer from transport concerns.
- Correctness is end-to-end. If the frontend rounds ordinals, the lab is misleading.

---

## API Reference

- **`WithEventBus(...)`**: Configure the bus publisher.
- **`WithUIFanout(...)`**: Configure the UI output boundary.
- **`Run(...)`**: Start the consumer loop.
- **`Shutdown(...)`**: Stop the consumer gracefully.
- **`OrdinalAssigner.Next(...)`**: Assign the next ordinal for a session.
- **`PartitionKeyForSession(...)`**: Derive the partition key from a session ID.

---

## File References

### Framework files

- `pkg/evtstream/bus.go` — bus publisher interface
- `pkg/evtstream/consumer.go` — consumer loop and orchestration
- `pkg/evtstream/ordinals.go` — ordinal assignment logic
- `pkg/evtstream/fanout.go` — UI output seam
- `pkg/evtstream/hub.go` — command routing

### Systemlab files

- `cmd/evtstream-systemlab/phase2_lab.go` — Phase 2 lab setup
- `cmd/evtstream-systemlab/static/partials/phase2.html` — page layout
- `cmd/evtstream-systemlab/static/js/pages/phase2.js` — page behavior

### Tests

- `pkg/evtstream/bus_test.go`
- `pkg/evtstream/ordinals_test.go`
- `cmd/evtstream-systemlab/lab_environment_test.go`