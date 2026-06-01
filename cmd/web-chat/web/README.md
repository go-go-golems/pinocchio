# Pinocchio web-chat frontend

This is the opinionated React + Storybook frontend for `cmd/web-chat`. The production chat route is provider-backed through `@go-go-golems/chat-provider`; the old Redux/WebSocket chat runtime has been removed.

For the Go backend package map, API routes, and runtime composition flow, read `../README.md` first.

## Package manager

Use **npm** in this directory. `package-lock.json` is the canonical lockfile; do not add `pnpm-lock.yaml` here.

## Local chat-provider dependency

`@go-go-golems/chat-provider` is currently linked through a local file dependency:

```json
"@go-go-golems/chat-provider": "file:../../../../2026-05-29--chatbot-overlay-glm/packages/chat-provider"
```

This is temporary while the provider package is developed in the sibling workspace. Replace it with a released package version once the Pinocchio core/provider package set is published.

## Development with devctl

Start the backend and Vite frontend from the Pinocchio repository root:

```bash
cd /home/manuel/workspaces/2026-05-29/chatbot-react/pinocchio
devctl up --force
```

Print the actual URLs selected by devctl:

```bash
cd cmd/web-chat/web
npm run dev:url
```

Default restored URLs are normally:

- web-chat: `http://127.0.0.1:5174/`
- backend profiles: `http://127.0.0.1:8092/api/chat/profiles`

When a default port is busy, trust `npm run dev:url` or `.devctl/state.json` instead of hardcoding ports.

## Validation

```bash
npm run typecheck
npm test
npm run lint
npm run build
npm run check:storybook
```

## Generated protobuf bindings

Generated TypeScript protobuf files live under `src/generated/chatapp`. Do not edit them by hand. Regenerate from the Pinocchio repository root:

```bash
buf generate --template buf.chatapp.web.gen.yaml --path proto/pinocchio
```
