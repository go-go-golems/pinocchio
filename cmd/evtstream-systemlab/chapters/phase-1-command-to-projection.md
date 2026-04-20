# Phase 1 — Command to Event to Projection

## Who this chapter is for

This chapter is for a new intern who has already understood the Phase 0 shape and is now ready to understand the **first real executable path** in `evtstream`.

By the end of this chapter, you should understand:

- what `Hub.Submit(...)` actually does,
- how commands differ from backend events,
- why projections exist as a pair,
- how hydration state is built,
- how the Phase 1 Systemlab controls map to framework behavior,
- what you should expect to see when the lab is working correctly,
- what mistakes to avoid when extending this phase.

This chapter is intentionally practical. It includes:

- prose explanations,
- diagrams,
- bullet-point walkthroughs,
- pseudocode,
- API references,
- file references,
- a list of things to try in the controls,
- and guidance on what to pay attention to while using the page.

---

## 1. What Phase 1 adds on top of Phase 0

Phase 0 gave us the architecture skeleton.

Phase 1 gives us the **first real end-to-end path**.

That path is:

```text
Command
  -> handler
  -> backend event(s)
  -> UI projection
  -> timeline projection
  -> hydration store snapshot/view/cursor
```

This is the moment where `evtstream` stops being just a shape and becomes an executable framework.

### What makes Phase 1 important

Phase 1 proves all of the following at once:

- a command can be registered and dispatched,
- a session can be created lazily,
- a handler can publish canonical backend events,
- UI events can be derived from those backend events,
- timeline entities can also be derived from the same backend events,
- the hydration store can accumulate durable-ish state,
- the lab can inspect the whole path without cheating.

### What Phase 1 still does not do

Phase 1 is still intentionally local and synchronous.

It does **not** yet prove:

- real bus-based consumption,
- websocket subscriptions,
- reconnect sequencing,
- SQL durability,
- real application behavior such as chat.

That is okay. The goal of Phase 1 is to prove the event model clearly before the architecture becomes distributed.

---

## 2. The core mental model

The most important Phase 1 idea is this:

> handlers publish backend events, and projections turn those backend events into views.

That separation is the heart of the system.

### Why this matters

If handlers directly wrote UI output or directly mutated timeline state, we would lose the framework's central design benefit:

- one canonical backend stream,
- multiple derived consumers of that stream,
- inspectable and testable behavior.

### The canonical flow

```text
[Command]
   |
   v
[CommandHandler]
   |
   | publishes canonical backend events
   v
[Backend Event Stream]
   |                   \
   |                    \
   v                     v
[UIProjection]      [TimelineProjection]
   |                     |
   v                     v
[UI Events]         [Timeline Entities]
                           |
                           v
                    [HydrationStore.Apply]
```

A new intern should be able to explain this diagram without looking at the code.

---

## 3. The Phase 1 lab scenario

The current Systemlab Phase 1 lab is intentionally simple. It uses a tiny synthetic application flow rather than a real chat backend.

### Lab vocabulary

#### Command
- `LabStart`

#### Backend events
- `LabStarted`
- `LabChunk`
- `LabFinished`

#### UI events
- `LabMessageStarted`
- `LabMessageAppended`
- `LabMessageFinished`

#### Timeline entity
- `LabMessage`

### Why the lab is synthetic

This lab is not trying to be product behavior. It is trying to teach the framework model.

The synthetic flow makes the following points very visible:

- the handler does not directly emit UI output,
- one command may yield multiple backend events,
- the same backend events drive both UI and timeline projections,
- the hydration store accumulates a final entity that matches the visible UI history.

---

## 4. The main files you need to read

Read these files in order if you want to understand the Phase 1 implementation.

### Framework files

- `pinocchio/pkg/evtstream/command_registry.go`
- `pinocchio/pkg/evtstream/session_registry.go`
- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/hydration/memory/store.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/schema.go`

### Test files

- `pinocchio/pkg/evtstream/command_registry_test.go`
- `pinocchio/pkg/evtstream/session_registry_test.go`
- `pinocchio/pkg/evtstream/hub_test.go`
- `pinocchio/pkg/evtstream/hydration/memory/store_test.go`
- `pinocchio/pkg/evtstream/schema_test.go`

### Systemlab files

- `pinocchio/cmd/evtstream-systemlab/lab_environment.go`
- `pinocchio/cmd/evtstream-systemlab/lab_environment_test.go`
- `pinocchio/cmd/evtstream-systemlab/static/partials/phase1.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase1.js`

---

## 5. The internal building blocks of Phase 1

### 5.1 Command registry

The command registry maps a command name to a `CommandHandler`.

### What it is responsible for

- registration of handlers,
- duplicate-registration protection,
- lookup by name.

### What it is not responsible for

- session creation,
- projection behavior,
- store mutation,
- transport concerns.

### Why the split matters

A registry should be boring. Its job is lookup, not orchestration.

---

### 5.2 Session registry

The session registry owns lazy session creation.

### Key idea

A session comes into existence on first reference.

That means there is no separate "create session" product action in this phase. Instead, the system creates a session when a command references a `SessionId` for the first time.

### Important behavior

- sessions are cached in memory,
- metadata is created by `SessionMetadataFactory`,
- the same session should not be rebuilt repeatedly.

### Why this matters

Later phases will depend on this lazy creation behavior for:

- command paths,
- subscription paths,
- reconnect and hydration.

---

### 5.3 Hydration store

In Phase 1 the hydration store is in-memory.

### Key responsibilities

- `Apply(...)`
- `Snapshot(...)`
- `View(...)`
- `Cursor(...)`

### What each one means

#### `Apply(...)`
Apply timeline entities at a given ordinal and advance the cursor.

#### `Snapshot(...)`
Return a serialized view of current session state.

#### `View(...)`
Return a read-only timeline view used by projections.

#### `Cursor(...)`
Return the latest applied ordinal for that session.

### Important property

The store makes defensive copies of payloads so callers cannot mutate stored state accidentally through shared references.

That sounds like a small implementation detail, but it is actually a correctness property.

---

### 5.4 Hub

The `Hub` is the orchestration entrypoint.

In Phase 1 it is where the command path comes together.

### What `Hub.Submit(...)` does conceptually

```go
func Submit(ctx, sid, name, payload) {
    validate command payload type
    build Command
    lookup handler
    get or create Session
    call handler with publisher
}
```

### What the Phase 1 publisher does

In Phase 1, the publisher is still local rather than bus-backed.

That means:

- the handler publishes an `Event`,
- the Hub assigns a local ordinal,
- the Hub runs both projections,
- the Hub applies timeline entities to the store,
- the Hub returns control.

That local shortcut is acceptable in Phase 1 because the goal is to prove the event model, not distribution.

---

## 6. Why there are two projections

A new intern often asks:

> Why not just have one projection that updates everything?

Because the system has two different view targets with different jobs.

### UI projection

The UI projection answers:

- what should the live client see right now?

Its output is transient, client-facing, and optimized for live updates.

### Timeline projection

The timeline projection answers:

- what entity state should the hydration store remember?

Its output is store-facing and optimized for reconstructing session state later.

### Important rule

Both projections consume the **same backend event**.

That is the key architectural idea.

---

## 7. The actual Phase 1 lab flow

Here is the simplified flow the current lab runs.

### User input

The user enters:

- Session ID
- Prompt

and presses:

- `Submit`

### Command created

The page sends `LabStart` with payload:

```json
{
  "prompt": "hello from systemlab"
}
```

### Handler behavior

The handler:

1. creates a message id,
2. emits `LabStarted`,
3. splits the prompt into chunks,
4. emits `LabChunk` for each chunk,
5. emits `LabFinished`.

### Resulting projections

For each backend event:

- the UI projection emits a client-facing UI event,
- the timeline projection updates the synthetic `LabMessage` entity.

### Final state

The final store snapshot contains one `LabMessage` with:

- `messageId`
- `text`
- `status=finished`

---

## 8. Phase 1 page anatomy

The Systemlab page is intentionally arranged to teach the event path.

### Controls panel

This is where you submit the command.

### Checks panel

This shows whether the key invariants passed.

### Trace panel

This shows the chronological internal story.

### Session + UI Events panel

This shows:

- session metadata,
- emitted UI events.

### Hydration Snapshot panel

This shows the current durable-ish store result.

### Why this arrangement works

A good debugging page should let you answer four questions quickly:

1. what did I send?
2. what happened internally?
3. what did the client-facing layer emit?
4. what state ended up persisted?

The Phase 1 layout is built around exactly those questions.

---

## 9. Things to try in the controls

This section is the most practical part of the chapter. Use it while you are on the Phase 1 page.

### Try 1: the default happy path

Use:

- Session ID: `lab-session-1`
- Prompt: `hello from systemlab`
- click `Submit`

### What should happen

You should see:

- a session created,
- one handler invocation,
- multiple backend events represented in the trace,
- UI events for start, append, append, finish,
- a final snapshot with a finished `LabMessage`.

### What to pay attention to

Pay attention to the fact that:

- the handler is invoked once,
- but multiple events are published,
- and those multiple events drive both the UI and timeline sides.

---

### Try 2: reuse the same session id

Use the same Session ID again and change the prompt.

### What should happen

You should still get a coherent run, but you should notice:

- the session already exists conceptually,
- the message sequence continues,
- the store snapshot reflects the latest entities for that session.

### What to pay attention to

Look at whether the session metadata remains stable while the actual event stream changes.

---

### Try 3: use a different session id

Change Session ID to something like:

- `lab-session-2`

and click `Submit`.

### What should happen

You should get a fresh session and a fresh timeline state for that session.

### What to pay attention to

This is the easiest way to understand that routing is by `SessionId`, not by prompt text or browser tab.

---

### Try 4: use a longer prompt

Try:

- `explain why projections should consume canonical backend events`

### What should happen

You should see more meaningful chunked text in the trace and final entity.

### What to pay attention to

Look at how intermediate chunk events build toward the final timeline entity.

---

### Try 5: click `Reset`

After a run, click `Reset`.

### What should happen

The page outputs should clear and the lab environment should return to a clean state.

### What to pay attention to

Reset is useful for separating one scenario from another when debugging or demonstrating.

---

### Try 6: export the transcript

After a successful run, click:

- `Export JSON`
- `Export Markdown`

### What should happen

You should get a saved artifact representing the run.

### What to pay attention to

The export artifacts are not just convenience output. They are intended as:

- review artifacts,
- onboarding material,
- regression fixtures.

---

## 10. What to expect in the trace

The trace is one of the best teaching tools in the whole lab.

For a successful run, you should expect a sequence like this:

1. command submitted,
2. session created,
3. handler invoked,
4. UI projection emitted event,
5. timeline projection updated entity,
6. UI projection emitted next event,
7. timeline projection updated entity,
8. ...,
9. final UI event,
10. final timeline update.

### What this means conceptually

The trace is showing that the framework path is not:

```text
command -> final state directly
```

It is instead:

```text
command -> handler -> event stream -> projections -> state/views
```

That difference is the whole point of the framework.

---

## 11. What to expect in the checks

The Phase 1 page currently tracks whether key invariants are true.

### Important checks

#### `sessionExists`
A session was created or found.

#### `cursorAdvanced`
The store cursor moved forward.

#### `timelineProduced`
The timeline projection emitted durable-ish state.

#### `uiEventsProduced`
The UI projection emitted client-facing events.

### Why these checks matter

They are a compact summary that the whole command-to-projection path worked.

If any one of them fails, you know which stage to investigate first.

---

## 12. What to expect in the hydration snapshot

The hydration snapshot is the current store view for the session.

### In the default scenario

You should end up with one `LabMessage` entity containing something like:

- `messageId: msg-1`
- `text: hello from systemlab`
- `status: finished`

### Why this matters

The snapshot is the first hint of how reconnect and hydration will work later.

Even though Phase 1 is local and in-memory, the snapshot teaches you that the system is already building state intended for reuse beyond the immediate event moment.

---

## 13. The relationship between trace, UI events, and snapshot

A very important review skill is learning to compare these three panels together.

### Trace
Tells you the chronological internal story.

### UI events
Tells you what a live client would receive.

### Snapshot
Tells you what state the store believes is current.

### Healthy Phase 1 mental check

Ask yourself:

- does the UI event history tell the same story as the final snapshot?
- does the final snapshot look like the accumulated result of the streamed UI data?
- does the trace explain why both of those are true?

If the answer is yes, the phase is doing its job.

---

## 14. Important API references

### `Hub.Submit(...)`
Public programmatic command entrypoint.

### `RegisterCommand(...)`
Registers a command handler.

### `RegisterUIProjection(...)`
Registers the one UI projection for the hub.

### `RegisterTimelineProjection(...)`
Registers the one timeline projection for the hub.

### `HydrationStore.Apply(...)`
Applies timeline entities and advances cursor.

### `HydrationStore.Snapshot(...)`
Returns current state for a session.

### `HydrationStore.View(...)`
Returns the read-only view used by projections.

### `HydrationStore.Cursor(...)`
Returns the latest applied ordinal.

---

## 15. Suggested pseudocode for Phase 1

Here is the simplest possible conceptual pseudocode for the full path:

```go
func Submit(ctx, sid, cmdName, payload) {
    handler := commands.Lookup(cmdName)
    session := sessions.GetOrCreate(sid)
    handler(ctx, Command{...}, session, publisher)
}

func publisher.Publish(ev Event) {
    ev.Ordinal = nextLocalOrdinal(ev.SessionId)
    view := store.View(ev.SessionId)

    uiEvents := uiProjection.Project(ev, session, view)
    entities := timelineProjection.Project(ev, session, view)

    store.Apply(ev.SessionId, ev.Ordinal, entities)
}
```

This is intentionally simplified, but it captures the shape you need to keep in your head.

---

## 16. Common mistakes to avoid in Phase 1

### Mistake 1: letting handlers write UI output directly

Handlers should publish canonical backend events, not directly author UI state.

### Mistake 2: letting handlers mutate the store directly

That would bypass the projection model and collapse the architecture.

### Mistake 3: confusing UI events with backend events

They are related but not interchangeable.

### Mistake 4: treating the snapshot as just debugging output

The snapshot is an early version of the durable state story.

### Mistake 5: forgetting that tests are part of the architecture

The focused tests in this phase are teaching tools as much as correctness checks.

---

## 17. How a reviewer should read Phase 1 code

Use this reading order:

1. `pinocchio/pkg/evtstream/hub.go`
2. `pinocchio/pkg/evtstream/command_registry.go`
3. `pinocchio/pkg/evtstream/session_registry.go`
4. `pinocchio/pkg/evtstream/hydration/memory/store.go`
5. `pinocchio/cmd/evtstream-systemlab/lab_environment.go`
6. `pinocchio/cmd/evtstream-systemlab/static/partials/phase1.html`
7. `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase1.js`
8. `pinocchio/cmd/evtstream-systemlab/lab_environment_test.go`
9. `pinocchio/pkg/evtstream/hub_test.go`

That order moves from core orchestration -> state -> lab usage -> test evidence.

---

## 18. Review checklist for Phase 1

### Framework checklist

- [ ] Does `Submit(...)` route through a real registry/handler path?
- [ ] Is session creation lazy and stable?
- [ ] Do handlers publish events instead of writing UI/store state directly?
- [ ] Are UI and timeline projections separate consumers of the same backend event?
- [ ] Does the store cursor advance coherently?

### Systemlab checklist

- [ ] Do the controls exercise the real command path?
- [ ] Does the trace tell a believable story?
- [ ] Do exported transcripts match what the page shows?
- [ ] Does the snapshot match the final accumulated message state?

### Validation checklist

- [ ] `go test ./pkg/evtstream/... ./cmd/evtstream-systemlab/...`
- [ ] `make evtstream-check`

---

## 19. Short glossary for Phase 1

### Command
A typed request entering the substrate.

### Backend event
The canonical event shape emitted by handlers and consumed by projections.

### UI event
A client-facing event derived from the canonical backend event stream.

### Timeline entity
A durable-ish entity derived from backend events and stored for hydration.

### Cursor
The latest applied ordinal for a session's timeline state.

---

## 20. Final summary

Phase 1 is where the framework first proves its central promise:

- a command can create backend events,
- those backend events can drive both live UI and durable state,
- and the entire path can be inspected in one place.

If Phase 0 gave the project a map, Phase 1 gives it a working engine.

And just as importantly for a new intern, Phase 1 gives you a page where you can see the engine operate.

---

## File references at a glance

### Core framework files

- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/command_registry.go`
- `pinocchio/pkg/evtstream/session_registry.go`
- `pinocchio/pkg/evtstream/hydration/memory/store.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/schema.go`

### Systemlab files

- `pinocchio/cmd/evtstream-systemlab/lab_environment.go`
- `pinocchio/cmd/evtstream-systemlab/lab_environment_test.go`
- `pinocchio/cmd/evtstream-systemlab/static/partials/phase1.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase1.js`
- `pinocchio/cmd/evtstream-systemlab/static/js/api.js`

### Validation commands

```bash
cd /home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio

go test ./pkg/evtstream/... ./cmd/evtstream-systemlab/...
make evtstream-check
```
