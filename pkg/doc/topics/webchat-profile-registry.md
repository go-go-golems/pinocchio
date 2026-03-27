---
Title: Webchat Engine Profile Guide
Slug: webchat-profile-registry
Short: Reference for engine-profile registry wiring, selection precedence, and the split between Geppetto engine profiles and app-owned webchat runtime policy.
Topics:
- webchat
- profiles
- registry
- http
- backend
Commands:
- web-chat
Flags:
- profile
- profile-registries
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## What This Page Covers

This page explains the current profile model for Pinocchio webchat applications.

The important rule is simple:

- Geppetto engine profiles choose engine settings
- the webchat app chooses prompt, middlewares, tools, runtime key, and runtime fingerprint

Use this page when you need to understand:

- how engine-profile registries are loaded
- how profile selection is resolved from requests
- how the resolver splits engine settings from app runtime policy
- what the shared read-only profile endpoints expose

## Architecture at a Glance

There are three different layers, and they should stay separate.

### 1. Engine profile registry

This is Geppetto-owned. It lives in [`geppetto/pkg/engineprofiles`](../../../../geppetto/pkg/engineprofiles) and resolves final [`InferenceSettings`](../../../../geppetto/pkg/steps/ai/settings/settings-inference.go).

### 2. App runtime policy

This is Pinocchio- or app-owned. It covers:

- `system_prompt`
- runtime middlewares
- tool exposure
- runtime identity and fingerprints

Pinocchio’s shared runtime payload is defined in [`pkg/inference/runtime/profile_runtime.go`](../../inference/runtime/profile_runtime.go).

### 3. Shared webchat transport

This is the boundary between app resolution and shared webchat execution. The app should build its own local plan first, then convert into the shared request transport once.

## Base Settings Source

The `web-chat` command uses a hidden base inference-settings path rather than exposing the full Geppetto AI surface on its public CLI.

That base currently comes from:

- shared Geppetto sections
- config files
- environment variables
- defaults

via `profilebootstrap.ResolveBaseInferenceSettings(...)` in `pinocchio/cmd/web-chat/main.go`.

That means shared baseline fields such as `ai-client.*` can already matter for webchat through config and environment even though the `web-chat` command does not currently expose those flags directly.

It also means widening `web-chat` to expose `ai-client` flags would require more than just mounting another section. The runtime baseline path would also need to preserve those parsed CLI values.

## Registry Bootstrap

The engine-profile registry stack usually comes from:

1. explicit CLI flags
2. app config
3. environment variables
4. `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` when present

Common bootstrap pattern:

```go
entries, err := gepprofiles.ParseEngineProfileRegistrySourceEntries(rawSources)
if err != nil { ... }

chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
if err != nil { ... }
defer chain.Close()

resolved, err := chain.ResolveEngineProfile(ctx, gepprofiles.ResolveInput{
    RegistrySlug:      selectedRegistry,
    EngineProfileSlug: selectedProfile,
})
if err != nil { ... }

finalSettings, err := gepprofiles.MergeInferenceSettings(baseInferenceSettings, resolved.InferenceSettings)
if err != nil { ... }
```

That is the engine half of the story only.

## Request Selection Precedence

For chat and websocket requests, the typical webchat selection order is:

1. path slug (`POST /chat/{profile}`)
2. request body `profile`
3. query `profile`
4. current-profile cookie
5. registry default profile

Registry selection usually comes from:

1. request body `registry`
2. query `registry`
3. app-configured default registry

Resolvers should return typed `RequestResolutionError` values for invalid selections so handlers can map them to `400`, `404`, or `500`.

## What the Resolver Should Return

Do not let the resolver think in terms of a monolithic “resolved profile runtime.” The resolver should compute two things separately:

- final `InferenceSettings`
- app-owned runtime policy

Good local-first pseudocode:

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

The shared transport conversion should happen after this local plan exists.

## App Runtime Policy

Pinocchio webchat uses an app-owned profile extension for its runtime policy:

- `pinocchio.webchat_runtime@v1`

This extension stores values like:

```yaml
extensions:
  pinocchio.webchat_runtime@v1:
    system_prompt: You are a careful analyst.
    middlewares:
      - name: agentmode
        id: analyst
        config:
          default_mode: analyst
    tools:
      - search
      - summarize
```

Those fields are not engine settings. They should not move back into Geppetto engine profiles.

## Runtime Composer Contract

By the time runtime composition runs, the split should already be resolved:

- `ResolvedInferenceSettings` contains final engine settings
- `ResolvedRuntime` contains app-owned prompt/tool/middleware policy

The composer should:

1. build the engine from `ResolvedInferenceSettings`
2. apply system prompt and middlewares
3. expose tool names for upstream registry filtering

It should not try to re-resolve engine profiles.

## Read-Only HTTP APIs

Mount reusable handlers with:

```go
webhttp.RegisterProfileAPIHandlers(mux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
    DefaultRegistrySlug:             gepprofiles.MustRegistrySlug("default"),
    EnableCurrentProfileCookieRoute: true,
    MiddlewareDefinitions:           middlewareDefinitions,
    ExtensionCodecRegistry:          extensionCodecRegistry,
})
```

### Endpoints

| Endpoint | Methods | Purpose |
|---|---|---|
| `/api/chat/profiles` | `GET` | list engine profiles |
| `/api/chat/profiles/{slug}` | `GET` | read one engine profile |
| `/api/chat/profile` | `GET`, `POST` | current-profile cookie read/write |
| `/api/chat/schemas/middlewares` | `GET` | discover middleware config schemas |
| `/api/chat/schemas/extensions` | `GET` | discover extension schemas |

These are read-only shared APIs. They are not profile CRUD endpoints.

## Current Profile vs Runtime Truth

`/api/chat/profile` stores UI selection state. It does not define runtime truth for all turns.

Runtime truth is per request and per turn:

- each incoming chat or websocket request resolves engine profile selection
- the app computes runtime key and fingerprint
- conversation state records which runtime was active for a given turn

This matters when a user changes profile mid-conversation. Old turns keep their original runtime attribution.

## Testing Recommendations

Minimum coverage for a profile-aware webchat app:

- list -> select -> chat request uses selected engine profile slug
- resolved engine profile changes final `InferenceSettings`
- local resolved plan captures app-owned prompt/tools/middlewares
- conversion to shared transport preserves those fields

These tests should live at the app layer, not only in the Geppetto engine-profile package.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Profile selection changes but the model does not change | Resolver never merged `resolved.InferenceSettings` onto base settings | Call `MergeInferenceSettings(...)` before runtime composition |
| Engine profile docs show prompt/middleware fields | You are still using legacy mixed-profile YAML | Migrate to engine-only `inference_settings` and move runtime policy to app config or extensions |
| Frontend schema pages are empty | Middleware definitions or extension codecs were not passed to the profile API handler | Wire `MiddlewareDefinitions` and `ExtensionCodecRegistry` into `RegisterProfileAPIHandlers(...)` |
| Runtime fingerprint does not rebuild when prompt/tools change | Fingerprint is computed from engine settings only | Include app runtime policy in the app-owned fingerprint payload |

## See Also

- [Webchat Engine Profile Migration Playbook](webchat-engine-profile-migration-playbook.md) — full migration from the older mixed model to the current split
- [Webchat Framework Guide](webchat-framework-guide.md) — handler-first backend integration and resolver/composer wiring
- [Webchat HTTP Chat Setup](webchat-http-chat-setup.md) — canonical route table and request contract
- [Third-Party Webchat Playbook](../tutorials/03-thirdparty-webchat-playbook.md) — build a webchat-style app from scratch with the current model
- [Pinocchio Profile Resolution and Runtime Switching](pinocchio-profile-resolution-and-runtime-switching.md) — explains hidden base settings, baseline ownership, and why webchat CLI widening needs a parsed-values-aware base path
