# Tasks

## TODO

- [x] Reproduce and log exact failing mode for local setup (`:8081` backend, `:5714` frontend), including whether `--debug-api` and `--root /chat` are set
- [x] Refactor debug UI API client (`debugApi`) to prepend runtime base prefix from location (same pattern as `profileApi`) so `/chat/api/debug/*` works
- [x] Add frontend tests for debug API URL construction under root-mounted paths (e.g. pathname `/chat/...`)
- [x] Add/adjust backend + CLI docs to make debug-route gate explicit (`--debug-api`) and reduce 404 ambiguity
- [ ] Add UI-facing error handling for "debug endpoints unavailable" so users get a clear message instead of generic fetch failure
- [x] Update `cmd/web-chat/web/vite.config.ts` docs and runbook to emphasize `VITE_BACKEND_ORIGIN=http://localhost:8081` when backend is not on `:8080`
- [ ] Add smoke/integration test coverage for debug endpoint access when mounted under `--root /chat` (`/chat/api/debug/conversations`)
- [x] Validate end-to-end manually with backend on `:8081` and Vite on `:5714`, and record results in ticket changelog/diary
