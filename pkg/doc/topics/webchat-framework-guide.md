---
Title: Building Web Chat Apps with the Webchat Framework
Slug: webchat-framework-guide
Short: End-to-end guide for the handler-first HTTP webchat setup.
Topics:
- webchat
- realtime
- middleware
- tools
- http
Commands:
- web-chat
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

The current webchat integration model is handler-first and app-owned:

- Your app owns transport routes like `/chat` and `/ws`.
- `pkg/webchat` provides services and helper handlers for those routes.
- `Router` remains useful for UI/core API helper mounting, but it is not the canonical owner of `/chat` and `/ws`.

Recommended baseline:

- Build `webchat.Server` with `webchat.NewServer(...)`.
- Register middleware and tool factories on the server.
- Mount app-owned handlers with:
  - `webchat.NewChatHTTPHandler(srv.ChatService(), resolver)`
  - `webchat.NewWSHTTPHandler(srv.StreamHub(), resolver, upgrader)`
  - `webchat.NewTimelineHTTPHandler(srv.TimelineService(), logger)`
- Optionally mount `srv.APIHandler()` and `srv.UIHandler()`.

## Core Pieces

- `Server`: lifecycle wrapper for event routing plus `http.Server` runtime.
- `ChatService`: submit prompt, queueing, idempotency, run start.
- `StreamHub`: conversation resolution and websocket attach lifecycle.
- `TimelineService`: hydration snapshots for timeline entities.
- `ConversationRequestResolver`: app policy for parsing request inputs into a `ConversationRequestPlan`.

## Canonical HTTP Contract

Routes below are the standard setup used by `cmd/web-chat` and `web-agent-example`.

- `POST /chat` and `POST /chat/{runtime}`: submit prompt/run request.
- `GET /ws?conv_id=<id>`: websocket streaming attach.
- `GET /api/timeline?conv_id=<id>&since_version=<n>&limit=<n>`: timeline hydration.
- `GET /api/debug/turns?...`: debug turn snapshots when turn store is configured.
- `GET /api/debug/timeline?...`: debug alias for timeline snapshot inspection.

Not canonical anymore:

- `/timeline` as a top-level default route.
- `/turns` as a top-level default route.
- `/hydrate` in webchat backend docs.

## Quick Start

```go
//go:embed static
var staticFS embed.FS

func run(ctx context.Context, parsed *values.Values) error {
  runtimeComposer := newWebChatRuntimeComposer(parsed, middlewareFactories)
  resolver := newWebChatProfileResolver(profiles)

  srv, err := webchat.NewServer(
    ctx,
    parsed,
    staticFS,
    webchat.WithRuntimeComposer(runtimeComposer),
  )
  if err != nil {
    return err
  }

  srv.RegisterMiddleware("agentmode", func(cfg any) geppettomw.Middleware {
    return agentmode.NewMiddleware(amSvc, cfg.(agentmode.Config))
  })

  srv.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
    return toolspkg.RegisterCalculatorTool(reg.(*geptools.InMemoryToolRegistry))
  })

  chatHandler := webchat.NewChatHTTPHandler(srv.ChatService(), resolver)
  wsHandler := webchat.NewWSHTTPHandler(
    srv.StreamHub(),
    resolver,
    websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
  )
  timelineHandler := webchat.NewTimelineHTTPHandler(
    srv.TimelineService(),
    log.With().Str("component", "webchat").Str("route", "/api/timeline").Logger(),
  )

  mux := http.NewServeMux()
  mux.HandleFunc("/chat", chatHandler)
  mux.HandleFunc("/chat/", chatHandler)
  mux.HandleFunc("/ws", wsHandler)
  mux.HandleFunc("/api/timeline", timelineHandler)
  mux.HandleFunc("/api/timeline/", timelineHandler)
  mux.Handle("/api/", srv.APIHandler())
  mux.Handle("/", srv.UIHandler())

  httpSrv := srv.HTTPServer()
  httpSrv.Handler = mux
  return srv.Run(ctx)
}
```

## Request Resolver Contract

`ConversationRequestResolver` is shared by both chat and websocket handlers. It produces `ConversationRequestPlan`:

- `ConvID`
- `RuntimeKey`
- `Overrides`
- `Prompt` (chat path)
- `IdempotencyKey`

Use this to keep runtime/profile/cookie policy in app code instead of framework internals.

## Root Prefix Mounting

For `--root /chat`, mount your app mux under a parent with `StripPrefix` and keep handler internals unchanged.

```go
parent := http.NewServeMux()
prefix := "/chat/"
parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), mux))
httpSrv.Handler = parent
```

Under a root prefix, effective endpoints become:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`
- `/chat/api/debug/turns`

## Timeline and Turns Persistence

Enable durable timeline snapshots:

- `--timeline-dsn "<sqlite dsn>"`
- `--timeline-db "<path/to/timeline.db>"`

Enable durable turn snapshots:

- `--turns-dsn "<sqlite dsn>"`
- `--turns-db "<path/to/turns.db>"`

Turn snapshots are debug-facing and served via `/api/debug/turns`.

## Router Usage in 2026

`Router` still matters for:

- `UIHandler()` static UI assets.
- `APIHandler()` core `/api/timeline` and `/api/debug/*` utilities.
- `Mount()` convenience when embedding helper mux behavior.

`Router` is no longer the primary route owner for `/chat` and `/ws` in top-level docs.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `POST /chat` returns 500 with resolver message | App resolver failed policy resolution | Return `RequestResolutionError` with explicit status/client message |
| WebSocket connects but no timeline hydration | UI is calling old `/timeline` path | Move hydration fetches to `/api/timeline` |
| Turns endpoint returns 404 | Turn store not configured | Start with `--turns-db` or `--turns-dsn` |
| Timeline endpoint returns 404 | Timeline service unavailable | Ensure `srv.TimelineService()` exists and route is mounted |

## See Also

- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat User Guide](webchat-user-guide.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
- [Webchat Debugging and Ops](webchat-debugging-and-ops.md)
