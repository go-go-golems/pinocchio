# Phase 4 — Chat Example

## What this chapter is about

Phase 1 showed you the framework working with a synthetic lab. Phase 4 shows you the framework working with a real example: chat. Not because the framework is a chat framework, but because chat is a teachable example of streamed activity becoming both live UI and durable state.

By the end of this chapter, you should understand how application code sits on top of the framework, how the same command → handler → events → projections pattern works in a real example, and why chat logic stays in example code, not the framework.

---

## 1. Why chat as the first example?

The framework could support many kinds of applications. Chat is the first example because it has exactly the properties the framework handles well:

- A command initiates work.
- Backend events stream in (tokens, chunks).
- Live UI updates show progress.
- A timeline entity accumulates the final result.
- Stop/cancel behavior is meaningful.

Chat is not a flashy demo. It is a rich teaching backend that forces every framework concept to prove itself in a recognizable context.

---

## 2. The framework and the example are separate

This is the most important rule of Phase 4:

> The chat example is a consumer of `evtstream`, not part of `evtstream`.

```text
pkg/evtstream          = generic substrate
examples/chat         = application on top of substrate
```

Chat-specific items stay in example code:

- `StartInference`, `StopInference` commands
- `TokensDelta`, `InferenceFinished` backend events
- `MessageStarted`, `MessageAppended` UI events
- `ChatMessage` timeline entities

These do not belong in `pkg/evtstream`. If chat logic leaks into the substrate, the framework stops being reusable.

---

## 3. The chat flow

Here is what happens when a user sends a chat message:

```text
User sends StartInference(prompt)
         ↓
Handler begins streaming
         ↓
Backend events:
  InferenceStarted
  TokensDelta (multiple)
  InferenceFinished
         ↓
UIProjection produces:
  MessageStarted
  MessageAppended (multiple)
  MessageFinished
         ↓
TimelineProjection reduces to:
  ChatMessage { text: accumulated, status: finished }
         ↓
HydrationStore records the ChatMessage
```

This is the same pattern as Phase 1, but in a recognizable context.

---

## 4. Why two projections makes sense in chat

The UI projection answers: what should a user watching live see right now?

- Message started.
- Text accumulating.
- Message finished.

The timeline projection answers: what should the system remember?

- One `ChatMessage` entity with accumulated text.

You can feel the difference:

- Live UI is about the unfolding experience.
- Timeline is about the remembered result.

That is why the framework treats them as separate. Collapsing them into one projection loses this distinction.

---

## 5. Stop behavior

Real chat supports stopping a response mid-stream. Here is how it works in the framework:

```text
User clicks Stop
         ↓
Handler receives StopInference
         ↓
Backend event: InferenceStopped
         ↓
UIProjection produces: MessageStopped
         ↓
TimelineProjection updates:
  ChatMessage { text: accumulated-so-far, status: stopped }
```

The key insight: stop is not an error. It is a normal part of the event language. The framework handles it the same way it handles normal finish.

---

## 6. The Phase 4 page

The Phase 4 page simulates chat behavior. It shows:

- prompt input and session selector
- scenario presets
- backend event trace
- UI event trace
- timeline entity evolution
- send and stop controls

The page should answer:

1. What command was sent?
2. What backend events did it create?
3. What did the live UI derive?
4. What final timeline entity did the store retain?
5. What happens when you click Stop?

---

## 7. Things to try

**Send a prompt.** Watch the backend event stream, the UI event stream, and the timeline entity evolve in parallel.

**Send a longer prompt.** More tokens, more UI events, more accumulated text. Notice how all three panels tell the same story from different angles.

**Click Stop mid-stream.** The UI shows MessageStopped. The timeline shows the message with accumulated text so far. The framework treated stop as normal, not as an error.

**Compare the panels.** Backend events, UI events, and final timeline should all be consistent. If they drift apart, something is wrong with the architecture.

**Export.** The exported transcript is a useful artifact for onboarding and debugging.

---

## Key Points

- The chat example is a consumer of `evtstream`, not part of it. Chat logic stays in example code.
- The same patterns from Phase 1 apply here: command → handler → backend events → UI events + timeline entities.
- The UI projection produces transient live updates. The timeline projection produces durable state.
- Stop behavior is normal, not exceptional. The framework handles it the same way as finish.
- Compare the panels. Consistent panels mean the architecture is working.

---

## API Reference

### Commands

- `StartInference`: Initiate chat response.
- `StopInference`: Stop a response mid-stream.

### Backend events

- `InferenceStarted`: Response beginning.
- `TokensDelta`: Token or chunk received.
- `InferenceFinished`: Response complete.
- `InferenceStopped`: Response interrupted.

### UI events

- `MessageStarted`: UI received start.
- `MessageAppended`: UI received token.
- `MessageFinished`: UI received finish.
- `MessageStopped`: UI received stop.

---

## File References

### Framework files

- `pkg/evtstream/hub.go` — command routing
- `pkg/evtstream/projection.go` — projection interfaces
- `pkg/evtstream/fanout.go` — UI output
- `pkg/evtstream/hydration.go` — hydration store

### Example files

- `pkg/evtstream/examples/chat/` — chat example package

### Systemlab files

- `cmd/evtstream-systemlab/static/partials/phase4.html`
- `cmd/evtstream-systemlab/static/js/pages/phase4.js`