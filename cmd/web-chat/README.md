---
Title: Pinocchio Web-Chat Example
Slug: pinocchio-web-chat
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

`cmd/web-chat` is the reference app for the new HTTP route ownership model.

- App code owns `/chat` and `/ws`.
- `pkg/webchat` provides service surfaces and helper handlers.
- Timeline hydration is served at `/api/timeline`.
- Debug endpoints live under `/api/debug/*` when `--debug-api` is enabled.

## Directory Structure

- `main.go`:
  - builds `webchat.Server` via `webchat.NewServer`
  - registers middleware/tool factories
  - mounts app-owned `/chat` and `/ws` handlers
  - mounts `/api/timeline`, `srv.APIHandler()`, `srv.UIHandler()`
- `runtime_composer.go`: runtime composition policy for engines/middlewares/tools
- `profile_policy.go`: request resolver and app-owned profile endpoints
- `web/`: Vite frontend source
- `static/`: embedded frontend assets (including `static/dist` build output)
- `gen_frontend.go`: `go generate` frontend build hook

## Runtime Flow

1. Browser opens UI from `/`.
2. Frontend connects `GET /ws?conv_id=<id>`.
3. Frontend submits prompt via `POST /chat`.
4. Backend emits SEM frames over websocket.
5. Frontend hydrates and reconciles with `GET /api/timeline`.

## HTTP API

- `POST /chat`
- `POST /chat/{runtime}`
- `GET /ws?conv_id=<id>`
- `GET /api/timeline?conv_id=<id>&since_version=<n>&limit=<n>`
- `GET /api/chat/profiles` (app-owned)
- `GET /api/chat/profile` (app-owned)
- `POST /api/chat/profile` (app-owned)
- `GET /api/debug/turns?...` (debug only, turn store required)
- `GET /api/debug/turn/:convId/:sessionId/:turnId` (debug only)

Debug routes are opt-in and only mounted when `--debug-api` is set.

Legacy route names are intentionally not documented here:

- `/timeline`
- `/turns`
- `/hydrate`

### `POST /chat` Request (Hard-Cut Contract)

Use canonical request keys:

```json
{
  "prompt": "hello",
  "conv_id": "optional-conversation-id",
  "runtime_key": "optional-profile-slug-or-runtime-key",
  "request_overrides": {
    "system_prompt": "optional policy-gated override"
  },
  "idempotency_key": "optional-client-idempotency-key"
}
```

Legacy aliases are removed from resolver handling:

- `profile`
- `registry`
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

With `--root /chat`, routes are mounted under `/chat`:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`
- `/chat/api/debug/conversations`
- `/chat/api/debug/turns`

## Related Docs

- [Webchat HTTP Chat Setup](../../pkg/doc/topics/webchat-http-chat-setup.md)
- [Webchat Framework Guide](../../pkg/doc/topics/webchat-framework-guide.md)
- [Webchat Frontend Integration](../../pkg/doc/topics/webchat-frontend-integration.md)
