# Tasks

## Phase 0: Design and planning

- [x] Create ticket PINO-STREAM-DEBUG.
- [x] Analyze the full streaming pipeline: backend event emission → WebSocket transport → frontend parsing → projection → Redux state → rendering.
- [x] Design the event recording format for backend events (what to persist, where, and how to query).
- [ ] Design the frontend debug mode: what to capture, how to store, how to present.
- [ ] Design the comparison/reconciliation tool for backend-emitted vs frontend-received event sequences.

## Phase 1: Backend event recording

- [x] Update Pinocchio to a `sessionstream` version that includes `SS-OBSERVERS` Hub pipeline and WebSocket transport observers.
- [x] Add a debug recorder in `cmd/web-chat/app` that captures Sessionstream `PipelineRecord` values by session.
- [x] Add a debug recorder in `cmd/web-chat/app` that captures Sessionstream WebSocket `TransportRecord` values by session and connection.
- [x] Record event ordinal, event name, payload type, stage outputs, target connection IDs, frame type, queue/write status, errors, and timestamp.
- [x] Add a debug API endpoint to retrieve pipeline records for a session (e.g., `GET /api/debug/sessions/{id}/pipeline`).
- [x] Add a debug API endpoint to retrieve transport records for a session (e.g., `GET /api/debug/sessions/{id}/transport`).
- [x] Add a combined debug API endpoint to retrieve all backend stream records for a session (e.g., `GET /api/debug/sessions/{id}/records`).
- [x] Make event recording opt-in via `--debug-api` flag or environment variable.
- [ ] If `SS-OBSERVERS` is not available yet, implement a temporary reduced recorder by wrapping `UIFanout`, and mark it as replaceable.

## Phase 2: Frontend debug mode

- [x] Add a `debugStream` flag activated by `localStorage.setItem('pinocchio.debugStream', '1')`.
- [x] When active, record every raw WebSocket message, every parsed frame, every hydration entity, every UI event, and every timeline mutation to an in-memory log.
- [x] Add a debug panel (collapsible overlay) that shows the recorded log with filtering by type (raw, parsed, snapshot, ui-event, mutation).
- [x] Show hydration snapshot details: entity count per kind, snapshot ordinal, hydration timestamp.
- [x] Allow exporting the debug log as JSON for offline comparison.

## Phase 3: Comparison and reconciliation tools

- [x] Build a backend reconciliation endpoint that loads Sessionstream observer records for the same session and highlights backend pipeline-vs-transport discrepancies.
- [ ] Extend reconciliation to load/export frontend logs for browser receipt and mutation comparison.
- [x] Detect backend missing fanout ordinals between Hub pipeline records and WebSocket transport records.
- [ ] Detect browser-side missing events, extra events, ordering differences, and payload mismatches from frontend debug JSON.
- [ ] Build a diff view for hydration snapshot vs live-rendered state after streaming completes.


## Phase 3b: SQLite reconcile upload artifact

- [x] Write a detailed design document for frontend-log upload returning a SQLite debug database.
- [x] Add `POST /api/debug/sessions/{id}/reconcile/upload`.
- [x] Accept frontend debug logs as either `{records:[...]}` or raw array JSON.
- [x] Create SQLite schema with raw backend/frontend record tables and parsed typed tables.
- [x] Insert backend pipeline and transport observer records into SQLite.
- [x] Insert frontend raw WebSocket, parsed frame, snapshot, UI-event, and lifecycle records into SQLite.
- [x] Return the SQLite DB as `application/vnd.sqlite3` with attachment headers.
- [x] Add integration test that opens the returned SQLite DB and verifies key tables contain rows.

## Phase 4: Robustness testing scenarios

- [ ] Scenario: normal chat flow — send prompt, receive streaming response, verify all entities rendered.
- [ ] Scenario: reload while streaming — mid-stream page reload, reconnect, re-hydrate, verify state matches.
- [ ] Scenario: second tab on same conversation — open second browser tab, subscribe to same session, verify both tabs receive events.
- [ ] Scenario: reload second tab — reload one tab while other continues streaming, verify recovery.
- [ ] Scenario: rapid sequential prompts — send multiple prompts without waiting for completion, verify no event cross-contamination.
- [ ] Scenario: network interruption — simulate WebSocket disconnect/reconnect, verify re-hydration.

## Phase 5: Hardening

- [ ] Use `SS-WS-RACE` observer traces as the canonical reload-during-streaming scenario.
- [ ] After `SS-WS-RACE` lands, verify that reload-during-streaming produces snapshot-first plus buffered-live event ordering.
- [ ] Fix any discrepancies found during Phase 4 testing.
- [ ] Add regression tests for discovered edge cases.
- [ ] Document the debug mode usage in a playbook.

## External dependencies

- [x] Track `SS-OBSERVERS` in `sessionstream/ttmp`: Hub pipeline observer and WebSocket transport observer.
- [x] Track `SS-WS-RACE` in `sessionstream/ttmp`: subscribe-first hydration buffer for reload/reconnect correctness.
