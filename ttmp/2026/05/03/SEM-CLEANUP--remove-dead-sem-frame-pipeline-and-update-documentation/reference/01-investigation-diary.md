---
Title: Implementation Diary
Ticket: SEM-CLEANUP
Status: active
Topics: []
DocType: reference
Intent: ""
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/web/src/debug-ui/components/AppShell.tsx
      Note: Rewritten session ID input + Follow/Disconnect (commit 2992681)
    - Path: cmd/web-chat/web/src/debug-ui/store/debugSlice.ts
      Note: New Redux slice replacing RTK Query debugApi (commit 2992681)
    - Path: cmd/web-chat/web/src/debug-ui/ws/debugWsManager.ts
      Note: New sessionstream WS client replacing dead debugTimelineWsManager (commit 2992681)
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---




# Implementation Diary

## Goal

Remove the dead SEM frame pipeline from the pinocchio codebase and migrate the debug UI to consume the production sessionstream WebSocket. The SEM registry, old debug API layer, obsolete tutorial, and stale documentation all had zero or near-zero production consumers after the webchat migration to sessionstream.

## Step 1: Investigation — Confirm SEM Pipeline Is Dead

The user asked whether SEM frames are still used after the webchat migration to sessionstream, pointing to tutorial 04 as potentially obsolete. A systematic grep search revealed that the entire SEM pipeline had zero production consumers: the Go registry was never imported, the TS registry was only used by a Storybook story, and the debug UI connected to a non-existent `/ws` endpoint.

### Prompt Context

**User prompt (verbatim):** "Investigate whether SEM frames are still used after webchat migration to sessionstream"

**Assistant interpretation:** Search the full codebase for any remaining consumers of the SEM frame pipeline (Go registry, TS registry, debug API, tutorials, documentation) and report what's dead vs. still active.

**Inferred user intent:** Determine if the SEM pipeline can be safely removed or if there are hidden consumers.

### What I did

- `grep -rn` for `RegisterByType`, `semregistry`, `sem/registry`, `handleSem` across Go, TS, and MD files
- Checked `wsManager.ts` (production frontend) for SEM references
- Checked `timelineMapper.ts` and `timelinePropsRegistry.ts` consumers
- Inspected all 4 doc topic files for stale references
- Verified `pkg/sem/pb/` is still widely imported (protobuf-generated types — must not touch)

### What I found

1. **Go SEM registry** (`pkg/sem/registry/registry.go`): Zero consumers. Never imported.
2. **TS SEM registry** (`sem/registry.ts`): Only consumer is `ChatWidget.stories.tsx`.
3. **Tutorial 04**: 1041 lines entirely about the old SEM pipeline. Superseded by tutorial 09.
4. **4 doc topics**: Stale SEM envelope/registry/pipeline references.
5. **`timelineMapper.ts`**: Still used by debug UI — not SEM-specific, just misplaced under `sem/`.
6. **`timelinePropsRegistry.ts`**: Active public export — not SEM-specific, just misplaced under `sem/`.
7. **`pkg/sem/pb/`**: Protobuf types still widely imported. Must not touch.
8. **Debug UI**: Connects to non-existent `/ws?conv_id=` endpoint, all `/api/debug/*` routes missing. Completely broken.

### What worked

- `grep -rn` for import paths and function names gave clear evidence of dead code.
- Checking the Storybook story as the only remaining TS registry consumer was straightforward.

### What was tricky to build

- The debug UI appeared to "use" SEM frames (`debugTimelineWsManager.ts` processed `{ sem: true }` envelopes), but the backend endpoint it connected to didn't exist. Distinguishing "code that references X" from "code that actually works with X" required tracing the full WS connection path.

### What warrants a second pair of eyes

- Confirm that `pkg/sem/pb/` really must stay (it does — protobuf-generated types used by sessionstream hydration).
- Confirm that no external consumer depends on the debug API endpoints.

### What should be done in the future

- Consider renaming `FeatureSet` → `ChatPlugin` in actual Go code (docs already use the new name).
- Consider removing the `sem/` directory entirely once `timelinePropsRegistry.ts` is relocated.

### Code review instructions

- Search for `RegisterByType` and `semregistry` — should return zero results in Go files.
- Search for `from.*sem/registry` — should return zero results in TS/TSX files.

---

## Step 2: Delete Go SEM Registry

Starting the execution phase with the safest possible change: deleting a Go package that has literally zero importers. This builds confidence before tackling the larger TS and debug-UI changes.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Execute Phase 1 of the cleanup plan — delete the dead Go SEM registry package.

**Inferred user intent:** Remove dead code in smallest-safe-steps, committing after each phase.

**Commit (code):** `ccceef6` — "refactor: delete dead Go SEM registry package (pkg/sem/registry/)"

### What I did

- Deleted `pkg/sem/registry/registry.go` (70 lines)
- Ran `make build` — passed
- Ran `go test ./... -count=1` — passed
- Committed

### Why

Zero consumers, zero risk. Starting here validates that the build + test pipeline works and that lefthook pre-commit hooks are green.

### What worked

- Go compiler immediately confirmed no broken imports.
- `make build` + `go test` gave fast feedback.

### What didn't work

- Nothing. Clean deletion.

### What was tricky to build

- N/A — straightforward deletion.

### What warrants a second pair of eyes

- Confirm no `go.sum` or `go.mod` changes were needed (they weren't — the package had no external deps).

### What should be done in the future

- N/A

### Code review instructions

- `git show ccceef6` — single file deletion, 70 lines.

---

## Step 3: Delete TS SEM Registry, Migrate Storybook, Relocate timelinePropsRegistry

This step removed the TypeScript side of the SEM registry, migrated the only consumer (a Storybook story) to populate Redux directly, and moved `timelinePropsRegistry.ts` out of the `sem/` directory into `webchat/` where it belongs.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Commit (code):** `e981ca2` — "refactor: delete TS SEM registry, migrate ChatWidget story to direct Redux, move timelinePropsRegistry to webchat/"

### What I did

- Deleted `sem/registry.ts` + `sem/registry.test.ts` (410 lines)
- Rewrote `ChatWidget.stories.tsx`: replaced `handleSem`/`registerDefaultSemHandlers` with direct Redux store population via `store.dispatch`
- Moved `sem/timelinePropsRegistry.ts` → `webchat/timelinePropsRegistry.ts`
- Updated import in `timelineMapper.ts` (kept temporarily — deleted in Step 6)
- Updated `webchat/index.ts` re-export path
- Ran `npm run check` — passed

### Why

The SEM registry was a pub/sub layer that the production frontend never used. The Storybook story was the sole consumer, and it only needed to populate Redux state — the registry was unnecessary indirection.

### What worked

- TypeScript compiler caught every broken import immediately.
- The Storybook story became simpler: just dispatch actions to set state, no middleware.

### What didn't work

- Had to keep `sem/timelineMapper.ts` temporarily because the debug UI still imported it. Updated its import path to point to the new `timelinePropsRegistry.ts` location, but couldn't delete it until the debug UI rewrite in Step 6.

### What was tricky to build

- The dependency chain: `timelineMapper.ts` → `timelinePropsRegistry.ts` → `debug-ui`. Had to trace that `timelineMapper.ts` was only consumed by the debug UI, which was about to be rewritten. Decided to keep it temporarily and delete in Step 6.

### What warrants a second pair of eyes

- Verify the Storybook story still renders correctly (it populates the same Redux state shape).

### What should be done in the future

- Add a Storybook smoke test that verifies the story renders without errors.

### Code review instructions

- `git show e981ca2` — key files: `ChatWidget.stories.tsx`, `webchat/index.ts`, `sem/timelineMapper.ts` (import path update only)

---

## Step 4: Delete Obsolete Tutorial 04

Tutorial 04 was a 1041-line guide to the old SEM frame pipeline, completely superseded by tutorial 09 (sessionstream-based). This step deleted it and cleaned up the cross-reference.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Commit (code):** `ab46129` — "docs: delete obsolete SEM pipeline tutorial (04), remove cross-reference in 09"

### What I did

- Deleted `pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md` (1041 lines)
- Updated `pkg/doc/tutorials/09-building-sessionstream-react-chat-apps.md`: removed the cross-reference to tutorial 04
- Ran `make build` — passed

### Why

The tutorial described a dead pipeline. Anyone reading it would be building against APIs that no longer exist.

### What worked

- Go `embed.FS` picked up the change automatically on rebuild — no manifest to update.

### What didn't work

- Nothing. Clean deletion.

### What was tricky to build

- N/A

### What warrants a second pair of eyes

- Confirm no other tutorials or docs reference tutorial 04 by slug.

### What should be done in the future

- N/A

### Code review instructions

- `git show ab46129` — two files changed: one deleted, one line removed from tutorial 09.

---

## Step 5: Rewrite Stale Doc Topics

Four documentation topic files still referenced the SEM envelope, SEM registry, and SEM frame pipeline. This step rewrote them to use sessionstream terminology.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Commit (code):** `7f52c12` — "docs: rewrite 4 topic files to replace SEM references with sessionstream"

### What I did

- **Full rewrite**: `webchat-frontend-integration.md`, `webchat-frontend-architecture.md`, `13-js-api-reference.md`
- **Small fix**: `webchat-debugging-and-ops.md` (removed stale SEM debug section)
- Ran `make build` — passed (docs are embedded)

### Why

The docs described an architecture that no longer exists. Developers reading them would be misled about how data flows through the system.

### What worked

- Full rewrites were cleaner than patching individual references — the old docs were deeply SEM-centric.

### What didn't work

- Nothing. Clean edits.

### What was tricky to build

- Had to accurately describe the sessionstream protocol without introducing new inaccuracies. Cross-referenced the actual Go source (`pkg/chatapp/features.go`, sessionstream package) to ensure correctness.

### What warrants a second pair of eyes

- Read through the rewritten docs for factual accuracy, especially the API reference (`13-js-api-reference.md`).

### What should be done in the future

- Consider adding the `ChatPlugin` (renamed from `FeatureSet`) API docs once the Go rename happens.

### Code review instructions

- `git show 7f52c12` — 4 markdown files. Read through for accuracy.

---

## Step 6: Migrate Debug UI from Dead SEM/Debug API to Sessionstream

This was the largest phase. The debug UI was completely broken: it connected to a non-existent `/ws?conv_id=` endpoint, and all `/api/debug/*` REST routes were missing from the server. The rewrite replaced the entire API layer with a sessionstream WebSocket client that subscribes to the same production endpoint (`/api/chat/ws`) that the chat widget uses.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Commit (code):** `2992681` — "feat: migrate debug-ui from dead SEM/debug API to sessionstream"

### What I did

**Deleted (89 files, -8638 lines):**
- `debug-ui/api/` — `debugApi.ts`, `debugApi.test.ts`, `turnParsing.ts`, `turnParsing.test.ts`
- `debug-ui/mocks/` — entire directory (factories, fixtures, scenarios, MSW handlers)
- `debug-ui/ui/format/` — `phase.ts`, `time.ts`, `text.ts`, `format.test.ts`
- `debug-ui/ui/presentation/` — `blocks.ts`, `events.ts`, `timeline.ts`, `presentation.test.ts`
- `debug-ui/ws/debugTimelineWsManager.ts` + `.test.ts`
- 20+ component files: `AnomalyPanel`, `BlockCard`, `ConversationCard`, `CorrelationIdBar`, `EventCard`, `EventInspector`, `FilterBar`, `SessionList`, `SnapshotDiff`, `StateTrackLane`, `TimelineEntityCard`, `TurnInspector`, `NowMarker`, plus all their `.stories.tsx` files
- `debug-ui/components/appShellSync.ts` + `.test.ts`, `turnInspectorState.ts` + `.test.ts`
- `debug-ui/routes/TurnDetailPage.tsx`
- `sem/timelineMapper.ts` (last consumer was debug UI — now deleted)
- `debug-ui/types/index.ts` gutted (was only TurnPhase types for old SEM)

**Created:**
- `debug-ui/store/debugSlice.ts` — Redux slice replacing RTK Query `debugApi`. Stores entities + events from sessionstream.
- `debug-ui/ws/debugWsManager.ts` — Connects to `/api/chat/ws`, sends `{ type: "subscribe", sessionId }`, receives `{ type: "snapshot", entities }` then `{ type: "ui-event", name, payload }` frames.
- `debug-ui/ws/useDebugTimelineFollow.ts` — React hook managing WS lifecycle.

**Rewrote:**
- `debug-ui/store/store.ts` — Replaced `debugApi` middleware with `debugSlice` reducer.
- `debug-ui/store/uiSlice.ts` — Stripped from 100+ lines (conversation/turn/run/phase selection + follow state) to 40 lines (session ID + follow toggle).
- `debug-ui/components/AppShell.tsx` — Session ID text input + Follow/Disconnect button replacing the old conversation list.
- `debug-ui/components/TimelineLanes.tsx` — 2 lanes (UI Events + Entities) replacing old 3-lane SEM layout.
- `debug-ui/components/EventTrackLane.tsx` — Renders named events with sequence numbers.
- `debug-ui/components/ProjectionLane.tsx` — Renders entity cards from sessionstream snapshot.
- `debug-ui/routes/OverviewPage.tsx`, `TimelinePage.tsx`, `EventsPage.tsx`, `routes/index.tsx` — All simplified to read from Redux instead of RTK Query hooks.
- `debug-ui/routes/useLaneData.ts` — Replaced 3 RTK Query calls with Redux selectors.
- `debug-ui/index.ts` — Simplified export.

### Why

The old debug UI was fundamentally broken — it assumed a backend that no longer exists. Rather than fix the broken backend endpoints, the rewrite makes the debug UI a passive observer of the same data the chat widget renders, using the same production WebSocket.

### What worked

- TypeScript compiler caught every broken import during the rewrite — instant feedback loop.
- Biome auto-fixed import sorting issues (`npm run lint:fix`).
- Lefthook pre-commit hooks (lint, test, web-check) validated every commit.
- Deleting entire directories at once (`mocks/`, `api/`) was much faster than file-by-file.
- The 2-lane layout (events + entities) is much simpler than the old 3-lane SEM layout (raw frames + projection + timeline).

### What didn't work

- First `tsc --noEmit` after the rewrite showed ~30 errors from dead component files still referencing deleted types/mocks. Had to delete all the stale components and stories in a second pass.
- The `types/index.ts` file was `export {}` (empty module), causing `TS2306: File is not a module` errors in components that imported `TurnPhase` from it. Fixed by deleting those components entirely.

### What was tricky to build

- **Dependency chain ordering**: `timelineMapper.ts` had to be kept in Step 3 because the debug UI imported it. Only after the debug UI rewrite removed its last consumer could it be deleted in this step.
- **TypeScript module errors**: Components importing from a gutted `types/index.ts` (empty export) caused confusing `TS2306` errors. The fix was deleting the dead components rather than keeping the type file alive.
- **Redux vs RTK Query**: The old store used RTK Query (`debugApi`) with auto-generated hooks. The new store uses a plain slice (`debugSlice`) populated by the WS manager. Routes that used `useGetXxxQuery()` hooks had to be rewritten to use `useAppSelector()` instead.

### What warrants a second pair of eyes

- The WS protocol in `debugWsManager.ts`: confirm the `{ type: "subscribe", sessionId }` message format matches the server's expected subscribe protocol.
- The `debugSlice.ts` entity merging logic: confirm `mergeEntities` correctly handles upserts and deletions from snapshot updates.

### What should be done in the future

- Add a "copy session ID" button that reads from the chat widget's URL parameter.
- Consider adding entity detail expansion (click an entity card to see full JSON).
- Consider adding event filtering by type name.
- The 4 biome warnings (unused imports in `OverviewPage.tsx`, `useDebugTimelineFollow.ts`) should be cleaned up.

### Code review instructions

- Start with `debug-ui/ws/debugWsManager.ts` — this is the core of the rewrite.
- Then `debug-ui/store/debugSlice.ts` — understand the state shape.
- Then `debug-ui/components/AppShell.tsx` — see the UI flow.
- Validate: `cd cmd/web-chat/web && npm run check` (typecheck + lint).
- Validate: `make build` (Go build with embedded frontend).

### Technical details

**Production WS protocol:**
```
connect to /api/chat/ws
send: { "type": "subscribe", "sessionId": "<uuid>" }
receive: { "type": "snapshot", "entities": { "<id>": { ... } } }
receive: { "type": "ui-event", "name": "ChatMessageAppended", "payload": { ... }, "seq": 123 }
```

**Key event names observed:**
- `ChatMessageAccepted` — user message registered
- `ChatReasoningStarted` — reasoning begins
- `ChatReasoningAppended` — reasoning chunk
- `ChatReasoningFinished` — reasoning complete
- `ChatMessageStarted` — assistant message begins
- `ChatMessageAppended` — content chunk
- `ChatMessageFinished` — message complete
- `ChatAgentModePreviewCleared` — mode preview reset

---

## Step 7: End-to-End Playwright Testing of Debug UI

After all code changes were committed, the debug UI was tested end-to-end using Playwright against a live Go server and Vite dev server. The test verified that the debug UI can connect to a real session, receive the initial snapshot, and stream live events as the chat generates responses.

### Prompt Context

**User prompt (verbatim):** "test the debug-ui with playwright. run the server in a tmux. run with vite as well so that you can quickly iterate on fixing the web part of things."

**Assistant interpretation:** Start the Go backend server and Vite dev server in tmux sessions, then use Playwright to navigate the debug UI, connect to a real session, and verify the full data flow.

**Inferred user intent:** Validate that the debug UI migration actually works with real data, not just type-checks.

### What I did

1. Started Go server in tmux session `go-server`:
   ```bash
   tmux new-session -d -s go-server
   go run ./cmd/web-chat web-chat --addr :8080 --timeline-db /tmp/pinocchio-debug-test/timeline.db --log-level debug
   ```
   Server started on `:8080`.

2. Started Vite dev server in tmux session `vite-dev`:
   ```bash
   tmux new-session -d -s vite-dev -c cmd/web-chat/web
   npx vite --port 5173 --host
   ```
   Vite picked port 5178 (5173-5177 were in use).

3. Navigated Playwright to `http://localhost:5178/?debug=1` — debug UI rendered correctly with 3 nav links (Overview, Timeline, Events), session ID input, and Follow button.

4. Entered fake session ID `test-session-123` and clicked Follow — WS connected (status "connected"), but no events (expected — fake session).

5. Opened second tab to `http://localhost:8080/` (production chat UI), sent message "Hello, this is a test message". Session ID appeared in URL: `3e867542-47ab-43c3-8b7e-eccb645b80b4`. Chat responded successfully.

6. Switched back to debug UI tab, disconnected from fake session, entered real session ID `3e867542-47ab-43c3-8b7e-eccb645b80b4`, clicked Follow.

7. **Snapshot received**: 3 entities rendered (ChatMessage assistant, ChatMessage user, ChatMessage thinking) with full content fields.

8. Sent second message from chat tab ("Thanks! That's all I needed."), switched back to debug UI.

9. **Live events streamed**: 795 events captured in real-time:
   - `ChatMessageAccepted` (#792)
   - `ChatMessageStarted` (#793)
   - `ChatReasoningStarted` (#794)
   - Hundreds of `ChatReasoningAppended` (#795–#1277)
   - `ChatReasoningFinished` (#1278)
   - Hundreds of `ChatMessageAppended` (#1279–#1585)
   - `ChatMessageFinished` (#1585)
   - `ChatAgentModePreviewCleared` (#1585)

10. Verified all 3 pages: Overview (summary + 2-lane view), Timeline (2-lane view), Events (event list).

### Why

Type-checking and linting are necessary but not sufficient. The debug UI had been completely broken before this migration — end-to-end testing with real data confirms the migration actually works.

### What worked

- Two-tab Playwright workflow was perfect: one tab for the chat (producing data), one tab for the debug UI (consuming data).
- The session ID in the chat tab's URL (`?sessionId=<uuid>`) made it trivial to connect the debug UI.
- The snapshot replay worked immediately — all 3 entities from the first conversation appeared.
- Live streaming worked with zero lag — events appeared in the debug UI as fast as the LLM generated them.

### What didn't work

- First attempt used a fake session ID (`test-session-123`). The WS connected successfully (server accepts any subscribe message) but no events appeared because no session existed. The user correctly suggested opening a second tab to create a real session.

### What was tricky to build

- The two-tab coordination: needed to create a session in tab 1, extract the session ID from the URL, then enter it in tab 0. Required switching tabs and waiting for the chat response to complete before the session ID appeared in the URL.

### What warrants a second pair of eyes

- The fact that the WS server accepts subscribe messages for non-existent sessions without error. This is correct behavior (the debug UI is a passive observer) but worth confirming it's intentional.

### What should be done in the future

- Add a "copy session ID from chat URL" button or auto-detect the `?sessionId=` parameter.
- Consider showing a warning when subscribed to a session with no events (currently just shows "No events yet").
- The Overview page shows entity count from the snapshot but the Events page shows event count from the live stream — these are different data sources and can be confusing.

### Code review instructions

- The test was performed with Playwright MCP against live servers. No automated test was added.
- To reproduce: start Go server + Vite, open chat, send message, copy session ID, open `?debug=1`, paste session ID, click Follow.

### Technical details

**Servers running in tmux:**
- `go-server`: `go run ./cmd/web-chat web-chat --addr :8080 --timeline-db /tmp/pinocchio-debug-test/timeline.db --log-level debug`
- `vite-dev`: `npx vite --port 5173 --host` (picked port 5178)

**Session ID used:** `3e867542-47ab-43c3-8b7e-eccb645b80b4`

**Console errors:** Only favicon 404 and React Router future flag warning — both pre-existing, not caused by the migration.

---

## Step 8: Rename FeatureSet to ChatPlugin

The design doc had already renamed `FeatureSet` to `ChatPlugin` in prose, but the actual Go code still used the old name. This step made the code match the documentation. The old name `FeatureSet` was ambiguous — it sounds like a data structure holding features, not a plugin interface that extends chat behavior.

### Prompt Context

**User prompt (verbatim):** "Continue with the rename."

**Assistant interpretation:** Rename `FeatureSet` → `ChatPlugin` and `WithFeatureSets` → `WithPlugins` across all Go code and documentation.

**Inferred user intent:** Make the code naming consistent with the design doc and the naming rationale ("plugin" conveys extension interface better than "feature set").

**Commit (code):** `8a2ca23` — "refactor: rename FeatureSet to ChatPlugin, WithFeatureSets to WithPlugins"

### What I did

Applied a systematic rename across 11 files using `sed`:

| Old name | New name |
|----------|----------|
| `FeatureSet` (interface) | `ChatPlugin` |
| `WithFeatureSets()` (option func) | `WithPlugins()` |
| `chatFeatures` (struct field) | `chatPlugins` |
| `WithChatFeatureSets()` (server option) | `WithChatPlugins()` |
| `agentModeChatFeature` (type) | `agentModePlugin` |
| `reasoningChatFeature` (type) | `reasoningPlugin` |
| `testFeatureProjection` (test type) | `testPlugin` |

Files changed:
- `pkg/chatapp/features.go` — interface + option func + internal methods
- `pkg/chatapp/chat.go` — struct field + `RegisterSchemas` signature
- `pkg/chatapp/chat_test.go` — test type + option call
- `cmd/web-chat/app/server.go` — struct field + server option func
- `cmd/web-chat/main.go` — constructor call
- `cmd/web-chat/agentmode_chat_feature.go` — type + constructor
- `cmd/web-chat/agentmode_chat_feature_test.go` — constructor call
- `cmd/web-chat/reasoning_chat_feature.go` — type + constructor
- `cmd/web-chat/reasoning_chat_feature_test.go` — constructor call
- `pkg/doc/tutorials/09-building-sessionstream-react-chat-apps.md` — tutorial references
- `.gitignore` — added `.playwright-mcp/`

### Why

"FeatureSet" describes what the thing contains (features), not what it does (extends chat behavior via a plugin interface). Every implementor (`agentModePlugin`, `reasoningPlugin`) is a plugin that registers schemas, handles runtime events, and projects UI/timeline entities. "ChatPlugin" names the role.

### What worked

- `sed` across all Go files was fast and accurate.
- The Go compiler caught two references in `server.go` that the first `sed` pass missed (the `chatapp.FeatureSet` type references in struct field and function signature).
- `gofmt -w` fixed a formatting issue that `golangci-lint` caught in the pre-commit hook.
- All lefthook hooks passed: lint (0 issues), test (all pass).

### What didn't work

- First commit attempt failed the pre-commit `lint` hook with a gofmt error on `server.go:34`. The `sed` replacement changed the field name length, breaking struct field alignment. Fixed with `gofmt -w`.
- The `agentmode_chat_feature_test.go` file was missed in the initial `sed` pass because it wasn't included in the grep results I reviewed. The compiler caught it immediately.

### What was tricky to build

- The `sed` commands had to run in the right order. Running `sed` for `chatFeatures` before `chatapp.FeatureSet` on `server.go` meant the first pattern didn't match the type references (which used the qualified form `chatapp.FeatureSet`). Had to run a separate `sed` pass for `chatapp.FeatureSet` → `chatapp.ChatPlugin` after the initial batch.

### What warrants a second pair of eyes

- Confirm the file names `agentmode_chat_feature.go` and `reasoning_chat_feature.go` are acceptable even though the internal types are now `agentModePlugin`/`reasoningPlugin`. A file rename would be more consistent but adds noise to the git history.

### What should be done in the future

- Consider renaming the files `agentmode_chat_feature.go` → `agentmode_plugin.go` and `reasoning_chat_feature.go` → `reasoning_plugin.go` for consistency with the type names.

### Code review instructions

- `git show 8a2ca23` — 11 files, all mechanical renames.
- Verify: `grep -rn 'FeatureSet\|WithFeatureSet\|ChatFeatureSet' --include="*.go" .` returns nothing.

---

## Summary

| Step | Description | Commit | Delta |
|------|-------------|--------|-------|
| 1 | Investigation — confirm SEM dead | (no commit) | 0 |
| 2 | Delete Go SEM registry | `ccceef6` | -70 |
| 3 | Delete TS SEM registry, migrate story, relocate timelinePropsRegistry | `e981ca2` | -453 |
| 4 | Delete obsolete tutorial 04 | `ab46129` | -1042 |
| 5 | Rewrite 4 stale doc topics | `7f52c12` | -48 |
| 6 | Migrate debug UI to sessionstream | `2992681` | -8638 |
| 7 | End-to-end Playwright testing | (no commit) | 0 |
| 8 | Rename FeatureSet → ChatPlugin | `8a2ca23` | +44/-43 |

**Totals:** 6 commits, ~9000 net lines deleted, 0 new dependencies.
