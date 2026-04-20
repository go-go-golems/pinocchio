# Phase 1 — Command to Event to Projection

## Welcome

Phase 1 is the first point where the framework starts to *feel alive*. Up to this point, you have seen the architecture laid out, the vocabulary stabilized, the shell app separated from the framework, and the boundaries made explicit. All of that matters. But if you are a new intern, there is a good chance that part of you is still asking a very reasonable question:

> Yes, but what actually happens when I do something?

That is exactly what Phase 1 answers.

This is the phase where a command enters the substrate, a handler responds to it, backend events are published, projections derive multiple views from those backend events, and a hydration snapshot emerges as the accumulated result. It is the first time the system demonstrates the full internal story that later phases will preserve while making it more distributed, more durable, and more product-like.

This chapter is meant to be read while the Phase 1 page is open in another tab. Read a section, then try the matching control, then come back and connect what you saw in the UI with what the framework is actually doing. If you use it that way, the chapter will feel less like documentation and more like guided lab work.

By the end, you should understand:

- what `Hub.Submit(...)` really means,
- why handlers publish backend events instead of touching UI state directly,
- why there are two projections rather than one,
- how hydration state is built,
- what the controls are exercising,
- what outputs are expected on a healthy run,
- what details deserve your attention when debugging.

---

## 1. What changes when Phase 1 arrives

Phase 0 gave us a map. Phase 1 gives us a working path through that map.

Before Phase 1, the architecture existed in shape and in naming. After Phase 1, the framework can actually perform its first real job. It can accept a typed command, route it to a handler, let that handler publish canonical backend events, and then use projections to build both live-facing and durable-ish views of the resulting activity.

That shift is subtle but profound. Once this phase works, you no longer have to imagine how the architecture is supposed to feel. You can submit a command and watch the pieces move.

### The full conceptual path

```text
Command
  -> handler
  -> backend event(s)
  -> UI projection
  -> timeline projection
  -> hydration store snapshot/view/cursor
```

If you can understand that sentence deeply, you can understand most of the framework's later phases.

---

## 2. The central idea of the framework becomes real here

The most important idea in the framework is not "commands exist" or "snapshots exist" or even "projections exist." The most important idea is that **canonical backend events sit in the middle of the architecture**.

That means handlers are not supposed to directly paint UI output. They are not supposed to directly mutate hydration state either. Instead, handlers publish canonical backend events that describe what happened. Then the framework uses projections to derive different views from those events.

This is easy to say in theory, but Phase 1 is where you get to see why it matters in practice.

Once backend events are the truth in the middle, the framework gains several benefits:

- the internal model becomes easier to reason about,
- multiple consumers can derive different views from the same source,
- state becomes easier to inspect,
- reconnect and hydration have a consistent conceptual basis,
- examples and tests become more teachable.

If instead handlers skipped that event layer and directly mutated everything they needed, the system might still *work*, but it would stop being the framework we are trying to build.

---

## 3. The Phase 1 lab is intentionally synthetic

The current Phase 1 Systemlab page does not try to be a real product backend. It is deliberately synthetic. That is a good thing.

New engineers sometimes feel disappointed when the first working example is not a real chat system or a production-like workflow. But that disappointment usually fades once they realize what the synthetic lab is buying us: clarity.

The lab flow is intentionally small enough that you can keep the whole thing in your head.

### Lab-specific vocabulary

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

This synthetic naming helps isolate the framework pattern from product semantics. Instead of immediately mixing in inference engines, models, tokens, cancellation semantics, and product rules, the lab teaches the event path itself.

---

## 4. The files that matter most in this phase

If you want to understand Phase 1 from the code, there is a reading order that works much better than randomly hopping around the repo.

### Framework files

Start with:

- `pinocchio/pkg/evtstream/command_registry.go`
- `pinocchio/pkg/evtstream/session_registry.go`
- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/hydration/memory/store.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/schema.go`

### Systemlab files

Then read:

- `pinocchio/cmd/evtstream-systemlab/lab_environment.go`
- `pinocchio/cmd/evtstream-systemlab/static/partials/phase1.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase1.js`

### Tests

Then use the tests as a second explanation layer:

- `pinocchio/pkg/evtstream/command_registry_test.go`
- `pinocchio/pkg/evtstream/session_registry_test.go`
- `pinocchio/pkg/evtstream/hub_test.go`
- `pinocchio/pkg/evtstream/hydration/memory/store_test.go`
- `pinocchio/cmd/evtstream-systemlab/lab_environment_test.go`

This order matters because the tests make more sense once you have seen the intended runtime path, and the lab code makes more sense once you understand the substrate pieces it is exercising.

---

## 5. The command path, explained like a story

Let us walk through the story that begins when you click `Submit` on the Phase 1 page.

You type a `Session ID` and a `Prompt`. The browser sends a request to the Systemlab backend. That backend takes your request and calls `Hub.Submit(...)` with a typed payload representing the command.

At that point, the framework starts doing the work it was designed for.

First, the hub validates that the command name is registered and that the payload type matches the schema the registry expects. Next, it looks up or lazily creates the session associated with that `SessionId`. Once it has both the handler and the session, it invokes the handler and gives it an `EventPublisher`.

The important part comes next. The handler does *not* return a final UI object. It does *not* directly insert a final entity into the store. Instead, it publishes backend events describing the unfolding activity.

That is the moment where the framework's architecture starts to make sense. The handler says, in effect: "Here is what happened." The rest of the framework then asks: "How should that be projected for live clients?" and "How should that be represented in durable state?"

That is the core separation of concerns in a single paragraph.

---

## 6. The command registry: simple on purpose

The command registry is one of those pieces that is easy to overlook because it feels so modest. But its modesty is a feature, not a flaw.

A command registry should not try to be clever. It should register handlers, protect against duplicate names, and return the right handler for a command name. That is all.

Why is it useful to keep it that small? Because the moment the registry starts owning more policy than lookup, the orchestration logic becomes harder to reason about. You want the registry to stay boring so the hub can stay readable.

### Responsibilities

- register a handler by command name,
- reject duplicate registration,
- return a handler by lookup.

### Non-responsibilities

- session creation,
- store mutation,
- UI projection,
- hydration behavior,
- transport logic.

When you review code in this repo, learning to love these small, well-owned pieces will help you avoid creating giant “god objects” later.

---

## 7. Lazy session creation and why it feels subtle at first

The session registry is another important Phase 1 building block. It creates sessions lazily. That means there is no special product-level "create session" flow in this phase. A session simply comes into existence when it is first referenced.

That may seem unremarkable, but it is actually laying the foundation for much later behavior. Commands, subscriptions, reconnects, and hydration all become simpler if a session can be created on first contact rather than requiring a separate lifecycle dance first.

The session registry also owns the use of `SessionMetadataFactory`. In practice, that means the framework can attach session metadata on first reference and keep it cached for later use.

You should pay attention to this because it is one of those patterns that seems almost too small to notice when it works well. But if it were missing, many later features would feel awkward and over-engineered.

---

## 8. The in-memory hydration store is already teaching future durability

Even though Phase 1 only uses an in-memory hydration store, it is already teaching the system a durable-state shape.

That store exposes the following semantics:

- `Apply(...)`
- `Snapshot(...)`
- `View(...)`
- `Cursor(...)`

Those names matter because they are exactly the kind of semantics later persistent stores will need to preserve.

### How to think about each one

#### `Apply(...)`
This says: given timeline entities and an ordinal, update the store and advance the cursor.

#### `Snapshot(...)`
This says: show me current session state in a serialized, inspectable form.

#### `View(...)`
This says: give projections a read-only view of current state so they can compute the next result.

#### `Cursor(...)`
This says: tell me the latest applied ordinal for that session.

Even in Phase 1, these operations are already nudging your mental model toward reconnect and restart behavior. That is why it is a mistake to think of the snapshot panel as "just debugging output." It is the earliest visible form of the hydration story.

---

## 9. The hub is where the framework's personality emerges

The `Hub` is the orchestration entrypoint, and in Phase 1 it is where the framework's personality really starts to become visible.

If you want to understand the architecture emotionally rather than just mechanically, `Hub.Submit(...)` is a very good place to focus. It is where the command path becomes a disciplined internal workflow.

### Conceptual pseudocode

```go
func Submit(ctx, sid, name, payload) {
    validate payload type
    cmd := Command{...}
    handler := commands.Lookup(cmd.Name)
    session := sessions.GetOrCreate(cmd.SessionId)
    handler(ctx, cmd, session, publisher)
}
```

In Phase 1, the publisher is still local rather than bus-backed. So when the handler publishes an event, the hub assigns a local ordinal, runs both projections, applies timeline entities to the store, and returns.

That is still simpler than the later bus-backed path, but it already proves the core conceptual shape. And that is why this phase matters so much.

---

## 10. Why there are two projections, not one

This is one of the most important architectural conversations in the whole framework.

A new engineer often wonders whether it would not be simpler to just have one projection that updates everything. And yes, it would be simpler in the short term. It would also be much harder to teach, test, and extend.

The framework separates projection responsibilities because they answer different questions.

### The UI projection asks:

> What should a live client see right now?

Its outputs are UI events. These are transient, client-facing, and optimized for live delivery.

### The timeline projection asks:

> What entity state should the hydration store remember?

Its outputs are timeline entities. These are store-facing, stateful, and designed to support later hydration and reconnect behavior.

What makes this elegant is that both of those answers come from the same backend event stream. That is the whole point.

---

## 11. The actual synthetic flow in the lab

The current Phase 1 lab behavior is small enough to keep in your head, which makes it perfect for learning.

### What the handler does

When you submit `LabStart`, the handler:

1. allocates a message id,
2. publishes `LabStarted`,
3. splits the prompt into chunks,
4. publishes one `LabChunk` event per chunk,
5. publishes `LabFinished`.

That means one command produces multiple backend events.

This is exactly the kind of thing the framework needs to handle well. Real application flows rarely map one command to one final result. The intermediate stream is often the interesting part.

### What the UI projection does

It translates:

- `LabStarted` -> `LabMessageStarted`
- `LabChunk` -> `LabMessageAppended`
- `LabFinished` -> `LabMessageFinished`

### What the timeline projection does

It reduces the same backend events into one synthetic timeline entity, `LabMessage`, whose content evolves as chunks arrive and is finally marked `finished`.

This is the first place in the project where you can feel the difference between stream and state.

---

## 12. How to read the page itself

The Phase 1 page is not arranged randomly. Its layout is trying to teach you a way of debugging.

### Controls panel

This is where you create the input stimulus.

### Checks panel

This answers: did the main invariants hold?

### Trace panel

This answers: what happened internally, in order?

### Session + UI Events panel

This answers: what session metadata exists, and what would a live client have seen?

### Hydration Snapshot panel

This answers: what does the store now believe is true for this session?

When you use the page properly, you should mentally travel from left to right and from top to bottom:

- input,
- checks,
- internal story,
- live-facing output,
- durable-ish state.

That is a very good way to debug event-streaming systems generally.

---

## 13. Things to try in the controls

This section is where the chapter becomes a real lab companion. Keep the Phase 1 page open while you try these.

### Try 1: the default happy path

Use:

- Session ID: `lab-session-1`
- Prompt: `hello from systemlab`
- click `Submit`

### What should happen

You should see:

- a session created,
- one handler invocation,
- multiple trace entries corresponding to the backend event path,
- multiple UI events,
- a final hydration snapshot with a finished `LabMessage`.

### What to pay attention to

Do not just look for success. Look for *shape*.

Pay attention to the fact that the command is singular but the resulting event story is plural. That is one of the first truly important lessons in the framework.

---

### Try 2: run again with the same session id

Keep the same Session ID and change the prompt.

### What should happen

You should get another coherent run associated with the same conceptual session.

### What to pay attention to

Watch whether the session metadata remains stable while the event and timeline outputs evolve. This helps you internalize that session identity is a routing/stability concept, not just a label pasted into output.

---

### Try 3: change the session id

Set Session ID to something like:

- `lab-session-2`

and click `Submit`.

### What should happen

A fresh session path should be created.

### What to pay attention to

This is one of the easiest ways to feel the role of `SessionId`. It is not decoration. It is what tells the framework where the work belongs.

---

### Try 4: use a longer, more expressive prompt

Try something like:

- `explain why projections should consume canonical backend events`

### What should happen

You should see longer accumulated text and more meaningful chunk progression.

### What to pay attention to

Observe how the final snapshot reflects the accumulated result of a sequence of backend events. That is exactly the pattern later chat and agent examples will rely on.

---

### Try 5: click `Reset`

After a run, click `Reset`.

### What should happen

The page outputs should clear, and the environment should return to a fresh state.

### What to pay attention to

Reset is especially useful when you are trying to understand the difference between one scenario and another. It removes the noise of prior state so you can isolate the current run.

---

### Try 6: export the transcript

After a successful run, click:

- `Export JSON`
- `Export Markdown`

### What should happen

You should get a portable artifact describing the run.

### What to pay attention to

These exports are not merely nice-to-have buttons. They are part of how the lab becomes teaching material and review material. The system is designed to make its own behavior portable and inspectable.

---

## 14. What the trace is really showing you

The trace panel is one of the most valuable parts of the page, and it is worth reading slowly.

In a healthy run, you will see a story something like this:

1. command submitted,
2. session created,
3. handler invoked,
4. UI projection emitted event,
5. timeline projection updated entity,
6. UI projection emitted next event,
7. timeline projection updated entity,
8. final UI event,
9. final timeline update.

That sequence is showing you that the framework is not a glorified reducer that jumps straight from command to final snapshot. It is showing you the *path* from command to event stream to views.

The trace is where the architecture becomes visible.

---

## 15. What the checks are trying to summarize

The checks panel compresses the health of the run into a few focused statements.

### The key checks

#### `sessionExists`
A session was found or created.

#### `cursorAdvanced`
The hydration store cursor moved forward.

#### `timelineProduced`
The timeline projection emitted durable-ish state.

#### `uiEventsProduced`
The UI projection emitted live-facing events.

### Why these matter

These checks are useful because each one points at a different part of the architecture. If one goes red later, you immediately know which subsystem to investigate first.

That is why even a simple Phase 1 page benefits from explicit invariant badges.

---

## 16. How to read the snapshot panel correctly

The hydration snapshot panel is easy to underappreciate if you are focused only on the event trace. But the snapshot is where the framework starts teaching you about eventual reconnect and state recovery.

In the default scenario, you should end with one `LabMessage` entity containing something like:

- `messageId: msg-1`
- `text: hello from systemlab`
- `status: finished`

The important thing is not just the final text. The important thing is that the snapshot looks like the accumulated result of the earlier stream.

When the event trace, the UI events, and the snapshot all tell the same story, the framework is healthy.

---

## 17. The three-panel comparison skill you should develop

One of the best habits you can build as an intern is learning to compare three kinds of output at once:

- the trace,
- the UI events,
- the final snapshot.

### Ask yourself these questions

- Does the UI event history make sense given the trace?
- Does the snapshot look like the accumulated result of those UI-visible updates?
- Does the trace explain why the snapshot became what it became?

If you can answer yes to all three, then you are no longer just reading output—you are understanding the architecture.

---

## 18. Important API references

The most important Phase 1 APIs to understand are:

### `Hub.Submit(...)`
The public programmatic command entrypoint.

### `RegisterCommand(...)`
Registers a handler for a command name.

### `RegisterUIProjection(...)`
Registers the UI projection.

### `RegisterTimelineProjection(...)`
Registers the timeline projection.

### `HydrationStore.Apply(...)`
Applies timeline entities and advances cursor.

### `HydrationStore.Snapshot(...)`
Returns current state for a session.

### `HydrationStore.View(...)`
Returns a read-only view used by projections.

### `HydrationStore.Cursor(...)`
Returns the latest applied ordinal.

You do not need to memorize their signatures immediately. You *do* need to understand their roles.

---

## 19. Suggested pseudocode for Phase 1

This pseudocode is not meant to be production-accurate in every line. It is meant to capture the mental shape of the framework.

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

What matters here is not the specific syntax. What matters is seeing the architecture as a chain of responsibilities rather than a black box.

---

## 20. Common mistakes to avoid in this phase

### Mistake 1: letting handlers write UI output directly

That would collapse the event model and make later transport logic much messier.

### Mistake 2: letting handlers mutate the store directly

That would bypass the projection discipline that the framework is trying to establish.

### Mistake 3: confusing backend events with UI events

They are related, but they do different jobs.

### Mistake 4: treating the snapshot as unimportant

The snapshot is the early form of a much larger hydration and reconnect story.

### Mistake 5: ignoring the tests because the page seems to work

The tests are not just correctness checks. They are another teaching surface for the intended architecture.

---

## 21. How a reviewer should read Phase 1

A good reading order for the code is:

1. `pinocchio/pkg/evtstream/hub.go`
2. `pinocchio/pkg/evtstream/command_registry.go`
3. `pinocchio/pkg/evtstream/session_registry.go`
4. `pinocchio/pkg/evtstream/hydration/memory/store.go`
5. `pinocchio/cmd/evtstream-systemlab/lab_environment.go`
6. `pinocchio/cmd/evtstream-systemlab/static/partials/phase1.html`
7. `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase1.js`
8. `pinocchio/cmd/evtstream-systemlab/lab_environment_test.go`
9. `pinocchio/pkg/evtstream/hub_test.go`

That reading order mirrors the architecture itself: core orchestration first, then lab usage, then evidence.

---

## 22. Review checklist for yourself

### Framework checklist

- [ ] Does `Submit(...)` route through a real registry/handler path?
- [ ] Is session creation lazy and stable?
- [ ] Do handlers publish backend events instead of mutating UI/store output directly?
- [ ] Are both projections consuming the same canonical backend event?
- [ ] Does the store cursor advance coherently?

### Systemlab checklist

- [ ] Do the controls exercise the real command path?
- [ ] Does the trace tell a believable internal story?
- [ ] Does the final snapshot match the accumulated event history?
- [ ] Do the exported transcripts match what the page shows?

### Validation checklist

- [ ] `go test ./pkg/evtstream/... ./cmd/evtstream-systemlab/...`
- [ ] `make evtstream-check`

---

## 23. Final summary

Phase 1 is the first time the framework truly demonstrates its central promise.

A command enters. A handler responds. Backend events become the canonical internal stream. UI and timeline projections each derive their own view from that stream. A hydration snapshot emerges that tells the same story in persistent form.

That is the conceptual engine of the framework.

Once you understand this phase, the later phases become much easier to reason about. Watermill changes *where* events are consumed. Websockets change *how* live updates are delivered. SQL changes *how durable state is kept*. But the core idea—canonical backend events projecting into multiple views—has already been taught here.

That is why this phase is such an important one to read carefully and to use hands-on.

---

## 24. File references at a glance

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
- `pinocchio/cmd/evtstream-systemlab/chapters/phase-1-command-to-projection.md`

### Validation commands

```bash
cd /home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio

go test ./pkg/evtstream/... ./cmd/evtstream-systemlab/...
make evtstream-check
```
