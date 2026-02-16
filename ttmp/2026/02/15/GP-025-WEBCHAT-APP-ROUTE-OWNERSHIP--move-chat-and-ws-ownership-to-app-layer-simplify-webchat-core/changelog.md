# Changelog

## 2026-02-15

- Initial workspace created.

## 2026-02-15

Added comprehensive analysis and design proposal for simplifying webchat by moving `/chat` and `/ws` route ownership to applications and reducing `pkg/webchat` to reusable primitives.

### Related Files

- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md — Primary analysis and design document
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/reference/01-diary.md — Detailed diary of exploration and writing decisions
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/tasks.md — Task breakdown for analysis completion and follow-up planning

## 2026-02-15

Uploaded bundled GP-025 analysis package to reMarkable for review.

### Related Files

- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/index.md — Included in uploaded bundle
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md — Included in uploaded bundle
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/reference/01-diary.md — Included in uploaded bundle

## 2026-02-15

Expanded `tasks.md` into a detailed cutover execution plan covering clean no-compatibility migration, `ConversationService`/publisher refactor, `cmd/web-chat` app-owned route migration, and `web-agent-example` migration.

### Related Files

- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/tasks.md — Detailed implementation task plan with phased work breakdown

## 2026-02-15

Completed Phase 1 contract freeze for clean cutover:
- Locked no-compatibility decision.
- Frozen `ConversationService`/`WSPublisher` surface.
- Frozen app-owned `/chat` and `/ws` contracts and route boundary.

### Related Files

- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/tasks.md

## 2026-02-15

Completed Phase 2 core refactor in `pkg/webchat`:
- Implemented `ConversationService` and conversation-scoped `WSPublisher`.
- Removed router-owned `/chat` and `/ws`.
- Routed non-transport timeline fanout through publisher API.

### Related Files

- pinocchio/pkg/webchat/conversation_service.go
- pinocchio/pkg/webchat/ws_publisher.go
- pinocchio/pkg/webchat/router.go
- pinocchio/pkg/webchat/app_owned_handlers.go

## 2026-02-15

Completed Phase 3 and 4 app migrations:
- `cmd/web-chat` now mounts app-owned `/chat` and `/ws` routes.
- `web-agent-example` now mounts app-owned `/chat` and `/ws` and validates live ws/timeline behavior.

### Related Files

- pinocchio/cmd/web-chat/main.go
- pinocchio/cmd/web-chat/profile_policy.go
- web-agent-example/cmd/web-agent-example/main.go
- web-agent-example/cmd/web-agent-example/app_owned_routes_integration_test.go

## 2026-02-15

Completed Phase 5 simplification pass:
- Removed obsolete router route-policy options and dead legacy resolver code paths.
- Clarified remaining router helpers as optional utilities.
- Added package docs for app-owned ownership model.

### Related Files

- pinocchio/pkg/webchat/router_options.go
- pinocchio/pkg/webchat/engine_from_req.go
- pinocchio/pkg/webchat/router.go
- pinocchio/pkg/webchat/doc.go

## 2026-02-15

Completed Phase 6 validation matrix:
- Added `ConversationService` and `WSPublisher` unit coverage.
- Added app-owned `/chat` and `/ws` integration tests for `cmd/web-chat`.
- Ran focused backend test matrix and frontend build checks.

### Related Files

- pinocchio/pkg/webchat/conversation_service_test.go
- pinocchio/pkg/webchat/ws_publisher_test.go
- pinocchio/cmd/web-chat/app_owned_chat_integration_test.go
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/tasks.md

## 2026-02-15

Completed Phase 7 documentation finalization:
- Updated design doc with as-built signatures and implementation deltas.
- Updated phase diary entries through Phase 7 progress.

### Related Files

- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/design/01-webchat-toolkit-app-owned-routes-analysis.md
- pinocchio/ttmp/2026/02/15/GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP--move-chat-and-ws-ownership-to-app-layer-simplify-webchat-core/reference/01-diary.md

## 2026-02-15

Ticket closed

