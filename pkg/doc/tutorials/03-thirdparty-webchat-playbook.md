---
Title: Third-Party Webchat Playbook
Slug: thirdparty-webchat-playbook
Short: A deep, end-to-end guide to embedding Pinocchio webchat in your own app and adding custom middlewares and widgets.
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

# Third-Party Webchat Playbook

This playbook is a deep, end-to-end tutorial for teams that want to build their own webchat on top of the Pinocchio webchat framework. It assumes you are comfortable with Go and TypeScript but are new to the webchat architecture. The goal is to help a new developer build a production-quality webchat by following a clear, structured path and copying patterns from the reference implementation.

The playbook is intentionally verbose. It mixes prose, checklists, pseudocode, diagrams, API references, and concrete file paths. Treat it like a guided walkthrough and a field manual.

## What You Will Build

A third-party webchat application that:

- Runs a Go backend using `pinocchio/pkg/webchat`.
- Streams semantic events over WebSocket.
- Hydrates state from `/timeline` on reload.
- Supports per-profile middlewares and per-request overrides.
- Adds a custom middleware that emits structured events.
- Renders a custom timeline widget in the frontend UI.

You can follow the exact steps or adapt them to your own architecture. All file paths referenced are taken from this repository, so you can open them for real examples.

## Table of Contents

1. Mental model of the system
2. Choosing an integration style
3. Repository and module wiring
4. Backend: minimal server setup
5. Backend: profiles, middlewares, and tools
6. Backend: request policy and profile selection
7. Backend: durable timeline and turn snapshots
8. Backend: custom middleware that emits events
9. Backend: structured sink pipeline (tagged YAML)
10. Backend: timeline projection for custom events
11. Backend: embedding, prefixes, and split UI/API
12. Frontend: consuming SEM events and timeline entities
13. Frontend: custom widgets and renderers
14. Frontend: build overrides and middleware toggles
15. Frontend: packaging and build system
16. Hands-on build (custom widget end-to-end)
17. Full example walkthrough (web-agent-example)
18. Debugging and diagnostics
19. Production hardening checklist
20. Quick reference and file index

## 1. Mental Model

Pinocchio webchat is a streaming event system. The backend runs a Geppetto engine, and every inference emits events that are translated into SEM frames. Those frames stream over WebSocket and also feed a timeline projector that writes durable snapshots. The frontend replays timeline entities on load and listens to the stream for live updates.

Think of the system as two planes operating in parallel:

- **Streaming plane**: WebSocket delivers SEM frames to the UI in real time.
- **Hydration plane**: `/timeline` delivers durable entities on reload.

Both planes feed the same UI state: a timeline of entities that are rendered by widget cards.

### System Diagram

```
User input
   ↓
POST /chat
   ↓
Conversation + Engine
   ↓
Geppetto events
   ↓
SEM translation
   ↓                       ↘
WebSocket stream            Timeline projector
   ↓                         ↓
UI handlers                 SQLite timeline store
   ↓                         ↓
Timeline state            GET /timeline (hydrate)
   ↓
Custom widgets render
```

### Fundamental Idea

When you add a custom middleware, you are doing three things:

1. **Emit events** from the middleware (custom `events.Event` types).
2. **Translate events** into SEM frames and timeline entities.
3. **Render entities** in the UI with a custom widget.

Everything else is plumbing and ergonomics.

## 2. Choosing an Integration Style

There are two common ways to integrate the webchat framework.

### Option A: Standalone server using `webchat.Server`

You create a Go binary that uses `webchat.NewRouter` and `webchat.NewFromRouter`. This is how `pinocchio/cmd/web-chat` and `web-agent-example` work.

**Pros**:
- Fastest to bootstrap.
- You get static UI hosting for free.
- Built-in CLI flags for timeline and Redis.

**Cons**:
- Less control over HTTP wiring.

### Option B: Embed `webchat.Router` into your existing server

You call `webchat.NewRouter` and mount `router.Handler()` under your own mux. You are responsible for running the event router.

**Pros**:
- Full control of routing and auth.
- Can mount under any prefix or behind existing APIs.

**Cons**:
- You must call `router.RunEventRouter(ctx)` yourself.

### Fundamental Callout: Event Router Must Run

If you embed a router directly, you must start the event router loop. If you do not, `/chat` requests will return but no streaming events will ever arrive.

Use this pattern:

```go
r, _ := webchat.NewRouter(ctx, parsed, staticFS)

// Important: start the event router loop
go func() { _ = r.RunEventRouter(ctx) }()
```

This is documented in `pinocchio/pkg/doc/topics/webchat-framework-guide.md`.

## 3. Repository and Module Wiring

If you are building a third-party webchat in a fresh repository, decide how you will pull in Pinocchio and Geppetto. The simplest approach during development is to use a `go.work` workspace so you can iterate on both codebases without publishing modules. In production, you can depend on the released modules with `go mod`.

### Option A: `go.work` for local development

Use a Go workspace to include your application and the Pinocchio repository:

```bash
go work init
go work use ./your-webchat-app
go work use /path/to/pinocchio
go work use /path/to/geppetto
```

This gives you live, in-repo references without `replace` directives. It is the easiest path when you want to mirror `web-agent-example` or contribute upstream changes.

### Option B: `go mod` with `replace`

If you want to keep a single repo but still reference local sources, use `replace` directives in `go.mod`:

```go
replace github.com/go-go-golems/pinocchio => ../pinocchio
replace github.com/go-go-golems/geppetto => ../geppetto
```

### Option C: Pure module dependencies

If you are consuming published modules only, you can import packages directly with standard `go get` and rely on tagged releases. This is the cleanest long-term path, but it is slower for iterative development when you are building custom webchat features.

### Fundamental Callout: Keep backend and frontend in the same repo

Pinocchio’s UI package is designed to be imported as source via a Vite alias. That is much easier when the frontend can resolve the UI directly from the backend repository. Even if you plan to split these later, keep them together while you are building your first third-party webchat.

## 4. Backend: Minimal Server Setup

Start with a minimal Go server that uses `webchat.NewRouter` and runs it. The fastest way is to copy the shape of `web-agent-example/cmd/web-agent-example/main.go`.

### Skeleton Server (pseudocode)

```go
ctx := context.Background()
parsed := loadGlazedLayers()
staticFS := embedStatic()

router, _ := webchat.NewRouter(ctx, parsed, staticFS)
profiles := newChatProfileRegistry(
    "default",
    &chatProfile{Slug: "default", DefaultPrompt: "You are a helpful assistant."},
)
router, _ = webchat.NewRouter(
    ctx,
    parsed,
    staticFS,
    webchat.WithConversationRequestResolver(newWebChatProfileResolver(profiles)),
)
registerProfileHandlers(router, profiles)

srv, _ := router.BuildHTTPServer()
return webchat.NewFromRouter(ctx, router, srv).Run(ctx)
```

### Key APIs and Files

- `pinocchio/pkg/webchat/router.go` — HTTP endpoints and WebSocket wiring.
- `pinocchio/pkg/webchat/types.go` — Core router/runtime types.
- `pinocchio/pkg/webchat/server.go` — `webchat.Server` orchestration.
- `pinocchio/cmd/web-chat/main.go` — Fully wired example server.

### Endpoints you get for free

- `POST /chat` — start an inference run.
- `GET /ws` — WebSocket streaming.
- `GET /timeline` — hydration snapshots.
- `GET /turns` — turn snapshots (if enabled).
- app-owned profile endpoints are optional (`/api/chat/profiles`, `/api/chat/profile`).

All of these are documented in `pinocchio/cmd/web-chat/README.md` and `pinocchio/pkg/doc/topics/webchat-framework-guide.md`.

## 5. Backend: Runtime Policy, Middlewares, Tools

Runtime policy is app-owned via `ConversationRequestResolver`. If you want profile UX, define it in your app layer and resolve to `ConversationRequestPlan` (`RuntimeKey`, `Overrides`, etc.).

### Registering middleware factories

A middleware factory is a function that takes `cfg any` and returns a Geppetto middleware. This lets the UI pass a JSON config for overrides.

```go
r.RegisterMiddleware("webagent-thinking-mode", func(cfg any) geppettomw.Middleware {
    return thinkingmode.NewMiddleware(thinkingmode.ConfigFromAny(cfg))
})
```

### Tools

Use `RegisterTool` to register tools with Geppetto’s tool registry.

```go
r.RegisterTool("calculator", func(reg geptools.ToolRegistry) error {
    return toolspkg.RegisterCalculatorTool(reg)
})
```

### Fundamental Callout: Middleware Ordering

Middleware order matters. The tool loop wraps the base engine, then appends a system prompt middleware, then your custom middlewares in order. If you add multiple system prompt blocks, the first one can be rewritten by the `systemprompt` middleware. If you need your own system block to survive, insert it after the first system block, as shown in `web-agent-example/pkg/discodialogue/middleware.go`.

## 6. Backend: Request Policy and Runtime Selection

The webchat router has a request policy layer that decides which runtime key to use and how overrides are applied. This logic is exposed via `ConversationRequestResolver`.

### Default behavior

The default resolver in `pkg/webchat` is runtime-key based:
- `/chat/{runtime}` path segment
- `runtime` query parameter
- existing conversation runtime key
- fallback `default`

### Custom request policy

If you want app-specific profile/cookie behavior, provide your own resolver with `WithConversationRequestResolver`:

```go
type MyResolver struct{}

func (r *MyResolver) Resolve(req *http.Request) (webchat.ConversationRequestPlan, error) {
  // Implement your own rules:
  // - map request/cookie/auth to RuntimeKey
  // - enforce override policy
  // - set ConvID/Prompt/IdempotencyKey as needed
}

router, _ := webchat.NewRouter(ctx, parsed, staticFS, webchat.WithConversationRequestResolver(&MyResolver{}))
```

### Fundamental Callout: Profile/cookie behavior is app-owned

`pkg/webchat` does not prescribe cookies or profile endpoints. If your UX needs profile switching, implement it in the app layer (as `cmd/web-chat` does).

## 7. Backend: Durable Timeline and Turn Snapshots

Pinocchio supports two persistence paths for webchat:

- **Timeline snapshots** for hydration (`/timeline`).
- **Turn snapshots** for debugging LLM inputs (`/turns`).

### Enable timeline persistence

Use flags `--timeline-dsn` or `--timeline-db`. This stores per-entity snapshots in SQLite and is the canonical hydration path.

### Enable turn persistence

Use flags `--turns-dsn` or `--turns-db`. This stores the exact turn blocks sent to the LLM, including middleware-injected prompts. It is invaluable for debugging missing prompts.

These are referenced in `pinocchio/cmd/web-chat/README.md` and implemented in `pinocchio/pkg/webchat/turn_store_sqlite.go`.

### Example run

```bash
go run ./cmd/web-chat --addr :8080 \
  --timeline-db /tmp/webchat-timeline.db \
  --turns-db /tmp/webchat-turns.db
```

### Inspecting turns

```bash
curl "http://localhost:8080/turns?conv_id=<uuid>&limit=5"
```

The result is a JSON list of YAML payloads representing the exact LLM blocks. This is the authoritative “what did we send to the model” record.

## 8. Backend: Custom Middleware That Emits Events

The simplest custom middleware emits its own events before and after inference. `web-agent-example/pkg/thinkingmode` is the canonical reference.

### Pattern

1. Build a config type and parser.
2. Emit a “started” event before `next`.
3. Emit “completed” event after `next`.
4. Register the event types for JSON decoding.
5. Register SEM mappings so they are streamed.

### Minimal example (pseudocode)

```go
func NewMiddleware(cfg Config) middleware.Middleware {
  return func(next middleware.HandlerFunc) middleware.HandlerFunc {
    return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
      meta := metadataFromTurn(t)
      payload := &Payload{Mode: cfg.Mode}
      gepevents.PublishEventToContext(ctx, NewStarted(meta, t.ID, payload))

      out, err := next(ctx, t)
      if err != nil {
        gepevents.PublishEventToContext(ctx, NewCompleted(meta, t.ID, payload, false, err.Error()))
        return out, err
      }

      gepevents.PublishEventToContext(ctx, NewCompleted(meta, t.ID, payload, true, ""))
      return out, nil
    }
  }
}
```

### Where to look in the repo

- `web-agent-example/pkg/thinkingmode/middleware.go`
- `web-agent-example/pkg/thinkingmode/events.go`
- `web-agent-example/pkg/thinkingmode/sem.go`
- `web-agent-example/pkg/thinkingmode/timeline.go`

## 9. Backend: Structured Sink Pipeline (Tagged YAML)

If your middleware expects the model to emit structured content, you should parse that content from the LLM output. This is done using the structured sink pipeline.

### Why this matters

You cannot rely on a model to always output perfect JSON. The structured sink pipeline parses partial YAML inside tagged blocks, publishes intermediate updates, and emits completed events for the UI.

### High-level pipeline

```
Middleware adds a system prompt with tagged YAML schema
   ↓
Model emits <pkg:type:v1> blocks in its response
   ↓
FilteringSink parses tagged blocks and emits custom events
   ↓
SEM registry converts custom events to SEM frames
   ↓
Timeline projector builds custom entities
```

### Example: Disco dialogue

The disco dialogue middleware (`web-agent-example/pkg/discodialogue`) is a complete example of this pipeline.

Key files:

- `web-agent-example/pkg/discodialogue/middleware.go` — injects a YAML schema into system prompt.
- `web-agent-example/pkg/discodialogue/extractor.go` — streaming YAML parser (structured sink).
- `web-agent-example/pkg/discodialogue/events.go` — custom event types.
- `web-agent-example/pkg/discodialogue/sem.go` — SEM mapping to protobuf.
- `web-agent-example/pkg/discodialogue/timeline.go` — timeline upsert mapping.
- `web-agent-example/cmd/web-agent-example/sink_wrapper.go` — wraps event sink with `FilteringSink`.

### FilteringSink usage (real pattern)

```go
func discoSinkWrapper() webchat.EventSinkWrapper {
  return func(convID string, cfg webchat.EngineConfig, sink events.EventSink) (events.EventSink, error) {
    if !hasMiddleware(cfg.Middlewares, discoMiddlewareName) {
      return sink, nil
    }

    extractors := []structuredsink.Extractor{
      discodialogue.NewDialogueLineExtractor(),
      discodialogue.NewDialogueCheckExtractor(),
      discodialogue.NewDialogueStateExtractor(),
    }

    return structuredsink.NewFilteringSink(
      sink,
      structuredsink.Options{Debug: false, Malformed: structuredsink.MalformedErrorEvents},
      extractors...,
    ), nil
  }
}
```

### Fundamental Callout: Wrap the Event Sink

If you want to parse the model output, you must wrap the `events.EventSink` in your router using `WithEventSinkWrapper`. This is how you hook structured parsing into the pipeline. See `web-agent-example/cmd/web-agent-example/main.go`.

## 10. Backend: Timeline Projection for Custom Events

Custom SEM events need to be projected into timeline entities so they appear during hydration. This is done via `webchat.RegisterTimelineHandler`.

### Pattern

1. Register a timeline handler for your custom SEM event type.
2. Decode the protobuf payload.
3. Build a timeline entity and upsert it into the store.

### Example: Thinking mode timeline mapping

```go
const TimelineKind = "webagent_thinking_mode"

func init() {
  webchat.RegisterTimelineHandler(string(EventThinkingStarted), func(ctx context.Context, p *webchat.TimelineProjector, ev webchat.TimelineSemEvent, _ int64) error {
    return upsertThinkingSnapshot(ctx, p, ev)
  })
}
```

The `upsertThinkingSnapshot` helper builds a `timelinepb.TimelineEntityV1` and calls `p.Upsert()`.

### Where to look

- `web-agent-example/pkg/thinkingmode/timeline.go`
- `web-agent-example/pkg/discodialogue/timeline.go`
- `pinocchio/pkg/webchat/timeline_projector.go`
- `pinocchio/pkg/webchat/timeline_registry.go`

### Fundamental Callout: Timeline upsert vs streaming

Streaming SEM handlers update the UI in real time. Timeline upserts persist those updates for hydration. If you do not register a timeline handler, your custom widget will appear only during streaming and disappear on reload.

## 11. Backend: Embedding, Prefixes, and Split UI/API

Third-party apps often need to mount webchat under a custom path or split UI and API across domains. The router is designed for this.

### Mount under a prefix

If you want to mount everything under `/chat`, use `Router.Mount` or `http.StripPrefix`:

```go
parent := http.NewServeMux()
prefix := "/chat"
r.Mount(parent, prefix)
httpSrv.Handler = parent
```

This is the safest approach because it preserves all internal paths under the prefix. It is documented in `pinocchio/pkg/doc/topics/webchat-framework-guide.md`.

### Serve API and UI separately

If you want to host the UI separately (for example, behind a CDN) but keep the Go backend private:

- Use `r.APIHandler()` for `/chat`, `/ws`, `/timeline`, `/api`.
- Serve your own React frontend that targets those endpoints.

Example pattern:

```go
parent := http.NewServeMux()
parent.Handle("/api/webchat/", http.StripPrefix("/api/webchat", r.APIHandler()))
httpSrv.Handler = parent
```

### Fundamental Callout: Frontend must know the prefix

The UI computes a base prefix from `window.location`. If you are not serving the UI from the same path, you must set the base prefix manually in your custom UI layer.

## 12. Frontend: Consuming SEM Events and Timeline Entities

The frontend uses a single store (`timelineSlice`) to render all entities. It merges streaming SEM events and hydrated timeline snapshots.

### Frontend flow

1. `ChatWidget` loads and calls `/timeline`.
2. `wsManager` opens `/ws?conv_id=...` and buffers events until hydration completes.
3. SEM registry handlers update the timeline store.
4. The UI renders entities via the `renderers` map.

### Key files

- `pinocchio/cmd/web-chat/web/src/sem/registry.ts` — SEM handlers.
- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts` — entity state.
- `pinocchio/cmd/web-chat/web/src/webchat/cards.tsx` — default renderers.
- `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx` — top-level widget.

### Important handler: `timeline.upsert`

The `timeline.upsert` handler is how custom entities arrive. It maps protobuf timeline entities to frontend entities using `timelineMapper.ts` and then calls `upsertEntity`.

If you add a new timeline entity type in protobuf, you must update the mapper in the frontend to decode it. `web-agent-example` uses this path to render disco dialogue entities.

## 13. Frontend: Custom Widgets and Renderers

A custom widget is just a renderer for a timeline entity kind. You register it with `ChatWidget` by passing a `renderers` map.

### Example: WebAgentThinkingModeCard

File: `web-agent-example/web/src/components/WebAgentThinkingModeCard.tsx`

Key pattern:

- Read `e.kind` and `e.props`.
- Use guard logic for missing fields.
- Render a card with semantic structure and status badges.

### Register in the App

```tsx
const renderers: ChatWidgetRenderers = {
  webagent_thinking_mode: WebAgentThinkingModeCard,
  disco_dialogue_line: DiscoDialogueCard,
  disco_dialogue_check: DiscoDialogueCard,
  disco_dialogue_state: DiscoDialogueCard,
}

<ChatWidget renderers={renderers} />
```

### Fundamental Callout: Entity Kind is the contract

The `kind` string used in timeline projection is the same string you use in `renderers`. If you change one, you must change the other. Treat it like an API contract.

## 14. Frontend: Build Overrides and Middleware Toggles

The UI can pass middleware overrides at request time. This is how you add knobs and switches without new backend profiles.

### Example: buildOverrides

```tsx
const buildOverrides = useCallback(() => {
  const middlewares = [{ name: 'webagent-thinking-mode', config: { mode } }]
  if (discoEnabled) {
    middlewares.push({ name: 'webagent-disco-dialogue', config: {} })
  }
  return { middlewares }
}, [mode, discoEnabled])

<ChatWidget buildOverrides={buildOverrides} />
```

The `buildOverrides` result is sent in the `/chat` request body under `overrides.middlewares`.

### Where to look

- `web-agent-example/web/src/App.tsx`
- `pinocchio/pkg/webchat/engine_config.go`
- `pinocchio/pkg/webchat/engine_from_req.go`

## 15. Frontend: Packaging and Build System

There are two supported frontend strategies:

- Build the UI directly in the Pinocchio repo and embed it in the Go binary.
- Run the UI in dev mode and proxy to the backend.

### Embedding the UI into Go

`pinocchio/cmd/web-chat` and `web-agent-example` both embed `static/dist` using Go’s `embed` package. The recommended pattern is:

- Build the Vite app into `cmd/<your-app>/static/dist`.
- Embed `static` in your Go binary.
- Serve `static/dist/index.html` first, fall back to `static/index.html` for dev.

Example build command from `web-agent-example/web/package.json`:

```bash
npm run build
```

This writes to `cmd/web-agent-example/static/dist` which is embedded by `//go:embed static` in `web-agent-example/cmd/web-agent-example/main.go`.

### Dev proxy mode

During development, you can run the Vite dev server and use the proxy settings in `web-agent-example/web/vite.config.ts` to forward `/chat`, `/ws`, `/timeline`, and `/api` to the Go backend. This lets you iterate on UI without rebuilding Go binaries.

### Fundamental Callout: Keep Vite base relative

The `base: './'` setting in Vite ensures asset paths are relative. This is critical if you mount the app under a prefix like `/chat`. Without it, the built asset URLs will break.

## 16. Hands-On Build: Your First Custom Widget (End-to-End)

This section walks through a full vertical slice using a small, concrete feature. The goal is to give a new developer a repeatable pattern that they can apply to any custom widget.

### Feature spec

We want a middleware that emits a “confidence badge” with a score and a short note. The badge should appear as a custom timeline card and be preserved across reloads.

### Step 1: Define the event payload (Go)

Create a new package in your app, for example `pkg/confidencebadge`, with a payload type and event types. Keep the payload small and JSON-friendly.

```go
type Payload struct {
  Score float64 `json:"score" yaml:"score"`
  Note  string  `json:"note" yaml:"note"`
}

const EventConfidenceStarted = gepevents.EventType("custom.confidence.started")
const EventConfidenceCompleted = gepevents.EventType("custom.confidence.completed")
```

Register factories using `gepevents.RegisterEventFactory` so the event can be decoded from JSON.

### Step 2: Emit events in middleware

Write a middleware that computes a score and emits start and completed events. This mirrors `web-agent-example/pkg/thinkingmode/middleware.go`.

```go
func NewMiddleware(cfg Config) middleware.Middleware {
  return func(next middleware.HandlerFunc) middleware.HandlerFunc {
    return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
      meta := metadataFromTurn(t)
      gepevents.PublishEventToContext(ctx, NewConfidenceStarted(meta, t.ID, &Payload{Score: 0.0}))
      out, err := next(ctx, t)
      if err != nil {
        gepevents.PublishEventToContext(ctx, NewConfidenceCompleted(meta, t.ID, &Payload{Score: 0.0, Note: err.Error()}, false, err.Error()))
        return out, err
      }
      gepevents.PublishEventToContext(ctx, NewConfidenceCompleted(meta, t.ID, &Payload{Score: 0.83, Note: "stable"}, true, ""))
      return out, nil
    }
  }
}
```

### Step 3: Register SEM mapping (Go)

Convert your custom event to a SEM frame. Follow `web-agent-example/pkg/thinkingmode/sem.go`. The key is to map your payload to a protobuf type and wrap it in a SEM envelope.

This ensures the event streams to the frontend immediately.

### Step 4: Project into timeline (Go)

Register a timeline handler so the event becomes a durable timeline entity. The handler should decode the protobuf payload and build a `timelinepb.TimelineEntityV1` with a stable `kind` string.

This is what lets the UI rehydrate from `/timeline`.

### Step 5: Add frontend renderer (TypeScript)

In your UI, create a `ConfidenceCard` that reads `props.score` and `props.note`. Register it in `ChatWidget` renderers with the same `kind` string you used in the timeline entity.

### Step 6: Verify streaming and hydration

Use these tests in order:

1. Send a message and confirm the card appears live in the UI.
2. Refresh the page and confirm the card reappears from `/timeline`.
3. Inspect `/turns` if the prompt or middleware appears to be missing.

This sequence ensures the full pipeline is wired.

## 17. Full Example Walkthrough: web-agent-example

The `web-agent-example` folder is a complete reference app that bundles a backend and frontend using Pinocchio webchat. It includes two custom features:

- Thinking mode (simple middleware events).
- Disco dialogue (structured sink + timeline projection + widget).

### Backend wiring

File: `web-agent-example/cmd/web-agent-example/main.go`

Key steps:

- Use `webchat.NewRouter` with `WithEventSinkWrapper`.
- Register custom middleware factories.
- Add a default profile with middleware defaults.
- Run the HTTP server.

### Frontend wiring

File: `web-agent-example/web/src/App.tsx`

Key steps:

- Alias `@pwchat` to `pinocchio/cmd/web-chat/web/src` in Vite config.
- Render `ChatWidget` from `@pwchat/webchat`.
- Pass custom renderers and a custom composer.
- Use `buildOverrides` to control middleware settings.

### Vite alias setup

File: `web-agent-example/web/vite.config.ts`

```ts
const webchatRoot = path.resolve(__dirname, '../../pinocchio/cmd/web-chat/web/src')

export default defineConfig({
  resolve: { alias: { '@pwchat': webchatRoot } },
  server: {
    proxy: {
      '/chat': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: 'http://localhost:8080', ws: true, changeOrigin: true },
      '/timeline': { target: 'http://localhost:8080', changeOrigin: true },
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
```

### Why the alias matters

The `@pwchat` alias lets you reuse the exact webchat UI package without publishing it to npm. It treats the Pinocchio UI source as a local package, which makes iteration fast and keeps the UI aligned with backend changes.

## 18. Debugging and Diagnostics

Pinocchio provides multiple layers of debugging. Use them early, not late.

### WebSocket debug logs

Enable debug logging in the browser:

- Add `?ws_debug=1` to the URL, or
- Run `window.__WS_DEBUG__ = true` in the console.

Logs appear as `[ws.mgr]` messages and show event flow and hydration ordering. See `pinocchio/pkg/doc/topics/webchat-debugging-and-ops.md` for log patterns.

### Turn snapshots for prompt inspection

If you suspect middleware prompts are missing, enable turn snapshots and inspect `/turns`. This is the definitive answer to “what did the LLM see”.

### Timeline hydration checks

If hydration order is wrong, check:

- `/timeline` payload ordering.
- Event `seq` values in streaming frames.
- The timeline store versions in SQLite.

### Common failure patterns

- **Unknown profile**: the profile slug in the request does not exist. List profiles at `GET /api/chat/profiles`.
- **No streaming**: event router not running or WebSocket not connected.
- **Missing custom widget on reload**: no timeline handler registered, or custom entity type not decoded in the frontend mapper.

## 19. Production Hardening Checklist

This is a checklist you can copy into your own onboarding docs.

- Confirm `webchat.RunEventRouter` is running in non-`webchat.Server` setups.
- Use `--timeline-db` for persistent hydration and set `--turns-db` for prompt debugging.
- Ensure middleware ordering is deliberate and documented.
- Validate custom SEM events are registered in Go and mapped in frontend.
- Confirm custom timeline entities are decoded in `timelineMapper.ts`.
- Add `AllowOverrides` only for profiles that are safe to override.
- Run load tests with multiple WebSocket clients per conversation.
- Use `idle-timeout-seconds` and eviction flags to avoid resource leaks.

## 20. Quick Reference and File Index

These are the files you will likely open most while building a third-party webchat:

- `pinocchio/pkg/webchat/router.go` — HTTP endpoints and core wiring.
- `pinocchio/pkg/webchat/engine_from_req.go` — profile selection and overrides.
- `pinocchio/pkg/webchat/timeline_projector.go` — timeline entity projection.
- `pinocchio/pkg/webchat/router_options.go` — extension points (custom builder, subscriber, hooks).
- `pinocchio/pkg/doc/topics/webchat-framework-guide.md` — full backend guide.
- `pinocchio/pkg/doc/topics/webchat-sem-and-ui.md` — event and UI schema.
- `pinocchio/pkg/doc/topics/webchat-frontend-architecture.md` — UI architecture.
- `web-agent-example/cmd/web-agent-example/main.go` — full example backend.
- `web-agent-example/web/src/App.tsx` — full example frontend.
- `web-agent-example/pkg/discodialogue/` — structured sink pipeline example.
- `web-agent-example/pkg/thinkingmode/` — simple event middleware example.

## Appendix A: Custom Protobuf and Timeline Mapping (Detailed)

If your widget needs structured payloads that are reused across backend and frontend, define them in protobuf. This is the same pattern used by thinking mode, planning, and disco dialogue.

### Step A1: Add a protobuf message

Create a new file in `pinocchio/proto/sem/middleware/` and add a payload message for your custom widget. Keep fields flat and stable.

Example shape:

```proto
message ConfidencePayload {
  double score = 1;
  string note = 2;
}

message ConfidenceStarted {
  string item_id = 1;
  ConfidencePayload data = 2;
}
```

### Step A2: Add a timeline snapshot type

Timeline snapshots live in `pinocchio/proto/sem/timeline/`. Add a snapshot message and wire it into `TimelineEntityV1`.

Example shape:

```proto
message ConfidenceSnapshotV1 {
  uint32 schema_version = 1;
  string item_id = 2;
  string status = 3;
  sem.middleware.custom.ConfidencePayload payload = 4;
}
```

### Step A3: Generate code

Regenerate protobuf bindings for Go and TypeScript using your existing build pipeline. This ensures:

- Go can decode and emit SEM frames.
- The frontend can decode `timeline.upsert` payloads into entities.

### Step A4: Map protobuf to timeline entity

Your timeline handler should decode the protobuf message and build a `TimelineEntityV1` of the correct kind. This is exactly what `web-agent-example/pkg/thinkingmode/timeline.go` does.

### Step A5: Map timeline entity in frontend

Update `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts` so that your new snapshot type is converted to a timeline entity with `kind: "custom_confidence"` and the right `props` structure.

### Fundamental Callout: Protobuf is the contract, not the UI

The protobuf schema is the shared contract between backend and frontend. The UI can change visuals freely, but the protobuf fields should remain stable to avoid hydration drift.

## Appendix B: End-to-End Validation Script (CLI)

These are minimal commands to validate that your backend and frontend are wired correctly.

### Start the backend

```bash
go run ./cmd/web-agent-example serve --addr :8080 --timeline-db /tmp/timeline.db --turns-db /tmp/turns.db
```

### Start the frontend

```bash
cd web-agent-example/web
npm run dev
```

### Send a message via CLI

```bash
curl -s http://localhost:5174/chat \
  -H "Content-Type: application/json" \
  -d '{"conv_id":"<uuid>","prompt":"hello","overrides":{"middlewares":[{"name":"webagent-disco-dialogue","config":{}}]}}'
```

### Inspect the timeline

```bash
curl -s "http://localhost:8080/timeline?conv_id=<uuid>" | jq
```

### Inspect turns

```bash
curl -s "http://localhost:8080/turns?conv_id=<uuid>&limit=3" | jq
```

If these commands show both your custom entity and the system prompt in the turns snapshot, your pipeline is correctly wired.

## Appendix C: Common Extension Points (Router Options)

These are the most important extension points for third-party apps. They live in `pinocchio/pkg/webchat/router_options.go`.

- `WithConversationRequestResolver` to control runtime selection and app-owned profile/cookie behavior.
- `WithBuildSubscriber` to override how subscribers are created.
- `WithTimelineUpsertHook` to intercept timeline upserts.
- `WithEventSinkWrapper` to wrap the event sink for structured parsing.
- `WithTimelineStore` and `WithTurnStore` for custom persistence.

Use these when you need to integrate webchat into an existing architecture, not when you are just getting started.

## Closing Notes

If you are new to the system, the fastest success path is:

1. Run `web-agent-example` locally.
2. Add a tiny custom middleware that emits a simple event.
3. Add a renderer for that event in the frontend.
4. Add a timeline handler for hydration.

This is the smallest “vertical slice” that proves you understand the system end-to-end. Once that works, you can confidently build more complex middlewares and widgets.
