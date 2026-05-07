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
