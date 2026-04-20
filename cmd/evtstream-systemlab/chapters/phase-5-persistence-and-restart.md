# Phase 5 — Persistence and Restart Correctness

## Welcome

By the time you arrive at Phase 5, the framework has already learned a lot. It has stable vocabulary. It has a command path. It has canonical backend events. It has projections. It has a bus-backed consumer. It has an ordering model. It even has the beginnings of a live transport and a concrete example story.

But all of that can still feel a little fragile until the system proves that it can survive a restart without losing its place.

That is why Phase 5 matters so much.

This is the phase where hydration stops being merely a conceptual interface and becomes a durable promise. The system should be able to shut down, come back, and resume from the right cursor with the right timeline state. A reconnecting client should not get duplicated or skipped ordinals just because the process restarted. The framework should not act like memory is truth when the real goal is recoverable state.

A lot of distributed systems feel convincing until the first restart. Phase 5 is where `evtstream` tries to earn trust under that more serious condition.

This chapter explains the motivation, the model, the planned controls, and the mistakes to avoid. Even if the full page is still evolving, you should read this chapter as preparation for one of the most important correctness phases in the whole project.

---

## 1. Why persistence changes the emotional feel of the system

Before persistence, a system can still be useful. It can still stream. It can still project. It can still reconnect in the same process. But it always carries a hidden fragility: somewhere in the back of your mind, you know the system's memory is also its confidence.

Phase 5 changes that.

Once the hydration store becomes SQL-backed and restart-aware, the framework can stop behaving like a clever in-memory demonstration and start behaving like a system that believes its own state across process lifetimes.

That shift matters psychologically as much as technically. It is one thing to trust a system while it remains warm. It is another thing to trust it after it has stopped and started again.

---

## 2. The central lesson of this phase

The most important idea in Phase 5 is this:

> Cursor state and timeline state must survive restart together, or the framework cannot honestly promise reconnect correctness.

That sentence is doing a lot of work.

If the timeline entities survive but the cursor does not, the framework may reuse ordinals incorrectly.

If the cursor survives but the entities do not, the framework may believe it is further along than the visible session state suggests.

If they survive independently without transactional discipline, the restart story becomes ambiguous and bugs become incredibly hard to reason about.

So Phase 5 is not just "store stuff in SQL." It is about making `Apply(...)`, `Snapshot(...)`, `View(...)`, and `Cursor(...)` mean the same thing durably that they already meant in memory.

---

## 3. Why this phase comes after the earlier ones

This phase would have been premature in Phase 1, and even in Phase 2 it would have been harder to explain well.

Why? Because durable restart only becomes meaningful once these earlier truths already exist:

- the framework has canonical backend events,
- the consumer owns final ordinals,
- the hydration store already has a clear interface shape,
- reconnect semantics are already conceptually tied to snapshots.

Persistence does not replace those ideas. It strengthens them.

That is why this phase belongs here rather than earlier. The system first learns what state means, then learns what it means to keep that state through restarts.

---

## 4. What a SQL hydration store is really promising

At a shallow level, a SQL hydration store promises that data will be written to a database. That is true but incomplete.

At the framework level, it is promising something more nuanced:

- `Apply(...)` should advance state and cursor atomically,
- `Snapshot(...)` should represent a coherent current view,
- `View(...)` should still support projection logic correctly,
- `Cursor(...)` should let the consumer resume ordinal assignment after restart.

That means a SQL implementation is only good if it preserves semantics, not merely data.

A new engineer should keep this in mind whenever they read persistent-store code in a framework. The hard part is rarely just schema definition. The hard part is preserving behavioral meaning.

---

## 5. The durable cursor is the quiet hero of the phase

If there is one concept that deserves more respect than it usually gets, it is the cursor.

The cursor may look like a tiny field compared to the richness of timeline entities or the drama of live events. But the cursor is what lets the framework know where the durable system believes it currently stands for a given session.

When the process restarts, the consumer needs to know:

- what ordinal has already been durably applied,
- where the next consumed event should continue,
- whether it is resuming cleanly or drifting.

Without that durable cursor, restart correctness becomes hand-wavy. With it, the framework has a real resume point.

That is why Phase 5 treats cursor preservation as a first-class correctness property rather than as a trivial implementation detail.

---

## 6. The memory-vs-SQL comparison story in Systemlab

The planned Systemlab page for this phase is especially important because it lets you compare two worlds side by side:

- memory-backed hydration,
- SQL-backed hydration.

That comparison is deeply useful for a new intern because it turns persistence into something observable rather than merely theoretical.

The page should let you see:

- pre-restart state,
- post-restart state,
- cursor before and after,
- entity snapshots before and after,
- divergence or convergence between memory and SQL modes.

This is not just a convenience page. It is one of the best ways to teach persistence semantics without forcing someone to reason only from code.

---

## 7. The planned controls and what they are meant to teach

The planned Phase 5 page includes controls like:

- storage mode toggle: memory vs SQL,
- seed session,
- restart backend,
- reconnect client,
- compare snapshots,
- possibly reset store.

At first glance, that might sound like an operational page. And it is. But it is also a conceptual one.

Each control is designed to expose a different part of the durability story.

### `Storage Mode`
Teaches that store semantics should remain stable across implementations.

### `Seed Session`
Creates a known state to compare later.

### `Restart Backend`
Forces the framework to prove that restart correctness is real and not aspirational.

### `Reconnect Client`
Links durable store semantics back to the live client story.

### `Compare Snapshots`
Makes divergence visible instead of leaving it hidden in logs.

---

## 8. Things to try once the controls are active

These scenarios are worth learning before the page is fully live, because they tell you what questions the page is trying to answer.

### Try 1: memory mode baseline

Seed a session in memory mode and inspect the resulting cursor and snapshot.

### What to pay attention to

This is your baseline. You are learning what the framework believes current state looks like before durable persistence is part of the equation.

---

### Try 2: SQL mode baseline

Now seed a comparable session in SQL mode.

### What to pay attention to

The main question is not whether SQL stores data at all. The question is whether the resulting snapshot and cursor semantics look the same as in memory mode.

This is one of the most important lessons of the phase:

> implementation changes are allowed, semantic drift is not.

---

### Try 3: restart after state exists

Create meaningful state, then restart the backend.

### What should happen

After restart, the SQL-backed store should still report the same session state and cursor.

### What to pay attention to

This is the emotional heart of the phase. Does the system still feel like the same system after restart, or does it behave like it has forgotten what it was doing?

---

### Try 4: reconnect after restart

Reconnect a client after the restart.

### What should happen

The client should hydrate from current state and continue without duplicate or skipped ordinals.

### What to pay attention to

This is where persistence and transport stop being separate ideas. Durable hydration exists to make reconnect meaningful across process lifetimes.

---

### Try 5: compare memory and SQL output

Use the compare view or comparison badges.

### What should happen

You should be able to see whether the two implementations agree or diverge.

### What to pay attention to

If they diverge, the problem is not necessarily that SQL is broken or memory is broken individually. The deeper issue is that the framework's semantics are no longer implementation-stable.

---

## 9. Why transactional `Apply` matters so much

A SQL hydration store that applies entities first and cursor later, or cursor first and entities later, without treating them as one transactional unit, is asking for trouble.

Why? Because a crash can happen in the middle.

If the store survives with:

- entities updated but cursor not advanced,
- or cursor advanced but entities not updated,

then the restart story becomes incoherent. The consumer may continue from a point that does not match the visible state, or visible state may appear older than the framework's own cursor position.

That is why the phase emphasizes atomic `Apply + cursor advance` semantics. This is not database purism. It is framework correctness.

---

## 10. How restart correctness connects back to earlier phases

One of the most satisfying things about this framework is that later phases often illuminate the value of earlier ones.

Phase 5 is a perfect example of that.

- Phase 1 gave us the shape of the hydration store.
- Phase 2 made the consumer authoritative for ordinals.
- Phase 3 clarified snapshot-before-live thinking.
- Phase 5 now asks whether those semantics survive restart.

Seen this way, persistence is not a bolt-on feature. It is the durability test of ideas we have already been building toward from the beginning.

---

## 11. The kinds of bugs this phase is designed to prevent

### Bug class 1: duplicate ordinals after restart

The consumer resumes incorrectly and restamps work that should already be considered applied.

### Bug class 2: skipped ordinals after restart

The cursor moves ahead of actual durable state.

### Bug class 3: entity state preserved but cursor lost

The framework now has a believable snapshot and an unbelievable ordering position.

### Bug class 4: cursor preserved but entity state lost

The framework believes it is further along than the visible state suggests.

### Bug class 5: memory and SQL implementations diverge semantically

This is one of the hardest classes of bugs because both implementations may appear locally reasonable while the framework as a whole becomes inconsistent.

---

## 12. The likely shape of the SQL store implementation

A durable SQL implementation will likely need:

- a per-session cursor table,
- a representation of timeline entities keyed by session, kind, and id,
- transactional `Apply(...)`,
- snapshot reconstruction,
- reset/bootstrap helpers for tests and labs.

What matters most is not the exact SQL schema shape but that the store preserves the meaning of the `HydrationStore` contract.

That is the right lens to use while reading the implementation.

---

## 13. The planned value of the Systemlab page

The best version of the Phase 5 page will make persistence feel visible and testable, not mystical.

The page should help an intern answer:

- what was true before restart,
- what is true after restart,
- which store mode is being used,
- whether the cursor survived,
- whether the entities survived,
- whether the next ordinal resumes correctly,
- whether reconnect behavior still looks coherent.

That is a very powerful set of teaching questions. It takes a topic that often gets trapped inside infrastructure code and turns it into something you can inspect with your own eyes.

---

## 14. Important file references and areas to watch

### Framework-side areas

- future SQL hydration store implementation under something like `pkg/evtstream/hydration/sqlite/` or similar
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/consumer.go`
- `pinocchio/pkg/evtstream/ordinals.go`

### Systemlab-side areas

- future Phase 5 backend lab file under `cmd/evtstream-systemlab/`
- future Phase 5 partial and JS page module
- this chapter file:
  - `pinocchio/cmd/evtstream-systemlab/chapters/phase-5-persistence-and-restart.md`

---

## 15. Common intern mistakes in this phase

### Mistake 1: treating SQL persistence as only a storage problem

It is also a semantic contract problem.

### Mistake 2: testing restart only manually

Manual testing is useful, but restart semantics should also be encoded in repeatable integration tests.

### Mistake 3: assuming memory mode defines truth and SQL mode merely approximates it

The real goal is semantic equivalence where the contract overlaps.

### Mistake 4: underestimating cursor correctness

Cursor handling is often where restart bugs quietly originate.

### Mistake 5: treating reconnect after restart as a separate story from persistence

In this framework, they are deeply connected.

---

## 16. Final summary

Phase 5 is where the framework tries to prove that its idea of truth survives interruption.

A system that only works while it is warm is still useful, but it is not yet deeply trustworthy. A system that can stop, start, recover its state, and resume ordinals coherently is beginning to earn a different level of confidence.

That is what this phase is trying to build.

It is not merely about SQL. It is about continuity.

And continuity, in this framework, means that timeline entities, cursors, reconnect semantics, and consumer behavior all continue telling the same story even after the process itself has gone away and returned.

---

## 17. File references at a glance

### Substrate references

- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/consumer.go`
- `pinocchio/pkg/evtstream/ordinals.go`
- future SQL hydration store implementation files

### Systemlab references

- planned Phase 5 partial and JS page module
- `pinocchio/cmd/evtstream-systemlab/chapters/phase-5-persistence-and-restart.md`

### The main review question

At every point in this phase, ask:

> If the process dies right now and comes back, will the framework still tell the same truth about this session?
