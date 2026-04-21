# Phase 1 — Command → Event → Projection

## What this chapter is about

This chapter shows you what happens when a client tells the server to do something. That sounds simple, but there are design choices embedded in every step: how the request is routed, what the handler produces, and how the system decides what the client should see next.

By the end of this chapter, you should understand the full path from a command entering the Hub to a hydration snapshot being recorded.

---

## 1. The problem: how does a client ask the server to do something?

In a simple system, a client calls a function directly:

```go
result := backend.DoSomething(request)
```

This works for single-machine systems. It does not scale to systems where:

- Multiple clients connect simultaneously
- The server needs to track session state
- Clients might disconnect and reconnect
- You want to understand what happened for debugging

Commands solve a different problem. A command is a named request that the framework routes to a handler. The client does not call a function directly—it sends a command, and the framework decides what to do.

---

## 2. The Hub: routing, not logic

The Hub sits at the center of the framework. Its job is to receive a command and route it to the right handler.

```go
func (h *Hub) Submit(cmd Command) error {
    handler := h.commands.Lookup(cmd.Name)
    if handler == nil {
        return fmt.Errorf("unknown command: %s", cmd.Name)
    }
    session := h.sessions.GetOrCreate(cmd.SessionId)
    return handler(cmd, session, h.publisher)
}
```

The Hub does three things:

1. **Looks up the handler** by name. Handlers are registered at startup.
2. **Gets or creates the session.** If the SessionId has never been seen, a fresh session is created.
3. **Calls the handler**, passing the command, session, and publisher.

The Hub does not know what any command does. It routes. Everything else flows from that.

---

## 3. Why handlers publish events instead of returning values

Here is the critical design choice.

A naive approach would have the handler return the result:

```go
func handler(cmd Command) ChatMessage {
    // do work
    return ChatMessage{Text: "hello"}
}
```

This has a problem: the framework must now know what to do with that return value. Route it somewhere? Store it? Send it to the client? That knowledge lives in the handler, or in the framework, or in both in ways that are hard to untangle.

Instead, the handler describes what happened by publishing events:

```go
func handler(cmd Command, session *Session, pub Publisher) error {
    msgId := generateMessageId()

    pub.Publish(Event{Name: "LabStarted", SessionId: cmd.SessionId, Payload: &LabStarted{MsgId: msgId}})

    for _, chunk := range splitPrompt(cmd.Payload.Prompt) {
        pub.Publish(Event{Name: "LabChunk", SessionId: cmd.SessionId, Payload: &LabChunk{MsgId: msgId, Text: chunk}})
    }

    pub.Publish(Event{Name: "LabFinished", SessionId: cmd.SessionId, Payload: &LabFinished{MsgId: msgId}})
    return nil
}
```

The handler says: here is what happened. It does not say: here is what the client should see. That is the framework's job.

One command produces multiple events. This is intentional. Real work has a beginning, intermediate states, and an end. The handler publishes all of them.

---

## 4. What happens when you click Submit

Here is the full sequence:

```text
Browser sends request
         ↓
  Hub.Submit(command)
         ↓
  Lookup handler by name
         ↓
  Get or create session
         ↓
  Call handler(session, publisher)
         ↓
  Handler publishes events
         ↓
  Framework assigns ordinals
         ↓
  UI projection runs → UI events
         ↓
  Timeline projection runs → entities
         ↓
  Store applies entities
         ↓
  Response returned to browser
```

Notice what the handler does NOT do:
- It does not format output
- It does not decide what the client sees
- It does not touch the store

The handler describes what happened. The framework handles the rest.

---

## 5. Ordinals: who decides the order?

Every event gets an ordinal—a number that defines its position in the sequence.

```text
Event: LabStarted      Ordinal: 1
Event: LabChunk        Ordinal: 2
Event: LabChunk        Ordinal: 3
Event: LabFinished     Ordinal: 4
```

Ordinals serve three purposes:

1. **Order.** If you know ordinals 1 through 4, you know the sequence.
2. **Reconnect.** A client that disconnects at ordinal 3 can ask for everything after ordinal 3.
3. **Hydration.** The store tracks the latest ordinal per session.

In Phase 1, ordinals are assigned by the publisher. Phase 2 explains why the consumer assigns ordinals in distributed systems.

---

## 6. Two projections, one source

After the handler publishes an event, the framework runs two projections:

**The UI projection** asks: what should a live client see right now?

```go
func (p *UIProjection) Project(event Event, view View) []UIEvent {
    switch event.Name {
    case "LabStarted":
        return []UIEvent{{Name: "LabMessageStarted", Payload: &LabMessageStarted{MsgId: event.Payload.(*LabStarted).MsgId}}}
    case "LabChunk":
        return []UIEvent{{Name: "LabMessageAppended", Payload: &LabMessageAppended{Text: event.Payload.(*LabChunk).Text}}}
    case "LabFinished":
        return []UIEvent{{Name: "LabMessageFinished", Payload: nil}}
    }
    return nil
}
```

**The timeline projection** asks: what persistent state should the system remember?

```go
func (p *TimelineProjection) Project(event Event, view View) []Entity {
    switch event.Name {
    case "LabStarted":
        return []Entity{{Id: event.Payload.(*LabStarted).MsgId, Kind: "LabMessage", Status: "started"}}
    case "LabChunk":
        msg := view.Get(event.Payload.(*LabChunk).MsgId)
        msg.Payload["text"] += event.Payload.(*LabChunk).Text
        return []Entity{msg}
    case "LabFinished":
        msg := view.Get(event.Payload.(*LabFinished).MsgId)
        msg.Status = "finished"
        return []Entity{msg}
    }
    return nil
}
```

Both projections consume the same event stream. They answer different questions.

| | UIProjection | TimelineProjection |
|--|-------------|-------------------|
| Output | UI events | Timeline entities |
| Purpose | Live client updates | Persistent state |
| Lifetime | Transient | Durable |

---

## 7. The hydration store

The hydration store records timeline entities.

```go
type HydrationStore interface {
    Apply(sessionId string, ordinal Ordinal, entities []Entity) error
    Snapshot(sessionId string) (Snapshot, error)
    View(sessionId string) View
    Cursor(sessionId string) Ordinal
}
```

**`Apply`** updates the store and advances the cursor. This is called after projections run.

**`Snapshot`** returns the current state for a session. This is what you see in the page's Snapshot panel.

**`View`** gives projections a read-only view of current state.

**`Cursor`** returns the latest applied ordinal for a session.

In Phase 1, the store is in-memory. Phase 5 explains how SQL persistence works.

---

## 8. Reading the page

The Phase 1 page shows the full path. Read it in this order:

```
Controls → Checks → Trace → Session + UI Events → Snapshot
```

- **Controls** is where you create input.
- **Checks** proves the main invariants held.
- **Trace** shows what happened internally, in order.
- **Session + UI Events** shows what a live client would have received.
- **Snapshot** shows what the store recorded.

This order mirrors how you debug: inject stimulus, check invariants, trace the path, see the outputs.

---

## 9. Things to try

**Submit.** Default session and prompt. Look at the trace. Each step shows a piece of the path. Look at the snapshot. It shows the final state after all events were processed.

**Submit again with the same session.** Session metadata persists. Event outputs evolve. The SessionId is what tells the framework where the work belongs.

**Submit with a different session.** A fresh session is created. State from the first session does not mix with the second.

**Submit with a longer prompt.** More chunks, more trace entries, more accumulated text in the snapshot.

**Click Reset.** The lab returns to a clean state. Outputs clear. This isolates one scenario from the next.

**Export.** The system is designed to make its own behavior portable and inspectable.

---

## 10. What the checks prove

| Check | What it proves |
|-------|----------------|
| `sessionExists` | A session was found or created |
| `cursorAdvanced` | The store's cursor moved forward |
| `timelineProduced` | The timeline projection emitted entities |
| `uiEventsProduced` | The UI projection emitted events |

Each check points at a different subsystem. If one goes red, you know where to investigate.

---

## Key Points

- The Hub routes commands to handlers. It does not know what commands do.
- Handlers publish events. They do not return values or format output.
- One command produces multiple events. This mirrors real work: beginning, intermediate states, end.
- Ordinals define order, enable reconnect, support hydration.
- Both projections consume the same event stream. They answer different questions.
- The UI projection produces transient events for live clients. The timeline projection produces durable entities for the store.
- The hydration store records entities and tracks the cursor per session.
- Read the page in order: Controls → Checks → Trace → Session + UI Events → Snapshot.

---

## API Reference

- **`Hub.Submit(...)`**: Route a command to its handler.
- **`RegisterCommand(...)`**: Register a handler for a command name.
- **`RegisterUIProjection(...)`**: Register the UI projection.
- **`RegisterTimelineProjection(...)`**: Register the timeline projection.
- **`HydrationStore.Apply(...)`**: Apply entities and advance the cursor.
- **`HydrationStore.Snapshot(...)`**: Return current state for a session.
- **`HydrationStore.View(...)`**: Return a read-only view for projections.
- **`HydrationStore.Cursor(...)`**: Return the latest applied ordinal.

---

## File References

### Framework files

- `pkg/evtstream/hub.go` — command routing
- `pkg/evtstream/command_registry.go` — handler registration
- `pkg/evtstream/session_registry.go` — session lifecycle
- `pkg/evtstream/projection.go` — projection interfaces
- `pkg/evtstream/hydration.go` — store interface
- `pkg/evtstream/hydration/memory/store.go` — Phase 1 store

### Systemlab files

- `cmd/evtstream-systemlab/lab_environment.go` — Phase 1 lab setup
- `cmd/evtstream-systemlab/static/partials/phase1.html` — page layout
- `cmd/evtstream-systemlab/static/js/pages/phase1.js` — page behavior

### Tests

- `pkg/evtstream/hub_test.go`
- `pkg/evtstream/hydration/memory/store_test.go`
- `cmd/evtstream-systemlab/lab_environment_test.go`