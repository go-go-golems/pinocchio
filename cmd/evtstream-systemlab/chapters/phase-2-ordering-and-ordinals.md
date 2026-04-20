# Phase 2 — Ordering and Ordinals

## Welcome

Phase 2 is where the framework begins to feel less like a tidy in-process exercise and more like a true event-streaming system. If Phase 1 taught you that commands can lead to backend events and projections, Phase 2 teaches you something deeper: *where does ordering actually come from, and who should be trusted to assign it?*

That question sounds technical, but it is really about honesty in distributed systems. The moment backend events can travel through a bus boundary, you have to stop pretending that the handler's local call stack is the whole story. Publishing is no longer the same thing as consumption. Local sequence and durable sequence are no longer the same thing either. If you want a substrate that can eventually run across real infrastructure, you need to be very careful about who is allowed to stamp the final ordinal, and when.

This phase is the point where `evtstream` starts taking that question seriously. We add a real Watermill-backed publish/consume split, move ordinal assignment into the consumer, and build a Systemlab page that lets you watch the difference between publish-time and consume-time reality.

You should read this chapter with the Phase 2 page open. Click a scenario, read the trace, then come back to the explanation. The lab makes much more sense when you move back and forth between the prose and the behavior.

By the time you finish this chapter, you should understand:

- why publish-time ordinal zero is intentional,
- why the consumer owns final ordinal assignment,
- how stream ids influence ordinal derivation,
- how per-session ordering is represented in the framework,
- what the Phase 2 controls are actually testing,
- what a healthy run looks like,
- and what kinds of bugs this phase is specifically designed to expose.

---

## 1. The leap from Phase 1 to Phase 2

Phase 1 gave us a local shortcut. A handler published a backend event and the framework immediately projected it in the same local process. That was the right teaching move at the time, because it made the event model easy to understand. But it was always a temporary simplification.

The problem is that local publication hides an important architectural truth: in a real event-streaming system, the point where an event is *published* is not the same as the point where the framework *consumes* it and turns it into durable state or live UI output. Those are different moments, and in a distributed architecture they may happen in different places or even different processes.

Phase 2 makes that distinction explicit.

### What this phase adds

This phase adds:

- a Watermill-backed event publisher,
- a real consumer loop,
- schema-aware decode on consumption,
- consumer-side ordinal assignment,
- a transport-neutral `UIFanout` seam,
- a lab specifically designed to make ordering visible.

### What remains intentionally unfinished

Even now, we are still not doing everything:

- websocket transport is still a later phase,
- durable SQL persistence is still a later phase,
- the example chat backend is still a later phase.

That is okay. Phase 2 is focusing on one architectural truth at a time.

---

## 2. The core lesson of this phase

The most important sentence in Phase 2 is this one:

> The final ordinal belongs to the consumer, not the handler.

That is easy to repeat and surprisingly hard to preserve if you are used to single-process code.

Why should the consumer own ordinals? Because the consumer is the first place where the framework can speak honestly about the observed stream of events. The handler can create an event. It can publish an event. But once there is a bus involved, the handler is no longer the authoritative source of ordering for durable framework behavior.

That means the framework chooses a deliberately humble rule:

- when an event is published, its ordinal is zero,
- when the consumer sees that event, it derives the real ordinal,
- projections and hydration state use the consumer-assigned ordinal.

That separation is one of the most important architectural milestones in the entire framework.

---

## 3. Why publish-time ordinal zero is not a hack

A new intern often looks at publish-time `ordinal=0` and wonders whether that is just a temporary placeholder. It is a placeholder in a sense, but not in a sloppy way. It is a statement of design.

It is the framework saying:

> At publish time, we know what event is being sent, but we are not yet claiming final authority over where it belongs in the consumed session stream.

That is a remarkably healthy stance. It prevents handlers from quietly pretending that local intent is the same thing as durable order.

### The conceptual split

```text
Handler publishes event      -> ordinal = 0
Consumer receives event      -> ordinal = derived final value
Projections use final value  -> store and live outputs see the same consumed order
```

That split is what lets the framework grow into a real bus-backed system later.

---

## 4. The Watermill bus in this phase

The framework now uses a real Watermill publisher/consumer boundary. For the current implementation, Systemlab uses a `gochannel` backend. That is a good stepping stone because it introduces the bus abstraction without requiring external infrastructure.

### Why `gochannel` first

`gochannel` is useful because it lets us prove the architecture locally while still teaching the right bus model. We can validate:

- publish/consume separation,
- message metadata,
- decode and schema validation,
- consumer-side ordinal assignment,
- projection behavior after consumption,
- output fanout after projection.

What `gochannel` does *not* do is magically solve distributed ordering for us. That is why this phase also records the partition key intent explicitly in metadata keyed by `SessionId`.

### File references

Read:

- `pinocchio/pkg/evtstream/bus.go`
- `pinocchio/pkg/evtstream/consumer.go`
- `pinocchio/pkg/evtstream/ordinals.go`
- `pinocchio/pkg/evtstream/fanout.go`

These files are where the clean local Phase 1 story becomes an honest bus-backed story.

---

## 5. Topic and partitioning rules

Even though the current Systemlab implementation uses a single Watermill topic, the framework still makes the partitioning intent explicit. That intent lives in metadata under the partition key derived from `SessionId`.

Why does that matter? Because ordering is only meaningful if you know *what it is ordered within*. In this framework, the answer is very clear:

- ordering matters per session,
- routing matters per session,
- hydration state is tracked per session,
- reconnect logic later will also be per session.

So the relevant ordered domain is not "all events everywhere." It is "the stream of events for one `SessionId`."

This is one of those ideas that sounds obvious once you say it, but it needs to be said explicitly because distributed systems often become confusing precisely where their ordering scope becomes vague.

---

## 6. The ordinal assigner and the role of stream ids

One of the most interesting parts of this phase is the ordinal assigner. It tries to derive a stable ordinal from stream metadata when that metadata is available. In the current lab, the main case is Redis-style stream ids that look like:

- `1713560000123-4`

The framework turns that into a large integer ordinal that preserves both timestamp and sequence information.

### Why derive ordinals from stream ids at all?

Because stream ids carry real ordering information when the backend supports them. If the consumer can recover that ordering signal, the framework gains a more stable and more explainable notion of consumed order.

### What if stream ids are missing or invalid?

The framework still needs to move forward safely. So the assigner has a fallback mode:

- if a valid stream id exists, derive from it,
- otherwise, advance monotonically from current cursor state.

That makes the system robust in the face of incomplete metadata while still preserving the architectural preference for stream-aware ordinal assignment when available.

### File references

- `pinocchio/pkg/evtstream/ordinals.go`
- `pinocchio/pkg/evtstream/ordinals_test.go`

---

## 7. The consumer is where the architecture becomes authoritative

Phase 2 makes the consumer into one of the most important pieces of the whole substrate.

This is where the framework now does all of the following together:

- consume a bus message,
- decode the event envelope,
- validate schema expectations,
- look up or create the session,
- assign the final ordinal,
- build the current timeline view,
- run projections,
- apply timeline entities to the store,
- publish UI outputs through a fanout seam.

This is a big job, and that is why it is such an important file to read carefully.

### Conceptual pseudocode

```go
func handleMessage(msg) {
    ev := decode(msg)
    sess := sessions.GetOrCreate(ev.SessionId)
    ord := ordinals.Next(ev.SessionId, msg.Metadata)
    ev.Ordinal = ord

    view := store.View(ev.SessionId)

    uiEvents := uiProjection.Project(ev, sess, view)
    entities := timelineProjection.Project(ev, sess, view)

    store.Apply(ev.SessionId, ev.Ordinal, entities)
    fanout.PublishUI(ev.SessionId, ev.Ordinal, uiEvents)
}
```

That pseudocode leaves out details, but it captures the architectural center of gravity of the phase.

---

## 8. The `UIFanout` seam and why it matters already

One subtle but important addition in Phase 2 is the `UIFanout` seam. This seam is the consumer-side output mechanism for projected UI events.

At first glance, that might just sound like plumbing. In reality, it is preparing the framework for Phase 3.

The consumer should not know about websocket objects. It should not know about browser tabs. It should not know about connection lifetimes. It should only know that once projections have produced UI events, those events need to be forwarded outward through a well-owned seam.

That is what `UIFanout` is doing. It is creating the place where future live transport logic can attach without polluting the consumer with transport details.

This is an excellent example of how one phase can quietly make the next phase much cleaner.

---

## 9. The Phase 2 Systemlab page

The Systemlab page for this phase is one of the most educational pages in the app because it forces ordering to become visible instead of invisible.

You are not just clicking buttons and seeing final state. You are looking at the path from publish intent to consumed order.

### What the page shows

- scenario controls,
- invariant checks,
- a bus/consumer trace,
- message history with publish and consume metadata,
- per-session ordinal tables,
- fanout payloads,
- current snapshots.

This combination matters because it lets you compare:

- what the publisher thought it sent,
- what the consumer says it observed,
- what the session's final state became.

That is exactly the comparison you want in an ordering lab.

---

## 10. The main files for the Systemlab side

### Backend/lab state

- `pinocchio/cmd/evtstream-systemlab/phase2_lab.go`
- `pinocchio/cmd/evtstream-systemlab/server.go`
- `pinocchio/cmd/evtstream-systemlab/lab_environment_test.go`

### Browser-side files

- `pinocchio/cmd/evtstream-systemlab/static/partials/phase2.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase2.js`

This is the pair you should read if you want to understand how the page's controls map to the backend lab state.

---

## 11. Things to try in the controls

Keep the Phase 2 page open while you read this section.

### Try 1: `Publish A`

Use the defaults and click `Publish A`.

### What should happen

You should see:

- a control trace entry describing the action,
- a handler trace entry,
- a publish trace entry,
- a consume trace entry,
- one message in message history,
- one ordinal under session A,
- one snapshot entry for session A.

### What to pay attention to

Look carefully at the message history.

You should notice that:

- the publish side records `publishedOrdinal = 0`,
- the consume side records a large assigned ordinal string,
- the fanout and snapshot agree with the consumed value rather than the published placeholder.

That is the entire architectural lesson of the phase in one scenario.

---

### Try 2: `Publish B`

After publishing for session A, click `Publish B`.

### What should happen

You should now see:

- one entry under session B,
- a separate consumed ordinal for session B,
- no corruption of session A's prior state.

### What to pay attention to

This is where you begin to feel session isolation. Even though messages may be flowing through the same topic, the framework is still tracking ordering and snapshot state per `SessionId`.

---

### Try 3: `Burst A`

Use the default burst count and click `Burst A`.

### What should happen

You should get multiple publishes followed by multiple consumed events for session A.

### What to pay attention to

Pay close attention to this subtle point:

- the consumed order in the message history may not line up naively with the publish sequence number embedded in the synthetic stream ids,
- but the per-session ordinal list should still be monotonic.

This is a very important lesson. The framework is not promising a simplistic visual story like "third publish must always display before fourth publish in every panel." It is promising a more precise story: the consumer will assign a coherent, monotonic ordinal sequence for the session.

---

### Try 4: change `Stream Mode` to `missing`

Then run a publish or burst scenario.

### What should happen

The framework should still work.

### What to pay attention to

Now the ordinal assigner cannot derive from a stream id, so the system falls back to monotonic local advancement. This is a good scenario to use when you want to understand the fallback semantics and prove to yourself that the framework does not collapse if metadata is incomplete.

---

### Try 5: change `Stream Mode` to `invalid`

Then run a publish scenario.

### What should happen

The framework should again still work, and the checks should still pass.

### What to pay attention to

This is a useful reminder that input metadata can be malformed, and the framework has to remain sane when that happens.

---

### Try 6: `Restart Consumer`

Click `Restart Consumer` after a few messages have already been processed.

### What should happen

You should see a consumer restart trace entry and the lab should remain responsive afterward.

### What to pay attention to

In this phase, restart is still being exercised in-memory rather than with a fully durable SQL store. But it is already teaching the idea that the consumer has its own lifecycle and that consumption is not synonymous with publication.

---

### Try 7: `Reset Phase 2`

Click the reset control.

### What should happen

The Phase 2 lab state should be rebuilt, and the page should return to a clean state.

### What to pay attention to

Reset is a good way to isolate scenarios when you are trying to understand one ordering pattern at a time.

---

### Try 8: export the transcript

After a run, click:

- `Export JSON`
- `Export Markdown`

### What should happen

You should get a portable description of the run.

### What to pay attention to

Notice that ordinals are rendered as strings in the exported artifacts. That is not arbitrary. The values are large enough to exceed JavaScript's safe integer range, so the lab deliberately renders them in a lossless format.

That is a small detail with big teaching value: correctness sometimes depends on how data is *shown*, not just how it is computed.

---

## 12. What a healthy trace looks like

A healthy Phase 2 trace typically includes events like:

1. consumer started,
2. control action requested,
3. session created (on first reference),
4. handler invoked,
5. event published,
6. event consumed.

If you run a burst scenario, you will see multiple publish and consume entries.

The trace is useful because it separates roles clearly:

- controls generate input intent,
- handlers publish canonical events,
- consumers assign final ordering and drive projections.

The page is teaching you to see those as different layers of responsibility.

---

## 13. What the checks are trying to prove

The checks panel condenses a lot of architecture into a few simple badges.

### `publishOrdinalZero`
Proves that the published form does not claim final ordinal authority.

### `monotonicPerSession`
Proves that consumed order remains coherent within each session.

### `sessionIsolation`
Proves that sessions are not being collapsed into one shared ordering bucket.

### `messagesConsumed`
Proves that the consumer really did process messages rather than the page merely reflecting a publish attempt.

If you can explain why each one matters, you have understood the heart of Phase 2.

---

## 14. The JavaScript precision lesson

One of the most interesting practical lessons in this phase has nothing to do with Watermill itself. It has to do with the browser.

The consumer-side ordinals derived from Redis-style stream ids are intentionally large. They are large enough that JavaScript cannot represent them precisely as ordinary numbers without rounding.

That created a subtle but important UI issue during development: the backend values were correct, but the browser was displaying rounded versions. That is exactly the kind of bug that can destroy trust in an ordering lab.

The fix was to render browser-facing ordinals as strings.

This is a wonderful reminder that correctness is end-to-end. If the backend is right but the teaching UI is wrong, the intern still learns the wrong thing.

---

## 15. Important API references

The most important Phase 2 APIs and files are:

### Framework side

- `WithEventBus(...)`
- `WithUIFanout(...)`
- `Run(...)`
- `Shutdown(...)`
- `OrdinalAssigner.Next(...)`
- `PartitionKeyForSession(...)`

### File references

- `pinocchio/pkg/evtstream/bus.go`
- `pinocchio/pkg/evtstream/consumer.go`
- `pinocchio/pkg/evtstream/ordinals.go`
- `pinocchio/pkg/evtstream/fanout.go`
- `pinocchio/pkg/evtstream/hub.go`

### Systemlab files

- `pinocchio/cmd/evtstream-systemlab/phase2_lab.go`
- `pinocchio/cmd/evtstream-systemlab/static/partials/phase2.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase2.js`

---

## 16. Common mistakes to avoid in this phase

### Mistake 1: assigning final ordinals in handlers

That undermines the architecture immediately.

### Mistake 2: forgetting that publish and consume are different moments

If you blur those together mentally, the whole reason for the bus boundary disappears.

### Mistake 3: ignoring the scope of ordering

The framework is primarily trying to preserve monotonicity per session, not invent one global total order for all activity everywhere.

### Mistake 4: coupling the consumer to transport concerns

The consumer should publish through `UIFanout`, not reach into websocket details.

### Mistake 5: trusting browser numbers for huge ordinals

If the UI rounds a value, the lab becomes misleading even if the backend is correct.

---

## 17. How a reviewer should read the code

A good reading order for Phase 2 is:

1. `pinocchio/pkg/evtstream/bus.go`
2. `pinocchio/pkg/evtstream/ordinals.go`
3. `pinocchio/pkg/evtstream/consumer.go`
4. `pinocchio/pkg/evtstream/hub.go`
5. `pinocchio/pkg/evtstream/bus_test.go`
6. `pinocchio/pkg/evtstream/ordinals_test.go`
7. `pinocchio/cmd/evtstream-systemlab/phase2_lab.go`
8. `pinocchio/cmd/evtstream-systemlab/static/partials/phase2.html`
9. `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase2.js`

That order walks you from bus plumbing, to ordering logic, to consumer orchestration, to lab presentation.

---

## 18. Final summary

Phase 2 is where the framework stops pretending that local call order is enough.

It introduces a real publisher/consumer split. It makes the consumer authoritative for final ordinals. It records the distinction between publish intent and consumed order. And it gives you a lab where you can watch those distinctions happen instead of merely reading about them.

That is why this phase matters so much. It is not only about adding Watermill. It is about teaching the framework how to be honest once there is a bus between cause and consequence.

---

## 19. File references at a glance

### Core framework files

- `pinocchio/pkg/evtstream/bus.go`
- `pinocchio/pkg/evtstream/consumer.go`
- `pinocchio/pkg/evtstream/ordinals.go`
- `pinocchio/pkg/evtstream/fanout.go`
- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/bus_test.go`
- `pinocchio/pkg/evtstream/ordinals_test.go`

### Systemlab files

- `pinocchio/cmd/evtstream-systemlab/phase2_lab.go`
- `pinocchio/cmd/evtstream-systemlab/static/partials/phase2.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/phase2.js`
- `pinocchio/cmd/evtstream-systemlab/chapters/phase-2-ordering-and-ordinals.md`

### Validation commands

```bash
cd /home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio

./.bin/golangci-lint run ./pkg/evtstream/... ./cmd/evtstream-systemlab/...
go test ./pkg/evtstream/... ./cmd/evtstream-systemlab/...
make evtstream-check
```
