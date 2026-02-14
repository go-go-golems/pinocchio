---
Title: Webchat User Guide
Slug: webchat-user-guide
Short: Practical guide to using the Pinocchio webchat packages in your app.
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

# Webchat User Guide

This guide explains how to use the `pinocchio/pkg/webchat` packages to build a reusable chat backend and integrate it with a frontend. Runtime/profile policy is owned by your app via a request resolver.

## Package Overview

Key backend packages:

- `pinocchio/pkg/webchat` — router, conversation lifecycle, timeline hydration, WS streaming.
- `geppetto/pkg/inference/session` — session + turn lifecycle.
- `geppetto/pkg/inference/middleware` — middleware chain (system prompt, tool reordering, etc.).

Key frontend pieces (example app):

- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts` — WS connect + hydration flow.
- `pinocchio/cmd/web-chat/web/src/sem/*` — SEM frame parsing and routing.
- `pinocchio/cmd/web-chat/web/src/webchat/*` — reusable UI widget.

## Minimal Backend Wiring

The `web-chat` command is the reference implementation. To embed webchat in your own app:

```go
//go:embed static
var staticFS embed.FS

func run(ctx context.Context, parsed *layers.ParsedLayers) error {
    profiles := newChatProfileRegistry(
        "default",
        &chatProfile{Slug: "default", DefaultPrompt: "You are an assistant"},
    )

    r, err := webchat.NewRouter(
        ctx,
        parsed,
        staticFS,
        webchat.WithConversationRequestResolver(newWebChatProfileResolver(profiles)),
    )
    if err != nil {
        return err
    }

    // Register middlewares and tools
    r.RegisterMiddleware("agentmode", func(cfg any) middleware.Middleware {
        return agentmode.NewMiddleware(cfg.(agentmode.Config))
    })
    r.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
        return toolspkg.RegisterCalculatorTool(reg.(*geptools.InMemoryToolRegistry))
    })

    // Optional app-owned profile endpoints
    registerProfileHandlers(r, profiles)

    // Build and run HTTP server
    srv, err := r.BuildHTTPServer()
    if err != nil {
        return err
    }
    return webchat.NewFromRouter(ctx, r, srv).Run(ctx)
}
```

## Runtime Policy

`pkg/webchat` is profile-agnostic. Your app defines request policy via `ConversationRequestResolver`, including:
- runtime key selection (`RuntimeKey`)
- default prompt/middleware/tool composition
- override allow/deny policy
- optional profile/cookie endpoints for UI convenience

## HTTP + WebSocket API

### WebSocket

```
GET /ws?conv_id=<uuid>&runtime=<key>
```

The server sends SEM frames over this socket. A `ws.hello` frame is emitted on connect.

### Chat POST

```
POST /chat
{
  "conv_id": "<uuid>",
  "prompt": "hello",
  "overrides": { ... }
}
```

If the prompt is empty, the session still starts because the seed turn contains the system prompt block.

### Optional app-owned profile endpoints

- `GET /api/chat/profiles` — list available profiles (app-owned, optional)
- `GET /api/chat/profile` — get current profile (app-owned, optional)
- `POST /api/chat/profile` — set current profile (app-owned, optional)

## Timeline Hydration

If a timeline store is configured (`--timeline-db` or `--timeline-dsn`), the server stores timeline entities and supports:

```
GET /timeline?conv_id=<uuid>&since_version=<n>&limit=<n>
```

Timeline entities are ordered by `version`, which is derived from `event.seq` in SEM frames (Redis stream ID when present, otherwise a time-based monotonic seq). This keeps user and assistant messages ordered consistently across hydration and streaming.

## Eviction

The backend evicts idle conversations when no sockets are connected and no runs are queued or active. Tune with:

- `--evict-idle-seconds` (0 disables eviction)
- `--evict-interval-seconds` (0 disables eviction)

The example UI calls `/timeline` on load to hydrate the timeline before handling WS frames.

## Base Prefix (mounting under a root)

If you run the server under a root prefix (`--root /chat`), the UI must use that base prefix for `/chat`, `/ws`, `/timeline`, and `/api`.

The example UI derives this from `window.location.pathname`.

## Common Customizations

1) **Add a middleware**  
Register the middleware factory with `RegisterMiddleware`, then include it in resolver defaults or request overrides.

2) **Enable tools**  
Register tools with `RegisterTool`, then include them in resolver defaults or in overrides.

3) **Custom system prompt**  
Set resolver default prompt and optionally allow overrides.

4) **Durable timeline**  
Pass `--timeline-db` or `--timeline-dsn` on startup.

## Troubleshooting Checklist

- **Prompt not delivered**: ensure the JSON key is `prompt` (backend also accepts `text` as an alias).
- **WS errors**: check `/ws` proxy in Vite and confirm backend port.
- **No timeline data**: confirm timeline store is configured.
- **Unknown runtime/profile**: verify app resolver logic and app-owned profile registry.
