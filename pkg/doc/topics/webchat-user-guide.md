---
Title: Webchat User Guide
Slug: webchat-user-guide
Short: Practical guide for wiring the new HTTP chat setup in your app.
Topics:
- webchat
- backend
- frontend
- guide
Commands:
- web-chat
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

## What This Guide Covers

This guide shows the current production pattern for integrating `pinocchio/pkg/webchat`:

- app-owned `/chat` and `/ws` handlers
- canonical timeline hydration path `/api/timeline`
- debug endpoints under `/api/debug/*`
- profile/runtime policy via app-owned request resolver
- middleware resolved through app-owned runtime/profile composition
- explicit dependency-injected server construction as the preferred embedding API

## Minimal Backend Wiring

```go
//go:embed static
var staticFS embed.FS

func run(ctx context.Context, parsed *values.Values) error {
  middlewareDefinitions := newMiddlewareDefinitionRegistry()
  runtimeComposer := newRuntimeComposer(parsed, middlewareDefinitions)
  resolver := newRequestResolver()

  deps, err := webchat.BuildRouterDepsFromValues(ctx, parsed, staticFS)
  if err != nil {
    return err
  }

  srv, err := webchat.NewServerFromDeps(
    ctx,
    deps,
    webchat.WithRuntimeComposer(runtimeComposer),
  )
  if err != nil {
    return err
  }

  chatHandler := webhttp.NewChatHandler(srv.ChatService(), resolver)
  wsHandler := webhttp.NewWSHandler(
    srv.StreamHub(),
    resolver,
    websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
  )
  timelineHandler := webhttp.NewTimelineHandler(
    srv.TimelineService(),
    log.With().Str("component", "webchat").Str("route", "/api/timeline").Logger(),
  )

  mux := http.NewServeMux()
  mux.HandleFunc("/chat", chatHandler)
  mux.HandleFunc("/chat/", chatHandler)
  mux.HandleFunc("/ws", wsHandler)
  mux.HandleFunc("/api/timeline", timelineHandler)
  mux.HandleFunc("/api/timeline/", timelineHandler)
  webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
    DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
    EnableCurrentProfileCookieRoute: true,
    MiddlewareDefinitions:           middlewareDefinitions,
  })
  mux.Handle("/api/", srv.APIHandler())
  mux.Handle("/", srv.UIHandler())

  httpSrv := srv.HTTPServer()
  httpSrv.Handler = mux
  return srv.Run(ctx)
}
```

Compatibility wrapper:

```go
srv, err := webchat.NewServer(ctx, parsed, staticFS, webchat.WithRuntimeComposer(runtimeComposer))
```

That remains supported, but it now acts as a parsed-values adapter around the dependency-injected constructor path.

## Route Contract

Use [Webchat HTTP Chat Setup](webchat-http-chat-setup.md) as the canonical route table and request/response shape reference.

At a high level, you will mount:

- `POST /chat` (and optionally `POST /chat/{profile}`)
- `GET /ws?conv_id=<id>` (WebSocket upgrade)
- `GET /api/timeline?conv_id=<id>&since_version=<n>&limit=<n>` (hydration)

## Request Policy Ownership

Runtime and profile policy is app-owned through `ConversationRequestResolver`.

Typical resolver behavior:

- parse body/query/path/cookies
- select runtime/profile key
- merge default overrides and request overrides
- enforce override policy
- return typed `RequestResolutionError` for client-visible errors

For the full registry model (selection precedence, CRUD endpoints, and policy/version semantics), use [Webchat Profile Registry Guide](webchat-profile-registry.md) as the authoritative reference.

## Middleware Ownership

Middleware registration is no longer a server-level `pkg/webchat` API.

Use this split instead:

- declare middleware schemas/builders in an app-owned `middlewarecfg.DefinitionRegistry`
- resolve enabled middleware in your runtime composer
- expose the same definition registry through `webhttp.RegisterProfileAPIHandlers(...)` so profile CRUD and schema routes stay aligned

For the migration details, see [Webchat Compatibility Surface Migration Guide](webchat-compatibility-surface-migration-guide.md).

## Timeline Hydration

Hydration should always call `/api/timeline`.

- `conv_id` is required.
- `since_version` and `limit` are optional.
- versions align with streaming sequence semantics.

## Turns Debugging

Enable turn snapshots:

- `--turns-dsn`
- `--turns-db`

Query:

- `GET /api/debug/turns?conv_id=<id>&session_id=<id>&phase=<phase>&since_ms=<ms>&limit=<n>`

## Root Prefix

If the app is mounted with `--root /chat`, prepend `/chat` to all endpoints:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`
- `/chat/api/debug/turns`

## Frontend Expectations

Frontend code should:

- call `POST /chat`
- connect to `/ws`
- hydrate from `/api/timeline`
- treat `/api/debug/*` as diagnostics-only

## Constructor Choice

Use these rules when embedding:

- if your app already has explicit infrastructure objects, call `webchat.NewServerFromDeps(...)` or `webchat.NewRouterFromDeps(...)`;
- if your app still begins from `*values.Values`, call `webchat.BuildRouterDepsFromValues(...)` and then the explicit constructor;
- keep `webchat.NewServer(...)` and `webchat.NewRouter(...)` only as convenience wrappers or migration bridges.

## See Also

- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Compatibility Surface Migration Guide](webchat-compatibility-surface-migration-guide.md)
- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat Values Separation Migration Guide](webchat-values-separation-migration-guide.md)
- [Webchat Profile Registry Guide](webchat-profile-registry.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
