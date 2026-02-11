---
Title: 'Study: SQLite persistence architecture for cmd/pinocchio chat'
Ticket: PIN-20260211-CMD-PINOCCHIO-SQLITE
Status: active
Topics:
    - pinocchio
    - chat
    - backend
    - analysis
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/cmds/cmd.go
      Note: Primary cmd/pinocchio chat execution path and current wiring gap
    - Path: pkg/cmds/cmdlayers/helpers.go
      Note: Helper flags surface where sqlite settings must be introduced
    - Path: pkg/cmds/run/context.go
      Note: RunContext currently lacks persistence configuration fields
    - Path: pkg/ui/backend.go
      Note: UI timeline forwarding behavior and event identity source
    - Path: pkg/webchat/router.go
      Note: Reference implementation for timeline/turn store bootstrap and hook wiring
    - Path: pkg/webchat/timeline_store_sqlite.go
      Note: Reusable sqlite timeline store schema and upsert semantics
    - Path: pkg/webchat/turn_store_sqlite.go
      Note: Reusable sqlite turn store schema and querying semantics
ExternalSources: []
Summary: Architecture study of why cmd/pinocchio chat currently does not persist turns/timeline to SQLite, what web-chat already implements, and what reusable seams exist for a low-risk integration.
LastUpdated: 2026-02-11T01:55:00-05:00
WhatFor: Understand persistence gaps and constraints before implementation.
WhenToUse: Use before implementing or reviewing sqlite persistence in cmd/pinocchio chat.
---


# Study

## Executive summary

`cmd/pinocchio` and `cmd/web-chat` share the same lower-level geppetto engine concepts (session, event sinks, tool loop, turn snapshots), but only `web-chat` wires durable stores today. In `web-chat`, timeline hydration and turn snapshots are first-class router concerns with concrete stores (`SQLiteTimelineStore`, `SQLiteTurnStore`) and explicit wiring points (`TimelineProjector`, `SnapshotHook`, `Persister`) in the inference run path.

In contrast, `cmd/pinocchio` chat mode currently creates an event router, starts Bubble Tea UI, streams events into a timeline widget, and returns; no durable store interface exists in its run context, helper layer, or UI runtime builder. This is why users can see transient timeline state while chatting but cannot later query conversation history or snapshots from SQLite like they can in `web-chat`.

The good news is that persistence does not require inventing new storage logic. The project already has production-grade SQLite stores and tests in `pkg/webchat`. The core architecture decision is whether to directly reuse these types in `cmd/pinocchio` run code (fastest path), or extract a thinner shared persistence package (cleaner boundaries, higher refactor cost). For the near-term goal, direct reuse with narrow adapters is the pragmatic choice.

## Scope and study questions

This study answers:

1. Where exactly does `cmd/pinocchio` chat mode execute and emit events?
2. Where do `web-chat` timeline and turn persistence happen?
3. Which pieces are reusable as-is, and where are coupling risks?
4. What minimum wiring is needed so `cmd/pinocchio` stores:
   - conversation turn snapshots,
   - timeline entities/snapshots,
   - session-scoped identity metadata,
   in SQLite with WAL/busy_timeout semantics.

This study does not define final CLI UX text or migration policy for existing local DB files. Those belong to implementation and follow-up review.

## Current architecture baseline

### `cmd/pinocchio` run modes and event lifecycle

Main execution starts in `pkg/cmds/cmd.go` through `PinocchioCommand.RunIntoWriter`, which resolves helper flags and builds a `run.RunContext`.

Important points:

- Run mode selection:
  - blocking (`RunModeBlocking`)
  - interactive (`RunModeInteractive`)
  - chat (`RunModeChat`)
- Chat and interactive modes require an event router and call `runChat`.
- Chat mode forces streaming (`rc.StepSettings.Chat.Stream = true`) so incremental events are available for UI timeline.

In `runChat` (`pkg/cmds/cmd.go`):

1. Start router (`rc.Router.Run(ctx)`).
2. Build UI runtime via `runtime.NewChatBuilder(...).BuildProgram()`.
3. Register UI event handler (`rc.Router.AddHandler("ui", "ui", sess.EventHandler())`).
4. Run handlers and Bubble Tea program.

No persistence object is passed to `RunContext`, `ChatBuilder`, or `EngineBackend`.

### UI timeline behavior in `cmd/pinocchio`

The TUI timeline is fed by `pkg/ui/backend.go` (`StepChatForwardFunc`). This function reads typed geppetto events and emits timeline widget messages (`UIEntityCreated`, `UIEntityUpdated`, `UIEntityCompleted`).

This means timeline state is currently ephemeral and process-local:

- state exists while process runs,
- timeline entities are not persisted to disk,
- no hydration endpoint exists for CLI mode,
- closing CLI process drops in-memory timeline.

### Turn state in `cmd/pinocchio`

A `session.Session` exists in `EngineBackend`, and turns are available in-memory at runtime (`AppendNewTurnFromUserPrompt`, `StartInference`, `handle.Wait()`).

`runEngineAndCollectMessages` stores `rc.ResultTurn` for immediate usage, but there is no generic turn persister configured in CLI paths. The result is one-shot output instead of durable conversation history.

## `web-chat` persistence architecture (existing reusable implementation)

### Router-level persistence settings

`pkg/webchat/router.go` exposes optional settings:

- `timeline-dsn`, `timeline-db`
- `turns-dsn`, `turns-db`

Behavior:

- if DSN provided, open SQLite store directly,
- if path provided, build DSN with WAL and busy timeout,
- otherwise fallback to in-memory timeline store.

This bootstrapping has useful behavior we can mirror:

- creates DB parent directories if missing,
- validates empty DSN/path early,
- centralizes store lifecycle and close path in server/router management.

### Timeline persistence path

Key files:

- `pkg/webchat/timeline_store.go` (interface)
- `pkg/webchat/timeline_store_sqlite.go` (SQLite impl)
- `pkg/webchat/timeline_projector.go` (event-to-entity projection)

Mechanics:

1. SEM frames are consumed per conversation.
2. `TimelineProjector` maps SEM events to `timelinepb.TimelineEntityV1` snapshots.
3. `Upsert` writes entity row + monotonic conversation version.
4. `GetSnapshot` returns full or incremental snapshots.

Storage model:

- `timeline_versions(conv_id, version)`
- `timeline_entities(conv_id, entity_id, kind, created_at_ms, updated_at_ms, version, entity_json)`

Important properties:

- stable entity identity by `conv_id + entity_id`,
- last-write-wins upsert with explicit projection version,
- JSON payload stored as protojson for interoperability,
- ordering by version for hydration replay.

### Turn persistence path

Key files:

- `pkg/webchat/turn_store.go` (interface)
- `pkg/webchat/turn_store_sqlite.go` (SQLite impl)
- `pkg/webchat/turn_persister.go` (bridge from turn to store)
- `pkg/webchat/router.go` (`snapshotHookForConv` and builder wiring)

Mechanics:

1. During inference start, router prepares tool loop builder.
2. Builder gets:
   - `SnapshotHook` (phase snapshots, optional file + store writes),
   - `Persister` (`newTurnStorePersister(..., "final")`).
3. Turn payload serialized to YAML and inserted with:
   - `conv_id`, `session_id`, `turn_id`, `phase`, timestamp, payload.

Storage model:

- single `turns` table with phase and query indexes.
- migration supports historical `run_id -> session_id` rename.

This is robust enough for CLI use without schema changes.

## Gap analysis: why `cmd/pinocchio` cannot persist today

### Gap 1: no persistence settings in helper layer

`pkg/cmds/cmdlayers/helpers.go` contains chat/interactive flags, but no persistence parameters. So users cannot request timeline or turn DB paths from CLI command flags/profiles.

### Gap 2: no persistence fields in run context

`pkg/cmds/run/context.go` has UI and run mode fields only. There is nowhere to carry configured stores or DSNs through command execution.

### Gap 3: no persistence wiring in `runChat`

`pkg/cmds/cmd.go` creates session/backend/program but does not:

- open SQLite stores,
- attach snapshot hook/persister to engine builder,
- project events into durable timeline store.

### Gap 4: event format mismatch risk between paths

`web-chat` timeline projector consumes SEM frames, while `cmd/pinocchio` TUI forwarder consumes typed geppetto events directly via Watermill handler.

This matters because there are two possible integration approaches:

- project from SEM in CLI path too (shared projector, extra translation setup),
- persist from typed events in CLI path (new adapter needed).

Either is viable, but current CLI chat code path has no explicit SEM projection component.

### Gap 5: store lifecycle ownership undefined in CLI

`web-chat` closes stores when server closes (`pkg/webchat/server.go`). CLI chat path currently has no cleanup hook for additional resources beyond router/program cancellation.

## Reuse opportunities

## Reuse opportunity A: SQLite store implementations directly

Best immediate reuse:

- `webchat.NewSQLiteTimelineStore`
- `webchat.NewSQLiteTurnStore`
- `webchat.SQLiteTimelineDSNForFile`
- `webchat.SQLiteTurnDSNForFile`

Pros:

- no schema redesign,
- already tested,
- migration behavior already encoded.

Risk:

- `cmd/pinocchio` imports `pkg/webchat` persistence types, which is conceptually cross-surface coupling.

Given local codebase ownership and urgency, this is acceptable for first iteration.

## Reuse opportunity B: turn persister strategy

`turnStorePersister` is minimal and generic. Equivalent logic can be used in CLI path with small adaptation to available session/conversation IDs.

## Reuse opportunity C: timeline entity schema

Even if CLI uses typed events rather than SEM projector, it should persist `timelinepb.TimelineEntityV1` entities to maintain parity with web hydration consumers.

## Data identity and keying constraints

Persistence quality depends on stable IDs. Current system has:

- turn-level `turn.ID` and session metadata key (`KeyTurnMetaSessionID`),
- event metadata ID in UI forwarder path (`md.ID.String()`).

Required invariants for SQLite:

1. `conv_id` must remain stable per CLI conversation run.
2. `session_id` must be propagated into persisted turns.
3. `entity_id` for timeline upserts must be deterministic across deltas/final for same message.
4. projection version must be monotonic per conversation.

In `web-chat`, version comes from SEM sequence. In CLI, we need a surrogate monotonic counter (atomic increment) or timestamp+counter to avoid collisions and ordering ambiguity.

## Failure modes and operational concerns

### SQLite locking

Long streaming chats generate frequent writes. Without WAL and busy timeout, contention errors can appear during reads/writes. Existing DSN helpers already set:

- `_journal_mode=WAL`
- `_busy_timeout=5000`
- `_foreign_keys=on`

CLI should reuse this exactly.

### Write amplification

Streaming per token can cause high write volume. `TimelineProjector` already throttles `llm.delta` to 250ms in web-chat. If CLI uses direct event persistence, it should implement equivalent throttling.

### Partial failures

If persistence write fails while inference succeeds, we should prefer preserving user-visible chat continuity and log warning-level failures, not abort inference loop. This matches existing snapshot hook behavior in web-chat.

### Storage growth

Turns and timeline entities can grow without bounds. First implementation can leave retention policy to users (DB file ownership), but plan should include follow-up for cleanup/compaction and optional max-history retention.

## Architecture options for CLI integration

### Option 1: direct import of webchat stores + CLI adapters (recommended)

- Add helper flags for timeline/turn DSN/DB.
- Open stores in `runChat` when configured.
- Attach a turn persister/snapshot hook to builder path.
- Persist timeline entities from event handler path using small adapter.

Pros:

- fastest path,
- minimal new schema code,
- low risk due to tested stores.

Cons:

- introduces dependency from CLI command package to `pkg/webchat` persistence package.

### Option 2: extract shared `pkg/persistence` package first

- Move `TimelineStore`, `TurnStore`, SQLite impls out of `pkg/webchat`.
- Wire both web-chat and CLI to new package.

Pros:

- cleaner architecture,
- lower conceptual coupling.

Cons:

- more refactor churn now,
- higher regression risk in web-chat path.

### Option 3: persist only turns in phase 1

- implement turn snapshots first, defer timeline entities.

Pros:

- smaller first patch.

Cons:

- does not satisfy "conversation and snapshots" parity ask,
- no timeline hydration parity for debug/replay use cases.

## Recommendation

Proceed with Option 1 now, then optionally refactor to Option 2 in follow-up once behavior is validated.

Reasons:

- user goal is practical parity now,
- existing code strongly favors reuse,
- new-ticket scope is study + actionable implementation plan, not large architecture surgery.

## Validation criteria derived from study

A complete implementation should satisfy:

1. User can enable SQLite persistence from `cmd/pinocchio` flags/profile.
2. Chat run creates turn rows with non-empty `conv_id`, `session_id`, `turn_id`, `phase`.
3. Timeline entities are persisted for assistant/thinking/tool events with monotonic versions.
4. Persistence failures do not crash active inference unless configured strict mode (not required for v1).
5. Stores close cleanly on exit.
6. Tests cover store opening, write/read behavior, and at least one end-to-end chat persistence path.

## Open questions to settle during implementation

1. Should CLI default to no persistence unless flags are set? (Recommended: yes)
2. Should timeline persistence include user prompt entities too? (Recommended: yes, parity with web-chat behavior)
3. Should file path flags imply auto-mkdir for parent dirs? (Recommended: yes, consistent with web-chat)
4. What is canonical `conv_id` in CLI chat mode?
   - candidate: session ID for single interactive run,
   - alternative: explicit generated conversation ID separate from session.

Recommendation: use generated conversation ID once per chat program start, keep session ID independent.

## Conclusion

The bug is not missing storage primitives; it is missing wiring. `web-chat` already contains the required SQLite stores and surrounding patterns (DSN construction, hook wiring, snapshot semantics). `cmd/pinocchio` can reach feature parity with a focused integration that introduces persistence settings, opens stores, hooks turn snapshots and timeline projections, and enforces safe lifecycle behavior.

This study supports moving directly into implementation with low schema risk and clear testable milestones.
