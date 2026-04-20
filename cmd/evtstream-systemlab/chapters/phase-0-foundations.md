# Phase 0 — Foundations, API Skeleton, and Systemlab Shell

## Welcome

If you are a new intern on this project, Phase 0 is where you should slow down, breathe, and orient yourself. It is tempting to look at the later phases—live event streams, websocket transport, reconnect logic, durable state—and assume that the exciting work must start there. But in systems like this, the quality of the later phases depends almost entirely on whether the early foundations were laid with discipline.

That is what Phase 0 is about. It is the phase where we decide what kind of system we are building before we fill it with behavior. It is where we make the package boundaries real, where we decide what belongs to the reusable framework and what belongs to the teaching app around it, and where we deliberately choose a shape that a new engineer can still understand several phases later.

In other words: Phase 0 is not "just scaffolding." It is the part of the build where we try to prevent the future system from collapsing under its own convenience.

This chapter is written as a careful walkthrough for someone who is still getting their bearings. By the end, you should understand:

- what `evtstream` is trying to become,
- why it lives in `pkg/evtstream`,
- why Systemlab is a separate app,
- what this phase proves and what it intentionally does *not* prove,
- which files matter first,
- which ideas are merely implementation details, and which are architectural rules.

---

## 1. The story this framework is trying to tell

At a high level, `evtstream` is trying to become a reusable substrate for realtime, event-streaming applications. The phrase "event-streaming substrate" can sound abstract at first, so it helps to translate it into the kind of user experience we actually care about.

Imagine a client sends a command—perhaps a prompt, perhaps a start action, perhaps a control signal. That command should trigger work on the backend. While the backend is doing that work, it should publish canonical backend events that describe what is happening. Those events should be usable for at least two different purposes at the same time: they should drive live UI updates, and they should also build durable state that can later be used for hydration, reconnect, and recovery.

The framework is trying to make that pattern reusable.

That means it is trying to answer a question like this:

> How do we build systems where commands start work, backend events become the canonical internal stream, UI updates remain derived rather than primary, and reconnecting clients can recover cleanly without the entire application becoming tangled?

That is a lot to ask from a framework. Which is why Phase 0 is so careful. We are not trying to implement the whole dream here. We are trying to create a structure that can support the dream without turning into a pile of accidental product code.

---

## 2. What Phase 0 is really proving

Phase 0 exists to prove that we can start from the right architecture instead of backing into it later. A lot of systems begin with a quick prototype, then keep adding behavior until the prototype quietly becomes production code. That path is fast in the short term and expensive forever afterward.

This phase chooses the slower, healthier path. It says: before we add more runtime behavior, let's make sure the package tree, the seams, the terminology, and the consumer app all line up with the long-term plan.

### What Phase 0 gives us

Phase 0 establishes:

- a dedicated package home for the substrate,
- stable vocabulary for the core concepts,
- public interfaces that later phases can implement more deeply,
- a separate teaching/debugging app called Systemlab,
- explicit rules about what the lab is allowed to touch,
- repeatable validation commands that let reviewers verify the boundary.

### What Phase 0 does *not* try to do yet

It does **not** yet promise:

- a real distributed bus,
- websocket connections,
- reconnect semantics,
- SQL durability,
- chat-specific behavior,
- compatibility with legacy webchat.

This matters because when you first open the code, you may notice that some pieces feel intentionally small. That is not because the phase is unfinished in a sloppy way. It is because the phase is focused. The point here is not to impress you with runtime complexity. The point is to create a codebase you can still reason about once the complexity arrives.

---

## 3. Why `pkg/evtstream` had to be created

One of the most important design decisions in this project is that we did **not** simply rename `pkg/webchat` and call it the framework.

That older package contains valuable donor logic. It has good ideas. It has examples of real streaming behavior. It has code that is worth studying carefully. But it is not the same thing as a reusable clean-room substrate.

There are several reasons for that.

First, `pkg/webchat` is still shaped by the needs of webchat itself. It carries assumptions that make perfect sense inside that product context but would become baggage in a generic substrate. Second, it includes product-specific transport and message-shape concerns that we explicitly do not want to make canonical in the new framework. Third, using donor code directly as the substrate would blur a line that we want to keep very sharp: donor code is where we learn from prior work, but the substrate is where we define the cleaner abstraction.

So the project created a new package home:

- `pinocchio/pkg/evtstream`

That package is where the reusable framework vocabulary and seams live.

### The healthy mental model

When you think about the relationship between the old code and the new code, use this model:

```text
pkg/webchat  -> donor and later consumer/example
pkg/evtstream -> reusable substrate
```

That distinction will help you make better decisions when you later wonder where a helper, projection, or transport concept belongs.

### The first files to read

Start with:

- `pinocchio/pkg/evtstream/doc.go`
- `pinocchio/pkg/evtstream/types.go`
- `pinocchio/pkg/evtstream/handler.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/transport/transport.go`

These are the files where the framework begins by naming the things it cares about.

---

## 4. Why Systemlab is separate, and why that matters so much

A new intern often notices Systemlab and thinks of it as a fancy demo shell. That is not quite right. It is more important than that.

Systemlab exists as a **separate app** because a framework boundary is not really proven until something outside the framework uses it honestly. If the framework and the lab were blended together, we would always be at risk of subtle cheating. The lab would call helpers that no real consumer could call. It would reach into private internals just because that was convenient. It would accidentally turn teaching code into framework dependency.

Keeping Systemlab separate prevents that kind of drift.

Systemlab lives here:

- `pinocchio/cmd/evtstream-systemlab`

### What Systemlab is trying to be

Systemlab is all of these at once:

- a guided explainer,
- a debugging environment,
- a regression surface,
- an onboarding tool,
- a public-API exerciser.

That means it must be capable enough to be useful, but disciplined enough not to distort the architecture.

### The rule you should remember

Systemlab may:

- import public `evtstream` APIs,
- expose its own HTTP endpoints,
- render labs and explanations,
- simulate consumers of the framework.

Systemlab may not:

- import `pkg/webchat` internals,
- bypass public seams just because the code lives in the same repo,
- redefine framework ideas in lab-specific ways,
- smuggle product logic into the substrate.

If you ever feel tempted to make Systemlab "just a little more convenient" by reaching into framework internals, that is usually a sign you are about to damage exactly the thing Systemlab is meant to protect.

### File references

Read:

- `pinocchio/cmd/evtstream-systemlab/README.md`
- `pinocchio/cmd/evtstream-systemlab/main.go`
- `pinocchio/cmd/evtstream-systemlab/server.go`

---

## 5. The vocabulary Phase 0 teaches you

This phase is the first place where the project teaches you its core nouns. These words are not decorative. They are the language the framework will use for every later phase.

### `SessionId`

`SessionId` is the universal routing key.

This is one of the most important clean-room choices. Instead of scattering identity and routing across multiple overlapping notions, the framework makes one session identifier the center of gravity for:

- command routing,
- event ordering,
- hydration state,
- reconnect semantics,
- later subscription behavior.

That clarity becomes more valuable, not less, as the system grows.

### `ConnectionId`

`ConnectionId` identifies one transport-level connection. It is deliberately not the same thing as a session. A session may later have multiple connections attached to it, and a transport must be free to manage that without changing the business-level concept of a session.

### `Command`

A command is the typed request entering the framework. It is what a caller wants to do.

### `Event`

An event is the canonical backend event moving through the substrate. This is critical. The framework does not treat UI output as its primary internal form. It treats backend events as the truth and derives UI and hydration state from them.

### `UIProjection`

A `UIProjection` turns canonical backend events into client-facing UI events.

### `TimelineProjection`

A `TimelineProjection` turns those same canonical backend events into timeline entities that the hydration store can retain.

### `HydrationStore`

A `HydrationStore` is the persistence seam. In later phases it will matter for reconnect, snapshotting, and durable restart behavior. In Phase 0, it matters because we need the interface shape right before we need the implementation depth.

---

## 6. The Phase 0 directory map

It helps to look at the codebase as a map rather than as a pile of files.

```text
pinocchio/
├── pkg/
│   └── evtstream/
│       ├── doc.go
│       ├── types.go
│       ├── handler.go
│       ├── projection.go
│       ├── hydration.go
│       ├── schema.go
│       ├── hub.go
│       ├── noop_store.go
│       └── transport/
│           └── transport.go
│
├── cmd/
│   └── evtstream-systemlab/
│       ├── README.md
│       ├── main.go
│       ├── server.go
│       ├── chapters/
│       └── static/
│           ├── index.html
│           ├── app.css
│           ├── partials/
│           └── js/
│
└── Makefile
```

When you read that tree slowly, you can already see the intended ownership model.

- `pkg/evtstream` owns the substrate.
- `cmd/evtstream-systemlab` owns the teaching app.
- `Makefile` owns the repeatable validation entrypoints.

That separation is not just tidy—it is a statement of design.

---

## 7. The API skeleton and why it is intentionally boring

New engineers often underestimate how valuable a boring API skeleton can be. They want more runtime behavior, more features, more proof that the thing is alive. But stable names and stable seams are one of the best gifts you can give a growing system.

Phase 0 establishes those names early.

### API references

The primary API surfaces are defined in:

- `pinocchio/pkg/evtstream/types.go`
- `pinocchio/pkg/evtstream/handler.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/transport/transport.go`

### Conceptual sketch

```go
type SessionId string
type ConnectionId string

type Command struct {
    Name      string
    Payload   proto.Message
    SessionId SessionId
}

type Event struct {
    Name      string
    Payload   proto.Message
    SessionId SessionId
    Ordinal   uint64
}

type CommandHandler func(ctx context.Context, cmd Command, sess *Session, pub EventPublisher) error

type UIProjection interface {
    Project(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]UIEvent, error)
}

type TimelineProjection interface {
    Project(ctx context.Context, ev Event, sess *Session, view TimelineView) ([]TimelineEntity, error)
}
```

It is okay if this feels abstract the first time through. Phase 0 is where you learn the nouns. Phase 1 and later phases make those nouns feel alive.

---

## 8. The import-cycle lesson: architecture shows up in errors

One of the most useful lessons of Phase 0 came from an actual failure. At one point, the core package tried to default directly to the in-memory store implementation. That seems harmless at first glance. It even sounds convenient. But it produces a dependency graph like this:

```text
evtstream -> hydration/memory -> evtstream
```

That is an import cycle.

More importantly, it is a sign of architectural confusion.

The core package should own interfaces and shared types. Concrete implementations should depend on the core—not the reverse. So the fix here was not some clever Go trick. The fix was to return to the architecture and make the dependency direction honest.

The result was:

- `evtstream` defines the `HydrationStore` interface,
- `evtstream/hydration/memory` implements it,
- callers inject implementations with options,
- the core keeps a root-local noop fallback rather than depending on the memory implementation.

### Read these files together

- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/noop_store.go`
- `pinocchio/pkg/evtstream/hydration/memory/store.go`
- `pinocchio/pkg/evtstream/hub.go`

### Pseudocode for the right dependency shape

```go
store := memory.New()

hub, err := evtstream.NewHub(
    evtstream.WithHydrationStore(store),
)
```

and *not*:

```go
func NewHub(...) {
    store := memory.New() // wrong place: core depending on concrete implementation
}
```

This is a beautiful example of why early phases matter. If you solve a problem like this cleanly in Phase 0, the later phases inherit that clarity automatically.

---

## 9. Why the shell app matters even before the framework does much

Phase 0 could have stopped with `pkg/evtstream` and claimed success. But that would not have been enough. The whole point of a public API is that something outside the core should be able to consume it.

That is why the shell app exists even this early.

The shell page is intentionally simple. It is not trying to fake a full product. It is not trying to wow you with visuals. Instead, it is doing something much more important: it is proving that the separate app can exist, can compile, can mount pages, can show framework status, and can become the place where each future phase is explained and exercised.

A young framework needs a public face, even before it needs a fancy one.

---

## 10. The frontend shape of Systemlab

Because Systemlab is browser-facing, there is also a frontend structure to understand. That structure is deliberately modular even though the UI is still relatively small.

Why? Because the team already knows that each future phase will add another lab page, another set of controls, another explanatory surface, and another set of debugging views. If all of that lived in one HTML file, the shell would quickly become harder to maintain than the framework it is trying to explain.

So the frontend is split into:

- `static/index.html` — shell only,
- `static/app.css` — shared styling,
- `static/partials/*.html` — page-level fragments,
- `static/js/main.js` — bootstrap and navigation,
- `static/js/pages/*.js` — per-page behavior,
- `static/js/api.js` and `static/js/dom.js` — shared helpers,
- `chapters/*.md` — long-form prose chapters like the one you are reading now.

This is one of those design choices that seems small until you try to extend the system. Then it becomes obvious why it was worth doing early.

---

## 11. How to use the Phase 0 page as a learning tool

Phase 0 does not have rich runtime controls the way later phases do, but the page is still meant to be used actively.

### Try 1: read the introductory prose, then inspect the status block

Start on the Overview / Phase 0 page. Read the short explanation text at the top, then scroll through the framework status panel.

What you are doing here is learning to connect prose and machine-readable state. The prose tells you what Systemlab is trying to be. The status panel tells you which phases currently exist, which ones are placeholders, and which boundary rules are being surfaced by the app.

Pay attention to the relationship between the two. Does the UI feel like it is explaining a real architecture? Does it feel like a separate consumer app rather than framework internals in disguise? Those are the right questions at this stage.

### Try 2: navigate between available phase pages

Click between:

- Overview / Phase 0
- Phase 1
- Phase 2

Even if you do not yet understand the later phases in detail, notice what the shell is already doing. It is making room for later learning without pretending that all later behavior is already built. That is a subtle but healthy design move.

### Try 3: compare the page with the source files

Leave the page open and read these files side by side:

- `pinocchio/cmd/evtstream-systemlab/server.go`
- `pinocchio/cmd/evtstream-systemlab/static/index.html`
- `pinocchio/cmd/evtstream-systemlab/static/partials/overview.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/overview.js`

As you do that, ask yourself: how much logic is the shell really owning? Ideally, the answer is "just enough to explain and present the system, but not so much that it becomes the system."

---

## 12. What to pay attention to when reading Phase 0 code

There are several patterns worth noticing because they teach you how this codebase wants to be extended.

### Pay attention to ownership

Whenever you open a file, ask: is this file describing a substrate concept, an implementation concept, or a consumer concept? If you build that instinct early, you will make cleaner changes later.

### Pay attention to dependency direction

The import-cycle story is not just a Go quirk. It is an early warning system for muddled boundaries.

### Pay attention to naming

Phase 0 is when naming is cheapest to stabilize. If a name is awkward now, it will be expensive later.

### Pay attention to what is *missing*

Sometimes the absence of something is deliberate. If the phase does not yet contain complex runtime logic, that might be because the team is preserving room for a cleaner implementation later.

---

## 13. Validation commands you should know immediately

These commands are part of how the architecture stays honest.

```bash
cd /home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio

make systemlab-run
make evtstream-test
make systemlab-build
make evtstream-boundary-check
make evtstream-check
```

### Why they matter

A framework is much easier to trust when its key design rules can be validated mechanically.

- `make systemlab-run` proves the shell app works as a separate app.
- `make evtstream-test` checks the targeted framework and Systemlab tests.
- `make systemlab-build` ensures the app builds cleanly.
- `make evtstream-boundary-check` helps catch boundary violations.
- `make evtstream-check` bundles the main targeted validation path.

The important thing to understand is that these are not afterthoughts. They are part of the architecture's defense system.

---

## 14. How a reviewer should read Phase 0

If you are reviewing Phase 0 for the first time, read in this order:

1. `pinocchio/pkg/evtstream/doc.go`
2. `pinocchio/pkg/evtstream/types.go`
3. `pinocchio/pkg/evtstream/handler.go`
4. `pinocchio/pkg/evtstream/projection.go`
5. `pinocchio/pkg/evtstream/hydration.go`
6. `pinocchio/pkg/evtstream/hub.go`
7. `pinocchio/cmd/evtstream-systemlab/README.md`
8. `pinocchio/cmd/evtstream-systemlab/server.go`
9. `pinocchio/cmd/evtstream-systemlab/static/index.html`
10. `pinocchio/Makefile`

That order walks you from vocabulary, to orchestration, to app boundary, to validation. It is the same path your mental model should follow.

---

## 15. The invariants you should remember after reading this chapter

If you walk away remembering only a handful of things, let them be these:

### Invariant 1: `SessionId` is the center of routing

The framework wants one canonical routing identity for stateful work.

### Invariant 2: backend events are canonical

UI-facing shapes are derived, not primary.

### Invariant 3: Systemlab is a consumer of the framework

It is not part of the substrate and should not quietly act like it is.

### Invariant 4: concrete implementations depend on interfaces

The core package should not point downward at implementations.

### Invariant 5: maintainability is part of the design

File layout, naming, and validation are not secondary concerns. They are how the system stays understandable.

---

## 16. Where later phases go from here

Phase 0 is the map. Later phases start traveling.

### Phase 1 will add

- real in-memory command dispatch,
- event publication,
- projection execution,
- hydration snapshots.

### Phase 2 will add

- a real Watermill-backed bus boundary,
- consumer-side ordinal assignment,
- ordering experiments.

### Phase 3 will add

- websocket transport,
- subscriptions,
- snapshot-before-live reconnect behavior.

### Phase 4 will add

- the first real example backend: chat.

### Phase 5 will add

- durable SQL hydration,
- restart correctness.

This is why it matters that Phase 0 stays generic and disciplined. Every later phase stands on it.

---

## 17. Common intern mistakes in this area

A few mistakes show up again and again when engineers are new to this code.

### Mistake 1: treating donor code as the substrate

Study donor code closely. Do not confuse it with the clean-room abstraction.

### Mistake 2: adding example-specific helpers to `pkg/evtstream`

If a helper only makes sense for one lab or one application, it probably belongs outside the substrate.

### Mistake 3: collapsing session and connection concepts

That makes future transport and reconnect logic much harder.

### Mistake 4: letting the lab cheat

If Systemlab can only work by calling non-public logic, then it is no longer validating the architecture.

### Mistake 5: underestimating validation tooling

If the build and boundary checks go stale, the architecture will eventually drift away from its documented shape.

---

## 18. Final summary

Phase 0 is the phase where the project decides to be legible.

It gives the system:

- a dedicated package home,
- stable core vocabulary,
- public seams,
- honest dependency direction,
- a separate consumer app,
- executable checks for the architecture.

It also gives a new intern something deeply valuable: a way to understand the project before they are asked to accelerate it.

That is what a good foundation phase should do. It should not only make future implementation possible. It should make future implementation teachable.

---

## 19. Things to try on the Phase 0 page

Phase 0 does not yet have a rich interactive control surface like later labs, but you should still use the page actively rather than only reading it.

### Try 1: read the overview text and then inspect framework status

On the page:

- read the short introductory text,
- then inspect the JSON status panel.

### What to look for

Pay attention to:

- which phases are marked implemented,
- which phases are only placeholders,
- how the boundary rules are described,
- whether the shell feels like a separate app instead of framework code.

### Try 2: navigate between available phase pages

Use the left navigation to move between:

- Overview / Phase 0
- Phase 1
- Phase 2

### What to look for

The main thing to notice is structural rather than behavioral:

- the shell already has places for later phases,
- but the architecture does not force unimplemented behavior to exist yet,
- which is exactly what a healthy foundation phase should feel like.

### Try 3: compare the page with the source files

Keep the page open and read these files side by side:

- `pinocchio/cmd/evtstream-systemlab/server.go`
- `pinocchio/cmd/evtstream-systemlab/static/index.html`
- `pinocchio/cmd/evtstream-systemlab/static/partials/overview.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/pages/overview.js`

### What to pay attention to

Notice how little product logic is required for the shell. That is a sign that the page is doing the right job: it is exposing architecture and status, not inventing fake runtime behavior.

---

## 20. Short glossary

### Substrate
The reusable framework layer, not the product-specific app.

### Canonical event
The backend event shape the framework treats as the primary internal stream.

### Projection
A transformation from canonical backend events into another view, such as UI events or timeline entities.

### Hydration
Reconstructing current state for reconnecting clients from durable or semi-durable store state.

### Systemlab
The separate explainer/testing/debugging app that consumes public framework seams.

---

## 21. File references at a glance

### Key API files

- `pinocchio/pkg/evtstream/types.go`
- `pinocchio/pkg/evtstream/handler.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/transport/transport.go`

### Key Systemlab files

- `pinocchio/cmd/evtstream-systemlab/README.md`
- `pinocchio/cmd/evtstream-systemlab/main.go`
- `pinocchio/cmd/evtstream-systemlab/server.go`
- `pinocchio/cmd/evtstream-systemlab/static/index.html`
- `pinocchio/cmd/evtstream-systemlab/static/app.css`
- `pinocchio/cmd/evtstream-systemlab/static/partials/overview.html`
- `pinocchio/cmd/evtstream-systemlab/static/js/main.js`
- `pinocchio/cmd/evtstream-systemlab/chapters/phase-0-foundations.md`

### Key validation file

- `pinocchio/Makefile`
