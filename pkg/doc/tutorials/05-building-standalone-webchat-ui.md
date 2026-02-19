---
Title: Building a Standalone Web UI for Webchat (Streaming + Timeline)
Slug: building-standalone-webchat-ui
Short: Exhaustive guide to building a standalone React webchat UI that connects to /chat and /ws, hydrates /api/timeline, projects SEM events, and renders a timeline-driven chat window.
Topics:
- webchat
- websocket
- timeline
- sem
- react
- redux
- streaming
- hydration
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

## 1. What This Tutorial Covers

This tutorial teaches how to build a standalone web UI for Pinocchio webchat, with the same architecture used by `cmd/web-chat/web`. The focus is not custom middleware widgets end-to-end. The focus is the chat application shell itself:

- opening and maintaining the websocket stream,
- sending prompts through `POST /chat`,
- hydrating conversation history through `GET /api/timeline`,
- projecting incoming SEM events into timeline entities,
- rendering those entities in a timeline/chat window.

By the end, you should be able to build a new web UI that interoperates with the existing backend without touching shared core internals.

## 2. Reference Implementation to Study First

Before implementing anything, read these files in this order:

- Backend request/stream/timeline HTTP surface:
  - `pkg/webchat/http/api.go`
- App wiring for canonical routes:
  - `cmd/web-chat/main.go`
- Frontend websocket lifecycle manager:
  - `cmd/web-chat/web/src/ws/wsManager.ts`
- Frontend SEM handler registry:
  - `cmd/web-chat/web/src/sem/registry.ts`
- Proto -> frontend entity mapper:
  - `cmd/web-chat/web/src/sem/timelineMapper.ts`
- Timeline state storage and merge semantics:
  - `cmd/web-chat/web/src/store/timelineSlice.ts`
- Chat container and prompt submission flow:
  - `cmd/web-chat/web/src/webchat/ChatWidget.tsx`
- Timeline rendering loop:
  - `cmd/web-chat/web/src/webchat/components/Timeline.tsx`
- Renderer registration/dispatch:
  - `cmd/web-chat/web/src/webchat/rendererRegistry.ts`

Those files define the working contract between backend and UI.

## 3. Contract-First Mental Model

Treat the system as four layers. If you keep them separate, your UI remains maintainable.

```text
Layer 1: Transport
  HTTP POST /chat
  WebSocket /ws
  HTTP GET /api/timeline

Layer 2: Event Protocol (SEM)
  { sem: true, event: { type, id, seq, data, ... } }

Layer 3: Projection State
  timelineSlice: byId + order, upsert semantics

Layer 4: Presentation
  ChatTimeline + renderer registry (kind -> React component)
```

Critical design rule: never let JSX depend on raw websocket payloads directly. Always normalize through SEM handlers and timeline state first.

## 4. Backend Surface You Need

A standalone UI only works reliably if the backend exposes the canonical endpoints and request policy.

### 4.1 Canonical handlers

`pkg/webchat/http/api.go` provides three handler builders:

- `NewChatHandler(...)` for `POST /chat` (and optional `/chat/{runtime}` path conventions in app resolvers),
- `NewWSHandler(...)` for websocket attach at `/ws`,
- `NewTimelineHandler(...)` for timeline hydration at `/api/timeline`.

In `cmd/web-chat/main.go`, those are mounted like this:

- `/chat`, `/chat/`
- `/ws`
- `/api/timeline`, `/api/timeline/`

### 4.2 Request resolution

Do not parse request policy ad hoc in each handler. Use a `ConversationRequestResolver` (see `pkg/webchat/http/api.go`) that resolves:

- `ConvID`,
- `RuntimeKey`,
- per-run `Overrides`,
- optional `IdempotencyKey`,
- prompt text.

This keeps HTTP and WS routes aligned and avoids split-brain behavior where websocket joins one conversation and chat posts to another.

### 4.3 Timeline hydration payload

`NewTimelineHandler` returns `TimelineSnapshotV2` JSON (protobuf JSON via `protojson`), with:

- `convId`
- `version`
- `entities[]` (each `TimelineEntityV2`)

On the frontend you decode this through `TimelineSnapshotV2Schema` (`@bufbuild/protobuf`) before mapping entities.

## 5. Frontend App Bootstrapping

A minimal standalone app still needs explicit boot order.

Current reference flow:

- `cmd/web-chat/web/src/main.tsx` creates React root and wraps with `ErrorBoundary`.
- `cmd/web-chat/web/src/App.tsx` wraps `ChatWidget` in Redux `Provider`.
- `cmd/web-chat/web/src/store/store.ts` combines slices including `timeline` and `app`.

If you build your own shell, keep this sequence:

1. Initialize store.
2. Register default SEM handlers.
3. Register feature SEM handlers/renderers.
4. Mount chat UI.

If you mount before registration, early frames can arrive and be ignored.

## 6. Conversation Identity and URL Ownership

`ChatWidget` (`cmd/web-chat/web/src/webchat/ChatWidget.tsx`) follows this policy:

- read `conv_id` from URL on load,
- if no conversation exists and user sends a prompt, generate `crypto.randomUUID()`,
- write `conv_id` back to URL (`history.replaceState`).

This is important for refresh/reopen behavior and for sharing links to active conversations.

Pseudo-flow:

```text
on mount:
  conv_id := query(conv_id|convId)
  if present -> appSlice.setConvId(conv_id)

on send(prompt):
  if no conv_id:
    conv_id = uuid
    push to URL
  POST /chat { conv_id, prompt, overrides? }
```

## 7. WebSocket Manager Design (Connection + Hydration + Replay)

The most important file for standalone UI robustness is `cmd/web-chat/web/src/ws/wsManager.ts`.

### 7.1 Responsibilities of `WsManager`

`WsManager.connect(...)` does more than opening a socket. It coordinates:

- connection lifecycle and status transitions,
- SEM handler bootstrap (`registerDefaultSemHandlers()` + module registrations),
- hydration from `/api/timeline` before replaying buffered stream events,
- sequence-aware ordering for buffered frames,
- dispatching app/timeline/error actions.

### 7.2 Why hydration-before-replay matters

Without hydration-first logic, you can render duplicate or stale entities when reconnecting.

Current strategy:

1. Open websocket.
2. Buffer incoming frames while `hydrated == false`.
3. Fetch timeline snapshot from `/api/timeline?conv_id=...`.
4. Map and upsert snapshot entities.
5. Mark hydrated.
6. Replay buffered frames sorted by `event.seq`.

This ensures the in-memory timeline starts from persisted truth, then catches up with live tail frames.

### 7.3 State fields worth copying

`WsManager` keeps internal fields that solve real race conditions:

- `connectNonce` to invalidate stale async callbacks,
- `hydrated` guard,
- `buffered` frame queue,
- cached `lastDispatch`/`lastOnStatus` for disconnect cleanup.

Do not remove `connectNonce`-style invalidation in your own implementation. It prevents old websocket callbacks from mutating state after conversation switches.

## 8. SEM Event Handling and Projection in the Browser

In `cmd/web-chat/web/src/sem/registry.ts`, frontend projection is explicit and type-based.

### 8.1 Core registry API

- `registerSem(type, handler)` adds a handler.
- `handleSem(envelope, dispatch)` validates and dispatches by `event.type`.
- `registerDefaultSemHandlers()` resets registry and installs built-ins.

### 8.2 Built-in mapping examples

- `timeline.upsert`:
  - decode `TimelineUpsertV2`, map entity with `timelineEntityFromProto`, upsert into `timelineSlice`.
- `llm.start/delta/final`:
  - project assistant/thinking message entities.
- `tool.start/delta/done/result`:
  - project `tool_call` and `tool_result` entities.
- `log`, `agent.mode`, `debugger.pause`:
  - project into dedicated kinds.

The important pattern is idempotent upserts keyed by stable entity IDs.

### 8.3 Decoder strategy

All handlers decode typed protobuf payloads using `fromJson(schema, raw, { ignoreUnknownFields: true })`. This makes the UI resilient to additive fields.

## 9. Mapping Proto Entities to UI State

`cmd/web-chat/web/src/sem/timelineMapper.ts` translates transport entities into Redux entities.

### 9.1 `TimelineEntityV2` mapping

`timelineEntityFromProto(e, version)` maps:

- `id`,
- `kind`,
- `createdAtMs` -> `createdAt`,
- `updatedAtMs` -> `updatedAt`,
- `version`,
- `props` after normalizers.

### 9.2 Props normalizer registry

`cmd/web-chat/web/src/sem/timelinePropsRegistry.ts` gives a per-kind normalization hook:

- builtin normalizer exists for `tool_result` (`resultRaw` -> `result` fallback),
- extensions can register custom normalizers via `registerTimelinePropsNormalizer(kind, fn)`.

This is where transport quirks are normalized once so renderers stay simple.

## 10. Timeline State Semantics (Redux)

`cmd/web-chat/web/src/store/timelineSlice.ts` is the truth store for rendered history.

Important reducers:

- `addEntity`: append only when ID is new,
- `upsertEntity`: merge updates by ID,
- `rekeyEntity`: transfer state when temporary IDs become canonical,
- `clear`: reset conversation timeline.

### 10.1 Version-aware merge behavior

`upsertEntity` has version logic:

- if incoming version is lower than existing version, drop update,
- if existing has version and incoming has none, preserve stable fields and only merge props,
- otherwise merge normally.

This avoids out-of-order overwrite during reconnects.

### 10.2 Ordering model

Timeline ordering uses `order[]` plus `byId`. Rendering reads `order.map(id => byId[id])`.

That means you can update existing rows without reordering the whole list.

## 11. Rendering Pipeline (Chat Window + Timeline)

### 11.1 Chat container responsibilities

`ChatWidget.tsx` orchestrates:

- URL conv_id sync,
- websocket connect/disconnect,
- prompt submission via `fetch(basePrefix + '/chat')`,
- profile switching,
- error panel toggling,
- new conversation reset.

### 11.2 Timeline component responsibilities

`ChatTimeline` (`cmd/web-chat/web/src/webchat/components/Timeline.tsx`) does not project data. It only:

- loops through already-normalized entities,
- chooses renderer by kind,
- applies structural shells (`turn`, `bubble`, `content`),
- renders error panel if needed.

This separation keeps render code deterministic and testable.

### 11.3 Kind-to-renderer dispatch

`cmd/web-chat/web/src/webchat/rendererRegistry.ts` resolves renderers in this order:

1. builtin renderers (`message`, `tool_call`, `tool_result`, `log`),
2. extension registry (`registerTimelineRenderer`),
3. call-site overrides,
4. default fallback (`GenericCard`).

This gives you a stable default UI while still allowing app-level specialization.

## 12. Base Prefix and Mount Path Handling

For standalone deployments under subpaths, use `basePrefixFromLocation()` (`cmd/web-chat/web/src/utils/basePrefix.ts`).

Resolution logic:

- if runtime config defines `window.__PINOCCHIO_WEBCHAT_CONFIG__.basePrefix`, use it,
- otherwise infer from first URL path segment.

All network calls must use this same prefix:

- `${basePrefix}/chat`
- `${basePrefix}/ws?...`
- `${basePrefix}/api/timeline?...`

This avoids mismatched requests when app is mounted under `/chat`, `/ai`, or another root.

## 13. End-to-End Sequence Diagram

```text
User types prompt
  -> ChatWidget.send()
     -> POST /chat { conv_id, prompt, overrides }

ChatWidget useEffect(conv_id)
  -> wsManager.connect()
     -> open WebSocket /ws?conv_id=...
     -> buffer incoming SEM frames while hydrate=false
     -> GET /api/timeline?conv_id=...
        -> decode TimelineSnapshotV2
        -> timelineEntityFromProto -> timelineSlice.upsertEntity
     -> replay buffered frames by seq
        -> handleSem(envelope)
           -> sem handler map by event.type
           -> timelineSlice add/upsert

Redux state update
  -> selectTimelineEntities
  -> ChatTimeline maps entities
  -> rendererRegistry resolves renderer by kind
  -> card component renders row in chat timeline
```

## 14. Minimal Pseudocode Blueprint

Use this as your implementation checklist when building a fresh standalone UI.

```ts
// bootstrap.tsx
createStore()
registerDefaultSemHandlers()
registerMyFeatureModule()
render(<Provider store={store}><ChatWindow /></Provider>)

// ChatWindow.tsx
if (convId) wsManager.connect({ convId, basePrefix, dispatch, hydrate: true })

function send(prompt) {
  ensureConvId()
  POST(`${basePrefix}/chat`, { conv_id: convId, prompt })
}

// wsManager.ts
connect() {
  open ws
  onmessage: if !hydrated buffer else handleSem()
  hydrate via /api/timeline
  apply snapshot
  hydrated = true
  replay buffered
}

// semRegistry.ts
registerSem('timeline.upsert', decodeAndUpsert)
registerSem('llm.delta', upsertMessage)
registerSem('tool.result', upsertToolResult)

// Timeline.tsx
for e in entities:
  Renderer = renderers[e.kind] ?? renderers.default
  <Renderer e={e} />
```

## 15. Testing Strategy for a Standalone UI

### 15.1 Integration tests (backend)

Use patterns from `cmd/web-chat/app_owned_chat_integration_test.go`:

- assert `/chat` returns `started` with IDs,
- connect `/ws` and assert hello/pong behavior,
- call `/api/timeline` and assert projected entities.

### 15.2 Frontend unit/integration tests

Recommended targets:

- `wsManager` buffering + hydration replay order,
- SEM handler registration behavior,
- `timelineEntityFromProto` mapping correctness,
- renderer registry resolution precedence,
- conversation reset behavior (`onNewConversation`).

Existing examples:

- `cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.test.tsx`
- `cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.test.ts`

### 15.3 Manual validation checklist

Run this exact checklist before shipping:

1. Start backend and open UI with no `conv_id`.
2. Send first prompt and confirm URL gains `conv_id`.
3. Refresh page and confirm timeline hydrates without duplication.
4. Open second tab on same `conv_id` and confirm live updates continue.
5. Kill/restart backend and confirm reconnect + hydrate behavior recovers.
6. Test app mounted under non-root prefix (for example `/chat`).

## 16. Common Failure Modes and Fixes

| Problem | Likely Cause | Fix |
| --- | --- | --- |
| Messages appear twice after reconnect | Live frames applied before hydration + replay | Buffer frames until hydration completes, then replay sorted by `seq` |
| No timeline after refresh | `GET /api/timeline` not mounted or wrong basePrefix | Verify `NewTimelineHandler` route and `basePrefixFromLocation()` |
| WS connected but UI empty | SEM handlers not registered before frames arrive | Call `registerDefaultSemHandlers()` during connect/bootstrap |
| First prompt starts a new hidden conversation | `conv_id` not persisted in URL/state | Keep `conv_id` in URL and Redux in sync |
| Events dropped on fast conversation switch | stale ws callbacks mutating current state | Use nonce/token cancellation like `connectNonce` |
| Unknown entity kinds render ugly JSON only | renderer not registered | register kind renderer via `registerTimelineRenderer(kind, component)` |

## 17. Recommended Extension Pattern

When you later add custom timeline kinds, keep extension points explicit and local:

- projection intake: `registerSem('my.kind.event', handler)`
- props shape normalization: `registerTimelinePropsNormalizer('my_kind', fn)`
- rendering: `registerTimelineRenderer('my_kind', Card)`

This avoids editing core chat window rendering logic and keeps module ownership clean.

## 18. Production Hardening Notes

Before production rollout, add the following:

- backoff and retry strategy for websocket reconnect,
- heartbeat/idle detection for stale sockets,
- telemetry around hydrate latency and replay count,
- per-conversation memory caps for buffered frames,
- structured UI error reporting (already scaffolded by `errorsSlice`).

Do not skip metrics around hydrate/replay; these are where timeline bugs hide.

## 19. Build Commands and Validation

Typical local validation loop:

```bash
# backend tests
cd pinocchio
go test ./cmd/web-chat/... -count=1

# frontend type checks/tests
cd pinocchio/cmd/web-chat/web
npm run check
npx vitest run
```

If you changed docs only, still run at least one targeted frontend test touching projection/WS behavior so regressions are caught early.

## 20. See Also

- `pkg/doc/tutorials/02-webchat-getting-started.md`
- `pkg/doc/tutorials/03-thirdparty-webchat-playbook.md`
- `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md`
- `pkg/doc/topics/webchat-sem-and-ui.md`
- `pkg/doc/topics/webchat-backend-internals.md`
