# Tasks

## TODO

- [x] Split app-owned middleware/timeline proto ownership out of shared `proto/sem` module into `cmd/web-chat/proto`:
  - move `proto/sem/middleware/*.proto` and `proto/sem/timeline/middleware.proto` to `cmd/web-chat/proto/...`
  - keep shared/core SEM schemas in root `proto/sem/...`
  - ensure root Buf module excludes `cmd/web-chat/proto` so app schemas are not regenerated into `pkg/sem/pb`
- [x] Add a dedicated `cmd/web-chat/proto` Buf generation pipeline that emits app-owned generated code under `cmd/web-chat` paths:
  - Go output under `cmd/web-chat/thinkingmode/pb`
  - TS output under `cmd/web-chat/web/src/features/thinkingMode/pb`
  - document deterministic commands for regenerating both shared and app-owned proto outputs
- [x] Cut over generated artifact locations and remove stale shared artifacts for moved app-owned schemas:
  - delete `pkg/sem/pb/proto/sem/middleware/*.pb.go`
  - delete `pkg/sem/pb/proto/sem/timeline/middleware.pb.go`
  - delete mirrored TS generated files in shared paths no longer sourced by root proto module
  - verify no runtime imports still point at removed generated paths
- [x] Update public docs to reflect app-owned proto generation ownership split and current TimelineEntityV2 model (remove stale oneof-era guidance in key docs).
- [x] Add an intern-oriented end-to-end tutorial (8+ pages) for building a thinking-mode-style feature module: middleware -> events -> SEM translation -> timeline projection -> React widget registration/testing.

- [x] Follow-up modularization: extract thinking-mode projection into self-contained backend module with explicit bootstrap (no `init()`):
  - create `pkg/webchat/timeline_handlers_thinking_mode.go` containing only thinking-mode SEM decode + `TimelineEntityV2` upsert logic
  - remove thinking-mode cases from `pkg/webchat/timeline_projector.go` switch
  - replace `init()` registration in timeline handlers with explicit bootstrap API (for example `RegisterDefaultTimelineHandlers()` called by router/service startup)
  - ensure startup bootstrap registers both generic built-ins and thinking-mode handler deterministically exactly once
  - add backend tests proving:
    - handler registration occurs through explicit bootstrap path
    - thinking-mode projection still produces expected timeline entities
- [x] Follow-up modularization: isolate thinking-mode frontend behavior into self-contained files and explicit registration:
  - add a thinking-mode frontend module (for example `cmd/web-chat/web/src/features/thinkingMode/timeline.tsx|.ts`) that exports:
    - props normalizer registration
    - renderer registration
    - a single bootstrap function used by app startup
  - stop referring to thinking-mode details from generic mapper/registry files after bootstrap wiring
  - add frontend tests proving thinking-mode still renders correctly after module registration
- [x] Move thinking-mode ownership to `cmd/web-chat` package modules (backend + web), removing app-specific thinking-mode behavior from `pkg/webchat` defaults:
  - backend module: `cmd/web-chat/thinkingmode` registers SEM translation + timeline projection handlers
  - frontend module: `cmd/web-chat/web/src/features/thinkingMode` registers SEM projection + props normalizer + renderer
  - startup bootstrap explicitly wires module registration in app entrypoints (`cmd/web-chat/main.go`, web `wsManager`/storybook scenario bootstrap)
- [x] Modularity acceptance gate: verify thinking-mode references are isolated:
  - add a test/check (or script + test) that fails if `thinking.mode.*` projection logic appears outside thinking-mode module files
  - add a test/check that fails if thinking-mode renderer/normalizer logic is duplicated outside thinking-mode frontend module files
- [x] Move remaining app-owned thinking-mode event contracts out of `pkg/` and into `cmd/web-chat/thinkingmode`, and remove backend/frontend dependence on shared thinking-mode protobuf payload wrappers:
  - `git mv pkg/inference/events/typed_thinking_mode.go -> cmd/web-chat/thinkingmode/events.go`
  - register event factories via explicit module bootstrap path (no `init()`)
  - keep `thinking.mode.*` SEM payloads as module-local JSON object shapes for this app module
  - update backend projection + frontend registration tests to validate module-local payload flow

- [x] Replace closed `TimelineEntityV1` oneof with open `TimelineEntityV2` transport model in `proto/sem/timeline/transport.proto`
- [x] Define and generate protobuf messages for `TimelineEntityV2`, `TimelineUpsertV2`, and `TimelineSnapshotV2` (Go + TS)
- [x] Update backend projection path to construct/store `TimelineEntityV2` instead of `TimelineEntityV1`:
  - `pkg/webchat/timeline_projector.go`
  - `pkg/persistence/chatstore/timeline_store_sqlite.go`
  - `pkg/persistence/chatstore/timeline_store_memory.go`
- [x] Update backend upsert emission to send V2 payloads under `event.type = timeline.upsert`:
  - `pkg/webchat/timeline_upsert.go`
  - `pkg/webchat/conversation_service.go`
- [x] Update hydration API to return `TimelineSnapshotV2` and remove V1 read/write path
- [x] Update frontend SEM decode + mapper for V2 payloads:
  - `cmd/web-chat/web/src/sem/registry.ts`
  - `cmd/web-chat/web/src/sem/timelineMapper.ts`
  - `cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts`
- [x] Remove oneof-case mapping logic and enforce `kind + props` as canonical frontend render contract
- [x] Update tests for V2-only behavior across backend/frontend/websocket hydration
- [x] Remove V1-specific helper code/comments/docs from active paths (hard cutover, no compatibility)
- [x] Document extension rule in pinocchio docs: new domain kinds must not require transport proto edits after V2
- [x] P2 fix: persist `LastSeenVersion` in conversation index records (issue from `pkg/webchat/conversation.go`):
  - populate `LastSeenVersion` in `buildConversationRecord` (`pkg/webchat/conversation.go`)
  - on timeline upsert, update `timeline_conversations.last_seen_version` (and `has_timeline`) in SQLite path (`pkg/persistence/chatstore/timeline_store_sqlite.go`)
  - add regression tests proving conversation listing persists version progression across restart

## Done

- [x] Create pinocchio ticket `GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL`
- [x] Add analysis + implementation-plan document (`design-doc/01-timelineentityv2-open-model-cutover-plan.md`)
- [x] Upload pinocchio GP-028 design doc to reMarkable (`GP-028 TimelineEntityV2 Open Model Cutover Plan.pdf`)
