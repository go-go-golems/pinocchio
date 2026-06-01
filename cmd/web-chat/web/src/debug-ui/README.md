# Debug UI boundary

The debug UI is an operator/developer surface for inspecting sessionstream frames and projected entities. It is intentionally separate from the production web-chat feature.

## Ownership rules

- Runtime entrypoint: `src/app/DebugUiRoot.tsx` selects this app only for `?debug=1`.
- UI/state lives under `src/debug-ui/**` and uses `src/debug-ui/store/**`.
- Do not import production web-chat Redux store types from `src/store/store.ts`.
- Shared transport helpers may come from `src/ws/protocol.ts` because both debug UI and web-chat diagnostics use the same sessionstream WebSocket protocol.
- CSS is imported by `DebugUIApp` for the runtime route and by debug-only Storybook stories that render individual debug components.

## Route decision

`?debug=1` remains available in the production dev server build for now. It is useful for local operator debugging and does not load unless explicitly requested by the query flag. If this becomes public-hosted UI, revisit this decision and gate the route behind a build-time/dev-only flag.
