---
Title: Webchat Refactor Proposal Validation and Public API Release Plan
Ticket: GP-026-WEBCHAT-PUBLIC-API-FINALIZATION
Status: active
Topics:
    - api
    - architecture
    - webchat
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/main.go
      Note: Current app wiring and route ownership
    - Path: pkg/webchat/conversation_service.go
      Note: Current service coupling points and direct timeline writes
    - Path: pkg/webchat/router.go
      Note: Current ownership boundary and utility handlers
    - Path: pkg/webchat/stream_coordinator.go
      Note: Current stream ingestion and seq assignment behavior
    - Path: pkg/webchat/timeline_projector.go
      Note: Projection path and timeline semantics
    - Path: ttmp/2026/02/15/GP-026-WEBCHAT-PUBLIC-API-FINALIZATION--validate-webchat-refactor-proposal-and-finalize-public-api/sources/local/webchat-refactor-details.md
      Note: Expert mechanical mapping and commit sequencing input
    - Path: ttmp/2026/02/15/GP-026-WEBCHAT-PUBLIC-API-FINALIZATION--validate-webchat-refactor-proposal-and-finalize-public-api/sources/local/webchat-refactor.md
      Note: Imported proposal being validated
ExternalSources: []
Summary: In-depth evaluation of /tmp/webchat-refactor.md against current pinocchio webchat architecture with final API recommendations.
LastUpdated: 2026-02-15T12:53:55-05:00
WhatFor: Validate the imported webchat refactor proposal against current code and define a final releasable public API with an execution approach.
WhenToUse: Use when planning or implementing the next webchat architecture cut after GP-025.
---



# Webchat Refactor Proposal Validation and Public API Release Plan

## Goal

Provide an implementation-grade validation of `/tmp/webchat-refactor.md` against the current Pinocchio codebase, then define the final public API shape and rollout strategy that should be considered releasable for third-party app embedding.

## Context

The imported proposal describes a hard cut from a router-centric webchat package to a toolkit with explicit domain services:

- Streaming/realtime service
- Chat/inference service
- Timeline projection and hydration service
- Optional thin HTTP helpers

The current codebase already completed GP-025 app-route ownership for `/chat` and `/ws`, but still retains internal coupling between conversation lifecycle, stream consumption, timeline projection, and websocket fanout.

This document answers four practical questions:

1. Which proposal claims are accurate versus current code.
2. Which proposal claims need correction before implementation.
3. What final public API should be committed as stable.
4. How to execute the cutover with acceptable risk.

## Executive Summary

Verdict: the proposal is directionally strong and mostly aligned with the right long-term decomposition. It correctly identifies the central architectural tension: chat orchestration still owns stream lifecycle details and still performs one direct timeline write for user messages, while stream projection owns most other timeline entities. That split prevents a clean "one event pipeline, one projection model" guarantee.

The proposal should be accepted with targeted amendments.

What is correct and should be adopted:

- Separate chat submission/inference orchestration from websocket transport attachment.
- Introduce a first-class stream domain abstraction that can serve chat and non-chat publishers.
- Move user message projection into the same stream->SEM->projector path as assistant/tool events.
- Keep timeline projection transport-agnostic and fed by SEM frames.
- Offer optional HTTP helpers but keep route ownership app-level.

What needs adjustment before implementation:

- Do not fully delete all router/build convenience on day one; migrate to an explicit "bootstrap builders" module to avoid breaking app wiring ergonomics.
- Keep `ConversationRequestResolver` as an HTTP policy interface in helpers; it is still a useful app-facing boundary even if runtime policy internals are composer/provider-based.
- Keep existing queue/idempotency semantics unchanged during structural migration.
- Be explicit that current canonical hydration route is `/api/timeline`, not `/timeline`.
- Account for current debug route behavior (`/turns`, `/api/debug/*`) when defining what is public and what is internal.

Recommended final API posture for release:

- Public stable services: `ChatService`, `StreamHub`, `TimelineService`, `RuntimeProvider`.
- Public stable helper interfaces: `RequestResolver`, `StreamBackend`, optional `HTTP helpers`.
- Public stable behavior contracts: sequence ordering, idempotency semantics, timeline hydration consistency, lifecycle/eviction semantics.
- Explicitly non-public internals: `ConvManager`, `Conversation`, `StreamCoordinator`, debug route wiring.

This gives a clean releasable API while minimizing migration risk.

## Current State Validation (Code Reality)

This section anchors the analysis in what the repository does today.

### A. Route ownership status is already app-owned

Current code confirms GP-025 behavior:

- `pkg/webchat/router.go` explicitly documents that `NewRouter` does not register app transport routes `/chat` or `/ws`.
- `pkg/webchat/router.go` also states `APIHandler()` intentionally excludes app-owned `/chat` and `/ws`.
- `cmd/web-chat/main.go` mounts `/chat` and `/ws` in app code using `NewChatHandler` and `NewWSHandler`.

Conclusion: route ownership objective is already achieved and should remain unchanged.

### B. Timeline hydration route is `/api/timeline`

Current code registers timeline snapshot handlers at:

- `/api/timeline`
- `/api/timeline/`

That is implemented in `pkg/webchat/router_timeline_api.go`.

Conclusion: any proposal text that treats `/timeline` as canonical hydration endpoint is stale relative to current implementation and should be normalized to `/api/timeline`.

### C. Service boundaries are still mixed

`ConversationService` currently contains:

- chat submission and queue/idempotency orchestration
- websocket attach and ping/pong behavior
- timeline upsert event emission pathway (`timeline.upsert` broadcast)
- direct user message timeline store writes before inference starts

The key coupling is in `startInferenceForPrompt` where user messages are inserted directly into `TimelineStore` with a locally derived timestamp-based version, then emitted through `emitTimelineUpsert`.

Conclusion: the proposal's observation is accurate that chat domain still performs direct timeline writes, which should be removed for a cleaner model.

### D. Stream pipeline currently assumes Geppetto event JSON input

`StreamCoordinator.consume` currently:

- parses payload via `events.NewEventFromJson`
- translates event to SEM with `SemanticEventsFromEventWithCursor`
- forwards frames to callbacks

There is no fast path for payloads that are already SEM envelopes.

Conclusion: proposal recommendation to support both Geppetto events and prebuilt SEM envelopes is valid and important for background-agent style publishers.

### E. Stream lifecycle has a real edge case

Current stream starts at conversation creation in `ConvManager.GetOrCreate`. Idle stopping is primarily tied to `ConnectionPool` idle callbacks on add/remove transitions. If no websocket is ever attached, that pool idle callback never fires, and stream stop depends on broader eviction behavior.

`ConvManager` does have an eviction loop that can eventually clean up conversations, but the stop behavior is not modeled as a first-class stream maintenance policy independent of websocket attachment history.

Conclusion: proposal callout about headless/no-WS lifecycle deserves action, with a dedicated stream maintenance policy.

### F. Runtime policy shape

Current runtime composition is app-owned via `RuntimeComposer` and `RuntimeComposeRequest`.

`ConversationRequestResolver` still exists as an HTTP request policy boundary for app handlers (`NewChatHandler`, `NewWSHandler`).

Conclusion: proposal should preserve this split conceptually: app HTTP request planning remains distinct from runtime engine composition.

## Proposal Claim-by-Claim Assessment

### 1) "Webchat becomes a toolkit, not a router"

Assessment: accepted.

Why:

- This aligns with existing direction started by GP-025.
- Current `Router` still does too much construction and composition, and naming can imply ownership beyond utility handlers.
- A toolkit model with explicit services creates clearer responsibilities and better reuse.

Required amendment:

- Keep a small set of builder helpers for store/backend setup so app integration remains straightforward.

### 2) Domain decomposition into Stream, Chat, Timeline

Assessment: accepted.

Why:

- Matches current fault lines in the code.
- Clarifies extension points and ownership of lifecycle concerns.
- Makes non-chat progress streaming feasible without importing chat concerns.

Required amendment:

- Maintain explicit contract boundaries between "internal state holders" and "public services" to avoid surfacing unstable internals.

### 3) Remove direct chat-owned timeline writes

Assessment: strongly accepted.

Why:

- Current direct upsert for user messages can drift from stream-derived ordering semantics.
- A single projection path improves ordering determinism and conceptual integrity.

Required amendment:

- Preserve existing user-visible behavior exactly while moving implementation path. Specifically keep user-message immediate appearance semantics via stream events.

### 4) Stream accepts both Geppetto event payloads and SEM envelope payloads

Assessment: strongly accepted.

Why:

- Enables background agents and non-Geppetto publishers.
- Reduces repeated translation work for upstream producers that already emit SEM.

Required amendment:

- Define strict envelope validation and normalization behavior so malformed SEM inputs fail predictably and do not poison stream loops.

### 5) "Router is not needed; delete it"

Assessment: partially accepted.

Why:

- Architectural direction is right.
- However, deleting all construction ergonomics at once increases migration risk and encourages duplicated app wiring.

Required amendment:

- Replace `Router` with narrow builder modules, not nothing. Recommended names:
  - `streambackend` builder helpers
  - `storeopen` helpers
  - `uihandler` helper

### 6) API sketch for `StreamBackend`, `StreamHub`, `TimelineService`, `ChatRuntimeProvider`, `ChatService`

Assessment: broadly accepted, with signature refinements.

Refinements recommended:

- Keep `ChatRuntimeProvider` returning runtime metadata and engine artifacts; sink wrapping should remain explicit in chat configuration and not leak transport details.
- Keep `ConversationRequestResolver` as optional HTTP helper boundary; proposal should not imply this disappears.
- `TimelineService.HTTPHandler()` should be optional convenience and not required for direct app integration.

### 7) Hard cutover implementation steps

Assessment: largely accepted.

Most useful steps:

- Extract stream backend.
- Add SEM fast path in stream coordinator.
- Create stream hub with maintenance policy.
- Move user message projection into SEM path.
- Rework chat service around stream ensure/publish model.

Primary risk:

- A single massive hard cut with no temporary adapters can create high regression risk in tests and downstream examples.

Recommendation:

- Keep the conceptual hard cut but execute as staged internal transitions with clear compatibility checkpoints.

## Gaps and Corrections in the Proposal

### Gap 1: Endpoint naming drift

The proposal frequently uses `/timeline`, but current canonical API path is `/api/timeline`.

Correction:

- All examples and contracts should use `/api/timeline`.
- If legacy `/timeline` is kept temporarily, mark it deprecated and not part of public release contract.

### Gap 2: Runtime policy layering terminology

The proposal sometimes speaks as if resolver behavior is core-owned. In current design:

- Request resolution is app-owned (`ConversationRequestResolver` in app handlers).
- Runtime composition is app-owned (`RuntimeComposer`).

Correction:

- Preserve this layered ownership in final public API docs.

### Gap 3: Implicit debug surface handling

Current code includes debug route toggles and legacy paths. Proposal does not define whether these are public.

Correction:

- Explicitly classify debug/offline endpoints as non-public and stability-not-guaranteed unless separately promoted.

### Gap 4: Migration ergonomics for `cmd/web-chat` and examples

Proposal assumes clean replacement without discussing temporary wiring support.

Correction:

- Provide a compact migration adapter package for one release cycle if needed internally, even if not publicly documented as stable.

### Gap 5: Sequence semantics under mixed producers

Proposal recommends mixed Geppetto and prebuilt SEM ingestion, but does not define deterministic seq patching precedence.

Correction:

- Define sequence policy:
  - stream cursor-derived sequence is authoritative in consumer output.
  - incoming SEM `event.seq` from publisher is ignored or replaced.
  - `stream_id` is normalized from cursor metadata when available.

## Recommended Final Releasable Public API

This section defines what should be public and stable.

### Public Package Surface

Primary package can remain `pkg/webchat` initially, but exported API should be intentionally small.

#### 1) Chat

```go
type ChatRuntimeRequest struct {
    ConvID     string
    RuntimeKey string
    Overrides  map[string]any
}

type ChatRuntime struct {
    Engine             engine.Engine
    RuntimeKey         string
    RuntimeFingerprint string
    SeedSystemPrompt   string
    AllowedTools       []string
}

type ChatRuntimeProvider interface {
    Resolve(ctx context.Context, req ChatRuntimeRequest) (ChatRuntime, error)
}

type ChatServiceConfig struct {
    BaseCtx          context.Context
    Streams          *StreamHub
    Runtime          ChatRuntimeProvider
    StepController   *toolloop.StepController
    TurnStore        chatstore.TurnStore
    ToolFactories    map[string]ToolFactory
    EventSinkWrapper EventSinkWrapper
}

type ChatService struct { /* opaque */ }

func NewChatService(cfg ChatServiceConfig) (*ChatService, error)
func (s *ChatService) EnsureConversation(ctx context.Context, req ChatRuntimeRequest) (*ConversationHandle, error)
func (s *ChatService) SubmitPrompt(ctx context.Context, in SubmitPromptInput) (SubmitPromptResult, error)
```

Stability contract:

- queue/idempotency behavior remains backward-compatible.
- no websocket concerns in chat API.
- no direct timeline store writes from chat API.

#### 2) Streaming

```go
type StreamBackend interface {
    Publisher() message.Publisher
    UISubscriber(convID string) (message.Subscriber, bool, error)
    Close() error
}

type StreamHubConfig struct {
    BaseCtx            context.Context
    Backend            StreamBackend
    TimelineStore      chatstore.TimelineStore
    SemBufferSize      int
    StopStreamAfter    time.Duration
    EvictStateAfter    time.Duration
    SweepEvery         time.Duration
    TimelineUpsertHook func(convID string, entity *timelinepb.TimelineEntityV1, version uint64)
}

type StreamHub struct { /* opaque */ }

func NewStreamHub(cfg StreamHubConfig) (*StreamHub, error)
func (h *StreamHub) Ensure(ctx context.Context, convID string) error
func (h *StreamHub) AttachWebSocket(ctx context.Context, convID string, conn *websocket.Conn, opts WebSocketAttachOptions) error
func (h *StreamHub) PublishSEM(ctx context.Context, convID string, ev SEMEvent) error
func (h *StreamHub) StartMaintenance(ctx context.Context)
```

Stability contract:

- supports both Geppetto payload inputs and prebuilt SEM envelope inputs through the same stream consumer path.
- stream seq ordering policy is explicit and deterministic.

#### 3) Timeline

```go
type TimelineService struct { /* opaque */ }

func NewTimelineService(store chatstore.TimelineStore) *TimelineService
func (s *TimelineService) Snapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, error)
func (s *TimelineService) HTTPHandler() http.HandlerFunc
```

Stability contract:

- canonical route semantics documented as `/api/timeline` in first-party examples.
- timeline payload schema remains protobuf-defined and versioned.

#### 4) Optional HTTP helpers

```go
func NewChatHandler(chat *ChatService, resolver ConversationRequestResolver) http.HandlerFunc
func NewWSHandler(streams *StreamHub, resolver ConversationRequestResolver, upgrader websocket.Upgrader) http.HandlerFunc
func NewTimelineHandler(timeline *TimelineService) http.HandlerFunc
func UIHandler(staticFS fs.FS) http.Handler
```

Stability contract:

- handlers are optional glue only.
- apps own mounting and auth.

### Non-Public Internals (Do Not Promise)

- `ConvManager`
- `Conversation`
- `StreamCoordinator`
- any router legacy behavior
- debug/offline route sets

These can continue evolving without semver promises.

## Recommended Implementation Approach

I would execute this in five phases, even if the merge narrative is "hard cutover".

### Phase 1: Stream input unification (low API blast radius)

Deliverables:

- Add SEM envelope fast path in `StreamCoordinator`.
- Add tests for mixed payload ingestion.
- Keep all external APIs unchanged.

Reason:

- This unlocks agent progress streaming without forcing immediate service decomposition.

### Phase 2: User message projection migration

Deliverables:

- Introduce `chat.message` (or equivalent) SEM event and timeline handler.
- Remove direct `TimelineStore.Upsert` user write from chat service.
- Preserve immediate UX by publishing user message SEM before inference start completion response.

Reason:

- This eliminates the largest conceptual inconsistency with minimal endpoint impact.

### Phase 3: Introduce `StreamHub` and move WS attach

Deliverables:

- Create `StreamHub` owning attach/broadcast/projection lifecycle.
- Update chat service to depend on stream hub ensure/publish and drop ws concerns.
- Introduce explicit maintenance policy independent from connection pool callbacks.

Reason:

- This achieves domain separation where transport and chat are no longer interleaved.

### Phase 4: Introduce `ChatRuntimeProvider` and `ChatService` public surface

Deliverables:

- Keep existing semantics but rename and reshape public constructors.
- Provide temporary internal adapters from old types where needed in command wiring.

Reason:

- This stabilizes the intended public API contract before removing old shims.

### Phase 5: Router deprecation and removal from public docs

Deliverables:

- Replace router-first docs with service-first integration docs.
- Keep narrow builders for backend/store/UI helper wiring.
- Remove or internalize legacy router tests.

Reason:

- Avoids breaking integration ergonomics while completing architecture cleanup.

## Plan Update After Expert Details Import

After importing `sources/local/webchat-refactor-details.md`, I validated that its mechanical mapping and commit sequence are useful and mostly consistent with this plan. I am updating the execution approach to incorporate the strongest expert recommendations.

### Adopted updates

1. Package split strategy for cutover:
   - Keep root `pkg/webchat` for now, but split into domain subpackages:
     - `pkg/webchat/stream/*`
     - `pkg/webchat/chat/*`
     - `pkg/webchat/timeline/*`
     - `pkg/webchat/http/*`
     - optional `pkg/webchat/bootstrap/*`
   - This preserves import stability while delivering architectural clarity.
2. Commit sequencing:
   - Use an explicit extraction sequence similar to the expert doc:
     - backend extraction
     - SEM fast path
     - timeline service extraction
     - user message timeline handler
     - stream hub introduction
     - chat service extraction
     - HTTP helper cutover
     - app cutover (`cmd/web-chat`, then `web-agent-example`)
     - final legacy deletion
   - This is still a hard cutover in outcome, but with safer intermediate buildability.
3. File move ledger usage:
   - Treat the expert file map as the implementation checklist during refactor execution.
   - This reduces missed references and package-cyclic surprises during large moves.
4. Explicit cutover warnings promoted to hard gates:
   - ordering/version monotonicity checks
   - no-WS lifecycle stop/eviction checks

### Expert recommendations I am keeping but constraining

1. Router deletion:
   - I still recommend deleting router as public abstraction.
   - I recommend retaining narrow bootstrap builders (store/backend/UI helpers) so application wiring remains ergonomic.
2. Naming and symbol migrations:
   - I accept the rename direction (`ConversationService` -> `chat.Service`, `ConvManager` streaming concerns -> `stream.Hub`).
   - I constrain public exposure to service-level APIs and keep lower-level state types internal.

### Net plan adjustment

The prior 5-phase plan remains valid, but implementation will now follow the expert's more detailed move/commit choreography. This increases delivery confidence without changing the target public API shape or ownership model.

## Detailed Risk Analysis

### Risk 1: Ordering regressions in timeline hydration

Cause:

- Changing where user messages are projected can reorder entities if seq assignment is inconsistent.

Mitigation:

- Enforce single seq authority at stream consumer output.
- Add regression tests for mixed user/assistant/tool ordering under both in-memory and Redis metadata.

### Risk 2: Silent behavior drift in idempotency/queue semantics

Cause:

- Service decomposition can accidentally alter response codes and queue position behavior.

Mitigation:

- Preserve existing queue/idempotency code paths exactly in first cut.
- Add golden tests for duplicate idempotency keys, queued requests, running status, completion status.

### Risk 3: No-WS conversation resource leaks

Cause:

- Streams started for headless conversations may not stop promptly.

Mitigation:

- Add stream-level maintenance policy independent from ws attachment transitions.
- Test headless conversation creation and eventual stream stop+state eviction.

### Risk 4: Integrator confusion during migration

Cause:

- Existing docs and examples still refer to outdated router semantics and endpoint paths.

Mitigation:

- Update first-party docs in lockstep with API cut.
- Publish a one-page migration map: old symbol -> new symbol.

### Risk 5: Public API overexposure

Cause:

- Exporting too many low-level types freezes internals prematurely.

Mitigation:

- Keep internals opaque and service-centric.
- Mark non-public extension points explicitly.

## Testing and Release Gates for Public API Readiness

A releasable public API needs explicit gates.

### Contract Tests

Required:

- Chat contract: `SubmitPrompt` behavior, status fields, idempotency behavior.
- Stream contract: ordered SEM broadcast, websocket hello/ping/pong behavior if retained.
- Timeline contract: snapshot correctness under incremental `since_version` queries.

### Integration Tests

Required:

- `cmd/web-chat` app-owned routes with new services.
- `web-agent-example` integration with event sink wrapper and custom timeline handlers.
- Mixed producer stream (chat + external agent) to one conversation.

### Performance/Safety Tests

Required:

- stream consumer stability under malformed SEM payloads.
- bounded memory behavior for sem buffer and state eviction.
- Redis stream ID ordering consistency.

### Documentation Gates

Required before release:

- All examples use `/api/timeline`.
- Docs no longer suggest removed options (`WithConversationRequestResolver` in `NewRouter`, etc).
- A dedicated "Public API" page listing stable surfaces and compatibility guarantees.

## My Recommended Final Position on the Proposal

I support adopting this proposal as the baseline architecture with the amendments in this document.

Specifically:

- Accept decomposition into `ChatService`, `StreamHub`, and `TimelineService`.
- Accept stream input dual-mode support (Geppetto events plus SEM envelopes).
- Accept user-message projection migration to stream/projector path.
- Accept app-owned route mounting as non-negotiable final model.
- Amend router deletion into "replace with narrow builders" rather than "remove all helpers".
- Preserve resolver/provider layering as separate app-owned policy boundaries.

If implemented this way, the resulting API is releasable for external consumers because it is:

- coherent (clear ownership boundaries)
- composable (apps can use only needed services)
- consistent (single projection model)
- stable (service-level contracts instead of structural internals)

## Usage Examples

### Example 1: App-owned route wiring with final API

```go
backend := webchat.NewStreamBackendFromValues(parsed)
timelineStore := webchat.OpenTimelineStoreFromValues(parsed)
turnStore := webchat.OpenTurnStoreFromValues(parsed)

streams, _ := webchat.NewStreamHub(webchat.StreamHubConfig{
    BaseCtx:         ctx,
    Backend:         backend,
    TimelineStore:   timelineStore,
    StopStreamAfter: 60 * time.Second,
    EvictStateAfter: 5 * time.Minute,
    SweepEvery:      1 * time.Minute,
})

chat, _ := webchat.NewChatService(webchat.ChatServiceConfig{
    BaseCtx:        ctx,
    Streams:        streams,
    Runtime:        runtimeProvider,
    StepController: stepCtrl,
    TurnStore:      turnStore,
    ToolFactories:  toolFactories,
})

timeline := webchat.NewTimelineService(timelineStore)

mux := http.NewServeMux()
mux.HandleFunc("/chat", webchat.NewChatHandler(chat, requestResolver))
mux.HandleFunc("/ws", webchat.NewWSHandler(streams, requestResolver, upgrader))
mux.HandleFunc("/api/timeline", timeline.HTTPHandler())
mux.Handle("/", webchat.UIHandler(staticFS))
```

### Example 2: Background agent publishes realtime progress

```go
_ = streams.PublishSEM(ctx, convID, webchat.SEMEvent{
    Type: "agent.progress",
    ID:   "agent.progress:" + taskID,
    Data: rawProgressPayload,
})
```

If hydration is required for this event class, register a timeline handler for `agent.progress`.

### Example 3: Migration map (old to new)

- `ConversationService.AttachWebSocket` -> `StreamHub.AttachWebSocket`
- `ConversationService.SubmitPrompt` -> `ChatService.SubmitPrompt`
- `router_timeline_api` usage -> `TimelineService.HTTPHandler`
- Router-owned setup -> builder helpers plus app mux wiring

## Quick Reference

### What matches today vs proposal

- Route ownership app-level: matches.
- Stream dual input mode: not yet, should be added.
- User message through projector pipeline: not yet, should be added.
- Timeline endpoint canonical `/api/timeline`: current code yes, proposal text needs normalization.
- Router elimination direction: mostly right, but keep minimal builders.

### Minimum viable "public API ready" checklist

1. Service decomposition complete.
2. Stream accepts Geppetto and SEM payloads.
3. User messages projected via stream/projector path.
4. `/api/timeline` contract documented and tested.
5. Docs and examples updated to service-first integration.
6. Internal-only types clearly marked non-public.

## Related

- Imported proposal source: `sources/local/webchat-refactor.md`
- Current route ownership and handler split: `pkg/webchat/router.go`, `pkg/webchat/app_owned_handlers.go`, `cmd/web-chat/main.go`
- Current chat/timeline coupling point: `pkg/webchat/conversation_service.go`
- Stream input behavior today: `pkg/webchat/stream_coordinator.go`
- Timeline projection behavior: `pkg/webchat/timeline_projector.go`, `pkg/webchat/timeline_registry.go`
