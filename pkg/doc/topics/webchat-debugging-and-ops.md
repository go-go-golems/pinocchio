---
Title: Webchat Debugging and Operations
Slug: webchat-debugging-and-ops
Short: Operational procedures for diagnosing and troubleshooting webchat issues.
Topics:
- webchat
- debugging
- operations
- websocket
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Scope

This guide focuses on operational debugging for the current sessionstream-backed HTTP chat setup:

- `POST /api/chat/sessions` to create a session
- `POST /api/chat/sessions/:sessionId/messages` to submit a prompt
- `GET /api/chat/sessions/:sessionId` for snapshot hydration
- `GET /api/chat/ws` for protobuf JSON websocket frames
- `GET /api/debug/conversations` for current runtime pointer inspection when debug APIs are enabled
- `GET /api/debug/turns` for turn inspection when a turn store is enabled
- `GET /api/chat/profiles` and schema endpoints for profile API health checks

## WebSocket Debugging

Enable frontend websocket debug logs:

- query: `?ws_debug=1`
- runtime: `window.__WS_DEBUG__ = true`
- persisted: `localStorage.setItem('__WS_DEBUG__', 'true')`

Look for lifecycle logs:

- `connect:begin`
- `socket:open`
- `message:forward`
- disconnect/retry markers

## Common Failures

### No events after websocket open

- Verify the client sends a subscribe frame with the intended `sessionId`.
- Check backend logs for run start and sessionstream publish.
- Confirm the app server registered chatapp schemas and plugins before constructing the hub.
- Confirm the selected profile resolves successfully for message submissions.

### Hydration ordering issues

- Inspect `GET /api/chat/sessions/:sessionId` output.
- Compare snapshot `snapshotOrdinal`, entity `createdOrdinal` / `lastEventOrdinal`, and live `eventOrdinal` values.
- Rebuild stale SQLite snapshots if legacy data used incompatible ordering.

### Chat request failures

- Check resolver errors from `POST /api/chat/sessions/:sessionId/messages`.
- Validate JSON body keys: `prompt`, `profile`, `registry`, and `idempotencyKey`.
- Confirm the profile slug exists in the selected profile registry.

### Turns endpoint unavailable

- `GET /api/debug/turns` returns 404 when turn store is disabled.
- Enable with `--turns-db` or `--turns-dsn`.

### Runtime history confusion

- conversation debug payloads expose `resolved_runtime_key` (latest pointer only),
- turn payloads expose per-turn `runtime_key` and `inference_id`,
- for historical attribution, query `/api/debug/turns`, not only `/api/debug/conversations/:id`.

### JS timeline runtime issues

- If startup fails after adding `--timeline-js-script`, check script path and syntax first.
- Startup loading is fail-fast: invalid script prevents server boot.
- Runtime callback errors are non-fatal and logged as warnings:
  - `js timeline handler threw; continuing`
  - `js timeline reducer threw; continuing`
  - `js timeline reducer upsert failed; continuing`
- JS bindings are exposed as a native module:
  - `const p = require("pinocchio")` (alias: `require("pnocchio")`)
  - `p.timeline.onUIEvent(eventName, fn)`
  - `p.timeline.upsertEntity(entity)`
- If builtin projection disappears for an event type (for example `llm.delta`), verify your reducer is not returning `consume: true`.
- For additive projection, return `consume: false` and only emit `upserts`.
- Wildcard hooks use `p.timeline.onUIEvent("*", fn)` and run for all UI event types.

## Operational Checks

Backend checks:

- Confirm HTTP server route mounts include `/api/chat/sessions`, `/api/chat/sessions/:sessionId/messages`, `/api/chat/sessions/:sessionId`, `/api/chat/ws`, and profile routes.
- Confirm `chatapp.RegisterSchemas(reg, plugins...)` runs with `NewReasoningPlugin()` and `NewToolCallPlugin()` when reasoning/tool rows are expected.
- Confirm timeline store configuration (`--timeline-db` or `--timeline-dsn`) when durability is expected.
- Confirm turn store configuration for debug turn queries.
- Confirm profile API mounts include `/api/chat/profiles`, `/api/chat/schemas/middlewares`, and `/api/chat/schemas/extensions`.
- Confirm JS runtime scripts are loaded via `--timeline-js-script` and inspect startup log for script list.

Frontend checks:

- Verify fetch targets `/api/chat/sessions/:sessionId`, not `/api/timeline`.
- Verify websocket URL is `/api/chat/ws` under the current base prefix.
- Ensure hydration gate is active before live stream replay.
- Verify the websocket frame parser reads role-specific ordinals (`snapshotOrdinal`, `eventOrdinal`) or explicitly supports compatibility aliases.

## Quick Curl Smoke Tests

```bash
session_id=$(curl -s -X POST http://localhost:8080/api/chat/sessions \
  -H 'content-type: application/json' \
  -d '{}' | jq -r '.sessionId')

curl -i -X POST "http://localhost:8080/api/chat/sessions/${session_id}/messages" \
  -H 'content-type: application/json' \
  -d '{"prompt":"hello"}'

curl -i "http://localhost:8080/api/chat/sessions/${session_id}"

curl -i "http://localhost:8080/api/debug/conversations/${session_id}"

curl -i "http://localhost:8080/api/debug/turns?conv_id=${session_id}&limit=5"

curl -i 'http://localhost:8080/api/chat/profiles'

curl -i 'http://localhost:8080/api/chat/schemas/middlewares'
```

Example runtime history query:

```bash
curl -s "http://localhost:8080/api/debug/turns?conv_id=${session_id}&limit=50" \
  | jq '.items[] | {turn_id, phase, runtime_key, inference_id, created_at_ms}'
```

## Non-Canonical Paths

If operational runbooks still mention these, update them:

- `/timeline`
- `/turns`
- `/hydrate`

## See Also

- [Chatapp Protobuf Schemas and Shared Plugins](chatapp-protobuf-plugins.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
- [Webchat Frontend Architecture](webchat-frontend-architecture.md)
- [Webchat Runtime Truth Migration Playbook](webchat-runtime-truth-migration-playbook.md)
