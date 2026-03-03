---
Title: 'Reusable Pinocchio Web Chat: analysis + extraction guide'
Ticket: PI-02-REUSABLE-PINOCCHIO-WEBCHAT
Status: active
Topics:
    - webchat
    - react
    - frontend
    - pinocchio
    - refactor
    - thirdparty
    - websocket
    - http-api
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/buf.gen.yaml
      Note: SEM protobuf codegen for TS/Go
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Example wiring (go:embed static
    - Path: pinocchio/cmd/web-chat/scripts/build-frontend.sh
      Note: Frontend build pipeline (static/dist)
    - Path: pinocchio/cmd/web-chat/timeline_js_runtime_loader.go
      Note: Cmd-owned JS script loader glue (candidate for extraction)
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: Frontend SEM handler registry
    - Path: pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts
      Note: Frontend basePrefix logic (runtime config + path heuristic)
    - Path: pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx
      Note: Primary reusable UI component and its extension points
    - Path: pinocchio/cmd/web-chat/web/src/ws/wsManager.ts
      Note: Frontend websocket + hydration gating
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: Conversation + ConvManager lifecycle (streaming
    - Path: pinocchio/pkg/webchat/doc.go
      Note: Ownership model (apps own /chat and /ws)
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: App-owned /chat
    - Path: pinocchio/pkg/webchat/http/profile_api.go
      Note: Reusable profile CRUD/schema HTTP handlers
    - Path: pinocchio/pkg/webchat/router.go
      Note: Utility mux (UI + core API) and static asset serving
    - Path: pinocchio/pkg/webchat/router_options.go
      Note: RouterOption hooks required for reuse
    - Path: pinocchio/pkg/webchat/sem_translator.go
      Note: SEM translation registry and default handlers
    - Path: pinocchio/pkg/webchat/server.go
      Note: Server lifecycle and NewServer contract
    - Path: pinocchio/pkg/webchat/stream_backend.go
      Note: Redis vs in-memory event stream backend
    - Path: pinocchio/pkg/webchat/stream_coordinator.go
      Note: Subscribe
    - Path: pinocchio/pkg/webchat/timeline_js_runtime.go
      Note: JS reducer/handler runtime for timeline projection
    - Path: pinocchio/pkg/webchat/timeline_projector.go
      Note: SEM -> timeline projection persistence
ExternalSources: []
Summary: Evidence-backed architecture map + refactor plan to make Pinocchio’s web-chat (Go server + React frontend) reusable from third-party packages.
LastUpdated: 2026-03-03T08:19:08.113338605-05:00
WhatFor: ""
WhenToUse: ""
---


# Reusable Pinocchio Web Chat: analysis + extraction guide

## Executive Summary

Pinocchio’s “web-chat” is already *mostly* reusable as a Go backend: the stable core lives in `pinocchio/pkg/webchat/*` and the app-owned HTTP handlers are in `pinocchio/pkg/webchat/http/*`. The remaining “not elegantly reusable” pieces for third parties are primarily:

- **Frontend source location**: the React/Vite app lives under `pinocchio/cmd/web-chat/web/` (example-app territory), which makes importing it from an external package undesirable.
- **UI asset embedding**: the example app embeds `cmd/web-chat/static` via `//go:embed static` (`pinocchio/cmd/web-chat/main.go:38-39`) and feeds that FS to `webchat.NewServer(...)`. A third party needs a clean, `pkg/...`-level way to embed/serve the same UI (or their own).
- **Runtime config injection**: the UI expects `window.__PINOCCHIO_WEBCHAT_CONFIG__` and `./app-config.js` (`pinocchio/cmd/web-chat/static/dist/index.html:7`) which is currently generated/served by the example app (`pinocchio/cmd/web-chat/main.go:65-74`, `pinocchio/cmd/web-chat/main.go:235-249`).
- **Timeline JS script loader**: the JS timeline runtime exists in `pkg` (`pinocchio/pkg/webchat/timeline_js_runtime.go:26+`), but the “load scripts from CLI flags / add require global folders / SetTimelineRuntime” glue is in `cmd` (`pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27-52`).

This document maps the backend + frontend architecture end-to-end (with evidence anchored to files), then proposes a concrete extraction/refactor plan so a third-party module can reuse:

1. **Go backend** via `pinocchio/pkg/webchat` + `pinocchio/pkg/webchat/http`.
2. **React UI source** as a reusable package (recommended: publishable workspace package).
3. **Optional prebuilt UI assets** as a Go-embedded `fs.FS` exported from `pkg`, for “just serve the UI” integrations.

## Problem Statement

We want a third-party package/application (outside `pinocchio/`) to reuse Pinocchio’s web-chat *end-to-end*:

- **Backend**: HTTP + WebSocket streaming + SEM translation + optional timeline persistence/hydration + optional debug API + profiles API.
- **Frontend**: React components, state/store, SEM handlers, renderer registry, and theming/extension points.

…without depending on `pinocchio/cmd/web-chat` (or any `cmd/...`) as a library.

### Why this is currently hard

Observed boundaries in the repo:

- The reusable backend pieces explicitly state an **app-owned transport model** (“apps own `/chat` and `/ws`”) in `pinocchio/pkg/webchat/doc.go:3-12`.
- The example server (`cmd/web-chat`) wires the app-owned routes and embeds the UI assets (`pinocchio/cmd/web-chat/main.go:38-39`, `pinocchio/cmd/web-chat/main.go:220-249`).
- The React UI is coupled to that canonical route set and to an injected runtime config (`pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:142-193`, `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:57-140`, `pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts:25-46`).

So a third party has to either:

- copy code out of `cmd/web-chat/web`, or
- import `pinocchio/cmd/...` (undesirable), or
- rebuild the UI from scratch.

### Success criteria

- A third-party Go app can run a web-chat server by importing **only** `pinocchio/pkg/...` packages.
- A third-party React app can render the Pinocchio Chat UI by importing a **reusable UI package** (no deep imports into a “cmd app” directory).
- The backend/frontend contract is explicit, versionable, and easy to extend.

### Non-goals (explicit)

- Authentication / multitenancy / user management: still app-owned.
- Designing a universal “Pinocchio product UI”: we focus on reusable *webchat* pieces, not global app shell.

## Current State (Evidence-Based Architecture Map)

This section is intentionally “how it works today”, with file anchors, so reuse/refactor decisions are grounded.

### Top-level layout

Backend:

- Example app entrypoint: `pinocchio/cmd/web-chat/main.go`
- Reusable core: `pinocchio/pkg/webchat/*`
- Reusable HTTP handlers: `pinocchio/pkg/webchat/http/*`
- SEM protobufs (Go): `pinocchio/pkg/sem/pb/...`
- Timeline persistence: `pinocchio/pkg/persistence/chatstore/*`

Frontend:

- React/Vite app (example): `pinocchio/cmd/web-chat/web/*`
- Built assets served by backend: `pinocchio/cmd/web-chat/static/dist/*`
- Runtime config injection: `pinocchio/cmd/web-chat/static/dist/index.html:7`, `pinocchio/cmd/web-chat/main.go:65-74`

### Backend: core composition boundaries

Key design constraint (already documented in code):

- `pinocchio/pkg/webchat/doc.go:3-12`:
  - apps own `/chat` and `/ws`
  - package provides optional utilities for UI + core APIs

#### “Server” is lifecycle + convenience, not a full app

`webchat.Server` wraps:

- An event router loop (`events.EventRouter`)
- An `http.Server` lifecycle
- Accessors to services/handlers

Evidence:

- `pinocchio/pkg/webchat/server.go:20-31` states the server does **not** add `/chat` or `/ws`.
- `pinocchio/pkg/webchat/server.go:28-43` shows `NewServer(ctx, parsed, staticFS, opts...)` -> `NewRouter` -> `BuildHTTPServer`.
- `pinocchio/pkg/webchat/server.go:117-171` runs event router + HTTP server + shutdown/cleanup.

#### “Router” owns the *utility mux*: UI + core API (timeline/debug)

Evidence:

- `pinocchio/pkg/webchat/types.go:31-78` shows `Router` holds:
  - `mux *http.ServeMux`
  - `staticFS fs.FS`
  - stream backend (publisher/subscriber)
  - services: `ChatService`, `StreamHub`, `TimelineService`, stores, etc.
- `pinocchio/pkg/webchat/router.go:45-47` “does not register app-owned transport routes such as /chat or /ws.”
- `pinocchio/pkg/webchat/router.go:281-339` registers UI assets and `"/"` index fallback behavior.
- `pinocchio/pkg/webchat/router.go:342-349` registers API handlers: timeline always, debug optionally.
- `pinocchio/pkg/webchat/router.go:192-204` has `Mount(...)` helper (prefix + `StripPrefix`).

Core API utilities:

- Timeline snapshot endpoint built into Router: `pinocchio/pkg/webchat/router_timeline_api.go:16-22`.
- Debug endpoints under `/api/debug/*`: `pinocchio/pkg/webchat/router_debug_routes.go:21+` (gated by `WithDebugRoutesEnabled` / `Router.enableDebugRoutes`).

Note: there is **duplication** here because there is also a timeline handler in `pinocchio/pkg/webchat/http/api.go:222-273` (see “Refactor opportunities” later).

#### “StreamBackend” selects Redis vs in-memory subscription

Evidence:

- `pinocchio/pkg/webchat/stream_backend.go:14-21` defines `StreamBackend` (EventRouter, Publisher, BuildSubscriber).
- `pinocchio/pkg/webchat/stream_backend.go:29-46` builds router based on redis settings decoded from `values.Values`.
- `pinocchio/pkg/webchat/stream_backend.go:63-82` shows:
  - Redis enabled -> ensure stream group -> build group subscriber -> return `subClose=true`
  - Redis disabled -> reuse in-memory subscriber -> `subClose=false`

This matters for reuse because a third-party app must decide:

- “am I okay with Glazed `values.Values` configuration?” (today: required by `webchat.NewServer/NewRouter`), or
- “do I want a `NewRouterFromConfig` API?” (proposed later).

### Backend: conversation lifecycle (the runtime core)

This is the minimum mental model an intern needs to safely reuse/extend.

#### Objects and responsibilities (today)

```
Conversation (per conv_id)
  - owns engine + sink + session state
  - owns StreamCoordinator (subscribes to chat:<conv_id>)
  - owns ConnectionPool (fanout to WS clients)
  - owns semFrameBuffer (in-memory “recent frames”)
  - owns TimelineProjector (optional; persists + emits timeline.upsert)

ConvManager (process-wide)
  - map[conv_id]*Conversation
  - creates/rebuilds conversations when runtime fingerprint changes
  - starts stream immediately; stops on idle when no sockets

ConversationService / ChatService / StreamHub
  - “service surface” used by HTTP handlers
```

Evidence:

- `pinocchio/pkg/webchat/conversation.go:27-62` shows the `Conversation` fields.
- `pinocchio/pkg/webchat/conversation.go:64-82` shows `ConvManager` storage + dependencies.
- `pinocchio/pkg/webchat/conversation_service.go:25-47` shows `ConversationService` has:
  - `streams *StreamHub`
  - `timelineStore`, `turnStore`
  - `semPublisher` and a `timelineUpsert` hook
- `pinocchio/pkg/webchat/chat_service.go:17-21` describes ChatService as “queue/idempotency/inference” and excludes websockets.
- `pinocchio/pkg/webchat/stream_hub.go:24-29` describes StreamHub as websocket attachment owner.

#### Conversation creation/rebuild rules

`ConvManager.GetOrCreate(...)` composes a runtime and either:

- reuses an existing conversation if runtime fingerprint unchanged, or
- rebuilds engine/sink/subscriber/stream coordinator if fingerprint changed.

Evidence (selected key points):

- Dependency requirements: `pinocchio/pkg/webchat/conversation.go:259-261` requires runtime composer + subscriber builder.
- Runtime fingerprint selection: `pinocchio/pkg/webchat/conversation.go:279-284`.
- Timeline projector auto-enabled when timeline store is present:
  - for existing conv: `pinocchio/pkg/webchat/conversation.go:300-305`
  - for new conv: `pinocchio/pkg/webchat/conversation.go:383-385`
- StreamCoordinator callback fanout and projection:
  - `pinocchio/pkg/webchat/conversation.go:336-356` (existing conv rebuild path)
  - `pinocchio/pkg/webchat/conversation.go:403-424` (new conv path)

That callback:

- broadcasts SEM frames to sockets (`ConnectionPool.Broadcast`)
- appends to `semBuf`
- applies the SEM frame into `TimelineProjector` (if enabled), which persists entities and emits `timeline.upsert` frames back to WS clients.

#### StreamCoordinator: “events in, SEM frames out”

The StreamCoordinator subscribes to the conversation topic and ensures ordering by stamping `seq`:

- `pinocchio/pkg/webchat/stream_coordinator.go:24-38` describes ownership.
- Subscription: `pinocchio/pkg/webchat/stream_coordinator.go:127-141`.
- Ordering cursor (`stream_id`, `seq`) creation: `pinocchio/pkg/webchat/stream_coordinator.go:144-151`, `pinocchio/pkg/webchat/stream_coordinator.go:192-218`.
- Fast-path: if message payload is already a SEM envelope (`{"sem": true, "event": ...}`), patch it with cursor fields and forward as-is: `pinocchio/pkg/webchat/stream_coordinator.go:152-163`.
- Otherwise decode event JSON and translate to SEM frames via registry-based translator: `pinocchio/pkg/webchat/stream_coordinator.go:165-182`.

The translator is registry-based (no giant switch):

- `pinocchio/pkg/webchat/sem_translator.go:25-38` describes EventTranslator and default handler registration.
- `pinocchio/pkg/webchat/sem_translator.go:132-170` shows `SemanticEventsFromEvent` and `Translate(...)` using `semregistry.Handle(e)`.

#### WebSocket fanout and connection semantics

ConnectionPool is a bounded-buffer writer with drop-on-backpressure:

- `pinocchio/pkg/webchat/connection_pool.go:11-15` default buffer + timeout.
- `pinocchio/pkg/webchat/connection_pool.go:106-126` broadcast: if send buffer full -> drop client.
- `pinocchio/pkg/webchat/connection_pool.go:225-247` idle timer triggers callback when last socket disconnects.

StreamHub is the HTTP-visible websocket attach API:

- Create/resolve conv: `pinocchio/pkg/webchat/stream_hub.go:43-71`.
- Attach WS, send optional hello and handle ping/pong: `pinocchio/pkg/webchat/stream_hub.go:73-166`.

#### Timeline persistence + hydration contract

TimelineStore is a projection store keyed by monotonic version:

- Interface: `pinocchio/pkg/persistence/chatstore/timeline_store.go:23-34` (`Upsert`, `GetSnapshot`, etc).
- TimelineService reads snapshots: `pinocchio/pkg/webchat/timeline_service.go:12-26`.
- Router’s timeline API: `pinocchio/pkg/webchat/router_timeline_api.go:41-92` serves snapshot JSON.

TimelineProjector converts incoming SEM frames into `TimelineEntityV2` upserts:

- `pinocchio/pkg/webchat/timeline_projector.go:30-46` describes projector role.
- Tool result ID convention matches frontend (`:result` / `:custom`):
  - `pinocchio/pkg/webchat/timeline_projector.go:288-315`.

TimelineUpsert frames are emitted back to WS clients by default:

- `pinocchio/pkg/webchat/conversation_service.go:274-298` publishes `{"sem": true, "event": {"type":"timeline.upsert", "id": entity.Id, "seq": version}}`.

Optional timeline runtime customization:

- Handler registry + runtime bridge: `pinocchio/pkg/webchat/timeline_registry.go:20-100`.
- JS runtime exists in `pkg`: `pinocchio/pkg/webchat/timeline_js_runtime.go:26-37`.
- Loading scripts from paths is currently in cmd: `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27-52`.

### Backend: canonical HTTP contract (what the UI assumes)

The example app wires these “app-owned” routes:

- `POST /chat` (and `/chat/`) -> `webhttp.NewChatHandler(...)` (`pinocchio/cmd/web-chat/main.go:220-229`, `pinocchio/pkg/webchat/http/api.go:104-164`)
- `GET /ws?conv_id=...` -> `webhttp.NewWSHandler(...)` (`pinocchio/cmd/web-chat/main.go:221-230`, `pinocchio/pkg/webchat/http/api.go:166-220`)
- `GET /api/timeline?conv_id=...` -> `webhttp.NewTimelineHandler(...)` (`pinocchio/cmd/web-chat/main.go:231-234`, `pinocchio/pkg/webchat/http/api.go:222-273`)

The example app also mounts:

- profile API handlers under `/api/chat/*` (implementation is currently split between `cmd/web-chat/profile_policy.go` and shared pkg handlers in `pinocchio/pkg/webchat/http/profile_api.go:137+`)
- core API utilities under `/api/` via `srv.APIHandler()` (`pinocchio/cmd/web-chat/main.go:248`)
- UI under `/` via `srv.UIHandler()` (`pinocchio/cmd/web-chat/main.go:249`)

Root prefix mounting (e.g. `--root /chat`) is implemented by wrapping the mux in a parent mux and `StripPrefix`:

- `pinocchio/cmd/web-chat/main.go:257-274` shows the pattern.

### Frontend: React architecture and extension points (today)

The UI’s entry point is tiny:

- `pinocchio/cmd/web-chat/web/src/App.tsx:15-19` renders `<ChatWidget />` inside a Redux `<Provider store={store}>`.

State/store:

- Store wiring: `pinocchio/cmd/web-chat/web/src/store/store.ts:9-18` combines slices:
  - `appSlice`, `timelineSlice`, `errorsSlice`, `profileApi`
- Timeline merge semantics (version-aware): `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts:59-98`

ChatWidget responsibilities:

- Read `conv_id` from URL and keep it in query params: `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:27-52`.
- Connect WebSocket and hydrate timeline:
  - `wsManager.connect({ convId, basePrefix, hydrate: true })`: `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:134-153`
  - basePrefix is derived from either runtime config or first path segment: `pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts:25-33`
- Submit prompt via `POST ${basePrefix}/chat`: `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:162-193`
- Provide extension points:
  - theming via `unstyled`, `theme`, `themeVars`
  - component slots (Header/Statusbar/Composer)
  - renderer overrides / registry
  - `partProps` to add attributes/styles per UI “part”
  - `buildOverrides` to inject request overrides

Evidence for extension types:

- `pinocchio/cmd/web-chat/web/src/webchat/types.ts:68-88` defines `ChatWidgetProps`, slots, and renderers.
- `pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts:13-47` supports registering custom timeline renderers.
- `pinocchio/cmd/web-chat/web/src/webchat/parts.ts:16-25` shows the “part props” pattern.

WebSocket + hydration gate:

- Connects to `${basePrefix}/ws?conv_id=...`: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:80-83`
- Buffers frames until hydration completes: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:123-126`, then sorts buffered frames by `event.seq`: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:212-219`
- Hydrates from `/api/timeline`: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:171-201`

SEM dispatch on frontend:

- Handler registry and default handlers: `pinocchio/cmd/web-chat/web/src/sem/registry.ts:117-256`
  - `timeline.upsert` is the “canonical” entity update path when timeline projection is enabled (`pinocchio/cmd/web-chat/web/src/sem/registry.ts:121-128`).

Runtime config injection:

- Reads `window.__PINOCCHIO_WEBCHAT_CONFIG__`: `pinocchio/cmd/web-chat/web/src/config/runtimeConfig.ts:6-24`
- basePrefix derivation uses runtime config if present: `pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts:25-33`
- built `index.html` loads `./app-config.js`: `pinocchio/cmd/web-chat/static/dist/index.html:7`
- example app generates that script: `pinocchio/cmd/web-chat/main.go:65-74` and serves it: `pinocchio/cmd/web-chat/main.go:235-249`

### Build pipeline: frontend -> static/dist -> go:embed

The frontend build produces `cmd/web-chat/static/dist`:

- `pinocchio/cmd/web-chat/web/package.json:10` `vite build --outDir ../static/dist`
- Shell helper: `pinocchio/cmd/web-chat/scripts/build-frontend.sh:20-22`

Backend embeds `cmd/web-chat/static`:

- `pinocchio/cmd/web-chat/main.go:38-39` `//go:embed static` -> `staticFS embed.FS`

Router UI handler serves:

- `/static/*` from embedded FS `static` subdir: `pinocchio/pkg/webchat/router.go:310-316`
- `/assets/*` from `static/dist/assets`: `pinocchio/pkg/webchat/router.go:317-322`
- `"/"` index from `static/dist/index.html` fallback to `static/index.html`: `pinocchio/pkg/webchat/router.go:324-339`

## What “Reuse” Actually Means (Contracts + Extension Points)

This section reframes the system into “stable contracts” a third party must implement or can depend on.

### Backend contracts

Minimum routes (must exist, under a consistent base prefix):

- `POST /chat` (JSON body contains `conv_id`, `prompt`, optional `request_overrides`)
  - Evidence for request shape: `pinocchio/pkg/webchat/http/api.go:20-29`, `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:171-183`
- `GET /ws?conv_id=<id>` upgrades to WebSocket
  - Evidence: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:80-83`, `pinocchio/pkg/webchat/http/api.go:166-220`
- `GET /api/timeline?conv_id=<id>` returns protobuf-json snapshot
  - Evidence: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:171-189`, `pinocchio/pkg/webchat/http/api.go:222-273`

Recommended routes:

- Profile CRUD and schema APIs under `/api/chat/*` (used by the UI to populate the profile selector):
  - Evidence: `pinocchio/cmd/web-chat/web/src/store/profileApi.ts:88-112`
  - Shared handler implementation: `pinocchio/pkg/webchat/http/profile_api.go:137+`

### SEM envelope contract

The frontend expects SEM frames shaped like:

```json
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "some-id",
    "seq": 1707053365123000000,
    "stream_id": "1707053365123-0",
    "data": { "cumulative": "..." }
  }
}
```

Key invariants:

- `sem: true` and `event.type` is the dispatch key (`pinocchio/cmd/web-chat/web/src/sem/registry.ts:34-40`).
- `event.seq` is used to order buffered WS frames during hydration (`pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:212-219`).
- Backend ensures `seq` is monotonic per conversation by stamping cursor values (`pinocchio/pkg/webchat/stream_coordinator.go:144-151`, `pinocchio/pkg/webchat/stream_coordinator.go:192-218`).

### Frontend contracts

The React UI depends on:

- A Redux store that includes at least:
  - `timelineSlice` and actions used by SEM handlers
  - `appSlice` (convId, wsStatus, lastSeq, queueDepth)
  - `errorsSlice`
  - `profileApi` (optional but used by default ChatWidget header)
- A runtime config injection mechanism (optional but recommended), with:
  - `basePrefix` (string)
  - `debugApiEnabled` (boolean)

Evidence:

- Runtime config interface: `pinocchio/cmd/web-chat/web/src/config/runtimeConfig.ts:1-24`
- Base prefix logic: `pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts:25-33`

## Proposed Solution (Make Webchat Elegantly Reusable)

This is the “target shape” we want a third-party intern to be able to implement with minimal glue.

### A) Backend reuse: keep `pkg/webchat` as the stable core (mostly done)

Today, a third party can already reuse the backend by:

1. Building a `webchat.Server` with `webchat.NewServer(...)` and `webchat.WithRuntimeComposer(...)` (`pinocchio/pkg/webchat/server.go:28-43`, `pinocchio/pkg/webchat/router_options.go:18-26`).
2. Mounting app-owned `/chat` and `/ws` via `pinocchio/pkg/webchat/http` handlers (`pinocchio/pkg/webchat/http/api.go:104-220`).
3. Mounting timeline hydration (`/api/timeline`) either via:
   - `webhttp.NewTimelineHandler(...)` (`pinocchio/pkg/webchat/http/api.go:222-273`), or
   - `srv.APIHandler()` (Router core API) (`pinocchio/pkg/webchat/router_timeline_api.go:16-22`)

What’s missing for “elegant reuse” is not the core engine lifecycle — it’s the “packaging and defaults” (assets, app-config, JS script loading, and the UI source location).

### B) Frontend reuse: extract a real “webchat-ui” package

Recommended target: make a reusable React package that exports the stable primitives:

- `ChatWidget` (main component)
- `createWebchatStore()` (or `configureWebchatStore()`) returning a Redux store instance
- `registerDefaultSemHandlers()` + a way to register custom SEM handlers
- `registerTimelineRenderer()` for custom timeline cards
- CSS (default theme + layout), optionally behind “unstyled” mode

We already have the internal shape to support this:

- `ChatWidget` has explicit extension props (`pinocchio/cmd/web-chat/web/src/webchat/types.ts:78-88`).
- Renderer registry is global + overridable (`pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts:20-47`).

What needs refactoring is **directory/module structure**, not core logic.

### C) Optional: export embedded UI assets from `pkg` for “Go-only consumers”

Some third parties will not want to maintain a Node toolchain. For them, provide:

- `pinocchio/pkg/webchat/uiassets` (new): exports `func FS() fs.FS` or `var Static embed.FS`
- `pinocchio/pkg/webchat/http/appconfig` (new): exports a handler or script generator for `app-config.js`

Then a third-party Go app can simply:

- embed the provided FS
- mount `srv.UIHandler()` and `app-config.js` handler
- mount the canonical APIs

### D) Move the timeline JS script loader glue from cmd to pkg

The runtime is reusable already (`pinocchio/pkg/webchat/timeline_js_runtime.go:26-37`), but the “configure from flags” part is not.

Proposed new `pkg` helper:

- `pinocchio/pkg/webchat/timelinejs` (new) with:
  - `NormalizePaths(raw []string) []string` (based on `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:13-25`)
  - `ConfigureFromPaths(paths []string) error` (based on `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27-52`)

This is small refactor with high leverage.

## Refactor Opportunities (Concrete, Evidence-Based)

These are changes that materially improve reuse while minimizing churn.

### 1) Remove the “cmd-owned embed FS” bottleneck

Problem:

- The only embed FS currently used by the UI server is in `cmd/web-chat` (`pinocchio/cmd/web-chat/main.go:38-39`).

Proposal:

- Introduce `pinocchio/pkg/webchat/uiassets` that embeds a *copy* of the built `static/dist` (and optionally `static/index.html`) for the default UI.
- Update `cmd/web-chat` to use that package instead of embedding itself.

Why:

- Makes UI-serving reusable without importing `cmd/...`.

### 2) Promote `app-config.js` generation to a reusable helper

Problem:

- UI runtime config is generated in `cmd` (`pinocchio/cmd/web-chat/main.go:65-74`) and served with an app-owned handler (`pinocchio/cmd/web-chat/main.go:235-249`).

Proposal:

- Add `pinocchio/pkg/webchat/http/appconfig`:
  - `func Script(basePrefix string, debugAPI bool) (string, error)`
  - `func Handler(script string) http.HandlerFunc` (or `NewHandler(cfg)` that marshals at request time)

Then all apps (including `cmd/web-chat`) call the same helper.

### 3) Consolidate the timeline HTTP handler

Problem:

- There are two similar timeline handlers:
  - `pinocchio/pkg/webchat/router_timeline_api.go:16-22` (Router core API)
  - `pinocchio/pkg/webchat/http/api.go:222-273` (webhttp helper)

Proposal:

- Keep *one* canonical implementation (prefer `pkg/webchat/http`, since it’s already the “app-owned handler” layer) and have Router call into it.
- This reduces drift and reduces “which handler do I mount?” confusion for third parties.

### 4) Extract the React UI into a workspace package

Problem:

- The reusable parts live under an example command (`pinocchio/cmd/web-chat/web/src/webchat/*`), not a package boundary.

Proposal (minimal-disruption):

1. Create `pinocchio/webchat-ui/` (new) with `package.json`, `src/` and Storybook config.
2. Move or copy:
   - `cmd/web-chat/web/src/webchat/*` -> `webchat-ui/src/*`
   - `cmd/web-chat/web/src/sem/*`, `ws/*`, `store/*`, `utils/*` as needed
3. Make the example app `cmd/web-chat/web` depend on `webchat-ui` (workspace / relative dependency).
4. Define a public API surface for the UI package (no deep imports).

This can be done incrementally:

- Phase 1: extract without changing runtime behavior.
- Phase 2: tighten the exported API and remove unnecessary exports.

### 5) (Optional) Add a “no glazed values” constructor for the backend

Problem:

- `webchat.NewServer/NewRouter` take `*values.Values` and decode settings (`pinocchio/pkg/webchat/router.go:66-92`, `pinocchio/pkg/webchat/router.go:239-261`, `pinocchio/pkg/webchat/stream_backend.go:29-46`).

If a third-party app already uses Glazed, this is fine.

If not, we can add:

- `NewRouterFromConfig(ctx, RouterSettings, StreamBackend, staticFS, opts...)`
- or `NewServerFromConfig(...)`

This is a larger refactor; treat as optional unless external consumers demand it.

## Recommended Target Architecture (After Refactor)

### Backend (Go): “app-owned routes” + small helpers

Expose a single “wiring helper” for third parties, while keeping ownership app-side:

```text
pinocchio/pkg/webchat/appwiring  (new)
  - RegisterCoreHandlers(mux, srv, resolver, opts)
  - RegisterUI(mux, srv, uiFS, basePrefix, debugApiEnabled)
  - RegisterProfiles(mux, profileRegistry, middlewareDefs, extensionSchemas, ...)
```

This keeps the “applications own /chat and /ws” model, but drastically reduces boilerplate.

### Frontend (React): “webchat-ui” as a package + example app wrapper

Public exports (suggested):

```ts
// @go-go-golems/pinocchio-webchat-ui
export { ChatWidget } from './ChatWidget';
export { createWebchatStore, WebchatProvider } from './store';
export { registerSem, registerDefaultSemHandlers } from './sem';
export { registerTimelineRenderer } from './renderers';
export type { ChatWidgetProps, ThemeVars, RenderEntity } from './types';
```

Example app:

- `cmd/web-chat/web` becomes “just an app wrapper” that:
  - creates a store
  - renders `<ChatWidget />`
  - maybe provides debug UI toggles

## Implementation Plan (Phased, Intern-Friendly)

### Phase 0 — Document contracts (this ticket)

Deliverables:

- This design doc + recipes
- Explicit backend route table + SEM envelope invariants
- Explicit UI package API proposal

### Phase 1 — Extract `webchat-ui` package (no behavior change)

Steps:

1. Create `pinocchio/webchat-ui/` with:
   - `package.json`, `tsconfig.json`, `vite.config.ts`
   - `src/` containing extracted code
2. Move the reusable UI pieces:
   - `ChatWidget`, renderer registry, SEM registry, ws manager, store slices
3. Update `cmd/web-chat/web` to import from the package and build the same output to `cmd/web-chat/static/dist`.

Validation:

- `npm --prefix pinocchio/cmd/web-chat/web run build` still produces the same UI output.
- Storybook still renders (if used).

### Phase 2 — Provide `pkg/webchat/uiassets` (embed default UI)

Steps:

1. Make `webchat-ui` build output land in a location that can be embedded by Go (e.g. `pinocchio/pkg/webchat/uiassets/static/dist`).
2. Add a `go:generate` in `pkg/webchat/uiassets` to run the frontend build (optional; depends on repo policy).
3. Export a stable `fs.FS`.
4. Update `cmd/web-chat` to use `uiassets.FS()` instead of embedding `static` directly.

Validation:

- `go test ./pinocchio/cmd/web-chat -count=1` (or the repo’s preferred suite)
- manual: `go run ./pinocchio/cmd/web-chat --root /chat` serves UI and connects.

### Phase 3 — Promote `app-config.js` helper into pkg

Steps:

1. Implement `pinocchio/pkg/webchat/http/appconfig` helper.
2. Update `cmd/web-chat` to call the helper.
3. Update docs/recipes so third parties do not copy/paste from cmd.

Validation:

- UI still sets basePrefix and profile API calls still work under `--root`.

### Phase 4 — Move timeline JS script loader into pkg

Steps:

1. Create a small package that configures JS runtime from file paths (adapt from `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27-52`).
2. Update `cmd/web-chat` to use it.

Validation:

- Existing behavior with `--timeline-js-script` stays the same.

## Testing and Validation Strategy

Backend:

- Prefer existing integration tests under `pinocchio/cmd/web-chat/*_test.go` (e.g. `app_owned_chat_integration_test.go` exists per `rg` output).
- Add focused unit tests where new helper packages are introduced (app-config helper, JS script loader).

Frontend:

- Keep/extend Vitest tests (e.g. `pinocchio/cmd/web-chat/web/src/store/profileApi.test.ts` exists per `rg` output).
- Add a Storybook story demonstrating:
  - default render
  - custom renderer registration
  - unstyled mode + themeVars

Manual smoke tests (must remain easy for interns):

- Start server with `--root /chat` and verify:
  - `GET /chat/` loads UI
  - `WS /chat/ws?conv_id=...` connects
  - `GET /chat/api/timeline?conv_id=...` returns JSON
  - profile list loads from `/chat/api/chat/profiles`

## Alternatives Considered

### Alternative 1: Keep everything in `cmd/web-chat` and ask third parties to copy/paste

Pros:

- No refactor work.

Cons:

- Leads to divergent forks of the UI and “invisible” behavioral drift.
- Makes bugfixes and feature additions expensive (must be re-applied in N downstream copies).

### Alternative 2: Only ship prebuilt UI assets; no reusable React source

Pros:

- Simplifies “serve UI” for Go apps.

Cons:

- Makes UI customization (new renderers, custom layout, new features) much harder.
- Third parties will still fork/build their own UI eventually.

Recommendation:

- Do both: extract reusable React source *and* optionally provide embedded prebuilt assets.

## Risks and Sharp Edges

- **Route prefixing**: the UI’s basePrefix logic defaults to “first path segment” when runtime config is absent (`pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts:25-33`). Nested mounts like `/a/b/chat` require explicit `basePrefix` injection via app-config.
- **Duplicate timeline projections**: when timeline projection is enabled, the backend emits both raw SEM events and `timeline.upsert` SEM events. The frontend should treat `timeline.upsert` as canonical; ensure handler semantics remain idempotent.
- **Glazed `values.Values` coupling**: backend constructors currently require `*values.Values`. This is fine inside Pinocchio, but may be friction for external consumers.
- **Node toolchain in Go module**: embedding UI assets requires a build step; decide whether CI/build should run Node or whether to check in built assets.

## Open Questions (Decisions Needed)

1. Do we want to **check in** built `static/dist` assets in git (for deterministic Go builds), or require `go generate` / CI to build them?
2. Should the UI package be published to npm as:
   - `@go-go-golems/pinocchio-webchat-ui`, or
   - an internal workspace-only package?
3. Do we want a “no glazed dependency” backend constructor, or is Glazed acceptable for third-party consumers?

## References (Key Files and APIs)

Backend:

- `pinocchio/pkg/webchat/doc.go:1` — ownership model (apps own `/chat` and `/ws`)
- `pinocchio/pkg/webchat/server.go:20` — `Server` lifecycle, `NewServer`, `Run`
- `pinocchio/pkg/webchat/types.go:31` — `Router` fields and boundaries
- `pinocchio/pkg/webchat/router.go:192` — `Mount(...)` prefix helper
- `pinocchio/pkg/webchat/router.go:302` — UI handler: `/static`, `/assets`, index fallback
- `pinocchio/pkg/webchat/stream_backend.go:14` — redis vs in-memory stream backend
- `pinocchio/pkg/webchat/conversation.go:248` — `ConvManager.GetOrCreate` lifecycle + StreamCoordinator callback
- `pinocchio/pkg/webchat/stream_coordinator.go:24` — subscribe -> translate -> stamp seq
- `pinocchio/pkg/webchat/sem_translator.go:25` — registry-based SEM translation
- `pinocchio/pkg/webchat/timeline_projector.go:30` — timeline projection semantics
- `pinocchio/pkg/webchat/timeline_registry.go:20` — custom timeline handler registry + runtime bridge
- `pinocchio/pkg/webchat/timeline_js_runtime.go:26` — JS reducers/handlers API
- `pinocchio/pkg/webchat/http/api.go:104` — app-owned `/chat` and `/ws` handlers
- `pinocchio/pkg/webchat/http/profile_api.go:137` — profile CRUD/schema handler registration

Example app (what to avoid importing as a library):

- `pinocchio/cmd/web-chat/main.go:38` — embeds static assets (cmd-owned today)
- `pinocchio/cmd/web-chat/main.go:65` — generates `window.__PINOCCHIO_WEBCHAT_CONFIG__`
- `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27` — JS timeline script loader glue

Frontend:

- `pinocchio/cmd/web-chat/web/src/App.tsx:15` — `<Provider store={store}><ChatWidget /></Provider>`
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:65` — main component + extension props
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:57` — WS connect + hydration gating
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts:117` — default SEM handlers (incl. `timeline.upsert`)
- `pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts:22` — register/resolve timeline renderers
- `pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts:25` — basePrefix derivation
- `pinocchio/cmd/web-chat/static/dist/index.html:7` — loads `./app-config.js`

Build/proto:

- `pinocchio/cmd/web-chat/scripts/build-frontend.sh:20` — builds UI into `static/dist`
- `pinocchio/buf.gen.yaml:1` — generates TS + Go SEM protobufs (note dual TS outputs)
