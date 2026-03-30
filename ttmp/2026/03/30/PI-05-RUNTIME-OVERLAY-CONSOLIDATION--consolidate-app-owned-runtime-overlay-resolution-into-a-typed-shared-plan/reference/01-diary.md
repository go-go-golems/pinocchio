---
Title: Diary
Ticket: PI-05-RUNTIME-OVERLAY-CONSOLIDATION
Status: active
Topics:
    - pinocchio
    - profiles
    - webchat
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/engineprofiles/registry.go
      Note: Added typed resolved stack lineage on the shared resolved-profile type
    - Path: geppetto/pkg/engineprofiles/service.go
      Note: Populates typed stack lineage and preserves legacy metadata
    - Path: pinocchio/pkg/inference/runtime/runtime_plan.go
      Note: Shared runtime-plan, merge, and fingerprint helpers
    - Path: pinocchio/cmd/web-chat/profile_policy.go
      Note: First in-repo consumer migrated to the shared helper
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-chat/pkg/profilechat/request_resolver.go
      Note: Downstream app migrated to the shared helper inside its workspace
    - Path: /home/manuel/code/gec/2026-03-16--gec-rag/internal/webchat/resolver.go
      Note: Adapter seam matching the shared plan shape without a local-path dependency hack
ExternalSources: []
Summary: Step-by-step diary for introducing a typed shared runtime-plan helper, migrating Pinocchio web-chat and go-go-os-chat to it, and aligning gec-rag with a typed adapter seam.
LastUpdated: 2026-03-30T18:40:00-04:00
WhatFor: Record the implementation sequence, exact commits, decisions, validation commands, and remaining follow-up for PI-05.
WhenToUse: Use when reviewing the implementation, replaying the migration in another repo, or understanding why gec-rag stopped short of a direct shared-helper dependency.
---

# Diary

## Goal

The goal was to turn the repeated “resolve engine profile, then apply app-owned runtime policy” pattern into a shared typed helper instead of three independent handwritten implementations. The intended end state was:

- a typed resolved runtime plan in `pinocchio/pkg/inference/runtime`
- typed stack lineage on `geppetto` resolved profiles
- one shared runtime fingerprint helper
- `pinocchio/cmd/web-chat` migrated to the helper
- `go-go-os-chat` migrated to the helper
- `gec-rag` moved toward the same shape without committing machine-specific dependency wiring

## Starting observations

The three implementations were close in architecture but not in details.

- `pinocchio/cmd/web-chat/profile_policy.go` already had the most robust behavior:
  base app runtime overlay, stack-aware runtime replay, default-on `agentmode`, and local runtime fingerprinting.
- `go-go-os-chat/pkg/profilechat/request_resolver.go` only resolved the leaf runtime extension from the selected engine profile.
- `gec-rag/internal/webchat/resolver.go` had a second overlay source, the application profile store, and merged it with the resolved inference runtime by hand.

The main duplication/risk points were:

- repeated `profile.version` extraction from `map[string]any`
- local parsing of `profile.stack.lineage`
- multiple local runtime fingerprint payload shapes
- multiple handwritten runtime merge semantics

## Decision: where the shared helper lives

The shared logic belongs in `pinocchio/pkg/inference/runtime`, not in app packages.

Reasoning:

- the output is still Pinocchio-owned runtime policy
- the helper needs direct access to `ProfileRuntime`, `MiddlewareUse`, and runtime merge semantics
- multiple apps already depend on `pinocchio/pkg/inference/runtime`

`geppetto` still needed one supporting change: the resolved engine profile type needed a typed stack-lineage field so apps would stop reparsing `profile.stack.lineage` from metadata blobs.

## Implementation sequence

### 1. Add typed stack lineage in Geppetto

Files changed in the main sanitize workspace:

- [registry.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/engineprofiles/registry.go)
- [service.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/engineprofiles/service.go)
- [service_test.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/geppetto/pkg/engineprofiles/service_test.go)

What changed:

- added `ResolvedProfileStackEntry`
- added `ResolvedEngineProfile.StackLineage []ResolvedProfileStackEntry`
- populated that field during profile resolution
- preserved the old `profile.stack.lineage` metadata entry for compatibility

Commit in the main sanitize workspace:

- `1b34ec8` `Add typed resolved profile stack lineage`

Validation:

```bash
go test ./pkg/engineprofiles -count=1
```

### 2. Add the shared runtime-plan helper in Pinocchio

Files changed:

- [runtime_plan.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/runtime_plan.go)
- [runtime_plan_test.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/runtime_plan_test.go)

What changed:

- added `ResolvedRuntimePlan`
- added `ResolveRuntimePlanOptions`
- added `MergeProfileRuntimeOptions`
- added `ToolMergeMode` with `union` and `replace`
- added `ResolveRuntimePlan(...)`
- added `MergeProfileRuntime(...)`
- added typed runtime fingerprint helpers:
  `BuildRuntimeFingerprint(...)` and `BuildRuntimeFingerprintFromSettings(...)`
- added `ProfileVersionFromResolvedMetadata(...)`

Important merge rules encoded in shared code:

- system prompt: last non-empty wins
- middleware identity: `name` plus optional `id`
- middleware merge behavior: later overlay replaces earlier entry with the same identity
- tool merge default: ordered union
- tool merge override: explicit replace mode

The first commit attempt failed repo lint for two small reasons:

- exhaustive switch lint wanted `ToolMergeModeUnion` as an explicit `switch` case
- `pinocchio/cmd/web-chat/profile_policy.go` still had an unused local inference-settings helper after the refactor work had already started

Those were fixed before the helper commit was finalized.

Commit in the main sanitize workspace:

- `65260a8` `Add shared runtime overlay plan helpers`

Validation:

```bash
go test ./pkg/inference/runtime -count=1
```

The actual commit also passed the repo pre-commit hooks, including lint and the full test run.

### 3. Migrate Pinocchio web-chat to the shared helper

Files changed:

- [profile_policy.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go)
- [runtime_composer.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/runtime_composer.go)
- [runtime_composer_test.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/runtime_composer_test.go)

What changed:

- removed local stack-lineage replay and local runtime merge code from `profile_policy.go`
- added a thin `resolveRuntimePlan(...)` wrapper that delegates to `infruntime.ResolveRuntimePlan(...)`
- kept `defaultWebChatProfileRuntime()` as the app-owned base overlay provider
- switched fingerprint generation to `infruntime.BuildRuntimeFingerprintFromSettings(...)`
- removed the local `RuntimeFingerprintInput` and `buildRuntimeFingerprint(...)` from the runtime composer

Net effect:

- `web-chat` still behaves the same externally
- stack-aware runtime replay now uses the shared helper
- runtime fingerprinting now uses the shared typed payload
- `profile_policy.go` got materially smaller and lost most of its ad hoc merge code

Commit in the main sanitize workspace:

- `9ed199c` `Use shared runtime overlay plan in web chat`

Focused validation:

```bash
go test ./pkg/inference/runtime ./cmd/web-chat -count=1
```

The commit also passed the repo pre-commit hooks.

### 4. Port the framework slice into the wesen-os workspace clones

`go-go-os-chat` sits under `wesen-os`, which already has a `go.work` file pointing at local clones of:

- `workspace-links/geppetto`
- `workspace-links/pinocchio`
- `workspace-links/go-go-os-chat`

That meant I could migrate the downstream app cleanly without adding local-path `replace` directives to `go-go-os-chat`.

I replayed the framework commits from the sanitize workspace into those sibling clones with `format-patch | git am -3`.

Companion commits:

- `wesen-os/workspace-links/geppetto`: `5958ed12` `Add typed resolved profile stack lineage`
- `wesen-os/workspace-links/pinocchio`: `6200c04` `Add shared runtime overlay plan helpers`

### 5. Migrate go-go-os-chat to the shared helper

Files changed:

- [/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-chat/pkg/profilechat/request_resolver.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-chat/pkg/profilechat/request_resolver.go)
- [/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-chat/pkg/profilechat/runtime_composer.go](/home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-chat/pkg/profilechat/runtime_composer.go)

What changed:

- `request_resolver.go` now calls `infruntime.ResolveRuntimePlan(...)` instead of resolving only the leaf runtime extension
- the resolver now uses the shared typed profile-version and runtime-fingerprint behavior
- the runtime composer now uses `BuildRuntimeFingerprintFromSettings(...)`
- local leaf-only runtime-resolution code and local fingerprint payload code were removed

This was the first real proof that the shared helper could replace duplicated app logic outside the main Pinocchio repo.

Commit:

- `3858008` `Use shared runtime overlay plan in profile chat`

Validation from the parent `wesen-os` workspace:

```bash
go test ./workspace-links/geppetto/pkg/engineprofiles \
  ./workspace-links/pinocchio/pkg/inference/runtime \
  ./workspace-links/go-go-os-chat/pkg/profilechat -count=1
```

### 6. Add a typed migration seam in gec-rag

`gec-rag` does not have a local workspace pointing at the in-progress Pinocchio and Geppetto clones. I explicitly avoided committing machine-specific `replace` paths or a local-path dependency hack in `go.mod`.

Instead, I implemented a migration seam that matches the shared model:

- introduced a typed `resolvedInferenceRuntimePlan`
- moved the “resolved inference settings + inference runtime + profile version + copied profile metadata” logic behind `resolveInferenceRuntimePlan(...)`
- renamed the app-level merge step to `mergeApplicationProfileRuntime(...)`
- replaced the remaining `map[string]any` runtime fingerprint payload with a typed struct:
  `resolvedRuntimeFingerprintInput`

File changed:

- [/home/manuel/code/gec/2026-03-16--gec-rag/internal/webchat/resolver.go](/home/manuel/code/gec/2026-03-16--gec-rag/internal/webchat/resolver.go)

This keeps `gec-rag` aligned with the shared runtime-plan shape without forcing an unreleased dependency.

Commit:

- `3ced495` `Add typed inference runtime plan seam`

Validation:

```bash
go test ./internal/webchat -count=1
```

## Pseudocode summary

The shared runtime-plan flow now looks like this:

```go
resolvedProfile := registry.ResolveEngineProfile(...)

plan := ResolveRuntimePlan(
    ctx,
    registry,
    resolvedProfile,
    ResolveRuntimePlanOptions{
        BaseInferenceSettings: appBaseInferenceSettings,
        BaseRuntime:           appBaseRuntimeOverlay,
    },
)

runtimeFingerprint := BuildRuntimeFingerprintFromSettings(
    runtimeKey,
    plan.ProfileVersion,
    plan.Runtime,
    plan.InferenceSettings,
)
```

The `gec-rag` transitional seam looks like this:

```go
inferenceProfile := resolveEffectiveProfile(...)
inferencePlan := resolveInferenceRuntimePlan(ctx, inferenceProfile)

runtime := mergeApplicationProfileRuntime(appProfile, inferencePlan.Runtime)
fingerprint := fingerprintResolvedRuntime(
    appProfileSlug,
    registrySlug,
    profileSlug,
    runtime,
    inferencePlan.InferenceSettings,
)
```

## Exact commits

Main sanitize workspace:

- `geppetto`: `1b34ec8` `Add typed resolved profile stack lineage`
- `pinocchio`: `65260a8` `Add shared runtime overlay plan helpers`
- `pinocchio`: `9ed199c` `Use shared runtime overlay plan in web chat`

Companion workspace clones:

- `wesen-os/workspace-links/geppetto`: `5958ed12` `Add typed resolved profile stack lineage`
- `wesen-os/workspace-links/pinocchio`: `6200c04` `Add shared runtime overlay plan helpers`
- `wesen-os/workspace-links/go-go-os-chat`: `3858008` `Use shared runtime overlay plan in profile chat`

Application adapter seam:

- `gec-rag`: `3ced495` `Add typed inference runtime plan seam`

## Validation summary

Commands run successfully:

```bash
# sanitize workspace geppetto
go test ./pkg/engineprofiles -count=1

# sanitize workspace pinocchio
go test ./pkg/inference/runtime ./cmd/web-chat -count=1

# wesen-os workspace
go test ./workspace-links/geppetto/pkg/engineprofiles \
  ./workspace-links/pinocchio/pkg/inference/runtime \
  ./workspace-links/go-go-os-chat/pkg/profilechat -count=1

# gec-rag
go test ./internal/webchat -count=1
```

Additional verification:

- both Pinocchio commits in the sanitize workspace passed the repo pre-commit hooks
- the first helper commit failure was corrected and documented above

## What remains

The remaining work is mostly release and documentation work, not local architecture uncertainty.

- `gec-rag` should switch from the local typed seam to direct `ResolveRuntimePlan(...)` consumption after a real `pinocchio` and `geppetto` dependency bump
- Pinocchio still needs a focused doc page that explains the runtime-overlay contract, the merge rules, and which `map[string]any` surfaces are still intentional
- if more apps introduce more overlay sources, it may become worth adding a first-class overlay-source interface on top of the current shared helper

## Review guidance

To review the implementation efficiently:

- start with [runtime_plan.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/pkg/inference/runtime/runtime_plan.go)
- then inspect [profile_policy.go](/home/manuel/workspaces/2026-03-28/sanitize-yaml-structured-events/pinocchio/cmd/web-chat/profile_policy.go) to see how much local code disappeared
- then compare `go-go-os-chat`’s resolver before and after the migration to confirm it is no longer leaf-only
- finish with [resolver.go](/home/manuel/code/gec/2026-03-16--gec-rag/internal/webchat/resolver.go) to see the incremental adapter seam
