---
Title: Third-Party Webchat Playbook
Slug: thirdparty-webchat-playbook
Short: End-to-end tutorial for embedding Pinocchio webchat with the handler-first HTTP API.
Topics:
- webchat
- middleware
- widgets
- timeline
- streaming
- thirdparty
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

## 1. What You Build

This playbook walks through building a third-party webchat app that:

- uses app-owned `POST /chat` and `GET /ws`
- hydrates with `GET /api/timeline`
- persists optional debug turn snapshots via `/api/debug/turns`
- adds custom middleware and custom timeline widgets

## 2. Current Integration Rule

Use handler-first setup as default.

- Build server with `webchat.NewServer(...)`.
- Mount `/chat` with `webchat.NewChatHTTPHandler`.
- Mount `/ws` with `webchat.NewWSHTTPHandler`.
- Mount `/api/timeline` with `webchat.NewTimelineHTTPHandler`.
- Optionally mount `srv.APIHandler()` and `srv.UIHandler()`.

Do not start from router-era examples that center `NewRouter + NewFromRouter`.

## 3. Backend Skeleton

```go
srv, err := webchat.NewServer(ctx, parsed, staticFS,
  webchat.WithRuntimeComposer(runtimeComposer),
)
if err != nil {
  return err
}

resolver := newRequestResolver()

chatHandler := webchat.NewChatHTTPHandler(srv.ChatService(), resolver)
wsHandler := webchat.NewWSHTTPHandler(
  srv.StreamHub(),
  resolver,
  websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
)
timelineHandler := webchat.NewTimelineHTTPHandler(
  srv.TimelineService(),
  log.With().Str("component", "my-webchat").Str("route", "/api/timeline").Logger(),
)

mux := http.NewServeMux()
mux.HandleFunc("/chat", chatHandler)
mux.HandleFunc("/chat/", chatHandler)
mux.HandleFunc("/ws", wsHandler)
mux.HandleFunc("/api/timeline", timelineHandler)
mux.HandleFunc("/api/timeline/", timelineHandler)
mux.Handle("/api/", srv.APIHandler())
mux.Handle("/", srv.UIHandler())

srv.HTTPServer().Handler = mux
return srv.Run(ctx)
```

## 4. Request Policy

Create an app resolver implementing `ConversationRequestResolver`.

Resolver should:

- parse request body/query/path/cookies
- resolve `RuntimeKey`
- set/generate `ConvID`
- merge default and request overrides
- return typed validation errors (`RequestResolutionError`)

The same resolver is used for chat and websocket flows.

## 5. Endpoint Contract

Canonical routes:

- `POST /chat`
- `POST /chat/{runtime}`
- `GET /ws?conv_id=<id>`
- `GET /api/timeline?conv_id=<id>&since_version=<n>&limit=<n>`
- `GET /api/debug/turns?...` (optional)

Non-canonical routes to avoid in client code:

- `/timeline`
- `/turns`
- `/hydrate`

## 6. Timeline and Turn Storage

Timeline durability:

- `--timeline-dsn`
- `--timeline-db`

Turn snapshot durability:

- `--turns-dsn`
- `--turns-db`

Turn snapshots are for debugging model input/output details and should be consumed from `/api/debug/turns`.

## 7. Middleware Registration

Register middleware factories on the server:

```go
srv.RegisterMiddleware("webagent-thinking-mode", func(cfg any) geppettomw.Middleware {
  return thinkingmode.NewMiddleware(thinkingmode.ConfigFromAny(cfg))
})
```

Use resolver defaults or request overrides to activate middleware per run.

## 8. Tool Registration

Register tools globally:

```go
srv.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
  return toolspkg.RegisterCalculatorTool(reg.(*geptools.InMemoryToolRegistry))
})
```

## 9. Frontend Transport Pattern

The frontend should do this sequence:

1. Determine `conv_id`.
2. Open websocket on `/ws?conv_id=...`.
3. Run hydration fetch from `/api/timeline`.
4. Replay buffered websocket frames.
5. Send prompts via `POST /chat`.

This ensures ordering consistency across reloads and reconnects.

## 10. Frontend Override Payload

```json
{
  "conv_id": "conv-123",
  "prompt": "hello",
  "overrides": {
    "middlewares": [
      { "name": "webagent-thinking-mode", "config": { "mode": "fast" } },
      { "name": "webagent-disco-dialogue", "config": { "tone": "noir" } }
    ]
  }
}
```

## 11. Custom Middleware Event Flow

To add a custom UI feature:

1. middleware emits typed events
2. SEM mapping translates events to stream frames
3. timeline projection maps events to snapshot entities
4. frontend renderer displays entity kind

Recommended references:

- `web-agent-example/pkg/thinkingmode/*`
- `web-agent-example/pkg/discodialogue/*`

## 12. Root Prefix Mounting

With `--root /chat`, all endpoints are served under `/chat`.

Examples:

- `/chat/chat`
- `/chat/ws`
- `/chat/api/timeline`

Frontend should compute and apply this base prefix automatically.

## 13. Vite Proxy Setup

For frontend dev mode, proxy backend routes:

```ts
proxy: {
  '/chat': { target: 'http://localhost:8080', changeOrigin: true },
  '/ws': { target: 'ws://localhost:8080', ws: true, changeOrigin: true },
  '/api': { target: 'http://localhost:8080', changeOrigin: true },
}
```

## 14. Smoke Test Script

1. Open UI and start a conversation.
2. Send a prompt and verify streaming text.
3. Refresh page and verify timeline hydration from `/api/timeline`.
4. If turn store enabled, query `/api/debug/turns` for the same conversation.

## 15. Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| No streaming events | Wrong websocket URL or missing `conv_id` | Verify `/ws?conv_id=<id>` |
| Timeline empty after refresh | Hydration still using `/timeline` | Update frontend to `/api/timeline` |
| 404 on turns endpoint | Turn store disabled | Start with `--turns-db` or `--turns-dsn` |
| Unexpected runtime/profile | Resolver policy mismatch | Log and validate resolved `ConversationRequestPlan` |

## 16. Migration Notes for Older Integrations

If migrating from old docs:

- replace all top-level `/timeline` calls with `/api/timeline`
- replace `/turns` references with `/api/debug/turns`
- remove `/hydrate` assumptions
- replace router-centric startup snippets with handler-centric snippets

## 17. Production Checklist

- resolver returns explicit typed policy errors
- websocket and chat endpoints share the same resolver rules
- timeline store persistence configured
- turn debug persistence gated and access-controlled
- frontend hydration gate enabled
- stale path aliases removed from clients and docs

## 18. See Also

- [Webchat HTTP Chat Setup](../topics/webchat-http-chat-setup.md)
- [Webchat Framework Guide](../topics/webchat-framework-guide.md)
- [Webchat Frontend Integration](../topics/webchat-frontend-integration.md)
- [Webchat Debugging and Ops](../topics/webchat-debugging-and-ops.md)
