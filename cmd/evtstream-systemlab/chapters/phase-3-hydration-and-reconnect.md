# Phase 3 — Hydration and Reconnect

## Welcome

Phase 3 is where the framework begins to feel like a realtime substrate instead of a beautifully structured backend engine. Up to this point, you have seen commands, backend events, projections, hydration state, and bus-backed ordering. But there is still a crucial piece missing from the user-facing story: what happens when a live client shows up, leaves, and comes back?

That is the question Phase 3 is built to answer.

A realtime system is not really trustworthy until it can handle the basic messiness of actual clients. Clients connect late. Clients disconnect unexpectedly. Clients reconnect after the system has already accumulated meaningful state. And when they return, they do not want a philosophical explanation of event ordering—they want a coherent view of the current world, followed by live updates that continue naturally from there.

This phase therefore introduces the first real live-client transport concept: websocket-based delivery, connection tracking, subscriptions by `SessionId`, and the all-important rule of **snapshot before live**.

At the time of writing, this chapter is intentionally ahead of the full interactive implementation in Systemlab. That is still valuable. You should read it as the conceptual foundation for what the Phase 3 page is trying to become, and as the preparation you need before the transport becomes more fully interactive.

By the end of this chapter, you should understand:

- why a live transport belongs after ordering and hydration basics,
- why `ConnectionId` exists separately from `SessionId`,
- what snapshot-before-live means and why it is non-negotiable,
- what the planned Phase 3 controls are designed to teach,
- what kinds of bugs this phase is meant to prevent,
- and what to watch closely once the phase becomes fully interactive.

---

## 1. Why reconnect is a framework problem, not just a UI problem

Many systems treat reconnect as a frontend concern. The browser dropped, so the frontend reconnects. The socket reopened, so the UI asks for state again. At a superficial level that is true. But a reconnect-safe system cannot be built only from frontend good intentions.

Reconnect is fundamentally a framework problem because the framework owns the relationship between:

- current durable state,
- live event delivery,
- session routing,
- and transport identity.

If the framework has no coherent answer to those relationships, the frontend can reconnect all it wants and still end up with duplicated updates, missing updates, inconsistent state, or live events arriving before the client has been rehydrated.

That is why Phase 3 sits where it does. It comes *after* hydration concepts and *after* consumer-side ordering, because reconnect only makes sense when the system already knows what state is current and how event order is defined.

---

## 2. The central lesson of Phase 3

The most important sentence in this phase is this one:

> A reconnecting client should receive a coherent snapshot first, and only then continue with live UI events.

This sounds simple, but it is one of the most delicate sequencing rules in the whole architecture.

If live events arrive before the client has been hydrated, the client can momentarily observe a world that does not line up with the snapshot it later receives. That leads to duplicated state, visual jumps, confusing debugging, and loss of trust in the system.

If, on the other hand, the framework ensures snapshot-before-live, then the reconnect story becomes much easier to reason about:

1. client subscribes,
2. framework loads snapshot,
3. client receives current state,
4. client then receives live UI events that continue from that state.

That is the narrative the transport must preserve.

---

## 3. Why `ConnectionId` matters more here than it did before

In earlier phases, `ConnectionId` was mostly a vocabulary concept. In Phase 3 it becomes operational.

This is where you really feel why the framework separated `ConnectionId` from `SessionId`.

### `SessionId`
Represents the business-level routing identity.

### `ConnectionId`
Represents one transport-level connection.

That distinction matters because a single session may later have:

- multiple browser tabs,
- a reconnecting tab replacing an older one,
- several observers attached to the same session,
- one client disconnecting while another remains subscribed.

If the framework had collapsed these concepts together, transport logic would quickly become awkward and brittle. But because the distinction was introduced early, Phase 3 gets to use it naturally instead of retrofitting it under pressure.

---

## 4. What websocket transport is supposed to do in this framework

The websocket transport is not supposed to become a second framework inside the framework. It has a focused role.

It should:

- accept live connections,
- assign `ConnectionId`s,
- track subscriptions by session,
- deliver snapshots on subscribe,
- deliver live UI events after that,
- accept unsubscribe or disconnect behavior cleanly,
- stay unaware of application-specific business logic.

It should **not**:

- invent application semantics,
- assign ordinals,
- decide what the canonical event model means,
- become the place where commands are secretly interpreted in app-specific ways.

This distinction is important because transport code is one of the easiest places for architectural leakage to happen.

---

## 5. The key design rule: snapshot before live

If you only remember one technical phrase from this chapter, let it be "snapshot before live."

The reason it matters is that reconnect is fundamentally a race between:

- the current store state,
- and the future live stream.

The framework must ensure the client receives those in the right order.

### Conceptual sequence

```text
Client connects
    -> receives ConnectionId
Client subscribes to SessionId
    -> framework loads snapshot from HydrationStore
    -> framework sends snapshot message first
    -> framework begins sending live UI events after snapshot
```

### Why the order matters

If the framework reversed that order, a client might:

- see live append events,
- then receive a stale snapshot,
- then need complex client logic to reconcile them,
- or simply end up wrong.

The framework is trying to avoid that entire category of complexity by sequencing the transport correctly.

---

## 6. How the transport fits into the wider architecture

It helps to visualize where the websocket transport sits relative to the rest of the system.

```text
Commands
   |
   v
Handlers
   |
   v
Canonical backend events
   |
   v
Consumer
   |---------------------> TimelineProjection -> HydrationStore
   |
   +---------------------> UIProjection -> UIFanout -> Websocket transport
                                                   |
                                                   v
                                              ConnectionId / Session subscriptions
```

This diagram is a reminder that the websocket transport is *downstream* of the consumer and the UI projection. That is exactly where it belongs.

The transport is not the source of truth. It is the live-delivery mechanism for one derived view of the truth.

---

## 7. What the planned Phase 3 page is trying to teach

The Systemlab Phase 3 page is meant to be more than a transport demo. It is meant to make reconnect semantics understandable.

### The page is supposed to show

- Client A state,
- Client B state,
- connect/disconnect transitions,
- subscribe actions,
- snapshot payloads,
- live UI events,
- the current store snapshot,
- invariant checks for sequencing and convergence.

### Why two clients matter

Using two simulated clients prevents you from thinking about reconnect as a single-tab toy problem. It forces you to think about:

- independent connection lifecycle,
- shared session state,
- different subscribe timings,
- reconvergence toward the same final session view.

That is exactly the right kind of teaching surface for this phase.

---

## 8. The controls this page is meant to exercise

At the time of this chapter, some of these controls may still be placeholder controls rather than fully live ones. That is okay. The important thing is to understand what they are *for*.

### Planned controls

- `Connect` / `Disconnect`
- `Subscribe` with a `SessionId`
- optional `sinceOrdinal`
- a way to seed or trigger session activity,
- reconnect controls,
- possibly a reset control.

These are not random buttons. Each one exists to teach a different part of the transport story.

---

## 9. Things to try once the controls are live

This section is written so that you already know what to look for when the Phase 3 page is fully interactive.

### Try 1: connect Client A and subscribe to an empty session

### What should happen

- the client should connect successfully,
- a subscription should be established,
- the snapshot should be empty or minimal,
- live state should begin from a coherent baseline.

### What to pay attention to

This is a subtle scenario. Even an empty session is teaching you whether the snapshot-before-live rule is being followed. The absence of state should still arrive in the correct shape and sequence.

---

### Try 2: generate activity, then connect Client B later

### What should happen

Client B should receive:

- a snapshot representing current state,
- then only the subsequent live UI events.

### What to pay attention to

The important question is not whether Client B eventually looks correct. The important question is whether Client B got there *cleanly*, without duplicated or out-of-order reasoning required in the browser.

---

### Try 3: disconnect Client A, keep activity going, then reconnect

### What should happen

On reconnect, Client A should:

- reconnect as a new or renewed `ConnectionId`,
- resubscribe to the same `SessionId`,
- receive a current snapshot,
- then receive live events after that snapshot.

### What to pay attention to

This is the real heart of the phase. Watch for whether the live stream continues naturally from the snapshot rather than racing ahead of it.

---

### Try 4: compare Client A and Client B final state

### What should happen

Even if they connected at different times and disconnected differently, the clients should converge to the same final understanding of the session.

### What to pay attention to

This is one of the strongest transport invariants in the whole framework: clients with different live histories should still end at the same session truth if the framework is sequencing hydration and live delivery correctly.

---

## 10. Why `sinceOrdinal` is interesting but dangerous

The moment you introduce a `sinceOrdinal` concept, you open the door to a more complicated story. It can be useful because it allows a client to describe what point in the stream it believes it has already seen. But it is also dangerous because it tempts the system to overcomplicate reconnect logic before its basic snapshot model is stable.

That is why this phase should stay conservative.

If `sinceOrdinal` exists, it should support understanding and optimization—not replace the core snapshot-before-live discipline.

A new engineer should be cautious here. Fancy reconnection logic often looks efficient before it becomes impossible to explain.

---

## 11. The kinds of bugs this phase is designed to prevent

Phase 3 is not just adding transport. It is preventing several classes of failure.

### Bug class 1: live before snapshot

This is the classic reconnect bug. The client receives live data before it has a coherent base state.

### Bug class 2: transport owning business semantics

If websocket code starts interpreting application meaning directly, the framework boundary gets polluted.

### Bug class 3: confusing connections with sessions

That leads to brittle subscription and reconnect behavior.

### Bug class 4: duplicated or skipped visible state after reconnect

Even if the store is correct, a bad subscribe sequence can make the client wrong.

### Bug class 5: hidden coupling between transport and one example app

If the transport only works because it secretly knows chat-specific message shapes, the framework has already drifted off course.

---

## 12. What a good Phase 3 implementation should feel like

A good Phase 3 implementation should feel almost calm. That is a strange word for a live transport phase, but it is the right one.

When you connect and subscribe, the system should feel unsurprising. The snapshot should arrive first. Live updates should feel like a continuation, not a correction. Disconnect and reconnect should not feel magical; they should feel understandable.

That is what you want in a framework like this. Not flashy behavior. Trustworthy behavior.

---

## 13. Important API references and files to study

### Phase 3 framework areas

- `pinocchio/pkg/evtstream/transport/transport.go`
- `pinocchio/pkg/evtstream/fanout.go`
- `pinocchio/pkg/evtstream/hydration.go`
- later websocket transport package under `evtstream/transport/ws`

### Systemlab files you should expect to matter

- future Phase 3 backend lab file in `cmd/evtstream-systemlab/`
- future partial under `static/partials/phase3.html`
- future page behavior module under `static/js/pages/phase3.js`
- this chapter file:
  - `pinocchio/cmd/evtstream-systemlab/chapters/phase-3-hydration-and-reconnect.md`

---

## 14. Common mistakes for new engineers in this phase

### Mistake 1: thinking websocket transport is the "main system"

It is not. It is one downstream transport for UI events.

### Mistake 2: treating reconnect as frontend-only

Reconnect correctness depends on framework sequencing and store semantics.

### Mistake 3: skipping snapshot-before-live because it seems simpler

That shortcut often creates a much bigger mess later.

### Mistake 4: letting transport logic know too much about app-specific event meanings

That will trap the framework inside its first example.

### Mistake 5: assuming one connection equals one session

That assumption breaks the moment you have multiple tabs or reconnecting clients.

---

## 15. Final summary

Phase 3 is where the framework learns how to meet real clients without losing its architectural discipline.

The essential lesson is simple to say and difficult to preserve:

- sessions own state,
- connections own transport presence,
- snapshots establish the current truth,
- live UI events continue from that truth,
- and reconnect correctness depends on the framework sequencing those things properly.

If Phase 2 taught the system to be honest about consumed order, Phase 3 teaches it to be honest about what a reconnecting client is allowed to see and when.

---

## 16. File references at a glance

### Framework references

- `pinocchio/pkg/evtstream/transport/transport.go`
- `pinocchio/pkg/evtstream/fanout.go`
- `pinocchio/pkg/evtstream/hydration.go`

### Systemlab references

- `pinocchio/cmd/evtstream-systemlab/chapters/phase-3-hydration-and-reconnect.md`
- planned future page partial and JS module for Phase 3

### Suggested validation mindset

When Phase 3 is live, your first validation questions should be:

- Did snapshot arrive before live?
- Did multiple clients converge to the same final state?
- Did the transport remain transport-only rather than turning into hidden business logic?
