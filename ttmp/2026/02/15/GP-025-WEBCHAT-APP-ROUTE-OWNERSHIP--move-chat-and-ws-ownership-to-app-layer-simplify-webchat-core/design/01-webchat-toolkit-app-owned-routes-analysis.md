---
Title: 'Webchat Toolkit Refactor: App-Owned /chat and /ws, Core as Reusable Primitives'
Ticket: GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP
Status: active
Topics:
    - architecture
    - webchat
    - refactor
    - api
    - routing
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/webchat/router.go
      Note: current all-in-one HTTP/router orchestration
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: conversation lifecycle, engine/session creation, stream callback wiring
    - Path: pinocchio/pkg/webchat/runtime_composer.go
      Note: runtime composition seam
    - Path: pinocchio/pkg/webchat/engine_from_req.go
      Note: request resolution and plan contract
    - Path: pinocchio/pkg/webchat/router_options.go
      Note: optional hook surface and interface proliferation
    - Path: pinocchio/pkg/webchat/timeline_upsert.go
      Note: websocket emission currently tied to ConnectionPool access
Summary: Detailed analysis and design for removing route ownership from core webchat and replacing the monolithic router with a simpler toolkit that end applications compose explicitly.
LastUpdated: 2026-02-15T01:25:00-05:00
WhatFor: Reduce conceptual and implementation complexity in webchat by moving /chat and /ws handlers to app code and shrinking core abstractions.
WhenToUse: Use when implementing or reviewing the post-router webchat architecture and cutover strategy.
---

# Webchat Toolkit Refactor: App-Owned `/chat` and `/ws`, Core as Reusable Primitives

## 1. Executive Summary
The current `pkg/webchat` architecture provides a convenient integrated router, but that convenience comes from centralizing many responsibilities in one place: static UI serving, route registration, request resolution, runtime composition, conversation lifecycle, websocket fanout, timeline hydration, debug APIs, and optional extension hooks. Over time, this has created a surface area that is powerful yet difficult to reason about. A large number of interfaces and options now exist primarily to make core behavior overridable from applications.

This document proposes a clean inversion: **make applications own `/chat` and `/ws` routing directly**, and reduce `pkg/webchat` to a focused toolkit of conversation/runtime primitives. The package remains reusable and feature-complete, but stops prescribing HTTP shape. This lowers abstraction pressure, simplifies the type graph, and makes application behavior explicit where it belongs.

The design intentionally favors clarity over maximal configurability. It removes the need to encode app policy through progressively more complex core interfaces, and replaces it with straightforward app-level assembly.

## 2. Broader Context and Why This Matters
There are two competing design instincts in reusable frameworks:

1. Offer batteries-included orchestration that hides wiring.
2. Offer low-level, composable primitives and let apps wire explicitly.

`pkg/webchat` started near (1), then gradually moved toward (2) by adding overridable seams (`RuntimeComposer`, `ConversationRequestResolver`, hook options, wrappers). This hybrid state creates a common cost: users still interact with a centralized router abstraction, but must understand many advanced hooks to express non-default behavior. The resulting cognitive load is higher than either pure model.

A simpler architecture should align ownership boundaries with decision boundaries:

1. App owns HTTP contract and route policy.
2. Core owns conversation/runtime state machine and streaming mechanics.
3. Extensions publish through stable core services, not transport internals.

## 3. Current-State Analysis
### 3.1 What `router.go` does today
`router.go` currently performs multiple roles:

1. Builds and configures event router infrastructure.
2. Initializes timeline/turn stores.
3. Installs static UI handlers.
4. Installs API handlers (chat/ws/timeline/debug/offline).
5. Owns websocket handshake and connection lifecycle.
6. Resolves request policy and runtime composition path.
7. Boots and coordinates conversation manager lifecycle.

This is convenient for “one command runs everything” use, but it couples app-specific concerns (route shape, profile policy, auth, query semantics) to core package behavior.

### 3.2 Why interfaces multiplied
As new app requirements arrived, they were frequently solved by adding options/hooks around the router. This is technically clean in isolation, but accumulates conceptual debt:

1. Many seams exist, but each seam requires mental mapping to invocation timing.
2. Data contracts become indirect (policy through options rather than obvious app code).
3. Debugging route behavior requires reading both app setup and core defaults.

### 3.3 Lifecycle fact that influences design
In current creation flow (`conversation.go`):

1. Runtime is composed and engine is built first.
2. Conversation struct is initialized.
3. `ConnectionPool` is created after engine composition.

Implication: any feature requiring `ConnectionPool` cannot be injected directly during engine build for first-create. This is a strong signal to avoid pool-coupled abstractions in middleware construction and prefer conversation-scoped publisher services that resolve transport later.

## 4. Problem Statement
We need to reduce architectural complexity without losing capabilities.

The core issue is not feature count; it is ownership placement. If core owns app routes, app policy keeps leaking into core extension points. This encourages more interfaces and data structures to “parameterize” behavior that should be plain app code.

## 5. Design Goals and Non-Goals
### 5.1 Goals
1. Make `/chat` and `/ws` route handling app-owned.
2. Keep webchat core reusable as primitives.
3. Minimize new abstractions; remove more than we add.
4. Preserve elegant implementation for first-party webchat app.
5. Enable backend subsystems to emit websocket frames without direct `ConnectionPool` access.

### 5.2 Non-goals
1. Replacing SEM event format.
2. Rewriting stream coordinator internals.
3. Adding websocket channel filtering/profile protocol in this proposal.
4. Preserving old API shapes for backward compatibility.

### 5.3 Locked Decision: Clean Cutover, No Compatibility Adapter
This ticket commits to a **clean cutover**. We will not add a compatibility adapter layer that keeps router-owned `/chat` or `/ws` behavior available behind transitional shims.

Concretely:
1. App-owned route handlers are the target behavior.
2. Legacy route ownership in `pkg/webchat` is removed rather than wrapped.
3. Migration effort is paid once in application code (`cmd/web-chat`, `web-agent-example`) instead of in long-lived core adapters.

## 6. Proposed Architectural Direction
### 6.1 Replace monolithic router with a toolkit
Instead of “core owns routes and exposes options,” adopt “app owns routes and calls core services.”

#### Core package responsibilities
1. Conversation lifecycle (`GetOrCreate`, queue semantics, eviction).
2. Runtime/session orchestration.
3. Stream coordinator integration.
4. Timeline/turn persistence integration.
5. Websocket publish service (conversation-scoped; no pool exposure).

#### App responsibilities
1. HTTP route registration (`/chat`, `/ws`, `/api/timeline`, debug routes).
2. Request parsing/validation/auth policy.
3. Runtime-key/profile selection policy.
4. Request-to-runtime mapping.

### 6.2 Conceptual package map (target)

```text
pkg/webchat/
  core/
    conversation manager
    conversation model
    stream coordinator wiring
    queue/idempotency logic
  transport/
    websocket connection pool (internal)
    ws publisher service (public interface)
  persistence/
    timeline + turn integration helpers
  handlers/ (optional, thin)
    reusable helper funcs (not owning mux)
```

Important: `handlers/` can be optional helpers but should not recreate a new god-router.

## 7. Minimal Public Surface (Proposed)
A minimal API can replace most current router indirection.

### 7.0 Frozen Cutover Surface (Locked)
For GP-025 implementation, the core cutover API is frozen to the `ConversationService` + `WSPublisher` surface below. New abstractions should not be added unless required to preserve correctness.

Frozen entries:
1. `ConversationService` constructor/config object.
2. `ConversationService.ResolveAndEnsureConversation(...)`.
3. `ConversationService.SubmitPrompt(...)`.
4. `ConversationService.AttachWebSocket(...)`.
5. `WSPublisher.PublishJSON(...)`.

```go
// App composes this from its own policy objects.
type ConversationService struct {
    // wraps ConvManager + runtime compose callback + persistence dependencies
}

// App route calls this directly.
func (s *ConversationService) ResolveAndEnsureConversation(ctx context.Context, req AppConversationRequest) (*ConversationHandle, error)

// App route calls this for prompt submission.
func (s *ConversationService) SubmitPrompt(ctx context.Context, in SubmitPromptInput) (SubmitPromptResult, error)

// App route calls this for websocket attach.
func (s *ConversationService) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn) error

// App and middleware can publish without touching ConnectionPool.
type WSPublisher interface {
    PublishJSON(ctx context.Context, convID string, envelope map[string]any) error
}
```

### 7.1 Frozen Constructor + Config Contract
Cutover constructor contract (locked for GP-025):

```go
type ConversationServiceConfig struct {
    BaseCtx            context.Context
    StepController     *toolloop.StepController
    RuntimeComposer    RuntimeComposer
    BuildSubscriber    func(convID string) (message.Subscriber, bool, error)
    TimelineStore      chatstore.TimelineStore
    TurnStore          chatstore.TurnStore
    TimelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)
    ToolRegistry       map[string]ToolFactory
    IdleTimeoutSec     int
    EvictIdle          time.Duration
    EvictInterval      time.Duration
}

func NewConversationService(cfg ConversationServiceConfig) (*ConversationService, error)
```

Behavior notes:
1. Constructor validates required dependencies (`BaseCtx`, `RuntimeComposer`, `BuildSubscriber`).
2. Constructor owns `ConvManager` initialization and persistence wiring.
3. Router/HTTP concerns are intentionally absent from config.

### 7.2 Frozen `ResolveAndEnsureConversation(...)` Contract

```go
type AppConversationRequest struct {
    ConvID     string
    RuntimeKey string
    Overrides  map[string]any
}

type ConversationHandle struct {
    ConvID             string
    SessionID          string
    RuntimeKey         string
    RuntimeFingerprint string
    SeedSystemPrompt   string
    AllowedTools       []string
}

func (s *ConversationService) ResolveAndEnsureConversation(
    ctx context.Context,
    req AppConversationRequest,
) (*ConversationHandle, error)
```

Behavior notes:
1. Empty `ConvID` is normalized to a generated conversation id.
2. Empty `RuntimeKey` is normalized to `"default"` unless app policy already resolved a value.
3. Method ensures the conversation exists and runtime composition is applied.
4. Returned handle is stable for downstream `SubmitPrompt` and `AttachWebSocket` calls.

The key simplification is that app code passes explicit values instead of registering many global options that interact indirectly.

## 8. Sequence Diagrams
### 8.1 `/chat` path (app-owned)

```text
Client
  |
  | POST /chat
  v
App Chat Handler ----------------------+
  | parse/auth/policy                  |
  | build AppConversationRequest       |
  v                                    |
ConversationService.ResolveAndEnsure   |
  |                                    |
  +--> runtime composer (app callback) |
  +--> conv manager GetOrCreate        |
  |                                    |
  v                                    |
ConversationService.SubmitPrompt       |
  +--> queue/idempotency               |
  +--> session append + run loop       |
  +--> stream emits SEM                |
  +--> ws publisher fanout             |
  |
  v
HTTP response
```

### 8.2 `/ws` path (app-owned)

```text
Client WS
  |
  | GET /ws?conv_id=...
  v
App WS Handler ----------------------+
  | parse/auth/policy                |
  | ensure conversation              |
  v                                  |
ConversationService.AttachWebSocket  |
  +--> connection pool add           |
  +--> ws hello send                 |
  +--> read loop ping/pong           |
  |
  v
Live SEM frame delivery
```

## 9. Simple Publisher Design (No Channels, No Filtering)
This proposal intentionally stays minimal:

1. One publisher service per conversation service.
2. Publish targets all websocket clients of a conversation.
3. No channel negotiation.
4. No per-client filtering.

```go
// ConversationService owns this.
type wsPublisher struct {
    cm *ConvManager
}

func (p *wsPublisher) PublishJSON(ctx context.Context, convID string, env map[string]any) error {
    if convID == "" {
        return errors.New("missing convID")
    }
    conv, ok := p.cm.GetConversation(convID)
    if !ok || conv == nil || conv.pool == nil {
        return ErrConversationNotReady
    }
    b, err := json.Marshal(env)
    if err != nil {
        return err
    }
    conv.pool.Broadcast(b)
    return nil
}
```

This is enough to decouple middleware/extensions from `ConnectionPool` while avoiding protocol complexity.

## 10. How Applications Build Engines in the New Model
Under app-owned routing, engine creation remains app-controlled through explicit callbacks.

### 10.1 Example assembly pseudocode

```go
runtimeComposer := func(ctx context.Context, req RuntimeComposeRequest) (RuntimeArtifacts, error) {
    // app policy in plain code
    profile := resolveProfile(req)

    eng := buildEngineFromStepSettings(ctx, profile, req.Overrides)

    return RuntimeArtifacts{
        Engine:             eng,
        RuntimeKey:         profile.Key,
        RuntimeFingerprint: fingerprint(profile, req.Overrides),
        SeedSystemPrompt:   profile.SystemPrompt,
        AllowedTools:       profile.Tools,
    }, nil
}

svc := webchat.NewConversationService(webchat.ConversationServiceConfig{
    BaseCtx:          ctx,
    RuntimeComposer:  runtimeComposer,
    BuildSubscriber:  buildSubscriber,
    TimelineStore:    timelineStore,
    TurnStore:        turnStore,
})

mux.HandleFunc("/chat", appChatHandler(svc, authz))
mux.HandleFunc("/ws", appWSHandler(svc, authz))
```

This removes implicit policy from core router options and makes behavior easy to audit.

## 11. Route Ownership and Complexity Reduction
### 11.1 What can be removed or shrunk
If route ownership moves to apps, we can simplify or remove:

1. Large parts of `router.go` that combine UI/static/API concerns.
2. Several route-level option hooks whose only purpose is HTTP policy indirection.
3. Debug route gating from core orchestration path.

### 11.2 Is full removal of `router.go` viable?
Yes, if we accept a clean cutover. A practical route is:

1. Introduce `ConversationService` + helper constructors.
2. Migrate first-party `cmd/web-chat` to app-owned handlers.
3. Delete or drastically reduce `router.go` to legacy shim (then remove).

## 12. Tradeoffs
### 12.1 Benefits
1. Lower conceptual load for contributors.
2. Fewer cross-cutting interfaces.
3. Clear app/core ownership boundary.
4. Easier custom auth and route policy.
5. Better testability of handlers as normal app code.

### 12.2 Costs
1. Apps must write and maintain handler glue.
2. Initial cutover touches multiple call paths.
3. Need disciplined examples/documentation to keep usage ergonomic.

## 13. Cutover Plan (No Backward Compatibility)
### Phase 1: Introduce service primitives
1. Add `ConversationService` and `WSPublisher` in core.
2. Route existing internal broadcast callsites through publisher API.

### Phase 2: Migrate first-party app handlers
1. Implement `/chat` and `/ws` directly in `cmd/web-chat`.
2. Move profile/runtime policy entirely to app handler + composer.

### Phase 3: Decommission router abstraction
1. Remove monolithic route registration path.
2. Keep only tiny helper utilities where needed.

### Phase 4: Documentation and examples
1. Update framework docs to app-owned handler assembly.
2. Provide minimal and advanced templates.

## 14. Testing Strategy
### 14.1 Core tests
1. Conversation lifecycle and queue semantics unchanged.
2. Publisher behavior (conversation exists/missing/closed).
3. Stream -> websocket fanout still works.

### 14.2 App-level tests
1. `/chat` request parsing and policy rules.
2. `/ws` connect/auth and attach behavior.
3. End-to-end prompt -> emitted websocket frames.

## 15. Open Questions to Resolve in Implementation Planning
1. Should helper handler builders exist at all, or should examples be the only guidance?
2. Do we keep any legacy path temporarily as a local branch-only migration aid?
3. What exact minimal public API for `ConversationService` avoids leaking internals while staying ergonomic?

## 16. Recommended Decision
Adopt the clean cutover to **app-owned `/chat` and `/ws` handlers** and refactor `pkg/webchat` into a toolkit-oriented core. Do not add new protocol complexity (no channels/filtering) during this refactor. Add only the minimum missing primitive: conversation-scoped websocket publishing without `ConnectionPool` exposure.

This strategy directly addresses the current complexity source: too much route/policy ownership in core plus too many extension seams compensating for that ownership.
