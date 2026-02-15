---
Title: Diary
Ticket: GP-020-DEBUG-UI-REALTIME-FOLLOW
Status: active
Topics:
    - frontend
    - debugging
    - websocket
    - webchat
    - timeline
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/web/src/debug-ui/api/debugApi.ts
      Note: Exploration source for debug-ui data model and conversation metadata
    - Path: cmd/web-chat/web/src/debug-ui/components/SessionList.tsx
    - Path: cmd/web-chat/web/src/debug-ui/store/uiSlice.ts
      Note: Added follow-mode state/actions/selectors and reconnect token for GP-020 Task 1 (commit 8c13fbe)
    - Path: cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts
      Note: Implements Task 2-4 follow transport core (connect/bootstrap/replay/upsert) in commit b6117d6
    - Path: cmd/web-chat/web/src/sem/registry.ts
      Note: Exploration source for SEM event type support
    - Path: cmd/web-chat/web/src/ws/wsManager.ts
      Note: Exploration source for websocket ordering and buffering semantics
    - Path: pkg/webchat/router.go
      Note: Snapshot hook persists turns but does not websocket-stream full turn snapshots
    - Path: pkg/webchat/sem_translator.go
      Note: Current websocket semantic event classes
    - Path: pkg/webchat/timeline_upsert.go
      Note: Evidence for timeline.upsert websocket emission
    - Path: pkg/webchat/turn_persister.go
      Note: Turn persistence path used for stored snapshots
ExternalSources: []
Summary: Exploration diary for realtime websocket follow planning in debug UI.
LastUpdated: 2026-02-14T12:12:00-05:00
WhatFor: Record how realtime follow architecture and tasks were derived.
WhenToUse: Use when implementing GP-020 or reviewing assumptions around live attach behavior.
---





# Diary

## Goal
Record the exploration process used to define the realtime websocket follow ticket and its implementation sequencing.

## Step 1: Trace websocket and SEM plumbing for reuse in debug UI
I focused on finding existing websocket and protobuf decoding logic that can be reused instead of re-invented. The key constraint was a read-only debug UI that can follow live activity from another tab.

The output of this step is a concrete architecture choice: a debug-ui-specific websocket manager reusing SEM decoding/mapping patterns, with conversation-scoped connect/hydrate/replay lifecycle.

### Prompt Context
**User prompt (verbatim):** "Create a new ticket to integrate it in the UI, and add a detailed implementation guide to the ticket. 

Also make a list of other functionality that might not be integrated in the UI.

Also make a ticket + implementation plan to add realtime websocket connection to the debug UI when selecting a socket, to be able to \"follow\" realtime conversations (happening in another tab).

Upload both implementation plans to the remarkable tablet. Keep a diary while exploring /"

**Assistant interpretation:** Create a second ticket specifically for websocket follow mode, including a detailed implementation plan and diary-backed reasoning.

**Inferred user intent:** Prepare implementation-ready design work so live-follow capabilities can be added without ambiguity and without coupling to legacy behavior.

**Commit (code):** N/A (documentation-only step)

### What I did
- Inspected `src/ws/wsManager.ts` lifecycle (connect, hydrate, buffer, replay, disconnect).
- Inspected `src/sem/registry.ts` for existing SEM event decode and timeline upsert behaviors.
- Inspected debug-ui API layer (`src/debug-ui/api/debugApi.ts`) and conversation summary shape (`ws_connections`).
- Created ticket workspace `GP-020-DEBUG-UI-REALTIME-FOLLOW` and authored detailed implementation plan.

### Why
- Reusing existing websocket and SEM primitives reduces integration risk and implementation time.
- Needed to preserve read-only debug semantics while still applying live updates.

### What worked
- Existing `wsManager` provided a clear hydration-before-replay ordering model.
- Existing SEM registry confirmed support for timeline widgets/custom kinds through proto-backed mapping.

### What didn't work
- N/A during this exploration step (no runtime implementation attempted yet).

### What I learned
- Debug UI has enough conversation metadata today to expose follow affordances (`ws_connections`).
- The main missing piece is not backend capability but debug-ui store lifecycle wiring for live streams.

### What was tricky to build
- The tricky part is merge semantics: combining hydrated snapshots and live stream updates without duplicates or stale overwrites.
- This is why the plan explicitly includes monotonic dedupe keys and a pause mode.

### What warrants a second pair of eyes
- Choice between RTK Query cache patching vs dedicated live overlay slice.
- Whether follow mode URL persistence should be enabled by default.

### What should be done in the future
1. Implement debug websocket adapter first, then wire controls.
2. Add two-tab manual validation workflow to avoid regressions in attach/detach behavior.

### Code review instructions
- Start with websocket lifecycle in `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`.
- Review SEM decode handlers in `pinocchio/cmd/web-chat/web/src/sem/registry.ts`.
- Compare with debug-ui API and store boundaries to validate proposed adapter seam.

### Technical details
- Key design requirement: hydrate once via HTTP, then replay buffered websocket frames by sequence order.
- Read-only guarantee: debug UI sends no control frames/messages; it only subscribes and renders.

## Step 2: Narrow follow scope to generic timeline upserts only
I validated the backend emit path to check whether debug UI can rely directly on projected entities over websocket. The result is that it can: `timeline.upsert` is already emitted after persistence, which matches your desired model.

I also verified the turn/block question: full turn snapshots are not currently streamed over websocket. They are persisted and queried via debug HTTP endpoints, so turn/block websocket streaming should be a later ticket.

### Prompt Context
**User prompt (verbatim):** "yeah, update the document. for now we just want generic timeline entities and upsert them. We'll do debug specific projection and hydration later on. Do we also receive the turns/blocks over websocket (maybe in a debug only mode)?"

**Assistant interpretation:** Update the GP-020 plan to focus on generic timeline upserts only, and confirm whether websocket currently carries full turns/blocks.

**Inferred user intent:** Keep current implementation scope minimal and aligned with what backend already provides, deferring heavier debug-specific streaming designs.

**Commit (code):** N/A (documentation-only step)

### What I did
- Verified websocket projection emit path in `pinocchio/pkg/webchat/timeline_upsert.go`.
- Verified projector persistence ordering in `pinocchio/pkg/webchat/timeline_projector.go`.
- Verified current websocket SEM translator event types in `pinocchio/pkg/webchat/sem_translator.go`.
- Verified turn snapshot persistence path in `pinocchio/pkg/webchat/router.go` (`snapshotHookForConv`) and `pinocchio/pkg/webchat/turn_persister.go`.
- Updated GP-020 design doc and tasks to use bootstrap + `timeline.upsert` follow only.

### Why
- Needed to avoid adding scope for events/turns websocket handling when projected timeline upserts already satisfy the immediate requirement.

### What worked
- Backend architecture cleanly supports this scope: persisted projection + websocket upsert broadcast.

### What didn't work
- Initial file probes used wrong filenames (`snapshots.go`, `turn_store_persister.go`) and failed with `No such file or directory`.
- Corrected to `router.go` snapshot hook and `turn_persister.go`.

### What I learned
- `timeline.upsert` is the right primary stream for debug follow mode right now.
- Full turns/blocks are currently API-backed (`/api/debug/turns`, `/api/debug/turn/...`), not websocket-backed snapshots.

### What was tricky to build
- Distinguishing "semantic event stream includes llm/tool updates" from "full persisted turn snapshots are streamed" required checking both emitters and persistence hooks.

### What warrants a second pair of eyes
- Whether to eventually add a debug-only websocket event type for turn snapshots (`turn.snapshot`) or keep turns strictly HTTP-sourced.

### What should be done in the future
1. Add a follow-up ticket if we want debug-only websocket turn/block streaming.
2. Keep the immediate implementation strictly generic timeline upsert to reduce risk.

### Code review instructions
- Start at `pinocchio/pkg/webchat/timeline_upsert.go` and `pinocchio/pkg/webchat/timeline_projector.go`.
- Confirm non-streaming of full turns in `pinocchio/pkg/webchat/router.go` and `pinocchio/pkg/webchat/turn_persister.go`.
- Review plan/task updates in GP-020 design/tasks docs.

### Technical details
- Upsert emission flow is persist-first then broadcast (`store.Upsert` then `onUpsert` -> `conv.pool.Broadcast`).
- Websocket payloads include `timeline.upsert`, plus SEM llm/tool/log-style frames, but not full turn YAML/blocks snapshots.

## Step 3: Add follow-mode state/actions/selectors in debug-ui store
I started implementation by adding a dedicated follow-state model in the debug-ui store, instead of interleaving follow flags into existing selection fields. This keeps websocket lifecycle control explicit and easier to test.

This step establishes the contract needed by later tasks: a target conversation id, status enum, reconnect token, and explicit pause/resume/start actions. That allows follow behavior to be wired from UI controls and websocket manager hooks without ad-hoc state mutation.

### Prompt Context
**User prompt (verbatim):** "alright, let's implement GP-020 then, task by task, committing as appropriate, keeping a frequent detailed diary, checking tasks off in docmgr."

**Assistant interpretation:** Implement GP-020 incrementally with small commits, update task checkboxes through docmgr, and keep a detailed ongoing diary.

**Inferred user intent:** Execute the follow-mode ticket as auditable engineering work with clear checkpoints and traceability between code, tasks, and docs.

**Commit (code):** `8c13fbe` — "feat(debug-ui): add realtime follow state/actions/selectors"

### What I did
- Extended `uiSlice` in `pinocchio/cmd/web-chat/web/src/debug-ui/store/uiSlice.ts`:
  - Added `FollowStatus` union type (`idle|connecting|bootstrapping|connected|error|closed`).
  - Added `follow` state object (`enabled`, `targetConvId`, `status`, `reconnectToken`, `lastError`).
  - Added actions for lifecycle control: `startFollow`, `pauseFollow`, `resumeFollow`, `stopFollow`, `setFollowTarget`, `setFollowStatus`, `setFollowError`, `requestFollowReconnect`.
  - Added selectors: `selectFollowState`, `selectFollowStatus`, `selectFollowEnabled`, `selectFollowTargetConvId`.
  - Updated `selectConversation` behavior so active follow mode retargets on conversation switch.
- Ran `npm run typecheck` in `pinocchio/cmd/web-chat/web`.
- Marked Task 1 complete with:
  - `docmgr task check --ticket GP-020-DEBUG-UI-REALTIME-FOLLOW --id 1`

### Why
- Follow mode needs explicit state transitions to avoid coupling websocket lifecycle to route components.
- Reconnect token is needed for user-driven reconnect semantics without inventing imperative side channels later.

### What worked
- TypeScript compiled cleanly after state/action additions.
- `docmgr task check` correctly updated ticket task markdown.

### What didn't work
- No hard failures in this step.

### What I learned
- Existing debug-ui store was small and selection-focused, so adding a nested `follow` state shape is low-risk and keeps reducer intent clear.
- Having explicit `FollowStatus` now will simplify consistent status badges across SessionList/AppShell/TimelinePage.

### What was tricky to build
- The main subtlety was deciding how conversation changes should interact with active follow mode.
- I chose to retarget active follow during `selectConversation` to preserve expected “follow currently selected live conversation” behavior and avoid stale sockets.

### What warrants a second pair of eyes
- Retarget-on-selection behavior in `selectConversation` may need UX confirmation if we later allow following a conversation different from currently selected one.

### What should be done in the future
1. Wire websocket manager to these actions and ensure status transitions are deterministic under reconnect/error paths.

### Code review instructions
- Start in `pinocchio/cmd/web-chat/web/src/debug-ui/store/uiSlice.ts`.
- Verify state model and action naming match GP-020 task intent.
- Validate with `cd pinocchio/cmd/web-chat/web && npm run typecheck`.

### Technical details
- Follow state now stores both control flags (`enabled`, `reconnectToken`) and presentation state (`status`, `lastError`) so UI can be stateless about transport internals.

## Step 4: Implement websocket manager core (connect/bootstrap/replay/upsert)
This step implemented the transport engine for follow mode as an isolated debug-ui manager. I kept it decoupled from UI components so lifecycle behavior can be tested directly and reused from a single hook.

The manager now handles canonical timeline bootstrap (`/api/timeline`), websocket buffering before bootstrap completion, deterministic replay ordering, and `timeline.upsert` merge rules with version-based dedupe.

### Prompt Context
**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Execute the next GP-020 tasks with focused commits while preserving traceability in docmgr.

**Inferred user intent:** Build the realtime-follow core first, then layer UI controls and tests on top of a stable transport/cache merge foundation.

**Commit (code):** `b6117d6` — "feat(debug-ui): add timeline follow websocket manager core"

### What I did
- Added `pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts`.
- Implemented conversation-scoped lifecycle:
  - `connect({ convId, basePrefix, dispatch })`
  - `disconnect()`
- Implemented bootstrap and replay:
  - Bootstrap from canonical `GET /api/timeline?conv_id=...`
  - Decode protobuf snapshot (`TimelineSnapshotV1`)
  - Seed RTK Query caches (`getTimeline`, `getEvents`)
  - Buffer websocket frames until bootstrap completes
  - Replay buffered frames sorted by `event.seq`
- Implemented `timeline.upsert` handling:
  - Decode `TimelineUpsertV1` protobuf
  - Convert proto entity to debug-ui `TimelineEntity`
  - Merge into `getTimeline` cache with version checks (skip stale lower versions)
  - Append corresponding `SemEvent` into `getEvents` cache with duplicate suppression
- Ran `npm run typecheck`.
- Marked tasks complete via docmgr:
  - Task 2
  - Task 3
  - Task 4

### Why
- Needed a dedicated manager before wiring UI so follow lifecycle logic remains deterministic and testable.
- Cache patching via RTK Query utilities avoids introducing a second shadow timeline state structure.

### What worked
- Canonical bootstrap and upsert decode pipeline compiled and integrated with existing debug-ui types.
- Version-based dedupe logic now exists in one place (manager) rather than spread across components.

### What didn't work
- No runtime failures in this phase; implementation compiled cleanly.

### What I learned
- Existing `timelineEntityFromProto` helper from the non-debug SEM path can be reused for debug follow mapping with a thin shape conversion layer.
- RTK Query `upsertQueryData` + `updateQueryData` is sufficient for live overlays without introducing a new reducer slice.

### What was tricky to build
- The tricky part was bootstrap/replay ordering with a strict no-fallback policy.
- I handled this by gating all websocket application behind bootstrap completion and tracking a single `highWaterVersion` to drop stale frames.

### What warrants a second pair of eyes
- Replay gating and `highWaterVersion` advancement logic should be reviewed carefully for edge cases when `seq` and `version` differ.
- Event cache growth policy is currently unbounded; this is acceptable for now but may need limits later.

### What should be done in the future
1. Wire manager lifecycle to UI follow controls and status badges (next phase).
2. Add tests that assert replay order, dedupe behavior, and reconnect semantics.

### Code review instructions
- Start in `pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts`.
- Validate connection flow (`connect`), bootstrap path (`bootstrapFromTimeline`), and merge path (`applyEnvelope`).
- Run `cd pinocchio/cmd/web-chat/web && npm run typecheck`.

### Technical details
- Manager intentionally ignores non-`timeline.upsert` websocket frames in this ticket scope.
- Bootstrap source is canonical `/api/timeline` only, consistent with fresh-cutover policy.
