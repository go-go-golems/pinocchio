---
Title: 'Third-party web-chat reuse: copy/paste recipes'
Ticket: PI-02-REUSABLE-PINOCCHIO-WEBCHAT
Status: active
Topics:
    - webchat
    - react
    - frontend
    - pinocchio
    - refactor
    - thirdparty
    - websocket
    - http-api
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Reference for app-config.js and --root mounting
    - Path: pinocchio/cmd/web-chat/static/dist/index.html
      Note: app-config.js inclusion contract
    - Path: pinocchio/cmd/web-chat/web/src/ws/wsManager.ts
      Note: Frontend expects canonical /ws and /api/timeline
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: Copy/paste handler constructors for /chat
    - Path: pinocchio/pkg/webchat/http/profile_api.go
      Note: Copy/paste profile API wiring for /api/chat/*
ExternalSources: []
Summary: Recipes for reusing Pinocchio’s web-chat Go backend and React frontend in another module/app.
LastUpdated: 2026-03-03T08:19:17.051802403-05:00
WhatFor: ""
WhenToUse: ""
---


# Third-party web-chat reuse: copy/paste recipes

## Goal

Provide “copy/paste first, refactor later” recipes that let a third-party Go app reuse Pinocchio webchat:

- Backend: `pinocchio/pkg/webchat` + `pinocchio/pkg/webchat/http`
- Frontend: either embed prebuilt assets (your own build) or (recommended) depend on an extracted reusable React package (proposed in PI-02 design doc).

## Context

Pinocchio webchat has an explicit ownership model:

- The **app** owns transport routes like `/chat` and `/ws`. (`pinocchio/pkg/webchat/doc.go:3-12`)
- `pkg/webchat` provides the core conversation lifecycle, streaming, projection, and *optional* UI/core-API handlers.

The shipped example app is `pinocchio/cmd/web-chat`, which you should treat as an **example**, not a reusable library.

This recipes doc focuses on *third-party* usage patterns.

## Quick Reference

### Canonical route table (minimum for UI)

All routes must be mounted under a single base prefix (possibly empty). The React UI will call them relative to that prefix.

| Route | Method | Purpose | Evidence |
|---|---|---|---|
| `/chat` | POST | Submit prompt | `pinocchio/pkg/webchat/http/api.go:104-164`, `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx:179-183` |
| `/ws?conv_id=...` | GET (upgrade) | Stream SEM frames | `pinocchio/pkg/webchat/http/api.go:166-220`, `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:80-83` |
| `/api/timeline?conv_id=...` | GET | Timeline hydration snapshot | `pinocchio/pkg/webchat/http/api.go:222-273`, `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts:171-189` |
| `/api/chat/profiles` | GET/POST | Profile list/create (optional but used by default UI header) | `pinocchio/cmd/web-chat/web/src/store/profileApi.ts:93-110`, `pinocchio/pkg/webchat/http/profile_api.go:161+` |
| `/api/chat/profile` | GET/POST | Current profile cookie route (optional) | `pinocchio/cmd/web-chat/web/src/store/profileApi.ts:97-110`, `pinocchio/pkg/webchat/http/profile_api.go:502+` |

### SEM envelope shape (frontend expects)

```jsonc
{
  "sem": true,
  "event": {
    "type": "llm.delta",
    "id": "some-id",
    "seq": 1707053365123000000,
    "stream_id": "1707053365123-0",
    "data": { "cumulative": "..." }
  }
}
```

### Runtime config injection (recommended)

The built UI loads `./app-config.js` (`pinocchio/cmd/web-chat/static/dist/index.html:7`) and expects:

```js
window.__PINOCCHIO_WEBCHAT_CONFIG__ = {
  basePrefix: "/chat",       // or ""
  debugApiEnabled: false
};
```

Backend example generator: `pinocchio/cmd/web-chat/main.go:65-74`.

## Usage Examples

### Recipe 1 — Minimal Go backend wiring (no profiles, no fancy policy)

This is the “hello world” shape. It is intentionally a **skeleton**, because you must still supply a real runtime composer (`webchat.WithRuntimeComposer`) that can create a geppetto engine (LLM provider, etc).

```go
package main

import (
  "context"
  "embed"
  "encoding/json"
  "errors"
  "net/http"
  "strings"

  "github.com/go-go-golems/glazed/pkg/cmds/values"
  "github.com/google/uuid"
  "github.com/gorilla/websocket"
  "github.com/rs/zerolog/log"

  infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
  webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
  webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
)

// 1) You embed *your* static UI assets (built Vite dist) into your module.
//go:embed static
var staticFS embed.FS

// 2) Implement a request resolver. This is app-owned policy.
type Resolver struct{}

func (r *Resolver) Resolve(req *http.Request) (webhttp.ResolvedConversationRequest, error) {
  // Minimal policy: conv_id required for ws; conv_id optional for chat body.
  switch req.Method {
  case http.MethodGet:
    convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
    if convID == "" {
      return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
        Status: http.StatusBadRequest, ClientMsg: "missing conv_id",
      }
    }
    return webhttp.ResolvedConversationRequest{
      ConvID: convID,
      RuntimeKey: "default",
      RuntimeFingerprint: "default",
      ProfileVersion: 0,
      ResolvedRuntime: nil, // fill this if you have a profile system; otherwise your composer may ignore it
      ProfileMetadata: nil,
      Overrides: nil,
    }, nil
  case http.MethodPost:
    var body webhttp.ChatRequestBody
    _ = json.NewDecoder(req.Body).Decode(&body)
    convID := strings.TrimSpace(body.ConvID)
    if convID == "" {
      convID = uuid.NewString()
    }
    prompt := body.Prompt
    if prompt == "" && body.Text != "" {
      prompt = body.Text
    }
    if strings.TrimSpace(prompt) == "" {
      return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
        Status: http.StatusBadRequest, ClientMsg: "missing prompt",
      }
    }
    return webhttp.ResolvedConversationRequest{
      ConvID: convID,
      RuntimeKey: "default",
      RuntimeFingerprint: "default",
      Prompt: prompt,
      Overrides: body.RequestOverrides,
      IdempotencyKey: strings.TrimSpace(body.IdempotencyKey),
    }, nil
  default:
    return webhttp.ResolvedConversationRequest{}, &webhttp.RequestResolutionError{
      Status: http.StatusMethodNotAllowed, ClientMsg: "method not allowed",
    }
  }
}

func run(ctx context.Context, parsed *values.Values) error {
  // TODO: implement this for your app. It must return a non-nil geppetto engine.
  // This stub compiles but will fail at runtime when a request needs a conversation.
  runtimeComposer := infruntime.RuntimeBuilderFunc(func(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
    return infruntime.ComposedRuntime{}, errors.New("TODO: implement runtimeComposer")
  })

  // 3) Create server core (requires runtime composer).
  srv, err := webchat.NewServer(
    ctx,
    parsed,
    staticFS,
    webchat.WithRuntimeComposer(runtimeComposer),
  )
  if err != nil {
    return err
  }

  resolver := &Resolver{}
  mux := http.NewServeMux()
  mux.HandleFunc("/chat", webhttp.NewChatHandler(srv.ChatService(), resolver))
  mux.HandleFunc("/chat/", webhttp.NewChatHandler(srv.ChatService(), resolver))
  mux.HandleFunc("/ws", webhttp.NewWSHandler(
    srv.StreamHub(),
    resolver,
    websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
  ))
  timelineLogger := log.With().Str("component", "webchat").Str("route", "/api/timeline").Logger()
  mux.HandleFunc("/api/timeline", webhttp.NewTimelineHandler(srv.TimelineService(), timelineLogger))
  mux.HandleFunc("/api/timeline/", webhttp.NewTimelineHandler(srv.TimelineService(), timelineLogger))

  // Optional: core API and UI
  mux.Handle("/api/", srv.APIHandler())
  mux.Handle("/", srv.UIHandler())

  httpSrv := srv.HTTPServer()
  httpSrv.Handler = mux
  return srv.Run(ctx)
}
```

Notes:

- This example omits profile registry wiring. If you want the default UI’s profile dropdown to work, mount `/api/chat/*` handlers.
- The runtime composer is the “hard part” and is intentionally app-specific (LLM provider config, middleware, tools).

### Recipe 2 — Mount under a root prefix (e.g. `/chat`)

The UI expects the base prefix to be consistent across `/chat`, `/ws`, `/api/*`, etc.

Copy the pattern used by the example app (`pinocchio/cmd/web-chat/main.go:257-274`):

```go
parent := http.NewServeMux()
prefix := "/chat/"
parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), mux))
httpSrv.Handler = parent
```

Also serve `app-config.js` (see next recipe) so the UI can set `basePrefix="/chat"` explicitly.

### Recipe 3 — Serve `app-config.js` so the UI knows its base prefix

The built UI loads `./app-config.js` (`pinocchio/cmd/web-chat/static/dist/index.html:7`).

Until this is moved into a reusable pkg helper, you can use the exact approach from the example app:

- script generator: `pinocchio/cmd/web-chat/main.go:65-74`
- handler: `pinocchio/cmd/web-chat/main.go:235-249`

Minimal handler shape:

```go
func serveAppConfigJS(script string) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet && r.Method != http.MethodHead {
      http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
      return
    }
    w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
    w.Header().Set("Cache-Control", "no-store")
    if r.Method == http.MethodHead {
      return
    }
    _, _ = w.Write([]byte(script))
  }
}
```

### Recipe 4 — Enable profile APIs (so the UI header dropdown works)

If you have a `geppetto/pkg/profiles.Registry`, mount the shared handlers:

```go
webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
  DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
  EnableCurrentProfileCookieRoute: true,
  MiddlewareDefinitions:           middlewareDefinitions,
  ExtensionCodecRegistry:          extensionCodecRegistry,
  WriteActor:                      "my-app",
  WriteSource:                     "http-api",
})
```

Implementation evidence: `pinocchio/pkg/webchat/http/profile_api.go:137-150`.

### Recipe 5 — Timeline JS reducers/handlers (server-side projection customization)

Today:

- The JS timeline runtime lives in `pkg` (`pinocchio/pkg/webchat/timeline_js_runtime.go:26-37`).
- Loading scripts from paths is in `cmd` (`pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27-52`).

If you need this immediately in a third-party app, copy that loader logic verbatim into your app (or wait for Phase 4 refactor described in the PI-02 design doc).

Example flow (from `pinocchio/cmd/web-chat/timeline_js_runtime_loader.go:27-52`):

1. `webchat.ClearTimelineRuntime()`
2. build `require.WithGlobalFolders(...)` from script paths
3. `runtime := webchat.NewJSTimelineRuntimeWithOptions(opts)`
4. `runtime.LoadScriptFile(path)` for each path
5. `webchat.SetTimelineRuntime(runtime)`

## Related

- PI-02 primary analysis: `../design-doc/01-reusable-pinocchio-web-chat-analysis-extraction-guide.md`
- Upstream docs worth reading:
  - `pinocchio/pkg/doc/topics/webchat-framework-guide.md`
  - `pinocchio/pkg/doc/topics/webchat-http-chat-setup.md`
  - `pinocchio/pkg/doc/topics/webchat-frontend-architecture.md`
  - `pinocchio/pkg/doc/topics/webchat-frontend-integration.md`
