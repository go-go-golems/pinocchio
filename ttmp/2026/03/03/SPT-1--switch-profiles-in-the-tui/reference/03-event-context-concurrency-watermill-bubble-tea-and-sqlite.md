---
Title: 'Event context & concurrency: Watermill, Bubble Tea, and SQLite'
Ticket: SPT-1
Status: active
Topics:
    - tui
    - profiles
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/chat/model.go
      Note: Bubble Tea submit/finish semantics affecting cancellation
    - Path: pinocchio/cmd/switch-profiles-tui/main.go
      Note: Watermill router + gochannel pubsub configuration
    - Path: pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go
      Note: SQLite transaction semantics and single-writer behavior
    - Path: pinocchio/pkg/ui/timeline_persist.go
      Note: Handler context usage and SQLite upserts
    - Path: pinocchio/scripts/switch-profiles-tui-tmux-smoke.sh
      Note: tmux-driven concurrency reproduction harness
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T19:14:53.368639244-05:00
WhatFor: ""
WhenToUse: ""
---


# Event context & concurrency: Watermill, Bubble Tea, and SQLite

## Goal

Explain (for a new intern) how context propagation and concurrency actually work in the TUI’s event pipeline, why it was tricky, and what invariants/patterns make it reliable. This doc focuses on the parts that caused real-world flakiness: Watermill pubsub ACK semantics, Bubble Tea command execution timing, SQLite single-writer constraints, and how context cancellation interacts with persistence.

## Context

The `switch-profiles-tui` command is a Bubble Tea application that runs real streaming inference via Geppetto engines and uses Watermill to move events from the inference engine to UI + persistence handlers.

There are multiple “clocks” and “threads of execution” involved:

1) **Bubble Tea update loop** (single-threaded model updates; commands run in goroutines).
2) **Inference engine goroutines** (streaming tokens, publishing events).
3) **Watermill router goroutines** (handlers per topic/subscriber).
4) **SQLite writes** (transactions; single-writer locking).

The tricky part is that these subsystems have different notions of lifecycle, and Go `context.Context` cancellation can happen “legitimately” (shutdown) or “accidentally” (due to ack/teardown ordering), and persistence must not silently disappear in either case.

Key files:

- Router + pubsub wiring: `pinocchio/cmd/switch-profiles-tui/main.go`
- UI forwarder handler: `pinocchio/pkg/ui/step_chat_forward.go` (called via `ui.StepChatForwardFunc(program)`)
- Timeline persistence handler: `pinocchio/pkg/ui/timeline_persist.go`
- Bubble Tea chat model submit/finish: `bobatea/pkg/chat/model.go`
- SQLite timeline store: `pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go`

## Quick Reference

### End-to-end event pipeline (ASCII diagram)

```
Bubble Tea (UI)                              Watermill                      SQLite
----------------                           ----------                    ----------
user presses <tab>
  ↓ submit()
backend.Start(ctx, prompt)
  ↓
session.StartInference(ctx)
  ↓ (streaming)
provider engine publishes EventPartialCompletion/EventFinal as JSON
  ↓
middleware.WatermillSink.Publish(topic="chat")
  ↓
gochannel pubsub delivers to router handlers:
  - ui-forward handler: decode event → program.Send(UIEntity*)
  - timeline-persist handler: decode event → timelineStore.Upsert(...)
  - (optional) debug handler
```

### “Contexts” to be aware of (table)

| Context | Where it comes from | Intended lifetime | Failure mode we saw |
|---|---|---|---|
| `ctx` passed to `backend.Start` | Bobatea chat model (`submit()`) | Per inference run | If canceled too early, persister may not flush turns |
| `groupCtx` passed to `router.Run` | `errgroup.WithContext` in main | Entire app run | Canceling shuts down router + handlers |
| `msg.Context()` in Watermill handlers | Watermill/router | Per message delivery | Can become canceled unexpectedly relative to persistence |
| SQLite operation context | What you pass into store methods | Should be bounded | If tied to `msg.Context`, writes can vanish |

### Safe patterns (copy/paste guidance)

**1) PubSub config for streaming**

If using Watermill’s in-memory `gochannel` in a streaming app, avoid ACK-coupled publish blocking:

```go
goPubSub := gochannel.NewGoChannel(gochannel.Config{
  OutputChannelBuffer:            256,
  BlockPublishUntilSubscriberAck: false,
}, watermill.NopLogger{})
```

Used in: `pinocchio/cmd/switch-profiles-tui/main.go`.

**2) Persist with a detached bounded context**

Don’t persist to SQLite using `msg.Context()` directly. Use a bounded detached context:

```go
persistCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
_ = store.Upsert(persistCtx, convID, version, entity)
```

Used in: `pinocchio/pkg/ui/timeline_persist.go`.

**3) Serialize SQLite writers**

If multiple goroutines can write to the same SQLite DB, add a single-writer gate:

```go
type lockedTimelineStore struct {
  chatstore.TimelineStore
  mu *sync.Mutex
}
func (s *lockedTimelineStore) Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV2) error {
  s.mu.Lock()
  defer s.mu.Unlock()
  return s.TimelineStore.Upsert(ctx, convID, version, entity)
}
```

Used in: `pinocchio/cmd/switch-profiles-tui/main.go`.

## Usage Examples

### Reproduce the tricky behavior (for learning)

**Baseline: run the smoke harness**

```bash
cd pinocchio
./scripts/switch-profiles-tui-smoke-and-verify.sh
```

This exercises the concurrency “hot spots”:
- streaming inference
- concurrent router handlers
- SQLite upserts
- profile switching and marker persistence

### Debugging checklist: “persistence missing”

If the UI shows an assistant response but DB checks fail:

1) Confirm inference ran:
   - turns DB should have non-empty `inference_id`
2) Confirm the router is running and handlers are attached:
   - `pinocchio/cmd/switch-profiles-tui/main.go` adds handlers:
     - `ui-forward`
     - `timeline-persist`
3) Check whether the failure is:
   - **turn persistence** missing → look at Bubble Tea completion cleanup (`bobatea/pkg/chat/model.go`)
   - **timeline persistence** missing → look at handler context usage (`pinocchio/pkg/ui/timeline_persist.go`)
   - **SQLite lock** errors → add serialization or a busy timeout (serialization is currently used)

## Deep Dive: why it was tricky

### 1) Watermill pubsub ACK semantics are easy to misuse in streaming

Watermill’s `gochannel` is convenient, but it’s still a queue with backpressure semantics.

If publish blocks waiting for subscriber ACK (or if channels are unbuffered/small), you can end up with:

- inference goroutine tries to publish a streaming event,
- publish blocks,
- the subscriber that would ACK is stalled (or not draining),
- inference can no longer progress.

This is especially likely in TUIs because:
- UI handlers often need to hop back into Bubble Tea (`program.Send(...)`) which can add latency.
- When you’re streaming tokens, the event rate can be high (partial completions).

**Design invariant**
- The event bus must not be a synchronization point that can stall inference.

**Implementation**
- Use buffering and non-blocking publish for the in-memory pubsub in `switch-profiles-tui`.

### 2) “Backend finished” is not always “all side-effects are done”

Bubble Tea UI transitions based on messages. In this architecture:

- UI completion state changes when the forwarder sees `EventFinal` / `EventError` and emits `BackendFinishedMsg`.
- But the backend also has side effects:
  - persisting turns
  - emitting final events, flushing sinks

We saw a race where:

1) `EventFinal` reaches UI and UI calls completion cleanup.
2) Cleanup calls `backend.Kill()` (cancel).
3) The inference pipeline/persister is still finishing, and gets canceled before it writes the final turn snapshot.

**Design invariant**
- UI cleanup should not cancel the backend after a “final” message unless the user explicitly requests interrupt/quit.

**Implementation**
- Remove `backend.Kill()` from completion cleanup (`bobatea/pkg/chat/model.go`).

### 3) Watermill message contexts are not persistence contexts

Watermill handlers receive `*message.Message` and can read `msg.Context()`. It is tempting to do:

```go
store.Upsert(msg.Context(), ...)
```

But `msg.Context()`:
- is scoped to the transport/handler lifecycle, not the app’s persistence guarantee,
- can become canceled based on ack/teardown ordering,
- may be canceled during shutdown while you still want best-effort final persistence.

**Design invariant**
- Persistence should be best-effort but robust; it must not be coupled to a transport message’s internal context.

**Implementation**
- Use a detached bounded context for SQLite writes.

### 4) SQLite is single-writer; concurrency must be explicit

Even if all writes are “small”, SQLite will return `SQLITE_BUSY` when:
- two goroutines begin transactions around the same time, or
- WAL checkpoints/locks overlap with a write transaction.

In this TUI we can have multiple writers:
- timeline persister (partials + finals)
- profile switch marker persistence

**Design invariant**
- “One writer at a time” per DB file in-process unless we have a deliberate retry/backoff strategy.

**Implementation**
- Serialize `Upsert` calls with a mutex wrapper store and a handler-level mutex.

## Practical guidance: how to reason about the system

### Step-by-step mental model

1) “User action” is a Bubble Tea event.
2) The backend starts inference and returns a command; the command runs in a goroutine.
3) The inference engine publishes events to Watermill. Publishing must not block inference.
4) Watermill handlers decode events and do side effects:
   - UI updates (forwarder)
   - persistence (timeline persister)
5) Persistence must:
   - not deadlock the event pipeline,
   - not disappear due to context cancellation,
   - not lose writes due to SQLite locking.

### Recommended invariants (print and tape to your monitor)

- Do not block inference on UI or persistence ACKs.
- Do not cancel inference during “completion cleanup” unless explicitly requested.
- Do not use `msg.Context()` as your SQLite write context.
- Do not let two goroutines write to the same SQLite file at the same time unless you built retries/locking.

## Related

- Design doc: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/design-doc/01-design-profile-switching-in-switch-profiles-tui.md`
- Postmortem: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/02-postmortem-profile-switching-in-switch-profiles-tui.md`
- Diary: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/01-investigation-diary.md`

## Related

<!-- Link to related documents or resources -->
