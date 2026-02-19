# Changelog

## 2026-02-19

- Initial workspace created


## 2026-02-19

Added pinocchio-specific architecture analysis and implementation plan for TimelineEntityV2 open-model hard cutover, and populated detailed execution tasks (including `LastSeenVersion` persistence bugfix task).

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/design-doc/01-timelineentityv2-open-model-cutover-plan.md — New design/implementation plan for replacing TimelineEntityV1 closed oneof with TimelineEntityV2 open kind/props payload model
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Detailed pinocchio TODO list including backend/frontend cutover steps and the conversation index version persistence task
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/index.md — Updated ticket summary, key links, and related files


## 2026-02-19

Uploaded the GP-028 design document bundle to reMarkable for review.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/design-doc/01-timelineentityv2-open-model-cutover-plan.md — Source markdown uploaded as `GP-028 TimelineEntityV2 Open Model Cutover Plan.pdf`


## 2026-02-19

Implemented Task 1 (P2): persist timeline progression in conversation index metadata by wiring `LastSeenVersion` through conversation record construction and SQLite entity upsert path.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation.go — Added conversation-level version tracking and projector upsert hook wrapper; `buildConversationRecord` now sets `LastSeenVersion`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go — Updated `Upsert` transaction to maintain `timeline_conversations.last_seen_version` and `has_timeline` as entity versions advance
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation_test.go — Added regression test for `buildConversationRecord` version propagation
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite_test.go — Added regression test proving upsert-driven conversation progress persistence
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/reference/01-diary.md — Added Step 1 implementation diary entry with command outputs and validation notes
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Checked off P2 LastSeenVersion persistence task


## 2026-02-19

Implemented Task 2: added TimelineEntityV2/TimelineUpsertV2/TimelineSnapshotV2 protobuf contracts and regenerated Go/TS bindings.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/proto/sem/timeline/transport.proto — Added V2 open model messages and required protobuf imports
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/sem/pb/proto/sem/timeline/transport.pb.go — Regenerated Go protobuf bindings for timeline V2 messages
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/pb/proto/sem/timeline/transport_pb.ts — Regenerated web-chat TS protobuf bindings for timeline V2 messages
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/web/src/sem/pb/proto/sem/timeline/transport_pb.ts — Regenerated shared web TS protobuf bindings for timeline V2 messages
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/reference/01-diary.md — Added Step 2 implementation diary entry and validation commands
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Checked off protobuf definition/generation task


## 2026-02-19

Implemented backend TimelineEntityV2 cutover for projection/store/upsert/hydration and migrated downstream Go/UI/CLI tests to V2 `kind + props`.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/persistence/chatstore/timeline_store.go — TimelineStore contract now uses `TimelineEntityV2` and `TimelineSnapshotV2`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_projector.go — Projector now writes V2 entities via `kind + props`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_entity_v2.go — Added shared helpers to materialize V2 props from snapshot protos/maps
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_upsert.go — `timeline.upsert` emission switched to `TimelineUpsertV2`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation_service.go — conversation service upsert emission switched to `TimelineUpsertV2`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/http/api.go — timeline HTTP service contract switched to `TimelineSnapshotV2`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/ui/timeline_persist.go — UI timeline persistence writes V2 message props
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/timeline/entity_helpers.go — CLI summarization switched from oneof getters to V2 props mapping
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/app_owned_chat_integration_test.go — Integration assertion updated for V2 `props`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/reference/01-diary.md — Added Step 3 implementation diary entry with failures and fixes
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Checked off backend projection/upsert/hydration tasks


## 2026-02-19

Implemented frontend TimelineEntityV2 decode/mapping cutover and added a timeline renderer registry for app-owned self-contained widget kinds.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/registry.ts — `timeline.upsert` now decodes `TimelineUpsertV2`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts — Replaced oneof-case mapping with V2 `kind + props` mapper
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/ws/wsManager.ts — Timeline hydration decode switched to `TimelineSnapshotV2`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts — Debug websocket bootstrap/upsert decode switched to V2 transport
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.test.ts — Updated fixture payloads to V2 `props` format
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts — Added register/unregister/resolve registry for timeline renderer dispatch
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx — Uses registry-resolved renderers
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/webchat/index.ts — Exports renderer registry APIs for app integration
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Checked off frontend decode/mapping + oneof-removal + test update tasks
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/reference/01-diary.md — Added Step 4 implementation diary entry


## 2026-02-19

Completed final V1 hard cut by removing V1 timeline transport messages from proto and adding a props-normalizer registry so special-case kind handling is registry-driven.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/proto/sem/timeline/transport.proto — Removed `TimelineEntityV1`/`TimelineUpsertV1`/`TimelineSnapshotV1` definitions and stale V1 imports
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/sem/pb/proto/sem/timeline/transport.pb.go — Regenerated Go bindings now V2-only
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/pb/proto/sem/timeline/transport_pb.ts — Regenerated web frontend bindings now V2-only
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/web/src/sem/pb/proto/sem/timeline/transport_pb.ts — Regenerated shared web bindings now V2-only
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/timelinePropsRegistry.ts — Added registry for kind-specific props normalizers (`tool_result`, `thinking_mode`)
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts — Delegates normalization to registry instead of hardcoded per-kind checks
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/webchat/index.ts — Exports props-normalizer registration API for app-level extensions
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/design-doc/01-timelineentityv2-open-model-cutover-plan.md — Added explicit extension rule: new kinds must not require transport proto edits
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Marked remaining V1-hard-cut and extension-rule tasks complete
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/reference/01-diary.md — Added Step 5 diary entry


## 2026-02-19

Added follow-up modularization tasks for thinking-mode explicit-bootstrap extraction (no `init()`), backend/frontend isolation, and enforceable modularity gates.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Added TODOs for explicit bootstrap registration and thinking-mode self-contained module extraction
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/index.md — Reopened ticket status to `active` for follow-up modularization work


## 2026-02-19

Implemented backend thinking-mode modularization with explicit bootstrap registration and registry-only projection dispatch (removed inline `thinking.mode.*` handling from projector).

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_handlers_bootstrap.go — Added `RegisterDefaultTimelineHandlers()` bootstrap with `sync.Once` and test reset helper
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_handlers_thinking_mode.go — Extracted thinking-mode SEM decode/upsert projection handler module
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_handlers_builtin.go — Replaced `init()` registration with explicit builtin registration function
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_projector.go — Removed inline `thinking.mode.*` switch branch so custom dispatch goes through handler registry
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation.go — Wired startup bootstrap via `RegisterDefaultTimelineHandlers()` in `NewConvManager`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_handlers_bootstrap_test.go — Added idempotence + bootstrap-required projection tests


## 2026-02-19

Moved thinking-mode ownership into app-scoped `cmd/web-chat` modules (backend + frontend), removing thinking-mode handlers/normalizers/renderers from `pkg/webchat` and core webchat registries.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/thinkingmode/backend.go — New app-owned backend module that registers SEM translation and timeline projection handlers for `thinking.mode.*`
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/thinkingmode/backend_test.go — Added backend tests for SEM translation and timeline projection registration
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/main.go — Explicit bootstrap for app-owned thinking-mode backend module
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/sem_translator.go — Removed thinking-mode SEM mapping from default core translator registration
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_handlers_bootstrap.go — Core default timeline handlers now register only generic built-ins
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_handlers_thinking_mode.go — Deleted; thinking-mode projection moved to app-owned module
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.tsx — New frontend feature module with explicit registration for SEM handlers, normalizer, and renderer
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.test.tsx — Added frontend tests proving module registration behavior
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/ws/wsManager.ts — Startup bootstrap now explicitly registers thinking-mode frontend module after core SEM handlers
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/registry.ts — Removed thinking-mode SEM projections from core default registry
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/timelinePropsRegistry.ts — Removed thinking-mode normalizer from core built-ins
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts — Removed thinking-mode renderer from core built-ins


## 2026-02-19

Added enforceable modularity acceptance gates for thinking-mode isolation via source-scanning tests.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/thinkingmode/isolation_test.go — Added backend/frontend isolation checks that fail if thinking-mode projection/registration markers leak outside designated module paths
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/tasks.md — Marked modularity acceptance gate task complete


## 2026-02-19

Moved remaining app-owned thinking-mode event contracts out of `pkg/` into `cmd/web-chat/thinkingmode`, and switched thinking-mode SEM payload translation/projection/render decoding to module-local JSON contract handling.

### Related Files

- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/thinkingmode/events.go — Moved typed thinking-mode event definitions from `pkg/inference/events`; replaced `init()` registration with explicit bootstrap helper
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/thinkingmode/backend.go — Updated SEM translation/projection to use module-local JSON payload structs (no shared thinking-mode protobuf wrapper dependency)
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/thinkingmode/backend_test.go — Updated translation tests to use module-local thinking-mode event constructors
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/features/thinkingMode/registerThinkingMode.tsx — Updated frontend SEM projection handlers to parse module-local JSON payloads
- /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/ttmp/2026/02/19/GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL--replace-timelineentityv1-oneof-with-open-timelineentityv2-for-decoupled-kind-extension/reference/01-diary.md — Added implementation diary step for this modularization pass
