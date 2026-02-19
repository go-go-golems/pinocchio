# Tasks

## TODO

- [ ] Replace closed `TimelineEntityV1` oneof with open `TimelineEntityV2` transport model in `proto/sem/timeline/transport.proto`
- [ ] Define and generate protobuf messages for `TimelineEntityV2`, `TimelineUpsertV2`, and `TimelineSnapshotV2` (Go + TS)
- [ ] Update backend projection path to construct/store `TimelineEntityV2` instead of `TimelineEntityV1`:
  - `pkg/webchat/timeline_projector.go`
  - `pkg/persistence/chatstore/timeline_store_sqlite.go`
  - `pkg/persistence/chatstore/timeline_store_memory.go`
- [ ] Update backend upsert emission to send V2 payloads under `event.type = timeline.upsert`:
  - `pkg/webchat/timeline_upsert.go`
  - `pkg/webchat/conversation_service.go`
- [ ] Update hydration API to return `TimelineSnapshotV2` and remove V1 read/write path
- [ ] Update frontend SEM decode + mapper for V2 payloads:
  - `cmd/web-chat/web/src/sem/registry.ts`
  - `cmd/web-chat/web/src/sem/timelineMapper.ts`
  - `cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts`
- [ ] Remove oneof-case mapping logic and enforce `kind + props` as canonical frontend render contract
- [ ] Update tests for V2-only behavior across backend/frontend/websocket hydration
- [ ] Remove V1-specific helper code/comments/docs from active paths (hard cutover, no compatibility)
- [ ] Document extension rule in pinocchio docs: new domain kinds must not require transport proto edits after V2
- [x] P2 fix: persist `LastSeenVersion` in conversation index records (issue from `pkg/webchat/conversation.go`):
  - populate `LastSeenVersion` in `buildConversationRecord` (`pkg/webchat/conversation.go`)
  - on timeline upsert, update `timeline_conversations.last_seen_version` (and `has_timeline`) in SQLite path (`pkg/persistence/chatstore/timeline_store_sqlite.go`)
  - add regression tests proving conversation listing persists version progression across restart

## Done

- [x] Create pinocchio ticket `GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL`
- [x] Add analysis + implementation-plan document (`design-doc/01-timelineentityv2-open-model-cutover-plan.md`)
- [x] Upload pinocchio GP-028 design doc to reMarkable (`GP-028 TimelineEntityV2 Open Model Cutover Plan.pdf`)
