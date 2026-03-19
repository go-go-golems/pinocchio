---
Title: Webchat Compatibility Surface Migration Guide
Slug: webchat-compatibility-surface-migration-guide
Short: How to migrate off the removed webchat compatibility APIs onto the app-owned handler-first embedding model.
Topics:
- webchat
- migration
- api
- backend
- middleware
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Goal

Explain how to migrate code that still relies on the removed webchat compatibility
surface:

- `webchat.NewFromRouter(...)`
- `(*webchat.Server).RegisterMiddleware(...)`
- `(*webchat.Router).RegisterMiddleware(...)`
- `(*webchat.Router).Mount(...)`
- `(*webchat.Router).Handle(...)`
- `(*webchat.Router).HandleFunc(...)`
- `(*webchat.Router).Handler()`

The supported model is now app-owned embedding:

- construct the webchat server/router from deps,
- mount app-owned `/chat`, `/ws`, and `/api/timeline` handlers explicitly,
- keep middleware definitions in an app-owned `middlewarecfg.DefinitionRegistry`,
- expose that same definition registry to the shared profile API handlers.

## What Changed

The compatibility helpers were removed because they encoded two outdated assumptions:

- middleware ownership lived inside `pkg/webchat`,
- `pkg/webchat` should also own generic mux composition for app routes.

That no longer matches the current architecture. `pkg/webchat` now owns the reusable
services and shared `/api/*` utilities, while the embedding application owns:

- request resolution,
- runtime/profile policy,
- `/chat` and `/ws` route registration,
- timeline route mounting,
- middleware-definition wiring.

## Removed API To Replacement Map

| Removed API | Replacement | Notes |
|---|---|---|
| `webchat.NewFromRouter(...)` | `webchat.NewServerFromDeps(...)` or `webchat.NewServer(...)` | Prefer the deps-first constructor for new code |
| `srv.RegisterMiddleware(...)` | app-owned `middlewarecfg.DefinitionRegistry` + runtime composer | Pass the same registry into `webhttp.RegisterProfileAPIHandlers(...)` |
| `router.RegisterMiddleware(...)` | app-owned `middlewarecfg.DefinitionRegistry` + runtime composer | Same replacement as server-level middleware registration |
| `router.Mount(mux)` | explicit `mux.HandleFunc(...)` / `mux.Handle(...)` calls | Mount `/chat`, `/ws`, `/api/timeline`, `/api/`, and `/` yourself |
| `router.Handle(...)` / `router.HandleFunc(...)` | call the parent mux directly | Keep route ownership in the app |
| `router.Handler()` | `srv.APIHandler()` and `srv.UIHandler()` plus app-owned handlers | There is no longer one combined compatibility handler |

## Canonical Embedding Pattern

```go
middlewareDefinitions, err := newWebChatMiddlewareDefinitionRegistry()
if err != nil {
  return err
}

runtimeComposer := newProfileRuntimeComposer(middlewareDefinitions, buildDeps, baseStepSettings)
requestResolver := newProfileRequestResolver(profileRegistry, gepprofiles.MustRegistrySlug("default"))

srv, err := webchat.NewServerFromDeps(ctx, deps, webchat.WithRuntimeComposer(runtimeComposer))
if err != nil {
  return err
}

chatHandler := webhttp.NewChatHandler(srv.ChatService(), requestResolver)
wsHandler := webhttp.NewWSHandler(
  srv.StreamHub(),
  requestResolver,
  websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
)
timelineHandler := webhttp.NewTimelineHandler(srv.TimelineService(), timelineLogger)

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
  WriteActor:                      "my-app",
  WriteSource:                     "http-api",
})

mux.Handle("/api/", srv.APIHandler())
mux.Handle("/", srv.UIHandler())

srv.HTTPServer().Handler = mux
```

This is the supported replacement for the old `router.Mount(...)` and
`router.Handler()` convenience model.

## Middleware Migration

If your old code looked like this:

```go
srv.RegisterMiddleware("agentmode", func(cfg any) geppettomw.Middleware {
  return agentmode.New(agentmode.ConfigFromAny(cfg))
})
```

move that behavior into a middleware definition registry:

```go
defs := middlewarecfg.NewInMemoryDefinitionRegistry()

_ = defs.RegisterDefinition(middlewarecfg.Definition{
  Name: "agentmode",
  Build: func(ctx context.Context, deps middlewarecfg.BuildDeps, cfg middlewarecfg.Config) (geppettomw.Middleware, error) {
    return agentmode.New(agentmode.ConfigFromAny(cfg.Config)), nil
  },
})
```

Then:

- inject `defs` into your runtime composer,
- use the runtime composer to resolve profile/runtime middleware inputs,
- pass `defs` to `webhttp.RegisterProfileAPIHandlers(...)` so schema and CRUD
  validation uses the same source of truth.

## Route Migration Checklist

- Replace `webchat.NewFromRouter(...)` with `webchat.NewServerFromDeps(...)` or keep `webchat.NewServer(...)` if you still want the parsed-values adapter.
- Delete any `srv.RegisterMiddleware(...)` / `router.RegisterMiddleware(...)` calls.
- Build an app mux explicitly.
- Mount:
  - `POST /chat`
  - optional `POST /chat/{profile}`
  - `GET /ws?conv_id=...`
  - `GET /api/timeline`
  - `srv.APIHandler()` under `/api/`
  - `srv.UIHandler()` under `/`
- Register profile/schema handlers with `webhttp.RegisterProfileAPIHandlers(...)`.
- Confirm your request resolver uses `profile` / `registry` selectors and rejects `runtime_key` / `registry_slug`.
- Confirm debug consumers read `resolved_runtime_key`, not `current_runtime_key`.

## Troubleshooting

| Problem | Cause | Fix |
|---|---|---|
| profile CRUD accepts middleware names but schema endpoint is empty | definition registry not passed to profile API handlers | pass `MiddlewareDefinitions` to `webhttp.RegisterProfileAPIHandlers(...)` |
| `/chat` works but websocket/timeline feel detached | app only mounted the chat handler | mount `/ws` and `/api/timeline` explicitly on the same mux |
| old embedding expected one `router.Handler()` | combined compatibility handler no longer exists | mount `srv.APIHandler()`, `srv.UIHandler()`, and app-owned handlers separately |
| clients still send `runtime_key` or `registry_slug` | old request contract not updated | switch clients to `profile` and `registry` |
| debug tools still read `current_runtime_key` | old debug contract assumption | switch to `resolved_runtime_key` or use per-turn `runtime_key` from `/api/debug/turns` |

## Verification

After migrating:

1. run `go test ./pkg/webchat ./cmd/web-chat`
2. submit a `POST /chat` request with a `profile` selector
3. connect `GET /ws?conv_id=...`
4. query `GET /api/timeline?conv_id=...`
5. query `GET /api/chat/schemas/middlewares`
6. query `GET /api/debug/conversations/:id` and confirm `resolved_runtime_key`

## See Also

- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Profile Registry Guide](webchat-profile-registry.md)
- [Webchat Values Separation Migration Guide](webchat-values-separation-migration-guide.md)
