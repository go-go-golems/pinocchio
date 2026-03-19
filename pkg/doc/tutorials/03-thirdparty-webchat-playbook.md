---
Title: Third-Party Webchat Playbook
Slug: thirdparty-webchat-playbook
Short: End-to-end tutorial for building a webchat-style app from scratch with Pinocchio's handler-first backend and local-first runtime planning.
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

## What You Will Build

This tutorial shows how to build a webchat-style application from scratch on top of Pinocchio. The result is an app that:

- exposes app-owned `POST /chat` and `GET /ws`
- hydrates the frontend through `GET /api/timeline`
- resolves engine settings from Geppetto engine profiles
- keeps prompt, middleware, and tool policy in app-owned runtime code
- converts into the shared Pinocchio transport only at the final boundary

This is the current architecture. Do not start from older mixed-profile examples that expect Geppetto to carry prompt and middleware policy for the app.

## Architecture First

Before you write code, lock in the separation of concerns:

```text
request
  -> app resolver
  -> resolve engine profile
  -> merge final InferenceSettings
  -> resolve app runtime policy
  -> local resolved conversation plan
  -> convert to shared webchat request
  -> ChatService / StreamHub / TimelineService
```

The rest of the tutorial follows that exact flow.

## Step 1: Build the server skeleton

Start with the shared webchat server plus app-owned handlers.

```go
//go:embed static
var staticFS embed.FS

func run(ctx context.Context, parsed *values.Values) error {
    middlewareDefinitions := newMiddlewareDefinitionRegistry()
    runtimeComposer := newRuntimeComposer(parsed, middlewareDefinitions)

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

    resolver := newRequestResolver()

    mux := http.NewServeMux()
    mux.HandleFunc("/chat", webhttp.NewChatHandler(srv.ChatService(), resolver))
    mux.HandleFunc("/chat/", webhttp.NewChatHandler(srv.ChatService(), resolver))
    mux.HandleFunc("/ws", webhttp.NewWSHandler(
        srv.StreamHub(),
        resolver,
        websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
    ))
    mux.HandleFunc("/api/timeline", webhttp.NewTimelineHandler(
        srv.TimelineService(),
        log.With().Str("component", "my-webchat").Str("route", "/api/timeline").Logger(),
    ))
    mux.HandleFunc("/api/timeline/", webhttp.NewTimelineHandler(
        srv.TimelineService(),
        log.With().Str("component", "my-webchat").Str("route", "/api/timeline").Logger(),
    ))

    mux.Handle("/api/", srv.APIHandler())
    mux.Handle("/", srv.UIHandler())

    srv.HTTPServer().Handler = mux
    return srv.Run(ctx)
}
```

This gives you the reusable infrastructure. It does not yet define engine selection or runtime policy.

## Step 2: Load engine profiles

Load Geppetto engine-profile registries from flags, config, or a default `profiles.yaml`.

```go
type requestResolver struct {
    registry              gepprofiles.Registry
    defaultRegistrySlug   gepprofiles.RegistrySlug
    baseInferenceSettings *aisettings.InferenceSettings
}

func newRequestResolver() *requestResolver {
    return &requestResolver{
        registry:              mustBuildRegistryChain(),
        defaultRegistrySlug:   gepprofiles.MustRegistrySlug("default"),
        baseInferenceSettings: mustBaseInferenceSettings(),
    }
}
```

At this point you have only the engine half of the system.

## Step 3: Define your app-owned runtime type

Do not push prompt, middleware, or tools back into engine profiles. Keep them local.

```go
type appRuntime struct {
    SystemPrompt string
    Middlewares  []runtime.MiddlewareUse
    ToolNames    []string
}
```

You can source this from:

- a small app-owned YAML file
- an app-owned profile extension
- code defaults
- request-specific selectors

The source does not matter as much as keeping it app-owned.

## Step 4: Build a local-first resolved plan

This is the core pattern. Your resolver should build a local plan first.

```go
type resolvedRuntime struct {
    RuntimeKey         string
    RuntimeFingerprint string
    ProfileVersion     uint64

    InferenceSettings *aisettings.InferenceSettings
    SystemPrompt      string
    Middlewares       []runtime.MiddlewareUse
    ToolNames         []string
    ProfileMetadata   map[string]any
}

type resolvedConversationPlan struct {
    ConvID         string
    Prompt         string
    IdempotencyKey string
    Runtime        *resolvedRuntime
}
```

This plan is the thing your tests should assert against. It reflects your app’s actual domain model.

## Step 5: Resolve engine settings and app runtime separately

Inside the resolver, do the split explicitly.

```go
func (r *requestResolver) buildPlan(ctx context.Context, req *http.Request, prompt string) (*resolvedConversationPlan, error) {
    selectedRegistry := resolveRegistrySelection(req)
    selectedProfile := resolveProfileSelection(req)

    resolvedProfile, err := r.registry.ResolveEngineProfile(ctx, gepprofiles.ResolveInput{
        RegistrySlug:      selectedRegistry,
        EngineProfileSlug: selectedProfile,
    })
    if err != nil {
        return nil, err
    }

    finalSettings, err := gepprofiles.MergeInferenceSettings(r.baseInferenceSettings, resolvedProfile.InferenceSettings)
    if err != nil {
        return nil, err
    }

    runtimePolicy := resolveAppRuntimePolicy(req, resolvedProfile)

    runtime := &resolvedRuntime{
        RuntimeKey:         computeRuntimeKey(selectedRegistry, selectedProfile),
        RuntimeFingerprint: computeRuntimeFingerprint(finalSettings, runtimePolicy),
        ProfileVersion:     profileVersionFromMetadata(resolvedProfile.Metadata),
        InferenceSettings:  finalSettings,
        SystemPrompt:       runtimePolicy.SystemPrompt,
        Middlewares:        runtimePolicy.Middlewares,
        ToolNames:          runtimePolicy.ToolNames,
        ProfileMetadata:    cloneStringAnyMap(resolvedProfile.Metadata),
    }

    return &resolvedConversationPlan{
        ConvID:         convIDFromRequest(req),
        Prompt:         prompt,
        IdempotencyKey: idempotencyKeyFromRequest(req),
        Runtime:        runtime,
    }, nil
}
```

Notice what is missing:

- no `EffectiveRuntime`
- no Geppetto-owned system prompt
- no mixed profile mutation

That is the point of the new architecture.

## Step 6: Convert into the shared transport once

Only when calling the shared webchat path should you convert into `webhttp.ResolvedConversationRequest`.

```go
func toResolvedConversationRequest(plan *resolvedConversationPlan) webhttp.ResolvedConversationRequest {
    return webhttp.ResolvedConversationRequest{
        ConvID:                    plan.ConvID,
        RuntimeKey:                plan.Runtime.RuntimeKey,
        RuntimeFingerprint:        plan.Runtime.RuntimeFingerprint,
        ProfileVersion:            plan.Runtime.ProfileVersion,
        ResolvedInferenceSettings: plan.Runtime.InferenceSettings.Clone(),
        ResolvedRuntime: &runtime.ProfileRuntime{
            SystemPrompt: plan.Runtime.SystemPrompt,
            Middlewares:  append([]runtime.MiddlewareUse(nil), plan.Runtime.Middlewares...),
            Tools:        append([]string(nil), plan.Runtime.ToolNames...),
        },
        ProfileMetadata: cloneStringAnyMap(plan.Runtime.ProfileMetadata),
        Prompt:          plan.Prompt,
        IdempotencyKey:  plan.IdempotencyKey,
    }
}
```

This explicit conversion is what keeps the app local-first instead of transport-first.

## Step 7: Register read-only profile and schema APIs

Mount the shared helpers for profile inspection and schema discovery.

```go
webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
    DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
    EnableCurrentProfileCookieRoute: true,
    MiddlewareDefinitions:           middlewareDefinitions,
    ExtensionCodecRegistry:          extensionCodecRegistry,
})
```

These endpoints are useful for UI selection and schema inspection. They are not CRUD APIs.

## Step 8: Build the runtime composer

Your runtime composer should assume the resolver already did the hard selection work.

```go
func (c *runtimeComposer) Compose(ctx context.Context, req runtime.ConversationRuntimeRequest) (runtime.ComposedRuntime, error) {
    eng, err := runtime.BuildEngineFromSettingsWithMiddlewares(
        ctx,
        req.ResolvedInferenceSettings,
        req.ResolvedProfileRuntime.SystemPrompt,
        resolveMiddlewares(req.ResolvedProfileRuntime.Middlewares),
    )
    if err != nil {
        return runtime.ComposedRuntime{}, err
    }

    return runtime.ComposedRuntime{
        Engine:             eng,
        RuntimeKey:         req.ProfileKey,
        RuntimeFingerprint: req.ResolvedProfileFingerprint,
        SeedSystemPrompt:   req.ResolvedProfileRuntime.SystemPrompt,
    }, nil
}
```

The composer builds and wraps the engine. It does not choose the engine profile.

## Step 9: Add tools and frontend hydration

Register tools on the server and let the frontend follow the standard transport pattern:

1. determine `conv_id`
2. open `/ws?conv_id=...`
3. fetch `/api/timeline`
4. replay buffered frames
5. send prompts through `POST /chat`

That is unchanged by the profile split.

## Step 10: Validate the migration

Add tests at three layers:

### Resolver tests

- selected engine profile changes final `InferenceSettings`
- local resolved plan carries prompt/tools/middlewares
- runtime key and fingerprint are computed from app policy

### Transport tests

- `toResolvedConversationRequest(...)` preserves runtime values
- nil settings/runtime behavior is explicit

### End-to-end tests

- `POST /chat` and `GET /ws` use the same selection rules
- `/api/chat/profiles` reflects the engine-profile registry
- `/api/timeline` hydrates the same conversation seen over WebSocket

Recommended commands:

```bash
go test ./cmd/web-chat ./pkg/webchat/... -count=1
go test ./cmd/pinocchio/... -count=1
```

## Complete Mental Model

If you remember only one diagram, use this one:

```text
engine profile registry (Geppetto)
  -> ResolveEngineProfile
  -> MergeInferenceSettings
  -> final engine settings

app runtime policy (your app)
  -> prompt
  -> middlewares
  -> tools
  -> runtime key/fingerprint

local resolved conversation plan
  -> convert once to shared transport
  -> webchat server + stream hub + chat service
```

That is the stable design.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| The selected profile slug changes but the model does not | Resolver still uses base settings only | Merge `resolvedProfile.InferenceSettings` into the base settings |
| Prompt/middlewares/tools still feel “magical” | They are still being read from the wrong layer | Move them into app runtime policy |
| Runtime transport leaks everywhere | App code is still building `webhttp.ResolvedConversationRequest` directly | Introduce a local plan type and convert once |
| `/api/chat/profiles` seems incomplete | The API only exposes engine profiles and current-profile selection | Keep app runtime docs and config in your app layer |

## See Also

- [Webchat Engine Profile Guide](../topics/webchat-profile-registry.md) — current engine-profile and app-runtime split
- [Webchat Engine Profile Migration Playbook](../topics/webchat-engine-profile-migration-playbook.md) — migration path from older mixed-profile apps
- [Webchat Framework Guide](../topics/webchat-framework-guide.md) — backend architecture and constructor choices
- [Webchat HTTP Chat Setup](../topics/webchat-http-chat-setup.md) — canonical route contract
