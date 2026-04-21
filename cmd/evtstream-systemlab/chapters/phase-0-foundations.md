# Phase 0 — Foundations

## What this chapter is about

When you finish Phase 0, you should understand what `evtstream` is trying to become, why it lives in its own package, why Systemlab is a separate app, and what invariants the rest of the framework will build on. Phase 0 is not exciting in the way later phases are exciting. It is important in the way that foundations are important: everything else depends on it.

---

## 1. The story this framework is trying to tell

`evtstream` is trying to become a reusable substrate for realtime, event-streaming applications. The phrase "event-streaming substrate" can sound abstract, so let me translate it.

Imagine a client sends a command—perhaps a prompt, perhaps a start action, perhaps a control signal. That command triggers work on the backend. While the backend is working, it publishes canonical backend events that describe what is happening. Those events serve two purposes at the same time: they drive live UI updates, and they build durable state that later phases will use for hydration and reconnect.

```
Command → Handler → Canonical Events → UIProjection + TimelineProjection
                                              ↓
                                        HydrationStore
```

The framework makes that pattern reusable. That means it is trying to answer a specific question:

> How do we build systems where commands start work, backend events become the canonical internal stream, UI updates remain derived rather than primary, and reconnecting clients can recover cleanly?

That is a lot to ask from a framework. Which is why Phase 0 is careful. We are not trying to implement the whole dream here. We are trying to create a structure that can support the dream without turning into accidental product code.

---

## 2. Why we created `pkg/evtstream` instead of renaming `pkg/webchat`

One of the most important design decisions was that we did **not** simply rename `pkg/webchat` and call it the framework.

`pkg/webchat` contains valuable donor logic. It has good ideas. It has examples of real streaming behavior. But it is shaped by the needs of webchat itself. It carries assumptions that make perfect sense in that product context but would become baggage in a generic substrate. It includes product-specific transport and message-shape concerns we explicitly do not want to make canonical.

Using donor code directly as the substrate would blur a line we want to keep very sharp: donor code is where we learn from prior work, but the substrate is where we define the cleaner abstraction.

So we created a new package home:

```
pinocchio/pkg/evtstream
```

That package is where the reusable framework vocabulary and seams live.

### The mental model

When you think about the relationship between the old code and the new code, use this:

```
pkg/webchat  → donor and later consumer/example
pkg/evtstream → reusable substrate
```

That distinction will help you make better decisions. When you wonder where a helper, projection, or transport concept belongs, ask: is this substrate logic or application logic? Substrate logic belongs in `pkg/evtstream`. Application logic belongs in a consumer package.

### The first files to read

Start here:

- `pkg/evtstream/doc.go` — what the package is
- `pkg/evtstream/types.go` — the core types
- `pkg/evtstream/hub.go` — the central routing point
- `pkg/evtstream/projection.go` — how events become views
- `pkg/evtstream/hydration.go` — the persistence seam

These are the files where the framework begins by naming the things it cares about.

---

## 3. Why Systemlab is a separate app

A new intern often notices Systemlab and thinks it is a demo shell. It is not. It is more important than that.

Systemlab exists as a **separate app** because a framework boundary is not really proven until something outside the framework uses it honestly. If the framework and the lab were blended together, we would always be at risk of subtle cheating. The lab would call helpers that no real consumer could call. It would reach into private internals just because that was convenient. It would accidentally turn teaching code into framework dependency.

Keeping Systemlab separate prevents that kind of drift.

Systemlab lives at `pinocchio/cmd/evtstream-systemlab`. It is all of these at once:

- a guided explainer,
- a debugging environment,
- a regression surface,
- an onboarding tool,
- a public-API exerciser.

It must be capable enough to be useful, but disciplined enough not to distort the architecture.

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

---

## 4. The vocabulary Phase 0 teaches you

These words are not decorative. They are the language the framework will use for every later phase.

### `SessionId`

`SessionId` is the universal routing key. Instead of scattering identity and routing across multiple overlapping notions, the framework makes one session identifier the center of gravity for command routing, event ordering, hydration state, reconnect semantics, and later subscription behavior. That clarity becomes more valuable as the system grows.

### `ConnectionId`

`ConnectionId` identifies one transport-level connection. It is deliberately **not** the same thing as a session. A session may later have multiple connections attached to it—a reconnecting tab, multiple observers, a disconnecting client while another remains subscribed. A transport must be free to manage connection lifecycle without changing the business-level concept of a session.

### `Command`

A command is the typed request entering the framework. It is what a caller wants to do. Commands are routed through the Hub to their registered handlers.

### `Event`

An event is the canonical backend event moving through the substrate. This is critical: the framework does not treat UI output as its primary internal form. It treats backend events as the truth and derives UI and hydration state from them.

### `UIProjection`

A `UIProjection` transforms canonical backend events into client-facing UI events. Its output is transient, live-facing, and optimized for delivery to connected clients.

### `TimelineProjection`

A `TimelineProjection` transforms the same canonical backend events into timeline entities that the hydration store can retain. Its output is persistent, stateful, and designed to support later hydration and reconnect.

### `HydrationStore`

A `HydrationStore` is the persistence seam. In later phases it matters for reconnect, snapshotting, and durable restart behavior. In Phase 0 it matters because we need the interface shape right before we need the implementation depth.

---

## 5. The import-cycle lesson

One of the most useful lessons of Phase 0 came from an actual failure. At one point, the core package tried to default directly to the in-memory store implementation. That seems harmless. It even sounds convenient. But it produces a dependency graph like this:

```
evtstream → hydration/memory → evtstream
```

That is an import cycle.

More importantly, it is a sign of architectural confusion. The core package should own interfaces and shared types. Concrete implementations should depend on the core—not the reverse.

The fix was not some clever Go trick. The fix was to return to the architecture and make the dependency direction honest.

The result:

- `evtstream` defines the `HydrationStore` interface,
- `evtstream/hydration/memory` implements it,
- callers inject implementations with options,
- the core keeps a root-local noop fallback rather than depending on the memory implementation.

### The right dependency shape

```go
store := memory.New()

hub, err := evtstream.NewHub(
    evtstream.WithHydrationStore(store),
)
```

and **not**:

```go
func NewHub(...) {
    store := memory.New() // wrong: core depending on concrete implementation
}
```

If you solve a problem like this cleanly in Phase 0, the later phases inherit that clarity automatically.

---

## 6. The directory map and what it means

It helps to look at the codebase as a map rather than a pile of files.

```
pinocchio/
├── pkg/
│   └── evtstream/           ← owns the substrate
│       ├── doc.go
│       ├── types.go
│       ├── hub.go
│       ├── projection.go
│       ├── hydration.go
│       └── transport/
│           └── transport.go
│
├── cmd/
│   └── evtstream-systemlab/  ← owns the teaching app
│       ├── main.go
│       ├── server.go
│       ├── chapters/
│       └── static/
│
└── Makefile                   ← owns validation
```

When you read that tree slowly, you can already see the intended ownership model. `pkg/evtstream` owns the substrate. `cmd/evtstream-systemlab` owns the teaching app. `Makefile` owns the repeatable validation entrypoints.

That separation is not just tidy. It is a statement of design.

---

## 7. Validation commands and why they matter

The architecture stays honest through mechanical checks, not just code review.

```bash
cd pinocchio

make systemlab-run      # prove the shell app works
make evtstream-test    # targeted framework tests
make systemlab-build   # clean build
make evtstream-boundary-check  # catch violations
make evtstream-check   # main validation path
```

These are not afterthoughts. They are part of the architecture's defense system. A framework is much easier to trust when its key design rules can be validated mechanically.

---

## Key Points

- Canonical events sit in the middle of the architecture. UI output and durable state are both derived from the same event stream.
- `SessionId` is the universal routing key. `ConnectionId` is transport-level identity, deliberately separate.
- Donor code (`pkg/webchat`) and substrate (`pkg/evtstream`) are different things with different purposes.
- Systemlab exists to prove the framework boundary is real. It is not a demo—it is a validation instrument.
- The core package should not point downward at concrete implementations. Interfaces own the seam; implementations depend on interfaces.
- File layout, naming, and validation are not secondary concerns. They are how the system stays understandable.

---

## File References

### Framework files

- `pkg/evtstream/doc.go` — package documentation
- `pkg/evtstream/types.go` — core type definitions
- `pkg/evtstream/hub.go` — central routing point
- `pkg/evtstream/projection.go` — UI and timeline projection interfaces
- `pkg/evtstream/hydration.go` — persistence seam
- `pkg/evtstream/hub.go` — orchestration entrypoint

### Systemlab files

- `cmd/evtstream-systemlab/main.go` — entrypoint
- `cmd/evtstream-systemlab/server.go` — HTTP server and routing
- `cmd/evtstream-systemlab/static/` — frontend structure

### Validation

- `Makefile` — all validation targets