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
- [ ] `ConversationService` constructor/config object.
- [ ] `ConversationService.ResolveAndEnsureConversation(...)`.
- [ ] `ConversationService.SubmitPrompt(...)`.
- [ ] `ConversationService.AttachWebSocket(...)`.
- [ ] `WSPublisher.PublishJSON(...)` conversation-scoped API.
- [ ] Freeze app-owned handler contracts:
- [ ] `/chat` request parsing, validation, response schema, and status codes.
- [ ] `/ws` connect requirements and websocket hello/ping/pong behavior.
- [ ] Freeze route ownership boundary: `pkg/webchat` does not register `/chat` or `/ws`.

## Phase 2: Core Refactor in `pkg/webchat`

- [ ] Introduce new `ConversationService` type and move relevant orchestration from router-centric path.
- [ ] Introduce conversation-scoped websocket publisher service that does not expose `ConnectionPool`.
- [ ] Replace direct `conv.pool.Broadcast(...)` calls in non-transport code with publisher usage.
- [ ] Move or split reusable pieces from `router.go` into focused files/types.
- [ ] Delete or deprecate monolithic route registration from `router.go` (clean cutover target: remove route ownership completely).
- [ ] Keep persistence wiring (timeline/turn store integration) available through service config.
- [ ] Keep request/runtime composition callback integration explicit and app-driven.
- [ ] Ensure stream coordinator and queue/idempotency behavior remain intact after refactor.

## Phase 3: `cmd/web-chat` Migration to App-Owned Routes

- [ ] Implement app-owned `/chat` handler in `cmd/web-chat` using `ConversationService`.
- [ ] Implement app-owned `/ws` handler in `cmd/web-chat` using `ConversationService`.
- [ ] Move websocket hello/ping/pong handling to app-owned path while reusing core helpers where sensible.
- [ ] Wire app profile/runtime policy directly in handlers and runtime composer without router indirection.
- [ ] Re-home timeline API and debug API mounting decisions in app code.
- [ ] Validate that web frontend endpoints still function with the new app-owned route wiring.

## Phase 4: `web-agent-example` Migration Plan and Execution

- [ ] Audit existing `web-agent-example` dependencies on `pkg/webchat` router route ownership.
- [ ] Define target app-owned handlers for `web-agent-example` (`/chat`, `/ws`, optional debug routes).
- [ ] Implement `ConversationService` wiring in `web-agent-example` main/bootstrap path.
- [ ] Move resolver/runtime policy usage to app-owned request handling code.
- [ ] Remove reliance on legacy router-owned `/chat` and `/ws`.
- [ ] Validate live conversations, websocket updates, and timeline upserts in `web-agent-example` after migration.

## Phase 5: Router Simplification and Deletion Pass

- [ ] Remove obsolete router options/interfaces that only existed to parameterize app route behavior.
- [ ] Remove dead code paths and tests tied to old monolithic route ownership.
- [ ] If any router helper remains, ensure it is clearly scoped as optional utility, not central architecture.
- [ ] Update package docs to reflect new ownership model and remove old setup guidance.

## Phase 6: Validation and Test Matrix

- [ ] Add/adjust unit tests for `ConversationService` lifecycle APIs.
- [ ] Add/adjust tests for `WSPublisher` behavior (conversation not found, no pool, successful fanout).
- [ ] Add integration tests for app-owned `/chat` flow in `cmd/web-chat`.
- [ ] Add integration tests for app-owned `/ws` flow in `cmd/web-chat`.
- [ ] Add integration tests for migrated `web-agent-example` chat/ws flow.
- [ ] Run focused backend tests:
- [ ] `go test ./pinocchio/pkg/webchat/...`
- [ ] `go test ./pinocchio/cmd/web-chat/...`
- [ ] `go test ./web-agent-example/...`
- [ ] Run relevant frontend checks for impacted apps.

## Phase 7: Documentation and Handoff

- [ ] Update GP-025 design doc with final API signatures and implementation deltas.
- [ ] Keep diary updated for each implementation slice and test run.
- [ ] Update changelog after each major cutover slice.
- [ ] Upload final implementation package to reMarkable after cutover completion.
