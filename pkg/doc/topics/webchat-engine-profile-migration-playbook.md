---
Title: Webchat Engine Profile Migration Playbook
Slug: webchat-engine-profile-migration-playbook
Short: Step-by-step migration guide from legacy mixed Geppetto profiles to the current engine-profile plus app-runtime webchat architecture.
Topics:
- webchat
- migration
- profiles
- engineprofiles
- backend
Commands:
- web-chat
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

## Why This Migration Exists

Older webchat-style apps in this workspace grew up around a mixed profile model. A single Geppetto profile used to carry both engine configuration and app runtime behavior. That meant one object tried to answer two unrelated questions:

- which provider/model/client settings should build the engine?
- which system prompt, middleware set, and tool exposure should the application apply around that engine?

That model was convenient at first, but it became hard to reason about. Engine settings belong to Geppetto. Prompt, middleware, tool exposure, runtime keys, and fingerprints belong to the application. The current architecture makes that split explicit.

This playbook explains how to migrate an older webchat-style backend all the way to the current model:

- Geppetto owns engine profiles and resolves final `InferenceSettings`
- your app owns runtime policy and request selection
- your app builds a local resolved runtime plan first
- only the final handoff uses the shared Pinocchio transport types

## The Old Model vs the New Model

The old shape looked like this:

```text
request
  -> mixed profile registry
  -> effective runtime
  -> engine config + prompt + middlewares + tools
  -> run
```

The current shape looks like this:

```text
request
  -> resolve engine profile
  -> final InferenceSettings
  -> resolve app runtime policy
  -> local resolved runtime plan
  -> convert once to shared webchat transport
  -> run
```

The important improvement is that the app owns the runtime layer instead of smuggling it through Geppetto.

## Target Architecture

Use this split as the migration target.

### Geppetto owns

- `engineprofiles.EngineProfile`
- `engineprofiles.EngineProfileRegistry`
- `ResolveEngineProfile(...)`
- `MergeInferenceSettings(...)`
- engine construction from final `InferenceSettings`

### Your webchat app owns

- system prompt
- runtime middlewares
- tool exposure / allowed tool names
- runtime key
- runtime fingerprint
- request selection precedence
- cookie semantics
- local runtime YAML or local profile extensions

### Shared Pinocchio webchat owns

- conversation lifecycle
- streaming coordination
- websocket attach flow
- timeline hydration
- a narrow transport contract between app resolution and shared runtime execution

## Migration Checklist

Use these steps in order. Do not try to preserve the old mixed semantics while also adopting the new split. That only creates a half-migrated system.

### 1. Remove mixed runtime data from Geppetto profiles

Your engine profile YAML should keep only engine settings. A profile should define:

- provider/api type
- model
- client/base URL/API key related fields
- inference controls like `max_tokens` and `temperature`

Do not keep these in Geppetto engine profiles:

- `system_prompt`
- `middlewares`
- `tools`
- `runtime_key`
- `runtime_fingerprint`

### 2. Introduce an app-owned runtime payload

Define a local type for the policy your webchat app owns. Pinocchio’s shared example is [`ProfileRuntime`](../../pkg/inference/runtime/profile_runtime.go), but app code should still treat this as a boundary shape, not as the primary domain model.

Example:

```go
type AppRuntime struct {
    SystemPrompt string
    Middlewares  []runtime.MiddlewareUse
    ToolNames    []string
}
```

If you need durable storage for app runtime policy, store it in app-owned YAML or in app-owned profile extensions.

### 3. Resolve engine profiles separately from runtime policy

Your request resolver should stop thinking in terms of “resolve one full profile.” Resolve two things:

1. engine settings from Geppetto engine profiles
2. runtime policy from app config

Pseudocode:

```go
resolvedEngineProfile, err := engineProfileRegistry.ResolveEngineProfile(ctx, engineprofiles.ResolveInput{
    RegistrySlug:      selectedRegistry,
    EngineProfileSlug: selectedProfile,
})
if err != nil { ... }

finalSettings, err := engineprofiles.MergeInferenceSettings(baseInferenceSettings, resolvedEngineProfile.InferenceSettings)
if err != nil { ... }

appRuntime := resolveAppRuntimePolicy(selectedProfile, req)
```

### 4. Build a local-first resolved runtime plan

Do not build the shared transport object immediately. First create a local type that reflects your app’s own domain.

Example:

```go
type ResolvedConversationPlan struct {
    ConvID         string
    Prompt         string
    IdempotencyKey string
    Runtime        *ResolvedRuntime
}

type ResolvedRuntime struct {
    RuntimeKey         string
    RuntimeFingerprint string
    ProfileVersion     uint64

    InferenceSettings *aisettings.InferenceSettings
    SystemPrompt      string
    Middlewares       []runtime.MiddlewareUse
    ToolNames         []string
    ProfileMetadata   map[string]any
}
```

This is the most important migration step. Once the app has this local plan, tests become clearer and future changes stop leaking through shared DTOs.

### 5. Convert to the shared transport only at the boundary

When you call the shared webchat services, convert the local plan once.

```go
func toResolvedConversationRequest(plan *ResolvedConversationPlan) webhttp.ResolvedConversationRequest {
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

If your app still constructs `webhttp.ResolvedConversationRequest` directly inside the resolver, you are not done yet.

### 6. Update the runtime composer

The runtime composer should now assume:

- `ResolvedInferenceSettings` already contains final engine settings
- `ResolvedRuntime` contains app-owned prompt/tool/middleware policy

The composer’s job is narrow:

- build engine from final `InferenceSettings`
- apply prompt/middlewares
- pass tool names upward so the app can filter registries before execution

### 7. Update profile and schema APIs

Shared profile endpoints are now read-only. They are still useful, but they should be interpreted differently:

- `/api/chat/profiles` and `/api/chat/profiles/{slug}` expose engine-profile documents
- `/api/chat/profile` is a cookie-based current-selection route, not a mutation API
- `/api/chat/schemas/middlewares` and `/api/chat/schemas/extensions` help the frontend understand app-owned runtime policy

Do not rebuild CRUD around the old mixed-profile model.

### 8. Update examples and local config

If your app ships a `profiles.yaml`, rewrite it to engine-only YAML. If your app also needs prompt/tools/middlewares, put that in:

- a separate app config file, or
- app-owned extensions on top of engine profiles

Use [examples/js/profiles/basic.yaml](/home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/profiles/basic.yaml) as the minimal reference shape.

### 9. Validate the new boundary explicitly

Add tests at three levels:

- engine-profile resolution tests
- local resolved plan tests
- shared transport conversion tests

Recommended commands:

```bash
go test ./cmd/web-chat ./pkg/webchat/... -count=1
go test ./cmd/pinocchio/... -count=1
```

## Concrete Before/After Example

### Before

```go
resolved, err := gepprofiles.ResolveEffectiveProfile(...)
if err != nil { ... }

return webhttp.ResolvedConversationRequest{
    ConvID:             convID,
    RuntimeKey:         resolved.RuntimeKey,
    RuntimeFingerprint: resolved.RuntimeFingerprint,
    ResolvedRuntime:    resolved.EffectiveRuntime,
}
```

This shape is wrong because the app is pretending Geppetto resolved app runtime truth for it.

### After

```go
resolvedEngineProfile, err := engineProfileRegistry.ResolveEngineProfile(...)
if err != nil { ... }

finalSettings, err := engineprofiles.MergeInferenceSettings(base, resolvedEngineProfile.InferenceSettings)
if err != nil { ... }

runtime := &ResolvedRuntime{
    RuntimeKey:         computeRuntimeKey(req, resolvedEngineProfile),
    RuntimeFingerprint: computeRuntimeFingerprint(finalSettings, appRuntime),
    ProfileVersion:     profileVersionFromMetadata(resolvedEngineProfile.Metadata),
    InferenceSettings:  finalSettings,
    SystemPrompt:       appRuntime.SystemPrompt,
    Middlewares:        appRuntime.Middlewares,
    ToolNames:          appRuntime.ToolNames,
    ProfileMetadata:    cloneStringAnyMap(resolvedEngineProfile.Metadata),
}

plan := &ResolvedConversationPlan{
    ConvID:         convID,
    Prompt:         prompt,
    IdempotencyKey: idempotencyKey,
    Runtime:        runtime,
}

return toResolvedConversationRequest(plan), nil
```

This is the stable target.

## Recommended File Layout

For a new webchat-style app, prefer this split:

```text
internal/appprofiles/
  store.go               # app runtime YAML or app-owned profile format
  runtime.go             # local runtime types

internal/webchat/
  resolver.go            # request -> local plan
  runtime_composer.go    # local plan -> composed runtime
  handlers.go            # app route wiring

pkg/doc/topics/
  webchat-engine-profile-migration-playbook.md
```

If the app is small, collapse this structure. Do not create layers just to mimic a framework diagram.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Engine profile selects the right slug but the model does not change | You resolved the profile but still used base settings | Call `MergeInferenceSettings(...)` and build from the merged result |
| System prompt still looks like it comes from Geppetto | App runtime is still encoded inside the engine-profile layer | Move prompt into app-owned runtime policy |
| Tests pass in the resolver but runtime behavior is wrong | Shared transport conversion mutates data or drops fields | Add a direct local-plan-to-transport regression test |
| Webchat docs still mention CRUD or mixed runtime profiles | The migration is incomplete at the documentation layer | Update the profile guide, framework guide, and tutorials together |

## See Also

- [Webchat Engine Profile Guide](webchat-profile-registry.md) — current engine-profile and app-runtime split in Pinocchio webchat
- [Webchat Framework Guide](webchat-framework-guide.md) — handler-first embedding model and backend wiring
- [Third-Party Webchat Playbook](../tutorials/03-thirdparty-webchat-playbook.md) — build a webchat-style app from scratch with the current architecture
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md) — canonical route table and request/response contract
