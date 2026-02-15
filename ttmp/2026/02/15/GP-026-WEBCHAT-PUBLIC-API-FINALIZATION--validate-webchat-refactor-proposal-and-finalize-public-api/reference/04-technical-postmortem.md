---
Title: Technical Postmortem
Ticket: GP-026-WEBCHAT-PUBLIC-API-FINALIZATION
Status: active
Topics:
    - api
    - architecture
    - webchat
    - documentation
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/webchat/stream_coordinator.go
      Note: Stream ingestion mode changes and startup readiness semantics
    - Path: pkg/webchat/conversation_service.go
      Note: Chat submission behavior and stream-published user message events
    - Path: pkg/webchat/stream_hub.go
      Note: Stream/websocket ownership extraction
    - Path: pkg/webchat/http_helpers.go
      Note: Public HTTP helper contracts
    - Path: pkg/webchat/timeline_service.go
      Note: Timeline hydration service extraction
    - Path: pkg/webchat/http_helpers_contract_test.go
      Note: API behavior contract tests
    - Path: cmd/web-chat/main.go
      Note: First-party service-based wiring
    - Path: ../web-agent-example/cmd/web-agent-example/main.go
      Note: Second app cutover to split services and sink wrapper validation
ExternalSources: []
Summary: Deep technical postmortem for GP-026 implementation: architecture deltas, sequencing rationale, failure analysis, validation matrix, and release-readiness outcomes.
LastUpdated: 2026-02-15T15:24:00-05:00
WhatFor: Provide a detailed engineering-grade retrospective that explains exactly what changed, why it changed, what failed, and how final release readiness was established.
WhenToUse: Use when reviewing GP-026 implementation quality, onboarding new maintainers to the split service API, or planning subsequent cleanup/refactor phases.
---

# Technical Postmortem

## Goal

Produce a full technical retrospective of GP-026 that goes beyond the implementation diary by synthesizing architecture intent, execution sequencing, design tradeoffs, failure analysis, testing strategy, and release-readiness criteria.

This postmortem is intended to answer five questions in a way that is directly useful to maintainers and reviewers:

1. What architecture was in place before GP-026 and where were the fault lines?
2. What was changed, in what order, and why was that order chosen?
3. Which technical failures occurred during execution and how were they resolved?
4. What objective evidence demonstrates correctness and release readiness?
5. What residual risks remain and what should the next follow-up slice prioritize?

## Context

### Baseline Prior to GP-026

Before this ticket, the system had already completed GP-025 route ownership changes, which moved `/chat` and `/ws` mounting into app code. However, inside `pkg/webchat`, core lifecycle behavior was still coupled:

- Stream ingestion and websocket fanout concerns were mixed with chat/inference concerns.
- Timeline projection was mostly stream-driven, but user message writes still had a direct path from chat submission logic into timeline storage.
- Public integration ergonomics still encouraged a router-centric mental model even when app-level ownership had shifted.

At a practical level, this meant there was a mismatch between desired architecture and actual failure domains. App developers thought in terms of explicit service boundaries (chat vs stream vs timeline), while internals still had a blended ownership model.

### Target Outcome

GP-026 was framed as a finalization pass for a releasable public API and required the following end state:

- Explicit service-level surfaces for chat, stream, and timeline concerns.
- Optional but explicit HTTP helper constructors against those services.
- Stable package-level API exports suitable for external use.
- First-party apps (`cmd/web-chat`, `web-agent-example`) migrated to the new surfaces.
- Legacy entry points removed where replacement was complete and verified.
- Documentation and contract tests demonstrating behavior, not only structure.

### Constraints

Execution was constrained by four realities:

1. Existing behavior had to remain stable across in-memory and Redis stream backends.
2. Route ownership semantics from GP-025 could not regress.
3. Integration tests already encoded important UX assumptions (hello/pong, timeline hydration, projection visibility).
4. Large package moves late in the ticket were risky; API stabilization needed to prioritize reliability over maximal internal relocation.

## System Delta Overview

### Pre-GP-026 Core Flow

The dominant flow before refactor completion can be summarized as:

1. App route calls into service methods associated with conversation lifecycle.
2. Service ensures conversation state and handles queue/idempotency.
3. Service starts inference, and events are emitted to stream.
4. Stream coordinator translates events to SEM envelopes and forwards to websocket + optional projector.
5. Timeline hydration is served from store via `/api/timeline`.

But user message entities had a special handling path that bypassed unified stream projection semantics in early versions.

### Post-GP-026 Core Flow

The finalized flow is explicitly service-oriented:

1. Chat submission enters `ChatService` (`SubmitPrompt`).
2. Conversation ensure + websocket lifecycle ownership goes through `StreamHub` for stream-facing operations.
3. User message events are published as `chat.message` SEM envelopes into stream path.
4. `StreamCoordinator` handles mixed payload inputs (Geppetto event JSON and prebuilt SEM envelope JSON).
5. Timeline projection is unified through SEM processing and exposed via `TimelineService` + `NewTimelineHTTPHandler`.
6. App-level routes are mounted via explicit helper constructors:
   - `NewChatHTTPHandler`
   - `NewWSHTTPHandler`
   - `NewTimelineHTTPHandler`

This changed the design from incidental decomposition to intentional decomposition.

## Execution Narrative and Sequencing

### Why the Sequence Mattered

The chosen sequence was not just implementation convenience; it was a risk-control strategy.

High-risk changes were deferred until enabling slices were proven:

- First, make stream ingestion flexible and deterministic.
- Then, remove direct timeline write divergence.
- Then, extract service boundaries while preserving callsite compatibility.
- Finally, remove legacy surface once first-party app migrations were green.

The sequence reduced the chance of intertwined regressions that are difficult to isolate in evented systems.

### Slice-by-Slice Technical Changes

#### Slice A: Stream ingestion unification and chat.message projection foundation

Key commits:

- `1fd28a1` stream coordinator SEM envelope ingestion
- `206f4eb` builtin timeline `chat.message` handler
- `492ed15` user message published as stream event

Technical delta:

- `StreamCoordinator` gained support for payloads that are already SEM envelopes.
- Cursor-based `seq` patching remained authoritative to avoid ambiguous ordering.
- `TimelineProjector` gained a builtin `chat.message` mapping, enabling user messages to enter hydration through the same event pipeline used for assistant/tool events.

Design impact:

- Eliminated semantic mismatch between user and assistant entity persistence paths.
- Enabled non-Geppetto publishers to participate in stream without double encoding.

#### Slice B: Stream backend abstraction and explicit HTTP helper extraction

Key commits:

- `41c3e47` StreamBackend extraction
- `4649683` explicit HTTP helper constructors

Technical delta:

- Backend subscriber/publisher logic moved behind `StreamBackend` (`in-memory` and `redis` routing concerns abstracted).
- New HTTP helper contracts were introduced:
  - chat handler contract against chat submit surface
  - websocket handler contract against stream ensure/attach surface
  - timeline handler contract against snapshot surface
- Legacy helper names temporarily became wrappers to minimize breakage during migration.

Design impact:

- Clarified where transport setup belongs.
- Reduced route handler duplication and moved toward explicit, testable API contracts.

#### Slice C: StreamHub extraction and ChatService API split

Key commits:

- `cf53370` StreamHub extraction
- `546bcbe` ChatService split API

Technical delta:

- Websocket attach, hello/pong behavior, and conversation ensure logic moved into `StreamHub`.
- `ConversationService` delegated stream ownership operations to `StreamHub`.
- `ChatService` surface introduced as chat-focused API (queue/idempotency/inference oriented, no websocket attach methods in primary surface).
- Router/server accessors were added for split services.

Design impact:

- Established separate operational ownership: chat orchestration vs stream/session fanout.
- Reduced cognitive load for app consumers integrating HTTP transport.

#### Slice D: First-party app cutover and timeline service extraction

Key commits:

- `1311694` cmd/web-chat service-based cutover
- `06e30a7` TimelineService extraction
- `6b25156` web-agent-example service-based cutover (other repo)

Technical delta:

- `cmd/web-chat` switched from router-centric wiring to server/service wiring.
- `TimelineService` extracted and mounted independently via explicit handler helper.
- `web-agent-example` switched to same split pattern and gained targeted sink wrapper tests.

Design impact:

- Public API was no longer theoretical; first-party usage proved it practical.
- Timeline hydration route handling became explicitly domain-backed and independently mountable.

#### Slice E: Legacy surface deletion, public subpackage exports, and release docs/tests

Key commits:

- `1a6b0ab` remove legacy conversation handler entry points
- `33386bb` subpackage exports + HTTP contract tests
- `7ca15c5`, `b0e9674` docs/diary finalization

Technical delta:

- Legacy exported handler entry points (`NewChatHandler`, `NewWSHandler`) removed from root surface.
- Stable public export packages added:
  - `pkg/webchat/chat`
  - `pkg/webchat/stream`
  - `pkg/webchat/timeline`
  - `pkg/webchat/http` (imported as `webhttp` to avoid stdlib naming conflict)
  - `pkg/webchat/bootstrap`
- HTTP helper contract tests added for behavior-level API verification.
- Migration notes and postmortem documentation added to ticket.

Design impact:

- API now has explicit stable entry points and documented migration path.
- Consumer-facing contract is decoupled from most root-package internal churn.

## Deep Technical Analysis

### 1) Stream Semantics: Mixed Input Modes and Sequence Authority

One of the most consequential changes was treating stream input mode as an explicit compatibility dimension rather than an assumption.

#### Problem

The previous stream path was oriented around Geppetto event JSON ingestion. This constrained extension points and created pressure to re-encode events unnecessarily.

#### Resolution

`StreamCoordinator` now supports two ingestion modes:

- Geppetto event JSON -> translate to SEM envelopes.
- SEM envelope JSON -> normalize and forward.

In both modes, stream cursor metadata retains authority for sequencing.

#### Why this matters

- Prevents producer-specific `seq` behavior from fragmenting timeline order semantics.
- Simplifies integration for systems that already produce SEM-compliant events.
- Preserves deterministic projection behavior across mixed publisher ecosystems.

### 2) Timeline Consistency: Single Projection Path

A key architectural invariant after GP-026 is that timeline hydration is sourced by projection from stream SEM events, including user messages.

#### Problem

Direct timeline writes from chat submission created a split-brain path:

- Some entities originated from stream projection.
- User messages could originate from direct store write.

This can produce subtle ordering drift and mental-model friction.

#### Resolution

User message creation now publishes `chat.message` SEM envelopes. Projector handlers persist them through the same projection pipeline.

#### Why this matters

- Establishes one canonical path from event to hydrated timeline state.
- Makes ordering and replay behavior easier to reason about and test.
- Reduces risk when introducing new publishers or advanced middleware.

### 3) Service Ownership: Chat vs Stream vs Timeline

The service split was intentionally pragmatic.

#### ChatService

Responsibilities:

- Queue semantics
- Idempotency behavior
- Inference orchestration
- Runtime/tool integration points

Non-responsibilities:

- Websocket attach/read loop ownership
- Stream pool lifecycle

#### StreamHub

Responsibilities:

- Conversation ensure for stream-facing operations
- Websocket attach/read loop management
- Hello/ping/pong transport behaviors
- Stream-running lifecycle linkage with conversation state

#### TimelineService

Responsibilities:

- Snapshot retrieval contract
- Independent handler mounting for `/api/timeline`

This separation maps directly to operational failure domains and makes on-call/debug paths less ambiguous.

### 4) HTTP API Contracts: Behavior Over Wiring

Extraction of explicit helper constructors was only useful if behavior remained stable and testable.

The contract tests validate:

- Resolver error propagation semantics (`RequestResolutionError` status/message behavior).
- Chat helper submit contract and response propagation.
- Timeline helper success/error semantics and JSON response shaping.

This moved API confidence from "we wired it" to "we verified behavior-level contract invariants".

## Failure Analysis

### Failure 1: First-message projection race

Observed symptom:

- Integration test intermittently failed to find first user timeline entity after chat submit.

Root cause:

- Stream startup path was not guaranteeing subscription readiness before first publish.

Fix:

- Stream coordinator startup was changed to wait until `Subscribe` is established before returning success.

Result:

- First-message projection became deterministic in tests and real flow.

### Failure 2: Timeline test compile mismatch on protobuf oneof shape

Observed symptom:

- Compile failure using incorrect struct field for timeline entity payload.

Root cause:

- Test used `Value` instead of `Snapshot` oneof placement in `TimelineEntityV1` construction.

Fix:

- Updated construction to match existing protobuf usage (`Kind` + `Snapshot`).

Result:

- Timeline service tests compile and validate as expected.

### Failure 3: Debug route regression during timeline service extraction

Observed symptom:

- Debug API test expected timeline route behavior but received `404` in manually-constructed router test context.

Root cause:

- `timelineService` was nil in tests that set `timelineStore` directly on ad-hoc router structs.

Fix:

- Lazy initialization in timeline snapshot handler: when service is nil but store exists, instantiate service on demand.

Result:

- Existing debug tests preserved without forcing broad test rewrites.

### Failure 4: Integration compile error from variable shadowing

Observed symptom:

- `no new variables on left side of :=` during command test cutover.

Root cause:

- Name collision between webchat server variable and httptest server variable.

Fix:

- Renamed variables (`webchatSrv`, `httpSrv`) to eliminate shadowing ambiguity.

Result:

- Compile and test suites stabilized.

## Verification Matrix

### Pinocchio

Validated repeatedly across slices:

- `go test ./pkg/webchat -count=1`
- `go test ./pkg/webchat/... -count=1`
- `go test ./cmd/web-chat -count=1`

Coverage focus:

- Stream ingestion semantics
- Timeline projection correctness
- HTTP helper behavior
- App-owned chat/ws/timeline integration flow
- Debug and offline route behavior

### Web-agent-example

Validated after service cutover:

- `go test ./cmd/web-agent-example -count=1`
- `go test ./... -count=1`

Coverage focus:

- Route ownership and live ws/chat/timeline behavior
- Ping/pong behavior
- Event sink wrapper activation conditions

### Contract Readiness Indicators

Release readiness was not inferred from green tests alone. It was based on alignment across:

- Task completion coverage (all 15 tasks checked).
- First-party app migrations complete.
- Legacy entry points removed where replacements were active.
- Dedicated migration notes available.
- Public API export namespaces present and tested for compile/import viability.

## Public API Final State

### Canonical Helper Constructors

- `NewChatHTTPHandler`
- `NewWSHTTPHandler`
- `NewTimelineHTTPHandler`

### Canonical Service Surfaces

- `ChatService`
- `StreamHub`
- `TimelineService`

### Canonical Subpackage Imports

- `github.com/go-go-golems/pinocchio/pkg/webchat/chat`
- `github.com/go-go-golems/pinocchio/pkg/webchat/stream`
- `github.com/go-go-golems/pinocchio/pkg/webchat/timeline`
- `github.com/go-go-golems/pinocchio/pkg/webchat/http`
- `github.com/go-go-golems/pinocchio/pkg/webchat/bootstrap`

### Deprecated/Removed Surface (This Phase)

- Legacy root exported handler entry points for app-owned chat/ws route convenience were removed.
- Router accessor exposing legacy conversation-centric path was removed for public integration use.

## Engineering Tradeoffs and Rationale

### Tradeoff A: Re-export subpackages vs full physical file migration

Decision:

- Introduce stable subpackage exports as public contract layer now.

Rationale:

- Full physical relocation of internals late in ticket had high regression risk.
- API stabilization for consumers was more important than immediate file topology purity.
- Re-export layer gives a low-risk bridge to future internal relocations.

### Tradeoff B: Compatibility facade retention for ConversationService internals

Decision:

- Keep compatibility internals while shifting first-party and public usage to split services.

Rationale:

- Allowed incremental safety during transition.
- Avoided large accidental behavior shifts in queue/idempotency code.
- Made deletion of external legacy entry points possible without destabilizing internals.

### Tradeoff C: Handler-level lazy init for timeline service in test-created routers

Decision:

- Lazy-initialize timeline service from store when absent.

Rationale:

- Preserved existing tests that instantiate slim router structs.
- Minimized noisy fixture rewrites while keeping production flow explicit.

## Operational Lessons

1. In stream systems, readiness guarantees must be explicit; "started" without subscriber ready can still lose events.
2. API refactors are safest when behavior contracts are formalized as tests before deleting entry points.
3. Maintaining one canonical data projection path dramatically improves debuggability and replay confidence.
4. First-party app migration should happen before final API freeze; it exposes practical ergonomics and edge-case wiring issues early.
5. Subpackage export layers are effective for achieving public stability without immediate high-risk internal churn.

## Residual Risks

1. Internals still contain legacy compatibility structures that may confuse maintainers if left undocumented.
2. Re-export package surfaces can drift if root symbols are renamed without synchronized updates.
3. Mixed payload mode introduces additional validation surface; malformed SEM publisher behavior should continue to be monitored.
4. `ConversationService` lifecycle as an internal compatibility type should have a clear deprecation/removal plan to avoid dual mental models.

## Recommendations for Next Iteration

1. Add a formal deprecation annotation plan for remaining compatibility-root symbols.
2. Add CI coverage for import-compile checks against `webchat/{chat,stream,timeline,http,bootstrap}` packages in downstream sample modules.
3. Add stress tests for stream startup/stop churn with high-frequency early publishes under both in-memory and Redis backends.
4. If targeting a major version boundary, consider full physical package migration to align source layout with exported namespaces.

## Quick Reference

### Commit Map (Core GP-026 Completion Window)

- `41c3e47` stream backend abstraction
- `4649683` explicit HTTP helper constructors
- `cf53370` stream hub extraction
- `546bcbe` chat service split API
- `1311694` cmd/web-chat service cutover
- `06e30a7` timeline service extraction
- `1a6b0ab` legacy handler/accessor cleanup
- `33386bb` subpackage exports + helper contract tests
- `7ca15c5` documentation package (analysis/diary/migration)
- `b0e9674` final phase diary checkpoint

### Final Test Commands

```bash
# pinocchio
cd /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio
go test ./pkg/webchat/... -count=1
go test ./cmd/web-chat -count=1

# web-agent-example
cd /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/web-agent-example
go test ./... -count=1
```

### reMarkable Publication

- Bundle name: `GP-026 Webchat API Finalization`
- Remote path: `/ai/2026/02/15/GP-026-WEBCHAT-PUBLIC-API-FINALIZATION`

## Usage Examples

### Example 1: App wiring with split services (shape)

```go
srv, _ := webchat.NewServer(ctx, parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))

chatHandler := webchat.NewChatHTTPHandler(srv.ChatService(), resolver)
wsHandler := webchat.NewWSHTTPHandler(srv.StreamHub(), resolver, websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }})
timelineHandler := webchat.NewTimelineHTTPHandler(srv.TimelineService(), logger)

mux := http.NewServeMux()
mux.HandleFunc("/chat", chatHandler)
mux.HandleFunc("/ws", wsHandler)
mux.HandleFunc("/api/timeline", timelineHandler)
```

### Example 2: Preferred public import layer

```go
import (
    wbootstrap "github.com/go-go-golems/pinocchio/pkg/webchat/bootstrap"
    whttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
    "github.com/go-go-golems/pinocchio/pkg/webchat/chat"
    "github.com/go-go-golems/pinocchio/pkg/webchat/stream"
    "github.com/go-go-golems/pinocchio/pkg/webchat/timeline"
)
```

The above import structure should be treated as the stable consumer-facing namespace set from GP-026 onward.

## Related

- `ttmp/2026/02/15/GP-026-WEBCHAT-PUBLIC-API-FINALIZATION--validate-webchat-refactor-proposal-and-finalize-public-api/reference/01-webchat-refactor-proposal-validation-and-public-api-release-plan.md`
- `ttmp/2026/02/15/GP-026-WEBCHAT-PUBLIC-API-FINALIZATION--validate-webchat-refactor-proposal-and-finalize-public-api/reference/02-diary.md`
- `ttmp/2026/02/15/GP-026-WEBCHAT-PUBLIC-API-FINALIZATION--validate-webchat-refactor-proposal-and-finalize-public-api/reference/03-public-api-migration-notes.md`
