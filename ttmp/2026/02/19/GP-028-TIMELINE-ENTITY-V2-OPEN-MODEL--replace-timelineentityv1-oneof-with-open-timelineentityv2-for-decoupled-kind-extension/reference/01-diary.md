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
LastUpdated: 2026-02-19T10:42:00-05:00
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
