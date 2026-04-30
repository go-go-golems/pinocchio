---
Title: Pinocchio Web-Chat Example
Slug: web-chat
Short: Handler-first webchat example with app-owned HTTP routes and SEM streaming.
Topics:
- webchat
- streaming
- events
- websocket
- middleware
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

`cmd/web-chat` now serves the migrated session-based `evtstream` path as its live chat runtime.

- App code owns the canonical routes under `/api/chat/...`.
- The main browser flow uses session-based HTTP + websocket transport:
  - `POST /api/chat/sessions`
  - `POST /api/chat/sessions/:sessionId/messages`
  - `GET /api/chat/sessions/:sessionId`
  - `GET /api/chat/ws`
- Profile selection APIs remain app-owned under `/api/chat/profile*`.
- The embedded web UI is served directly from `cmd/web-chat/static`.
- Legacy `/chat`, `/ws`, and `/api/timeline` are no longer part of the live default path.

## Directory Structure

- `main.go`:
  - builds the canonical `cmd/web-chat/app` server
  - mounts profile APIs plus canonical session/snapshot/websocket routes
  - serves embedded UI assets directly
- `app/`: app-owned canonical chat contracts and handlers
- `profile_policy.go`: app-owned profile endpoints and request/profile selection policy
- `web/`: Vite frontend source
- `static/`: embedded frontend assets (with optional `static/dist` build output)
- `gen_frontend.go`: `go generate` frontend build hook

## Runtime Flow

1. Browser opens UI from `/`.
2. Frontend creates or resumes a `sessionId`.
3. Frontend connects `GET /api/chat/ws` and sends a subscribe frame.
4. Frontend submits prompts via `POST /api/chat/sessions/:sessionId/messages`.
5. Backend sends websocket `snapshot`, `subscribed`, and `ui-event` frames.
6. Frontend reload/reconnect uses `GET /api/chat/sessions/:sessionId` plus websocket resubscription.

## HTTP API

Canonical live routes:

- `POST /api/chat/sessions`
- `POST /api/chat/sessions/:sessionId/messages`
- `GET /api/chat/sessions/:sessionId`
- `GET /api/chat/ws`
- `GET /api/chat/profiles` (app-owned)
- `GET /api/chat/profile` (app-owned)
- `POST /api/chat/profile` (app-owned)

Legacy route names are intentionally no longer part of the live contract:

- `/chat`
- `/ws`
- `/api/timeline`
- `/timeline`
- `/turns`
- `/hydrate`

### `POST /chat` Request (Hard-Cut Contract)

Use canonical request keys:

```json
{
  "prompt": "hello",
  "conv_id": "optional-conversation-id",
  "profile": "optional-profile-slug-or-runtime-key",
  "registry": "optional-registry-slug",
  "idempotency_key": "optional-client-idempotency-key"
}
```

Legacy aliases are removed from resolver handling:

- `runtime_key`
- `registry_slug`
- `overrides`
- `runtime` query alias

### Runtime Metadata in Responses

Chat responses now include resolver/runtime metadata fields:

- `runtime_fingerprint`
- `profile_metadata`
  - includes resolver metadata keys such as:
    - `profile.stack.lineage`
    - `profile.stack.trace`

## Durable Stores

Timeline store:

- `--timeline-dsn "<sqlite dsn>"`
- `--timeline-db "<path/to/timeline.db>"`

Turn store:

- `--turns-dsn "<sqlite dsn>"`
- `--turns-db "<path/to/turns.db>"`

## JavaScript Timeline Runtime

`web-chat` can load JavaScript SEM reducers/handlers at startup:

- `--timeline-js-script <path>` (repeat flag or pass comma-separated list)

Each script can register handlers via the `pinocchio` native module:

- `const p = require("pinocchio")`
- `p.timeline.registerSemReducer(eventType, fn)`
- `p.timeline.onSem(eventType, fn)`
- alias module name: `require("pnocchio")`

Runtime contract:

- reducer callback signature: `fn(event, ctx)`
- handler callback signature: `fn(event, ctx)`
- `event` fields: `type`, `id`, `seq`, `stream_id`, `data`, `now_ms`
- `ctx` fields: `now_ms`
- reducer may return:
  - `true` / `{ consume: true }` to consume event and skip builtin projection
  - entity object `{ id, kind, props, meta, created_at_ms, updated_at_ms }`
  - array of entities
  - `{ consume, upserts }` where `upserts` is entity or array

Safety/behavior notes:

- script load/parse failures are startup errors (server does not continue with bad script state)
- runtime reducer/handler throw is logged and processing continues
- reducer upsert failure is logged and processing continues
- use `consume: false` to keep builtin projection active (recommended for additive projections)
- use `consume: true` only when intentionally replacing builtin projection for that event

## Run

```bash
go generate ./cmd/web-chat
go run ./cmd/web-chat web-chat --addr :8080 --profile-registries ./profiles.db
```

Open `http://localhost:8080/`.

Enable debug API routes:

```bash
go run ./cmd/web-chat web-chat --addr :8080 --debug-api --profile-registries ./profiles.db
```

Example with JS reducer runtime and `gpt-5-nano` profile registry:

```bash
go run ./cmd/web-chat web-chat \
  --addr :8080 \
  --profile-registries ./profiles.yaml \
  --timeline-js-script ./scripts/timeline-llm-delta-reducer.js
```

Runtime configuration note:

- `web-chat` no longer exposes direct AI runtime flags such as `--ai-engine` or `--ai-api-type`.
- Model/provider settings, API keys, and related step settings must come from the selected profile stack in `--profile-registries`.
- Command parsing is now limited to server/profile/transport settings plus standard command/config handling.

Example with root mount and non-default dev ports:

```bash
# backend
go run ./cmd/web-chat web-chat --addr :8081 --root /chat --debug-api --profile-registries ./profiles.db

# frontend (from cmd/web-chat/web)
VITE_BACKEND_ORIGIN=http://localhost:8081 \
npm run dev -- --port 5714
```

Runtime prefix is communicated to the TS app via `app-config.js`:

- Go backend serves `app-config.js` from command settings (`--root`, `--debug-api`)
- when mounted under `--root /chat`, backend exposes both `/chat/app-config.js` and `/app-config.js`
- Vite dev server proxies `/app-config.js` to `VITE_BACKEND_ORIGIN`

## Frontend Dev Checks

Run from `cmd/web-chat/web`:

```bash
npm run typecheck
npm run lint
npm run check
```

## Root Prefix

With `--root /chat`, canonical routes are mounted under `/chat`:

- `/chat/api/chat/sessions`
- `/chat/api/chat/sessions/:sessionId/messages`
- `/chat/api/chat/sessions/:sessionId`
- `/chat/api/chat/ws`
- `/chat/api/chat/profiles`
- `/chat/api/chat/profile`

## Related Docs

- [Webchat HTTP Chat Setup](../../pkg/doc/topics/webchat-http-chat-setup.md)
- [Webchat Framework Guide](../../pkg/doc/topics/webchat-framework-guide.md)
- [Webchat Frontend Integration](../../pkg/doc/topics/webchat-frontend-integration.md)
