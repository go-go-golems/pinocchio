---
Title: Diary
Ticket: GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL
Status: active
Topics:
    - architecture
    - backend
    - frontend
    - timeline
    - webchat
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation.go
      Note: Added conversation-level LastSeenVersion tracking and projector upsert hook wrapper
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go
      Note: Updated SQLite timeline upsert path to persist timeline_conversations.last_seen_version progression
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/persistence/chatstore/timeline_store_sqlite_test.go
      Note: Added regression test ensuring Upsert advances conversation index metadata
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation_test.go
      Note: Added test verifying buildConversationRecord includes LastSeenVersion
ExternalSources: []
Summary: Implementation diary for GP-028 task-by-task execution and commits.
LastUpdated: 2026-02-19T10:49:25-05:00
WhatFor: Preserve exact implementation history, commands, failures, and validation for the TimelineEntityV2 cutover ticket.
WhenToUse: Use when reviewing GP-028 progress or continuing implementation from the latest completed task.
---

# Diary

## Goal

Track GP-028 implementation in strict task-by-task slices, with one focused commit per completed task and explicit validation evidence.

## Step 1: Persist conversation LastSeenVersion progression (P2)

This first implementation slice targeted the P2 persistence bug called out in review: `LastSeenVersion` in conversation index records was stale because `buildConversationRecord` never set it and SQLite entity upserts never updated `timeline_conversations`. I implemented both sides in one coherent change so persisted debug metadata remains truthful after restart.

I intentionally handled this before starting TimelineEntityV2 protocol changes because it is an isolated correctness fix with immediate operational impact and minimal dependency on the larger transport migration.

### Prompt Context

**User prompt (verbatim):** "Ok, let's work task by task on the pinocchio work, committing as we go, and keeping a frequent implement diary (in the pinocchio ttmp still)."

**Assistant interpretation:** Begin executing GP-028 incrementally, commit each completed task, and maintain a detailed diary inside pinocchio `ttmp`.

**Inferred user intent:** Keep migration risk low by making progress auditable and reversible in small validated commits.

**User prompt (verbatim):** "While we're at it, also add a task to handle 
pkg/webchat/conversation.go
Comment on lines +208 to +214
	return chatstore.ConversationRecord{
		ConvID:         conv.ID,
		SessionID:      conv.SessionID,
		RuntimeKey:     conv.RuntimeKey,
		CreatedAtMs:    createdAtMs,
		LastActivityMs: lastActivityMs,
		HasTimeline:    conv.timelineProj != nil,
@chatgpt-codex-connector
chatgpt-codex-connector bot
21 minutes ago

P2 Badge Persist timeline version in conversation index records

buildConversationRecord never populates LastSeenVersion, and the SQLite Upsert path only updates timeline_entities/timeline_versions (not timeline_conversations), so persisted conversation rows stay at version 0 even after many SEM upserts. In practice this makes persisted debug metadata stale after restart (you lose progression signals and ordering fidelity for long-running conversations), which undermines the new persisted conversation listing behavior.

Useful? React with 👍 / 👎.
@wesen"

**Assistant interpretation:** Add and then implement a concrete task that makes conversation index version metadata track real timeline progression in both in-memory and SQLite-backed persistence paths.

**Inferred user intent:** Ensure persisted debug listings are operationally accurate, especially across process restarts.

### What I did

- Added conversation-level version tracking:
  - `Conversation.lastSeenVersion uint64` in `pkg/webchat/conversation.go`.
- Updated conversation persistence payload:
  - `buildConversationRecord` now sets `LastSeenVersion` from the conversation struct.
- Wrapped timeline projector upsert callbacks:
  - new `timelineProjectorUpsertHook(...)` updates `conv.lastSeenVersion` whenever projected upserts occur, then forwards to downstream websocket upsert hook.
  - replaced direct hook wiring with wrapper hook in both projector creation paths.
- Updated SQLite store upsert path:
  - `pkg/persistence/chatstore/timeline_store_sqlite.go` now updates/inserts `timeline_conversations` during every timeline entity `Upsert(...)`, advancing `last_seen_version` and setting `has_timeline=1`.
- Added/updated tests:
  - `pkg/webchat/conversation_test.go`: `TestBuildConversationRecord_IncludesLastSeenVersion`.
  - `pkg/persistence/chatstore/timeline_store_sqlite_test.go`: `TestSQLiteTimelineStore_UpsertAdvancesConversationProgress`.
- Validated task with targeted tests:
  - `go test ./pkg/persistence/chatstore ./pkg/webchat`.

### Why

- This resolves stale persisted metadata for conversation listing/debug APIs.
- It also creates a reliable baseline before TimelineEntityV2 migration changes alter projection payload contracts.

### What worked

- The callback wrapper pattern cleanly preserved existing upsert emission behavior while adding in-memory progression tracking.
- SQLite transactional update of `timeline_conversations` kept version metadata in sync with timeline entity/version writes.
- Targeted package tests passed after changes.

### What didn't work

- Initial `go test` run failed in restricted environment with Go build cache permission errors:
  - `open /home/manuel/.cache/go-build/...: permission denied`
- The first retry was interrupted when the turn was aborted.
- Re-ran tests after environment switched to full access; tests then passed.

### What I learned

- Relying only on explicit `UpsertConversation` calls is insufficient for progression metadata; timeline entity upsert path must also maintain conversation-level version state.

### What was tricky to build

- The projector callback was previously optional and tied to websocket emission. Tracking version reliably required a wrapper hook that always runs and only then delegates to the existing hook, without changing stream behavior.

### What warrants a second pair of eyes

- Confirm that updating `timeline_conversations.last_activity_ms` on every timeline entity upsert is desired for debug listing semantics (currently aligned with progression recency).

### What should be done in the future

- Next task slice: begin TimelineEntityV2 transport cutover (`transport.proto` + generated artifacts) and then propagate V2 through projector/store/frontend decode.

### Code review instructions

- Start at:
  - `pkg/webchat/conversation.go`
  - `pkg/persistence/chatstore/timeline_store_sqlite.go`
- Validate tests:
  - `pkg/webchat/conversation_test.go`
  - `pkg/persistence/chatstore/timeline_store_sqlite_test.go`
- Run:
  - `go test ./pkg/persistence/chatstore ./pkg/webchat`

### Technical details

- New wrapper hook behavior:
  - updates `conv.lastSeenVersion` with `max(current, version)`
  - forwards to original timeline upsert hook for websocket emission.
- SQLite `Upsert(...)` now has an extra statement in the existing transaction:
  - `INSERT ... ON CONFLICT` into `timeline_conversations`
  - monotonic update for `last_seen_version`
  - monotonic update for `last_activity_ms`
  - forced `has_timeline=1`.

## Step 2: Add and generate TimelineEntityV2 protobuf contracts

After completing the P2 persistence fix, I moved to the next discrete migration task: define V2 timeline transport messages and regenerate bindings. This commit is intentionally schema-focused and does not yet switch runtime codepaths from V1 to V2.

I kept V1 and V2 side-by-side in `transport.proto` for staged migration. That allows subsequent task commits (projector/store/frontend cutover) to be narrow and testable while still advancing the core decoupling goal.

### Prompt Context

**User prompt (verbatim):** "continue"

**Assistant interpretation:** Continue executing GP-028 task-by-task with one validated commit per completed task slice.

**Inferred user intent:** Maintain forward momentum without bundling too many migration layers into a single risky change.

### What I did

- Updated timeline transport schema:
  - `proto/sem/timeline/transport.proto`
  - added imports for `google.protobuf.Struct` and `google.protobuf.Any`
  - added:
    - `TimelineEntityV2`
    - `TimelineUpsertV2`
    - `TimelineSnapshotV2`
- Regenerated protobuf artifacts scoped to this proto path:
  - `buf generate --path proto/sem/timeline/transport.proto`
- Verified affected backend packages still compile/test:
  - `go test ./pkg/webchat ./pkg/persistence/chatstore`

### Why

- This establishes the open model contract needed for future decoupled kinds.
- It keeps transport evolution and runtime migration as separate commits.

### What worked

- Scoped `buf generate --path ...` completed quickly and touched only expected generated files.
- Go package tests passed after regeneration.

### What didn't work

- N/A for this step.

### What I learned

- The repo supports narrow proto regeneration by path, which is helpful for task-isolated commits.

### What was tricky to build

- The subtle part is balancing hard-cutover intent with incremental delivery: V2 must be introduced first without immediately breaking active V1 runtime paths.

### What warrants a second pair of eyes

- Confirm generated TS outputs in both frontend trees are both intended to remain in sync:
  - `cmd/web-chat/web/src/sem/pb/...`
  - `web/src/sem/pb/...`

### What should be done in the future

- Next task: switch backend projector/store/upsert emission to V2 payloads and update hydration responses accordingly.

### Code review instructions

- Review schema additions in:
  - `proto/sem/timeline/transport.proto`
- Review regenerated outputs:
  - `pkg/sem/pb/proto/sem/timeline/transport.pb.go`
  - `cmd/web-chat/web/src/sem/pb/proto/sem/timeline/transport_pb.ts`
  - `web/src/sem/pb/proto/sem/timeline/transport_pb.ts`
- Validation command:
  - `go test ./pkg/webchat ./pkg/persistence/chatstore`

### Technical details

- Added open payload fields on `TimelineEntityV2`:
  - `props` (`google.protobuf.Struct`)
  - `typed` (`google.protobuf.Any`)
  - `meta` (`map<string,string>`)
- `TimelineUpsertV2` and `TimelineSnapshotV2` mirror existing V1 envelope semantics with V2 entity payload.

## Step 3: Backend hard-cut to TimelineEntityV2 for projection/store/upsert/hydration

This slice moved runtime behavior from V1 transport entities to V2 across backend projection/storage and timeline APIs. I combined closely coupled TODOs (projection path, upsert emission, hydration response) in one commit-sized unit because partial signature changes between these layers do not compile independently.

### Prompt Context

**User prompt (verbatim):** "continue"

**Assistant interpretation:** Continue GP-028 execution task-by-task, commit each completed slice, and keep the diary current.

**Inferred user intent:** Keep momentum while preserving strict implementation traceability.

### What I did

- Switched core timeline store contracts to V2:
  - `chatstore.TimelineStore.Upsert(..., *TimelineEntityV2)`
  - `chatstore.TimelineStore.GetSnapshot(... ) -> *TimelineSnapshotV2`
  - updated both implementations:
    - `pkg/persistence/chatstore/timeline_store_memory.go`
    - `pkg/persistence/chatstore/timeline_store_sqlite.go`
- Migrated projector writes from V1 oneof entities to V2 `kind + props`:
  - `pkg/webchat/timeline_projector.go`
  - added `pkg/webchat/timeline_entity_v2.go` helper for converting typed snapshot protos into `Struct` props consistently.
- Updated custom timeline handler path:
  - `pkg/webchat/timeline_handlers_builtin.go` now upserts V2 entities.
- Updated conversation/router hook types and emitted timeline upsert payloads to V2:
  - `pkg/webchat/conversation.go`
  - `pkg/webchat/conversation_service.go`
  - `pkg/webchat/timeline_upsert.go`
  - `pkg/webchat/router_options.go`
  - `pkg/webchat/types.go`
- Updated hydration service/API surfaces to V2 snapshots:
  - `pkg/webchat/timeline_service.go`
  - `pkg/webchat/http/api.go`
  - `pkg/webchat/router_timeline_api.go`
- Updated CLI/debug/UI code paths expecting V1 snapshots:
  - `cmd/web-chat/timeline/snapshot.go`
  - `cmd/web-chat/timeline/entity_helpers.go`
  - `cmd/web-chat/timeline/verify.go`
  - `pkg/ui/timeline_persist.go`
- Updated tests to assert V2 `props` shape (instead of oneof getters):
  - `pkg/persistence/chatstore/*_test.go`
  - `pkg/webchat/*_test.go`
  - `pkg/ui/timeline_persist_test.go`
  - `cmd/web-chat/app_owned_chat_integration_test.go`

### Why

- This establishes V2 as the active backend contract and removes runtime dependence on `TimelineEntityV1` oneof decoding/encoding in active paths.
- It directly enables domain-specific kinds to flow through without adding new `oneof` members.

### What worked

- Focused package tests passed after migration:
  - `go test ./pkg/persistence/chatstore ./pkg/webchat`
- Full repo tests passed after patching downstream V1 assumptions:
  - `go test ./...`

### What didn't work

- Initial full test run failed in several downstream packages due stale V1 assumptions:
  - `pkg/ui/timeline_persist.go` still writing V1 entities.
  - `cmd/web-chat/timeline/*` still decoding V1 oneof snapshots.
  - integration assertion in `cmd/web-chat/app_owned_chat_integration_test.go` still expected top-level `"message"` instead of `props`.
- Resolved by systematically migrating those paths to V2 `props` reads/writes.

### What I learned

- The decoupled V2 model is straightforward in storage and transport, but many tests had implicit coupling to oneof helper accessors (`GetMessage` etc.). Converting assertions to inspect `props` is the critical cleanup step.

### What was tricky to build

- `protojson` omits zero-values by default; this initially dropped `streaming:false` from V2 props. I switched helper marshaling to `EmitUnpopulated: true` in `timeline_entity_v2.go` so boolean lifecycle flags remain stable in test and UI expectations.

### What warrants a second pair of eyes

- Confirm `EmitUnpopulated: true` on props generation is desired globally for payload consistency (it increases payload verbosity but avoids missing false/zero fields).

### What should be done in the future

- Next slice: update frontend websocket decode/mappers (`registry.ts`, `timelineMapper.ts`, debug timeline ws manager) to consume `TimelineUpsertV2`/`TimelineSnapshotV2` directly, then remove remaining V1 mapper branches.

### Code review instructions

- Start with interface/type changes:
  - `pkg/persistence/chatstore/timeline_store.go`
  - `pkg/webchat/timeline_projector.go`
  - `pkg/webchat/timeline_upsert.go`
- Review V2 helper:
  - `pkg/webchat/timeline_entity_v2.go`
- Review consumer updates:
  - `pkg/ui/timeline_persist.go`
  - `cmd/web-chat/timeline/entity_helpers.go`
  - `cmd/web-chat/app_owned_chat_integration_test.go`
- Validate:
  - `go test ./pkg/persistence/chatstore ./pkg/webchat`
  - `go test ./...`

### Technical details

- Backend now emits `timeline.upsert` with protobuf payload `TimelineUpsertV2`.
- Hydration `/api/timeline` now returns protobuf JSON for `TimelineSnapshotV2`.
- Active Go paths no longer use `TimelineEntityV1`/`TimelineSnapshotV1` symbols (excluding generated protobuf bindings retained temporarily in `transport.proto`).
