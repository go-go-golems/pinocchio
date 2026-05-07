# Tasks

## Phase 0: Design and planning

- [x] Create ticket PINO-STREAM-DEBUG.
- [ ] Analyze the full streaming pipeline: backend event emission → WebSocket transport → frontend parsing → projection → Redux state → rendering.
- [ ] Design the event recording format for backend events (what to persist, where, and how to query).
- [ ] Design the frontend debug mode: what to capture, how to store, how to present.
- [ ] Design the comparison/reconciliation tool for backend-emitted vs frontend-received event sequences.

## Phase 1: Backend event recording

- [ ] Add a debug event recorder that captures all backend events for a session in a structured log (SQLite table or JSONL file).
- [ ] Record event ordinal, event name, payload type, payload JSON, and timestamp.
- [ ] Add a debug API endpoint to retrieve recorded events for a session (e.g., `GET /api/debug/sessions/{id}/events`).
- [ ] Make event recording opt-in via `--debug-api` flag or environment variable.

## Phase 2: Frontend debug mode

- [ ] Add a `debugStream` flag activated by `localStorage.setItem('pinocchio.debugStream', '1')`.
- [ ] When active, record every raw WebSocket message, every parsed frame, every hydration entity, every UI event, and every timeline mutation to an in-memory log.
- [ ] Add a debug panel (collapsible overlay) that shows the recorded log with filtering by type (raw, parsed, snapshot, ui-event, mutation).
- [ ] Show hydration snapshot details: entity count per kind, snapshot ordinal, hydration timestamp.
- [ ] Allow exporting the debug log as JSON for offline comparison.

## Phase 3: Comparison and reconciliation tools

- [ ] Build a reconciliation script/endpoint that loads backend events and frontend log for the same session and highlights discrepancies.
- [ ] Detect: missing events (emitted but not received), extra events (received but not emitted), ordering differences, payload mismatches.
- [ ] Build a diff view for hydration snapshot vs live-rendered state after streaming completes.

## Phase 4: Robustness testing scenarios

- [ ] Scenario: normal chat flow — send prompt, receive streaming response, verify all entities rendered.
- [ ] Scenario: reload while streaming — mid-stream page reload, reconnect, re-hydrate, verify state matches.
- [ ] Scenario: second tab on same conversation — open second browser tab, subscribe to same session, verify both tabs receive events.
- [ ] Scenario: reload second tab — reload one tab while other continues streaming, verify recovery.
- [ ] Scenario: rapid sequential prompts — send multiple prompts without waiting for completion, verify no event cross-contamination.
- [ ] Scenario: network interruption — simulate WebSocket disconnect/reconnect, verify re-hydration.

## Phase 5: Hardening

- [ ] Fix any discrepancies found during Phase 4 testing.
- [ ] Add regression tests for discovered edge cases.
- [ ] Document the debug mode usage in a playbook.
