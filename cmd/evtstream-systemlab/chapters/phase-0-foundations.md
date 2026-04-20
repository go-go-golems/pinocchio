# Phase 0 — Foundations, API Skeleton, and Systemlab Shell

## Who this chapter is for

This chapter is written for a new intern joining the `evtstream` work for the first time. You should be able to read this chapter before touching the code and come away understanding:

- what the framework is trying to become,
- why it lives in `pkg/evtstream`,
- why Systemlab is a separate application,
- what Phase 0 is supposed to prove,
- which files matter first,
- which boundaries are architectural rules rather than temporary preferences.

This is a **textbook-style explanation chapter**, not just a changelog. It mixes prose, diagrams, pseudocode, file references, and API references so you can use it both as onboarding material and as a review checklist.

---

## 1. The big picture

The long-term goal of `evtstream` is to become a reusable substrate for realtime, event-streaming applications such as chat systems, agent systems, and other live UIs that need:

- typed commands coming in,
- typed backend events flowing through a canonical stream,
- projection into live UI updates,
- projection into durable hydration state,
- reconnect behavior that can replay current state and then continue live.

In plain English, the framework is trying to answer this question:

> How do we build systems where a command starts work, backend events stream out of that work, the UI updates live, durable state stays coherent, and clients can reconnect without confusion?

Phase 0 does **not** implement all of that behavior. Phase 0 instead creates the **shape** that future phases need so the implementation can grow without becoming tangled.

If you remember only one sentence from this chapter, remember this one:

> Phase 0 is about making the architecture real in code before the behavior becomes complex.

---

## 2. What Phase 0 is trying to prove

Phase 0 is intentionally modest. It proves that the codebase is arranged correctly before we add more runtime behavior.

### Phase 0 goals

Phase 0 establishes:

- a dedicated framework package home,
- stable substrate vocabulary,
- public seams for later implementations,
- a separate Systemlab app shell,
- a clear rule that Systemlab may consume public APIs but not framework internals,
- validation commands that make those boundaries reviewable.

### Phase 0 non-goals

Phase 0 does **not** yet promise:

- real distributed event delivery,
- websocket subscriptions,
- durable SQL hydration,
- chat-specific behavior,
- webchat compatibility.

That distinction matters because interns often read scaffolding code and assume it is incomplete production logic. In this phase, some pieces are intentionally minimal because the primary deliverable is **structure**, not runtime sophistication.

---

## 3. Why `pkg/evtstream` exists

Before this work, `pinocchio/pkg/webchat` already contained strong event-streaming donor ideas. But it is not the right generic substrate as-is.

Why not?

- It carries chat-specific assumptions.
- It includes legacy shapes tied to SEM/webchat behavior.
- It mixes transport, business semantics, and product-specific details.
- It has identity and routing concerns that do not match the clean-room target.

So instead of renaming `pkg/webchat`, the project created a new package:

- `pinocchio/pkg/evtstream`

That is where the generic substrate lives.

### Mental model

Think of the relationship this way:

```text
webchat is a donor and later a consumer/example

evtstream is the reusable substrate
```

### File references

Start here:

- `pinocchio/pkg/evtstream/doc.go`
- `pinocchio/pkg/evtstream/types.go`
- `pinocchio/pkg/evtstream/handler.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/transport/transport.go`

These files define the vocabulary and seams that later phases plug real behavior into.

---

## 4. Why Systemlab is a separate app

One of the most important architectural decisions is that **Systemlab is not the framework**.

Systemlab is a separate app under:

- `pinocchio/cmd/evtstream-systemlab`

That decision is not cosmetic. It is what makes API boundaries testable.

If the framework and the lab were the same app, it would become too easy to:

- call internal helpers directly,
- bypass public seams,
- leak example-specific concepts into the substrate,
- build demos that only work because they cheat.

### What Systemlab is for

Systemlab is meant to be all of these at once:

- an onboarding artifact,
- an interactive explainer,
- a debugging surface,
- a regression tool,
- a public-API exerciser.

### Boundary rule

Systemlab may:

- import public `evtstream` APIs,
- expose its own HTTP endpoints,
- present views and labs that explain framework behavior,
- simulate transport and application use.

Systemlab may not:

- import `pkg/webchat` internals,
- mutate framework internals by private access,
- redefine substrate concepts in a lab-specific way,
- turn phase demos into hidden framework dependencies.

### File references

Read:

- `pinocchio/cmd/evtstream-systemlab/README.md`
- `pinocchio/cmd/evtstream-systemlab/main.go`
- `pinocchio/cmd/evtstream-systemlab/server.go`

---

## 5. The Phase 0 architectural vocabulary

A new intern should become comfortable with a handful of core terms immediately.

### 5.1 `SessionId`

`SessionId` is the universal routing key.

This is one of the most important clean-room decisions.

Instead of splitting routing between multiple overlapping identity concepts, the framework centers on:

- one session,
- one routing identity,
- one place where ordered state and subscriptions accumulate.

You will see `SessionId` referenced repeatedly in:

- `types.go`
- `hub.go`
- hydration store APIs
- bus/topic/partitioning rules
- future websocket subscriptions

### 5.2 `ConnectionId`

`ConnectionId` identifies one transport connection.

It is not the same thing as a session.

A session may later have:

- zero connections,
- one connection,
- multiple simultaneous connections.

That separation matters because the framework routes business state by `SessionId`, while transport-specific routing later happens by `ConnectionId`.

### 5.3 `Command`

A `Command` is the typed request entering the substrate.

Examples in later phases might include:

- `StartInference`
- `StopInference`
- `LabStart`

A command is not a UI event, not a websocket envelope, and not durable state. It is simply the typed request shape entering dispatch.

### 5.4 `Event`

An `Event` is the canonical backend event carried through the substrate.

This is a central concept.

The framework does **not** treat UI events as the primary internal currency. Backend events are the canonical internal stream. Projections then derive:

- UI events for live clients,
- timeline entities for hydration state.

### 5.5 `UIProjection`

A `UIProjection` transforms one backend event into zero or more UI events.

That is how live client updates are derived.

### 5.6 `TimelineProjection`

A `TimelineProjection` transforms one backend event into zero or more timeline entities.

That is how durable hydration state is built.

### 5.7 `HydrationStore`

A `HydrationStore` is the persistence seam behind snapshots, views, and cursors.

It is intentionally abstract because later phases need multiple implementations:

- in-memory store,
- SQL store,
- possibly others later.

---

## 6. Phase 0 directory map

This is the Phase 0 mental map of the important code.

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
│       └── static/
│           ├── index.html
│           ├── app.css
│           ├── partials/
│           └── js/
│
└── Makefile
```

### What each area means

#### `pkg/evtstream/*`
This is the substrate itself.

#### `cmd/evtstream-systemlab/*`
This is the separate lab app that explains and exercises the substrate.

#### `Makefile`
This contains executable validation hooks for the boundary and build shape.

---

## 7. The public API skeleton in Phase 0

Phase 0 does not try to finish all runtime behavior. Instead, it establishes stable names and seams.

### API references

The main public types and interfaces appear in these files:

- `pinocchio/pkg/evtstream/types.go`
- `pinocchio/pkg/evtstream/handler.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/transport/transport.go`

### Example mental sketch

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

### Why this matters

The value of this skeleton is that later phases can add real behavior without renaming fundamental concepts every day.

That stability is extremely important for:

- tests,
- examples,
- docs,
- onboarding,
- future external consumers.

---

## 8. The import-cycle lesson from Phase 0

One of the most important technical lessons of Phase 0 came from a failure.

### The problem

A naive version of `Hub` tried to default directly to the in-memory hydration store implementation.

That creates this dependency chain:

```text
evtstream -> hydration/memory -> evtstream
```

That is an import cycle.

### Why it is wrong architecturally

The core package should define interfaces and types.
Concrete implementations should depend on the core, not the other way around.

### The fix

The fix used in Phase 0 is dependency inversion:

- `evtstream` owns the `HydrationStore` interface,
- `evtstream/hydration/memory` implements it,
- callers inject that implementation,
- the core package uses a root-local noop fallback when no store is supplied.

### File references

Read these together:

- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/noop_store.go`
- `pinocchio/pkg/evtstream/hydration/memory/store.go`
- `pinocchio/pkg/evtstream/hub.go`

### Pseudocode for the right pattern

```go
store := memory.New()

hub, err := evtstream.NewHub(
    evtstream.WithHydrationStore(store),
)
```

and **not** this:

```go
func NewHub(...) {
    store := memory.New() // wrong place, causes core -> implementation dependency
}
```

---

## 9. Why Phase 0 also includes a shell app

A common intern mistake is to think that because Phase 0 is a foundation phase, it should only produce library code. That is not enough.

The shell app matters because a framework boundary is not proven until something outside the framework uses it.

### What the shell app proves

The shell app proves that:

- the package tree compiles coherently,
- the framework can be consumed by another app,
- the public names are good enough to build against,
- the system can have a separate explainer surface.

### What the shell page shows

The early shell page is intentionally simple:

- navigation,
- overview text,
- framework status,
- placeholder pages for future labs.

That simplicity is not a weakness. It is exactly the right amount of behavior for a foundation phase.

---

## 10. The frontend shape of Systemlab

Even though Systemlab is a Go app, the browser-facing UI still needs structure.

### Why the frontend is split into multiple files

Systemlab is expected to grow phase by phase. If everything stayed in one HTML file, future labs would quickly become hard to read and hard to review.

So the UI is intentionally split into:

- `static/index.html` — shell only,
- `static/app.css` — shared styles,
- `static/partials/*.html` — page-level HTML fragments,
- `static/js/main.js` — bootstrap and navigation,
- `static/js/pages/*.js` — per-page behavior,
- `static/js/api.js` and `static/js/dom.js` — shared helpers.

### Intern rule

When you add a new lab page:

- add a new partial,
- add a new page module,
- keep `index.html` as a shell,
- avoid dumping unrelated logic into `main.js`.

That rule keeps the browser UI aligned with the phase-by-phase architecture of the framework.

---

## 11. Validation commands you should know in Phase 0

These commands are part of the architecture, not just convenience helpers.

### Main commands

```bash
cd /home/manuel/workspaces/2026-04-07/extract-webchat/pinocchio

make systemlab-run
make evtstream-test
make systemlab-build
make evtstream-boundary-check
make evtstream-check
```

### What each one means

#### `make systemlab-run`
Runs the Systemlab app locally.

#### `make evtstream-test`
Runs the targeted framework and Systemlab test set.

#### `make systemlab-build`
Builds the shell app without polluting the repo root.

#### `make evtstream-boundary-check`
Checks that Systemlab does not import forbidden legacy internals.

#### `make evtstream-check`
Runs the combined targeted validation flow for the current work.

### Why these commands matter

A good architecture is one that can be mechanically checked. These commands turn design rules into reviewable behavior.

---

## 12. How a reviewer should read Phase 0 code

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

This order moves from vocabulary -> orchestration -> app boundary -> validation.

---

## 13. The most important architectural invariants from Phase 0

You should be able to state these invariants from memory.

### Invariant 1: the substrate is centered on `SessionId`

No alternative routing identity should quietly become the real one.

### Invariant 2: backend events are canonical

UI messages are derived projections, not the main internal stream.

### Invariant 3: Systemlab is a consumer, not part of the substrate

It should exercise public APIs, not become an excuse to bypass them.

### Invariant 4: implementations depend on interfaces, not the reverse

The import-cycle fix is not a workaround; it is the intended architecture.

### Invariant 5: file layout is part of maintainability

Small, clearly-owned files matter because the system is expected to grow by phase.

---

## 14. Phase 0 runtime flow, conceptually

Even with minimal behavior, you should understand the conceptual runtime shape.

```text
Systemlab shell app
    |
    | consumes public evtstream package APIs
    v
Hub / registries / store seams
    |
    | later phases will add real command dispatch, bus consumption,
    | projections, hydration, and transport
    v
Future event-streaming substrate
```

This is the key idea:

- Phase 0 does not finish the runtime,
- but it ensures there is a coherent place for the runtime to arrive.

---

## 15. What changes in later phases

A new intern should know what Phase 0 hands to later work.

### Phase 1 will add

- real in-memory command dispatch,
- session creation,
- event publication,
- projections,
- hydration snapshot behavior.

### Phase 2 will add

- Watermill-backed publish/consume,
- consumption-time ordinal assignment,
- ordering lab behavior.

### Phase 3 will add

- websocket transport,
- subscription sets,
- snapshot-before-live reconnect behavior.

### Phase 4 will add

- the first real example backend: chat.

### Phase 5 will add

- durable SQL hydration store,
- restart correctness.

This is why Phase 0 must stay generic and clean. Every later phase depends on it.

---

## 16. Common mistakes for new interns in this area

### Mistake 1: treating donor code as the substrate

Do not assume that because `pkg/webchat` has working logic, its structure is the right generic structure.

### Mistake 2: adding app-specific helpers to `pkg/evtstream`

If a helper only makes sense for one example or one lab, it probably belongs outside the substrate.

### Mistake 3: collapsing session and connection concepts

That will make later transport and reconnect behavior much harder.

### Mistake 4: letting the lab cheat

If Systemlab calls internal logic that a real consumer could not call, the lab is undermining the architecture instead of validating it.

### Mistake 5: ignoring validation commands

A reviewer needs more than prose. Always keep the build and boundary checks healthy.

---

## 17. Suggested pseudocode for explaining Phase 0 to someone else

If you had to explain Phase 0 on a whiteboard, you could use this simplified pseudocode:

```go
// 1. Create the substrate home.
package evtstream

// 2. Define stable nouns.
type SessionId string
type Command struct { ... }
type Event struct { ... }

// 3. Define stable seams.
type CommandHandler func(...)
type UIProjection interface { ... }
type TimelineProjection interface { ... }
type HydrationStore interface { ... }

// 4. Create the top-level orchestration object.
func NewHub(opts ...HubOption) (*Hub, error)

// 5. Build a separate shell app against public APIs.
func main() {
    app := systemlab.New()
    http.ListenAndServe(":8091", app.Routes())
}
```

This is intentionally boring pseudocode. That is good. Foundation code should be boring in the sense that the names, seams, and ownership are clear.

---

## 18. Review checklist for Phase 0

Use this checklist when reviewing or extending the phase.

### Architecture checklist

- [ ] Is the code in `pkg/evtstream` generic rather than chat-specific?
- [ ] Is `SessionId` still the central routing key?
- [ ] Do concrete store implementations depend on the core, not vice versa?
- [ ] Is Systemlab still a separate consumer app?

### Systemlab checklist

- [ ] Does the shell use public APIs only?
- [ ] Is the frontend still split into shell/partials/page modules?
- [ ] Are new pages added as separate partial + JS module pairs?

### Validation checklist

- [ ] Does `make evtstream-test` pass?
- [ ] Does `make systemlab-build` pass?
- [ ] Does `make evtstream-boundary-check` pass?
- [ ] Does `make evtstream-check` pass?

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

## 21. Final summary

Phase 0 is the phase where the project stops being an idea and starts being a real architecture.

It gives the framework:

- a dedicated package home,
- stable vocabulary,
- public seams,
- clean dependency direction,
- a separate consumer app,
- executable validation commands.

It gives a new intern something even more important:

- a reliable map.

Once you understand Phase 0, the later phases make sense as additions to a stable structure instead of a pile of unrelated features.

---

## 22. API references and file references at a glance

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

### Key validation file

- `pinocchio/Makefile`
