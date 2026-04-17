---
Title: ConfigFilesFunc removal analysis and appconfig cleanup plan
Ticket: PI-CONFIGFILESFUNC-REMOVAL
Status: active
Topics:
    - config
    - glazed
    - pinocchio
    - cleanup
    - appconfig
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../../../../code/wesen/corporate-headquarters/prescribe/cmd/prescribe/cmds/generate.go
      Note: The main non-example production appconfig caller found during the cross-repo usage audit
    - Path: ../../../../../../../glazed/cmd/examples/config-overlay/main.go
      Note: Example ConfigFilesFunc caller that should migrate to an explicit declarative config plan
    - Path: ../../../../../../../glazed/cmd/examples/overlay-override/main.go
      Note: Example of parsed-command-driven file composition that should migrate from ConfigFilesFunc to a plan builder
    - Path: ../../../../../../../glazed/pkg/appconfig/options.go
      Note: Shows the current appconfig option surface including WithConfigFiles and profile bootstrap behavior
    - Path: ../../../../../../../glazed/pkg/cli/cobra-parser.go
      Note: Defines ConfigFilesFunc today and contains the legacy implicit config-loading fallback that should be removed
    - Path: cmd/agents/simple-chat-agent/main.go
      Note: Pinocchio caller using the same no-op suppression pattern as web-chat
    - Path: cmd/web-chat/main.go
      Note: Pinocchio production caller currently using a no-op ConfigFilesFunc to suppress implicit CobraParser config loading
    - Path: ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/reference/01-diary.md
      Note: Implementation diary for the ConfigFilesFunc/ConfigPath/appconfig removal work
ExternalSources: []
Summary: Analysis of removing glazed/pkg/cli.CobraParserConfig.ConfigFilesFunc, migrating current callers to declarative config plans, and deciding whether pkg/appconfig should be modernized or removed instead of preserved.
LastUpdated: 2026-04-14T18:25:00-04:00
WhatFor: Plan a focused cleanup that removes the legacy string-list config-files hook from CobraParser, reduces implicit config-loading behavior, and assesses whether pkg/appconfig should be modernized or retired.
WhenToUse: Use when migrating CLI commands away from ConfigFilesFunc/ConfigPath-style config discovery and when deciding whether appconfig is worth preserving as a public compatibility layer.
---



# ConfigFilesFunc removal analysis and appconfig cleanup plan

## Executive summary

`ConfigFilesFunc` in `glazed/pkg/cli/cobra-parser.go` is now the wrong abstraction for the codebase. It returns only `[]string`, so it throws away config-layer and config-source provenance that the new declarative config-plan API already models. It also encourages old single-path or ad hoc ordered-file thinking right next to the new `config.Plan`/`ResolvedConfigFile` design.

The current live call-site surface is small. In this workspace, there are only six `ConfigFilesFunc` call sites total, and four of them are Pinocchio/example wiring rather than broadly shared library consumers. The most important Pinocchio call sites are actually **no-op resolvers** whose only job is to disable CobraParser’s implicit app-config loading while still keeping `AppName` for env prefix behavior. That is a strong signal that the current API shape is actively fighting the intended architecture.

My recommendation is to remove `ConfigFilesFunc` rather than preserve it. The cleanup should be opinionated:

- stop implicit config-file discovery from `AppName`
- make config loading opt-in via a single plan-aware hook
- migrate Pinocchio and examples directly
- avoid adding a new backwards-compatible string-list hook
- do **not** expand `pkg/appconfig` as another compatibility layer unless a second real production consumer appears

## Goals

- Remove `CobraParserConfig.ConfigFilesFunc`
- Move `CobraParser` config loading to an explicit declarative-plan path
- Simplify Pinocchio callers that currently install no-op resolvers just to suppress default config loading
- Inventory `pkg/appconfig` usage across this workspace and `~/code/wesen/corporate-headquarters`
- Decide whether `pkg/appconfig` should be modernized or removed rather than preserved

## Non-goals

- Do not preserve old path-list APIs just for compatibility
- Do not add multiple overlapping replacement hooks if one plan-aware hook is sufficient
- Do not redesign Geppetto bootstrap in this ticket; consume the already-added plan primitives where appropriate

## Current architecture

### Where `ConfigFilesFunc` lives

`ConfigFilesFunc` is currently defined on `glazed/pkg/cli.CobraParserConfig`:

```go
type CobraParserConfig struct {
    MiddlewaresFunc ...
    AppName string
    ConfigPath string
    ConfigFilesFunc func(parsed *values.Values, cmd *cobra.Command, args []string) ([]string, error)
}
```

When `MiddlewaresFunc` is nil, `NewCobraParserFromSections(...)` builds a default middleware chain that currently does all of the following:

1. parse Cobra flags
2. parse args
3. parse env using `strings.ToUpper(AppName)`
4. resolve config files using either:
   - caller-provided `ConfigFilesFunc`, or
   - fallback `ResolveAppConfigPath(AppName, explicit)`
5. load those config files
6. apply defaults

### Why this is now a mismatch

This shape predates the newer config-plan work. It has several architectural problems now:

- `[]string` is too weak: it loses `config_layer`, `config_source_name`, and `config_source_kind`
- it nudges callers toward ad hoc ordered-file logic instead of declarative source specs
- it couples `AppName` to both env parsing **and** implicit config loading
- it encourages silent library-level magic instead of explicit application policy

The Pinocchio no-op callers make the coupling problem especially obvious: they want `AppName` for env handling, but they **do not** want CobraParser to auto-load app config because Pinocchio already owns config policy through profile bootstrap and config plans.

## Current `ConfigFilesFunc` call-site inventory

### In this workspace

#### 1. `glazed/pkg/cli/cobra-parser.go`
- definition site and fallback behavior
- old default path remains `ResolveAppConfigPath(AppName, explicit)`

#### 2. `glazed/cmd/examples/config-overlay/main.go`
- custom fixed ordered file list (`base -> env -> local`)
- demo/example only
- best migration: explicit plan builder with static files in named layers

#### 3. `glazed/cmd/examples/overlay-override/main.go`
- custom resolver based on `command-settings.config-file`
- optionally adds `*.override.yaml`
- demo/example only
- best migration: plan builder using explicit file + optional computed override source

#### 4. `pinocchio/cmd/web-chat/main.go`
- sets `AppName: "pinocchio"`
- installs a no-op `ConfigFilesFunc` returning `nil, nil`
- this is not using the hook for real config loading; it is suppressing implicit CobraParser config behavior
- Pinocchio already resolves base settings/profile selection through `profilebootstrap` and Geppetto bootstrap

#### 5. `pinocchio/cmd/web-chat/main_profile_registries_test.go`
- same pattern as `main.go`
- test-support call site

#### 6. `pinocchio/cmd/examples/simple-chat/main.go`
- same no-op suppression pattern

#### 7. `pinocchio/cmd/agents/simple-chat-agent/main.go`
- same no-op suppression pattern

### Classification

There are really only two kinds of live caller:

1. **Example/demo custom file composition**
   - `config-overlay`
   - `overlay-override`
2. **Pinocchio suppression shims**
   - `web-chat`
   - `web-chat` test
   - `simple-chat`
   - `simple-chat-agent`

That is a very favorable cleanup shape. There is no broad ecosystem dependence visible here.

## Why the Pinocchio no-op callers matter

These callers are the clearest sign that the old API should go away.

Today they must say roughly:

```go
cli.CobraParserConfig{
    AppName: "pinocchio",
    ConfigFilesFunc: func(...) ([]string, error) { return nil, nil },
}
```

They are not asking for custom config resolution. They are asking for:

- env prefix behavior from `AppName`
- **no** implicit config loading in CobraParser

That means the current config API is over-coupled. The clean fix is not another compatibility shim. The clean fix is:

- `AppName` (or later `EnvPrefix`) controls env parsing
- config loading only happens when an explicit plan hook is configured

Once that is true, these Pinocchio call sites can simply delete the no-op resolver.

## Recommended API cleanup

## Decision

Replace `ConfigFilesFunc` with a single explicit plan-aware hook and remove implicit config loading from `AppName`.

### Recommended replacement

Add one hook to `CobraParserConfig`:

```go
type CobraParserConfig struct {
    MiddlewaresFunc ...
    AppName string // retained for env prefix only
    ConfigPlanBuilder func(parsed *values.Values, cmd *cobra.Command, args []string) (*config.Plan, error)
}
```

### Behavioral rules

- If `MiddlewaresFunc` is provided, it still wins as the full escape hatch.
- If `MiddlewaresFunc` is nil and `ConfigPlanBuilder` is nil:
  - CobraParser parses flags, args, env, defaults
  - **no config files are loaded implicitly**
- If `ConfigPlanBuilder` is set:
  - resolve the plan
  - load via `FromResolvedFiles(...)`
  - preserve provenance metadata

### Why choose `ConfigPlanBuilder` instead of another files hook

Because we already have the right abstraction now.

A new `ResolvedConfigFilesFunc` would be better than `ConfigFilesFunc`, but it would still introduce another parallel customization surface. If the goal is simplification rather than compatibility, `ConfigPlanBuilder` is enough.

The plan builder can express:
- fixed ordered files
- repo/cwd/explicit layering
- dynamic sources derived from parsed command settings
- dedupe and stop-if-found behavior
- provenance-rich loading

## Recommended scope for this ticket

To keep the cleanup meaningful rather than half-finished, this ticket should remove the whole old CobraParser path-list config story, not just rename it.

### In scope

- remove `ConfigFilesFunc`
- remove implicit default config discovery tied to `AppName`
- route config loading through `ConfigPlanBuilder`
- migrate current workspace callers
- update docs/examples accordingly

### Strongly consider including in the same change

- remove `ConfigPath` from `CobraParserConfig`
- remove the `ResolveAppConfigPath(...)` fallback from `glazed/pkg/cli/cobra-parser.go`

Why: `ConfigPath` and `ConfigFilesFunc` are part of the same old path-based configuration model. If the parser is being cleaned up aggressively, keeping `ConfigPath` would leave a smaller but still conceptually similar legacy path in place.

## Migration plan

### Phase 1: reshape `glazed/pkg/cli/cobra-parser.go`

1. Add `ConfigPlanBuilder` to `CobraParserConfig`
2. Stop using `AppName` to trigger implicit config discovery
3. Remove `ConfigFilesFunc`
4. Prefer `FromResolvedFiles(...)` after resolving the plan
5. If feasible in the same step, remove `ConfigPath` too

### Phase 2: migrate Pinocchio

#### `pinocchio/cmd/web-chat/main.go`
- remove the no-op resolver
- keep env-prefix behavior only
- continue using `profilebootstrap` for real config loading

#### `pinocchio/cmd/examples/simple-chat/main.go`
- remove the no-op resolver
- keep command behavior unchanged

#### `pinocchio/cmd/agents/simple-chat-agent/main.go`
- remove the no-op resolver
- keep command behavior unchanged

#### `pinocchio/cmd/web-chat/main_profile_registries_test.go`
- update the parser config in tests to match the new API

### Phase 3: migrate glazed examples

#### `glazed/cmd/examples/config-overlay/main.go`
Replace the fixed string-list resolver with a small declarative plan. Example sketch:

```go
ConfigPlanBuilder: func(_ *values.Values, _ *cobra.Command, _ []string) (*config.Plan, error) {
    return config.NewPlan(
        config.WithLayerOrder(config.LayerSystem, config.LayerUser, config.LayerRepo, config.LayerCWD, config.LayerExplicit),
    ).Add(
        config.SourceSpec{...base.yaml...}.Named("base").InLayer(config.LayerSystem),
        config.SourceSpec{...env.yaml...}.Named("env").InLayer(config.LayerUser),
        config.SourceSpec{...local.yaml...}.Named("local").InLayer(config.LayerCWD),
    ), nil
}
```

Because this is example code, it is acceptable to use simple custom `SourceSpec` values or add a tiny helper for fixed files if that makes the example cleaner.

#### `glazed/cmd/examples/overlay-override/main.go`
Use a plan builder that:
- decodes `command-settings.config-file`
- adds `ExplicitFile(cs.ConfigFile)`
- adds an optional computed override source for `*.override.yaml`

### Phase 4: docs cleanup

Update any docs/examples that still present:
- `ConfigFilesFunc`
- `ConfigPath`
- implicit `AppName` config discovery

The docs should make the new rule obvious: **config loading is explicit policy, not automatic magic.**

## `pkg/appconfig` usage inventory

## Workspace (`/home/manuel/workspaces/2026-04-10/pinocchiorc`)

### Real usage
- No production Pinocchio or Geppetto command in this workspace currently depends on `pkg/appconfig`

### Current references
- `glazed/cmd/examples/appconfig-parser/main.go`
- `glazed/cmd/examples/appconfig-profiles/main.go`
- `glazed/pkg/appconfig/*`
- tests for `pkg/appconfig`

Interpretation: inside this workspace, `pkg/appconfig` is mostly a local facade with examples/tests, not a widely adopted integration point.

## Corporate-headquarters (`/home/manuel/code/wesen/corporate-headquarters`)

### Real non-example usage found
- `prescribe/cmd/prescribe/cmds/generate.go`

That call site uses `appconfig.NewParser(...)` for a **bootstrap parse of profile selection**. It combines:
- `WithDefaults()`
- `WithConfigFiles(configFiles...)`
- `WithEnv("PINOCCHIO")`
- `WithCobra(cmd, args)`

The usage is narrow and local. It does not look like broad application-wide adoption of `pkg/appconfig`.

## Recommendation for `pkg/appconfig`

Given the user preference to avoid backwards compatibility, the best cleanup is **not** to invest in `pkg/appconfig` as another long-term compatibility facade unless a second serious production consumer appears.

### Recommended direction

#### Preferred
- Do **not** expand `pkg/appconfig` first
- Migrate the one real production caller (`prescribe`) off `pkg/appconfig`
- After that, reassess whether `pkg/appconfig` still deserves to exist as a public package

#### Why this is cleaner
- usage is tiny
- the workspace itself does not depend on it for production CLI bootstrap
- expanding it to support plans would create another public surface to maintain
- the same bootstrap behavior can be expressed directly with schema + `cmd_sources.Execute(...)` or with a higher-level shared helper where warranted

### If `prescribe` is kept on `pkg/appconfig`

Only do the minimum viable modernization:
- add `WithResolvedConfigFiles(...)` or `WithConfigPlan(...)`
- update `WithProfile(...)` bootstrap parsing to use resolved files too

But this should be treated as a deliberate decision to preserve `pkg/appconfig`, not the default path.

## Best cleanup strategy if we want simplicity over compatibility

### For `CobraParser`
- remove `ConfigFilesFunc`
- strongly consider removing `ConfigPath`
- keep `MiddlewaresFunc` as the advanced escape hatch
- make plan-based config loading the only built-in config-discovery path

### For Pinocchio
- delete the no-op suppression shims
- let profile/bootstrap own config loading fully

### For examples
- migrate to `ConfigPlanBuilder`
- use the examples to teach explicit config policy and provenance-aware loading

### For `pkg/appconfig`
- avoid broad modernization unless usage grows
- prefer migrating `prescribe` off it
- after that, consider deprecating or removing the package rather than turning it into another compatibility boundary

## Risks and review points

### 1. `AppName` meaning changes
Today `AppName` implies env prefix **and** default config discovery. After cleanup it should mean env prefix only. That is a behavior change and needs explicit release-note/doc treatment.

### 2. Example churn is intentional
Some Glazed examples currently exist to show the old path-based story. Those should be updated rather than preserved.

### 3. `prescribe` follow-up is outside this workspace
The workspace cleanup can proceed independently, but deleting `pkg/appconfig` entirely would require a coordinated follow-up in `corporate-headquarters`.

## Proposed implementation tasks

1. Add `ConfigPlanBuilder` to `CobraParserConfig`
2. Remove `ConfigFilesFunc` from `CobraParserConfig`
3. Remove implicit config discovery from the default `CobraParser` middleware chain
4. Migrate Pinocchio no-op callers to the simplified parser config
5. Migrate Glazed examples to declarative config plans
6. Update docs that still teach the old path-list API
7. Audit `ConfigPath` and either remove it in the same ticket or open an immediate follow-up
8. In `corporate-headquarters`, migrate `prescribe` away from `pkg/appconfig` if the goal is to retire that package

## File inventory used for this analysis

### Workspace files
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/config-overlay/main.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/overlay-override/main.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-chat/main.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/agents/simple-chat-agent/main.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/options.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/parser.go`

### Corporate-headquarters files
- `/home/manuel/code/wesen/corporate-headquarters/prescribe/cmd/prescribe/cmds/generate.go`

## Recommendation

Proceed with an aggressive cleanup ticket:

- remove `ConfigFilesFunc`
- make config loading explicit and plan-based
- simplify Pinocchio by deleting its no-op suppression shims
- avoid treating `pkg/appconfig` as a compatibility layer to preserve by default

If we want the codebase simpler rather than more accommodating, this is the right moment to make the cut while the live usage surface is still small.
