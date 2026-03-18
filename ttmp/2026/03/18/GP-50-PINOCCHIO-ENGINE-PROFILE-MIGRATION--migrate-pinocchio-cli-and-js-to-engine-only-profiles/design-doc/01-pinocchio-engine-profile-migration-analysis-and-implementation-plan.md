---
Title: Pinocchio engine-profile migration analysis and implementation plan
Ticket: GP-50-PINOCCHIO-ENGINE-PROFILE-MIGRATION
Status: active
Topics:
    - pinocchio
    - migration
    - config
    - js-bindings
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/engineprofiles/types.go
      Note: Defines the new engine-only profile payload Pinocchio must target
    - Path: cmd/pinocchio/cmds/js.go
      Note: JS command bootstrap that still wires profile resolution through the old model
    - Path: cmd/pinocchio/main.go
      Note: Repository-loaded command path that underlies pinocchio code ... behavior
    - Path: examples/js/profiles/basic.yaml
      Note: Stale mixed-profile fixture that needs conversion to engine-profile format
    - Path: examples/js/runner-profile-demo.js
      Note: Current JS example that still teaches the wrong architecture
    - Path: pkg/cmds/helpers/profile_runtime.go
      Note: Shared CLI bootstrap helper that currently drops resolved engine profile settings
    - Path: pkg/js/modules/pinocchio/module.go
      Note: Base-config engine builder semantics that need to stay clear during migration
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-18T16:22:21.602594057-04:00
WhatFor: ""
WhenToUse: ""
---


# Pinocchio engine-profile migration analysis and implementation plan

## Executive Summary

Pinocchio is still wired around the old mixed runtime/profile model that Geppetto just removed. The immediate downstream breakage is concentrated in two shared entry points:

- repository-loaded CLI commands, which should make `pinocchio code unix hello` work with config and default profile registries
- `pinocchio js`, which should run a real inference script using Pinocchio config plus engine profiles

The clean migration target is:

```text
Pinocchio config/env/defaults
  -> base InferenceSettings
  -> optional engine profile registry resolution
  -> final merged InferenceSettings
  -> engine construction

Pinocchio app runtime config
  -> prompt / tools / middlewares / runtime identity
```

This document argues for a hard cut in Pinocchio as well: no compatibility wrappers for the removed mixed profile model, no attempt to preserve old runtime/profile semantics inside shared helpers, and no partial migration that keeps repository-loaded commands and `pinocchio js` on different bootstrap rules.

## Problem Statement

The old Pinocchio downstream code assumed one "profile" abstraction could handle both:

- engine/provider/model/client configuration
- app runtime behavior such as prompts, tool names, and middleware composition

That assumption is no longer valid after the Geppetto hard cut in [`pkg/engineprofiles`](../../../../../../geppetto/pkg/engineprofiles), which now exposes engine-only profiles that resolve into final [`InferenceSettings`](../../../../../../geppetto/pkg/steps/ai/settings).

Pinocchio still has several places that reflect the old model:

1. [`pkg/cmds/helpers/profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go)
   - `ResolveInferenceSettings(...)` resolves an engine profile but returns `base.Clone()` instead of applying the resolved engine profile to the base settings.
   - This means callers believe they are profile-aware while the selected engine profile may never affect the resulting engine.

2. [`cmd/pinocchio/cmds/js.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/js.go)
   - still builds a default profile-resolution path for the Geppetto JS module
   - still assumes JS can resolve "runtime" from profile registries in a way that meaningfully drives execution

3. [`examples/js/profiles/basic.yaml`](../../../../../../pinocchio/examples/js/profiles/basic.yaml)
   - still uses the removed mixed format with `runtime.system_prompt`

4. [`examples/js/runner-profile-demo.js`](../../../../../../pinocchio/examples/js/runner-profile-demo.js)
   - still uses `gp.runner.resolveRuntime({})`
   - prints runtime metadata and then builds an engine separately via `pinocchio.engines.fromDefaults(...)`
   - demonstrates the conceptual split incorrectly

5. Repository-loaded commands
   - `pinocchio code unix hello` is not a built-in Cobra subtree
   - it comes from repository-loaded YAML commands wired through [`cmd/pinocchio/main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go) and [`pkg/cmds/cobra.go`](../../../../../../pinocchio/pkg/cmds/cobra.go)
   - that means the migration target is the shared command bootstrap, not a single command package

This produces three real user-facing problems:

- selected engine profiles do not reliably influence actual engines
- the default `~/.config/pinocchio/profiles.yaml` behavior is ambiguous because the file format itself is stale
- `pinocchio js` examples appear profile-driven while actually using separately configured or hard-coded engines

## Current State Map

### 1. Shared CLI bootstrap

The main helper flow today is:

```text
ResolveBaseInferenceSettings(parsed)
  -> hidden Geppetto sections from env/config/defaults

ResolveEngineProfileSettings(parsed)
  -> profile + profile-registries from env/config/defaults

ResolveInferenceSettings(parsed)
  -> base settings
  -> optional engine profile resolution
  -> returns base.Clone(), resolved profile, closer
```

The critical issue is that the final step does not currently merge the resolved engine profile into the base settings.

### 2. Repository-loaded command path

The `pinocchio` binary loads many commands from repositories in [`cmd/pinocchio/main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go). The user's `pinocchio code unix hello` path is part of that mechanism, not a separate built-in implementation.

That means the effective migration boundary is:

```text
repository YAML command
  -> Pinocchio command loader
  -> Geppetto Cobra middleware/sections
  -> Pinocchio command runtime helper
  -> final InferenceSettings
```

If the shared helper path is wrong, all repository-loaded commands inherit that wrongness.

### 3. JS command path

The JS command currently does this:

```text
parse --config-file / --profile / --profile-registries
  -> ResolveBaseInferenceSettings
  -> loadPinocchioProfileRegistryStack
  -> gp.Register(... EngineProfileRegistry, DefaultProfileResolve ...)
  -> pjs.Register(... BaseInferenceSettings ...)
```

This reflects the old mixed-profile mental model:

- Geppetto JS handles profile resolution as if it were "runtime"
- Pinocchio JS handles engine construction from base settings separately

After the Geppetto hard cut, that split is no longer coherent. Engine profiles need to affect engine construction directly, not be treated as a runtime side channel.

## Proposed Solution

Pinocchio should adopt the same conceptual split that now exists in Geppetto:

### Geppetto-owned

- engine profile registries
- engine profile resolution
- final `InferenceSettings`
- engine construction from final settings

### Pinocchio-owned

- command prompts
- tool selection and registries
- middlewares
- app-level runtime identity and caching
- web-chat multi-profile behavior

For the CLI and JS migration, this means Pinocchio needs one canonical helper:

```go
type ResolvedEngineSettings struct {
    InferenceSettings *settings.InferenceSettings
    ResolvedProfile   *engineprofiles.ResolvedEngineProfile
    ConfigFiles       []string
    Close             func()
}

func ResolveFinalInferenceSettings(
    ctx context.Context,
    parsed *values.Values,
) (*ResolvedEngineSettings, error)
```

The helper should:

1. Resolve base settings from config/env/defaults.
2. Resolve profile selection and registry sources from config/env/defaults.
3. If no engine profile registry is configured, return the base settings as-is.
4. If a registry is configured, resolve the selected engine profile.
5. Merge the resolved profile `InferenceSettings` into a final settings object.
6. Return that final settings object plus the resolved profile metadata.

### Target CLI flow

```text
Pinocchio command
  -> parse config/profile flags
  -> ResolveFinalInferenceSettings(...)
  -> build engine from final InferenceSettings
  -> run command
```

### Target JS flow

```text
pinocchio js
  -> parse config/profile flags
  -> ResolveFinalInferenceSettings(...)
  -> JS gets:
       - base/final engine settings through pinocchio.engines helpers
       - engine profile registry for inspection only if still useful
  -> scripts that want a real inference build an engine from the resolved settings path
```

## Design Decisions

### Decision 1: Hard cut, no compatibility wrappers

We will not preserve old mixed runtime profile behavior inside Pinocchio helpers.

Rationale:

- the old mental model is exactly what caused the current confusion
- compatibility wrappers would keep CLI and JS behavior ambiguous
- downstream apps do not need that complexity

### Decision 2: Fix the shared bootstrap before individual commands

We start with helper and command-loading infrastructure, not with a single command or example.

Rationale:

- `pinocchio code unix hello` is repository-loaded
- many downstream command paths inherit the same helper logic
- fixing the shared path gives immediate leverage

### Decision 3: `pinocchio.engines.fromDefaults(...)` should remain base-config-centric unless renamed

The current `fromDefaults` name strongly implies:

- "start from Pinocchio base config"
- "apply explicit JS overrides"

It should not silently become engine-profile aware under the same name unless that behavior is clearly intentional and documented.

Preferred outcome:

- keep `fromDefaults(...)` for base config + overrides
- add a separate profile-aware path if needed, such as:

```javascript
pinocchio.engines.fromResolvedProfile({ profile: "gpt-5-mini" })
```

or expose final selected settings through a separate inspection/build helper.

### Decision 4: Default `~/.config/pinocchio/profiles.yaml` stays, but the format changes

The default discovery behavior is acceptable for the CLI if the file clearly means "engine profiles".

Rationale:

- the user explicitly wants config plus default `~/.config/pinocchio/profiles.yaml`
- this is a Pinocchio bootstrap convenience, not a Geppetto core concern
- the file needs reformatting because the current mixed format is no longer valid

## Concrete Breakages To Fix First

### A. `ResolveInferenceSettings(...)` returns the wrong settings

Current bug:

```go
resolved, err := chain.ResolveEngineProfile(ctx, in)
...
return base.Clone(), resolved, func() { _ = chain.Close() }, nil
```

This drops the resolved engine profile on the floor.

### B. JS fixture format is obsolete

Current example fixture:

```yaml
slug: workspace
profiles:
  assistant:
    runtime:
      system_prompt: ...
```

That is invalid for the new engine-only model. The migration needs either:

- a rewritten fixture, or
- a conversion script, or both

### C. JS example teaches the wrong architecture

Current example does:

```javascript
const runtime = gp.runner.resolveRuntime({});
const engine = pinocchio.engines.fromDefaults({ model: "gpt-4o-mini", apiType: "openai" });
```

This makes it look as if the selected profile and the actual engine are linked when they are not.

## Alternatives Considered

### Alternative 1: Reintroduce mixed runtime profiles inside Pinocchio only

Rejected.

This would recreate the conceptual overlap that Geppetto just removed and would leave Pinocchio with a misleading downstream abstraction.

### Alternative 2: Patch only `pinocchio js` first

Rejected.

The user's first concern includes `pinocchio code unix hello`, which rides the shared command bootstrap. Fixing only JS would leave the main CLI behavior inconsistent.

### Alternative 3: Make `fromDefaults(...)` implicitly consult profile registries

Rejected for now.

That would blur the meaning of "defaults" and make the engine-building surface harder to reason about. If Pinocchio needs a profile-aware engine helper, it should get a separate explicit API.

## Implementation Plan

### Phase 1: Shared helper cut

Files:

- [`pkg/cmds/helpers/profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go)
- [`cmd/agents/simple-chat-agent/main.go`](../../../../../../pinocchio/cmd/agents/simple-chat-agent/main.go)
- [`cmd/examples/internal/tuidemo/profile.go`](../../../../../../pinocchio/cmd/examples/internal/tuidemo/profile.go)

Work:

- replace the current misleading helper with a final-settings resolver
- merge resolved engine profile settings into base settings
- return resolved profile metadata only as inspection/provenance, not as a runtime side channel

Pseudocode:

```go
func ResolveFinalInferenceSettings(ctx context.Context, parsed *values.Values) (*ResolvedEngineSettings, error) {
    base := ResolveBaseInferenceSettings(parsed)
    profileSelection := ResolveEngineProfileSettings(parsed)

    if profileSelection.ProfileRegistries == "" {
        return &ResolvedEngineSettings{InferenceSettings: base}, nil
    }

    resolved := resolveEngineProfile(profileSelection)
    final := base.Clone()
    final.UpdateFromInferenceSettings(resolved.InferenceSettings)

    return &ResolvedEngineSettings{
        InferenceSettings: final,
        ResolvedProfile: resolved,
        Close: closeFn,
    }, nil
}
```

### Phase 2: Repository-loaded command validation

Files:

- [`cmd/pinocchio/main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go)
- [`pkg/cmds/cobra.go`](../../../../../../pinocchio/pkg/cmds/cobra.go)
- any command middleware or loader path reached by repository YAML commands

Work:

- trace the loaded command path from CLI parse to engine construction
- ensure selected engine profile settings flow through that path
- add a smoke validation for the `code`-style command flow

### Phase 3: JS command migration

Files:

- [`cmd/pinocchio/cmds/js.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/js.go)
- [`pkg/js/modules/pinocchio/module.go`](../../../../../../pinocchio/pkg/js/modules/pinocchio/module.go)
- Pinocchio JS example scripts

Work:

- stop relying on Geppetto runtime resolution to represent engine choice
- make engine-profile-aware engine creation explicit
- keep inspection output so users can see:
  - selected engine profile slug
  - final model/provider/base URL/API key presence

### Phase 4: Profile file migration

Files:

- `~/.config/pinocchio/profiles.yaml` migration path
- [`examples/js/profiles/basic.yaml`](../../../../../../pinocchio/examples/js/profiles/basic.yaml)
- a new conversion script under the ticket or repo scripts if needed

Work:

- define the new engine-profile-only YAML shape
- rewrite checked-in fixtures
- add a conversion script if users are likely to have old mixed files

### Phase 5: Docs and examples

Files:

- [`README.md`](../../../../../../pinocchio/README.md)
- [`examples/js/README.md`](../../../../../../pinocchio/examples/js/README.md)
- [`cmd/pinocchio/doc/general/05-js-runner-scripts.md`](../../../../../../pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md)

Work:

- teach "config + default profiles.yaml + final engine settings"
- provide one real inference JS example
- make the debug/inspection story explicit

## File Reference Map

### Pinocchio files that matter first

- [`profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go)
- [`js.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/js.go)
- [`module.go`](../../../../../../pinocchio/pkg/js/modules/pinocchio/module.go)
- [`main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go)
- [`runner-profile-demo.js`](../../../../../../pinocchio/examples/js/runner-profile-demo.js)
- [`basic.yaml`](../../../../../../pinocchio/examples/js/profiles/basic.yaml)

### Geppetto files that define the new contract

- [`engineprofiles/types.go`](../../../../../../geppetto/pkg/engineprofiles/types.go)
- [`engineprofiles/registry.go`](../../../../../../geppetto/pkg/engineprofiles/registry.go)
- [`settings` package](../../../../../../geppetto/pkg/steps/ai/settings)

## Open Questions

1. Should Pinocchio add a new explicit JS helper for profile-aware engine creation, or should JS scripts call into a host-provided resolver and then use `fromDefaults(...)` only for explicit overrides?
2. Where should a `profiles.yaml` conversion script live long-term: repo `scripts/`, ticket `scripts/`, or a first-class Pinocchio migration command?
3. Once CLI and JS are migrated, should web-chat keep a separate app profile format rather than reusing engine profile registries directly?

## References

- Ticket index: [index.md](../index.md)
- Diary: [01-manuel-investigation-diary.md](../reference/01-manuel-investigation-diary.md)
- Task board: [tasks.md](../tasks.md)

## Proposed Solution

<!-- Describe the proposed solution in detail -->

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
