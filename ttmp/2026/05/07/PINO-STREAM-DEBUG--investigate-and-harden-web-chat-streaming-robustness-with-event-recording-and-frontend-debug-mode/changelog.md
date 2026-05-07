# Changelog

## 2026-05-07

- Initial workspace created

## 2026-05-06

- Updated the streaming investigation guide to consume the new Sessionstream observer plan from `SS-OBSERVERS`.
- Updated tasks so backend recording uses Sessionstream Hub `PipelineRecord` and WebSocket `TransportRecord` values when available.
- Added dependency on `SS-WS-RACE` for proving and fixing reload-during-streaming subscribe/hydration races.

## 2026-05-07

- Noted that Sessionstream observer APIs (`SS-OBSERVERS`) and subscribe hydration buffering (`SS-WS-RACE`) have landed.
- Updated the implementation guide with the corrected reconnect trace that Pinocchio should now consume and verify.

## 2026-05-07

- Implemented backend debug recorder wiring for Sessionstream `PipelineRecord` and WebSocket `TransportRecord` values.
- Added debug endpoints under `/api/debug/sessions/{id}/{pipeline,transport,records}` behind `--debug-api`.
- Added Pinocchio implementation diary and backend recorder integration test.

## 2026-05-07

- Implemented frontend `pinocchio.debugStream` recorder for raw WebSocket frames, parsed frames, snapshots, UI-event mutations, and lifecycle events.
- Added `StreamDebugPanel` overlay with filtering, clear, and JSON export.

## 2026-05-07

- Added backend reconciliation endpoint `/api/debug/sessions/{id}/reconcile` comparing Hub pipeline fanout ordinals with WebSocket transport fanout ordinals.

## 2026-05-07

- Added design document for SQLite reconcile upload artifacts.
- Implemented `POST /api/debug/sessions/{id}/reconcile/upload`, returning a SQLite DB populated with backend observer records and uploaded frontend debug records.
- Added integration test that validates the returned SQLite schema contains backend and frontend rows.

## 2026-05-07 (step 5)

- Added timeline entities and turns tables to the SQLite reconcile artifact.
- Added `DebugDataProvider` interface and `exportDataProvider` adapter.
- Added `Download SQLite` button to the frontend StreamDebugPanel.
- Added `uploadAndDownloadSQLite()` to the frontend stream debug module.
- Added `TestDebugReconcileUploadIncludesTimelineAndTurns` with mock provider.
- Performed full-circle Playwright validation with real server on :8092.
- Verified all 15 SQLite tables populated: backend pipeline/transport, frontend raw/parsed/snapshot/ui/lifecycle, timeline entities, turns.
- Commit: `7f9ca6c feat(web-chat): include timeline entities and turns in reconcile SQLite, add download button`

## 2026-05-07 (step 6)

- Added devctl plugin `cmd/web-chat/plugins/webchat.py` for backend + Vite lifecycle management.
- Added `cmd/web-chat/.devctl.yaml` configuration.
- Plugin discovers free ports for backend (default 8092) and Vite (default 5174).
- Plugin builds Go binary from module root, wires `VITE_BACKEND_ORIGIN` for Vite proxy.
- Added frontend debug toggle button: "Debug" (enable) and "Stop" (disable) in the StreamDebugPanel.
- Added `toggleStreamDebug()` function and UI button for enabling/disabling without console.
- Commit: `fe89fd6 feat(web-chat): add devctl plugin for backend+vite, add debug toggle button`

## 2026-05-07 (step 7)

- Added 9 SQL views to the SQLite reconcile artifact for common delivery-chain analysis.
- Views: `missing_transport_fanout`, `extra_transport_fanout`, `backend_pipeline_errors`, `backend_transport_errors`, `frontend_parsed_no_mutation`, `frontend_dropped_entities`, `tombstoned_entities`, `delivery_chain`, `entity_kind_summary`.
- Added view existence check to `TestDebugReconcileUploadIncludesTimelineAndTurns`.
- Ran full reload-during-streaming Playwright validation: mid-stream reload, re-hydration, streaming resumed with 0 errors and 0 dropped entities.
- Validated all 9 views produce correct results against real session data.
- Commit: `330950a feat(web-chat): add SQL views to reconcile SQLite for delivery-chain analysis`

## 2026-05-07 (step 8)

- Investigated duplicated thinking block visible in the timeline.
- Confirmed root cause in persisted timeline: `reasoning-summary` created `chat-msg-1:thinking:2` after `thinking-ended` completed `chat-msg-1:thinking:1`.
- Fixed `ReasoningPlugin` so `reasoning-summary` updates the completed current segment instead of allocating a new segment.
- Added regression test `TestReasoningPluginSummaryUpdatesCompletedSegment`.
- Live-smoke validated new session has exactly one thinking entity.
- Commit: `0e927f6 fix(web-chat): avoid duplicate reasoning summary timeline blocks`
