---
Title: 'Shared assessment: centralize profile registry discovery and loading in Geppetto bootstrap'
Ticket: PIN-20260418-PROFILE-ENV-RESOLUTION
Status: active
Topics:
    - pinocchio
    - profiles
    - cli
    - bootstrap
    - configuration
    - runtime
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/cli/bootstrap/bootstrap_test.go
      Note: Current bootstrap test asserts no implicit fallback and captures the semantic conflict
    - Path: geppetto/pkg/cli/bootstrap/config.go
      Note: Defines AppBootstrapConfig and is the natural place to add a default registry source hook
    - Path: geppetto/pkg/cli/bootstrap/profile_registry.go
      Note: Central validation and chain-loading helper that should remain the single shared guard
    - Path: geppetto/pkg/cli/bootstrap/profile_selection.go
      Note: Shared profile-selection resolution path that should inject implicit registry sources
    - Path: glazed/pkg/cli/cobra-parser.go
      Note: Generic parser/config-plan integration layer; infrastructure
    - Path: glazed/pkg/config/plan.go
      Note: Declarative layered config discovery that underpins the ecosystem-wide config override pattern
    - Path: pinocchio/cmd/pinocchio/cmds/js.go
      Note: Second example of duplicated registry validation/loading in a command
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Example of duplicated registry validation/loading in a command
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-18T14:15:00-04:00
WhatFor: ""
WhenToUse: ""
---


# Shared assessment: centralize profile registry discovery and loading in Geppetto bootstrap

## Executive Summary

I do **not** think the long-term fix should live only in Pinocchio.

The shared responsibility boundary in the current code already says:

- **Glazed** owns generic Cobra/config/env/default parsing infrastructure and config-plan discovery.
- **Geppetto** owns profile selection, registry-chain loading, and final inference-settings resolution.
- **Pinocchio** should mostly provide app identity and app-specific config mapping (`AppName`, env prefix, config mapper, command/runtime wiring).

The strongest evidence is that Geppetto previously had a **shared** default-XDG `profiles.yaml` fallback in its section/bootstrap path, and current bootstrap tests now explicitly assert the opposite. That suggests the current behavior is not a clean intentional design so much as a regression or semantic loss during the bootstrap extraction/refactor.

My recommendation is:

1. **Fix the immediate bug in Geppetto bootstrap**, not in Pinocchio-only wrappers.
2. **Remove duplicated registry validation and registry loading from commands** by adding a higher-level shared helper in Geppetto bootstrap.
3. **Keep Glazed unchanged for the immediate bug**, but explicitly recognize that its config-plan API is the right foundation for any future generalized “secondary resource discovery” abstraction.
4. **Refactor Pinocchio helper paths** so they consume the shared Geppetto bootstrap result instead of manually reconstructing profile/registry behavior.

In short:

```text
generic config discovery mechanics -> Glazed
profile/registry semantics -> Geppetto
app identity + app-specific config mapping -> Pinocchio
```

## Problem Statement

The current behavior has two separate problems:

### 1. The explicit-profile bug

When the user sets `--profile` or `PINOCCHIO_PROFILE`, the command can fail with:

```text
validation error (profile-settings.profile-registries): must be configured when profile-settings.profile is set
```

This happens because shared Geppetto bootstrap currently resolves `profile` and `profile-registries`, but does **not** inject the previously documented/default-discovered registry source before strict validation happens.

Relevant code:

- `geppetto/pkg/cli/bootstrap/profile_selection.go:55-90`
- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-56`

### 2. Duplication across callers

Even worse, multiple Pinocchio callers now duplicate parts of the same contract:

- `pinocchio/cmd/web-chat/main.go:121-170`
- `pinocchio/cmd/pinocchio/cmds/js.go:238-310`
- `pinocchio/pkg/cmds/helpers/parse-helpers.go:56-128`

That duplication means even if one path gets fixed, others can drift.

## What the Codebase Already Says About Ownership

### Glazed’s role

Glazed owns the generic parsing and config-plan building machinery.

Evidence:

- `glazed/pkg/cli/cobra-parser.go:91-103` defines `ConfigPlanBuilder` and makes it part of the generic parser config.
- `glazed/pkg/cli/cobra-parser.go:142-185` wires Cobra flags, args, env, config-plan files, and defaults into a generic middleware chain.
- `glazed/pkg/config/plan.go:11-220` defines a general layered config resolution system (`LayerSystem`, `LayerUser`, `LayerRepo`, `LayerCWD`, `LayerExplicit`).
- `glazed/pkg/config/plan_sources.go:18-149` provides reusable app config discovery helpers (`SystemAppConfig`, `XDGAppConfig`, `HomeAppConfig`, `ExplicitFile`, `WorkingDirFile`, `GitRootFile`).

This is **generic infrastructure**, not profile semantics.

### Geppetto’s role

Geppetto owns profile selection and registry-chain loading.

Evidence:

- `geppetto/pkg/cli/bootstrap/config.go:17-24` defines `AppBootstrapConfig`.
- `geppetto/pkg/cli/bootstrap/profile_selection.go:55-90` resolves profile selection from config/env/defaults plus explicit values.
- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-56` validates and loads the registry chain.
- `geppetto/pkg/cli/bootstrap/engine_settings.go:83-132` uses that profile selection and registry chain to build final merged inference settings.

So Geppetto is already the shared layer that should answer:

- Which profile is selected?
- Which registry sources are in play?
- How are those sources loaded?
- How do we resolve a profile to final engine settings?

### Pinocchio’s role

Pinocchio provides app-local identity and config shape, but should not have to own the profile algorithm itself.

Evidence:

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` wraps `AppBootstrapConfig` with:
  - `AppName: "pinocchio"`
  - `EnvPrefix: "PINOCCHIO"`
  - `ConfigFileMapper: configFileMapper`
- the same wrapper delegates the real resolution work back to Geppetto bootstrap.

That is the correct architectural shape. The problem is that not all behavior was carried over into the extracted shared bootstrap path.

## Evidence That This Was Already a Shared Geppetto Behavior

This is the most important evidence in the whole assessment.

### Prior shared implementation existed

Commit:

- `c6ec017` — `profiles: default to XDG profiles.yaml and refresh docs`

That commit added shared default-XDG fallback logic in Geppetto’s section layer. The diff shows:

- a helper `defaultPinocchioProfileRegistriesIfPresent()` in `geppetto/pkg/sections/sections.go`
- logic that inserted the XDG `profiles.yaml` path when `profile-registries` was empty
- regression test coverage proving `PINOCCHIO_PROFILE=gpt-5` worked without explicit `--profile-registries`

This is supported by historical diary evidence in:

- `geppetto/ttmp/2026/02/24/GP-21-PROFILE-MW-REGISTRY-JS--port-profile-registry-schema-middleware-schema-support-to-js-bindings/reference/01-investigation-diary.md:780-782`

That diary explicitly states:

- when `profile-registries` is empty, middleware auto-uses `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` if the file exists
- a test verified `PINOCCHIO_PROFILE=gpt-5` resolves from default XDG `profiles.yaml` without passing `--profile-registries`

### Current code contradicts that older shared behavior

Current Geppetto bootstrap test:

- `geppetto/pkg/cli/bootstrap/bootstrap_test.go:126-158`

This test is named:

- `TestResolveCLIProfileSelection_DoesNotUseImplicitProfilesFallback`

and it explicitly asserts that no implicit fallback should happen.

That is a major semantic reversal from the earlier shared behavior and from Pinocchio’s current docs.

## Likely Regression Story

The likely sequence looks like this:

```text
old section-based Geppetto profile wiring
  -> had shared default XDG profiles.yaml fallback
  -> validated/loaded in one place

bootstrap extraction + config-plan refactor
  -> moved shared logic into geppetto/pkg/cli/bootstrap
  -> shared fallback behavior was not carried forward
  -> callers started duplicating validation/loading again
  -> tests were rewritten around the new stricter behavior
```

Two commits are especially suspicious in the recent regression window:

- `63d56ad` — `bootstrap: share config and registry helpers`
- `095f056` — `bootstrap: drop path list config wrappers`

I am not claiming those commits are “wrong” overall. I am saying they are the right place to inspect because they represent the extraction and centralization work where the old implicit fallback semantics appear to have been lost.

## Why the Fix Should Be Geppetto-First

### Reason 1: the behavior is shared by definition

Profile selection and registry loading are already shared responsibilities in Geppetto bootstrap. If Pinocchio reimplements them locally, that duplicates the same semantics that Geppetto is explicitly meant to own.

### Reason 2: current duplication is already causing drift

We currently have at least three extra registry-related logic sites in Pinocchio:

1. `cmd/web-chat/main.go` validates empty registries and manually loads chains.
2. `cmd/pinocchio/cmds/js.go` validates empty registries and manually loads chains.
3. `pkg/cmds/helpers/parse-helpers.go` manually reads `PINOCCHIO_PROFILE` and errors before using the shared bootstrap contract.

That is exactly the sort of duplicated “config + profile + registry” work the user wants to avoid.

### Reason 3: repository-loaded commands inherit the shared helper path

Pinocchio loads many commands dynamically from repositories. If the shared helper path is wrong, every repository-loaded command inherits the wrong behavior. A Pinocchio-only band-aid fixes symptoms for some callers while leaving the platform contract inconsistent.

### Reason 4: the previous shared behavior already existed in Geppetto

This is not a speculative “let’s move it there because it sounds cleaner.” The codebase already did it there once.

## Where Glazed Is Implicated, and Where It Is Not

### Glazed is implicated in the infrastructure sense

Glazed owns the generic layered discovery primitives and Cobra parser integration. That matters because:

- the config override + repositories pattern is now expressed as a declarative plan
- app code can describe repo/cwd/home/xdg/system precedence cleanly
- Geppetto bootstrap already consumes this API through `ConfigPlanBuilder`

If we ever want a truly generic abstraction for “discover a secondary file stack next to app config,” Glazed’s `Plan`/`SourceSpec` model is the obvious foundation.

### Glazed is probably not the right place for the immediate fix

Profiles and profile registries are not generic CLI concerns. They are AI/runtime concerns that belong to Geppetto.

Glazed should not need to understand:

- engine profile slugs
- registry chains
- `profile-settings.profile-registries`
- engineprofile-specific validation

So for the immediate bug:

- **Glazed provides the generic discovery machinery**
- **Geppetto should provide the profile-registry policy**

### One caveat: a future generic “resource plan” could be useful

If the ecosystem keeps needing “discover one app config file, then from that derive another optional app-owned resource stack,” there may be value in extracting a generic Glazed abstraction like:

```go
type SourcePlan = config.Plan
```

or a small wrapper that is not hardcoded to `config.yaml` semantics.

But that is future cleanup, not the right first move for this ticket.

## Proposed Target Architecture

### Principle

Commands should not do any of the following themselves:

- validate whether profile registries are required
- discover default registry files
- build registry chains
- convert selected profile strings into `ResolveInput`

Instead, commands should ask Geppetto bootstrap for an already-resolved profile runtime input.

### Proposed shared layers

```text
Glazed
  - generic Cobra/env/config/default parsing
  - generic config plan / source discovery

Geppetto bootstrap
  - resolve profile selection
  - discover default registry sources when app policy provides them
  - validate registry requirements
  - open chained registry
  - return selection + chain + final inference settings

Pinocchio
  - app name/env prefix/config mapper
  - command-specific runtime behavior
  - no duplicate profile-registry validation/loading
```

## Concrete Geppetto Changes I Recommend

## 1. Extend `AppBootstrapConfig` with app-owned registry discovery hooks

Current `AppBootstrapConfig` has:

- `AppName`
- `EnvPrefix`
- `ConfigFileMapper`
- `NewProfileSection`
- `BuildBaseSections`
- `ConfigPlanBuilder`

Relevant file:

- `geppetto/pkg/cli/bootstrap/config.go:17-24`

I recommend adding one of these two shapes.

### Option A: simple and pragmatic

```go
type DefaultProfileRegistrySources func() ([]string, error)
```

And then:

```go
type AppBootstrapConfig struct {
    ...
    DefaultProfileRegistrySources DefaultProfileRegistrySources
}
```

This is enough for the immediate fix.

### Option B: more future-proof

```go
type ProfileRegistrySourceBuilder func(parsed *values.Values) ([]string, error)
```

This lets the app derive default/implicit registry sources from resolved command context if needed.

For this bug, Option A is probably sufficient.

## 2. Make `ResolveCLIProfileSelection` inject shared fallback sources

Today it resolves env/config/defaults into a profile section and merges explicit parsed values.

Relevant file:

- `geppetto/pkg/cli/bootstrap/profile_selection.go:55-90`

Recommended behavior:

1. resolve `profile` + `profile-registries` from config/env/defaults
2. merge explicit parsed values
3. if `profile-registries` is still empty, ask `cfg.DefaultProfileRegistrySources`
4. normalize and apply those implicit sources only if they exist
5. return the final resolved selection

This restores shared behavior while keeping the app-specific default-path policy configurable.

## 3. Add a higher-level helper so commands stop loading registries themselves

Current duplication exists because commands only get part of the story back.

Recommended helper:

```go
type ResolvedCLIProfileRuntime struct {
    Selection            *ResolvedCLIProfileSelection
    RegistryChain        *ResolvedProfileRegistryChain
    ConfigFiles          []string
}

func ResolveCLIProfileRuntime(ctx context.Context, cfg AppBootstrapConfig, parsed *values.Values) (*ResolvedCLIProfileRuntime, error)
```

This helper should:

1. call `ResolveCLIProfileSelection`
2. call `ResolveProfileRegistryChain`
3. return one object with selection + chain + close function

Then:

- `ResolveCLIEngineSettings(...)` can reuse it internally
- JS runtime bootstrap can reuse it
- web-chat can reuse it
- commands no longer hand-roll validation and chain loading

## 4. Keep strict validation in one place only

`ResolveProfileRegistryChain` should remain the single shared place that says:

- if a profile is selected and there are still no registry sources, error

Relevant file:

- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-56`

But command code should stop duplicating that logic.

## What Pinocchio Should Do After the Geppetto Fix

### Pinocchio wrapper responsibilities

Pinocchio should keep only:

- `AppName: "pinocchio"`
- `EnvPrefix: "PINOCCHIO"`
- `ConfigFileMapper` that strips app-local keys like `repositories`
- app-specific default-registry discovery hook passed into Geppetto bootstrap

### Pinocchio command responsibilities

Pinocchio commands should:

- call Geppetto bootstrap
- consume returned selection/chain/settings
- stop validating empty registries themselves
- stop opening registry chains themselves where possible

### Pinocchio helper cleanup

`pkg/cmds/helpers/parse-helpers.go:56-128` is the most obvious cleanup target. It currently re-reads env vars and independently errors on missing `profile-registries`.

That should be rewritten to reuse the shared bootstrap path or be retired.

## What I Would *Not* Do

### I would not put profile-registry semantics into Glazed

That would mix application-domain runtime semantics into generic CLI parsing infrastructure.

### I would not leave the fix in Pinocchio only

That would make the wrapper larger, keep behavior duplicated, and preserve the risk that another Geppetto consumer reintroduces the same bug.

### I would not let command surfaces own validation/loading

That defeats the purpose of extracting shared bootstrap in the first place.

## Pseudocode for the Recommended Shape

### Geppetto bootstrap config

```go
type AppBootstrapConfig struct {
    AppName                      string
    EnvPrefix                    string
    ConfigFileMapper             sources.ConfigFileMapper
    NewProfileSection            func() (schema.Section, error)
    BuildBaseSections            func() ([]schema.Section, error)
    ConfigPlanBuilder            ConfigPlanBuilder
    DefaultProfileRegistrySources func() ([]string, error)
}
```

### Shared selection resolver

```go
func ResolveCLIProfileSelection(cfg AppBootstrapConfig, parsed *values.Values) (*ResolvedCLIProfileSelection, error) {
    resolved := parseProfileSettingsFromConfigEnvDefaults(cfg, parsed)
    resolved = mergeExplicitValues(resolved, parsed)

    if len(resolved.ProfileRegistries) == 0 && cfg.DefaultProfileRegistrySources != nil {
        defaults, err := cfg.DefaultProfileRegistrySources()
        if err != nil {
            return nil, err
        }
        if len(defaults) > 0 {
            resolved.ProfileRegistries = normalizeProfileRegistries(defaults)
        }
    }

    return resolved, nil
}
```

### Shared runtime helper

```go
func ResolveCLIProfileRuntime(ctx context.Context, cfg AppBootstrapConfig, parsed *values.Values) (*ResolvedCLIProfileRuntime, error) {
    selection, err := ResolveCLIProfileSelection(cfg, parsed)
    if err != nil {
        return nil, err
    }

    chain, err := ResolveProfileRegistryChain(ctx, selection.ProfileSettings)
    if err != nil {
        return nil, err
    }

    return &ResolvedCLIProfileRuntime{
        Selection: selection,
        RegistryChain: chain,
        ConfigFiles: append([]string(nil), selection.ConfigFiles...),
    }, nil
}
```

## Implementation Plan

### Phase 1: Restore shared fallback behavior in Geppetto bootstrap

Files:

- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/bootstrap_test.go`

Work:

1. add an app-configurable default registry source hook
2. restore fallback behavior through shared bootstrap
3. replace the current “does not use implicit fallback” test with tests that prove the intended shared behavior

### Phase 2: Add a higher-level shared registry-runtime helper

Files:

- `geppetto/pkg/cli/bootstrap/profile_registry.go`
- possibly a new helper file in `geppetto/pkg/cli/bootstrap`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`

Work:

1. return selection + registry chain in one shared helper
2. make engine-settings resolution consume that helper

### Phase 3: Remove Pinocchio duplication

Files:

- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/cmd/pinocchio/cmds/js.go`
- `pinocchio/pkg/cmds/helpers/parse-helpers.go`

Work:

1. stop per-command registry validation
2. stop per-command chain loading where possible
3. route all callers through Geppetto bootstrap

### Phase 4: Reconcile docs across repos

Files:

- Pinocchio README and docs
- Geppetto bootstrap/profile docs
- any tests or migration guides that still describe the strict “no implicit fallback” model

## Risks and Tradeoffs

### Risk: app hooks make Geppetto bootstrap too app-aware

Mitigation:

- keep the hook generic (`DefaultProfileRegistrySources`), not Pinocchio-specific
- Geppetto should not hardcode `pinocchio`; the app passes the policy in

### Risk: we over-generalize too early into Glazed

Mitigation:

- use Glazed’s existing config plan as infrastructure only
- do not move engine-profile semantics into Glazed

### Risk: helper migration touches many callers

Mitigation:

- add the new higher-level Geppetto helper first
- migrate callers incrementally
- keep the old lower-level helper temporarily if needed

## Final Recommendation

My recommendation is:

1. **Restore the implicit/default registry-source behavior in Geppetto bootstrap** through an app-configurable hook.
2. **Introduce a Geppetto helper that returns resolved profile selection + registry chain together**.
3. **Delete command-local registry validation/loading in Pinocchio** once the shared helper exists.
4. **Do not change Glazed for the immediate bug**, but note that Glazed’s declarative config-plan API is the right substrate if we later want a reusable “secondary resource discovery” pattern across the ecosystem.

This gets us to the design the user described:

- Pinocchio only configures app identity and app-specific config mapping.
- Geppetto owns the shared config + profile + profile-registry contract.
- Glazed stays the generic layered parsing/discovery engine beneath both.

## References

### Current shared code

- `geppetto/pkg/cli/bootstrap/config.go:17-24`
- `geppetto/pkg/cli/bootstrap/profile_selection.go:55-90`
- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-56`
- `geppetto/pkg/cli/bootstrap/engine_settings.go:83-132`
- `glazed/pkg/cli/cobra-parser.go:91-185`
- `glazed/pkg/config/plan.go:11-220`
- `glazed/pkg/config/plan_sources.go:18-149`

### Current duplication in Pinocchio

- `pinocchio/cmd/web-chat/main.go:121-170`
- `pinocchio/cmd/pinocchio/cmds/js.go:238-310`
- `pinocchio/pkg/cmds/helpers/parse-helpers.go:56-128`

### Regression evidence

- historical commit: `c6ec017` — `profiles: default to XDG profiles.yaml and refresh docs`
- historical diary evidence: `geppetto/ttmp/2026/02/24/GP-21-PROFILE-MW-REGISTRY-JS--port-profile-registry-schema-middleware-schema-support-to-js-bindings/reference/01-investigation-diary.md:780-782`
- current contradictory test: `geppetto/pkg/cli/bootstrap/bootstrap_test.go:126-158`
- likely extraction/refactor window:
  - `63d56ad` — `bootstrap: share config and registry helpers`
  - `095f056` — `bootstrap: drop path list config wrappers`
