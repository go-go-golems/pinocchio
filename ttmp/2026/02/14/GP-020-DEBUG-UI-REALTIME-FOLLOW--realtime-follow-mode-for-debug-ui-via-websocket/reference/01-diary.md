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
    - Path: cmd/web-chat/web/src/debug-ui/components/AppShell.tsx
      Note: Task 5 app-level follow hook mount and global status badge (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/components/SessionList.tsx
      Note: Task 5/6 follow controls and status chip wiring (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/routes/OverviewPage.tsx
      Note: Task 5 route live status indicator and isLive semantics (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/routes/TimelinePage.tsx
      Note: Task 5 route live status indicator and isLive semantics (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/store/uiSlice.ts
      Note: Added follow-mode state/actions/selectors and reconnect token for GP-020 Task 1 (commit 8c13fbe)
    - Path: cmd/web-chat/web/src/debug-ui/styles/components/AppShell.css
      Note: Task 5 follow badges/chips/styles for status visibility (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/styles/primitives.css
      Note: Task 5 small button primitive for follow controls (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.test.ts
      Note: Task 9 lifecycle and two-tab follow integration coverage (commit dd35d25)
    - Path: cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts
      Note: |-
        Implements Task 2-4 follow transport core (connect/bootstrap/replay/upsert) in commit b6117d6
        Task 7 read-only websocket behavior note and lifecycle integration point (commit c4a7c4c)
    - Path: cmd/web-chat/web/src/debug-ui/ws/useDebugTimelineFollow.ts
      Note: Task 8 lifecycle hook with mount-aware basePrefix connect/bootstrap URLs (commit c4a7c4c)
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

## Step 5: Wire follow controls, status badges, and mount-aware lifecycle hook
This phase connected the follow-mode transport core into the debug UI so operators can actually control live attach behavior. I focused on minimal UI controls in `SessionList`, global status visibility in the shell, and route-level live indicators, while keeping lifecycle logic centralized in one hook.

I also completed the two policy tasks in this phase: read-only behavior and base-prefix correctness. Read-only is enforced by design (no `ws.send` paths), and connect/bootstrap URLs are now derived from runtime mount prefix so follow mode works when the app is served from a non-root base path.

### Prompt Context
**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Finish the next GP-020 task group by wiring user-facing follow controls and lifecycle behavior, then keep ticket docs/checklists in sync.

**Inferred user intent:** Move GP-020 from backend-ready plumbing to usable debug UI behavior with explicit status, reconnect control, and no legacy/fallback behavior.

**Commit (code):** `c4a7c4c` — "feat(debug-ui): wire follow controls and lifecycle"

### What I did
- Added follow controls and status/error rendering in `pinocchio/cmd/web-chat/web/src/debug-ui/components/SessionList.tsx`:
  - `Follow Live`, `Pause Follow`, `Resume Follow`, `Reconnect`
  - Status chip and inline error message
  - Selection behavior retargets follow to the newly selected conversation
- Mounted follow lifecycle hook in `pinocchio/cmd/web-chat/web/src/debug-ui/components/AppShell.tsx` and added a global header status badge (`live: <status>`).
- Added route-level live status badges and `isLive` wiring in:
  - `pinocchio/cmd/web-chat/web/src/debug-ui/routes/TimelinePage.tsx`
  - `pinocchio/cmd/web-chat/web/src/debug-ui/routes/OverviewPage.tsx`
- Added new lifecycle hook `pinocchio/cmd/web-chat/web/src/debug-ui/ws/useDebugTimelineFollow.ts`:
  - Connects/disconnects manager based on follow state and selected/target conversation
  - Uses `basePrefixFromLocation()` to build mount-aware `/ws` and `/api/timeline` paths
  - Supports reconnect via `reconnectToken`
- Added supporting styles in:
  - `pinocchio/cmd/web-chat/web/src/debug-ui/styles/components/AppShell.css`
  - `pinocchio/cmd/web-chat/web/src/debug-ui/styles/primitives.css` (`.btn-sm`)
- Added explicit read-only note in `pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts`.
- Validation:
  - `cd pinocchio/cmd/web-chat/web && npm run typecheck && npm run lint`
  - Git hook `web-check` re-ran these checks during commit and passed.

### Why
- Tasks 5 and 6 required tangible operator controls and visibility into follow lifecycle state.
- Task 8 required path correctness under app mounting; using location-derived base prefix avoids hardcoded-root assumptions.
- Task 7 required confirming read-only semantics end-to-end in the debug UI transport path.

### What worked
- Follow control state transitions are now fully wired from UI -> store -> hook -> websocket manager.
- Status visibility is consistent across shell/session/timeline surfaces.
- Frontend checks passed before and during commit, confirming no type/lint regressions.

### What didn't work
- A temporary Biome warning in `useDebugTimelineFollow.ts` (`lint/correctness/useExhaustiveDependencies`) appeared when reconnect logic referenced nested state indirectly.
- Resolved by capturing `reconnectToken` as a local dependency and using it directly in the effect dependency list.

### What I learned
- A single mount-level hook in `AppShell` keeps follow lifecycle deterministic and prevents duplicated websocket ownership across routes.
- Follow UX is clearer when pause/resume/reconnect are explicit buttons instead of implicit reconnect side effects.

### What was tricky to build
- The sharp edge was preventing reconnect churn while still honoring explicit reconnect requests.
- Symptoms included effect dependency ambiguity and potential duplicate connect attempts if conversation/follow fields changed together.
- I handled it by:
  - Centralizing lifecycle in one `useEffect`
  - Using `reconnectToken` as the explicit reconnect trigger
  - Ensuring cleanup always disconnects and marks status closed

### What warrants a second pair of eyes
- `SessionList` button enable/disable logic around paused follow with conversation switches.
- `useDebugTimelineFollow` cleanup/status behavior during rapid route unmount/remount cycles.
- Whether `isLive` should include `connecting` state or remain limited to `bootstrapping|connected`.

### What should be done in the future
1. Implement Task 9 tests for lifecycle/reconnect/two-tab follow assertions.
2. Consider adding bounded retention policy for `getEvents` cache if long-running follow sessions grow large.

### Code review instructions
- Start in `pinocchio/cmd/web-chat/web/src/debug-ui/ws/useDebugTimelineFollow.ts` for lifecycle ownership.
- Review UI/store integration in `pinocchio/cmd/web-chat/web/src/debug-ui/components/SessionList.tsx` and `pinocchio/cmd/web-chat/web/src/debug-ui/components/AppShell.tsx`.
- Verify route status wiring in `pinocchio/cmd/web-chat/web/src/debug-ui/routes/TimelinePage.tsx` and `pinocchio/cmd/web-chat/web/src/debug-ui/routes/OverviewPage.tsx`.
- Validate with `cd pinocchio/cmd/web-chat/web && npm run typecheck && npm run lint`.

### Technical details
- Connect URL format: `${proto}://${host}${basePrefix}/ws?conv_id=<id>`.
- Bootstrap URL format: `${basePrefix}/api/timeline?conv_id=<id>`.
- Reconnect semantics: `requestFollowReconnect` increments `reconnectToken`; hook observes token and forces disconnect + reconnect.

## Step 6: Add Task 9 lifecycle and two-tab follow tests for timeline upserts
This phase closed the final GP-020 task by adding executable coverage around the websocket follow manager. I wrote integration-style unit tests that exercise the manager against a real RTK store, mocked fetch bootstrap responses, and a controllable mock websocket transport.

The goal was to validate the exact behaviors that are risky in production: bootstrap + buffered replay ordering, mount-aware URL construction, conversation-switch lifecycle teardown, and read-only websocket semantics. This gives us a guardrail for future refactors while preserving the fresh-cutover API contract.

### Prompt Context
**User prompt (verbatim):** (same as Step 3)

**Assistant interpretation:** Finish GP-020 by implementing the final test task and keep ticket/diary/changelog updated as part of the same execution loop.

**Inferred user intent:** Ensure realtime follow is not only implemented but releasable with concrete automated coverage for lifecycle and cross-tab update behavior.

**Commit (code):** `dd35d25` — "test(debug-ui): add follow websocket lifecycle coverage"

### What I did
- Added `pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.test.ts`.
- Implemented a `MockWebSocket` transport with controllable `emitOpen`, `emitMessage`, `emitClose`, `close`, and `send` recording.
- Implemented `createTestStore()` using:
  - `debugApi.reducer` + `debugApi.middleware`
  - `uiSlice.reducer`
- Added test coverage:
  - `replays buffered timeline.upsert after bootstrap (two-tab follow path)`
  - `handles connect-switch lifecycle across conversations`
- Assertions include:
  - mount-aware websocket URL (`/chat/ws`) and bootstrap URL (`/chat/api/timeline`)
  - buffered `timeline.upsert` replay updates timeline entity state and events cache
  - conversation switch closes previous socket and boots a new one
  - status transitions reach `connected` and `closed` as expected
  - no outbound websocket sends in follow mode (`sendCalls.length === 0`)
- Validation commands:
  - `cd pinocchio/cmd/web-chat/web && npx vitest run src/debug-ui/ws/debugTimelineWsManager.test.ts`
  - `cd pinocchio/cmd/web-chat/web && npm run typecheck && npm run lint`

### Why
- Task 9 explicitly required lifecycle and two-tab follow coverage.
- The manager is stateful and sequence-sensitive, so regression risk is highest around connect/bootstrap/replay and conversation switching.

### What worked
- Both new tests passed immediately under Vitest.
- Full frontend typecheck/lint remained green with the new test file included in `tsconfig` scope.
- Pre-commit `web-check` re-ran and passed, confirming no local-only assumptions.

### What didn't work
- No failing test iterations were required in this phase.

### What I learned
- Testing the manager against a real Redux store (instead of action snapshots only) gives much stronger confidence for RTK Query cache mutation behavior.
- A focused mock transport is enough to model two-tab follow semantics without UI rendering tests.

### What was tricky to build
- The tricky edge was preserving deterministic async ordering between websocket open, bootstrap fetch, and buffered message replay.
- I solved this by explicitly controlling socket open timing in the test and sending `timeline.upsert` before bootstrap completion to assert replay semantics.

### What warrants a second pair of eyes
- Mock websocket fidelity vs browser WebSocket behavior for error edge cases (this suite currently focuses on happy-path + close lifecycle).
- Whether we want additional coverage for bootstrap HTTP failure and reconnect-token driven reconnect loops.

### What should be done in the future
1. Add negative-path tests for bootstrap decode/HTTP failures and stale-version upsert drops.
2. Consider a UI-level test harness once React test tooling is introduced for this package.

### Code review instructions
- Start in `pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.test.ts`.
- Check how `MockWebSocket` drives ordering and how assertions read RTK Query cache via endpoint selectors.
- Re-run validation with:
  - `cd pinocchio/cmd/web-chat/web && npx vitest run src/debug-ui/ws/debugTimelineWsManager.test.ts`
  - `cd pinocchio/cmd/web-chat/web && npm run typecheck && npm run lint`

### Technical details
- Test bootstrap payloads are protobuf-JSON compatible (`convId`, `version`, `entities`, oneof `message` payload).
- Timeline cache assertion path:
  - `debugApi.endpoints.getTimeline.select({ convId })(state).data`
- Events cache assertion path:
  - `debugApi.endpoints.getEvents.select({ convId })(state).data`
