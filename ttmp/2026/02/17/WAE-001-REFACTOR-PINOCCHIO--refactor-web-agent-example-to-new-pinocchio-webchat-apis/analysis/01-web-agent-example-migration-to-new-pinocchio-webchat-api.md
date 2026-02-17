---
Title: Web-agent-example migration to new Pinocchio webchat API
Ticket: WAE-001-REFACTOR-PINOCCHIO
Status: complete
Topics:
    - chat
    - backend
    - refactor
    - analysis
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../web-agent-example/cmd/web-agent-example/engine_from_req.go
      Note: Request resolver still bound to old root package types
    - Path: ../../../../../../../web-agent-example/cmd/web-agent-example/main.go
      Note: |-
        Current external integration still using pre-extraction webchat symbols
        Compile-failing external wiring using old handler symbols
    - Path: ../../../../../../../web-agent-example/cmd/web-agent-example/runtime_composer.go
      Note: |-
        Main compile-failing file after runtime type extraction
        Compile-failing consumer of moved runtime types
    - Path: ../../../../../../../web-agent-example/cmd/web-agent-example/sink_wrapper.go
      Note: EventSinkWrapper closure still bound to old root runtime types
    - Path: cmd/web-chat/main.go
      Note: |-
        Canonical app-owned route composition and server wiring
        Reference app route and service wiring
    - Path: cmd/web-chat/profile_policy.go
      Note: Reference request resolver and runtime/profile policy ownership
    - Path: cmd/web-chat/runtime_composer.go
      Note: |-
        Reference runtime composer implementation against new inference runtime APIs
        Reference runtime composer against extracted runtime APIs
    - Path: pkg/inference/runtime/composer.go
      Note: |-
        New location of RuntimeComposeRequest/RuntimeArtifacts/RuntimeComposer
        New runtime compose contracts used by external apps
    - Path: pkg/inference/runtime/engine.go
      Note: New location of MiddlewareFactory/MiddlewareUse/ComposeEngineFromSettings
    - Path: pkg/webchat/http/api.go
      Note: |-
        New location of ConversationRequestPlan, resolver errors, and HTTP handler constructors
        New request-plan and HTTP handler contracts
ExternalSources: []
Summary: Deep migration analysis for aligning web-agent-example with the post-GP-026 Pinocchio webchat and runtime API surfaces.
LastUpdated: 2026-02-17T00:00:00-05:00
WhatFor: Provide implementation-grade context for migrating web-agent-example to new APIs using cmd/web-chat as the reference architecture.
WhenToUse: Use when upgrading or reviewing any third-party app that embeds Pinocchio webchat from outside the pinocchio module.
---


# Web-agent-example Migration Analysis to New Pinocchio Webchat API

## Goal

This document provides a thorough migration analysis for `web-agent-example` after the recent Pinocchio webchat refactors. It has two purposes:

1. Explain, in detail, how `pinocchio/cmd/web-chat` works today against the new API surfaces.
2. Provide a concrete refactor/rewrite blueprint for `web-agent-example` as an external consumer of Pinocchio.

The immediate trigger is current compile failure in `web-agent-example`, caused by API extractions from `pkg/webchat` into newer package boundaries.

## Baseline Failure Snapshot

Running `go test ./...` in `web-agent-example` currently fails with missing symbols that no longer exist in `pkg/webchat`, including:

- `webchat.MiddlewareFactory`
- `webchat.RuntimeComposeRequest`
- `webchat.RuntimeArtifacts`
- `webchat.MiddlewareUse`
- `webchat.ConversationRequestPlan`

This is expected after the refactor series that moved runtime contracts into `pkg/inference/runtime` and HTTP boundary contracts into `pkg/webchat/http`.

---

## Section 1: How `pinocchio/cmd/web-chat` Works Now, Which APIs It Uses, and What Documentation Matters

### 1.1 Context: What changed recently and why this matters

The migration is not a random rename exercise. It is the result of a deliberate architecture split across several recent tickets in `pinocchio/ttmp`:

- `GP-022`: profile semantics decoupled from `pkg/webchat` core and moved toward app-owned policy.
- `GP-023`: runtime composition policy extraction from webchat core into app-owned composer boundaries.
- `GP-025`: app-owned `/chat` and `/ws` routes as the default ownership model.
- `GP-026-WEBCHAT-CORE-EXTRACTIONS`: core type extraction pass:
  - runtime compose contracts moved from `pkg/webchat` to `pkg/inference/runtime`
  - HTTP request/handler contracts moved from root into `pkg/webchat/http`
- `GP-026-WEBCHAT-PUBLIC-API-FINALIZATION`: public API posture and split services stabilized.

From an external app point of view, this means Pinocchio now has a layered API, and older root-level symbols are intentionally no longer exported from `pkg/webchat`.

### 1.2 Architectural model in one paragraph

`cmd/web-chat` now follows a handler-first app composition model: the app creates a `webchat.Server`, injects an app-owned runtime composer, defines an app-owned request resolver policy, mounts app-owned `/chat` and `/ws` handlers, mounts `/api/timeline`, and optionally mounts utility handlers (`srv.APIHandler()` and `srv.UIHandler()`). The core webchat package owns lifecycle and runtime mechanics; the app owns transport policy and runtime selection policy.

### 1.3 Package boundaries: the new API map

The clean mental model for a new developer is:

- `pinocchio/pkg/inference/runtime`
  - Runtime composition contracts and runtime utility composition helpers.
- `pinocchio/pkg/webchat` (root)
  - Server, Router, ChatService, StreamHub, TimelineService, lifecycle, persistence wiring, conversation manager.
- `pinocchio/pkg/webchat/http` (package name `webhttp`)
  - HTTP boundary contracts for request resolution and helper HTTP handlers.
- `pinocchio/cmd/web-chat`
  - Reference app wiring and app-owned policy (profiles, resolver, runtime composer defaults/override rules).

A concise signature-level view:

```go
// pkg/inference/runtime/composer.go
type RuntimeComposeRequest struct {
    ConvID     string
    RuntimeKey string
    Overrides  map[string]any
}

type RuntimeArtifacts struct {
    Engine             engine.Engine
    Sink               events.EventSink
    RuntimeFingerprint string
    RuntimeKey         string
    SeedSystemPrompt   string
    AllowedTools       []string
}

type RuntimeComposer interface {
    Compose(ctx context.Context, req RuntimeComposeRequest) (RuntimeArtifacts, error)
}
```

```go
// pkg/webchat/http/api.go
type ConversationRequestPlan struct {
    ConvID         string
    RuntimeKey     string
    Overrides      map[string]any
    Prompt         string
    IdempotencyKey string
}

type ConversationRequestResolver interface {
    Resolve(req *http.Request) (ConversationRequestPlan, error)
}

type RequestResolutionError struct {
    Status    int
    ClientMsg string
    Err       error
}
```

```go
// pkg/webchat/http/api.go
func NewChatHandler(svc ChatService, resolver ConversationRequestResolver) http.HandlerFunc
func NewWSHandler(svc StreamService, resolver ConversationRequestResolver, upgrader websocket.Upgrader) http.HandlerFunc
func NewTimelineHandler(svc TimelineService, logger zerolog.Logger) http.HandlerFunc
```

These three blocks are the core of why `web-agent-example` currently fails: it is still reading these contracts from the wrong package.

### 1.4 Detailed boot and wiring flow in `cmd/web-chat`

`cmd/web-chat/main.go` executes a clear startup sequence:

1. Build CLI sections/flags and parse values.
2. Construct optional middleware services (agent mode/sqlite tool middleware contexts).
3. Build an app profile registry and app request resolver (`newWebChatProfileResolver`).
4. Build runtime composer (`newWebChatRuntimeComposer`) implementing `infruntime.RuntimeComposer`.
5. Create server with `webchat.NewServer(...)` and `webchat.WithRuntimeComposer(runtimeComposer)`.
6. Register middleware factories and tools on server.
7. Create app-owned handlers via `webhttp`:
   - `webhttp.NewChatHandler(srv.ChatService(), requestResolver)`
   - `webhttp.NewWSHandler(srv.StreamHub(), requestResolver, upgrader)`
   - `webhttp.NewTimelineHandler(srv.TimelineService(), logger)`
8. Mount routes on an app mux and optionally mount root prefix.
9. Run server lifecycle with `srv.Run(ctx)`.

This is the pattern `web-agent-example` should mirror.

### 1.5 Runtime composition path: where policy lives now

`cmd/web-chat/runtime_composer.go` is now the reference for where runtime policy belongs:

- Input: `infruntime.RuntimeComposeRequest`
- Output: `infruntime.RuntimeArtifacts`
- Responsibilities:
  - validate override schema
  - merge defaults and overrides (system prompt, middleware list, tool allowlist)
  - convert parsed CLI/provider config into `settings.StepSettings`
  - call `infruntime.ComposeEngineFromSettings(...)`
  - generate deterministic `RuntimeFingerprint`

This keeps core webchat generic. The core only compares fingerprints and rebuilds when needed; it does not own app prompt/middleware policy.

Pseudo-flow of composer:

```go
func Compose(ctx, req):
  validate(req.Overrides)
  runtimeKey = defaultIfEmpty(req.RuntimeKey, "default")
  systemPrompt, middlewares, tools = mergeDefaultsAndOverrides(req.Overrides)
  stepSettings = settings.NewStepSettingsFromParsedValues(parsed)
  engine = infruntime.ComposeEngineFromSettings(ctx, stepSettings.Clone(), systemPrompt, middlewares, mwFactories)
  return RuntimeArtifacts{
    Engine: engine,
    RuntimeKey: runtimeKey,
    RuntimeFingerprint: hash(runtimeKey, systemPrompt, middlewares, tools, stepSettings.metadata),
    SeedSystemPrompt: systemPrompt,
    AllowedTools: tools,
  }
```

### 1.6 Request resolution path: where transport/runtime policy lives now

`cmd/web-chat/profile_policy.go` demonstrates the policy split:

- `Resolve(req)` branches by method (`GET` websocket attach vs `POST` chat submit).
- WS path requires `conv_id`; no prompt is required.
- Chat path parses JSON body, supports `text` alias for prompt, creates `conv_id` if missing.
- Runtime/profile selection is app-owned and can inspect path/query/cookie.
- Override merging is app-owned (including allow/deny policy).
- Typed client errors are surfaced through `webhttp.RequestResolutionError`.

Pseudo-flow:

```go
Resolve(req):
  if method == GET: return resolveWS(req)
  if method == POST: return resolveChat(req)
  return RequestResolutionError(405)

resolveWS(req):
  convID = required(query.conv_id)
  runtime, profile = resolveProfileFrom(path/query/cookie/default)
  return Plan{ConvID: convID, RuntimeKey: runtime, Overrides: defaults(profile)}

resolveChat(req):
  body = decodeJSON(ChatRequestBody)
  convID = body.conv_id or uuid()
  runtime, profile = resolveProfileFrom(path/query/cookie/default)
  overrides = merge(profileDefaults, body.overrides, allowOverridePolicy)
  return Plan{ConvID, RuntimeKey: runtime, Overrides: overrides, Prompt: body.prompt or body.text}
```

### 1.7 Core webchat internals that app code depends on

Even though app code should not use internals directly, new developers need to understand how handlers map into core services.

#### 1.7.1 `ChatService`

`ChatService.SubmitPrompt(...)` does:

- resolve/create conversation via `ResolveAndEnsureConversation`
- enforce prompt validation
- enforce idempotency/queue semantics
- start inference (or queue if conversation busy)
- return standardized status payload (`queued`, `running`, `started`, etc.)

#### 1.7.2 `StreamHub`

`StreamHub` owns websocket and conversation-stream attachment:

- `ResolveAndEnsureConversation(...)` normalizes `conv_id` and `runtime_key`, then asks `ConvManager.GetOrCreate`.
- `AttachWebSocket(...)` adds connection, sends optional `ws.hello`, handles ping/pong, and read-loop lifecycle.

#### 1.7.3 `ConvManager`

`ConvManager.GetOrCreate(convID, runtimeKey, overrides)`:

- calls `runtimeComposer.Compose(...)`
- compares `RuntimeFingerprint`
- reuses or rebuilds engine/sink/subscriber/stream when fingerprint changed
- initializes `Session`, `ConnectionPool`, and `StreamCoordinator`

This is the core invariant behind runtime hot-swap behavior.

### 1.8 End-to-end runtime sequence diagrams

#### 1.8.1 Chat submit sequence

```text
Browser POST /chat
  -> webhttp.NewChatHandler
    -> resolver.Resolve(req) -> ConversationRequestPlan
    -> ChatService.SubmitPrompt(plan)
      -> StreamHub.ResolveAndEnsureConversation
        -> ConvManager.GetOrCreate
          -> RuntimeComposer.Compose
      -> Conversation.PrepareSessionInference (idempotency + queue)
      -> startInferenceForPrompt
        -> publish user chat.message SEM event
        -> geppetto inference loop emits events
        -> StreamCoordinator translates/publishes SEM frames
        -> ConnectionPool broadcasts WS frames
        -> TimelineProjector persists snapshots
  <- HTTP response {status, conv_id, session_id, ...}
```

#### 1.8.2 WebSocket attach sequence

```text
Browser GET /ws?conv_id=...
  -> webhttp.NewWSHandler
    -> resolver.Resolve(req)
    -> StreamHub.ResolveAndEnsureConversation
      -> ConvManager.GetOrCreate
    -> StreamHub.AttachWebSocket
      -> ConvManager.AddConn
      -> optional ws.hello frame
      -> read loop + optional ping/pong
```

### 1.9 Why extracted packages exist: practical reasoning

The extraction to `pkg/inference/runtime` and `pkg/webchat/http` is not only code style. It prevents a common failure mode where all semantics collect in `pkg/webchat` root and external apps depend on unstable internals. The current design isolates:

- runtime composition policy (`inference/runtime`) from
- web transport policy (`webchat/http`) from
- lifecycle services (`webchat` root).

As a result, external apps can compose handlers without importing internal lifecycle types that may move.

### 1.10 Relevant documentation and how to read it safely

For new developers, the useful documentation order is:

1. `cmd/web-chat/README.md`
   - Best quick reference for current reference app shape.
2. `pkg/doc/topics/webchat-http-chat-setup.md`
   - Endpoint contracts and resolver rules.
3. `pkg/doc/topics/webchat-framework-guide.md`
   - Bigger-picture backend composition.
4. `pkg/doc/tutorials/03-thirdparty-webchat-playbook.md`
   - Third-party app embedding pattern.
5. Recent ticket docs:
   - `ttmp/.../GP-022` (profile policy decoupling)
   - `ttmp/.../GP-023` (runtime composer extraction)
   - `ttmp/.../GP-025` (app route ownership)
   - `ttmp/.../GP-026-WEBCHAT-CORE-EXTRACTIONS` (type moves)
   - `ttmp/.../GP-026-WEBCHAT-PUBLIC-API-FINALIZATION` (public API posture)

Important caveat for this migration: several docs still use helper names like `webchat.NewChatHTTPHandler` / `webchat.NewWSHTTPHandler`. The current codebase exports `webhttp.NewChatHandler` / `webhttp.NewWSHandler` / `webhttp.NewTimelineHandler` from `pkg/webchat/http`. Use source code as the final authority when docs and package exports diverge.

### 1.11 API and ownership cheat sheet for new contributors

```text
You are writing app policy?            -> cmd/<app>/*.go
You are composing runtime engine?      -> pkg/inference/runtime + app composer
You are parsing HTTP/WS request rules? -> pkg/webchat/http contracts + app resolver
You are mounting routes?               -> app mux in cmd/<app>/main.go
You need core lifecycle services?      -> pkg/webchat Server/ChatService/StreamHub/TimelineService
```

### 1.12 Concrete “old vs new” symbol map (high-value subset)

| Old usage in external app | New usage |
|---|---|
| `webchat.RuntimeComposeRequest` | `infruntime.RuntimeComposeRequest` |
| `webchat.RuntimeArtifacts` | `infruntime.RuntimeArtifacts` |
| `webchat.MiddlewareFactory` | `infruntime.MiddlewareFactory` |
| `webchat.MiddlewareUse` | `infruntime.MiddlewareUse` |
| `webchat.ComposeEngineFromSettings` | `infruntime.ComposeEngineFromSettings` |
| `webchat.ConversationRequestPlan` | `webhttp.ConversationRequestPlan` |
| `webchat.RequestResolutionError` | `webhttp.RequestResolutionError` |
| `webchat.ChatRequestBody` | `webhttp.ChatRequestBody` |
| `webchat.NewChatHTTPHandler` | `webhttp.NewChatHandler` |
| `webchat.NewWSHTTPHandler` | `webhttp.NewWSHandler` |
| `webchat.NewTimelineHTTPHandler` | `webhttp.NewTimelineHandler` |

This table alone explains the majority of compile failures in `web-agent-example`.

### 1.13 Deep dive: server and router lifecycle (what is created when)

For a new contributor, one of the most important non-obvious facts is that `webchat.NewServer(...)` is not only creating an HTTP server. It also wires event transport, persistence backends, conversation lifecycle, and service facades. Most migration confusion comes from assuming “server” means only HTTP.

At creation time (`pkg/webchat/server.go` -> `pkg/webchat/router.go`), the flow is roughly:

1. Parse routing/storage settings from glazed values.
2. Build `StreamBackend` (`in-memory` or `redis`) and get event router.
3. Create timeline store (`sqlite` if configured, memory otherwise).
4. Create optional turn store (`sqlite` if configured).
5. Apply options (`WithRuntimeComposer`, `WithTimelineStore`, etc.).
6. Construct `ConvManager` with runtime composer + subscriber builder.
7. Construct `ConversationService` with `ConvManager` and persistence/publisher dependencies.
8. Expose split services:
   - `ChatService`
   - `StreamHub`
   - `TimelineService`
9. Register utility handlers (UI + API utilities).

This means `WithRuntimeComposer(...)` is effectively a required dependency for live runtime composition. The core now fails fast when composer is missing, which is architecturally useful: there is no hidden default policy in core.

Pseudo-lifecycle (constructor path):

```text
NewServer(ctx, parsed, fs, opts...)
  -> NewRouter(...)
     -> NewStreamBackendFromValues(...)
     -> open timeline/turn stores
     -> apply options
     -> require runtime composer
     -> build ConvManager
     -> build ConversationService
     -> derive ChatService + StreamHub + TimelineService
     -> register UI/API handlers
  -> BuildHTTPServer()
```

The consequence for external packages is practical: if you instantiate `webchat.Server`, you do not need to manually build `ConvManager` or stream subscribers. But you do need to supply runtime composition and request policy at app level.

### 1.14 Deep dive: conversation and runtime rebuild semantics

`ConvManager.GetOrCreate(...)` is the runtime identity gate. Understanding this function avoids many migration mistakes because runtime-request data (`runtime_key`, `overrides`) does not directly trigger rebuild; rebuild is controlled by `RuntimeFingerprint`.

Key logic:

1. Build request: `{ConvID, RuntimeKey, Overrides}`.
2. Call app runtime composer.
3. Validate artifacts (`Engine` and `Sink` must not be nil).
4. Normalize missing runtime metadata.
5. If conversation exists:
   - compare existing fingerprint vs new fingerprint
   - if changed: rebuild runtime and stream/subscriber attachments
   - if same: reuse existing runtime/session
6. If conversation does not exist:
   - create conversation + session + stream + connection pool
   - seed turn with system prompt from artifacts

Minimal signature and shape:

```go
// pkg/webchat/conversation.go
func (cm *ConvManager) GetOrCreate(convID, runtimeKey string, overrides map[string]any) (*Conversation, error)

type Conversation struct {
    ID                 string
    SessionID          string
    RuntimeKey         string
    RuntimeFingerprint string
    SeedSystemPrompt   string
    AllowedTools       []string
    // plus engine/session/stream/pool state
}
```

From an app-design perspective, this is the critical invariant:

- `RuntimeKey` is label/debug identity.
- `RuntimeFingerprint` is rebuild identity.

If external code sets `RuntimeKey` but does not evolve fingerprint, conversations may not rebuild even when expected. This is why composer implementations should hash all meaningful runtime dimensions (prompt defaults, middleware config, tool allowlist, provider metadata).

### 1.15 Deep dive: ChatService behavior and queue/idempotency contract

`ChatService` is intentionally thin over `ConversationService`, but the behavior it exposes is rich and should be treated as a contract by external apps.

Submit flow key points:

1. Input validation:
   - empty prompt -> `400` response with error payload.
2. Resolve conversation using `ConvID`/`RuntimeKey`/`Overrides`.
3. Generate idempotency key if missing.
4. Call `PrepareSessionInference(...)` on conversation:
   - if duplicate idempotency key seen -> return cached response
   - if conversation busy -> enqueue request and return `202` queued metadata
   - else -> mark running and start inference
5. Start inference loop and return start metadata.
6. Completion path updates internal request record and drains queue.

Contract shape:

```go
type SubmitPromptInput struct {
    ConvID         string
    RuntimeKey     string
    Overrides      map[string]any
    Prompt         string
    IdempotencyKey string
}

type SubmitPromptResult struct {
    HTTPStatus int
    Response   map[string]any
}
```

A subtle but important recent behavior is user-message projection path. Instead of direct timeline writes in chat path, chat service now publishes `chat.message` SEM envelopes that flow through stream/timeline projection, preserving one projection model.

Practical migration implication: external apps should not bypass chat service with ad-hoc direct session starts unless they are prepared to reproduce idempotency/queue/projection semantics.

### 1.16 Deep dive: StreamHub behavior, WS protocol surface, and operational semantics

`StreamHub` is the dedicated boundary for websocket lifecycle. This separation is exactly why `/ws` route ownership can remain app-level while stream mechanics remain in core.

Core behaviors:

1. Resolve or create conversation for a WS attach.
2. Add connection to connection pool.
3. Send optional `ws.hello` SEM frame with `conv_id`, `runtime_key`, and server time.
4. Maintain read loop:
   - detect text `ping` or semantic ping envelopes
   - emit semantic `ws.pong` replies
5. Remove connection on disconnect and allow idle timeout logic to stop stream reader.

Signature surface:

```go
type WebSocketAttachOptions struct {
    SendHello      bool
    HandlePingPong bool
}

func (h *StreamHub) ResolveAndEnsureConversation(ctx context.Context, req AppConversationRequest) (*ConversationHandle, error)
func (h *StreamHub) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts WebSocketAttachOptions) error
```

Operational note for developers: if a conversation has no active sockets, idle policies can stop stream readers; reconnection restarts as needed. This behavior interacts with backend choice (in-memory/redis) and eviction settings parsed from CLI flags.

### 1.17 Deep dive: timeline projection and custom entity ecosystem

For teams using custom timeline cards (as in `web-agent-example`), the timeline projection path is as important as chat/ws routes.

Projection model:

1. StreamCoordinator yields SEM frames with sequence metadata.
2. TimelineProjector consumes SEM frames.
3. Built-in handlers map core SEM event types (`llm.*`, `tool.*`, `chat.message`) into timeline entities.
4. Custom handlers can be registered by event type through `RegisterTimelineHandler(...)`.

Custom extension contracts:

```go
type TimelineSemEvent struct {
    Type     string
    ID       string
    Seq      uint64
    StreamID string
    Data     json.RawMessage
}

type TimelineSemHandler func(ctx context.Context, p *TimelineProjector, ev TimelineSemEvent, now int64) error

func RegisterTimelineHandler(eventType string, handler TimelineSemHandler)
```

This is why `web-agent-example/pkg/thinkingmode` and `web-agent-example/pkg/discodialogue` do not fundamentally need migration for this ticket. They already depend on timeline-handler APIs that remain rooted in `pkg/webchat`.

Conceptual pipeline diagram:

```text
Engine events + app SEM publishes
  -> stream backend topic
  -> StreamCoordinator consume
  -> SEM frames with seq
  -> ConnectionPool broadcast to websocket clients
  -> TimelineProjector.ApplySemFrame
  -> built-in/custom timeline handlers
  -> TimelineStore.Upsert
  -> /api/timeline hydration snapshots
```

### 1.18 Deep dive: extracted HTTP boundary package and why resolver type placement matters

Moving `ConversationRequestPlan` and related types into `pkg/webchat/http` is a strong architectural signal: request parsing and transport-layer policy are treated as HTTP boundary concerns, not core runtime lifecycle concerns.

In practical terms, external apps should now treat resolver logic as part of the HTTP adapter layer. This makes it easier to keep core services transport-agnostic and to reuse policy logic consistently between `/chat` and `/ws`.

Pattern summary:

```go
// boundary layer
plan, err := resolver.Resolve(req) // webhttp.ConversationRequestPlan

// service layer
chatSvc.SubmitPrompt(ctx, webchat.SubmitPromptInput{...plan fields...})
streamHub.AttachWebSocket(ctx, plan.ConvID, conn, opts)
```

The old root placement made it easy for consumers to blur boundaries and import everything from one package. The new placement is stricter but healthier for long-term API stability.

### 1.19 Documentation reliability matrix (for onboarding and migration execution)

Because this migration spans active refactor documentation, it helps to explicitly categorize documentation by trust level relative to source code.

| Source | Reliability for symbol names | Reliability for architecture | Usage guidance |
|---|---|---|---|
| `cmd/web-chat/*.go` | High | High | Treat as executable reference |
| `pkg/inference/runtime/*.go` | High | High | Source of truth for runtime compose contracts |
| `pkg/webchat/http/api.go` | High | High | Source of truth for HTTP boundary contracts |
| `pkg/doc/topics/webchat-http-chat-setup.md` | Medium | High | Good contract narrative, verify constructor names in code |
| `pkg/doc/topics/webchat-framework-guide.md` | Medium | High | Good conceptual model, validate snippets against exports |
| `pkg/doc/tutorials/03-thirdparty-webchat-playbook.md` | Medium | Medium/High | Good embedding flow, validate helper names |
| `ttmp/GP-026-WEBCHAT-CORE-EXTRACTIONS/changelog.md` | High | High | Good change chronology and intent |

Recommended onboarding practice for new contributors:

1. Read docs for conceptual overview.
2. Immediately reconcile import paths/signatures against current source files.
3. Build migration map from source, not from prose snippets.

### 1.20 New-developer implementation checklist based on current architecture

If a developer new to this codebase needs to implement or migrate an app today, this checklist avoids the common mistakes:

1. Runtime composer:
   - implement with `infruntime.RuntimeComposer`
   - return deterministic `RuntimeFingerprint`
2. Request resolver:
   - implement with `webhttp.ConversationRequestResolver`
   - return `webhttp.RequestResolutionError` for policy failures
3. HTTP handlers:
   - use `webhttp.NewChatHandler`, `webhttp.NewWSHandler`, `webhttp.NewTimelineHandler`
4. Server assembly:
   - use `webchat.NewServer(..., webchat.WithRuntimeComposer(...))`
   - mount app-owned routes explicitly
5. Extension hooks:
   - register middleware/tool factories on server
   - use timeline handlers for custom entity projections
6. Verification:
   - run package tests + live route smoke tests for chat/ws/timeline

---

## Section 2: How to Refactor/Rewrite `web-agent-example` as an External Package Consumer

### 2.1 Migration objective

`web-agent-example` should be a clean third-party embedding example, not a privileged sibling relying on root `pkg/webchat` symbols that were intentionally extracted. The migration objective is to make it compile and remain aligned with Pinocchio’s public layering model.

### 2.2 Current gap analysis (file-by-file)

#### `cmd/web-agent-example/runtime_composer.go`

Current problems:

- uses old types from `webchat` root:
  - `webchat.MiddlewareFactory`
  - `webchat.RuntimeComposeRequest`
  - `webchat.RuntimeArtifacts`
  - `webchat.MiddlewareUse`
- calls old helper path:
  - `webchat.ComposeEngineFromSettings`

Migration:

- import `infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"`
- switch all runtime and middleware types to `infruntime.*`
- call `infruntime.ComposeEngineFromSettings(...)`

Reference implementation is almost line-for-line available in `pinocchio/cmd/web-chat/runtime_composer.go`.

#### `cmd/web-agent-example/engine_from_req.go`

Current problems:

- `Resolve` returns `webchat.ConversationRequestPlan` (moved)
- uses `webchat.RequestResolutionError` (moved)
- decodes `webchat.ChatRequestBody` (moved)

Migration:

- import `webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"`
- update types to `webhttp.ConversationRequestPlan`, `webhttp.RequestResolutionError`, `webhttp.ChatRequestBody`

#### `cmd/web-agent-example/sink_wrapper.go`

Current problems:

- closure signature expects `webchat.RuntimeComposeRequest` (moved)

Migration:

- import `infruntime`
- update closure signature to use `infruntime.RuntimeComposeRequest`
- keep `webchat.EventSinkWrapper` return type (that one is still root-owned via `types.go`)

#### `cmd/web-agent-example/main.go`

Current problems:

- middleware factory map uses old root type
- handler constructors referenced from old root names

Migration:

- middleware map type: `map[string]infruntime.MiddlewareFactory`
- switch handler creation to `webhttp` package:
  - `chatHandler := webhttp.NewChatHandler(...)`
  - `wsHandler := webhttp.NewWSHandler(...)`
  - `timelineHandler := webhttp.NewTimelineHandler(...)`

#### Tests needing synchronized updates

- `app_owned_routes_integration_test.go`
  - runtime composer test func should use `infruntime.RuntimeComposerFunc` and request/artifact types from `infruntime`
  - handlers should come from `webhttp`
- `engine_from_req_test.go`
  - error/assertion types should move from `webchat.RequestResolutionError` to `webhttp.RequestResolutionError`
- `sink_wrapper_test.go`
  - runtime request type must move to `infruntime.RuntimeComposeRequest`

### 2.3 Recommended migration strategy: refactor, not rewrite

A full rewrite is not necessary. `web-agent-example` already follows the right architecture (app-owned routes, resolver, runtime composer). The failure is largely symbol movement. Therefore:

- keep behavior and structure
- update imports/types/functions in place
- run tests and then refresh docs/comments

This minimizes risk and preserves existing custom middleware examples.

### 2.4 Suggested implementation sequence

1. `runtime_composer.go` type migration first (largest compile blocker).
2. `engine_from_req.go` and `sink_wrapper.go` migration.
3. `main.go` handler/factory migration.
4. Update tests to new packages.
5. `go test ./...` in `web-agent-example`.
6. Update README and any stale references in `ttmp` docs.

### 2.5 Pseudocode: target-state `main.go`

```go
import (
  infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
  webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
  webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
)

func run(...) error {
  middlewareFactories := map[string]infruntime.MiddlewareFactory{...}
  runtimeComposer := newWebAgentRuntimeComposer(parsed, middlewareFactories)
  resolver := newNoCookieRequestResolver() // returns webhttp.ConversationRequestPlan

  srv, err := webchat.NewServer(ctx, parsed, staticFS,
    webchat.WithRuntimeComposer(runtimeComposer),
    webchat.WithEventSinkWrapper(discoSinkWrapper()),
  )

  for name, f := range middlewareFactories { srv.RegisterMiddleware(name, f) }

  mux := http.NewServeMux()
  mux.HandleFunc("/chat", webhttp.NewChatHandler(srv.ChatService(), resolver))
  mux.HandleFunc("/ws", webhttp.NewWSHandler(srv.StreamHub(), resolver, upgrader))
  mux.HandleFunc("/api/timeline", webhttp.NewTimelineHandler(srv.TimelineService(), logger))
  mux.Handle("/api/", srv.APIHandler())
  mux.Handle("/", srv.UIHandler())

  srv.HTTPServer().Handler = mux
  return srv.Run(ctx)
}
```

### 2.6 Pseudocode: target-state resolver

```go
type noCookieRequestResolver struct{}

func (r *noCookieRequestResolver) Resolve(req *http.Request) (webhttp.ConversationRequestPlan, error) {
  switch req.Method {
    case http.MethodGet:  return r.fromWS(req)
    case http.MethodPost: return r.fromChat(req)
    default:
      return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{Status: 405, ClientMsg: "method not allowed"}
  }
}
```

### 2.7 Validation matrix after migration

Minimum backend validation commands:

```bash
cd web-agent-example
go test ./... -count=1
go run ./cmd/web-agent-example serve --addr :8080 --log-level debug
```

Manual smoke checks:

1. `POST /chat` returns `started` or `queued` and includes `conv_id`.
2. `GET /ws?conv_id=<id>` returns `ws.hello`.
3. `GET /api/timeline?conv_id=<id>` returns snapshot JSON.
4. Existing custom timeline entities (`thinkingmode`, `discodialogue`) still appear in stream and hydration paths.

### 2.8 Risk analysis and mitigations

Risk: docs in `pkg/doc/topics` still reference helper names not currently exported from root.

- Mitigation: treat code exports as source of truth; update docs after code migration.

Risk: test updates may accidentally hide type drift by widening interfaces.

- Mitigation: keep tests strict on concrete types (`webhttp.RequestResolutionError`, `infruntime.RuntimeComposeRequest`).

Risk: third-party consumers copy outdated snippets from old docs.

- Mitigation: update tutorial/playbook snippets to explicitly import `webhttp` and `infruntime`.

### 2.9 Definition of done for WAE-001

`WAE-001` can be considered complete when all of the following are true:

1. `web-agent-example` compiles and tests pass with no references to removed runtime/request symbols in `pkg/webchat` root.
2. `main.go`, resolver, runtime composer, sink wrapper, and tests all use extracted packages consistently.
3. Route behavior remains app-owned and identical to reference app expectations.
4. Documentation snippets used by external consumers reflect actual exported symbol locations.

### 2.10 Optional hardening improvements after migration

These are optional follow-ups after compile recovery:

- Prefer importing subpackage facades where appropriate:
  - `pkg/webchat/bootstrap`, `pkg/webchat/http`, `pkg/webchat/chat`, `pkg/webchat/stream`, `pkg/webchat/timeline`
- Add a tiny compile-time “API surface contract” test to catch future symbol moves quickly.
- Add a short `MIGRATING.md` in `web-agent-example` summarizing old-to-new symbol mapping.

---

## Final Recommendation

Use `cmd/web-chat` as implementation reference, but use package boundaries as the authority:

- runtime contracts from `pkg/inference/runtime`
- HTTP resolver/handlers from `pkg/webchat/http`
- lifecycle services from `pkg/webchat`

`web-agent-example` does not need a conceptual rewrite. It needs a precise package-boundary migration to the extracted API surfaces. Once this is done, it will again be a valid external embedding example for the new Pinocchio architecture.
