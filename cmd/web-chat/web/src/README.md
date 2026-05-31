# Pinocchio web-chat frontend architecture

This directory contains the React/Vite frontend for `cmd/web-chat`. Keep it clean and opinionated: the production chat app uses the provider-backed runtime, while demo and legacy code are temporary until the cleanup plan in `CHATOVERLAY-009` is complete.

## Canonical boundaries

```text
src/
  app/          App entry, route-mode parsing, and root composition.
  features/     Target home for production feature modules as cleanup proceeds.
  shared/       Target home for reusable UI, utilities, and test fixtures.
  generated/    Target home for generated protobuf/client code.
  legacy/       Target home for code kept only until parity/deletion.
```

The current tree is in transition. Some canonical files still live under `webchat/`, `chat/provider/`, `store/`, and `ws/`; do not copy those layouts into new code without checking the migration checklist.

## Runtime decisions

- **Canonical production chat runtime:** `@go-go-golems/chat-provider` via the provider-backed web-chat shell.
- **Temporary demo code:** provider capability demo routes/tools/widgets are scaffolding and should be deleted once replacement production tests exist.
- **Temporary legacy code:** Redux/WebSocket chat runtime code is kept only until the provider-backed app passes the parity checklist, then it should be deleted.
- **Debug UI:** the debug UI is a separate diagnostic app and should keep a clear store/runtime boundary from production web-chat.

## Development URLs

`devctl` may use default ports or free ephemeral ports. To find the actual URL, run:

```bash
cd /home/manuel/workspaces/2026-05-29/chatbot-react/pinocchio/cmd/web-chat/web
npm run dev:url
```

## Validation baseline

Before and after refactors, run at least:

```bash
npm run typecheck
npm run lint
npm run build
npm run build-storybook
```

For end-to-end validation, run the Pinocchio web-chat Playwright smokes from the chat-overlay ticket workspace.
