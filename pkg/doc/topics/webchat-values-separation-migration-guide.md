---
Title: Webchat Values Separation Migration Guide
Slug: webchat-values-separation-migration-guide
Short: How to migrate webchat embeddings from parsed-values constructors to explicit dependency-injected constructors.
Topics:
- webchat
- backend
- migration
- api
- docs
Commands:
- web-chat
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Goal

Explain how to migrate from the older parsed-values-centric constructor flow:

- `webchat.NewRouter(ctx, parsed, staticFS, ...)`
- `webchat.NewServer(ctx, parsed, staticFS, ...)`
- `webchat.NewStreamBackendFromValues(ctx, parsed)`

to the explicit dependency-injected flow introduced by the Values separation refactor.

## What Changed

`pkg/webchat` now exposes explicit constructor layers:

- `webchat.NewStreamBackend(ctx, redisSettings)`
- `webchat.BuildRouterDepsFromValues(ctx, parsed, staticFS)`
- `webchat.NewRouterFromDeps(ctx, deps, ...)`
- `webchat.NewServerFromDeps(ctx, deps, ...)`

Compatibility wrappers remain:

- `webchat.NewRouter(...)`
- `webchat.NewServer(...)`
- `webchat.NewStreamBackendFromValues(...)`

The wrappers still work, but they now delegate to the explicit constructors instead of decoding Glazed values inside the core constructor bodies.

## Why This Matters

This split gives embedders a cleaner boundary:

- apps can own parsed-values handling, config normalization, and infra creation;
- Pinocchio core receives already-resolved dependencies;
- router/server construction no longer needs to retain parsed values just to build the HTTP server later.

It also makes partial customization easier. An app can:

- decode Glazed values once;
- override one dependency;
- still construct the final server through `NewServerFromDeps(...)`.

## New API Surface

### Explicit stream backend

```go
backend, err := webchat.NewStreamBackend(ctx, rediscfg.Settings{
  Enabled: false,
})
```

Use this if your app already decoded Redis settings or wants to provide them from another config source.

### Parsed-values adapter

```go
deps, err := webchat.BuildRouterDepsFromValues(ctx, parsed, staticFS)
if err != nil {
  return err
}
```

This helper decodes:

- `RouterSettings`
- Redis stream settings
- timeline store configuration
- turn store configuration

and returns a `webchat.RouterDeps` value.

### Explicit router/server construction

```go
router, err := webchat.NewRouterFromDeps(
  ctx,
  deps,
  webchat.WithRuntimeComposer(runtimeComposer),
)
```

```go
srv, err := webchat.NewServerFromDeps(
  ctx,
  deps,
  webchat.WithRuntimeComposer(runtimeComposer),
)
```

These are the preferred constructors for new integrations.

## Before And After

### Before

```go
srv, err := webchat.NewServer(
  ctx,
  parsed,
  staticFS,
  webchat.WithRuntimeComposer(runtimeComposer),
)
if err != nil {
  return err
}
```

### After

```go
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
```

### Fully explicit variant

```go
streamBackend, err := webchat.NewStreamBackend(ctx, redisSettings)
if err != nil {
  return err
}

deps := webchat.RouterDeps{
  StaticFS:      staticFS,
  Settings:      webchat.RouterSettings{Addr: ":8080"},
  StreamBackend: streamBackend,
  TimelineStore: chatstore.NewInMemoryTimelineStore(0),
  TurnStore:     nil,
}

srv, err := webchat.NewServerFromDeps(
  ctx,
  deps,
  webchat.WithRuntimeComposer(runtimeComposer),
)
if err != nil {
  return err
}
```

## Migration Strategy

### Option 1: Low-risk incremental migration

1. Keep using `webchat.NewServer(...)`.
2. Make no behavior changes.
3. Migrate later when you need explicit dependency control.

This works because the old constructor remains supported.

### Option 2: Parsed-values bridge

1. Replace `webchat.NewServer(...)` with `webchat.BuildRouterDepsFromValues(...)`.
2. Pass the returned deps into `webchat.NewServerFromDeps(...)`.
3. Keep the rest of your app wiring unchanged.

This is the recommended migration path for current CLI and Glazed-based apps.

### Option 3: Full explicit construction

1. Decode config in your own app layer.
2. Create `StreamBackend`, timeline store, and turn store yourself.
3. Build `webchat.RouterDeps` directly.
4. Construct the router or server from deps.

Use this when embedding Pinocchio in another backend that does not want `pkg/webchat` to know about Glazed at all.

## Practical Notes

- `BuildHTTPServer()` now uses retained `RouterSettings` on the router instead of reaching back into parsed values.
- `RouterDeps.TimelineStore` is optional. If omitted, the router falls back to the in-memory default timeline store.
- `RouterDeps.TurnStore` is optional. If omitted, turn snapshot APIs remain unavailable unless a turn store is later attached through options.
- Runtime composition is still supplied through router options such as `webchat.WithRuntimeComposer(...)`.

## Checklist

- Replace direct `NewRouter(...)` or `NewServer(...)` usage with `BuildRouterDepsFromValues(...) + ...FromDeps(...)`.
- Keep middleware and tool registration unchanged.
- Keep app-owned `/chat` and `/ws` handler wiring unchanged.
- Verify `/api/timeline` and `/api/debug/*` behavior after the constructor change.
- If using custom store or stream backends, prefer building them before calling `NewRouterFromDeps(...)`.

## Verification

After migrating:

1. run `go test ./pkg/webchat/...`
2. run the embedding app’s integration or smoke tests
3. confirm the UI still serves, websocket attaches, and timeline hydration works

## See Also

- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat User Guide](webchat-user-guide.md)
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
