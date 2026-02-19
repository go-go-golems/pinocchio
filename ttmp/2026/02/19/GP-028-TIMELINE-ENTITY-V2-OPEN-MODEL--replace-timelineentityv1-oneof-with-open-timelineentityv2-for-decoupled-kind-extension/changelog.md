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
