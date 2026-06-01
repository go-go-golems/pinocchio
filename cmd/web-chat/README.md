---
Title: Pinocchio Web-Chat Example
Slug: web-chat
Short: Handler-first web-chat example with internal Go app wiring, sessionstream transport, and provider-backed React UI.
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

`cmd/web-chat` is Pinocchio's opinionated web-chat example. It combines a Go backend, sessionstream command/event transport, profile-backed Geppetto runtime construction, and an embedded React frontend.

The backend is deliberately organized as a command-owned application under `cmd/web-chat/internal`. Reusable mechanics live in `pkg/chatapp/...`; web-chat-specific route wiring, profile APIs, runtime composition, and app-owned plugins stay private to this command.

## High-level flow

1. The browser loads the embedded React UI from `/` or the configured `--root` prefix.
2. The frontend creates a chat session with `POST /api/chat/sessions`.
3. The frontend subscribes to live updates through `GET /api/chat/ws`.
4. Prompts are submitted to `POST /api/chat/sessions/{sessionId}/messages`.
5. The Go backend resolves the selected profile, builds a runtime, submits a prompt to `chatapp.Service`, and emits protobuf-backed sessionstream events.
6. The frontend hydrates from snapshots and applies live WebSocket frames through `@go-go-golems/chat-provider` timeline adapters.

## Backend directory map

- `main.go` — thin executable/Glazed/Cobra entrypoint. It embeds `static/`, declares command flags/sections, and delegates execution to `internal/webchatcmd`.
- `gen_frontend.go` — `go generate` hook that builds the React frontend into `static/dist`.
- `static/` — embedded frontend assets used by the production single-binary path.
- `internal/webchatcmd/` — command composition root: decodes server settings, resolves profiles, builds middleware/runtime/appserver dependencies, and starts HTTP serving.
- `internal/webapp/` — browser HTTP shell: `app-config.js`, static assets, SPA fallback, root-prefix mounting, and HTTP server lifecycle.
- `internal/appserver/` — chat HTTP adapter: session routes, WebSocket route, export routes, frontend-tool manifest/result routes, hydration store setup, snapshots, and server construction.
- `internal/profiles/` — app-owned profile APIs and request/profile resolution helpers.
- `internal/runtime/` — Geppetto runtime composer, canonical runtime resolver, turn persistence, and agent-mode sink wrapper.
- `internal/middlewaredefs/` — web-chat middleware definition catalog, currently including the agent-mode middleware definition.
- `internal/plugins/agentmode/` — app-owned chat plugin that projects agent-mode runtime events into UI/timeline events.
- `internal/mockruntime/` — deterministic `mock_parity` runtime used by parity/smoke tests.
- `web/` — React + Storybook frontend source. See `web/README.md`.

## API routes

Canonical live routes:

- `POST /api/chat/sessions`
- `POST /api/chat/sessions/{sessionId}/messages`
- `GET /api/chat/sessions/{sessionId}`
- `GET /api/chat/sessions/{sessionId}/timeline`
- `GET /api/chat/sessions/{sessionId}/turns`
- `GET /api/chat/sessions/{sessionId}/export`
- `POST /api/chat/sessions/{sessionId}/tools/manifest`
- `POST /api/chat/sessions/{sessionId}/tools/results`
- `GET /api/chat/ws`
- `GET /api/chat/profiles`
- `GET /api/chat/profiles/{slug}`
- `GET /api/chat/profile`
- `POST /api/chat/profile`
- `GET /api/chat/schemas/middlewares`
- `GET /api/chat/schemas/extensions`

Legacy routes such as `/chat`, `/ws`, `/api/timeline`, `/timeline`, `/turns`, and `/hydrate` are intentionally not part of the live contract.

## Message contracts

Create a session:

```json
POST /api/chat/sessions

{
  "profile": "optional-profile-slug",
  "registry": "optional-registry-slug"
}
```

Submit a message:

```json
POST /api/chat/sessions/{sessionId}/messages

{
  "prompt": "hello",
  "profile": "optional-profile-slug",
  "registry": "optional-registry-slug",
  "idempotencyKey": "optional-client-idempotency-key"
}
```

## Profiles and runtime construction

Profile registries are resolved through the shared Pinocchio profile bootstrap layer. The selected profile determines runtime metadata, middleware uses, tools, model settings, and profile version/fingerprint information.

Runtime composition happens in `internal/runtime`:

- profile runtime metadata becomes an `infruntime.ConversationRuntimeRequest`;
- the canonical resolver short-circuits `mock_parity` into `internal/mockruntime`;
- regular profiles build a Geppetto engine plus middleware chain;
- turn persistence is attached when a turn store is configured.

## Middleware and plugins

The web-chat command has both middleware definitions and chat plugins:

- `internal/middlewaredefs` defines middleware configuration schemas and builders. Its agent-mode definition consumes an `agentmode.Service` dependency.
- `internal/plugins/agentmode` translates agent-mode runtime events into app-visible sessionstream events, UI events, and timeline entities.
- Shared reasoning, tool-call, frontend-tool, and widget plugins come from `pkg/chatapp/...`.

## Durable stores

Timeline store:

- `--timeline-dsn "<sqlite dsn>"`
- `--timeline-db "path/to/timeline.db"`

Turn store:

- `--turns-dsn "<sqlite dsn>"`
- `--turns-db "path/to/turns.db"`

## Run

```bash
go generate ./cmd/web-chat
go run ./cmd/web-chat web-chat --addr :8080 --profile-registries ./profiles.yaml
```

Open `http://localhost:8080/`.

Example with a root prefix:

```bash
go run ./cmd/web-chat web-chat \
  --addr :8081 \
  --root /chat \
  --profile-registries ./profiles.yaml
```

With `--root /chat`, the backend serves the app under `/chat/` and exposes runtime config through both `/app-config.js` and `/chat/app-config.js`.

## Development with devctl

From the Pinocchio repository root:

```bash
devctl up --force
```

Then from `cmd/web-chat/web`:

```bash
npm run dev:url
```

Default restored URLs are normally:

- frontend: `http://127.0.0.1:5174/`
- backend profiles: `http://127.0.0.1:8092/api/chat/profiles`

## Validation

Backend-focused checks:

```bash
go test ./cmd/web-chat/... -count=1
```

Full pre-commit style checks are run by the repository hook on commit and include `go generate`, frontend build, `go build ./...`, lint/vet, and `go test ./...`.

Frontend checks from `cmd/web-chat/web`:

```bash
npm run typecheck
npm test
npm run lint
npm run build
npm run check:storybook
```

## Generated frontend protobuf bindings

Generated TypeScript protobuf files live under `web/src/generated/chatapp`. Do not edit them by hand. Regenerate from the Pinocchio repository root:

```bash
buf generate --template buf.chatapp.web.gen.yaml --path proto/pinocchio
```
