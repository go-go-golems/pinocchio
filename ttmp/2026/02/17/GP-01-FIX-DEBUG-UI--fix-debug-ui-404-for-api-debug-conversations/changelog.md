# Changelog

## 2026-02-17

- Initial workspace created
- Added investigation document with root-cause matrix for debug-route gating, root-prefix mismatch, and Vite backend-origin mismatch
- Added detailed diary with command-level exploration notes and observed failures
- Replaced placeholder tasks with concrete implementation/test/docs checklist for fixing debug UI endpoint reachability
- Uploaded analysis document to reMarkable at `/ai/2026/02/17/GP-01-FIX-DEBUG-UI` and verified remote listing
- Implemented root-aware debug-ui API base query so `/chat/api/debug/*` works under `--root /chat`
- Replaced env-based debug route enablement with Glazed flag `--debug-api` in `cmd/web-chat`
- Removed per-handler env checks for step-control endpoints; debug gating now centralized at router registration
- Updated `cmd/web-chat` README with `--debug-api` usage and `:8081`/`:5714` runbook
- Validated with `go test ./pkg/webchat`, `go test ./cmd/web-chat/...`, and frontend `npm run check`
- Reproduced reported issue in tmux with exact commands and confirmed `/api/debug/*` 404 vs `/chat/api/debug/*` 200
- Added debugApi fallback retry (`/api/debug/*` -> `/chat/api/debug/*` on 404) plus runtime prefix cache
- Added regression test `src/debug-ui/api/debugApi.test.ts` covering fallback and sticky-prefix behavior
- Added explicit runtime config channel via `app-config.js` for both Go-served and Vite-served modes
- Wired frontend prefix consumers to runtime config (`basePrefix`) and debug router basename
- Added `src/utils/basePrefix.test.ts` and rebuilt bundled frontend assets in `static/dist`
