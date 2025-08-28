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
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Webchat Framework: Build Composable Web Chat Applications

## 1. Overview

The Webchat framework provides a composable way to build multi-profile chat applications on top of Geppetto. It cleanly separates: (1) engine creation from step settings, (2) middleware and tool registration, (3) routing and streaming to the browser via Semantic Events. This design lets you define multiple “profiles” (e.g., `default`, `agent`) that differ in middlewares, tools, and prompts, selectable per-request or via a cookie, without duplicating server code.

With Webchat you can:

- Register tools and middlewares globally, then configure per-profile.
- Compose engines from `StepSettings` using `NewEngineFromStepSettings`.
- Serve a minimal frontend that listens to SEM events over WebSocket and renders Markdown with syntax highlighting.
- Support multiple chat profiles: `POST /chat`, `POST /chat/{profile}`, `GET /ws?profile=...`, or set the `chat_profile` cookie via `/default` and `/agent` endpoints.

## 2. Core Concepts

- **Router**: Central object wiring HTTP endpoints, WebSocket, profiles, and registries.
- **Profile**: A named configuration (slug) with default system prompt and an ordered list of middlewares.
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

If you integrate `webchat.Router` into your own server and need a custom root, mount the router’s handler under a prefix using `http.ServeMux` and `http.StripPrefix`:

```go
parent := http.NewServeMux()
prefix := "/chat/" // ensure trailing slash
parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), r.Handler()))
httpSrv.Handler = parent
```

This preserves all internal paths (`/`, `/assets`, `/static`, `/ws`, `/chat`, `/chat/{profile}`, `/api/chat/profiles`) under the chosen root.

### 4.6. Embedding the Router Without webchat.Server (start the event loop!)

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

Middlewares are composed per profile (and can be overridden via request body).

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

### 4.4. Define Profiles

```go
r.AddProfile(&webchat.Profile{
  Slug: "default",
  DefaultPrompt: "You are a helpful assistant. Be concise.",
  DefaultMws: []webchat.MiddlewareUse{},
})

r.AddProfile(&webchat.Profile{
  Slug: "agent",
  DefaultPrompt: "You are a helpful assistant. Be concise.",
  DefaultMws: []webchat.MiddlewareUse{{Name: "agentmode", Config: amCfg}},
})
```

Profiles are selected by path (`/chat/agent`) or via the `chat_profile` cookie.

## 5. HTTP API

### 5.0. Serving Under a Custom Root

If you pass `--root /xyz`, all routes are mounted under that prefix:

- `GET /` → `GET /xyz/`
- `GET /assets/*` → `GET /xyz/assets/*`
- `GET /static/*` → `GET /xyz/static/*`
- `GET /ws` → `GET /xyz/ws`
- `POST /chat` → `POST /xyz/chat`
- `POST /chat/{profile}` → `POST /xyz/chat/{profile}`
- `GET /api/chat/profiles` → `GET /xyz/api/chat/profiles`

Frontend considerations:
- Use relative asset URLs in `index.html` so Vite emits root-agnostic paths.
- In JavaScript, derive the prefix from `location.pathname` for `POST ${prefix}/chat` and `WS ${prefix}/ws`.

### 5.1. Static and Index

- `GET /static/*` – static assets
- `GET /assets/*` – built dist assets
- `GET /` – returns index.html (tries dist first, falls back to dev index)

### 5.2. Profiles

- `GET /api/chat/profiles` → list available profiles; the UI can present these.
- `GET /default` → set `chat_profile=default` (204 No Content)
- `GET /agent` → set `chat_profile=agent` (204 No Content)

### 5.3. WebSocket

- `GET /ws?conv_id=<id>&profile=<slug>` – join streaming for a conversation
  - If `profile` is omitted, the server falls back to `chat_profile` cookie, else `default`.

### 5.4. Start a Chat Run

- `POST /chat` – use cookie or default profile
- `POST /chat/{profile}` – force a given profile

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
{ "run_id": "<uuid>", "conv_id": "<uuid>" }
```

## 6. Engine Composition

Engines are built from `StepSettings` using Geppetto’s factory:

```go
eng, err := factory.NewEngineFromStepSettings(stepSettings)
eng = middleware.NewEngineWithMiddleware(eng, middleware.NewSystemPromptMiddleware(sysPrompt))
// + per-profile middlewares in order
```

This ensures parity with standard StepSettings (model, timeouts, provider settings) while keeping middleware composition declarative.

## 7. Run Loop

The default run loop uses `toolhelpers.RunToolCallingLoop`:

```go
updatedTurn, _ := toolhelpers.RunToolCallingLoop(
  ctx, eng, turn, registry,
  toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second),
)
```

You can plug custom loops by wiring your own handler within the router, or extend the framework to allow per-profile loop selection.

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
// Register middlewares/tools, add profiles, run server
r.RegisterMiddleware("agentmode", func(cfg any) middleware.Middleware { return agentmode.NewMiddleware(amSvc, cfg.(agentmode.Config)) })
r.RegisterMiddleware("sqlite", func(cfg any) middleware.Middleware { return sqlitetool.NewMiddleware(sqlitetool.Config{DB: db}) })
r.RegisterTool("calculator", registerCalculator)
r.AddProfile(&webchat.Profile{Slug: "default", DefaultPrompt: "Be concise.", DefaultMws: nil})
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

- Keep profiles minimal; prefer request-time overrides for experimentation.
- Add middlewares in an intentional order (e.g., system prompt first, tool result reordering last).
- Limit run loop iterations and timeouts to avoid runaway tool calling.
- When adding DB-backed tools, ensure safe limits (e.g., `MaxRows`, output truncation) and timeouts.

## 11. Troubleshooting

- 301/405 on `/chat`: Use `POST /chat` (no trailing slash) or `POST /chat/{profile}`; the router handles both now. Ensure your frontend isn’t redirecting.
- No WS updates: Confirm `conv_id` is sent and the WebSocket URL points to `/ws?conv_id=...`. If using Redis Streams, verify settings and connectivity.
- No syntax highlighting: Ensure the highlight.js theme loads (see page head for a stylesheet) and code blocks have `language-xyz` classes.

- Got `run_id`/`conv_id` back from `/chat` but no streaming: Ensure the event router loop is running. If you didn’t use `webchat.Server`, call `go r.RunEventRouter(ctx)` after `NewRouter(...)`. Check logs for “starting run loop” and “run loop finished”.
- Unknown profile: Verify your profile registration and list profiles via `GET /api/chat/profiles`. If no profile cookie is set and none provided, the router uses `default`.
- Mounting under a prefix: Ensure you’re using `http.StripPrefix(strings.TrimRight(prefix, "/"), r.Handler())` and your frontend assets use relative paths (Vite `base: './'`).

## 12. Next Steps

- Add a new profile (e.g., `rag`) with a retrieval middleware.
- Implement a custom run loop for multi-turn planning.
- Extend the frontend with profile selectors using `/api/chat/profiles`.


