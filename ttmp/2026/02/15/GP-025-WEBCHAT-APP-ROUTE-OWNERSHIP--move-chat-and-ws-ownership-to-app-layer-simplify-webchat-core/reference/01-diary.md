---
Title: 'Diary: GP-025 Analysis and Design Work Log'
Ticket: GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP
Status: active
Topics:
    - architecture
    - webchat
    - routing
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/webchat/router.go
    - Path: pinocchio/pkg/webchat/conversation.go
    - Path: pinocchio/pkg/webchat/runtime_composer.go
    - Path: pinocchio/pkg/webchat/types.go
    - Path: pinocchio/pkg/webchat/timeline_upsert.go
Summary: Detailed chronological diary for GP-025 analysis and implementation.
LastUpdated: 2026-02-15T02:10:00-05:00
WhatFor: Preserve step-by-step exploration, findings, and writing decisions.
WhenToUse: Use when reviewing why the new proposal moved route ownership to applications.
---

# Diary: GP-025 Analysis and Design Work Log

## Step 1: Ticket creation and workspace setup
- Command: `docmgr ticket create-ticket --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP ...`
- Result: Created ticket scaffold under `pinocchio/ttmp/2026/02/15/GP-025-...` with standard folders (`design`, `reference`, `tasks`, `changelog`, etc.).
- Note: Chose `GP-025` because existing tickets in scope ended at `GP-024`.

## Step 2: Baseline architecture inventory
- Commands:
  - `rg -n "type Conversation struct|GetOrCreate\(|RuntimeComposer|ConnectionPool" pinocchio/pkg/webchat -S`
  - `sed -n ... pinocchio/pkg/webchat/conversation.go`
  - `sed -n ... pinocchio/pkg/webchat/router.go`
- Findings:
  - `GetOrCreate` composes runtime engine first (`runtimeComposer.Compose`) and only later creates `ConnectionPool` for new conversations.
  - Stream callbacks directly call `conv.pool.Broadcast(...)`.
  - Route handling and policy orchestration are centralized in router code.

## Step 3: Verify emission coupling points
- Commands:
  - `sed -n ... pinocchio/pkg/webchat/timeline_upsert.go`
  - `nl -ba .../timeline_upsert.go`
- Findings:
  - Timeline upsert websocket emission is coupled directly to `conv.pool.Broadcast`.
  - This confirmed the need for a publisher primitive that can be consumed by extension points without exposing the pool.

## Step 4: Confirm runtime composition/app integration boundaries
- Commands:
  - `sed -n ... pinocchio/pkg/webchat/runtime_composer.go`
  - `sed -n ... pinocchio/cmd/web-chat/runtime_composer.go`
  - `sed -n ... pinocchio/pkg/webchat/types.go`
- Findings:
  - App runtime composition is already callback-driven.
  - App code has `convID` in compose request, but no direct pool access.
  - Existing architecture is already close to app-owned policy; HTTP routing is the remaining large ownership knot.

## Step 5: Draft architectural framing
- Decision:
  - Treat the core issue as ownership mismatch, not missing hooks.
  - Position router simplification as moving route ownership to applications and shrinking core to reusable primitives.
- Drafted key thesis:
  - The system currently sits in an unstable middle: central router + many override seams.
  - Cleaner end-state is explicit app handlers + small core toolkit.

## Step 6: Write design document (textbook style)
- Wrote: `design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Included:
  - narrative context sections,
  - explicit goals/non-goals,
  - architecture diagrams,
  - minimal API sketches,
  - pseudocode for app assembly,
  - phased cutover plan,
  - testing strategy and open questions.
- Important explicit policy:
  - no backward compatibility assumptions in the cutover plan.

## Step 7: Produce detailed diary and align ticket docs
- Wrote this diary file with command-level and finding-level trace.
- Planned update scope for ticket files:
  - `tasks.md` should reflect analysis completion and next implementation planning tasks.
  - `index.md` should point to design + diary explicitly.
  - `changelog.md` should record analysis completion and design direction.

## Step 8: Prepare reMarkable upload package
- Target bundle contents:
  1. analysis/design document,
  2. diary,
  3. tasks,
  4. index.
- Planned location: `/ai/2026/02/15/GP-025/`.

## Step 9: Quality checks performed during authoring
- Ensured analysis answers key architectural question explicitly:
  - whether pool exists at engine creation time (no for first-create).
- Ensured proposal avoids adding extra protocol complexity.
- Ensured diagrams and pseudocode are aligned with actual file structure.

## Step 10: Follow-up implementation topics identified
- If implementation proceeds, first coding ticket slice should define:
  1. `ConversationService` minimal API,
  2. conversation-scoped `WSPublisher`,
  3. app-owned handler templates for `/chat` and `/ws` in `cmd/web-chat`.

## Step 11: Upload bundle to reMarkable
- Commands:
  - `remarquee upload bundle --dry-run <index/design/diary/tasks/changelog> --name \"GP-025 Webchat Toolkit Refactor Analysis\" --remote-dir \"/ai/2026/02/15/GP-025\" --toc-depth 2`
  - `remarquee upload bundle <same inputs> --name \"GP-025 Webchat Toolkit Refactor Analysis\" --remote-dir \"/ai/2026/02/15/GP-025\" --toc-depth 2`
  - `remarquee cloud ls \"/ai/2026/02/15/GP-025/\" --long --non-interactive`
- Results:
  - Dry-run validated bundle content and rendering path.
  - Upload succeeded with document name `GP-025 Webchat Toolkit Refactor Analysis`.
  - Cloud listing confirmed file presence in the target folder.

## Step 12: Expand task plan for implementation cutover
- Trigger: follow-up request to prepare detailed actionable tasks for execution.
- Action:
  - Replaced the high-level `tasks.md` with a detailed phased plan.
  - Added explicit workstreams for:
    - no-compat clean cutover contract,
    - `ConversationService` + `WSPublisher` refactor,
    - app-owned `/chat` and `/ws` migration in `cmd/web-chat`,
    - `web-agent-example` migration.
- Result:
  - Ticket now contains execution-ready tasks rather than planning placeholders.
  - Changelog updated to reflect the task-plan expansion.

## Step 13: Lock clean-cutover decision in design contract
I started execution from the first open GP-025 implementation task and treated the design contract as a hard prerequisite for code movement. The first slice needed to explicitly remove ambiguity about compatibility strategy before refactoring API ownership boundaries.

This step updated the design text to state a locked no-adapter migration policy and checked off task 6 so all following tasks can assume clean cutover semantics.

### Prompt Context

**User prompt (verbatim):** "Work on docmgr ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP and work on each task one by one, committing and checking it off and keeping a frequent diary."

**Assistant interpretation:** Execute GP-025 tasks sequentially, with a commit and task check for each completed task, and update the diary continuously.

**Inferred user intent:** Drive the ticket to completion with strong traceability between implementation steps, tasks, and commits.

**Commit (code):** pending — "docs(gp-025): lock clean-cutover decision note"

### What I did
- Added a dedicated "Locked Decision" section to the GP-025 design doc stating clean cutover only and no compatibility adapter layer.
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 6` to mark task 6 done.

### Why
- Later refactor slices need a frozen migration rule so we do not accidentally preserve router-owned paths through temporary shims.

### What worked
- The design now contains an explicit, auditable decision note tied to the task checklist.
- Task tracking reflects completion of the first open Phase 1 item.

### What didn't work
- N/A

### What I learned
- Even when non-goals mention compatibility, a separate locked decision statement reduces interpretation risk during implementation.

### What was tricky to build
- The subtle part was avoiding restating existing non-goals and instead writing an unambiguous operational rule that downstream refactor steps can enforce.

### What warrants a second pair of eyes
- Confirm that the wording of the locked decision is strict enough to reject any transitional adapter proposals during review.

### What should be done in the future
- Record commit hashes directly in each diary step once commits are finalized.

### Code review instructions
- Start at the GP-025 design doc decision section and verify it explicitly forbids compatibility adapters.
- Validate task state with `docmgr task list --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP`.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `6`

## Step 14: Freeze the minimal core cutover surface
After locking the no-compatibility rule, I needed a single section that defines exactly which core APIs are in-scope for migration. This prevents scope drift before coding starts.

I added a frozen-surface declaration and checked task 7 so method-level contract tasks can now be completed one by one.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Continue sequential ticket execution by locking API scope before implementation.

**Inferred user intent:** Ensure refactor work follows an explicit, versioned contract instead of ad-hoc API expansion.

**Commit (code):** pending — "docs(gp-025): freeze minimal core cutover surface"

### What I did
- Added a `Frozen Cutover Surface (Locked)` subsection to the design doc.
- Enumerated the five allowed surface entries (`ConversationService` constructor + three methods + `WSPublisher.PublishJSON`).
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 7`.

### Why
- Parent task 7 requires a contract-level freeze independent of any specific method signature detail.

### What worked
- The design now has a single canonical list for the upcoming method-level freeze tasks.

### What didn't work
- N/A

### What I learned
- Separate parent-level scope freeze from method-level details to keep task boundaries clear.

### What was tricky to build
- The challenge was writing task-7 content that is meaningful without duplicating all method signatures that belong to tasks 8–12.

### What warrants a second pair of eyes
- Confirm whether the frozen-surface wording is strict enough to block accidental API additions during code review.

### What should be done in the future
- Complete tasks 8–12 by locking exact signatures and behavior notes for each listed API.

### Code review instructions
- Review section `7.0` in the GP-025 design doc and compare it with open tasks 8–12.
- Validate checklist state via `docmgr task list --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP`.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `7`

## Step 15: Lock `ConversationService` constructor/config contract
I treated constructor/config as the first concrete API boundary because it determines which responsibilities remain core versus app-owned. Without this lock, later method contracts can silently depend on router-era fields.

This step adds an explicit config shape and constructor signature, then checks off task 8.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Continue methodical task-by-task contract freezing before refactor implementation.

**Inferred user intent:** Keep migration deterministic by freezing dependency injection and lifecycle ownership up front.

**Commit (code):** pending — "docs(gp-025): freeze conversation service constructor contract"

### What I did
- Added `ConversationServiceConfig` fields to the design contract (runtime, subscriber, persistence, tool factories, timeouts).
- Added `NewConversationService(cfg ConversationServiceConfig) (*ConversationService, error)` as the locked constructor signature.
- Documented constructor validation and ownership expectations.
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 8`.

### Why
- Constructor shape governs composability and is the primary cut line between app wiring and core behavior.

### What worked
- The design now has an explicit DI contract instead of a vague “constructor/config object” placeholder.

### What didn't work
- N/A

### What I learned
- Locking constructor dependencies early reduces accidental backsliding into router-owned responsibilities.

### What was tricky to build
- Choosing fields that preserve current behavior without leaking HTTP transport concerns required careful boundary discipline.

### What warrants a second pair of eyes
- Validate that `ToolRegistry` and persistence fields are sufficient for current runtime and timeline flows.

### What should be done in the future
- Align implementation types with this config shape in task 17.

### Code review instructions
- Review section `7.1` and compare to existing `Router` dependency fields for migration parity.
- Confirm task 8 is checked in `tasks.md`.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `8`

## Step 16: Lock `ResolveAndEnsureConversation(...)` request/handle semantics
With constructor scope fixed, the next critical contract is the method that bridges app request policy into conversation lifecycle. This method defines canonical normalization and conversation existence guarantees.

I added explicit request/handle types, method signature, and behavior notes, then checked task 9.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Continue GP-025 checklist execution by freezing each required service API one at a time.

**Inferred user intent:** Make the migration sequence auditable and enforceable through concrete contracts before code changes.

**Commit (code):** pending — "docs(gp-025): freeze resolve-and-ensure contract"

### What I did
- Added `AppConversationRequest` and `ConversationHandle` contract types.
- Locked `ResolveAndEnsureConversation(ctx, req)` method signature in the design.
- Documented normalization behavior for empty `conv_id` and `runtime_key`.
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 9`.

### Why
- The app-owned `/chat` and `/ws` handlers both depend on a deterministic “ensure conversation” method.

### What worked
- Contract now clarifies exactly what callers can expect after resolve/ensure succeeds.

### What didn't work
- N/A

### What I learned
- Making normalization behavior explicit in the contract reduces hidden coupling to current resolver behavior.

### What was tricky to build
- Needed to balance fidelity to existing runtime behavior with cleaner service-level semantics that are independent of HTTP parsing.

### What warrants a second pair of eyes
- Confirm whether `ConversationHandle` should include additional fields for debug/timeline workflows.

### What should be done in the future
- Map this contract directly onto `ConvManager.GetOrCreate` during task 17 implementation.

### Code review instructions
- Review section `7.2` and verify contract clarity for both `/chat` and `/ws` handler usage.
- Confirm task 9 is checked in ticket tasks.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `9`

## Step 17: Lock `SubmitPrompt(...)` lifecycle contract
After resolve/ensure, prompt submission is the highest-risk behavior because it touches queueing, idempotency, runtime start, and client-visible status semantics. I froze this method next to avoid accidental behavior changes during refactor.

This step added explicit input/result shapes and status guarantees for submission behavior, then checked task 10.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Keep executing the GP-025 checklist in strict order with committed contract slices.

**Inferred user intent:** Preserve existing runtime semantics while changing architecture, with clear reviewable contracts.

**Commit (code):** pending — "docs(gp-025): freeze submit-prompt contract"

### What I did
- Added `SubmitPromptInput` and `SubmitPromptResult` contract types.
- Locked `SubmitPrompt(ctx, in)` signature.
- Documented queue/idempotency status behavior and required response keys.
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 10`.

### Why
- The `/chat` migration depends on preserving observable enqueue/start semantics while moving ownership to app handlers.

### What worked
- The design now defines transport-neutral submission behavior that HTTP handlers can adapt with stable status mapping.

### What didn't work
- N/A

### What I learned
- Explicitly documenting response-status vocabulary (`queued`/`running`/`started`/`completed`/`error`) makes integration-test expectations easier to derive.

### What was tricky to build
- Avoiding over-coupling to HTTP while still documenting HTTP-facing status implications required a split between service result and adapter behavior.

### What warrants a second pair of eyes
- Validate whether `HTTPStatus` belongs in service result or should stay entirely in handler adapters.

### What should be done in the future
- Reconcile this service contract with existing `/chat` response behavior in `router.go` during task 25.

### Code review instructions
- Review section `7.3` and compare to current `Conversation.PrepareSessionInference` flow.
- Confirm task 10 is checked.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `10`

## Step 18: Lock `AttachWebSocket(...)` contract
Websocket attach behavior is where route ownership and core lifecycle behavior intersect most directly. I froze this method contract before implementation so handler migration can stay mechanical instead of interpretive.

This step defined attach options/signature and locked expected hello/ping/pong behavior at the service boundary, then checked task 11.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Continue sequentially by freezing the websocket lifecycle API before route migration.

**Inferred user intent:** Keep websocket semantics stable while moving HTTP ownership out of core.

**Commit (code):** pending — "docs(gp-025): freeze websocket attach contract"

### What I did
- Added `WebSocketAttachOptions` and `AttachWebSocket(...)` signature.
- Documented attach validation, stream start behavior, and hello/ping/pong expectations.
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 11`.

### Why
- Task 26/27 depend on a clear core websocket contract that app handlers can call directly.

### What worked
- The design now separates HTTP upgrade concerns (app-owned) from connection lifecycle mechanics (service-owned).

### What didn't work
- N/A

### What I learned
- Encoding ping/pong behavior explicitly in the service contract reduces ambiguity when moving reader loops out of router code.

### What was tricky to build
- Needed to keep the contract small while still capturing behavior that currently lives in router handler internals.

### What warrants a second pair of eyes
- Decide whether ping/pong should always be enabled or remain configurable via attach options.

### What should be done in the future
- Implement this method in core and use it from both `cmd/web-chat` and `web-agent-example` websocket handlers.

### Code review instructions
- Review section `7.4` for transport boundary correctness.
- Confirm task 11 is checked in ticket tasks.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `11`

## Step 19: Lock conversation-scoped `WSPublisher` contract
The publisher boundary is the key decoupling point for removing direct `ConnectionPool` access from non-transport code. I froze this contract now so core refactor tasks can replace direct broadcasts without inventing new behavior ad hoc.

This step formalized publisher error semantics and broadcast behavior, then checked task 12.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Continue the task-by-task freeze by locking the publisher API before code migration.

**Inferred user intent:** Ensure middleware and timeline code can emit websocket frames through a stable service abstraction.

**Commit (code):** pending — "docs(gp-025): freeze ws publisher contract"

### What I did
- Added explicit publisher sentinel errors: `ErrConversationNotFound`, `ErrConnectionPoolAbsent`.
- Locked `WSPublisher.PublishJSON(ctx, convID, envelope)` contract and behavior.
- Documented no direct `ConnectionPool` exposure.
- Ran `docmgr task check --ticket GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP --id 12`.

### Why
- Task 19 requires replacing direct pool broadcast calls; a frozen publisher contract is the prerequisite.

### What worked
- The design now states exactly how publish failures should surface to callers.

### What didn't work
- N/A

### What I learned
- Error vocabulary at the interface boundary makes later test design (task 42) much clearer.

### What was tricky to build
- Needed to define useful error modes without overfitting to current internal pool lifecycle details.

### What warrants a second pair of eyes
- Validate whether `ErrConnectionPoolAbsent` should be distinct from not-found in caller-facing behavior.

### What should be done in the future
- Implement publisher tests for not-found/no-pool/success fanout exactly against this contract.

### Code review instructions
- Review section `7.5` and compare against current direct `conv.pool.Broadcast(...)` callsites.
- Confirm task 12 is checked.

### Technical details
- Updated file: `pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Checked task: `12`

## Step 20: Phase 1 completion summary (cutover contract locked)
Phase 1 is complete. I finalized the no-compatibility decision, froze the minimal core service API, froze app-owned `/chat` and `/ws` contracts, and locked the route-ownership boundary so implementation work can proceed without contract churn.

I also executed per-task commits/checkoffs for tasks 6 through 16, then switched diary cadence to phase-level updates per user instruction.

### Prompt Context

**User prompt (verbatim):** (see Step 13)

**Assistant interpretation:** Complete tasks sequentially with commits/checkoffs, while maintaining diary updates at the requested cadence.

**Inferred user intent:** Enforce disciplined execution traceability before starting risky refactor implementation.

**Commit (code):** pending — "docs(gp-025): phase 1 diary checkpoint"

### What I did
- Completed and checked off tasks `6..16`.
- Added/updated design sections for:
  - clean cutover decision,
  - frozen `ConversationService`/`WSPublisher` contracts,
  - frozen app-owned `/chat` and `/ws` handler contracts,
  - frozen route ownership boundary (`pkg/webchat` does not mount `/chat` or `/ws`).
- Recorded per-task commits for the contract-freeze sequence.

### Why
- Core refactor and app migration tasks need a stable contract baseline to avoid mid-implementation API drift.

### What worked
- Task board now shows full Phase 1 completion.
- Design contract is explicit enough to guide implementation tasks 17+.

### What didn't work
- Earlier in Phase 1, diary entries were created per task; cadence has now been adjusted to phase-level updates.

### What I learned
- Contract freezing before code changes made task sequencing straightforward and reviewable.

### What was tricky to build
- Balancing service-level contracts (transport-neutral) with concrete HTTP behavior guarantees required explicit adapter notes.

### What warrants a second pair of eyes
- Confirm contract granularity is sufficient to implement Phase 2 without reopening design decisions.

### What should be done in the future
- Start Phase 2 by introducing `ConversationService` and publisher implementation in `pkg/webchat`.

### Code review instructions
- Verify tasks `6..16` are checked in `tasks.md`.
- Review design sections `5.3`, `7.x`, and `11.x` for consistency with Phase 2 goals.

### Technical details
- Ticket: `GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP`
- Completed phase: `Phase 1`

## Step 21: Phase 2 completion summary (`pkg/webchat` core refactor)
Phase 2 is complete. The core moved from router-centric orchestration toward explicit service primitives, with `ConversationService` and conversation-scoped websocket publishing as first-class APIs.

### What I did
- Added `ConversationService` implementation and tests in `pkg/webchat`.
- Added `WSPublisher` abstraction (`PublishJSON`) and replaced non-transport direct pool broadcasts with publisher calls.
- Split router helper logic out of `router.go` into focused files (`idempotency`, `turn snapshot hook`, app-owned handler adapters).
- Removed router-owned `/chat` and `/ws` registration from `pkg/webchat/router.go`.
- Preserved persistence wiring (timeline/turn stores), explicit runtime composition callbacks, and queue/idempotency behavior.

### Key commits
- `11357a2` introduce conversation service orchestration.
- `27b6b39` add conversation-scoped ws publisher.
- `c47b9ed` route timeline upserts through publisher.
- `afe05f9` split router helpers into focused files.
- `26e84d8` remove router-owned chat/ws registration.
- `3e56b9c` add app-owned handler adapters.
- `ed4ea78` queue semantics regression test.

### Validation
- `go test ./pkg/webchat/... -count=1`

### Technical details
- Primary files:
- `pinocchio/pkg/webchat/conversation_service.go`
- `pinocchio/pkg/webchat/ws_publisher.go`
- `pinocchio/pkg/webchat/app_owned_handlers.go`
- `pinocchio/pkg/webchat/router.go`
- Completed phase: `Phase 2`

## Step 22: Phase 3 completion summary (`cmd/web-chat` app-owned route migration)
Phase 3 is complete. `cmd/web-chat` now owns `/chat` and `/ws` directly, while `pkg/webchat` is used as reusable core (`ConversationService`, `UIHandler`, `APIHandler`).

### What I did
- Mounted app-owned `/chat` and `/ws` handlers in `cmd/web-chat/main.go`.
- Moved websocket hello/ping/pong behavior into app-owned websocket handler path using explicit attach options.
- Re-homed API/UI composition decisions in app mux (`/api/` and `/` mounting).
- Kept profile/runtime policy app-driven through resolver/composer integration in app code.

### Key commits
- `232368e` mount app-owned chat handler in `cmd/web-chat`.
- `15e874a` mount app-owned websocket handler in `cmd/web-chat`.
- `b730c84` make ping/pong behavior explicit in app-owned path.
- `aeb6ab2` compose UI/API mounts in app router.
- `c6587c5` validate route wiring with focused smoke tests.

### Validation
- `go test ./cmd/web-chat/... -count=1`
- `go test ./pkg/webchat/... -count=1`

### Technical details
- Primary files:
- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/cmd/web-chat/profile_policy.go`
- `pinocchio/cmd/web-chat/profile_policy_test.go`
- Completed phase: `Phase 3`

## Step 23: Phase 4 completion summary (`web-agent-example` migration)
Phase 4 is complete. `web-agent-example` no longer relies on router-owned `/chat` and `/ws`, and now validates live websocket/timeline behavior through an integration test that uses app-owned handler wiring.

### What I did
- Completed dependency audit and defined target app-owned handlers.
- Wired `ConversationService` explicitly in `web-agent-example` bootstrap.
- Moved resolver/runtime policy usage fully into app-owned request handling.
- Removed reliance on router mux route registration for `/chat` and `/ws` by introducing app mux composition using `APIHandler`/`UIHandler`.
- Added integration validation for live `/chat` + `/ws` + `/api/timeline` flow.

### Key commits (`web-agent-example`)
- `127e54e` define app-owned chat/ws handlers.
- `e384903` wire conversation service in bootstrap.
- `24b6ce2` move resolver policy to app-owned handlers.
- `327b4df` remove router mux ownership of chat/ws routes.
- `a4b5101` validate app-owned chat/ws/timeline flow.

### Validation
- `go test ./...` (in `web-agent-example`)
- Integration test added:
- `web-agent-example/cmd/web-agent-example/app_owned_routes_integration_test.go`

### Technical details
- Primary files:
- `web-agent-example/cmd/web-agent-example/main.go`
- `web-agent-example/cmd/web-agent-example/app_owned_routes_integration_test.go`
- Completed phase: `Phase 4`

## Step 24: Phase 5 completion summary (router simplification + docs cleanup)
Phase 5 is complete. I removed obsolete router route-parameterization options, deleted dead legacy resolver paths tied to monolithic ownership, clarified remaining router helpers as optional utilities, and updated package docs to match the new ownership model.

### What I did
- Removed obsolete router options/state:
- deleted `WithConversationRequestResolver` and `WithWebSocketUpgrader`.
- removed unused `Router` fields and constructor branches that supported those options.
- Removed dead default resolver implementation and tests no longer used after app-owned handler migration:
- deleted `DefaultConversationRequestResolver` + `ConversationLookup` path.
- deleted `pkg/webchat/engine_from_req_test.go`.
- Clarified helper scoping:
- updated comments to make `Handler`/`Handle`/`Mount` utility role explicit.
- added test asserting `APIHandler` does not serve `/chat` or `/ws`.
- Added package docs with current setup guidance:
- new `pkg/webchat/doc.go` describing app-owned `/chat`/`/ws` and optional UI/API helper usage.

### Key commits
- `fcff5b9` remove obsolete router request/ws options.
- `30609fe` remove legacy default request resolver path.
- `a093f94` clarify router helpers as optional utilities.
- `79e5f67` document app-owned route ownership model.

### Validation
- `go test ./pkg/webchat/... -count=1`
- `go test ./cmd/web-chat/... -count=1`
- pre-commit `go test ./...` + lint suite repeatedly passed after each slice.

### Notes
- Observed one intermittent pre-commit failure (`go test ./...` trying to open a missing `cmd/web-chat/web/node_modules/...` path). Immediate retry succeeded without code changes, consistent with prior transient behavior in this workspace.

### Technical details
- Primary files:
- `pinocchio/pkg/webchat/router_options.go`
- `pinocchio/pkg/webchat/types.go`
- `pinocchio/pkg/webchat/router.go`
- `pinocchio/pkg/webchat/engine_from_req.go`
- `pinocchio/pkg/webchat/doc.go`
- `pinocchio/pkg/webchat/server.go`
- `pinocchio/pkg/webchat/router_handlers_test.go`
- Completed phase: `Phase 5`
