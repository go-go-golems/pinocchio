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

## Minimal Backend Wiring

```go
//go:embed static
var staticFS embed.FS

func run(ctx context.Context, parsed *values.Values) error {
  runtimeComposer := newRuntimeComposer(parsed)
  resolver := newRequestResolver()

  srv, err := webchat.NewServer(
    ctx,
    parsed,
    staticFS,
    webchat.WithRuntimeComposer(runtimeComposer),
  )
  if err != nil {
    return err
  }

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

## Route Contract

- `POST /chat` and `POST /chat/{runtime}`
- `GET /ws?conv_id=<id>`
- `GET /api/timeline?conv_id=<id>&since_version=<n>&limit=<n>`
- `GET /api/debug/turns?...` (when turn store enabled)

Legacy paths that should be removed from app docs and clients:

- `/timeline`
- `/turns`
- `/hydrate`

## Request Policy Ownership

Runtime and profile policy is app-owned through `ConversationRequestResolver`.

Typical resolver behavior:

- parse body/query/path/cookies
- select runtime/profile key
- merge default overrides and request overrides
- enforce override policy
- return typed `RequestResolutionError` for client-visible errors

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

## See Also

- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md)
- [Webchat Framework Guide](webchat-framework-guide.md)
- [Webchat Frontend Integration](webchat-frontend-integration.md)
