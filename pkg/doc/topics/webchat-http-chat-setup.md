---
Title: Webchat HTTP Chat Setup
Slug: webchat-http-chat-setup
Short: Canonical HTTP wiring for app-owned /chat and /ws with timeline hydration and debug APIs.
Topics:
- webchat
- http
- websocket
- api
- routes
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Goal

This page defines the canonical public HTTP contract for new webchat integrations.

Use this page as the source of truth when wiring server routes, frontend clients, and API docs.

For the reference `cmd/web-chat` application, runtime engine/provider settings now come exclusively from the resolved profile registry runtime. Direct `--ai-*` CLI overrides are no longer part of that command surface.

## Canonical Setup Pattern

- Build server core with `webchat.NewServerFromDeps(...)` when possible.
- Use `webchat.BuildRouterDepsFromValues(...)` if you start from parsed Glazed values.
- Keep `webchat.NewServer(...)` as the compatibility wrapper.
- Register tools on the returned server.
- Resolve middleware through app-owned runtime/profile composition.
- Mount app-owned handlers:
  - `webhttp.NewChatHandler(srv.ChatService(), resolver)`
  - `webhttp.NewWSHandler(srv.StreamHub(), resolver, upgrader)`
  - `webhttp.NewTimelineHandler(srv.TimelineService(), logger)`
- Mount reusable profile API handlers:
  - `webhttp.RegisterProfileAPIHandlers(...)`
- Mount utility handlers:
  - `srv.APIHandler()` for core `/api/*` utilities
  - `srv.UIHandler()` for static UI

## Route Table

| Route | Method | Ownership | Notes |
|---|---|---|---|
| `/chat` | POST | App | Submit prompt |
| `/chat/{profile}` | POST | App | Force profile/runtime selection from path |
| `/ws?conv_id=<id>` | GET (upgrade) | App | Attach websocket stream |
| `/api/timeline` | GET | App/Core | Hydration snapshot endpoint |
| `/api/chat/profiles` | `GET` | Shared (`pkg/webchat/http`) | List profiles |
| `/api/chat/profiles/{slug}` | `GET` | Shared (`pkg/webchat/http`) | Read one profile |
| `/api/chat/profile` | `GET`, `POST` | Shared (`pkg/webchat/http`) | Current-profile cookie route (optional) |
| `/api/chat/schemas/middlewares` | `GET` | Shared (`pkg/webchat/http`) | Middleware schema catalog |
| `/api/chat/schemas/extensions` | `GET` | Shared (`pkg/webchat/http`) | Extension schema catalog |
| `/api/debug/timeline` | GET | Core debug | Debug alias for snapshot inspection |
| `/api/debug/turns` | GET | Core debug | Requires turn store |
| `/api/debug/turn/:conv/:session/:turn` | GET | Core debug | Per-turn detail endpoint |
| `/` | GET | Core UI | Embedded UI index/assets |

## Profile API Wiring

Mount the shared read-only profile/schema handlers once per app mux:

```go
webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
  DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
  EnableCurrentProfileCookieRoute: true,
  MiddlewareDefinitions:           middlewareDefinitions,
  ExtensionCodecRegistry:          extensionCodecs,
})
```

This route set is app-agnostic and reusable across pinocchio `cmd/web-chat` and go-go-os backend servers.

## Request Resolver Contract

Handlers for `/chat` and `/ws` both depend on a `ConversationRequestResolver`.

The resolver returns `ResolvedConversationRequest` with:

- `ConvID`
- `RuntimeKey`
- `RuntimeFingerprint`
- `ProfileVersion`
- `ResolvedRuntime`
- `ProfileMetadata` (includes stack lineage/trace metadata)
- `Prompt`
- `IdempotencyKey`

Use resolver policy for:

- profile/runtime selection
- cookie or query-based defaults
- request validation and typed errors (`RequestResolutionError`)

## Request and Response Shapes

### POST /chat

Request body:

```json
{
  "prompt": "Classify these transactions",
  "conv_id": "optional-conversation-id",
  "profile": "optional-profile-slug",
  "registry": "optional-registry-slug",
  "idempotency_key": "optional-client-key"
}
```

Response shape is service-defined and may include queue metadata.
Current reference apps return start/queued metadata plus conversation/session IDs and runtime/profile metadata fields:

- `runtime_fingerprint`
- `profile_metadata` (including `profile.stack.lineage` and `profile.stack.trace`)

Legacy selector aliases are removed:

- `runtime_key`
- `registry_slug`

Middleware and tool activation are profile/runtime-composer concerns resolved before the run starts.

## Middleware Definition Wiring

Expose middleware catalogs through the shared profile API and resolve them in your app-owned runtime composer.

```go
middlewareDefinitions := newMiddlewareDefinitionRegistry()

runtimeComposer := newRuntimeComposer(parsed, middlewareDefinitions)

webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
  DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
  EnableCurrentProfileCookieRoute: true,
  MiddlewareDefinitions:           middlewareDefinitions,
})
```

There is no longer a server-level middleware registration API in `pkg/webchat`.

### GET /api/timeline

Query params:

- `conv_id` (required)
- `since_version` (optional)
- `limit` (optional)

Response is a `TimelineSnapshotV2` JSON payload.

## Persistence Flags

Timeline snapshots:

- `--timeline-dsn`
- `--timeline-db`

Turn snapshots:

- `--turns-dsn`
- `--turns-db`

Without explicit SQLite config, timeline may use in-memory storage and turn debug endpoints can be unavailable.

## Root Prefix Behavior

When you mount under `--root /chat`, prepend `/chat` to every route in the table.

Examples:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`
- `/chat/api/debug/turns`

## Deprecated or Non-Canonical Paths

Do not document or depend on these paths in new integrations:

- `/timeline` (top-level)
- `/turns` (top-level)
- `/hydrate`

## Migration Checklist

- Update frontend hydration fetches to `/api/timeline`.
- Update debug tooling to `/api/debug/turns`.
- Remove any `/hydrate` endpoint assumptions.
- Replace router-era setup examples (`NewRouter + NewFromRouter`) with `NewServer + webhttp handler constructors` in onboarding docs.
- Prefer `BuildRouterDepsFromValues(...) + NewServerFromDeps(...)` when migrating existing parsed-values integrations.
- Move any `srv.RegisterMiddleware(...)` usage into app-owned middleware definition registries plus runtime-composer wiring.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `missing conv_id` on websocket | Client omitted query param | Ensure `conv_id` is always included in websocket URL |
| `timeline service not enabled` | Timeline service unavailable or route not mounted | Mount `/api/timeline` with `webhttp.NewTimelineHandler` and confirm service non-nil |
| middleware schema endpoint is empty | No definition registry passed to profile API handlers | Pass `MiddlewareDefinitions` into `webhttp.RegisterProfileAPIHandlers(...)` |
| `turn store not enabled` | No turn store config | Start with `--turns-db` or `--turns-dsn` |
| Resolver returns `400` from `/chat` | Request selection or validation failed | Return `RequestResolutionError` with explicit status/client message |

## Sources of Truth (When Docs and Reality Disagree)

If you suspect drift between docs and behavior, these tend to be the most reliable references:

- Reference app wiring: `pinocchio/cmd/web-chat/main.go`
- Frontend hydration gate implementation: `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- HTTP helper contract tests: `pinocchio/pkg/webchat/http_helpers_contract_test.go`

## See Also

- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat Compatibility Surface Migration Guide](webchat-compatibility-surface-migration-guide.md)
- [Webchat Values Separation Migration Guide](webchat-values-separation-migration-guide.md)
- [Webchat User Guide](webchat-user-guide.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
