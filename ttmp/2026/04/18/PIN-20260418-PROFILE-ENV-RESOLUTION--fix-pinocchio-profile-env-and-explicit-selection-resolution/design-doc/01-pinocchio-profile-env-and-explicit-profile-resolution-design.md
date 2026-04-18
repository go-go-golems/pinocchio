---
Title: Pinocchio profile env and explicit profile resolution design
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
    - Path: geppetto/pkg/cli/bootstrap/profile_registry.go
      Note: Shows the strict registry validation that currently triggers the bug
    - Path: geppetto/pkg/sections/profile_sections.go
      Note: Provides WithProfileRegistriesDefault for default registry discovery
    - Path: pinocchio/cmd/pinocchio/cmds/js.go
      Note: Secondary command path that resolves profile settings and registries
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Command-level profile guard that should stop failing once default registry discovery is wired in
    - Path: pinocchio/pkg/cmds/helpers/parse-helpers.go
      Note: Example/helper path that reads env profile selection separately
    - Path: pinocchio/pkg/cmds/profilebootstrap/profile_selection.go
      Note: Defines the shared Pinocchio app bootstrap config and profile-selection wrapper
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-18T13:28:04.052111967-04:00
WhatFor: ""
WhenToUse: ""
---


# Pinocchio profile env and explicit profile resolution design

## Executive Summary

Pinocchio currently treats explicit profile selection as requiring an already-populated `profile-settings.profile-registries` list. That means `--profile ...` and `PINOCCHIO_PROFILE=...` can fail early with a validation error even when the user expects Pinocchio to discover the default registry file automatically.

The failure is visible in the reported commands:

```bash
pinocchio --profile gemini-2.5-pro code professional hello
# Error: resolve engine profile settings for command run: validation error (profile-settings.profile-registries): must be configured when profile-settings.profile is set

PINOCCHIO_PROFILE=gemini-2.5-pro pinocchio code professional hello
# same validation error
```

By contrast, the no-profile path can continue far enough to create inference settings and reach the model provider, which is why the user also sees a later 401 from OpenAI when no profile is selected. That 401 is a separate runtime-credentials issue, not the profile-resolution bug.

The fix should be Pinocchio-local and should make the shared `profilebootstrap` path discover the default `profiles.yaml` registry file when it exists. That keeps explicit profile selection working without requiring every user to pass `--profile-registries` manually, while preserving the current validation error when a profile is requested and no registry source exists anywhere.

## Problem Statement and Scope

### What is broken

Pinocchio has a profile-selection contract documented in the README and tutorial pages:

- `--profile` should win over environment variables and config
- `PINOCCHIO_PROFILE` should work
- the default `profiles.yaml` registry should be discovered when present
- default profile selection should continue to work when no explicit profile is set

Relevant documentation says the registry stack should include the default XDG path when present:

- `README.md:84-108`
- `cmd/pinocchio/doc/general/05-js-runner-scripts.md:180-216`
- `pkg/doc/topics/webchat-profile-registry.md:70-123`

The current implementation does not consistently supply that fallback registry source. The core shared bootstrap code rejects profile selection when `profile-registries` is empty:

- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-42`
- `cmd/web-chat/main.go:121-135`
- `cmd/pinocchio/cmds/js.go:238-270`

### What is not broken

The engine-resolution path itself is not the root cause of the early failure. The following pieces already do their jobs correctly:

- hidden base inference settings are resolved from config/env/defaults
- explicit profile selection is merged onto the base once registries exist
- runtime profile switching preserves a base and reapplies overlays

Useful reference files:

- `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go:15-46`
- `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go:17-95`
- `geppetto/pkg/cli/bootstrap/engine_settings.go:26-154`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`

### Scope

This ticket should cover:

1. how Pinocchio discovers its default engine-profile registry file
2. how that discovery is injected into the shared profile bootstrap path
3. how helper paths such as `ParseGeppettoLayers()` stay aligned
4. regression tests for env/flag/default behavior
5. documentation updates where the current docs and implementation disagree

It should not redesign the engine/profile model itself.

## Current-State Architecture

This is the system the intern needs to understand before touching the bug.

### 1) Glazed parses layered values into sections

Pinocchio commands are built on Glazed sections and values. Command parsing produces a `values.Values` object with per-field provenance. Different sources include:

- CLI flags
- environment variables
- config files
- defaults
- profile middleware

That provenance matters because the profile/bootstrap layer uses it to decide what came from config versus what came from profiles.

Relevant files:

- `pkg/cmds/cmd.go:236-274`
- `pkg/cmds/helpers/parse-helpers.go:54-128`
- `cmd/examples/simple-chat/main.go:85-94`

### 2) Pinocchio wraps Geppetto bootstrap with app-specific config

The Pinocchio wrapper in `pkg/cmds/profilebootstrap` gives Geppetto an app name, env prefix, and config mapper:

- `AppName: "pinocchio"`
- `EnvPrefix: "PINOCCHIO"`
- `ConfigFileMapper: configFileMapper`
- `NewProfileSection: geppettosections.NewProfileSettingsSection`
- `BuildBaseSections: geppettosections.CreateGeppettoSections`

Relevant file:

- `pkg/cmds/profilebootstrap/profile_selection.go:18-90`

This wrapper is the right place for Pinocchio-specific profile behavior because it is shared by CLI verbs, JS execution, and examples.

### 3) Profile selection is resolved before registry loading

The shared Geppetto bootstrap path resolves profile settings first and registry loading second:

```text
ResolveCLIProfileSelection
  -> read env/config/defaults
  -> merge explicit parsed values
  -> return Profile + ProfileRegistries

ResolveCLIEngineSettingsFromBase
  -> ResolveCLIProfileSelection
  -> ResolveProfileRegistryChain
  -> ResolveEngineProfile
  -> MergeInferenceSettings(base, profile)
```

Relevant files:

- `geppetto/pkg/cli/bootstrap/profile_selection.go:55-90`
- `geppetto/pkg/cli/bootstrap/engine_settings.go:69-132`
- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-42`

### 4) Registry loading is strict about empty registries when a profile is requested

`ResolveProfileRegistryChain` currently does this:

- if `profile-registries` is empty and `profile` is set, return a validation error
- otherwise parse the registry sources and build a chained registry

That is the exact error the user sees.

Relevant file:

- `geppetto/pkg/cli/bootstrap/profile_registry.go:17-42`

### 5) The runtime layer is not the issue

Once `ResolvedCLIEngineSettings` exists, the runtime uses `FinalInferenceSettings` to build the engine. That path is already separated from profile selection.

Relevant files:

- `geppetto/pkg/cli/bootstrap/engine_settings.go:97-154`
- `pkg/cmds/helpers/profile_runtime.go:11-32`
- `pkg/cmds/profilebootstrap/engine_settings.go:15-46`

### 6) There are multiple consumers of the same contract

The same profile-selection logic is used by:

- top-level Pinocchio commands: `pkg/cmds/cmd.go`
- web chat: `cmd/web-chat/main.go`
- JS runner: `cmd/pinocchio/cmds/js.go`
- example helpers: `pkg/cmds/helpers/parse-helpers.go`

That means a fix in only one caller would leave the others inconsistent.

## Gap Analysis

### Gap 1: No app-local default registry discovery is injected into profile selection

The docs say Pinocchio should discover `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` when present, but the current bootstrap path does not consistently add that source before the strict validation in `ResolveProfileRegistryChain`.

This is why `PINOCCHIO_PROFILE=...` can fail even though the registry file exists on disk.

### Gap 2: Some commands duplicate the same error check

`cmd/web-chat/main.go` and `cmd/pinocchio/cmds/js.go` both repeat the validation message after calling `ResolveCLIProfileSelection`.

That is acceptable as a guard, but it is not a substitute for the shared discovery contract. If the shared helper is wrong, every caller must rediscover the same rule manually.

### Gap 3: The helper path `ParseGeppettoLayers()` reads env directly and revalidates manually

`pkg/cmds/helpers/parse-helpers.go` pulls `PINOCCHIO_PROFILE` and `PINOCCHIO_PROFILE_REGISTRIES` directly from the process environment and errors out before the rest of the middleware chain runs if the profile is set and registries are empty.

That makes the helper a second, independent source of truth. If it is not updated, example code will keep failing even after the main CLI path is fixed.

### Gap 4: Documentation and tests disagree about default registry fallback

Current Pinocchio docs describe default registry discovery, but tests in the current codebase assert that no implicit registry fallback occurs.

This mismatch is a strong signal that the implementation has drifted from the intended contract and that the ticket should include test updates plus doc updates.

## Proposed Solution

### Decision summary

Make default profile-registry discovery a Pinocchio-owned bootstrap responsibility, not a per-command concern.

The implementation should:

1. discover the default `profiles.yaml` path for the app
2. only include it when the file exists
3. inject it into the shared profile-settings section so `ResolveCLIProfileSelection` sees it
4. keep explicit `--profile-registries`, config, and environment overrides ahead of the default
5. update helper paths so they reuse the same resolved selection instead of reconstructing a second contract

### Why this is the right layer

- It keeps the fix local to Pinocchio instead of changing Geppetto behavior for every app
- It matches the docs that already describe Pinocchio-specific default registry discovery
- It avoids duplicating fallback rules in every command
- It keeps `ResolveProfileRegistryChain` strict, which is good: the registry loader should still fail if the user asks for a profile and no registry source exists at all

### Proposed shape of the fix

#### A. Add a Pinocchio default-registry discovery helper

Create a small helper in `pkg/cmds/profilebootstrap` that computes the default profile registry path and only returns it when the file exists.

Pseudo-contract:

```go
func defaultProfileRegistrySources() []string
```

Possible implementation responsibilities:

- compute the XDG profile path for Pinocchio
- check `os.Stat`
- return `[]string{path}` only when the file exists
- otherwise return `nil`

#### B. Use that helper when building the profile section

Update `pinocchioBootstrapConfig()` so its `NewProfileSection` builder supplies the discovered default registry sources through `geppettosections.WithProfileRegistriesDefault(...)` or an equivalent app-local default injection path.

That keeps the default visible to the parser before profile resolution runs.

#### C. Reuse the same selection in helper consumers

Update `ParseGeppettoLayers()` and similar helpers so they stop inventing their own fallback logic.

They should receive or derive profile selection from the shared bootstrap path, then pass that profile selection downstream.

#### D. Keep the strict validation as a final guard

The final validation error is still useful when:

- a profile is requested
- no default registry file exists
- no explicit registry source was configured

In that case, the same error should remain.

## API References

These are the functions and types an intern should recognize while implementing the fix.

### Profile bootstrap and selection

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - `BootstrapConfig()` returns the Pinocchio app config
  - `ResolveCLIProfileSelection()` resolves profile + registry sources
  - `ResolveEngineProfileSettings()` returns the resolved profile settings and config files

- `geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `ResolveCLIProfileSelection(cfg, parsed)` merges env/config/defaults with explicit values
  - `NewCLISelectionValues(cfg, input)` is the test-friendly constructor for layered values

- `geppetto/pkg/cli/bootstrap/profile_registry.go`
  - `ResolveProfileRegistryChain(ctx, selection)` loads registry sources and validates them

### Registry and profile data structures

- `geppetto/pkg/sections/profile_sections.go`
  - `ProfileSettings` has `Profile` and `ProfileRegistries`
  - `WithProfileDefault(...)`
  - `WithProfileRegistriesDefault(...)`
  - `NewProfileSettingsSection(...)`

- `geppetto/pkg/engineprofiles/source_chain.go`
  - `ParseRegistrySourceSpecs(...)`
  - `NewChainedRegistryFromSourceSpecs(...)`
  - `ResolveEngineProfile(...)`

### Runtime resolution

- `geppetto/pkg/cli/bootstrap/engine_settings.go`
  - `ResolveBaseInferenceSettings(...)`
  - `ResolveCLIEngineSettings(...)`
  - `ResolveCLIEngineSettingsFromBase(...)`
  - `ResolvedCLIEngineSettings`

- `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go`
  - `ResolveParsedBaseInferenceSettingsWithBase(...)`
  - strips profile-derived parse steps from the base

### Current consumers

- `pkg/cmds/cmd.go:245-274`
- `cmd/web-chat/main.go:121-170`
- `cmd/pinocchio/cmds/js.go:238-310`
- `pkg/cmds/helpers/parse-helpers.go:54-128`
- `cmd/examples/simple-chat/main.go:87-94`

## Current Flow vs Proposed Flow

### Current flow

```text
user passes --profile or PINOCCHIO_PROFILE
  -> parsed values contain profile
  -> profile registries remain empty unless explicitly set
  -> ResolveProfileRegistryChain rejects the selection
  -> command returns validation error
```

### Proposed flow

```text
user passes --profile or PINOCCHIO_PROFILE
  -> Pinocchio bootstrap discovers existing ~/.config/pinocchio/profiles.yaml
  -> profile-settings includes that registry source by default
  -> ResolveProfileRegistryChain loads the default registry stack
  -> profile resolves
  -> merged FinalInferenceSettings are built
  -> engine runs
```

### Important nuance

The user-facing 401 from OpenAI is a separate path:

```text
no explicit profile
  -> default profile / base settings resolved
  -> engine starts
  -> provider request fails because no API key is configured
```

That confirms the engine-resolution path is alive. It does not prove profile selection is correct.

## Pseudocode

### 1. App-local default registry discovery

```go
func defaultProfileRegistrySources() []string {
    path := profiles.GetProfilesPathForApp("pinocchio")
    if strings.TrimSpace(path) == "" {
        return nil
    }
    if _, err := os.Stat(path); err != nil {
        return nil
    }
    return []string{path}
}
```

### 2. Inject defaults into the profile section

```go
func pinocchioBootstrapConfig() bootstrap.AppBootstrapConfig {
    defaults := defaultProfileRegistrySources()

    return bootstrap.AppBootstrapConfig{
        AppName: "pinocchio",
        EnvPrefix: "PINOCCHIO",
        ConfigFileMapper: configFileMapper,
        NewProfileSection: func() (schema.Section, error) {
            opts := []geppettosections.ProfileSettingsSectionOption{}
            if len(defaults) > 0 {
                opts = append(opts, geppettosections.WithProfileRegistriesDefault(defaults...))
            }
            return geppettosections.NewProfileSettingsSection(opts...)
        },
        BuildBaseSections: func() ([]schema.Section, error) {
            return geppettosections.CreateGeppettoSections()
        },
    }
}
```

### 3. Keep helper consumers aligned

```go
func ParseGeppettoLayers(cmd *cmds.PinocchioCommand, opts ...GeppettoLayersHelperOption) (*values.Values, error) {
    selection, err := profilebootstrap.ResolveCLIProfileSelection(profilebootstrap.BootstrapConfig(), parsedValues)
    if err != nil {
        return nil, err
    }

    // pass selection.Profile and selection.ProfileRegistries forward
}
```

The point is not the exact code shape; the point is that the helper should stop independently inventing a second profile-selection contract.

## Implementation Plan

### Phase 1: Add default registry discovery in the Pinocchio bootstrap wrapper

Files to touch:

- `pkg/cmds/profilebootstrap/profile_selection.go`
- possibly a new helper file in `pkg/cmds/profilebootstrap`

Work:

1. compute the default profiles path for Pinocchio
2. check whether the file exists
3. inject the path into the profile section defaults
4. preserve current explicit overrides

Success criteria:

- `ResolveCLIProfileSelection(values.New())` includes the default registry path when the file exists
- the existing validation error only appears when there is truly no registry source available

### Phase 2: Reuse shared resolution in helper consumers

Files to inspect/update:

- `pkg/cmds/helpers/parse-helpers.go`
- `cmd/examples/simple-chat/main.go`
- any other direct callers that manually pass profile env state around

Work:

1. eliminate direct env parsing where possible
2. use the already resolved profile settings object
3. keep the helper’s middleware chain behavior unchanged apart from the source of profile selection

Success criteria:

- example entrypoints do not diverge from the CLI’s profile semantics

### Phase 3: Tighten the command-level guards

Files to inspect:

- `cmd/web-chat/main.go`
- `cmd/pinocchio/cmds/js.go`
- `pkg/cmds/cmd.go`

Work:

1. keep the current error guard only as a sanity check
2. ensure the guard no longer trips when a valid default registry file exists

Success criteria:

- `--profile` and `PINOCCHIO_PROFILE` work in the top-level CLI surfaces without requiring explicit `--profile-registries`

### Phase 4: Update tests and docs

Work:

1. change the tests that currently assert “no implicit registry fallback”
2. add positive coverage for default registry discovery
3. update docs so they reflect the actual bootstrap behavior

Success criteria:

- the documented and tested behavior match the implementation

## Testing Strategy

The test plan should prove both the happy path and the guardrails.

### Unit tests

Add or update tests for:

1. `ResolveCLIProfileSelection` with a present default registry file
2. `ResolveCLIProfileSelection` with `PINOCCHIO_PROFILE` set and no explicit registries
3. `ResolveCLIProfileSelection` with `--profile`/explicit values winning over env/config
4. `ResolveCLIProfileSelection` with no registry file present and no selected profile
5. `ResolveCLIProfileSelection` with a selected profile and no registry sources at all

### Integration / command tests

Add command-level tests for:

- `pinocchio code professional hello`
- `pinocchio js ... --profile ...`
- `web-chat` startup profile selection
- the example helper path that uses `ParseGeppettoLayers()`

### Validation commands

Useful commands during implementation:

```bash
go test ./pkg/cmds/profilebootstrap -count=1
go test ./pkg/cmds/helpers -count=1
go test ./cmd/pinocchio/cmds -count=1
go test ./cmd/web-chat -count=1
```

For runtime confirmation, run the reported commands again after the fix and verify that:

- profile selection no longer fails early
- the chosen profile is reflected in the resolved engine settings
- a missing API key still produces the expected provider-level 401 when appropriate

## Risks, Alternatives, and Open Questions

### Risk: a global Geppetto change would affect other apps

Do not move this into generic Geppetto bootstrap unless every consumer should inherit the same default registry discovery. The bug is Pinocchio-specific, so the fix should stay in the Pinocchio wrapper.

### Risk: unconditional defaults would break missing-file cases

Do not set a default registry path unless the file exists. Otherwise empty workspaces would start failing during profile resolution for no good reason.

### Alternative: leave the code as-is and change the docs

This is not a good outcome. The current docs already promise behavior that the implementation does not provide, so the right fix is to align the code with the docs, not the other way around.

### Alternative: keep per-command registry loading

That would keep the bug alive in helper paths and create more duplication. The shared bootstrap wrapper is a better single source of truth.

### Open question

Should the default registry discovery helper live in `pkg/cmds/profilebootstrap` or be shared with any other Pinocchio app entrypoint that needs the same semantics?

The safest answer for now is to keep it in `profilebootstrap` and reuse it from the callers that already depend on that package.

## References

### Primary code files

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`
- `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go`
- `pinocchio/pkg/cmds/helpers/parse-helpers.go`
- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/cmd/pinocchio/cmds/js.go`
- `pinocchio/cmd/examples/simple-chat/main.go`

### Shared Geppetto files

- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/profile_registry.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/sections/profile_sections.go`
- `geppetto/pkg/engineprofiles/source_chain.go`

### Documentation

- `pinocchio/README.md:84-108`
- `pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md:180-216`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
- `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`
