# Web-chat feature boundary

This folder is the target home for the canonical Pinocchio web-chat React feature. Code moved here should be provider-backed, production-oriented, and organized by feature/component rather than by migration history.

## Current status

Phase 2 of `CHATOVERLAY-009` has started the move by creating explicit provider-backed app folders:

```text
WebChatProviderShell/  ChatProvider config, profile bridge, and runtime shell.
WebChatApp/            Provider-backed web-chat chrome/body and provider renderers.
provider-support/      Small provider support helpers such as session URL sync and debug bridging.
extensions/            App-owned ChatProvider extensions and projectors.
demos/                 Temporary demo/test harnesses scheduled for deletion or test-only relocation.
```

Some imports still point back into `src/webchat/*` for visual components, CSS, parts, and types. That is expected until later phases split header/statusbar/composer/timeline/cards into one-folder-per-component modules.

## Rules for new code

- Prefer provider-backed runtime APIs from `@go-go-golems/chat-provider`.
- Do not add new imports from legacy `src/webchat/ChatWidget.tsx` or singleton `src/ws/wsManager.ts`.
- Do not add import-side-effect global registries.
- Keep demo-only code under `demos/` and plan to delete it in Phase 5.
- Keep production app-specific projectors/extensions under `extensions/`.

## Near-term cleanup

- Phase 3 moved visual components into `ChatHeader/`, `ChatStatusbar/`, `ChatComposer/`, and `ChatTimeline/`, each with focused Storybook coverage.
- Phase 4 split `cards.tsx` into card folders under `cards/`; `src/webchat/cards.tsx` remains a temporary compatibility barrel.
- Phase 5 will delete provider capability demo code.
- Phase 7 will delete legacy Redux/WebSocket chat after parity.
