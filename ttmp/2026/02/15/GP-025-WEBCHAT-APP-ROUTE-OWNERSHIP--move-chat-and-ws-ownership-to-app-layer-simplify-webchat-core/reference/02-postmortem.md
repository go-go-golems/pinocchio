---
Title: 'Postmortem: GP-025 App-Owned Webchat Route Ownership Cutover'
Ticket: GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP
Status: active
Topics:
    - postmortem
    - architecture
    - webchat
    - routing
    - migration
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Core orchestration service introduced during cutover
    - Path: pinocchio/pkg/webchat/ws_publisher.go
      Note: Conversation-scoped websocket publisher
    - Path: pinocchio/pkg/webchat/router.go
      Note: Router ownership reductions and utility scoping
    - Path: pinocchio/cmd/web-chat/main.go
      Note: App-owned /chat and /ws wiring
    - Path: web-agent-example/cmd/web-agent-example/main.go
      Note: Migrated app-owned route composition in downstream example
Summary: Detailed 5+ page postmortem of GP-025 execution, technical decisions, implementation outcomes, validation evidence, incidents, and follow-up recommendations.
LastUpdated: 2026-02-15T01:45:00-05:00
WhatFor: Preserve technical context and lessons learned from the app-owned route ownership cutover.
WhenToUse: Use for architecture review, onboarding, and follow-up planning for webchat core and app integration work.
---

# Postmortem: GP-025 App-Owned Webchat Route Ownership Cutover

## 1. Executive Summary
GP-025 successfully completed a clean-cutover migration of webchat transport ownership from core router code to application code. Before this work, `pkg/webchat` still carried significant implicit ownership over HTTP transport behavior through router-era patterns. After this work, applications explicitly own `/chat` and `/ws`, while `pkg/webchat` is reduced to reusable conversation/runtime/persistence primitives and optional UI/API utility handlers.

The migration was executed in phased slices with disciplined task checkoff and commit granularity. Both first-party application paths (`cmd/web-chat`) and downstream example consumer (`web-agent-example`) were migrated. Core behavioral guarantees were preserved through targeted regression and integration testing, including queue/idempotency behavior, websocket hello/ping/pong semantics, timeline upserts, and end-to-end route behavior.

The ticket ended with all listed tasks checked complete, final design/contracts updated to match as-built signatures, phase-level diary documentation in place, changelog coverage across major cutover slices, and a final implementation bundle uploaded to reMarkable.

In short: the architectural objective was achieved, behavior remained stable, and documentation/handoff quality is materially better than at start.

## 2. Objectives, Constraints, and Success Criteria

### 2.1 Original objective
The objective was to move `/chat` and `/ws` route ownership out of `pkg/webchat` and into app code, while simplifying core architecture and preserving runtime behavior.

### 2.2 Explicit constraints
The migration was constrained by the following deliberate choices:
1. Clean cutover only: no backward-compatibility adapter layer.
2. Preserve stream, queue/idempotency, and persistence behavior.
3. Keep app policy decisions app-owned (resolver/runtime/profile behavior).
4. Maintain webchat ergonomics through thin app-owned handler adapters where useful.

### 2.3 Success criteria used in execution
The work was treated as successful only if all of the following were true:
1. `pkg/webchat` no longer registers `/chat` or `/ws`.
2. `cmd/web-chat` runs app-owned `/chat` and `/ws` handlers.
3. `web-agent-example` no longer relies on router-owned `/chat`/`/ws` behavior.
4. Validation matrix passes across webchat core, web-chat app, and web-agent-example.
5. Docs reflect as-implemented contracts and not just planning intent.
6. Ticket tasks are fully checked with traceable commits.

## 3. Baseline State and Why Change Was Needed
Before GP-025, architecture was functionally moving toward composable ownership, but still carried router-centric coupling that created friction:
1. Router code owned too much policy and transport behavior.
2. Optional hooks/options had multiplied to allow applications to influence router-owned decisions.
3. App concerns (request policy, runtime/profile selection, route composition) were frequently expressed through indirection rather than explicit app mux code.
4. Non-transport code still had direct websocket broadcast coupling in places (`ConnectionPool` exposure pattern).

This was not a feature deficit; it was an ownership mismatch. Complexity came from blurred boundaries, not missing functionality.

## 4. Execution Strategy and Phasing
Execution proceeded in strict phases with commit-and-check discipline:
1. Phase 1: Contract freeze (no-compat cutover decision, frozen service and handler contracts, explicit route boundary).
2. Phase 2: Core refactor in `pkg/webchat`.
3. Phase 3: App migration in `cmd/web-chat`.
4. Phase 4: App migration in `web-agent-example`.
5. Phase 5: Router simplification/deletion pass.
6. Phase 6: Validation and matrix testing.
7. Phase 7: Documentation and handoff completion.

This phasing was effective because it forced contract stability ahead of broad refactoring and reduced backtracking risk.

## 5. What Was Implemented

### 5.1 Core primitives introduced and stabilized

#### 5.1.1 `ConversationService`
`ConversationService` became the central app-consumable orchestration surface for:
1. `ResolveAndEnsureConversation(...)`
2. `SubmitPrompt(...)`
3. `AttachWebSocket(...)`
4. Persistence and timeline upsert integration through service config

This removed router-path dependence for key lifecycle operations.

#### 5.1.2 `WSPublisher`
A conversation-scoped websocket publisher API was introduced:
1. `PublishJSON(ctx, convID, envelope)`
2. `ErrConversationNotFound`
3. `ErrConnectionPoolAbsent`

This replaced non-transport direct pool broadcast dependencies and improved separation of concerns.

### 5.2 Core route ownership removal
Router-owned `/chat` and `/ws` registration was removed from `pkg/webchat`. Core retained optional helpers (`UIHandler`, `APIHandler`, `Mount`, `Handler`) but these were explicitly scoped as utilities and not canonical transport ownership.

### 5.3 App-owned route migration in `cmd/web-chat`
`cmd/web-chat` now explicitly composes:
1. `/chat` via `webchat.NewChatHandler(...)`
2. `/ws` via `webchat.NewWSHandler(...)`
3. App profile/runtime resolver logic in app code
4. `/api` and UI mounts through core utility handlers

Ping/pong behavior is explicit through websocket attach options and app-owned path composition.

### 5.4 App-owned route migration in `web-agent-example`
`web-agent-example` was migrated to:
1. Wire `ConversationService` explicitly.
2. Mount app-owned `/chat` and `/ws` directly.
3. Move resolver/runtime policy into app-owned handler path.
4. Remove router mux ownership dependency.

### 5.5 Simplification pass
Obsolete router route-policy options and dead legacy request resolver paths were removed. Package documentation was updated to declare and enforce app-owned transport ownership model.

## 6. Validation and Quality Evidence

### 6.1 Unit and regression coverage added
1. `ConversationService` lifecycle tests expanded to cover default ensure path, missing prompt validation, and websocket argument validation.
2. `WSPublisher` tests added for not-found, no-pool, and successful fanout.
3. Queue semantics regression protection retained and revalidated.

### 6.2 Integration coverage added
1. `cmd/web-chat` `/chat` integration flow test added.
2. `cmd/web-chat` `/ws` integration flow test added, asserting `ws.hello` and `ws.pong` behavior.
3. `web-agent-example` integrated chat/ws/timeline test added and then strengthened with explicit ping/pong assertion.

### 6.3 Matrix test runs completed
Focused matrix commands executed and recorded:
1. `go test ./pinocchio/pkg/webchat/...`
2. `go test ./pinocchio/cmd/web-chat/...`
3. `go test ./web-agent-example/...`

All passed.

### 6.4 Frontend check
Frontend build verification for impacted app:
1. `cd pinocchio/cmd/web-chat/web && npm run build`

Build succeeded (existing chunk-size warning only; no failure).

## 7. Operational Issues Encountered
Two recurring/real issues occurred during execution:

### 7.1 Intermittent pre-commit workspace failure
Intermittently, pre-commit repo-wide test hook failed with a transient missing `node_modules` path under `cmd/web-chat/web`. Immediate re-run succeeded without code changes.

Handling:
1. Failure was treated as environmental/transient, not silently ignored.
2. Commit was retried and hook eventually passed.
3. Incident was recorded in diary.

Impact:
1. Small time loss.
2. No functional risk introduced into code.

### 7.2 Lint failure on new integration test (`errcheck`)
A newly added websocket integration test initially failed lint due to unchecked `conn.Close` and `SetReadDeadline` returns.

Handling:
1. Fixed test to check/handle all returns.
2. Re-ran tests and commit hooks.
3. Commit then passed.

Impact:
1. No release risk.
2. Good reminder that integration tests must satisfy full lint discipline, not only behavior assertions.

## 8. Results Versus Objectives

### 8.1 Objective completion status
All stated objectives were achieved:
1. `/chat` and `/ws` are app-owned in both target applications.
2. `pkg/webchat` route ownership removed for transport routes.
3. Core service/publisher primitives implemented and used.
4. Validation matrix completed and passing.
5. Documentation finalized to as-built reality.
6. Ticket is fully checked complete.

### 8.2 What measurably improved
1. Ownership clarity: route/policy decisions now visibly live in app code.
2. Core decoupling: non-transport code no longer directly depends on pool broadcast internals for timeline upsert fanout.
3. Testability: app-owned behavior tested through normal integration tests without hidden router assumptions.
4. Maintainability: fewer route-policy router options and less monolithic behavior concentration.

### 8.3 What remained intentionally unchanged
1. SEM event format behavior.
2. Stream coordinator fundamentals.
3. General runtime/engine tool-loop semantics.

The migration changed ownership and composition, not product behavior contracts.

## 9. Design and Process Decisions That Worked

### 9.1 Freezing contracts first
Locking no-compatibility and API/handler contracts before broad code movement was the most important quality control. It reduced decision churn and made review criteria explicit.

### 9.2 Commit-per-task discipline
Task-level commits with explicit checkoff commits gave clean traceability from ticket to code and reduced ambiguity in review/handoff.

### 9.3 Phase-level diary cadence
Switching to phase-level diary updates (per user instruction) kept documentation useful without excessive noise.

### 9.4 Keeping app-owned adapter helpers thin
Retaining thin helpers (`NewChatHandler`, `NewWSHandler`) was a pragmatic middle-ground: app owns routing and policy, but boilerplate is still minimized.

## 10. Tradeoffs and Remaining Risks

### 10.1 Tradeoffs accepted
1. Apps now own more explicit mux composition code.
2. Consumers must understand route ownership model rather than relying on implicit router registration.
3. Documentation burden shifted toward explicit setup guidance.

These are acceptable tradeoffs because they exchange hidden coupling for explicit composition.

### 10.2 Residual risks
1. Additional downstream consumers (outside `cmd/web-chat` and `web-agent-example`) may still carry old assumptions and need migration guidance.
2. Repo-wide hook pipeline remains expensive (full tests, frontend build, lint), which can mask transient environment issues as commit friction.
3. Frontend bundle size warnings remain and may warrant separate optimization ticketing.

## 11. Follow-Up Recommendations

### 11.1 Immediate follow-ups
1. Add a short migration note/template for other consumers adopting app-owned `/chat` and `/ws`.
2. Consider adding a lightweight preflight script for frontend dependency readiness to reduce transient hook failures.

### 11.2 Medium-term follow-ups
1. Review large frontend chunk warning and decide whether to split bundles.
2. Optionally add one additional cross-repo smoke target that exercises both migrated apps under a shared script.

### 11.3 Long-term follow-ups
1. Keep `pkg/webchat` utility handlers intentionally minimal.
2. Resist re-centralizing app policy into router options unless a clearly reusable core concern emerges.

## 12. Outcome Summary for Stakeholders
This cutover materially improved architecture quality without sacrificing behavior.

For maintainers:
1. Core boundaries are clearer.
2. Behavior remains covered by tests.
3. Documentation now reflects implementation reality.

For application owners:
1. Route ownership and policy are explicit and local.
2. Runtime/profile behavior is easier to audit.
3. Integration tests now mirror real production path composition.

For future work:
1. The system now has a cleaner foundation for extension.
2. Additional consumer migrations can follow a documented path.

## 13. Appendix A: Representative Commit Landmarks

### 13.1 Core cutover highlights (`pinocchio`)
1. `11357a2` - introduce conversation service orchestration.
2. `27b6b39` - add conversation-scoped ws publisher.
3. `26e84d8` - remove router-owned chat/ws registration.
4. `fcff5b9` - remove obsolete router request/ws options.
5. `30609fe` - remove legacy default request resolver path.
6. `79e5f67` - document app-owned route ownership model.

### 13.2 Validation and integration highlights (`pinocchio`)
1. `420e665` - expand conversation service lifecycle tests.
2. `d4e3499` - cover ws publisher behavior.
3. `b4e1e91` - add app-owned chat integration coverage.
4. `cf402ee` - add app-owned websocket integration coverage.

### 13.3 Downstream migration highlights (`web-agent-example`)
1. `327b4df` - remove router mux ownership of chat/ws.
2. `a4b5101` - validate app-owned chat/ws/timeline flow.
3. `b0b9c81` - assert websocket ping/pong in integration flow.

## 14. Appendix B: Final Handoff Artifacts
1. Design document finalized with as-built signatures and deltas.
2. Diary updated through Phase 7 completion.
3. Changelog updated across major cutover slices.
4. Final reMarkable bundle uploaded as `GP-025 Webchat Toolkit Refactor Final` to `/ai/2026/02/15/GP-025` and verified via cloud listing.

## 15. Detailed Phase-by-Phase Technical Timeline

### 15.1 Phase 1: Contract freeze and boundary hardening
Phase 1 was deliberately documentation-first, but it was not “paperwork only.” It was the risk-control phase that prevented churn later.

Key technical outcomes:
1. Locked the no-compatibility migration decision.
2. Froze the target service API shape and behavior.
3. Froze app-owned handler contracts for `/chat` and `/ws`.
4. Froze route-ownership boundary semantics.

What this changed in practice:
1. Reviewers had a stable source of truth for what “done” means.
2. Implementation tasks could reject scope creep by pointing to frozen sections.
3. Migration discussions shifted from “what should we expose?” to “are we implementing the frozen contract correctly?”

Why this mattered:
1. Without this freeze, route ownership refactors tend to re-open interface debates mid-stream.
2. It made downstream app migration mechanical rather than interpretive.

### 15.2 Phase 2: Core refactor mechanics in `pkg/webchat`
Phase 2 was where the architecture changed materially. The core had to be re-centered around service primitives instead of route-driven orchestration.

Major technical changes:
1. Added `ConversationService` and moved orchestration there.
2. Added conversation-scoped `WSPublisher`.
3. Rewired timeline upsert fanout through publisher interface in non-transport paths.
4. Split previously monolithic helper logic from `router.go` into focused files.
5. Removed router registration of `/chat` and `/ws`.

Important implementation detail:
1. This did not dismantle `ConvManager`; it re-homed who calls it and where app policy enters.
2. Runtime composition remained explicit and app-driven.
3. Persistence wiring stayed available through service and manager config.

Regression risk addressed:
1. Queue/idempotency behavior risk was explicitly covered with regression tests.
2. Stream/persistence behavior was preserved while ownership moved.

### 15.3 Phase 3: `cmd/web-chat` migration details
Phase 3 validated whether first-party app composition could stay elegant without router-owned transport routes.

Actual composition after migration:
1. App mux owns `/chat` and `/ws` with explicit handler mounting.
2. Profile/runtime request resolution remains in app code.
3. Core utility routes are mounted intentionally (`/api/` and UI handler), not implicitly.

Key behavioral outcomes:
1. Websocket hello/ping/pong path stayed intact through service attach options.
2. Timeline/debug route mount decisions became explicit app choices.
3. Existing frontend behavior remained functional.

What got better:
1. Route-level ownership became obvious in `main.go`.
2. Policy decisions became easier to read and test.
3. There was less hidden coupling to router defaults.

### 15.4 Phase 4: `web-agent-example` migration details
This phase was critical because it tested whether the new model works for a non-primary app consumer.

Migration elements:
1. Audited router-route ownership dependencies.
2. Added explicit app-owned handler target shape.
3. Wired `ConversationService` in bootstrap.
4. Removed reliance on router mux route ownership behavior.
5. Added integration validation for live chat/ws/timeline behavior.

Why this mattered:
1. If only `cmd/web-chat` migrated cleanly, the architecture could still be too coupled to first-party assumptions.
2. Successful migration here showed the cutover model is general enough for downstream consumers.

### 15.5 Phase 5: Simplification/deletion pass details
Phase 5 intentionally removed debt, not only moved functionality.

Removed:
1. Obsolete router options tied to route-policy indirection.
2. Legacy default request resolver implementation path no longer used after app-owned migration.
3. Legacy tests tied only to removed path.

Clarified:
1. Remaining router helper APIs are optional utility handlers.
2. They are not central ownership abstractions and do not own `/chat` or `/ws`.

Documented:
1. Added package-level docs explicitly stating ownership model.
2. Updated server comments to avoid implying transport-route ownership.

### 15.6 Phase 6: Validation matrix details
Phase 6 validated correctness through layered tests and explicit matrix commands.

Coverage added:
1. Service lifecycle unit tests (`ResolveAndEnsureConversation`, prompt validation, websocket arg validation).
2. `WSPublisher` unit tests (not found/no pool/fanout).
3. `cmd/web-chat` integration tests for app-owned `/chat`.
4. `cmd/web-chat` integration tests for app-owned `/ws` hello/ping/pong behavior.
5. `web-agent-example` integration test enhancement with explicit ping/pong assertion.

Matrix executed:
1. `go test ./pinocchio/pkg/webchat/...`
2. `go test ./pinocchio/cmd/web-chat/...`
3. `go test ./web-agent-example/...`
4. Frontend build: `npm run build` in `pinocchio/cmd/web-chat/web`

Outcome:
1. Matrix passed.
2. Existing frontend bundle-size warning remained informational.

### 15.7 Phase 7: Documentation and handoff closure
Phase 7 ensured technical and process closure.

Completed:
1. Design doc updated to as-built signatures and implementation deltas.
2. Diary updated with phase checkpoints through completion.
3. Changelog updated for major cutover slices.
4. Final bundle uploaded to reMarkable and verified.

Why this phase is not optional:
1. Route-ownership migrations are high-context changes.
2. Without complete documentation, future contributors can accidentally reintroduce ownership ambiguity.

## 16. File-Level Delta Catalog

### 16.1 Core `pkg/webchat` deltas
Primary implementation files changed:
1. `conversation_service.go`: new central orchestration surface.
2. `ws_publisher.go`: new conversation-scoped publish API.
3. `router.go`: removed transport route ownership and clarified helper scope.
4. `router_options.go`: obsolete options removed.
5. `types.go`: obsolete route-policy fields removed.
6. `engine_from_req.go`: legacy default resolver path removed; shared request/plan types retained.
7. `doc.go`: package-level ownership model documentation added.

Primary test files changed:
1. `conversation_service_test.go`: expanded lifecycle coverage.
2. `ws_publisher_test.go`: added publisher contract coverage.
3. `router_handlers_test.go`: guard test to ensure API handler does not own `/chat` or `/ws`.

### 16.2 `cmd/web-chat` deltas
1. `main.go`: app-owned `/chat` and `/ws` route mounting.
2. `profile_policy.go`: app-level resolver/profile policy flow retained and clarified.
3. `app_owned_chat_integration_test.go`: `/chat` and `/ws` integration coverage with ping/pong assertion.

### 16.3 `web-agent-example` deltas
1. `main.go`: app mux ownership for `/chat` and `/ws`.
2. `app_owned_routes_integration_test.go`: live conversation validation plus explicit ping/pong assertion.

## 17. Quality of Execution: What Worked Operationally

### 17.1 High-confidence patterns
1. Contract-first sequencing prevented drift.
2. Task-level commit granularity gave clean auditability.
3. Matrix command checkoff ensured final validation was explicit, not implicit.
4. Phase-level diary cadence balanced detail and signal.

### 17.2 Collaboration and traceability quality
1. The ticket file remained the live source of state.
2. Checkoff commits separated status tracking from code behavior changes.
3. Major doc artifacts (design/diary/changelog) were each updated at natural phase boundaries.

## 18. What Could Be Improved Next Time

### 18.1 Hook pipeline ergonomics
The pre-commit hook pipeline is very heavy for every commit in this repository context (repo-wide tests, generation, frontend build, lint, vet). While this improves confidence, it increases turnaround time and surfaces transient environment instability.

Potential improvements:
1. Add a “fast path” mode for docs-only/task-check commits.
2. Add deterministic frontend dependency preflight to reduce transient failures.
3. Keep full pipeline in CI and for merge gates while allowing narrower local checks for small changes.

### 18.2 Ticket scaffold hygiene
Untracked scaffold artifacts (`README.md`, `index.md`) remained in the workspace throughout. They were not required for code delivery, but this is slightly noisy and can confuse quick status checks.

Potential improvements:
1. Decide at ticket start whether to track scaffold files.
2. If tracked, wire them into final handoff package intentionally.

### 18.3 Cross-repo synchronization
GP-025 touched both `pinocchio` and `web-agent-example`. Coordination worked, but commit timeline review requires checking two repositories.

Potential improvements:
1. Add a cross-repo synchronization note template in ticket docs.
2. Maintain a small “cross-repo milestones” table in the postmortem/design during execution.

## 19. Final Assessment
GP-025 delivered the intended architectural shift with controlled risk and strong validation discipline. The final state is both simpler and clearer:
1. Applications own transport routes and policy.
2. Core webchat exposes reusable lifecycle/publisher primitives.
3. Downstream consumer migration is proven, not hypothetical.
4. Documentation now describes actual implementation rather than aspirational architecture.

Given the scope and multi-phase nature of the cutover, this was a high-quality outcome with manageable operational issues and clear forward paths for incremental improvement.
