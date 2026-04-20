# Phase 4 — Chat Example

## Welcome

By the time you reach Phase 4, the framework has already taught you a lot of abstract truths. You have seen why the substrate needs clean nouns, why canonical backend events should live in the middle, why the consumer owns final ordinals, and why reconnect behavior belongs to the framework rather than to ad hoc frontend code.

But there is a natural question that always comes next:

> Does this architecture actually feel good when a real application uses it?

That is the role of Phase 4.

Phase 4 introduces the first real example backend: chat. Not because chat is the only thing the framework should ever support, and not because the framework is secretly a chat framework, but because chat is a wonderfully teachable example of streamed activity becoming both live UI output and durable state.

A chat example lets you see something human-scaled. A prompt goes in. Streaming backend activity begins. Token-like deltas accumulate. UI events show progress. A timeline entity becomes the durable record of the message. In one compact example, you can feel what the architecture is buying you.

That is why this phase matters so much pedagogically. It is where the framework stops feeling like a set of elegant abstractions and starts feeling like a system someone could actually build on.

---

## 1. Why chat is the first example

There are many kinds of applications that could sit on top of `evtstream`. So why choose chat first?

Because chat has the exact properties this framework was designed to handle well.

A good chat example naturally includes:

- a command that initiates work,
- streamed backend events,
- live UI feedback,
- a timeline entity that accumulates state over time,
- a meaningful final state after a series of deltas,
- optional interruption or stop behavior.

That means chat is not just a flashy example. It is an unusually rich teaching backend. It forces the framework to prove that its abstractions are usable without making them chat-specific.

That is the balance this phase is trying to strike.

---

## 2. The most important design rule of Phase 4

The chat example is a **consumer of `evtstream`**, not part of `evtstream` itself.

That rule matters a lot.

If chat-specific logic starts leaking back into the substrate, then the framework stops being a reusable substrate and starts becoming a disguised application layer. That would be a major architectural regression.

So the phase is careful about package ownership.

### The core idea

```text
evtstream = generic substrate
chat example = application built on top of substrate
```

### What that means in practice

Chat-specific items such as:

- `StartInference`,
- `StopInference`,
- token delta events,
- message-append UI events,
- chat-specific timeline entities,

should live in example code or example registration helpers, not in the generic `pkg/evtstream` core.

This may feel inconvenient at first, but it is exactly what protects the framework from becoming too specialized too early.

---

## 3. What the chat flow is meant to teach

The chat example is a more emotionally intuitive version of the patterns you already saw in the synthetic Phase 1 lab.

In the synthetic lab, the message flow was intentionally generic and abstract. In the chat example, the same architecture is presented in a way that feels closer to something a user would actually recognize.

### A likely chat flow

1. the user submits `StartInference`,
2. the handler begins streaming backend events,
3. the backend emits something like `InferenceStarted`,
4. then a sequence of token or chunk events,
5. then a finish or stop event,
6. the UI projection converts these to message-start, append, and finish UI events,
7. the timeline projection reduces them into a final `ChatMessage` entity.

What makes this such a useful example is that it preserves the architecture while making the result feel human.

---

## 4. Why this phase is more than "make a demo"

A weaker version of this phase would build a cute page that looks like a product demo but does not really teach anything. That is not what we want.

This phase should instead produce something that is useful in at least three ways:

- as a teaching backend for new contributors,
- as a live framework exerciser for debugging,
- as a reference example showing how application code should sit on top of the substrate.

That is why the page needs to show more than just a prompt box and a final message. It should show:

- backend events,
- UI events,
- timeline entities,
- scenario presets,
- and stop/cancel behavior.

We want the example to be explanatory, not merely attractive.

---

## 5. What the chat example should contain conceptually

A good chat example on this framework has four kinds of schema:

- commands,
- backend events,
- UI events,
- timeline entities.

### Commands

These are the typed requests entering the system.

Typical examples:

- `StartInference`
- `StopInference`

### Backend events

These are the canonical internal events.

Typical examples:

- `InferenceStarted`
- `TokensDelta`
- `InferenceFinished`
- `InferenceStopped`

### UI events

These are the live-facing events delivered to clients.

Typical examples:

- `MessageStarted`
- `MessageAppended`
- `MessageFinished`
- `MessageStopped`

### Timeline entities

These are the durable state shapes built from the event stream.

Typical example:

- `ChatMessage`

This is one of the best phases for learning the architecture because the mapping between these four kinds of schema is easy to visualize.

---

## 6. The projection pair becomes much more intuitive here

In earlier phases, the existence of two projections may have felt technically correct but slightly abstract. In chat, the reason becomes much more visceral.

### UI projection in chat

The UI projection answers:

> What should a user watching the conversation live see right now?

That means it emits a flow of live updates such as:

- message started,
- message appended,
- message finished.

### Timeline projection in chat

The timeline projection answers:

> What chat message entity should the system remember as the durable current truth?

That means it reduces streaming deltas into one coherent timeline entity.

You can almost feel the difference physically:

- one is about the experience of watching a message arrive,
- the other is about the remembered result of that arrival.

That is exactly why the framework treats them as siblings instead of collapsing them into one projection.

---

## 7. The planned Phase 4 page

The Systemlab page for this phase should feel more concrete than the earlier labs, but it must still remain educational.

### The page is expected to show

- a prompt input,
- a session selector,
- scenario presets,
- backend event trace,
- UI event trace,
- timeline entity evolution,
- send and stop controls,
- transcript export.

### Why those pieces matter

The page should let an intern answer these questions:

1. What command was sent?
2. What backend events did that command create?
3. What did the live UI derive from those events?
4. What final timeline entity did the store retain?
5. What changes when the run is interrupted instead of finishing normally?

If the page can answer those clearly, it becomes a very strong teaching tool.

---

## 8. Things to try once the page is active

This section is written so that when the page becomes fully interactive, you already know how to use it as a learning tool.

### Try 1: the happy-path prompt

Use a normal prompt such as:

- `Explain ordinals in plain language`

Then click `Send`.

### What should happen

You should see:

- a start event,
- one or more token or chunk delta events,
- a finish event,
- matching UI message-start/append/finish events,
- a final `ChatMessage` timeline entity whose text matches the accumulated output.

### What to pay attention to

Do not just read the final answer. Watch how the live event story and the final durable state story tell the same narrative from different angles.

---

### Try 2: a streaming-focused preset

Use a preset that emphasizes incremental output.

### What should happen

You should see more visible append behavior in both the backend and UI event panels.

### What to pay attention to

This is one of the best scenarios for feeling the difference between stream and state. The live UI is about the unfolding message. The timeline entity is about the accumulated message.

---

### Try 3: stop or cancel mid-stream

Start a run and then click `Stop` before it completes.

### What should happen

The event stream should show an interruption or stop-related event. The final timeline state should remain coherent rather than looking half-corrupted.

### What to pay attention to

This scenario is extremely valuable because interruptions are where architectures reveal whether they really understand their own event model. A strong implementation will preserve a believable final state even when the generation does not end normally.

---

### Try 4: compare the panels, not just the output text

Even if the message looks fine, examine:

- backend event stream,
- UI event stream,
- final timeline entity.

### What should happen

All three should tell consistent versions of the same story.

### What to pay attention to

This is how you train yourself not to treat the framework as a black box. The final text is not enough; you want to understand how the framework *arrived* at that text.

---

### Try 5: export the transcript

After a scenario, export the run.

### Why this matters

The exported transcript is useful as:

- an onboarding artifact,
- a debugging artifact,
- a regression fixture,
- documentation evidence.

The best labs are not only interactive; they are also portable.

---

## 9. What a coherent stop/cancel path teaches you

One of the most valuable things this phase can teach an intern is that a system's quality often becomes visible not on the happy path but on the interrupted path.

A start-only and finish-only story is easy. The more revealing story is what happens when the message begins, makes partial progress, and then stops.

Does the UI receive a coherent final event? Does the timeline entity settle into a believable state? Does the system treat interruption as part of the model, or as an awkward exception?

The stronger the architecture, the more interruption feels like a normal part of the event language rather than an embarrassing edge case.

---

## 10. The relationship between this phase and the example package boundary

This is the phase where a new engineer is most likely to accidentally put code in the wrong package. The temptation is understandable. If a chat helper feels useful and generic enough, why not move it into `pkg/evtstream`?

The answer is that convenience is not the same thing as generic truth.

If a helper only makes sense because you are dealing with:

- chat messages,
- inference starts and stops,
- token delta reduction,
- message lifecycle semantics,

then it almost certainly belongs in the chat example layer rather than the substrate core.

The framework should remain capable of supporting chat, not defined by chat.

---

## 11. Important file references and expected package ownership

### Expected example-side files or areas

You should expect this phase to introduce or expand code in places like:

- `pinocchio/pkg/evtstream/examples/chat/...` or similar example package tree
- Systemlab Phase 4 page files under:
  - `static/partials/phase4.html`
  - `static/js/pages/phase4.js`
- this chapter file:
  - `pinocchio/cmd/evtstream-systemlab/chapters/phase-4-chat-example.md`

### Framework files still relevant

Even though the example is outside the core, these substrate files remain central to understanding the run:

- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/fanout.go`
- `pinocchio/pkg/evtstream/hydration.go`

The example should plug into these, not reinvent them.

---

## 12. The bugs this phase should help prevent

### Bug class 1: example code quietly becoming framework code

This is the big package-boundary danger of Phase 4.

### Bug class 2: UI and timeline drifting apart

If the chat message visible in the UI does not match the final timeline entity, the architecture is no longer coherent.

### Bug class 3: stop/cancel leaving broken final state

If interruption produces nonsense state, the event model is not robust enough.

### Bug class 4: product demo replacing debug surface

If the page becomes flashy but stops being explanatory, it has failed as Systemlab.

### Bug class 5: chat-specific message shapes leaking into generic transport or consumer code

That would damage the framework's reusability.

---

## 13. How a reviewer should approach this phase

Once the page is active, a good review order is:

1. read the example package registration and handlers,
2. inspect the example's projections,
3. use the Systemlab page with a happy-path prompt,
4. run a stop/cancel path,
5. compare backend events, UI events, and timeline final state,
6. export a transcript.

The important thing here is not just static code review. This phase is meant to be exercised.

---

## 14. Final summary

Phase 4 is where the framework finally proves that its abstractions can support something that feels like a real product interaction without giving up architectural clarity.

That is a delicate balance. The example needs to feel concrete enough to be persuasive, but not so convenient that it starts redefining the substrate itself. It needs to feel like a chat example built on top of `evtstream`, not like `evtstream` secretly admitting it was always a chat framework.

When done well, this phase becomes one of the most welcoming parts of the whole project for a new intern. It lets them see the architecture not only as a set of rules, but as a set of rules that can produce a familiar, satisfying behavior.

---

## 15. File references at a glance

### Substrate references

- `pinocchio/pkg/evtstream/hub.go`
- `pinocchio/pkg/evtstream/projection.go`
- `pinocchio/pkg/evtstream/hydration.go`
- `pinocchio/pkg/evtstream/fanout.go`

### Example/Systemlab references

- expected chat example package tree under `pinocchio/pkg/evtstream/examples/chat/` or equivalent
- future Phase 4 partial and JS page module
- `pinocchio/cmd/evtstream-systemlab/chapters/phase-4-chat-example.md`

### The main review question

At every point in this phase, ask:

> Is this code teaching how to build on the framework, or is it quietly turning the framework into the example?
