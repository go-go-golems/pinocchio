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
