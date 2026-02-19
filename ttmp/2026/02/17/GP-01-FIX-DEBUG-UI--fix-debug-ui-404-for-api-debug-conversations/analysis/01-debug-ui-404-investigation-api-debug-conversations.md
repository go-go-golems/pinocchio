---
Title: Debug UI 404 Investigation (api/debug/conversations)
Ticket: GP-01-FIX-DEBUG-UI
Status: active
Topics:
    - bug
    - analysis
    - chat
    - backend
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/README.md
      Note: Route prefix behavior under --root /chat
    - Path: pinocchio/cmd/web-chat/main.go
      Note: |-
        Debug-route enable gate and root mount behavior
        Implemented Glazed --debug-api flag wiring
    - Path: pinocchio/cmd/web-chat/static/dist/index.html
      Note: Bundled output now includes app-config.js bootstrap script
    - Path: pinocchio/cmd/web-chat/web/src/config/runtimeConfig.ts
      Note: Runtime prefix channel source for TS app
    - Path: pinocchio/cmd/web-chat/web/src/debug-ui/api/debugApi.test.ts
      Note: Evidence that fallback from /api/debug to /chat/api/debug is covered
    - Path: pinocchio/cmd/web-chat/web/src/debug-ui/api/debugApi.ts
      Note: |-
        Absolute base URL currently causes root-prefix mismatch
        Implemented root-aware debug API base query wrapper
    - Path: pinocchio/cmd/web-chat/web/src/store/profileApi.ts
      Note: Prefix-aware API pattern to reuse
    - Path: pinocchio/cmd/web-chat/web/src/utils/basePrefix.ts
      Note: Runtime prefix cache support used by debugApi fallback
    - Path: pinocchio/cmd/web-chat/web/vite.config.ts
      Note: Backend proxy origin defaults and override via VITE_BACKEND_ORIGIN
    - Path: pinocchio/pkg/webchat/router.go
      Note: Conditional debug handler registration
    - Path: pinocchio/pkg/webchat/router_debug_api_test.go
      Note: Evidence for enabled/disabled debug route behavior
    - Path: pinocchio/pkg/webchat/router_debug_routes.go
      Note: Conversation debug endpoint handlers
ExternalSources: []
Summary: Investigate why debug UI receives 404 on /api/debug/conversations and define concrete fixes; updated with flag/path changes, root-load fallback, and explicit runtime app-config prefix channel.
LastUpdated: 2026-02-17T14:01:00-05:00
WhatFor: Diagnose route, mount, and dev-proxy causes of the 404 and provide implementation-ready fix tasks.
WhenToUse: Use when debugging debug-ui API failures in cmd/web-chat, especially with custom root mounts and non-default ports.
---





# Debug UI 404 Investigation

## Reported Symptom

- Debug UI request to `GET /api/debug/conversations` returns HTTP 404.
- Active dev setup uses backend on `:8081` and frontend Vite on `:5714`.

## Key Findings

### 1) Debug routes are explicitly gated by command flag

- `cmd/web-chat` enables debug route registration when `--debug-api` is set:
  - `pinocchio/cmd/web-chat/main.go`
- API registration only mounts debug handlers when that flag-driven option is enabled:
  - `pinocchio/pkg/webchat/router.go`
- When disabled, `/api/debug/*` is absent and returns 404 by design:
  - `pinocchio/pkg/webchat/router_debug_api_test.go` (`TestAPIHandler_DebugRoutesDisabled`)

### 2) Frontend debug API ignores URL root prefix

- Debug API client is hardcoded to absolute `/api/debug/`:
  - `pinocchio/cmd/web-chat/web/src/debug-ui/api/debugApi.ts:253`
- Backend supports mounting under custom root (`--root /chat`), which prefixes all routes:
  - `pinocchio/cmd/web-chat/main.go:178`
  - `pinocchio/cmd/web-chat/README.md:100`
- Under root mount, canonical path becomes `/chat/api/debug/conversations`, but frontend still requests `/api/debug/conversations`.
- This causes 404 even when debug routes are enabled.

### 3) Vite backend origin defaults to :8080 unless overridden

- Vite proxy target is `process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080'`:
  - `pinocchio/cmd/web-chat/web/vite.config.ts:18`
- If backend is actually on `:8081` and `VITE_BACKEND_ORIGIN` is not set, debug API calls may hit the wrong backend and fail.

## Reproduction Matrix (2026-02-17)

1. Debug API flag off:
   - Backend command omits `--debug-api`.
   - Result: `/api/debug/conversations` returns 404 by intended server behavior.
2. Root mount mismatch:
   - Backend started with `--root /chat`, debug flag on.
   - Frontend still calls absolute `/api/debug/conversations`.
   - Result: 404 (correct path is `/chat/api/debug/conversations`).
3. Dev proxy target mismatch:
   - Backend runs on `:8081`.
   - Frontend Vite uses default proxy target `:8080`.
   - Result: request reaches wrong service and can return 404.

## Commands Used During Investigation

```bash
rg -n "debug/conversations|/api/debug|enableDebugRoutes" pinocchio/cmd/web-chat pinocchio/pkg/webchat
cd pinocchio && go test ./pkg/webchat -run 'TestAPIHandler_DebugRoutesDisabled|TestAPIHandler_DebugConversationsAndDetail' -count=1
```

Result:
- Route handlers exist.
- Tests confirm debug-route enabled/disabled behavior.

## Proposed Fix Direction

1. Make debug UI API base path root-aware:
   - Mirror `profileApi` pattern that prepends `basePrefixFromLocation()`.
   - Add fallback retry from `/api/debug/*` to `/chat/api/debug/*` when UI is loaded at `/` and backend is mounted under `/chat`.
2. Keep debug route gate explicit, but improve operator clarity:
   - Document startup requirement `--debug-api`.
   - Optionally return structured debug-disabled signal for UI.
3. Improve dev setup docs/scripts for custom ports:
   - Backend `:8081`, frontend `:5714`, with `VITE_BACKEND_ORIGIN=http://localhost:8081`.
4. Add regression tests:
   - Frontend URL construction for `/chat` prefix.
   - Integration/smoke coverage for root-mounted debug endpoints.

## Implemented Delta (2026-02-17)

- `--debug-api` Glazed flag replaces env gating for debug routes.
- Debug API client now:
  - prefixes with `basePrefixFromLocation()` when available
  - retries `/chat/api/debug/*` after 404 when mounted at `/` in dev
  - caches discovered runtime prefix for subsequent requests
- Added unit test proving fallback from `/api/debug/conversations` to `/chat/api/debug/conversations`:
  - `pinocchio/cmd/web-chat/web/src/debug-ui/api/debugApi.test.ts`

## Runtime Prefix Channel (Implemented)

To avoid implicit prefix guessing, runtime now communicates prefix through `app-config.js`:

- Go app serves `/<root>/app-config.js` with:
  - `basePrefix` derived from `--root`
  - `debugApiEnabled` derived from `--debug-api`
- Vite dev server serves `/app-config.js` from:
  - `VITE_WEBCHAT_BASE_PREFIX` (for `basePrefix`)
  - `VITE_WEBCHAT_DEBUG_API` (optional, for `debugApiEnabled`)
- TS app reads `window.__PINOCCHIO_WEBCHAT_CONFIG__` at startup and uses it for API prefix + router basename decisions.

## Explicit Runbook for Your Local Setup

```bash
# Terminal 1 (backend)
cd pinocchio
go run ./cmd/web-chat --addr :8081 --root /chat --debug-api

# Terminal 2 (frontend dev server)
cd pinocchio/cmd/web-chat/web
VITE_BACKEND_ORIGIN=http://localhost:8081 npm run dev -- --port 5714
```

Then open `http://localhost:5714/chat/`.
