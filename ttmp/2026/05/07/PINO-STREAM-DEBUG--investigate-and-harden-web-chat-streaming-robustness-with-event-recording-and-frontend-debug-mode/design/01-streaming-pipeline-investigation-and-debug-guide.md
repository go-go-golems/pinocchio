# Streaming Pipeline Investigation and Debug Guide

This document explains every layer of the Pinocchio web-chat streaming pipeline in enough detail for a new intern to understand the data flow, identify where things can go wrong, and build tools that surface discrepancies between what the backend emitted and what the frontend rendered.

It is organized as a walk from the user's "Submit" button all the way down to the Redux store and back up to the screen. Each section explains what the code does, why it does it that way, where the common failure modes are, and what a debug tool should capture at that layer.

---

## Table of Contents

1. [The Big Picture](#the-big-picture)
2. [Layer 1: The Submit Button and the Inference Request](#layer-1-the-submit-button-and-the-inference-request)
3. [Layer 2: The Chatapp Engine and Backend Event Emission](#layer-2-the-chatapp-engine-and-backend-event-emission)
4. [Layer 3: The SessionStream Service](#layer-3-the-sessionstream-service)
5. [Layer 4: WebSocket Transport](#layer-4-websocket-transport)
6. [Layer 5: Frontend Frame Parsing](#layer-5-frontend-frame-parsing)
7. [Layer 6: Hydration and Snapshot Application](#layer-6-hydration-and-snapshot-application)
8. [Layer 7: UI Event Projection and Timeline Mutation](#layer-7-ui-event-projection-and-timeline-mutation)
9. [Layer 8: Redux State and Entity Ordering](#layer-8-redux-state-and-entity-ordering)
10. [Layer 9: Rendering](#layer-9-rendering)
11. [What Can Go Wrong: A Failure Mode Catalog](#what-can-go-wrong-a-failure-mode-catalog)
12. [Backend Event Recording Design](#backend-event-recording-design)
13. [Frontend Debug Mode Design](#frontend-debug-mode-design)
14. [Reconciliation and Comparison Tools](#reconciliation-and-comparison-tools)
15. [Test Scenarios](#test-scenarios)
16. [File Reference](#file-reference)
17. [API Reference](#api-reference)

---

## The Big Picture

When a user types a prompt and presses Enter in the Pinocchio web-chat, a chain of transformations begins. The prompt travels from the browser to a Go backend. The backend runs inference through a Geppetto engine. As the model generates tokens, the backend publishes events. Those events flow through a sessionstream service, which projects them into UI events and persists timeline entities. The UI events travel over a WebSocket to the browser. The browser parses them, updates a Redux store, and React renders the result.

The key insight is that there are **four distinct representations** of the same conversation at any given moment:

1. **Backend events** — the raw emissions from the inference engine and feature plugins.
2. **Timeline entities** — the persisted, durable state that survives page reloads.
3. **UI events** — the ephemeral signals that tell the frontend what changed.
4. **Redux entities** — the in-browser state that React renders.

A bug can appear when any of these representations diverge from the others. The most common symptom is: "I sent a prompt, the model replied, but after reload something is missing or in the wrong order."

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Inference   │────>│  Backend     │────>│  Session     │
│   Engine      │     │  Events      │     │  Stream      │
│  (Geppetto)   │     │  (protobuf)  │     │  Service     │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                  │
                                     ┌────────────┴────────────┐
                                     │                         │
                               ┌─────▼─────┐           ┌──────▼─────┐
                               │  Timeline  │           │  UI Events │
                               │  Entities  │           │  (ephemeral)│
                               │  (durable) │           └──────┬─────┘
                               └─────┬─────┘                  │
                                     │              ┌─────────▼─────────┐
                                     │              │   WebSocket       │
                                     │              │   Transport       │
                                     │              └─────────┬─────────┘
                                     │                        │
                               ┌─────▼─────┐          ┌──────▼──────┐
                               │  Snapshot  │◄─────────│  Frontend   │
                               │  (hydrate) │          │  Parser     │
                               └─────┬─────┘          └──────┬──────┘
                                     │                        │
                               ┌─────▼─────┐          ┌──────▼──────┐
                               │  Redux     │◄─────────│  Timeline   │
                               │  Store     │          │  Slice      │
                               └─────┬─────┘          └─────────────┘
                                     │
                               ┌─────▼─────┐
                               │  React     │
                               │  Render    │
                               └───────────┘
```

Every arrow in this diagram is a place where things can go wrong. The debug tools this ticket creates will instrument every arrow.

## Layer 1: The Submit Button and the Inference Request

When the user types text into the composer and presses Enter (or clicks Send), the `ChatWidget` component in `cmd/web-chat/web/src/webchat/ChatWidget.tsx` runs its `send` callback. This callback does three things in sequence:

1. **Ensures a session exists.** If there is no `convId` in the Redux `app` slice, it calls `POST /api/chat/sessions` to create one. The server returns `{ sessionId: "..." }` which the frontend stores in Redux and writes to the URL query string via `window.history.replaceState`.

2. **Ensures the WebSocket is connected.** It calls `wsManager.connect(...)` with the session ID. If the WebSocket is already open for the same session, this is a no-op. Otherwise, it opens a new connection and subscribes.

3. **Posts the prompt.** It calls `POST /api/chat/sessions/${sessionId}/messages` with a JSON body `{ prompt, profile }`. The server acknowledges with `{ status: "running" }`.

The important thing to notice is that the prompt submission is a **regular HTTP POST**, not a WebSocket message. The WebSocket is used only for receiving streaming responses. This design means the prompt path and the response path are separate transports, which has implications for debugging: a prompt can succeed (200 OK) even if the WebSocket is disconnected, and vice versa.

```typescript
// Simplified from ChatWidget.tsx send()
const res = await fetch(`${basePrefix}/api/chat/sessions/${sessionId}/messages`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ prompt, profile: selectedProfile }),
});
```

**What can go wrong here:**

- The session creation POST fails (network error, server down). The user sees an error in the error panel but the WebSocket may already be connecting.
- The WebSocket connects before the session is created, causing a subscribe for an empty session ID.
- The prompt POST returns 200 but the backend never publishes events because the runtime resolver returned an error that was swallowed.
- The profile selector shows a stale profile if the profile mutation is still in flight.

**What the debug tool should capture:**

- Timestamp of the submit button press.
- The session ID used (newly created or existing).
- The prompt text.
- The HTTP response status and body from the POST.

## Layer 2: The Chatapp Engine and Backend Event Emission

The HTTP handler for `POST /sessions/{id}/messages` calls `chatapp.Engine.SubmitPrompt()`. The engine is defined in `pkg/chatapp/chat.go`. Its job is to:

1. Accept the prompt text and resolve a runtime (Geppetto engine + turn store persister + history loader).
2. Create or load a Geppetto session with the conversation history.
3. Run inference in a goroutine.
4. Publish backend events as the inference progresses.

The engine publishes events by calling `e.publish(ctx, sid, pub, eventName, payload)`. The `pub` function is a sessionstream event publisher that wraps `sessionstream.Service.PublishBackendEvent()`. Each event has:

- A **session ID** (which session this event belongs to).
- An **event name** (e.g., `ChatInferenceStarted`, `ChatMessageAppended`, `ChatReasoningFinished`).
- A **payload** (a protobuf message, e.g., `ChatMessageUpdate`, `ReasoningUpdate`).
- An **ordinal** (assigned by the sessionstream service, strictly increasing per session).

The core inference loop looks like this (simplified):

```go
// In chatapp/chat.go, runRuntimeInference()
e.publish(ctx, sid, pub, EventInferenceStarted, started)

for iteration := 0; iteration < maxIterations; iteration++ {
    result, err := runtime.RunInference(ctx, accumulatedTurn)
    if err != nil {
        e.publish(ctx, sid, pub, EventInferenceStopped, stopped)
        return
    }

    // Publish streaming text events
    for _, block := range result.Blocks {
        if block.Kind == "reasoning" {
            e.handleFeatureRuntimeEvent(ctx, sid, messageID, pub, reasoningEvent)
        }
        // ... tool calls, etc.
    }

    e.publish(ctx, sid, pub, EventInferenceFinished, finished)
}
```

**Feature plugins** sit between the raw Geppetto events and the published backend events. The `ChatPlugin` interface (in `pkg/chatapp/features.go`) has three methods:

- `HandleRuntimeEvent` — translates a Geppetto engine event into one or more published backend events.
- `ProjectUI` — translates a backend event into UI events for the frontend.
- `ProjectTimeline` — translates a backend event into durable timeline entities.

The engine calls `HandleRuntimeEvent` on every plugin for every Geppetto event. The first plugin that returns `handled: true` wins — plugins are tried in registration order. This means plugin ordering matters, and a bug in one plugin can silently swallow events that another plugin would have handled.

There are currently three plugins:

| Plugin | File | Handles |
|--------|------|---------|
| Reasoning | `pkg/chatapp/plugins/reasoning.go` | `EventReasoningStarted`, `EventReasoningDelta`, `EventReasoningFinished` |
| ToolCall | `pkg/chatapp/plugins/toolcall.go` | `EventToolCallStarted`, `EventToolCallUpdated`, `EventToolCallFinished`, `EventToolResultReady` |
| AgentMode | `cmd/web-chat/agentmode_chat_feature.go` | `EventModeSwitchPreview`, `EventAgentModeSwitch` |

Each plugin registers its events, UI events, and timeline entities with the `sessionstream.SchemaRegistry` at startup. The registry maps event names to protobuf message types so the sessionstream service can serialize and deserialize them.

**What can go wrong here:**

- A Geppetto event is not handled by any plugin and becomes a silent drop.
- A plugin publishes the wrong event name or wrong payload type.
- The inference loop hits max iterations and publishes a warning `EventInferenceFinished` instead of `EventInferenceStopped`, confusing the frontend.
- A runtime resolver error is swallowed and only a `stopped` event is published without a corresponding `started` event, leaving the frontend in an inconsistent state.
- History loading fails and publishes a `stopped` with an error message, but the frontend has already shown a "user message" entity from the local optimistic update.

**What the debug tool should capture:**

- Every backend event published: ordinal, name, payload type, payload JSON.
- The Geppetto event that triggered each backend event (for traceability).
- Which plugin handled each event (or whether it was unhandled).
- Errors from the inference loop, including max-iteration warnings.

## Layer 3: The SessionStream Service

The sessionstream service is a standalone package (`sessionstream/pkg/sessionstream/`) that sits between the chatapp engine and the WebSocket transport. It does three things:

1. **Persists backend events** in an event log (SQLite `sessionstream_events` table). Each event gets a monotonically increasing ordinal per session.

2. **Projects backend events** into two derived streams:
   - **UI events** — ephemeral signals sent to connected WebSocket clients. Produced by `ProjectUI`.
   - **Timeline entities** — durable state persisted for hydration. Produced by `ProjectTimeline`.

3. **Manages WebSocket subscriptions** — fans out UI events to all connected clients subscribed to a session.

The projection pipeline works like this:

```
Backend Event
    │
    ├──► UI Projection (Engine.uiProjection)
    │       ├── base projection (chat messages)
    │       ├── reasoning plugin
    │       ├── toolcall plugin
    │       └── agentmode plugin
    │       │
    │       └──► UI Events → WebSocket → Frontend
    │
    └──► Timeline Projection (Engine.timelineProjection)
            ├── base projection (chat message entities)
            ├── reasoning plugin (chat message entities)
            ├── toolcall plugin (tool_call, tool_result entities)
            └── agentmode plugin (AgentMode entities)
            │
            └──► Timeline Entities → Hydration Store (SQLite)
```

A single backend event can produce **multiple** UI events and **multiple** timeline entities. For example, `ChatAgentModeCommitted` produces:
- 1 UI event: `ChatAgentModeCommitted`
- 1 UI event: `ChatAgentModePreviewCleared`
- 1 timeline entity: `AgentMode` (with a unique entity ID based on the message ID)

The UI events and timeline entities are produced independently. They can have different payloads and different shapes. The frontend receives UI events over the WebSocket but hydrates from timeline entities. This means there are **two independent paths** to the same visual state, and they must agree.

**What can go wrong here:**

- A backend event is projected to UI events but not to timeline entities (or vice versa). The live stream looks correct but after reload, the entity is missing.
- A timeline entity is projected with the wrong entity ID, causing it to overwrite a different entity on upsert.
- A projection returns an error that is logged but not surfaced to the frontend.
- The projection cursor (tracking which ordinals have been materialized) gets out of sync, causing events to be skipped or replayed.
- Two plugins project timeline entities with the same `(kind, id)` key, and the second one silently overwrites the first.

**What the debug tool should capture:**

- Every UI event produced from each backend event: name, payload type, ordinal.
- Every timeline entity produced from each backend event: kind, ID, created ordinal, payload.
- Any projection errors.
- The mapping from backend event ordinal → UI events and timeline entities, so a comparison tool can verify completeness.

## Layer 4: WebSocket Transport

The Go backend serves a WebSocket endpoint at `GET /api/chat/ws`. The sessionstream service manages the WebSocket lifecycle. When a client connects, it receives:

1. A `hello` frame with the server protocol version.
2. After the client sends a `subscribe` frame with a session ID, it receives:
   - A `snapshot` frame containing all current timeline entities (the hydration payload).
   - A `subscribed` confirmation frame.
3. From then on, every UI event produced by the projection is serialized as a `uiEvent` frame and sent to all subscribed WebSocket connections.

The wire format is **protobuf JSON with oneof wrapper frames**. This is important: the raw JSON on the wire is not a flat event object. It is a wrapper with exactly one field set, corresponding to the protobuf oneof in `sessionstream.proto`. For example:

```json
{
  "uiEvent": {
    "sessionId": "...",
    "eventOrdinal": "215",
    "name": "ChatAgentModeCommitted",
    "payload": {
      "@type": "type.googleapis.com/pinocchio.chatapp.v1.AgentModeCommittedUpdate",
      "messageId": "chat-msg-7",
      "title": "agentmode: mode switched",
      "from": "financial_analyst",
      "to": "category_regexp_designer",
      "analysis": "..."
    }
  }
}
```

The `payload` field is a `google.protobuf.Any`, which means it has an `@type` field identifying the concrete message type, followed by the message fields. The frontend must parse the `@type` to know which protobuf schema to use.

**What can go wrong here:**

- A WebSocket frame is malformed (missing `@type`, wrong oneof field, non-JSON).
- A frame is dropped due to a slow consumer (WebSocket backpressure).
- Multiple tabs subscribe to the same session. Each tab gets its own copy of every UI event. If one tab disconnects and reconnects, it gets a fresh snapshot.
- The `eventOrdinal` field is a string, not a number. The frontend must parse it carefully.
- Large payloads (long assistant messages, big tool results) may be split across TCP packets. The WebSocket protocol handles framing, but intermediate proxies or load balancers may not.

**What the debug tool should capture:**

- Every raw WebSocket message received: timestamp, raw string (first 500 chars for large messages).
- The parsed frame type: `hello`, `snapshot`, `subscribed`, `uiEvent`, `error`, `ping`, `pong`.
- For `uiEvent` frames: the event ordinal, the event name, the `@type` of the payload.
- For `snapshot` frames: the snapshot ordinal, the number of entities, the time to process.
- WebSocket lifecycle events: open, close, error.

## Layer 5: Frontend Frame Parsing

All WebSocket frame parsing happens in `cmd/web-chat/web/src/ws/protocol.ts`. The core function is `parseServerFrame(raw: string): CanonicalFrame`. It:

1. Parses the raw JSON string.
2. Detects which oneof field is present: `hello`, `snapshot`, `subscribed`, `uiEvent`, `error`, `ping`, `pong`.
3. Normalizes each into a flat `CanonicalFrame` object with a `type` field.

The normalizer unwraps nested structures. For example, a raw `uiEvent` frame arrives as:

```json
{
  "uiEvent": {
    "sessionId": "...",
    "eventOrdinal": "215",
    "name": "ChatMessageAppended",
    "payload": { "@type": "...", "content": "Hello", "role": "assistant" }
  }
}
```

The normalizer produces:

```typescript
{
  type: "ui-event",
  sessionId: "...",
  ordinal: 215,
  name: "ChatMessageAppended",
  payload: { content: "Hello", role: "assistant" }  // unwrapped from Any
}
```

The `unwrapAnyPayload` function handles the `google.protobuf.Any` unwrapping. Concrete protobuf payloads arrive with their fields next to `@type`. Legacy `Struct` payloads arrive as `{ "@type": "...Struct", "value": { ... } }`. The function returns `value` if it exists, otherwise the top-level fields.

This is a critical parsing step. If the unwrap logic is wrong, the entire payload is lost and the UI event becomes a no-op.

**What can go wrong here:**

- A new protobuf message type is added to the backend but the frontend `unwrapAnyPayload` does not handle it correctly.
- A `Struct` payload has a `value` field that is not an object (e.g., a string or array), causing the unwrap to produce an empty object.
- The `eventOrdinal` is a large number string that loses precision when parsed as a JavaScript number. The `safeOrdinal` function handles this by checking `Number.isSafeInteger`.
- The frame has an unexpected oneof field (e.g., a new server sends a frame type the frontend does not know about), and the normalizer returns the raw frame unchanged, which downstream code ignores.

**What the debug tool should capture:**

- Every raw frame and its parsed result side by side.
- Whether `unwrapAnyPayload` used the `value` path or the top-level path.
- Any frames where `safeOrdinal` returned `null` (ordinal overflow or parse failure).
- Any frames where the type was not recognized by the normalizer.

## Layer 6: Hydration and Snapshot Application

When the WebSocket connects and subscribes, the server sends a `snapshot` frame. This frame contains every timeline entity currently persisted for the session. The frontend applies the snapshot by:

1. Clearing the Redux timeline state (`timelineSlice.actions.clear()`).
2. Iterating over each entity in the snapshot and converting it from the raw protobuf JSON to a frontend `TimelineEntity`.
3. Upserting each entity into the Redux store.

The conversion happens in `timelineEntityFromSnapshotEntity()` in `wsManager.ts`. This function is the **bridge between the backend's protobuf entity model and the frontend's display model**. It maps backend entity kinds to frontend entity shapes:

| Backend Kind | Frontend Kind | ID Source |
|-------------|--------------|-----------|
| `ChatMessage` | `message` | `payload.messageId` or entity ID |
| `AgentMode` | `agent_mode` | entity ID (now message-based) |
| — (preview) | `agent_mode_preview` | `agent-mode-preview:{messageId}` |

The function also unwraps the payload. For `AgentMode` entities, it checks whether `payload.data` is a non-empty object (new typed protobuf format) and falls back to extracting `from`, `to`, `analysis` from the top-level payload (old `Struct` format). This dual-path unwrapping is a historical artifact from the typed protobuf migration.

**What can go wrong here:**

- The snapshot entity has a kind the frontend does not recognize. The function returns `null` and the entity is silently dropped.
- The payload unwrap produces an empty object because neither `data` nor the top-level fields are present.
- The entity ID is empty or null, causing the function to return `null`.
- A `ChatMessage` entity has no `messageId` in the payload, so it falls back to the entity ID, which might be wrong for the frontend's entity model.
- The snapshot is applied, but then buffered UI events are replayed on top of it. If a buffered UI event has a different entity ID for the same logical entity, the result is a duplicate.
- A second tab connects to the same session and receives a snapshot while the first tab is still streaming. The first tab's live state and the second tab's hydrated state may differ if projections have advanced between the two connections.

**What the debug tool should capture:**

- The raw snapshot frame: number of entities, snapshot ordinal.
- Each entity in the snapshot: kind, ID, whether `timelineEntityFromSnapshotEntity` returned null (dropped).
- For each entity: which unwrap path was used (`data` or top-level fields).
- The number of buffered UI events that were replayed after the snapshot.
- The time from snapshot receipt to Redux state update complete.

## Layer 7: UI Event Projection and Timeline Mutation

After hydration, every incoming `ui-event` frame is processed by `applyUIEvent()` in `wsManager.ts`. This function calls `timelineMutationFromUIEvent()` to convert the frame into a `TimelineMutation`:

```typescript
type TimelineMutation = {
  upsert?: TimelineEntity;   // insert or update an entity
  deleteId?: string;          // remove an entity
  status?: string;            // update the app-level status
};
```

The `timelineMutationFromUIEvent` function is a large switch statement that maps UI event names to Redux mutations. Here is the complete mapping:

| UI Event Name | Mutation |
|--------------|----------|
| `ChatMessageAccepted` | upsert `message` (role=user, status=submitted) |
| `ChatMessageStarted` | upsert `message` (role=assistant, status=streaming) + status=streaming |
| `ChatMessageAppended` | upsert `message` (role=assistant, status=streaming) + status=streaming |
| `ChatMessageFinished` | upsert `message` (role=assistant, status=finished) + status=finished |
| `ChatMessageStopped` | upsert `message` (role=assistant, status=stopped) + status=stopped |
| `ChatReasoningStarted` | upsert `message` (role=thinking, status=streaming) + status=streaming |
| `ChatReasoningAppended` | upsert `message` (role=thinking, status=streaming) + status=streaming |
| `ChatReasoningFinished` | upsert `message` (role=thinking, status=finished) |
| `ChatAgentModePreviewUpdated` | upsert `agent_mode_preview` |
| `ChatAgentModeCommitted` | upsert `agent_mode` |
| `ChatAgentModePreviewCleared` | delete `agent_mode_preview` |

**Critical detail about entity IDs during live streaming:**

During live streaming, the entity ID used for `upsert` is critical. For `ChatMessageStarted` and `ChatMessageAppended`, the ID is `messageId` from the payload. For `ChatReasoningStarted`, the ID is also `messageId`. This means the reasoning entity and the assistant message entity have **different IDs** — the reasoning block gets its own ID (e.g., `chat-msg-2-thinking`) separate from the assistant message (e.g., `chat-msg-2`).

But during hydration, the backend stores reasoning as a `ChatMessage` entity with `role=thinking` and a different entity ID. The frontend's `timelineEntityFromSnapshotEntity` function maps it to a `message` entity with `role=thinking`. The entity IDs in the snapshot come from the timeline projection, which uses the message ID as the entity ID.

This means there is a **semantic difference** between how live streaming creates entities and how hydration restores them. Live streaming uses the message ID directly. Hydration uses whatever entity ID the timeline projection assigned.

**What can go wrong here:**

- A UI event name is not handled by the switch statement. The mutation returns `null` and the event is silently ignored.
- The `messageId` is missing from the payload, causing the mutation to return `null`.
- Two different UI events upsert entities with the same ID but different kinds (e.g., a reasoning entity and an assistant message both using `chat-msg-2`). The second upsert overwrites the first.
- A `ChatMessageStarted` event has no `content` field, so the upsert is skipped (the function returns only a status mutation). If no subsequent `ChatMessageAppended` event arrives, the assistant message never appears.
- The `status` mutation sets the app-level status to `streaming` but the corresponding `ChatMessageFinished` event sets it to `finished`. If the finished event is missed (WebSocket disconnect), the app stays in `streaming` forever.

**What the debug tool should capture:**

- Every UI event processed: event name, message ID, ordinal.
- The resulting mutation: upsert (entity ID, kind, key props), delete, or status change.
- Any UI events that produced `null` mutations (unhandled events).
- The current app status after each mutation.

## Layer 8: Redux State and Entity Ordering

The Redux timeline state is defined in `cmd/web-chat/web/src/store/timelineSlice.ts`. It has two fields:

```typescript
type TimelineState = {
  byId: Record<string, TimelineEntity>;  // entity ID → entity
  order: string[];                        // entity IDs in insertion order
};
```

The `upsertEntity` reducer is the most important operation. It handles both insert (new entity) and update (existing entity) cases. The ordering rules are:

- **New entity**: Add to `byId` and append ID to `order`.
- **Existing entity, with version**: If the incoming version is greater than the stored version, replace. If less, ignore. This is the "versioned" path for entities that carry explicit version numbers.
- **Existing entity, without version**: Merge props, preserve `createdAt`, update `updatedAt`. The entity keeps its position in `order` (no re-ordering).

The `createdAt` field is **never overwritten** by an upsert of an existing entity. This is a deliberate design choice: `createdAt` represents when the entity first appeared, and subsequent updates should not change that.

The `order` array determines the rendering order. Entities are rendered in the order they were first inserted. This means:

1. A tool call entity inserted at ordinal 10 appears before a tool result entity inserted at ordinal 20.
2. If a tool call completed event arrives at ordinal 21 and upserts the tool call entity, the entity stays at its original position in `order` (position of ordinal 10).
3. This prevents the bug where a late-arriving update causes a tool call to jump after its result.

**What can go wrong here:**

- An entity is upserted with the wrong ID, creating a duplicate instead of updating the original.
- The `rekeyEntity` action is used incorrectly, leaving stale IDs in the order array.
- An entity is deleted but its ID remains in the `order` array (the `deleteEntity` reducer does clean this up, but a custom reducer might not).
- Two entities with the same ID but different kinds compete for the same slot. The last upsert wins, changing the kind and potentially the renderer.
- The `order` array grows without bound for long sessions, causing React to re-render the entire timeline on every update.

**What the debug tool should capture:**

- Every Redux action dispatched: type, payload entity ID/kind.
- The resulting state change: added, updated, or deleted entity; new order length.
- Any upserts where the incoming version was less than the stored version (ignored stale update).
- The current `order` array after each mutation, for ordering comparison.

## Layer 9: Rendering

The `ChatTimeline` component in `cmd/web-chat/web/src/webchat/components/Timeline.tsx` renders the timeline. It receives the ordered list of entities from the Redux selector `selectTimelineEntities` and maps each entity to a renderer.

The renderer registry is built in `cmd/web-chat/web/src/webchat/rendererRegistry.ts`. The default renderers are defined in `cmd/web-chat/web/src/webchat/cards.tsx`. The mapping is:

| Entity Kind | Renderer | Visual |
|------------|----------|--------|
| `message` (role=user) | UserMessageCard | User bubble with prompt text |
| `message` (role=assistant) | AssistantMessageCard | Assistant bubble with response text |
| `message` (role=thinking) | ThinkingCard | Collapsible reasoning block |
| `agent_mode` | AgentModeCard | Mode switch indicator with analysis bullets |
| `agent_mode_preview` | AgentModePreviewCard | Live preview of candidate mode |
| `tool_call` | ToolCallCard | Tool call with arguments |
| `tool_result` | ToolResultCard | Tool result with output |
| default | DefaultCard | Fallback: raw JSON dump |

The sticky scroll behavior is managed by `useStickyScrollFollow` in `cmd/web-chat/web/src/webchat/hooks/useStickyScrollFollow.ts`. This hook tracks whether the user has scrolled up from the bottom and auto-scrolls to the bottom when new entities arrive during streaming.

**What can go wrong here:**

- An entity has a kind that no renderer is registered for. The `DefaultCard` renders raw JSON, which is correct but ugly.
- An entity has the right kind but missing or malformed props (e.g., no `content` field on a message). The renderer shows an empty bubble.
- The sticky scroll hook does not detect the bottom correctly (browser rounding, CSS transforms, or dynamic heights).
- React re-renders the entire timeline on every entity upsert because the `entities` array reference changes. This can cause flickering or performance issues in long conversations.
- A deleted entity (`agent_mode_preview` after `ChatAgentModePreviewCleared`) leaves a brief flash before React removes it from the DOM.

**What the debug tool should capture:**

- The list of rendered entity IDs and their kinds after each update.
- The scroll position and sticky mode after each entity update.
- Any entities that fell through to the default renderer.

## What Can Go Wrong: A Failure Mode Catalog

This section lists the specific failure modes that have been observed or are plausible, organized by where they manifest.

### Missing entities after reload

**Symptom**: After page reload, one or more entities that were visible during live streaming are gone.

**Possible causes**:
1. The timeline projection did not create a timeline entity for the event. The live stream used the UI event directly, but the hydration path requires a timeline entity.
2. The timeline entity was created with the wrong entity ID, overwriting a different entity.
3. The hydration store failed to persist the entity (SQLite error, disk full).
4. The snapshot frame was sent before the projection cursor advanced past the event.

**How to diagnose**: Compare the backend events for the session (Layer 2) with the timeline entities in the hydration store (Layer 3). The entity should exist in the store for every backend event that `ProjectTimeline` handles.

### Wrong entity ordering

**Symptom**: Tool results appear before tool calls, or thinking blocks appear after the assistant response.

**Possible causes**:
1. The `upsertEntity` reducer overwrote `createdAt` with a later timestamp, moving the entity to the wrong position in the `order` array. (Fixed by preserving `createdAt` on upsert.)
2. A live UI event used a different entity ID than the hydration snapshot, creating a duplicate instead of updating the original.
3. The backend emitted events out of order (e.g., tool result before tool call finished).

**How to diagnose**: Record the entity IDs and `createdAt` values before and after each upsert during live streaming. Compare with the snapshot entities after reload.

### Duplicate entities

**Symptom**: The same message or thinking block appears twice in the timeline.

**Possible causes**:
1. The hydration snapshot contains the entity, and then a buffered UI event re-creates it with a different ID.
2. A `rekeyEntity` action failed to remove the old ID from the `order` array.
3. The backend projected two timeline entities for the same logical item with different IDs.

**How to diagnose**: Check the `byId` map for entities with identical `kind` and overlapping `props` but different IDs.

### Stuck in streaming state

**Symptom**: The app status shows "streaming" and never transitions to "finished" or "stopped".

**Possible causes**:
1. The `ChatMessageFinished` or `ChatMessageStopped` event was lost (WebSocket disconnect, frame dropped).
2. The event arrived but the mutation returned `null` because `messageId` was missing.
3. The event arrived but the status mutation was not applied because the Redux dispatch failed silently.

**How to diagnose**: Record every UI event that sets `status: "streaming"`. Verify that a corresponding `status: "finished"` or `status: "stopped"` event exists with the same or later ordinal.

### Hydration mismatch with live state

**Symptom**: After reload, the conversation looks different than it did before reload. Missing entities, different ordering, or extra entities.

**Possible causes**:
1. The snapshot was taken at an earlier ordinal than the last UI event the frontend processed. Events between the snapshot ordinal and the last UI event are lost.
2. The snapshot and the live stream use different entity IDs for the same logical entity.
3. The `timelineEntityFromSnapshotEntity` function drops entities that `timelineMutationFromUIEvent` creates, or vice versa.

**How to diagnose**: Record the snapshot ordinal and the last UI event ordinal before disconnect. After reconnect, compare the snapshot entity list with the pre-disconnect entity list.

### Second tab divergence

**Symptom**: Two tabs connected to the same session show different content.

**Possible causes**:
1. The second tab connected after the first tab had already processed several UI events. The second tab's snapshot is from an earlier ordinal.
2. One tab reconnected and received a fresh snapshot while the other continued from live events.
3. The `WsManager` is a singleton. The second tab's connect call disconnects the first tab's WebSocket.

**How to diagnose**: Record WebSocket lifecycle events per tab. Check if a `disconnect` was issued for the first tab when the second tab connected.

## Backend Event Recording Design

The backend already persists backend events in the `sessionstream_events` SQLite table. This table has columns: `(session_id, ordinal, name, payload_json, created_at)`. It is the authoritative record of what the backend emitted.

The existing table is sufficient for most investigation. What is missing is:

1. **A convenient API to query it.** The current `GET /api/debug/sessions/{id}/events` endpoint (if it exists) or a new one should return events filtered by session, with optional filtering by event name and ordinal range.

2. **Projection traceability.** For each backend event, we should be able to see which UI events and timeline entities it produced. This requires recording the projection output alongside the backend event.

### Proposed schema additions

```sql
-- Existing table (already exists in sessionstream)
CREATE TABLE sessionstream_events (
    session_id TEXT NOT NULL,
    ordinal INTEGER NOT NULL,
    name TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY(session_id, ordinal)
);

-- New: projection trace table
CREATE TABLE debug_projection_trace (
    session_id TEXT NOT NULL,
    backend_ordinal INTEGER NOT NULL,
    projection_type TEXT NOT NULL,  -- "ui" or "timeline"
    output_index INTEGER NOT NULL,  -- 0, 1, 2... for multiple outputs
    output_name TEXT NOT NULL,      -- UI event name or timeline entity kind
    output_id TEXT,                 -- timeline entity ID (null for UI events)
    output_payload_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY(session_id, backend_ordinal, projection_type, output_index)
);
```

### Proposed API

```text
GET /api/debug/sessions/{id}/events?after=0&limit=1000&name=ChatMessage
GET /api/debug/sessions/{id}/projections?after=0&limit=1000
GET /api/debug/sessions/{id}/trace?after=0&limit=100
```

The `trace` endpoint joins backend events with projection outputs, producing a single response that shows the full transformation chain for each ordinal.

### Implementation approach

The projection trace recording should be a thin wrapper around the existing `Engine.uiProjection()` and `Engine.timelineProjection()` methods. It should be opt-in (only when `--debug-api` is enabled) and should not affect the hot path when disabled.

```go
// Pseudocode for trace recording
func (e *Engine) uiProjectionWithTrace(ctx context.Context, ev sessionstream.Event, ...) ([]sessionstream.UIEvent, error) {
    projected, err := e.uiProjection(ctx, ev, sess, view)
    if err != nil { return nil, err }
    if e.debugRecorder != nil {
        for i, ui := range projected {
            e.debugRecorder.RecordProjection(ctx, ev.SessionId, ev.Ordinal, "ui", i, ui.Name, "", ui.Payload)
        }
    }
    return projected, nil
}
```

## Frontend Debug Mode Design

The frontend debug mode captures everything that happens inside the browser: raw WebSocket frames, parsed frames, hydration snapshots, UI event mutations, and Redux state changes. It is activated by setting a flag in localStorage:

```javascript
localStorage.setItem('pinocchio.debugStream', '1')
```

When active, the `WsManager` records every frame it processes into an in-memory ring buffer. The buffer has a configurable maximum size (default: 10,000 entries). When the buffer fills, it drops the oldest entries.

### Log entry types

```typescript
type DebugLogEntry =
  | { type: 'raw-ws'; timestamp: number; data: string }
  | { type: 'parsed-frame'; timestamp: number; frame: CanonicalFrame }
  | { type: 'snapshot'; timestamp: number; entityCount: number; snapshotOrdinal: number; entities: Array<{kind: string; id: string; dropped: boolean}> }
  | { type: 'ui-event'; timestamp: number; ordinal: number; name: string; messageId: string; mutation: TimelineMutation | null }
  | { type: 'redux-action'; timestamp: number; actionType: string; entityId?: string; entityKind?: string }
  | { type: 'ws-lifecycle'; timestamp: number; event: 'open' | 'close' | 'error' | 'subscribe' }
  | { type: 'status'; timestamp: number; from: string; to: string };
```

### Debug panel

A collapsible overlay panel renders the debug log. It supports:

- **Filtering by type**: show only raw frames, parsed frames, mutations, or lifecycle events.
- **Filtering by entity ID**: show only events related to a specific entity.
- **Search**: full-text search across all log entries.
- **Export**: download the entire log as a JSON file for offline analysis.

The panel is rendered as a fixed-position overlay at the bottom of the screen, toggled by a keyboard shortcut (e.g., Ctrl+Shift+D) or a small button in the status bar.

### Implementation approach

The debug recorder is a class that wraps the existing `WsManager`. It intercepts calls to `handleFrame`, `applySnapshot`, and `applyUIEvent` and records the inputs and outputs.

```typescript
// Pseudocode for the debug recorder
class StreamDebugRecorder {
  private log: DebugLogEntry[] = [];
  private maxEntries: number = 10_000;

  record(entry: DebugLogEntry) {
    if (!localStorage.getItem('pinocchio.debugStream')) return;
    this.log.push(entry);
    if (this.log.length > this.maxEntries) {
      this.log.shift();
    }
  }

  export(): DebugLogEntry[] { return [...this.log]; }
  clear() { this.log = []; }
}

const debugRecorder = new StreamDebugRecorder();
```

The recorder is called from inside `WsManager.handleFrame()` and the projection functions. It is a no-op when the localStorage flag is not set.

### Hydration detail recording

When a snapshot arrives, the debug recorder should capture:

- The raw snapshot ordinal.
- The number of entities in the snapshot.
- For each entity: kind, ID, whether `timelineEntityFromSnapshotEntity` mapped it successfully or returned `null`.
- The number of buffered UI events that were replayed after the snapshot.
- The time from snapshot receipt to all entities being upserted.

This hydration detail is the most important debug output for investigating "missing after reload" bugs.

## Reconciliation and Comparison Tools

The reconciliation tool loads backend events and frontend debug logs for the same session and compares them. It runs as a script (or a debug API endpoint) and produces a report highlighting discrepancies.

### Comparison algorithm

```python
# Pseudocode for reconciliation
def reconcile(backend_events, frontend_log):
    # 1. Build ordinal-indexed maps
    backend_by_ordinal = {e.ordinal: e for e in backend_events}
    frontend_received = {e.ordinal for e in frontend_log if e.type == 'ui-event'}

    # 2. Find missing events (emitted but not received)
    missing = set(backend_by_ordinal.keys()) - frontend_received

    # 3. Find extra events (received but not emitted)
    extra = frontend_received - set(backend_by_ordinal.keys())

    # 4. Compare payloads for matching ordinals
    mismatches = []
    for ordinal in frontend_received & set(backend_by_ordinal.keys()):
        be = backend_by_ordinal[ordinal]
        fe = next(e for e in frontend_log if e.type == 'ui-event' and e.ordinal == ordinal)
        if be.name != fe.name:
            mismatches.append(('name', ordinal, be.name, fe.name))
        if be.payload['messageId'] != fe.payload.get('messageId'):
            mismatches.append(('messageId', ordinal, be.payload['messageId'], fe.payload.get('messageId')))

    return {
        'total_backend': len(backend_by_ordinal),
        'total_frontend': len(frontend_received),
        'missing': sorted(missing),
        'extra': sorted(extra),
        'mismatches': mismatches,
    }
```

### Output format

The reconciliation report is a JSON object:

```json
{
  "session_id": "...",
  "backend_event_count": 215,
  "frontend_event_count": 213,
  "missing_ordinals": [199, 200],
  "extra_ordinals": [],
  "payload_mismatches": [],
  "hydration_snapshot_ordinal": 215,
  "hydration_entity_count": 12,
  "final_redux_entity_count": 11,
  "entities_missing_from_redux": [
    {"kind": "AgentMode", "id": "chat-msg-7"}
  ]
}
```

### Comparison between hydration and live state

After a reload, the reconciliation tool should compare:

1. The entities in the snapshot frame (what the server sent).
2. The entities in the Redux store after hydration (what the frontend stored).
3. The entities in the Redux store before reload (what the frontend had during live streaming).

Any differences between these three sets indicate a hydration bug.

### Implementation approach

The reconciliation tool can be:

1. **A Go script** that reads the backend events from the SQLite database and the frontend debug log from an exported JSON file.
2. **A debug API endpoint** (`GET /api/debug/sessions/{id}/reconcile`) that reads both sources server-side.
3. **A browser-side tool** that runs inside the debug panel and compares the in-memory log with a fetched backend event list.

The first approach is the most practical for initial implementation because it does not require changes to the running server.

## Test Scenarios

Each scenario describes a specific user interaction pattern, what should happen, and what to look for in the debug logs.

### Scenario 1: Normal chat flow

1. Open Pinocchio web-chat.
2. Type a prompt and press Enter.
3. Wait for the response to complete (status changes to `finished`).

**What to verify:**

- The backend emitted `ChatInferenceStarted`, `ChatMessageAccepted`, `ChatMessageStarted`, one or more `ChatMessageAppended`, and `ChatMessageFinished`.
- The frontend received all of these as UI events.
- The Redux store has a user message entity and an assistant message entity.
- No entities were dropped during parsing.

### Scenario 2: Reload while streaming

1. Open Pinocchio web-chat.
2. Type a prompt and press Enter.
3. While the response is still streaming (status is `streaming`), press F5 to reload the page.

**What to verify:**

- The backend continues running inference (it does not know the browser reloaded).
- On reload, the frontend reconnects the WebSocket.
- The server sends a snapshot containing all timeline entities that existed at the time of reconnection.
- If inference completed while the browser was reloading, the snapshot includes the finished assistant message.
- The frontend applies the snapshot and then replays any buffered UI events.
- The final rendered state matches what a user who never reloaded would see.

**What can go wrong:**

- The snapshot is from an earlier ordinal than the last event the frontend processed before reload. Events between the snapshot and the disconnect are lost.
- The frontend does not clear the Redux store before applying the snapshot, causing duplicate entities from the optimistic local state plus the snapshot entities.

### Scenario 3: Open a second tab on the same conversation

1. Open Pinocchio web-chat in Tab A.
2. Send a prompt and wait for the response.
3. Open the same URL (with `sessionId` query parameter) in Tab B.

**What to verify:**

- Tab B receives a snapshot with all entities from Tab A's conversation.
- Tab B shows the same conversation as Tab A.

**What can go wrong:**

- The `WsManager` is a singleton. When Tab B calls `connect()`, it disconnects Tab A's WebSocket. Tab A stops receiving events.
- The backend sends different snapshots to each tab if they subscribe at different times and projections have advanced.

### Scenario 4: Reload second tab

1. Open Tab A and Tab B as in Scenario 3.
2. Send a new prompt in Tab A.
3. While the response is streaming in Tab A, reload Tab B.

**What to verify:**

- Tab B reconnects and receives a snapshot that includes the entities from Tab A's original conversation plus any new entities from the streaming response.
- Tab A continues streaming without interruption.

**What can go wrong:**

- Tab B's reconnect causes the `WsManager` singleton to disconnect Tab A. This is the same singleton issue as Scenario 3.
- The snapshot sent to Tab B is stale (from before the new prompt) because the projection cursor has not advanced past the new events yet.

### Scenario 5: Rapid sequential prompts

1. Open Pinocchio web-chat.
2. Send prompt A.
3. Immediately send prompt B without waiting for prompt A to complete.

**What to verify:**

- The backend queues or cancels prompt A and processes prompt B.
- The frontend does not mix entities from prompt A and prompt B.
- Each message ID is unique and correctly mapped to its prompt.

**What can go wrong:**

- The backend publishes a `ChatMessageStopped` for prompt A and a `ChatInferenceStarted` for prompt B. If the frontend processes these out of order (due to buffering or WebSocket frame ordering), the status display may flicker.
- Two inference loops run concurrently and publish events for different message IDs. The frontend must handle interleaved events correctly.

### Scenario 6: Network interruption

1. Open Pinocchio web-chat.
2. Send a prompt.
3. While streaming, disable the network (e.g., turn off Wi-Fi or use browser DevTools to go offline).
4. Wait 10 seconds.
5. Re-enable the network.

**What to verify:**

- The WebSocket detects the disconnect and fires `onclose`.
- The frontend shows a disconnected status.
- When the network returns, the frontend reconnects (if auto-reconnect is implemented) or the user manually reloads.
- After reconnect, the frontend receives a snapshot that is consistent with the backend's current state.

**What can go wrong:**

- The WebSocket does not detect the disconnect for a long time (TCP keepalive timeout). The frontend appears connected but receives no events.
- The backend continues processing and publishes events that no client receives. These events are persisted in the event log and timeline, so a future reconnect will see them.

## File Reference

### Backend (Go)

| File | Purpose |
|------|---------|
| `pkg/chatapp/chat.go` | Engine: submits prompts, runs inference, publishes backend events. |
| `pkg/chatapp/features.go` | `ChatPlugin` interface, plugin orchestration, projection routing. |
| `pkg/chatapp/plugins/reasoning.go` | Reasoning plugin: handles Geppetto reasoning events, projects to UI events and timeline entities. |
| `pkg/chatapp/plugins/toolcall.go` | Tool call plugin: handles tool call/result events, projects to UI events and timeline entities. |
| `cmd/web-chat/agentmode_chat_feature.go` | AgentMode plugin: handles mode switch preview/committed events. |
| `cmd/web-chat/app/server.go` | HTTP handler routing, WebSocket upgrade, session management. |
| `cmd/web-chat/app/server_export.go` | Export endpoints (timeline, turns, minitrace). |
| `cmd/web-chat/main.go` | CLI entry point, flag parsing, server setup. |

### Sessionstream (Go, separate module)

| File | Purpose |
|------|---------|
| `sessionstream/pkg/sessionstream/projection.go` | `UIProjection`, `TimelineProjection`, `UIEvent`, `TimelineEntity` types. |
| `sessionstream/pkg/sessionstream/types.go` | `Event`, `Session`, `Snapshot`, `SessionId` types. |
| `sessionstream/pkg/sessionstream/hydration.go` | `HydrationStore`, `EventStore`, `Snapshot` interfaces. |
| `sessionstream/pkg/sessionstream/schema.go` | `SchemaRegistry` for event/entity name → protobuf type mapping. |

### Frontend (TypeScript)

| File | Purpose |
|------|---------|
| `cmd/web-chat/web/src/ws/protocol.ts` | WebSocket frame parsing, normalization, protobuf Any unwrapping. |
| `cmd/web-chat/web/src/ws/wsManager.ts` | WebSocket lifecycle management, hydration, UI event dispatch, snapshot application. |
| `cmd/web-chat/web/src/store/timelineSlice.ts` | Redux slice for timeline entities: upsert, delete, rekey, clear. |
| `cmd/web-chat/web/src/store/appSlice.ts` | Redux slice for app state: convId, profile, status, wsStatus, lastSeq. |
| `cmd/web-chat/web/src/webchat/ChatWidget.tsx` | Main chat widget: connects WebSocket, sends prompts, manages state. |
| `cmd/web-chat/web/src/webchat/components/Timeline.tsx` | Timeline renderer: maps entities to renderers. |
| `cmd/web-chat/web/src/webchat/cards.tsx` | Card renderers for each entity kind. |
| `cmd/web-chat/web/src/webchat/hooks/useStickyScrollFollow.ts` | Sticky bottom-scroll hook. |
| `cmd/web-chat/web/src/webchat/rendererRegistry.ts` | Resolves entity kind → React component. |
| `cmd/web-chat/web/src/webchat/types.ts` | TypeScript types for entities, props, widget components. |

### Geppetto (Go, separate module)

| File | Purpose |
|------|---------|
| `geppetto/pkg/events/events.go` | Event types: `EventFinalEvent`, `EventAgentModeSwitch`, etc. |
| `geppetto/pkg/inference/session/session.go` | Session management, history loading. |
| `geppetto/pkg/turns/` | Turn accumulator model, block types, serialization. |

## API Reference

### HTTP Endpoints (Pinocchio web-chat)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/chat/sessions` | Create a new chat session. Returns `{ sessionId }`. |
| `POST` | `/api/chat/sessions/{id}/messages` | Submit a prompt. Body: `{ prompt, profile }`. Returns `{ status }`. |
| `GET` | `/api/chat/sessions/{id}` | Get current session snapshot (JSON). |
| `POST` | `/api/chat/sessions/{id}/stop` | Stop running inference. |
| `GET` | `/api/chat/ws` | WebSocket upgrade. Client sends `subscribe` frame after `hello`. |
| `GET` | `/api/chat/sessions/{id}/timeline` | Export timeline entities. Query: `?format=json\|yaml&download=true`. |
| `GET` | `/api/chat/sessions/{id}/turns` | Export turns. Query: `?format=json\|yaml\|minitrace&download=true`. |
| `GET` | `/api/chat/sessions/{id}/export` | Full export (timeline + turns). Query: `?format=json\|yaml&download=true`. |

### WebSocket Frame Types (Server → Client)

| Frame Type | Direction | Fields |
|-----------|-----------|--------|
| `hello` | Server → Client | `{ type: "hello" }` |
| `snapshot` | Server → Client | `{ type: "snapshot", sessionId, ordinal, entities[] }` |
| `subscribed` | Server → Client | `{ type: "subscribed", sessionId, ordinal }` |
| `uiEvent` | Server → Client | `{ type: "ui-event", sessionId, ordinal, name, payload }` |
| `error` | Server → Client | `{ type: "error", message, code, detail }` |
| `ping` | Server → Client | `{ type: "ping" }` |

### WebSocket Frame Types (Client → Server)

| Frame Type | Direction | Fields |
|-----------|-----------|--------|
| `subscribe` | Client → Server | `{ subscribe: { sessionId, sinceSnapshotOrdinal } }` |

### UI Event Names

| Event Name | Payload Type | Produces |
|-----------|-------------|----------|
| `ChatMessageAccepted` | `ChatMessageUpdate` | Upsert user message |
| `ChatMessageStarted` | `ChatMessageUpdate` | Upsert assistant message (streaming start) |
| `ChatMessageAppended` | `ChatMessageUpdate` | Upsert assistant message (text chunk) |
| `ChatMessageFinished` | `ChatMessageUpdate` | Upsert assistant message (complete) |
| `ChatMessageStopped` | `ChatMessageUpdate` | Upsert assistant message (stopped/error) |
| `ChatReasoningStarted` | `ReasoningUpdate` | Upsert thinking entity (start) |
| `ChatReasoningAppended` | `ReasoningUpdate` | Upsert thinking entity (delta) |
| `ChatReasoningFinished` | `ReasoningUpdate` | Upsert thinking entity (complete) |
| `ChatAgentModePreviewUpdated` | `AgentModePreviewUpdate` | Upsert mode preview |
| `ChatAgentModeCommitted` | `AgentModeCommittedUpdate` | Upsert committed mode + clear preview |
| `ChatAgentModePreviewCleared` | `AgentModePreviewCleared` | Delete mode preview |
| `ChatToolCallStarted` | `ToolCallUpdate` | Upsert tool call entity |
| `ChatToolCallUpdated` | `ToolCallUpdate` | Upsert tool call entity |
| `ChatToolCallFinished` | `ToolCallUpdate` | Upsert tool call entity |
| `ChatToolResultReady` | `ToolResultUpdate` | Upsert tool result entity |

### Timeline Entity Kinds

| Kind | Protobuf Type | ID Source |
|------|--------------|-----------|
| `ChatMessage` | `ChatMessageEntity` | `payload.messageId` |
| `AgentMode` | `AgentModeEntity` | Message ID of the committed event |
| `tool_call` | `ToolCallEntity` | Tool call ID from the model |
| `tool_result` | `ToolResultEntity` | Tool result ID |

### Redux Actions (timelineSlice)

| Action | Effect |
|--------|--------|
| `upsertEntity` | Insert new entity or merge props into existing entity. Preserves `createdAt`. |
| `deleteEntity` | Remove entity by ID. |
| `rekeyEntity` | Change entity ID (used for optimistic → final ID mapping). |
| `clear` | Remove all entities. Called before snapshot application. |

### Debug Flags (localStorage)

| Flag | Purpose |
|------|---------|
| `pinocchio.debugStream` | Enable frontend debug recording. |
| `pinocchio.debugScroll` | Enable sticky scroll debug logging (prefix: `[pinocchio-scroll]`). |

## Update: Integrating Sessionstream Observers

After the initial Pinocchio-specific debug design, we identified two generic observability seams that belong in `sessionstream` rather than in `cmd/web-chat`: the Hub pipeline and the WebSocket transport. These are now tracked in the Sessionstream ticket `SS-OBSERVERS`.

The Pinocchio debug implementation should take advantage of those hooks instead of scraping internal state where possible. The boundary should be:

- `sessionstream` emits neutral structured observations about pipeline and transport behavior.
- `pinocchio/cmd/web-chat` records those observations in a debug recorder when debug mode is enabled.
- `pinocchio/cmd/web-chat/web` renders browser-side debug records and reconciliation output.
- `pinocchio/pkg/chatapp` remains focused on chat semantics and should not own debug storage or HTTP endpoints.

### New backend evidence chain

With `SS-OBSERVERS`, the backend-side debug story becomes much clearer:

```text
Hub PipelineObserver
  Event ordinal 101 entered projectAndApply
  UI projection produced ChatMessageAppended
  Timeline projection produced ChatMessage entity chat-msg-2
  Hydration store applied that entity
  Fanout handed UI event 101 to the websocket server

WebSocket TransportObserver
  fanout_started ordinal=101 targets=[conn-A, conn-B]
  server_frame_queued conn-A frame=uiEvent ordinal=101
  server_frame_written conn-A bytes=...
  server_frame_queued conn-B frame=uiEvent ordinal=101
  server_frame_written conn-B bytes=...
```

If a frontend log later shows that `conn-B` never rendered ordinal 101, we can separate the possibilities:

- The Hub never produced the UI event.
- The Hub produced it, but fanout failed.
- The websocket server queued it, but write failed.
- The browser received it, but frontend parsing or Redux mutation dropped it.

This distinction is the purpose of the observer split.

### Pinocchio debug recorder wiring

The Pinocchio server should wire both observers in `cmd/web-chat/app/server.go` when debug recording is enabled.

```go
recorder := debug.NewStreamRecorder(debug.Options{MaxRecords: 10000})

wsServer, err := wstransport.NewServer(snapshotProvider,
    wstransport.WithTransportObserver(wstransport.TransportObserverFunc(func(ctx context.Context, rec wstransport.TransportRecord) {
        recorder.RecordTransport(ctx, rec)
    })),
)

hub, err := sessionstream.NewHub(
    sessionstream.WithSchemaRegistry(reg),
    sessionstream.WithHydrationStore(store),
    sessionstream.WithUIFanout(wsServer),
    sessionstream.WithPipelineObserver(sessionstream.PipelineObserverFunc(func(ctx context.Context, rec sessionstream.PipelineRecord) {
        recorder.RecordPipeline(ctx, rec)
    })),
)
```

The recorder output should be available from Pinocchio debug endpoints, not from Sessionstream:

```text
GET /api/debug/sessions/{sessionId}/pipeline
GET /api/debug/sessions/{sessionId}/transport
GET /api/debug/sessions/{sessionId}/records
GET /api/debug/sessions/{sessionId}/reconcile
```

### Relationship to the websocket race ticket

The reload-during-streaming race is now tracked separately in `SS-WS-RACE`. The relevant failure is:

```text
snapshot loaded at ordinal 100
live UI event ordinal 101 emitted before subscription registration
new browser connection does not receive ordinal 101
future ordinal 102+ events arrive normally
```

For Pinocchio, the debug implementation should first consume observer records that prove this interleaving. Once Sessionstream implements the subscribe-first hydration buffer from `SS-WS-RACE`, Pinocchio can use the same recorder to verify the fixed sequence:

```text
subscription_registered state=hydrating
snapshot_loaded ordinal=100
fanout_started ordinal=101 targets=[conn]
ui_event_buffered ordinal=101
snapshot_queued ordinal=100
buffer_flushed ordinal=101
subscription_live
```

### Revised implementation order for PINO-STREAM-DEBUG

The recommended Pinocchio implementation order is now:

1. Wait for or vendor/update `sessionstream` with `SS-OBSERVERS` if we want full backend evidence.
2. Implement a Pinocchio debug recorder that accepts `PipelineRecord` and `TransportRecord` values.
3. Add Pinocchio debug HTTP endpoints that expose recorded observer data by session ID.
4. Implement the frontend debug mode that records raw WebSocket frames, parsed frames, hydration application, UI-event mutations, Redux actions, and rendered entity lists.
5. Implement reconciliation between Sessionstream observer records and frontend debug records.
6. Use `SS-WS-RACE` traces as the primary reload-during-streaming robustness scenario.

### If the observer work is not ready yet

Pinocchio can still implement a reduced version by wrapping `UIFanout` and recording frontend frames. That is useful, but it cannot see projection outputs or exact transport queue/write stages. The observer-based design is preferred because it gives a single causal chain from backend event ordinal to browser frame write.

## Update: Sessionstream Observers and Subscribe Race Fix Landed

The Sessionstream-side foundation described above has now been implemented:

- `SS-OBSERVERS` landed Hub `PipelineObserver` support for live and rebuild event processing.
- `SS-OBSERVERS` landed WebSocket `TransportObserver` support for connection lifecycle, client-frame decoding, subscribe/snapshot stages, fanout target selection, frame queueing, and frame writes.
- `SS-WS-RACE` landed subscribe-first hydration buffering so a reconnecting WebSocket is visible to fanout as `hydrating` before snapshot load starts.

The corrected reconnect sequence is now:

```text
subscribe_received
subscription_registered(state=hydrating)
snapshot_load_started
fanout_started(ordinal=N+1, targets=[conn])
ui_event_buffered(ordinal=N+1)
snapshot_loaded(snapshotOrdinal=N)
server_frame_queued(snapshot N)
hydration_buffer_flushed(ordinals>N)
server_frame_queued(uiEvent N+1)
subscription_live
server_frame_queued(subscribed)
```

For Pinocchio, this means the backend debug recorder no longer needs to infer the race by scraping SQLite tables or wrapping only `UIFanout`. It can subscribe directly to `PipelineRecord` and `TransportRecord`, then correlate those records with browser-side frontend debug logs.
