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

This guide focuses on operational debugging for the current HTTP chat setup:

- `POST /chat`
- `GET /ws?conv_id=...`
- `GET /api/timeline`
- `GET /api/debug/conversations` for current runtime pointer inspection
- `GET /api/debug/turns` for turn inspection
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

- Verify `conv_id` is present in websocket URL.
- Check backend logs for run start and stream publish.
- Confirm resolver accepts websocket requests.

### Hydration ordering issues

- Inspect `/api/timeline?conv_id=...` output.
- Compare entity versions with websocket `event.seq` ordering.
- Rebuild stale SQLite snapshots if legacy data used incompatible ordering.

### Chat request failures

- Check resolver errors from `POST /chat`.
- Validate JSON body keys: `prompt`, `conv_id`, `overrides`, `idempotency_key`.
- Confirm runtime/profile slug exists when calling `/chat/{runtime}`.

### Turns endpoint unavailable

- `GET /api/debug/turns` returns 404 when turn store is disabled.
- Enable with `--turns-db` or `--turns-dsn`.

### Runtime history confusion

- conversation debug payloads expose `current_runtime_key` (latest pointer only),
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
  - `p.timeline.registerSemReducer(eventType, fn)`
  - `p.timeline.onSem(eventType, fn)`
- If builtin projection disappears for an event type (for example `llm.delta`), verify your reducer is not returning `consume: true`.
- For additive projection, return `consume: false` and only emit `upserts`.
- Wildcard hooks use `p.timeline.onSem("*", fn)` and run for all SEM event types.

## Operational Checks

Backend checks:

- Confirm HTTP server route mounts include `/chat`, `/ws`, `/api/timeline`, `/api/`.
- Confirm timeline store configuration (`--timeline-db` or `--timeline-dsn`) when durability is expected.
- Confirm turn store configuration for debug turn queries.
- Confirm profile API mounts include `/api/chat/profiles`, `/api/chat/schemas/middlewares`, and `/api/chat/schemas/extensions`.
- Confirm JS runtime scripts are loaded via `--timeline-js-script` and inspect startup log for script list.

Frontend checks:

- Verify fetch targets `/api/timeline`, not `/timeline`.
- Verify websocket URL is `/ws?conv_id=...` under current base prefix.
- Ensure hydration gate is active before live stream replay.

## Quick Curl Smoke Tests

```bash
curl -i -X POST http://localhost:8080/chat \
  -H 'content-type: application/json' \
  -d '{"prompt":"hello","conv_id":"conv-smoke"}'

curl -i 'http://localhost:8080/api/timeline?conv_id=conv-smoke'

curl -i 'http://localhost:8080/api/debug/conversations/conv-smoke'

curl -i 'http://localhost:8080/api/debug/turns?conv_id=conv-smoke&limit=5'

curl -i 'http://localhost:8080/api/chat/profiles'

curl -i 'http://localhost:8080/api/chat/schemas/middlewares'
```

Example runtime history query:

```bash
curl -s 'http://localhost:8080/api/debug/turns?conv_id=conv-smoke&limit=50' \
  | jq '.items[] | {turn_id, phase, runtime_key, inference_id, created_at_ms}'
```

## Non-Canonical Paths

If operational runbooks still mention these, update them:

- `/timeline`
- `/turns`
- `/hydrate`

## See Also

- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat Runtime Truth Migration Playbook](webchat-runtime-truth-migration-playbook.md)
