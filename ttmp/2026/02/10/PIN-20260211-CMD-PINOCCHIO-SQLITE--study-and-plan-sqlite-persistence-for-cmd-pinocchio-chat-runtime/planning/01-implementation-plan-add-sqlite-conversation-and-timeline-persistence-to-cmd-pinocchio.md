---
Title: 'Implementation Plan: Add SQLite conversation and timeline persistence to cmd/pinocchio'
Ticket: PIN-20260211-CMD-PINOCCHIO-SQLITE
Status: active
Topics:
    - pinocchio
    - chat
    - backend
    - analysis
DocType: planning
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/cmds/cmd.go
      Note: Target for runChat persistence bootstrap and handler integration
    - Path: pkg/cmds/cmdlayers/helpers.go
      Note: Target for new timeline/turn sqlite CLI parameters
    - Path: pkg/cmds/run/context.go
      Note: Target for propagation of persistence settings through runtime
    - Path: pkg/ui/runtime/builder.go
      Note: Current UI runtime composition boundary where persistence side-handler can be attached
    - Path: pkg/webchat/timeline_projector.go
      Note: Reference projection/throttle patterns for timeline upserts
    - Path: pkg/webchat/turn_persister.go
      Note: Reference persister design for storing final turns
ExternalSources: []
Summary: Phased implementation plan to add SQLite persistence for conversation turns and timeline snapshots in cmd/pinocchio, reusing existing webchat stores and adding tests, rollout, and operational safeguards.
LastUpdated: 2026-02-11T02:02:00-05:00
WhatFor: Execution blueprint for implementing sqlite persistence in cmd/pinocchio chat.
WhenToUse: Use during implementation and PR review for persistence parity with web-chat.
---


# Implementation Plan

## 1. Goal and acceptance criteria

### Goal

Enable `cmd/pinocchio` chat mode to persist both:

- conversation turn snapshots,
- timeline entities/snapshots,

into SQLite, with behavior and schema compatible with existing `web-chat` persistence infrastructure.

### Acceptance criteria

1. New optional CLI/profile settings can enable persistence via DSN or DB path for:
   - timeline store,
   - turn store.
2. Running `pinocchio ... --chat` with persistence configured writes rows to SQLite for each chat inference run.
3. Turn rows include non-empty `conv_id`, `session_id`, `turn_id`, and phase values (`final`, plus optional intermediate phases if configured).
4. Timeline entities are durably upserted with monotonic versioning and recoverable by snapshot query API or helper.
5. Existing behavior with no persistence flags remains unchanged.
6. Automated tests cover store configuration wiring and at least one persistence write/read path for CLI chat.

## 2. Non-goals for this PR

1. No cross-package extraction/refactor of all persistence code out of `pkg/webchat`.
2. No retention policy/DB vacuum scheduler in v1.
3. No server API endpoints in `cmd/pinocchio` (this is CLI runtime, not HTTP service).
4. No migration of historical non-SQLite artifacts.

## 3. Proposed architecture

## 3.1 High-level approach

Implement a narrow integration in `cmd/pinocchio` runtime that reuses existing store implementations from `pkg/webchat` and adds CLI-specific adapter wiring.

### Core principle

Reuse tested storage types; add minimal glue in run path.

### Components

1. Configuration:
   - extend helper parameters (`pkg/cmds/cmdlayers/helpers.go`) with optional persistence settings.
2. Runtime context:
   - extend `pkg/cmds/run/context.go` with persistence fields.
3. Store bootstrap:
   - in chat run path, resolve DSN/path and open stores.
4. Turn persistence:
   - configure turn persister/snapshot hook during inference builder creation.
5. Timeline persistence:
   - connect event stream to timeline upsert adapter with monotonic versioning.
6. Lifecycle:
   - close stores on run termination.

## 3.2 Data model choices

### Conversation identity

Introduce a per-chat-run `conversationID` (UUID) in CLI path. Keep session ID separate.

Rationale:

- aligns with web-chat schema (`conv_id`),
- supports multiple sessions or future sub-conversations,
- avoids overloading session ID semantics.

### Version strategy for timeline upserts

Use per-conversation monotonic counter (atomic uint64) initialized at 1 and incremented per write.

Rationale:

- deterministic and collision-free,
- easier to reason about than timestamp-only versions,
- consistent with requirement that versions are monotonic per conversation.

### Timeline entity IDs

Prefer event metadata IDs already used in UI flow (`md.ID.String()`) plus suffixes for derived entities (e.g. `:thinking`, `:tool_result`) to mirror existing rendering identity.

## 3.3 Error handling policy

v1 policy: persistence is best-effort unless store initialization fails when explicitly requested.

- initialization failures with explicit settings: return error and fail command start.
- write failures during active inference: log warnings and continue user chat.

Rationale:

- do not break conversation because local DB write fails mid-stream,
- preserve user trust in live interaction.

## 4. Detailed work breakdown

## Milestone 0: baseline test harness and branch safety

### Tasks

1. Add focused test scaffolding in CLI package for persistence-enabled chat path.
2. Confirm existing tests for `pkg/webchat/*sqlite*` pass and can be reused as reference.

### Deliverables

- test file skeletons under `pkg/cmds` and/or `pkg/ui/runtime`.

### Exit criteria

- new tests compile and fail for expected missing behavior.

## Milestone 1: configuration surface in helper layer

### Files

- `pkg/cmds/cmdlayers/helpers.go`
- `pkg/cmds/cmd.go`

### Changes

1. Extend `HelpersSettings` with fields:
   - `TimelineDSN string`
   - `TimelineDB string`
   - `TurnsDSN string`
   - `TurnsDB string`
2. Add parameter definitions with explicit help text (copy style from web-chat router settings).
3. Parse these into run context in `RunIntoWriter`.

### Validation

- command help prints new flags.
- parsing works from both CLI flags and profile YAML.

## Milestone 2: runtime context and store bootstrap

### Files

- `pkg/cmds/run/context.go`
- `pkg/cmds/cmd.go`

### Changes

1. Add persistence fields to `RunContext`:
   - either raw settings (DSN/path) or opened stores.
2. In `runChat`, open stores conditionally:
   - use `webchat.SQLiteTimelineDSNForFile` / `SQLiteTurnDSNForFile` when DB file path provided.
   - create parent dirs as needed.
3. Ensure `defer Close()` for opened stores.

### Validation

- with no settings, behavior unchanged.
- with invalid path/DSN, fail early with actionable error.

## Milestone 3: turn snapshot persistence wiring

### Files

- `pkg/cmds/cmd.go`
- optional new file: `pkg/cmds/persistence.go` for adapters.

### Changes

1. Add/port turn persister bridge logic equivalent to `webchat/turn_persister.go`.
2. Attach persister to engine builder in chat inference path.
3. Optionally attach snapshot hook for phase-based snapshots if intermediate phases are desired.

### Data contract

- `conv_id`: generated conversation ID for chat run.
- `session_id`: resolved from turn/session metadata.
- `turn_id`: seed/input turn ID.
- `phase`: at minimum `final`.

### Validation

- run one prompt in chat mode and verify row in `turns` table.

## Milestone 4: timeline persistence wiring

### Files

- `pkg/ui/backend.go` (or wrapper around `StepChatForwardFunc`)
- optional new files:
  - `pkg/ui/timeline_persistence_adapter.go`
  - `pkg/cmds/timeline_persistor.go`

### Changes

1. Introduce a persistence adapter that observes same events used for UI timeline.
2. Convert events into `timelinepb.TimelineEntityV1` snapshots.
3. Upsert entities into `TimelineStore` with monotonic version.
4. Preserve UI behavior regardless of persistence write success.

### Design note

We have two implementation options:

- Option A: extend `StepChatForwardFunc` to call an injected callback after each entity mutation.
- Option B: add a second Watermill handler subscribed to same topic that independently projects events.

Recommendation: Option B for separation of concerns and testability.

### Validation

- verify assistant/thinking/tool entities appear in timeline table.
- verify updates overwrite same entity IDs.

## Milestone 5: integration tests

### Test categories

1. Unit tests:
   - DSN resolution and config precedence (DSN over DB path).
   - version monotonicity in timeline adapter.
2. Component tests:
   - chat inference event sequence persists expected entities.
   - final turn persisted once per completion path.
3. Failure tests:
   - store write errors do not abort UI inference path.

### Suggested tests

1. `TestRunChat_NoPersistenceFlags_NoStoreOpen`
2. `TestRunChat_WithTurnsDB_PersistsFinalTurn`
3. `TestTimelineAdapter_AssistantThinkingAssistantFlow`
4. `TestTimelineAdapter_VersionMonotonic`
5. `TestPersistenceWriteFailure_DoesNotAbortInference`

### Tooling

- use `t.TempDir()` SQLite DB files,
- assert DB rows using existing store `List/GetSnapshot` APIs.

## Milestone 6: documentation and operator workflow

### Files

- `pkg/doc` or command help docs for new flags
- ticket docs and diary entries

### Content

1. How to enable persistence with `--timeline-db` and `--turns-db`.
2. Default behavior when flags absent.
3. Recommended DSN/path examples.
4. Troubleshooting `SQLITE_BUSY` and file permission issues.

## 5. Proposed code-level blueprint

## 5.1 New/updated structs

### `HelpersSettings` additions

```go
type HelpersSettings struct {
    // existing fields...
    TimelineDSN string `glazed.parameter:"timeline-dsn"`
    TimelineDB  string `glazed.parameter:"timeline-db"`
    TurnsDSN    string `glazed.parameter:"turns-dsn"`
    TurnsDB     string `glazed.parameter:"turns-db"`
}
```

### `RunContext` additions

```go
type PersistenceSettings struct {
    TimelineDSN string
    TimelineDB  string
    TurnsDSN    string
    TurnsDB     string
}

type RunContext struct {
    // existing fields...
    Persistence PersistenceSettings
}
```

## 5.2 Store bootstrap helper

```go
func openChatStores(p PersistenceSettings) (timelineStore webchat.TimelineStore, turnStore webchat.TurnStore, cleanup func(), err error)
```

Behavior:

1. resolve DSN/path precedence,
2. create parent dirs for DB paths,
3. open stores if configured,
4. return combined cleanup function.

## 5.3 Timeline persistence adapter

```go
type TimelineWriter interface {
    Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV1) error
}

type ChatTimelinePersister struct {
    convID string
    store  TimelineWriter
    next   atomic.Uint64
}
```

Methods:

- `OnAssistantDelta(event...)`
- `OnAssistantFinal(event...)`
- `OnThinkingDelta(event...)`
- `OnToolStart/Result/...`

## 5.4 Hooking strategy in runChat

1. Build persister after store open and conversation ID creation.
2. Register extra event handler on `ui` topic that feeds persister.
3. Keep existing UI handler unchanged.

Pseudo-flow:

```go
sess, p, _ := runtime.NewChatBuilder()...BuildProgram()
rc.Router.AddHandler("ui", "ui", sess.EventHandler())
if timelineStore != nil {
    rc.Router.AddHandler("ui-persist", "ui", persistenceHandler)
}
```

This decouples persistence from UI rendering and avoids regressions in timeline widget behavior.

## 6. Migration and compatibility

### Backward compatibility

- Default path (no flags) remains in-memory and unchanged.
- New flags are additive.

### Schema compatibility

- Reuse existing store migrations in `pkg/webchat`.
- No new schema version table required for v1.

### Operational compatibility

- Users can point CLI and web-chat to same DB if they intentionally share file, but this should be documented as advanced usage.

## 7. Risk matrix and mitigations

1. Risk: high-frequency delta writes inflate DB and reduce performance.
   - Mitigation: throttle timeline delta writes (250ms, same as projector).
2. Risk: inconsistent conversation IDs between sessions.
   - Mitigation: create explicit conversation ID once per chat run and keep in run context.
3. Risk: handler ordering races (UI vs persistence).
   - Mitigation: design persistence as eventually consistent; ordering does not affect user-facing UI.
4. Risk: new flags collide with existing profile configs.
   - Mitigation: use same names/help semantics as web-chat.
5. Risk: persistence errors are silent.
   - Mitigation: structured warning logs with conv/session/entity identifiers.

## 8. Test and validation plan

## 8.1 Unit tests

- Store bootstrap precedence and error handling.
- Conversation ID generation and propagation.
- Timeline version monotonic increments.

## 8.2 Component tests

- Chat run with mocked event sequence writes timeline + turns.
- Verify schema rows via store API.

## 8.3 Manual smoke test script

1. Start chat with persistence flags.
2. Send one prompt that triggers thinking + assistant response.
3. Exit chat.
4. Query DB using sqlite CLI:

```sql
SELECT conv_id, session_id, turn_id, phase, created_at_ms FROM turns ORDER BY created_at_ms DESC LIMIT 5;
SELECT conv_id, entity_id, kind, version FROM timeline_entities ORDER BY version DESC LIMIT 20;
```

5. Confirm non-empty rows and coherent IDs.

## 8.4 Regression checks

- run existing `pkg/ui` tests,
- run existing `pkg/webchat` sqlite tests,
- run command smoke paths without persistence flags to ensure no behavior drift.

## 9. Rollout strategy

### Phase 1 (single PR)

- Add flags,
- Add store bootstrap,
- Persist final turns,
- Persist core timeline entities (assistant/thinking).

### Phase 2 (follow-up)

- Expand tool event persistence richness,
- Add optional retention cleanup command,
- Consider extracting shared persistence package from `pkg/webchat`.

## 10. PR structure recommendation

To keep reviewable diffs, split commits logically:

1. `feat(cmd): add sqlite persistence flags and run context plumbing`
2. `feat(chat): persist turns from cmd/pinocchio chat loop`
3. `feat(chat): persist timeline entities from ui event stream`
4. `test(chat): add sqlite persistence coverage for cmd/pinocchio`
5. `docs(pin-20260211): record study, plan, diary and usage notes`

## 11. Definition of done checklist

1. New flags visible in help and profile parsing.
2. Turn rows written and queryable.
3. Timeline entities written and queryable.
4. Tests added and passing.
5. Diary and changelog updated.
6. Ticket tasks checked off and status moved to review or complete as requested.

## 12. Final recommendation

Implement persistence in `cmd/pinocchio` by reusing `pkg/webchat` SQLite stores now, with CLI-specific adapters for event projection and turn persisting. This achieves practical parity quickly and safely. If team later wants cleaner package boundaries, extract shared persistence into a dedicated package in a follow-up PR after behavior is stable.
