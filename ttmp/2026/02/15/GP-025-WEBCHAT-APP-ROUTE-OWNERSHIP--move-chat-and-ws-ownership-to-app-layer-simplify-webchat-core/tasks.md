# Tasks

## Phase 0: Analysis and Ticket Setup

- [x] Create GP-025 ticket workspace in `pinocchio/ttmp`.
- [x] Analyze current `pkg/webchat` architecture and identify core complexity sources.
- [x] Produce textbook-style design and analysis document with diagrams and pseudocode.
- [x] Produce detailed step-by-step implementation diary.
- [x] Upload analysis bundle to reMarkable.

## Phase 1: Finalize Cutover Contract (No Backward Compatibility)

- [x] Add explicit decision note in design doc: clean cutover only, no compatibility adapter layer.
- [x] Freeze minimal core API surface for cutover:
- [x] `ConversationService` constructor/config object.
- [x] `ConversationService.ResolveAndEnsureConversation(...)`.
- [x] `ConversationService.SubmitPrompt(...)`.
- [x] `ConversationService.AttachWebSocket(...)`.
- [x] `WSPublisher.PublishJSON(...)` conversation-scoped API.
- [x] Freeze app-owned handler contracts:
- [x] `/chat` request parsing, validation, response schema, and status codes.
- [x] `/ws` connect requirements and websocket hello/ping/pong behavior.
- [x] Freeze route ownership boundary: `pkg/webchat` does not register `/chat` or `/ws`.

## Phase 2: Core Refactor in `pkg/webchat`

- [x] Introduce new `ConversationService` type and move relevant orchestration from router-centric path.
- [x] Introduce conversation-scoped websocket publisher service that does not expose `ConnectionPool`.
- [x] Replace direct `conv.pool.Broadcast(...)` calls in non-transport code with publisher usage.
- [x] Move or split reusable pieces from `router.go` into focused files/types.
- [x] Delete or deprecate monolithic route registration from `router.go` (clean cutover target: remove route ownership completely).
- [x] Keep persistence wiring (timeline/turn store integration) available through service config.
- [x] Keep request/runtime composition callback integration explicit and app-driven.
- [x] Ensure stream coordinator and queue/idempotency behavior remain intact after refactor.

## Phase 3: `cmd/web-chat` Migration to App-Owned Routes

- [x] Implement app-owned `/chat` handler in `cmd/web-chat` using `ConversationService`.
- [x] Implement app-owned `/ws` handler in `cmd/web-chat` using `ConversationService`.
- [x] Move websocket hello/ping/pong handling to app-owned path while reusing core helpers where sensible.
- [x] Wire app profile/runtime policy directly in handlers and runtime composer without router indirection.
- [x] Re-home timeline API and debug API mounting decisions in app code.
- [x] Validate that web frontend endpoints still function with the new app-owned route wiring.

## Phase 4: `web-agent-example` Migration Plan and Execution

- [x] Audit existing `web-agent-example` dependencies on `pkg/webchat` router route ownership.
- [x] Define target app-owned handlers for `web-agent-example` (`/chat`, `/ws`, optional debug routes).
- [x] Implement `ConversationService` wiring in `web-agent-example` main/bootstrap path.
- [x] Move resolver/runtime policy usage to app-owned request handling code.
- [x] Remove reliance on legacy router-owned `/chat` and `/ws`.
- [x] Validate live conversations, websocket updates, and timeline upserts in `web-agent-example` after migration.

## Phase 5: Router Simplification and Deletion Pass

- [x] Remove obsolete router options/interfaces that only existed to parameterize app route behavior.
- [x] Remove dead code paths and tests tied to old monolithic route ownership.
- [x] If any router helper remains, ensure it is clearly scoped as optional utility, not central architecture.
- [x] Update package docs to reflect new ownership model and remove old setup guidance.

## Phase 6: Validation and Test Matrix

- [x] Add/adjust unit tests for `ConversationService` lifecycle APIs.
- [x] Add/adjust tests for `WSPublisher` behavior (conversation not found, no pool, successful fanout).
- [x] Add integration tests for app-owned `/chat` flow in `cmd/web-chat`.
- [x] Add integration tests for app-owned `/ws` flow in `cmd/web-chat`.
- [x] Add integration tests for migrated `web-agent-example` chat/ws flow.
- [x] Run focused backend tests:
- [x] `go test ./pinocchio/pkg/webchat/...`
- [x] `go test ./pinocchio/cmd/web-chat/...`
- [x] `go test ./web-agent-example/...`
- [x] Run relevant frontend checks for impacted apps.

## Phase 7: Documentation and Handoff

- [x] Update GP-025 design doc with final API signatures and implementation deltas.
- [x] Keep diary updated for each implementation slice and test run.
- [x] Update changelog after each major cutover slice.
- [ ] Upload final implementation package to reMarkable after cutover completion.
