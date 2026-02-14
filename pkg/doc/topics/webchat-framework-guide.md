---
Title: Building Web Chat Apps with the Webchat Framework
Slug: webchat-framework-guide
Short: End-to-end guide to compose engines, tools, and routes to ship a web chat.
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

# Webchat Framework: Build Composable Web Chat Applications

## 1. Overview

The Webchat framework provides a composable way to build chat applications on top of Geppetto. It cleanly separates: (1) engine creation from step settings, (2) middleware and tool registration, (3) routing and streaming to the browser via Semantic Events, and (4) request policy via app-owned resolvers. Runtime/profile policy is implemented in your app, not in `pkg/webchat`.

With Webchat you can:

- Register tools and middlewares globally, then configure them per runtime plan.
- Compose engines from `StepSettings` using `NewEngineFromStepSettings`.
- Serve a minimal frontend that listens to SEM events over WebSocket and renders Markdown with syntax highlighting.
- Support multiple chat runtimes: `POST /chat`, `POST /chat/{runtime}`, `GET /ws?runtime=...`, with optional app-owned profile/cookie endpoints.

## 2. Core Concepts

- **Router**: Central object wiring HTTP endpoints, WebSocket, and runtime execution dependencies.
- **ConversationRequestResolver**: App-provided request policy for runtime selection and override handling.
- **Middleware Registry**: Map of name → factory(cfg) that returns a Geppetto middleware.
- **Tool Registry**: Builder for tools (e.g., calculator, SQL) that the tool-calling loop can invoke.
- **Run Loop**: The server-side orchestration (default provided: tool-calling loop).

## 3. Project Structure

Key files you’ll work with:

- `pinocchio/pkg/webchat/` – framework package
  - `router.go`, `engine.go`, `conversation.go`, `forwarder.go`, `loops.go`, `types.go`, `server.go`
- `pinocchio/cmd/web-chat/main.go` – example server wiring the framework
- `pinocchio/cmd/web-chat/web/` – frontend (Preact/Zustand) rendering SEM timeline with Markdown + highlighting

## 4. Quick Start

### 4.1. Create a Router and HTTP Server

```go
// main.go (excerpt)
ctx := context.Background()
r, err := webchat.NewRouter(ctx, parsedLayers, staticFS)
if err != nil { return err }

httpSrv, err := r.BuildHTTPServer()
if err != nil { return err }
srv := webchat.NewFromRouter(ctx, r, httpSrv)
return srv.Run(ctx)
```

This:
- Creates a Router using parsed parameter layers (Geppetto + Redis).
- Builds an `http.Server` with sane timeouts.
- Runs the event router and HTTP server.

To mount the entire app under a custom root (e.g., `/chat`), start with:

```bash
web-chat --addr :8080 --root /chat
```

Then the index is served at `/chat`, assets under `/chat/assets` and `/chat/static`, and all API/WS endpoints are prefixed with `/chat`.

### 4.5. How to Mount Under a Custom Root (Implementation Pattern)

If you integrate `webchat.Router` into your own server and need a custom root, mount the router under a prefix using `Router.Mount` (it wraps `http.StripPrefix` for you):

```go
parent := http.NewServeMux()
prefix := "/chat"
r.Mount(parent, prefix)
httpSrv.Handler = parent
```

This preserves all internal paths (`/`, `/assets`, `/static`, `/ws`, `/chat`, `/chat/{runtime}` and any app-owned endpoints like `/api/chat/profiles`) under the chosen root.

### 4.6. Split API and UI Handlers (Optional)

If you want to serve the web UI separately from the API/websocket endpoints, use the dedicated handlers:

```go
parent := http.NewServeMux()
parent.Handle("/api/webchat/", http.StripPrefix("/api/webchat", r.APIHandler()))
parent.Handle("/chat/", http.StripPrefix("/chat", r.UIHandler()))
httpSrv.Handler = parent
```

You can omit `UIHandler()` entirely if you serve the frontend elsewhere.

### 4.7. Embedding the Router Without webchat.Server (start the event loop!)

If you integrate `webchat.Router` into an existing server and do not use `webchat.Server`, you must start the event router loop yourself or runs will never progress beyond initialization. Pattern:

```go
ctx := context.Background()
r, _ := webchat.NewRouter(ctx, parsedLayers, staticFS)

// IMPORTANT: start the event router loop
go func() { _ = r.RunEventRouter(ctx) }()

// Mount under a prefix (see 4.5)
parent := http.NewServeMux()
prefix := "/chat/" // ensure trailing slash
parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), r.Handler()))
httpSrv.Handler = parent
```

This mirrors what `webchat.Server.Run(ctx)` does for you (it runs the event router and the HTTP server). When you self-manage, be sure to pass a cancellable top-level `ctx` so you can stop both cleanly.

### 4.2. Register Middlewares

```go
// Agent mode middleware
r.RegisterMiddleware("agentmode", func(cfg any) middleware.Middleware {
  return agentmode.NewMiddleware(amSvc, cfg.(agentmode.Config))
})

// SQLite tool middleware
r.RegisterMiddleware("sqlite", func(cfg any) middleware.Middleware {
  c := sqlitetool.Config{DB: dbWithRegexp}
  if cfg_, ok := cfg.(sqlitetool.Config); ok { c = cfg_ }
  return sqlitetool.NewMiddleware(c)
})
```

Middlewares are composed per runtime plan (and can be overridden via request body).

### 4.3. Register Tools

```go
r.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
  if im, ok := reg.(*geptools.InMemoryToolRegistry); ok {
    return toolspkg.RegisterCalculatorTool(im)
  }
  tmp := geptools.NewInMemoryToolRegistry()
  if err := toolspkg.RegisterCalculatorTool(tmp); err != nil { return err }
  for _, td := range tmp.ListTools() { _ = reg.RegisterTool(td.Name, td) }
  return nil
})
```

Tools become available to the tool-calling loop for the active conversation.

### 4.4. Define Runtime Policy in App Layer

```go
profiles := newChatProfileRegistry(
  "default",
  &chatProfile{Slug: "default", DefaultPrompt: "You are a helpful assistant. Be concise."},
  &chatProfile{
    Slug: "agent",
    DefaultPrompt: "You are a helpful assistant. Be concise.",
    DefaultMws: []webchat.MiddlewareUse{{Name: "agentmode", Config: amCfg}},
    AllowOverrides: true,
  },
)

r, _ := webchat.NewRouter(
  ctx,
  parsedLayers,
  staticFS,
  webchat.WithConversationRequestResolver(newWebChatProfileResolver(profiles)),
)
registerProfileHandlers(r, profiles)
```

`pkg/webchat` core is profile-agnostic. Profile selection/cookies/endpoints are app-owned policy built on `ConversationRequestResolver`.

## 5. HTTP API

### 5.0. Serving Under a Custom Root

If you pass `--root /xyz`, all routes are mounted under that prefix:

- `GET /` → `GET /xyz/`
- `GET /assets/*` → `GET /xyz/assets/*`
- `GET /static/*` → `GET /xyz/static/*`
- `GET /ws` → `GET /xyz/ws`
- `POST /chat` → `POST /xyz/chat`
- `POST /chat/{runtime}` → `POST /xyz/chat/{runtime}`
- app-owned profile endpoints (optional), e.g. `GET /xyz/api/chat/profiles`

Frontend considerations:
- Use relative asset URLs in `index.html` so Vite emits root-agnostic paths.
- In JavaScript, derive the prefix from `location.pathname` for `POST ${prefix}/chat` and `WS ${prefix}/ws`.

### 5.1. Static and Index

- `GET /static/*` – static assets
- `GET /assets/*` – built dist assets
- `GET /` – returns index.html (tries dist first, falls back to dev index)

### 5.2. App-Owned Profile Endpoints (Optional)

When your app defines profile policy (like `cmd/web-chat`), it may expose:
- `GET /api/chat/profiles` → list available profiles.
- `GET /api/chat/profile` → get current selected profile (often cookie-backed).
- `POST /api/chat/profile` → set selected profile.

### 5.3. WebSocket

- `GET /ws?conv_id=<id>&runtime=<key>` – join streaming for a conversation
  - Runtime resolution order is app-defined by your `ConversationRequestResolver`.
  - SEM envelopes include `seq` and `stream_id`; when Redis stream metadata is present (`xid`/`redis_xid`), `seq` is derived from it for stable ordering. If missing, the backend uses a time-based monotonic `seq` so timeline versions stay ordered.

### 5.4. Start a Chat Run

- `POST /chat` – resolve runtime from app policy.
- `POST /chat/{runtime}` – force a runtime key via path.

Request body:

```json
{
  "prompt": "Classify these transactions...",
  "conv_id": "optional",
  "overrides": {
    "system_prompt": "You are a category designer...",
    "middlewares": [
      { "name": "agentmode", "config": { "default_mode": "financial_analyst" } },
      { "name": "sqlite", "config": { "dsn": "file.db", "max_rows": 500 } }
    ]
  }
}
```

Response:

```json
{ "session_id": "<uuid>", "conv_id": "<uuid>" }
```

### 5.5. Timeline Snapshots

- `GET /timeline?conv_id=<id>&since_version=<n>&limit=<n>` – returns durable timeline entities for hydration.
- Versions are derived from `event.seq` (Redis stream ID when present, time-based monotonic fallback otherwise).
- Backed by SQLite when configured (`--timeline-db`/`--timeline-dsn`), otherwise in-memory.

## 6. Engine Composition

Engines are built from `StepSettings` using Geppetto’s factory:

```go
import (
    "context"
    "github.com/go-go-golems/geppetto/pkg/inference/toolloop"
)

eng, err := factory.NewEngineFromStepSettings(stepSettings)
runner, err := toolloop.NewEngineBuilder(
    toolloop.WithBase(eng),
    toolloop.WithMiddlewares(middleware.NewSystemPromptMiddleware(sysPrompt)),
    // + per-runtime middlewares in order
).Build(context.Background(), "")
eng = runner
```

This ensures parity with standard StepSettings (model, timeouts, provider settings) while keeping middleware composition declarative.

## 7. Run Loop

The default run loop uses the tool loop (`toolloop.Loop`) via the engine builder:

```go
b := toolloop.NewEngineBuilder(
  toolloop.WithBase(eng),
  toolloop.WithToolRegistry(registry),
  toolloop.WithToolConfig(toolloop.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second)),
)
runner, _ := b.Build(ctx, "session-id")
updatedTurn, _ := runner.RunInference(ctx, turn)
```

You can plug custom loops by wiring your own handler within the router, or extend the framework to allow per-runtime loop selection.

## 8. Frontend Rendering (Timeline)

The included UI is a lightweight Preact app that renders a timeline of messages and tool activity. It consumes SEM (semantic event) frames from the server. Assistant messages support:

- Markdown via `marked`
- Sanitization via `DOMPurify`
- Syntax highlighting via `highlight.js`

To customize UI behavior, edit `pinocchio/cmd/web-chat/web/src/` and rebuild your static assets.

Mount-aware assets and API references:

- The Vite config sets `base: './'` so built asset paths are relative and work under any root.
- `index.html` references CSS/JS relatively (e.g., `./static/css/timeline.css`, `./src/main.js` for dev; the built `index.html` will point to `./assets/...`).
- The store computes a prefix from the current URL to call `POST ${prefix}/chat` and connect `WS ${prefix}/ws`.
- If you want Vite to manage CSS (instead of serving from `static/css`), import it in `web/src/main.js` (e.g., `import '../static/css/timeline.css'`) so Vite emits and links it automatically.

### 8.1. Frontend Build from Go (`go generate`)

You can automate the frontend build via a `go:generate` directive in a Go file colocated with your static assets. Example:

```go
//go:generate sh -c "cd cmd/web-chat/web && npm ci && npm run build"
```

With this in place, running `go generate ./...` triggers the Node build, which (via Vite) writes to `cmd/web-chat/static/dist`. The server serves `static/dist/index.html` (built) or falls back to `static/index.html` (dev). Ensure your Vite config sets `outDir` accordingly and `base: './'`.

## 9. Examples

### 9.1. Minimal Server Setup

```go
// Register middlewares/tools, define app runtime policy, run server
r.RegisterMiddleware("agentmode", func(cfg any) middleware.Middleware { return agentmode.NewMiddleware(amSvc, cfg.(agentmode.Config)) })
r.RegisterMiddleware("sqlite", func(cfg any) middleware.Middleware { return sqlitetool.NewMiddleware(sqlitetool.Config{DB: db}) })
r.RegisterTool("calculator", registerCalculator)
profiles := newChatProfileRegistry("default", &chatProfile{Slug: "default", DefaultPrompt: "Be concise."})
r, _ := webchat.NewRouter(ctx, parsedLayers, staticFS, webchat.WithConversationRequestResolver(newWebChatProfileResolver(profiles)))
registerProfileHandlers(r, profiles)
httpSrv, _ := r.BuildHTTPServer()
_ = webchat.NewFromRouter(ctx, r, httpSrv).Run(ctx)
```

### 9.2. Dynamic Overrides from the UI

```json
{
  "prompt": "Write SQL to list top categories",
  "overrides": {
    "middlewares": [{"name":"sqlite","config":{"dsn":"file:analytics.db","max_rows":200}}]
  }
}
```

## 10. Best Practices

- Keep runtime policies minimal; prefer request-time overrides for experimentation.
- Add middlewares in an intentional order (e.g., system prompt first, tool result reordering last).
- Limit run loop iterations and timeouts to avoid runaway tool calling.
- When adding DB-backed tools, ensure safe limits (e.g., `MaxRows`, output truncation) and timeouts.

## 11. Troubleshooting

- 301/405 on `/chat`: Use `POST /chat` (no trailing slash) or `POST /chat/{runtime}`; ensure your frontend isn’t redirecting.
- No WS updates: Confirm `conv_id` is sent and the WebSocket URL points to `/ws?conv_id=...`. If using Redis Streams, verify settings and connectivity.
- No syntax highlighting: Ensure the highlight.js theme loads (see page head for a stylesheet) and code blocks have `language-xyz` classes.

- Got `session_id`/`conv_id` back from `/chat` but no streaming: Ensure the event router loop is running. If you didn’t use `webchat.Server`, call `go r.RunEventRouter(ctx)` after `NewRouter(...)`. Check logs for “starting inference loop” and “inference loop finished”.
- Unknown runtime/profile: Verify your app resolver and app-owned profile registry/handlers.
- Mounting under a prefix: Ensure you’re using `http.StripPrefix(strings.TrimRight(prefix, "/"), r.Handler())` and your frontend assets use relative paths (Vite `base: './'`).

## 12. Related Documentation

For deeper understanding of specific components:

- [Backend Reference](webchat-backend-reference.md) — API reference for StreamCoordinator and ConnectionPool
- [Backend Internals](webchat-backend-internals.md) — Implementation details, concurrency, and performance
- [Debugging and Ops](webchat-debugging-and-ops.md) — Operational procedures and troubleshooting
- [Frontend Integration](webchat-frontend-integration.md) — WebSocket and HTTP integration patterns
- [SEM and UI](webchat-sem-and-ui.md) — SEM event format, routing, and timeline entities

For geppetto core concepts (session lifecycle, events, tool loop):

- See `geppetto/pkg/doc/topics/` for event sinks and session management
- See `geppetto/pkg/doc/playbooks/04-migrate-to-session-api.md` for session migration

## 13. Next Steps

- Add a new runtime preset (e.g., `rag`) with a retrieval middleware.
- Extend the frontend with app-owned profile selectors using `/api/chat/profiles`.
