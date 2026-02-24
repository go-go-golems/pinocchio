---
Title: Pinocchio Runtime Symbol Migration Playbook
Slug: runtime-symbol-migration-playbook
Short: Upgrade third-party integrations from legacy runtime symbol names to canonical names after alias removal.
Topics:
- pinocchio
- runtime
- migration
- thirdparty
Commands:
- web-chat
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Runtime Symbol Migration Playbook

## Purpose

This playbook covers the runtime API rename in `pinocchio/pkg/inference/runtime` where compatibility aliases were removed. It explains what changed, how to update imports and types, and how to verify your integration after migration.

Use this guide if your code imports any of:

- `RuntimeComposeRequest`
- `RuntimeArtifacts`
- `RuntimeComposer`
- `RuntimeComposerFunc`
- `ComposeEngineFromSettings`
- `MiddlewareFactory`
- `ToolFactory`
- `MiddlewareUse`

## Old-to-New Symbol Map

| Old symbol | New symbol |
|---|---|
| `RuntimeComposeRequest` | `ConversationRuntimeRequest` |
| `RuntimeArtifacts` | `ComposedRuntime` |
| `RuntimeComposer` | `RuntimeBuilder` |
| `RuntimeComposerFunc` | `RuntimeBuilderFunc` |
| `ComposeEngineFromSettings` | `BuildEngineFromSettings` |
| `MiddlewareFactory` | `MiddlewareBuilder` |
| `ToolFactory` | `ToolRegistrar` |
| `MiddlewareUse` | `MiddlewareSpec` |

## Why This Rename Happened

The previous names leaked historical webchat terminology and were too generic in a few places. The new names make the ownership and intent explicit:

- request DTOs are conversation-runtime requests,
- composed outputs are runtime artifacts for execution,
- factories are builders/registrars, not generic factories.

This improves API readability for downstream users and makes future runtime abstractions easier to evolve.

## Migration Steps

### 1. Replace type names in signatures

Before:

```go
func build(r infruntime.RuntimeComposer) error
```

After:

```go
func build(r infruntime.RuntimeBuilder) error
```

### 2. Replace function adapters and request/response DTOs

Before:

```go
composer := infruntime.RuntimeComposerFunc(
    func(ctx context.Context, req infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
        // ...
    },
)
```

After:

```go
composer := infruntime.RuntimeBuilderFunc(
    func(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
        // ...
    },
)
```

### 3. Replace runtime engine assembly symbols

Before:

```go
eng, err := infruntime.ComposeEngineFromSettings(ctx, stepSettings, prompt, uses, factories)
```

After:

```go
eng, err := infruntime.BuildEngineFromSettings(ctx, stepSettings, prompt, uses, factories)
```

### 4. Replace middleware/tool helper type names

Before:

```go
uses := []infruntime.MiddlewareUse{...}
factories := map[string]infruntime.MiddlewareFactory{...}
tools := map[string]infruntime.ToolFactory{...}
```

After:

```go
uses := []infruntime.MiddlewareSpec{...}
factories := map[string]infruntime.MiddlewareBuilder{...}
tools := map[string]infruntime.ToolRegistrar{...}
```

## Suggested Search-and-Replace Commands

Run these from repository root:

```bash
rg -n "RuntimeComposeRequest|RuntimeArtifacts|RuntimeComposerFunc|RuntimeComposer|ComposeEngineFromSettings|MiddlewareFactory|ToolFactory|MiddlewareUse"
```

Then replace symbols in this order:

1. DTOs (`RuntimeComposeRequest`, `RuntimeArtifacts`)
2. Interfaces/adapters (`RuntimeComposer`, `RuntimeComposerFunc`)
3. Builders/factories (`ComposeEngineFromSettings`, `MiddlewareFactory`, `ToolFactory`, `MiddlewareUse`)

## Validation Checklist

1. `go test ./pkg/inference/runtime -count=1`
2. `go test ./cmd/web-chat -count=1`
3. `go test ./pkg/webchat/... -count=1`
4. `go test ./... -count=1`

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `undefined: infruntime.RuntimeComposeRequest` | Alias removed | Replace with `infruntime.ConversationRuntimeRequest` |
| `undefined: infruntime.RuntimeComposerFunc` | Alias removed | Replace with `infruntime.RuntimeBuilderFunc` |
| `undefined: infruntime.ComposeEngineFromSettings` | Wrapper removed | Replace with `infruntime.BuildEngineFromSettings` |
| Type mismatch in middleware arrays | Legacy `MiddlewareUse` still referenced | Change to `[]infruntime.MiddlewareSpec` |
| Type mismatch in factory maps | Legacy `MiddlewareFactory`/`ToolFactory` still referenced | Use `MiddlewareBuilder` and `ToolRegistrar` |

## See Also

- `pinocchio/pkg/doc/topics/webchat-backend-reference.md`
- `pinocchio/pkg/doc/topics/webchat-framework-guide.md`
- `pinocchio/pkg/doc/topics/webchat-http-chat-setup.md`
