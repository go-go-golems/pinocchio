# Changelog

## 2026-02-14

- Initial workspace created


## 2026-02-14

Created realtime websocket follow implementation plan and execution task list for debug UI attach mode.

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/geppetto/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/design/01-implementation-plan-realtime-follow-via-websocket.md — Primary implementation guide
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/geppetto/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/reference/01-diary.md — Exploration diary


## 2026-02-14

Refined scope: follow mode now targets generic timeline.upsert stream + API bootstrap only; turn/block websocket streaming deferred.

### Related Files

- geppetto/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/design/01-implementation-plan-realtime-follow-via-websocket.md — Updated architecture
- geppetto/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/reference/01-diary.md — Added exploration step documenting scope change and websocket findings
- geppetto/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/tasks.md — Updated task list for timeline-upsert-only scope


## 2026-02-15

Aligned GP-020 plan with current app-owned webchat HTTP setup and canonical routes.

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/design/01-implementation-plan-realtime-follow-via-websocket.md — Switched bootstrap guidance to `/api/timeline`, added app-owned route ownership notes, and documented root-prefix requirement
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/tasks.md — Updated task checklist for canonical timeline endpoint and base-prefix handling
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/index.md — Refreshed ticket summary/intent metadata


## 2026-02-15

Applied fresh-cutover policy to GP-020: removed all legacy fallback wording for follow bootstrap.

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/design/01-implementation-plan-realtime-follow-via-websocket.md — Removed `/api/debug/timeline` fallback path from implementation plan
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/14/GP-020-DEBUG-UI-REALTIME-FOLLOW--realtime-follow-mode-for-debug-ui-via-websocket/tasks.md — Updated bootstrap task to canonical-only `/api/timeline`

## 2026-02-15

Step 3: implemented follow state/actions/selectors in debug-ui store and checked Task 1 (commit 8c13fbe).

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/cmd/web-chat/web/src/debug-ui/store/uiSlice.ts — Store contract for realtime follow lifecycle


## 2026-02-15

Step 4: added debug timeline websocket manager core and checked Tasks 2-4 (commit b6117d6).

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts — Canonical bootstrap and timeline.upsert dedupe pipeline


## 2026-02-15

Step 5: implemented Tasks 5-8 by wiring follow controls/status UI, app-level lifecycle hook, mount-aware follow connect/bootstrap paths, and explicit read-only follow transport note (commit c4a7c4c).

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/cmd/web-chat/web/src/debug-ui/components/SessionList.tsx — User-facing follow/pause/resume/reconnect controls
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/cmd/web-chat/web/src/debug-ui/ws/useDebugTimelineFollow.ts — Central follow lifecycle owner with reconnect token and base prefix support

