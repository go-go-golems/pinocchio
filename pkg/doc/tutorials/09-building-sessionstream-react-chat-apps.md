---
Title: Building a React + Go LLM Chat App on Sessionstream and Pinocchio
Slug: building-sessionstream-react-chat-apps
Short: Step-by-step playbook for building a sessionstream-backed React chat app with Go handlers, Geppetto inference, hydration, websockets, and custom widgets.
Topics:
- pinocchio
- sessionstream
- geppetto
- webchat
- react
- frontend
- backend
- tutorial
Commands:
- web-chat
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

This tutorial explains how to build an application shaped like `cmd/web-chat` without copying the old historical `pkg/evtstream` layout. The goal is not merely to get a chat box on the screen. The goal is to build a **maintainable session-based chat application** with a clear ownership model: `sessionstream` owns the event-streaming substrate, `pinocchio` and `geppetto` own runtime composition and inference machinery, and your application owns product-specific HTTP contracts, feature extensions, widgets, and frontend state.

By the end, you should understand where each piece belongs, how a prompt moves from React to Go to Geppetto and back again, how hydrated snapshots and live UI events fit together, and how to add custom widgets without polluting the shared substrate.

## What you are building

You are building a system with this shape:

```text
React UI
  -> HTTP create/submit APIs
  -> WebSocket subscribe stream
      -> app-owned Go server
          -> sessionstream Hub + projections + hydration
              -> pinocchio chat package + app-owned features
                  -> geppetto engine + runtime middlewares + model providers
```

The important design decision is that the **canonical internal truth is the backend event stream**, not the React component tree and not a frontend-only message array. React renders what the backend has already turned into snapshot entities and live UI events.

## When to use this pattern

Use this pattern when you want all of the following:

- a Go backend with real LLM/runtime control,
- a React frontend with live streaming,
- reconnect-safe snapshots,
- durable or optional persisted session state,
- room for custom widgets such as mode cards, tool status panels, or structured event-driven sidebars,
- and an architecture where custom features stay app-owned rather than leaking into the shared framework.

If you only need a quick demo with no reconnect semantics and no app-owned extensions, this architecture may be heavier than necessary. It becomes worth it when the application must survive reloads, support custom events, and remain understandable after multiple feature additions.

## Ownership model

Start here, because most mistakes in this space are ownership mistakes.

### `sessionstream` owns

- `Hub`, command routing, event publishing, projection plumbing,
- hydration store interfaces and implementations,
- websocket transport,
- framework-oriented examples and framework-oriented Systemlab,
- generic session-based streaming semantics.

### `pinocchio` / `geppetto` own

- inference runtime composition,
- model/provider wiring,
- middleware definition/build logic,
- profile and runtime selection,
- Geppetto event types and tool/inference loops.

### your application owns

- HTTP contract,
- frontend message model and React widgets,
- feature-specific event translation,
- app-specific timeline entities,
- app-specific UI events,
- app-specific feature registration and wiring.

That last bullet is the one to keep repeating to yourself. If your application wants a custom card, custom event, or custom mode switch widget, **the application should own it**.

## The reference files to study first

Read these files before starting implementation:

### Framework substrate

- `sessionstream/doc.go`
- `sessionstream/hub.go`
- `sessionstream/projection.go`
- `sessionstream/hydration.go`
- `sessionstream/transport/ws/server.go`

### Downstream chat app reference in pinocchio

- `pinocchio/pkg/chatapp/chat.go`
- `pinocchio/pkg/chatapp/service.go`
- `pinocchio/pkg/chatapp/features.go`
- `pinocchio/cmd/web-chat/app/server.go`
- `pinocchio/cmd/web-chat/main.go`

### App-owned custom feature reference

- `pinocchio/cmd/web-chat/agentmode_chat_feature.go`
- `pinocchio/cmd/web-chat/agentmode_chat_feature_test.go`
- `pinocchio/pkg/middlewares/agentmode/middleware.go`
- `pinocchio/pkg/middlewares/agentmode/preview_event.go`

### Frontend reference

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.test.ts`
- `pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`
- `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx`

## Architecture at a glance

Here is the end-to-end flow you should be aiming for.

```text
1. React submits a message
   -> POST /api/chat/sessions/:id/messages

2. App server resolves runtime/profile
   -> pinocchio runtime composer
   -> geppetto engine + middleware chain

3. App service submits a command to sessionstream
   -> ChatStartInference

4. Chat handler publishes canonical backend events
   -> ChatUserMessageAccepted
   -> ChatInferenceStarted
   -> ChatTokensDelta
   -> ChatInferenceFinished
   -> app-owned feature events when applicable

5. sessionstream projections derive outputs
   -> TimelineEntity records for hydration
   -> UIEvent frames for live delivery

6. WebSocket transport fans UI events to subscribers
   -> snapshot first
   -> live UI events after subscribe

7. React updates state from snapshot + UI events
   -> timeline entities map to cards/messages/widgets
```

The virtue of this model is that the frontend does not invent truth. It renders truth derived by the backend.

## Suggested application layout

A practical layout for a new app looks like this:

```text
cmd/my-chat-app/
  main.go
  app/
    server.go
    contracts.go
    runtime.go
  features/
    myfeature.go
    myfeature_test.go
  web/
    src/
      ws/
      features/
      webchat/
      store/
      App.tsx
pkg/mychatapp/
  chat.go
  service.go
  features.go
```

A useful rule is:

- `pkg/mychatapp` owns reusable app-grade chat behavior for this one product family,
- `cmd/my-chat-app` owns delivery, wiring, and product-specific features,
- `sessionstream` stays framework-grade and unaware of your product.

## Step 1 — define the HTTP and websocket contract first

Before writing handlers, define what the browser talks to.

A minimal contract looks like this:

```text
POST   /api/chat/sessions
POST   /api/chat/sessions/:sessionId/messages
GET    /api/chat/sessions/:sessionId
WS     /api/chat/ws
```

Why define this first? Because otherwise the backend and frontend drift into ad hoc assumptions. The app server should be the place where browser-facing payloads are stabilized.

A minimal `contracts.go` usually contains shapes like:

```go
type CreateSessionRequest struct {
    Profile  string `json:"profile,omitempty"`
    Registry string `json:"registry,omitempty"`
}

type SubmitMessageRequest struct {
    Prompt         string `json:"prompt"`
    Profile        string `json:"profile,omitempty"`
    Registry       string `json:"registry,omitempty"`
    IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type SessionSnapshotResponse struct {
    SessionID string           `json:"sessionId"`
    Ordinal   string           `json:"ordinal"`
    Status    string           `json:"status,omitempty"`
    Entities  []SnapshotEntity `json:"entities"`
}
```

The browser should not know about Geppetto turn internals, middlewarecfg details, or storage internals. It should know about sessions, messages, snapshots, and live events.

## Step 2 — create the downstream chat package

Your downstream chat package is where you turn generic `sessionstream` primitives into a chat-shaped application surface.

The reference pattern is `pinocchio/pkg/chatapp`.

### What belongs in this package

- chat command names,
- core chat backend events,
- base message timeline projection,
- base UI projection,
- prompt submission service methods,
- runtime-event handling for generic completion/error/interrupt events,
- and a feature extension seam.

### What does not belong here

- product-specific HTTP handlers,
- browser transport decisions,
- one-off app widgets,
- product-only middleware semantics such as `agentmode` cards.

### Minimal shape

```go
type Service struct {
    hub    *sessionstream.Hub
    engine *Engine
}

type PromptRequest struct {
    Prompt         string
    IdempotencyKey string
    Runtime        *infruntime.ComposedRuntime
}
```

This package should give callers domain methods like `SubmitPromptRequest`, `Stop`, `WaitIdle`, and `Snapshot` instead of making every caller work directly with raw command names.

## Step 3 — build the feature seam before you need custom widgets

Do this early. If you wait until the third or fourth app-owned widget, you will end up shoving app-specific logic into your base chat package.

The reference seam is `pinocchio/pkg/chatapp/features.go`.

```go
type FeatureSet interface {
    RegisterSchemas(reg *sessionstream.SchemaRegistry) error
    HandleRuntimeEvent(ctx context.Context, runtime RuntimeEventContext, event gepevents.Event) (bool, error)
    ProjectUI(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.UIEvent, bool, error)
    ProjectTimeline(ctx context.Context, ev sessionstream.Event, session *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, bool, error)
}
```

This seam gives you three critical powers:

- register app-specific schemas without changing `sessionstream`,
- translate runtime/middleware events into app-owned backend events,
- project those events into timeline entities and live UI events.

That is the mechanism that makes custom widgets clean instead of invasive.

## Step 4 — wire the app server to sessionstream

Your app server should compose the substrate, the downstream chat package, and any app-owned feature sets.

The reference is `pinocchio/cmd/web-chat/app/server.go`.

### Core responsibilities of the app server

- create a `SchemaRegistry`,
- register base chat schemas plus feature schemas,
- build hydration store,
- build websocket fanout transport,
- create the `sessionstream.Hub`,
- install your chat package handlers/projections,
- expose HTTP create/submit/snapshot routes,
- expose websocket subscribe route.

### Pseudocode

```go
reg := sessionstream.NewSchemaRegistry()
_ = mychatapp.RegisterSchemas(reg, myFeatures...)

store := storesqlite.New(...)
ws := wstransport.NewServer(snapshotProvider)

hub, _ := sessionstream.NewHub(
    sessionstream.WithSchemaRegistry(reg),
    sessionstream.WithHydrationStore(store),
    sessionstream.WithUIFanout(ws),
)

engine := mychatapp.NewEngine(
    mychatapp.WithFeatureSets(myFeatures...),
)

_ = mychatapp.Install(hub, engine)
svc, _ := mychatapp.NewService(hub, engine)
```

The app server is where browser delivery and runtime resolution meet. It is not where shared framework abstractions should be invented.

## Step 5 — resolve runtime and profile selection in app-owned code

A real chat app needs more than demo inference. It needs profile selection, middleware composition, provider settings, and runtime fingerprinting.

In Pinocchio, the reference pieces are:

- `pinocchio/cmd/web-chat/canonical_runtime_resolver.go`
- `pinocchio/cmd/web-chat/runtime_composer.go`
- `pinocchio/cmd/web-chat/profiles/*`

The key point is that your app server accepts a `RuntimeResolver` interface, and the app owns how a browser request becomes a composed Geppetto runtime.

That separation matters because `sessionstream` should not know what a profile registry is, what `agentmode` means, or how your application chooses among runtime stacks.

## Step 6 — make hydration and reconnect part of the first implementation

If you postpone hydration, your frontend will silently become the truth source. That creates pain later.

A robust application should support:

- snapshot on subscribe,
- live UI events after snapshot,
- reload and reconnect without losing conversation state,
- optional durable SQLite persistence.

Use the existing `sessionstream` stores:

- `sessionstream/hydration/memory`
- `sessionstream/hydration/sqlite`

Start with memory if you must, but keep the store seam active from day one.

## Step 7 — keep the frontend message model role-aware and snapshot-driven

The frontend should merge:

- initial snapshot entities,
- subsequent UI events,
- and custom widget entities.

The reference websocket client is `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`.

The key frontend rules are:

- subscribe by `sessionId`,
- accept snapshot before live events,
- preserve `role`, `content`, `status`, and `streaming`,
- map custom entities by kind instead of hard-coding one UI card type.

A good mental model is:

```text
snapshot entity / ui event
  -> normalized timeline mutation
      -> store update
          -> renderer registry lookup
              -> React component
```

Do not let raw websocket frames leak all the way into React components.

## Step 8 — add custom widgets by translating app-owned events, not by special-casing React first

This is the part most people get wrong.

If you want a custom widget, start on the backend.

### Correct pattern

```text
runtime middleware event
  -> app-owned feature HandleRuntimeEvent(...)
      -> app backend event
          -> timeline/UI projection
              -> hydrated entity + UI event
                  -> frontend renderer
```

### Incorrect pattern

```text
middleware does something
  -> frontend invents a local card shape from an unrelated text stream
```

The correct pattern is more work upfront, but it gives you durable state, reconnect correctness, and testable semantics.

### Reference: `agentmode`

Study:

- `pinocchio/cmd/web-chat/agentmode_chat_feature.go`
- `pinocchio/pkg/middlewares/agentmode/*`
- `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx`

What this feature shows:

- runtime-specific middleware events stay outside `sessionstream`,
- app-owned feature code registers extra schemas,
- preview vs committed state are treated differently,
- frontend gets both hydrated entity state and live preview-clearing events.

## Step 9 — give each widget a backend entity kind and a frontend renderer

A custom widget is not just a React component. It is a contract between backend and frontend.

### Backend side

Your feature should emit a timeline entity kind such as:

```text
AgentMode
ToolCall
ResearchPlan
ApprovalGate
```

### Frontend side

Register a renderer keyed by entity kind.

```ts
rendererRegistry.register("AgentMode", AgentModeCard)
```

The renderer should consume normalized entity payloads, not raw websocket envelopes.

This gives you a powerful property: the same entity can appear after reload from snapshot or live during the current session, and the renderer does not care which path produced it.

## Step 10 — write the tests in the same slices you add behavior

Do not leave this architecture untested. The whole benefit of this shape is that each layer has a clean seam.

### Recommended test layers

#### Downstream chat package tests

Test:

- base chat event projection,
- user + assistant message behavior,
- stop path,
- feature hook dispatch behavior.

Reference:

- `pinocchio/pkg/chatapp/chat_test.go`
- `pinocchio/pkg/chatapp/service_test.go`

#### App feature tests

Test:

- runtime-event translation,
- UI projection,
- timeline projection,
- payload shape correctness.

Reference:

- `pinocchio/cmd/web-chat/agentmode_chat_feature_test.go`

#### App server tests

Test:

- create session,
- submit message,
- snapshot shape,
- websocket hello / subscribe / ui-event flow,
- runtime-backed inference path,
- sqlite persistence across restart.

Reference:

- `pinocchio/cmd/web-chat/app/server_test.go`

#### Frontend tests

Test:

- snapshot-to-entity mapping,
- ui-event-to-mutation mapping,
- custom entity rendering behavior,
- reconnect merge behavior.

Reference:

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.test.ts`

## Step 11 — a minimal build order that keeps risk low

If you are building a new app from scratch, do it in this order:

1. define HTTP and websocket contract,
2. stand up app server + base chat package on `sessionstream`,
3. get snapshot and streaming assistant response working,
4. add runtime/profile-backed inference,
5. add reconnect/persistence,
6. add frontend renderer registry,
7. add first app-owned feature/widget,
8. add feature tests and browser checks,
9. only then broaden to more custom widgets.

That order prevents you from building a beautiful frontend on top of a backend that still lacks durable truth.

## Step 12 — common failure modes

The point of this architecture is not that it prevents all mistakes. It prevents some classes of mistakes if you notice them early.

### Failure mode: app-specific logic leaks into `sessionstream`

This usually starts as “just one convenience helper.” It ends with the framework knowing about your product.

Fix: move the logic into the downstream chat package or app-owned feature file.

### Failure mode: custom widget exists only in React

This feels fast until reload/reconnect happens.

Fix: give the widget a backend event and timeline entity first.

### Failure mode: chat package owns product-specific middleware semantics

This makes your supposedly reusable app-grade package harder to reason about and harder to reuse across sibling applications.

Fix: keep the feature seam generic and implement concrete features in app-owned code.

### Failure mode: snapshot and live event payloads disagree

Then the frontend behaves differently on reload than during live streaming.

Fix: normalize both paths through one mapping layer and write explicit tests.

## Complete assembly checklist

Before calling your app “done,” make sure you can say yes to all of these:

- Does the Go server expose app-owned create/submit/snapshot/ws routes?
- Does the app use `sessionstream.Hub` instead of inventing its own event bus?
- Does the downstream chat package own only base chat behavior?
- Are custom widgets implemented through app-owned feature sets?
- Can the frontend rebuild from snapshot alone?
- Can a live session continue over websocket after initial snapshot subscribe?
- Do feature-specific entities survive reconnect?
- Are base chat tests, feature tests, app server tests, and frontend mapping tests present?

If any of those answers is no, the application may still work, but it probably does not yet have the durability or extensibility this playbook is aiming for.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| User messages appear live but disappear on reload | Frontend inserted optimistic state but backend never projected a durable user entity | Publish a backend `...UserMessageAccepted` event and persist it through timeline projection |
| Custom widget appears only during streaming | Widget state is frontend-only or emitted as UI-only event | Add an app-owned backend event and timeline entity kind |
| Middleware event never shows in UI | Runtime event is emitted but no app-owned feature translates it | Implement `HandleRuntimeEvent(...)` in a feature set and register it with the app server |
| Reload restores messages but not widget state | Snapshot encoding does not include the custom entity kind | Register schemas and timeline projection for the feature entity |
| Websocket subscribes but no initial state appears | Snapshot provider or hydration store is missing/incorrect | Verify `transport/ws` is built with a working snapshot provider backed by the same store used by the Hub |
| Frontend works in dev but breaks after restart | State depends on transient UI events instead of hydrated entities | Make snapshot entities sufficient to rebuild the page |
| Feature logic keeps creeping into the chat core | No generic feature seam exists | Add a `FeatureSet` interface like `pinocchio/pkg/chatapp/features.go` and move product features out |

## See Also

- `webchat-getting-started` — quick local run workflow for the existing reference app
- `webchat-backend-reference` — current backend contract details
- `webchat-frontend-architecture` — current React-side structure
- `webchat-sem-and-ui` — frontend/backend event and rendering model
- `intern-app-owned-middleware-events-timeline-widgets` — deeper feature and widget reference
- `sessionstream/cmd/sessionstream-systemlab` — framework-oriented lab examples and chapters in the extracted framework repo
