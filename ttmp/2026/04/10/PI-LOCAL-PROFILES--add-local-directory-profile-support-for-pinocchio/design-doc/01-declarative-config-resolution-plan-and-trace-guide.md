---
Title: Declarative Config Resolution Plan and Trace Guide
Ticket: PI-LOCAL-PROFILES
Status: active
Topics:
    - pinocchio
    - profiles
    - config
    - geppetto
    - glazed
    - tracing
    - api-design
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/config.go
      Note: Bootstrap config should grow a config plan builder
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/engine_settings.go
      Note: Hidden base inference settings path must consume resolved config-file metadata
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/inference_debug.go
      Note: Inference debug trace should preserve config-layer provenance
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_selection.go
      Note: Current hardcoded config-file resolution to replace with declarative plan
    - Path: ../../../../../../../glazed/pkg/cli/helpers.go
      Note: Prints parsed field history and should surface config-layer metadata clearly
    - Path: ../../../../../../../glazed/pkg/cmds/fields/field-value.go
      Note: Stores parse history on FieldValue.Log and merge behavior
    - Path: ../../../../../../../glazed/pkg/cmds/fields/parse.go
      Note: Defines ParseStep and parse metadata carrier for config layer provenance
    - Path: ../../../../../../../glazed/pkg/cmds/sources/load-fields-from-config.go
      Note: Config file loader that should gain FromResolvedFiles and enriched provenance metadata
    - Path: ../../../../../../../glazed/pkg/config/resolve.go
      Note: Current single-path config resolver to evolve into declarative plan primitives
    - Path: pkg/cmds/profilebootstrap/parsed_base_settings.go
      Note: Confirms baseline/profile source filtering still behaves after richer config provenance
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Pinocchio wiring point for the new declarative plan
    - Path: pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md
      Note: User-facing docs that should explain the new local layers and tracing behavior
ExternalSources: []
Summary: |
    Detailed intern-friendly design and implementation guide for replacing hardcoded config-file lookup order with a declarative config resolution plan in glazed, integrating it into geppetto bootstrap, surfacing config layers in parsed field value history, and wiring project-local profile files in pinocchio.
LastUpdated: 2026-04-10T00:00:00Z
WhatFor: ""
WhenToUse: ""
---


# Declarative Config Resolution Plan and Trace Guide

## Executive Summary

This document proposes a new **declarative config resolution system** for the go-go-golems stack, with Pinocchio as the motivating use case. Today, config file discovery is mostly hardcoded: helper functions decide where to look, then the caller manually prepends or appends files to get the desired precedence. That works for simple cases, but it becomes difficult to reason about when we add new sources such as:

- a local project file in the current working directory
- a local project file in the git repository root
- explicit CLI config files
- future env-driven config file lists
- profile-specific overlays vs full app config files

The core proposal is to move from **hardcoded path resolution** to a **declarative resolution plan**. Instead of asking a helper for “the config path”, callers will build a plan that explicitly lists:

- **which sources exist**
- **what semantic layer each source belongs to**
- **what order those layers apply in**
- **which sources are optional, conditional, or required**
- **what provenance metadata should be recorded for tracing**

This document also proposes a matching **trace/provenance improvement**: whenever a field value comes from a config file, its parse history should record not only `source=config` and `config_file=...`, but also the **config layer** and **config source name**. This matters because once we support multiple config layers, path-only provenance is not enough. Reviewers and users need to be able to answer questions like:

- Did this value come from the repo-level file or the cwd-level file?
- Did the explicit CLI file override the user config?
- Why did this profile slug win?
- Which config layer should I edit to change this behavior?

The design aims to keep the generic machinery in **glazed**, keep **geppetto** responsible for profile/bootstrap wiring, and keep **pinocchio** as a thin consumer that assembles the right plan.

---

## Who This Document Is For

This guide is written for a new intern or contributor who has never worked in this part of the codebase before. If that is you, read this document linearly. It explains both the current system and the proposed design in enough detail that you should be able to:

- understand the current profile/config bootstrap path
- understand why the current API is too implicit
- understand the proposed declarative API and trace model
- implement the work in phases without guessing architecture
- validate your changes with unit and integration tests

You do **not** need prior knowledge of Pinocchio’s profile system to follow this document, but it helps if you are comfortable with:

- Go interfaces and builder-style APIs
- CLI config precedence concepts
- YAML config parsing
- field-level provenance / parse logs

---

## Problem Statement

### The immediate product problem

Pinocchio currently resolves config files from conventional global locations:

1. `$XDG_CONFIG_HOME/pinocchio/config.yaml`
2. `$HOME/.pinocchio/config.yaml`
3. `/etc/pinocchio/config.yaml`
4. explicit `--config-file`

The feature request behind this ticket is to also support project-local profile/config overlays such as:

- `.pinocchio-profile.yml` in the current working directory
- `.pinocchio-profile.yml` in the git repository root

This enables project-scoped behavior that travels with a repository.

### The deeper architecture problem

The deeper issue is not only “add two more lookup paths.” The deeper issue is that the current API shape makes precedence hard to express clearly.

Today, a developer has to infer the loading model by reading several helpers and then mentally combining:

- a path resolver
- explicit file handling
- environment handling
- default value handling
- merge order
- profile overlay logic

This is fragile because the intent is hidden in control flow rather than represented as data.

### The tracing problem

Even if we add local config files, the parse history only partially explains where a value came from. Current config-file parse steps already carry useful metadata such as `config_file` and `index`, but once there are many sources this is still not enough. We need parse history to say things like:

- `config_layer: system`
- `config_layer: user`
- `config_layer: repo`
- `config_layer: cwd`
- `config_layer: explicit`
- `config_source_name: git-root-local-profile`

Without that, debugging multi-layer config systems becomes guesswork.

---

## Goals

### Primary goals

- Introduce a **declarative and configurable config resolution plan**.
- Make the **source order readable in one place**.
- Support **project-local config/profile files** without hardcoding append/prepend logic in app code.
- Preserve the current bootstrap split:
  - generic config discovery in glazed
  - profile/bootstrap orchestration in geppetto
  - app-specific wiring in pinocchio
- Surface the **config layer** in parsed field value history for tracing and debugging.

### Secondary goals

- Make the plan **explainable** in logs and debug output.
- Keep the API ergonomic for common cases.
- Allow future extensions such as:
  - env-provided config files
  - upward search for files
  - stop-at-first-found semantics
  - different merge strategies for different source kinds

### Non-goals for v1

- Do not redesign all field parsing APIs.
- Do not invent a full external DSL for config resolution.
- Do not introduce a huge graph planner unless it is truly needed.
- Do not change how profiles merge into inference settings beyond provenance improvements.
- Do not implement profile CRUD or repository-managed profile editing.

---

## Current System: What Exists Today

This section is important. Before proposing changes, we need to understand what already exists and where it lives.

## High-Level Stack Ownership

### Glazed

Glazed owns generic building blocks:

- schema/sections/fields
- parsed field values and parse logs
- config-file loading middleware
- generic app config path resolution helpers

Relevant files:

- `glazed/pkg/config/resolve.go`
- `glazed/pkg/cmds/sources/load-fields-from-config.go`
- `glazed/pkg/cmds/fields/parse.go`
- `glazed/pkg/cmds/fields/field-value.go`
- `glazed/pkg/cli/helpers.go`

### Geppetto

Geppetto owns profile/bootstrap orchestration:

- profile settings resolution from config/env/defaults/cli
- hidden base inference settings construction
- merging resolved engine profiles on top of the baseline
- inference debug trace assembly

Relevant files:

- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/inference_debug.go`

### Pinocchio

Pinocchio is currently a thin wrapper over geppetto bootstrap, plus app-specific config mapping and profile runtime behavior.

Relevant files:

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go`
- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`

---

## Current Config Resolution Flow

### Diagram: current file resolution model

```text
                 ┌───────────────────────────────────┐
                 │ CLI parsed values                 │
                 │ command-settings.config-file      │
                 └───────────────────────────────────┘
                                  │
                                  ▼
             ┌────────────────────────────────────────────┐
             │ geppetto/pkg/cli/bootstrap/                │
             │ ResolveCLIConfigFiles()                    │
             └────────────────────────────────────────────┘
                                  │
                 ┌────────────────┴────────────────┐
                 │                                 │
                 ▼                                 ▼
  ResolveAppConfigPath(app, "")      ResolveAppConfigPath(app, explicit)
                 │                                 │
                 ▼                                 ▼
  first found among XDG/home/etc            exact explicit path
                 │                                 │
                 └──────────────┬──────────────────┘
                                ▼
                        []string configFiles
                                ▼
                 sources.FromFiles(configFiles, ...)
                                ▼
                      parsed values with source=config
```

### Important detail: current helper shape

`glazed/pkg/config/resolve.go` currently exposes:

```go
func ResolveAppConfigPath(appName string, explicit string) (string, error)
```

This API returns only **one path**. That is a clue that the original design was for a simple “find first config file” workflow, not for multi-layer composition.

### Important detail: current FromFiles middleware

`glazed/pkg/cmds/sources/load-fields-from-config.go` already applies multiple files in order and records config-file metadata:

- `source = "config"`
- `metadata["config_file"] = <path>`
- `metadata["index"] = <position in list>`

This means we already have a partial provenance mechanism. We do **not** need to invent provenance from scratch. We need to enrich it.

### Important detail: current parse log model

`glazed/pkg/cmds/fields/parse.go` defines:

```go
type ParseStep struct {
    Source   string
    Value    interface{}
    Metadata map[string]interface{}
}
```

and `FieldValue.Log []ParseStep` stores the history. This is our hook for traceability.

---

## Why the Current API Is Too Implicit

The current API makes several things hard to see.

### 1. It hides ordering intent

When you see this:

```go
path, _ := ResolveAppConfigPath(appName, explicit)
if path != "" {
    files = append(files, path)
}
```

you cannot immediately tell:

- whether repo-local files were considered
- whether cwd should override git-root or the reverse
- whether explicit files override everything or replace everything
- whether `/etc` is lower precedence than `$HOME`

The ordering is hidden in helper implementation.

### 2. It mixes discovery policy with app policy

Questions like these are policy questions:

- should pinocchio read `.pinocchio-profile.yml`?
- should repo config be enabled for all apps or only some apps?
- should explicit `--config-file` append or replace?

But the current shape pushes these decisions into ad hoc branching.

### 3. It is hard to explain to users

If a user asks “why is this profile selected?”, the best answer should come from the system itself. Hardcoded helper chains are harder to introspect than a declarative resolution plan.

### 4. It under-specifies provenance

Knowing only `config_file=/repo/.pinocchio-profile.yml` is useful, but not sufficient. You also want to know that the file came from the **repo** layer and not the **cwd** or **explicit** layer.

---

## Proposed Design: Declarative Resolution Plan

## Design Summary

The central proposal is to introduce a **Config Resolution Plan** in glazed.

Instead of “find the config path,” callers will create a plan consisting of ordered source specifications. A source specification says:

- what this source is called
- what semantic layer it belongs to
- how to discover candidate paths
- whether it is optional
- whether it should stop after first match
- what metadata should be attached to parse steps for traceability

The plan then resolves into:

- a list of resolved file paths in precedence order
- an explanation/report of what was found and skipped
- per-source metadata that can be attached during `FromFiles` loading

### Mental model

Think of the new system in three phases:

1. **Discovery**: each source asks “do I yield any config files?”
2. **Planning**: the plan orders, filters, dedupes, and explains them
3. **Loading**: the resulting files are passed into `sources.FromFiles(...)` with enriched metadata

---

## Core Concepts

## Concept 1: Layer

A **layer** is a semantic precedence bucket.

Recommended initial layers:

- `system`
- `user`
- `repo`
- `cwd`
- `explicit`

Layers are more readable than raw priority numbers. A developer can look at the layer names and understand intent immediately.

### Example

```text
system   -> /etc/pinocchio/config.yaml
user     -> ~/.config/pinocchio/config.yaml
repo     -> /repo/.pinocchio-profile.yml
cwd      -> /repo/subdir/.pinocchio-profile.yml
explicit -> --config-file /tmp/debug.yaml
```

## Concept 2: Source Spec

A **source spec** is one unit of discovery policy.

Example source specs:

- `system-app-config`
- `xdg-app-config`
- `home-app-config`
- `git-root-local-profile`
- `cwd-local-profile`
- `explicit-config-file`

Each source spec is responsible for discovering zero or more candidate files.

## Concept 3: Resolved Source

A **resolved source** is the runtime result of executing one source spec.

It should record:

- source name
- layer
- discovered paths
- whether it was skipped
- why it was skipped

This enables explain/debug output.

## Concept 4: Plan Report

A **plan report** is the human-readable summary of the resolution process.

Example:

```text
Config resolution plan:
1. system-app-config      layer=system   found  /etc/pinocchio/config.yaml
2. xdg-app-config         layer=user     found  /home/manuel/.config/pinocchio/config.yaml
3. home-app-config        layer=user     skipped (not found)
4. git-root-local-profile layer=repo     found  /repo/.pinocchio-profile.yml
5. cwd-local-profile      layer=cwd      skipped (same path as repo source)
6. explicit-config-file   layer=explicit skipped (empty)
```

This is valuable both for tests and for user-facing debugging.

---

## Proposed API Shape

## Recommended public API: plan + source specs + options

This is the recommended v1 shape because it is declarative without being overengineered.

### Pseudocode: core types

```go
package config

type ConfigLayer string

const (
    LayerSystem   ConfigLayer = "system"
    LayerUser     ConfigLayer = "user"
    LayerRepo     ConfigLayer = "repo"
    LayerCWD      ConfigLayer = "cwd"
    LayerExplicit ConfigLayer = "explicit"
)

type DiscoverFunc func(ctx context.Context) ([]string, error)

type SourceSpec struct {
    Name        string
    Layer       ConfigLayer
    Discover    DiscoverFunc
    Optional    bool
    StopIfFound bool
    Metadata    map[string]any
    EnabledIf   func(context.Context) bool
}

type ResolvedSource struct {
    Name    string
    Layer   ConfigLayer
    Paths   []string
    Found   bool
    Skipped string
}

type Plan struct {
    layerOrder []ConfigLayer
    sources    []SourceSpec
    dedupe     bool
}
```

### Pseudocode: core methods

```go
func NewPlan(opts ...PlanOption) *Plan
func (p *Plan) Add(sources ...SourceSpec) *Plan
func (p *Plan) Resolve(ctx context.Context) ([]string, *PlanReport, error)
func (p *Plan) Explain(ctx context.Context) (*PlanReport, error)
```

### Example: pinocchio plan

```go
plan := config.NewPlan(
    config.WithLayerOrder(
        config.LayerSystem,
        config.LayerUser,
        config.LayerRepo,
        config.LayerCWD,
        config.LayerExplicit,
    ),
    config.WithDedupePaths(),
).Add(
    config.SystemAppConfig("pinocchio").InLayer(config.LayerSystem),
    config.XDGAppConfig("pinocchio").InLayer(config.LayerUser),
    config.HomeAppConfig("pinocchio").InLayer(config.LayerUser),
    config.GitRootFile(".pinocchio-profile.yml").InLayer(config.LayerRepo),
    config.WorkingDirFile(".pinocchio-profile.yml").InLayer(config.LayerCWD),
    config.ExplicitFile(explicit).InLayer(config.LayerExplicit),
)

files, report, err := plan.Resolve(ctx)
```

That one block tells a reviewer almost everything they need to know.

---

## Optional Builder Sugar

To make app wiring even easier, we can optionally add a fluent builder.

### Example

```go
plan := config.ForApp("pinocchio").
    UseStandardSystemConfig().
    UseStandardUserConfig().
    UseGitRootFile(".pinocchio-profile.yml").
    UseWorkingDirFile(".pinocchio-profile.yml").
    UseExplicitFile(explicit).
    WithLayerOrder(
        config.LayerSystem,
        config.LayerUser,
        config.LayerRepo,
        config.LayerCWD,
        config.LayerExplicit,
    ).
    Build()
```

This is nice ergonomically, but it should compile down to the same underlying `Plan` and `SourceSpec` model.

**Recommendation:** implement the lower-level plan first, then add builder sugar only if it improves clarity.

---

## Built-In Source Constructors

These should live in glazed because they are generic.

### Required initial source constructors

- `SystemAppConfig(appName string)`
- `XDGAppConfig(appName string)`
- `HomeAppConfig(appName string)`
- `ExplicitFile(path string)`
- `WorkingDirFile(name string)`
- `GitRootFile(name string)`

### Nice-to-have future source constructors

- `EnvFile(envVar string)`
- `UpwardSearchFile(name string, stopAt string)`
- `Files(paths ...string)`
- `Glob(pattern string)`

### Example implementation sketch

```go
func WorkingDirFile(name string) SourceSpec {
    return SourceSpec{
        Name:     "working-dir-file",
        Layer:    LayerCWD,
        Optional: true,
        Discover: func(ctx context.Context) ([]string, error) {
            cwd, err := os.Getwd()
            if err != nil {
                return nil, err
            }
            p := filepath.Join(cwd, name)
            if !fileExists(p) {
                return nil, nil
            }
            return []string{p}, nil
        },
    }
}
```

---

## Why Layers Are Better Than Plain Priorities

A numeric-priority API would work technically, but it is harder to read.

### Less good

```go
Add(config.SourceSpec{Name: "repo", Priority: 300})
Add(config.SourceSpec{Name: "cwd", Priority: 400})
```

A reviewer now has to remember what 300 and 400 mean.

### Better

```go
Add(config.GitRootFile(".pinocchio-profile.yml").InLayer(config.LayerRepo))
Add(config.WorkingDirFile(".pinocchio-profile.yml").InLayer(config.LayerCWD))
```

The intent is embedded in the API.

**Recommendation:** use named layers in v1. If necessary, a future version can add before/after constraints.

---

## Recommended Precedence Model

For the Pinocchio local-profile use case, the recommended order is:

```text
system -> user -> repo -> cwd -> explicit
```

### Why this order?

- `system`: global machine defaults
- `user`: personal preferences
- `repo`: settings that travel with the repository
- `cwd`: the most local working-directory override
- `explicit`: user explicitly asked for this file right now

### Important subtlety

This document uses “low -> high precedence” list order to match how `sources.FromFiles(...)` currently applies files in order.

That means later files win.

### Resulting precedence table

| Layer | Example | Why it exists |
|---|---|---|
| system | `/etc/pinocchio/config.yaml` | machine-wide defaults |
| user | `~/.config/pinocchio/config.yaml` | user preference |
| repo | `/repo/.pinocchio-profile.yml` | repo-shared behavior |
| cwd | `/repo/subdir/.pinocchio-profile.yml` | task/subdir-local behavior |
| explicit | `--config-file /tmp/debug.yaml` | immediate override |

---

## Provenance / Trace Requirements

This is a critical part of the proposal.

## Current state

When config files are loaded via `sources.FromFiles(...)`, parse metadata already includes:

- `config_file`
- `index`

That is useful, but insufficient in a layered system.

## Required new trace fields

Every parse step originating from a config file should also include:

- `config_layer` — semantic layer (`system`, `user`, `repo`, `cwd`, `explicit`)
- `config_source_name` — stable source identifier (`git-root-local-profile`, `xdg-app-config`, etc.)
- `config_source_kind` — optional classification such as `app-config`, `profile-overlay`, `explicit-file`

### Recommended metadata example

```yaml
source: config
value: local-dev
metadata:
  config_file: /repo/.pinocchio-profile.yml
  index: 3
  config_layer: repo
  config_source_name: git-root-local-profile
  config_source_kind: profile-overlay
```

## Why this matters

This gives us end-to-end traceability:

- the path tells us **which file**
- the layer tells us **why that file had that precedence**
- the source name tells us **which discovery rule found it**
- the source kind tells us **what semantic role it played**

That is exactly the information you want when debugging config behavior.

---

## Parsed Field Value History: Concrete Requirements

The user specifically asked that the **config layer** be visible in the parsed field value history.

This means the following outputs should become more informative.

### 1. `glazed/pkg/cli/helpers.go` parsed-fields printer

Today, the parsed-fields printer renders `value` and `log`. It already prints metadata if present. Once we enrich config parse metadata, this printer will automatically become more useful.

Example desired output:

```yaml
profile-settings:
  profile:
    value: local-dev
    log:
      - source: defaults
        value: ""
      - source: config
        value: repo-default
        metadata:
          config_file: /repo/.pinocchio-profile.yml
          config_layer: repo
          config_source_name: git-root-local-profile
      - source: cli
        value: local-dev
```

### 2. geppetto inference debug trace

`geppetto/pkg/cli/bootstrap/inference_debug.go` should preserve and display these metadata fields when building inference-setting traces.

If a final engine field value is traced back to config, reviewers should be able to identify not only the file path but the source layer.

### 3. tests

Unit and integration tests should explicitly assert `config_layer` for relevant fields. This avoids regressions where path metadata survives but layer metadata disappears.

---

## Recommended Internal Data Flow

### Diagram: proposed end-to-end flow

```text
                  ┌─────────────────────────────┐
                  │ app/bootstrap code          │
                  │ builds config.Plan          │
                  └──────────────┬──────────────┘
                                 │
                                 ▼
                     ┌──────────────────────────┐
                     │ plan.Resolve(ctx)        │
                     │ - discover               │
                     │ - order by layer         │
                     │ - dedupe                 │
                     │ - build report           │
                     └──────────────┬───────────┘
                                    │
                                    ▼
                         []ResolvedConfigFile
                                    │
                                    ▼
                   sources.FromFilesWithResolvedSources(...)
                                    │
                                    ▼
                 parsed field values with config-layer metadata
                                    │
                 ┌──────────────────┴──────────────────┐
                 ▼                                     ▼
        profile selection tracing            inference/base tracing
                 ▼                                     ▼
             debug output                         user-visible logs
```

---

## Proposed API Details

## Proposal A: return richer resolved file objects, not just paths

This is strongly recommended.

If the plan only returns `[]string`, then the caller must separately carry layer/source metadata, which is brittle.

Instead, the plan should return something like:

```go
type ResolvedConfigFile struct {
    Path       string
    Layer      ConfigLayer
    SourceName string
    SourceKind string
    Index      int
}
```

Then we add a new glazed loader helper:

```go
func FromResolvedFiles(files []ResolvedConfigFile, options ...ConfigFileOption) Middleware
```

This helper can attach metadata directly without the caller doing custom plumbing.

### Why this is better

- metadata stays coupled to the path
- tests become clearer
- debug output has first-class access to layer/source info

### Backward-compatibility path

Keep `FromFiles([]string, ...)` as-is and add `FromResolvedFiles(...)` alongside it.

---

## Proposal B: keep `ParseStep` generic, do not create a new provenance type yet

`ParseStep.Metadata map[string]interface{}` is already flexible enough.

For v1, the best move is:

- keep `ParseStep` unchanged
- standardize metadata keys for config provenance
- document those keys
- add tests around them

### Standard metadata keys for config parse steps

```text
config_file
config_index
config_layer
config_source_name
config_source_kind
```

Note: current code uses `index`. For clarity and future-proofing, prefer `config_index` for the new API. If we need compatibility, we can keep both for a transition period.

---

## Proposal C: AppBootstrapConfig should accept a plan, not just app name

Today `geppetto/pkg/cli/bootstrap/config.go` defines:

```go
type AppBootstrapConfig struct {
    AppName           string
    EnvPrefix         string
    ConfigFileMapper  sources.ConfigFileMapper
    NewProfileSection func() (schema.Section, error)
    BuildBaseSections func() ([]schema.Section, error)
}
```

This is not enough for a configurable config resolution model.

### Recommended extension

```go
type AppBootstrapConfig struct {
    AppName           string
    EnvPrefix         string
    ConfigFileMapper  sources.ConfigFileMapper
    NewProfileSection func() (schema.Section, error)
    BuildBaseSections func() ([]schema.Section, error)

    ConfigPlanBuilder func(parsed *values.Values) (*config.Plan, error)
}
```

This keeps geppetto generic while allowing each app to define its own plan.

### Why builder instead of static plan?

Because the explicit config-file source depends on parsed CLI values:

- if `--config-file` is empty, there may be no explicit source path
- if it is present, the plan should include it

A builder function lets bootstrap derive the plan from already-parsed CLI values.

---

## Proposal D: keep a convenience fallback for simple apps

Not every app wants to hand-build a config plan.

So geppetto/bootstrap should provide a default plan builder for standard cases.

### Example

```go
func StandardAppConfigPlan(appName string, explicit string) *config.Plan
```

and apps can either:

- use that helper directly
- or compose their own richer plan

This keeps the framework ergonomic.

---

## Pinocchio-Specific Recommended Plan

For Pinocchio, the recommended initial plan is:

```go
func pinocchioConfigPlan(parsed *values.Values) (*config.Plan, error) {
    explicit := readExplicitConfigFileFromCommandSettings(parsed)

    return config.NewPlan(
        config.WithLayerOrder(
            config.LayerSystem,
            config.LayerUser,
            config.LayerRepo,
            config.LayerCWD,
            config.LayerExplicit,
        ),
        config.WithDedupePaths(),
    ).Add(
        config.SystemAppConfig("pinocchio").
            Named("system-app-config").
            Kind("app-config"),

        config.XDGAppConfig("pinocchio").
            Named("xdg-app-config").
            Kind("app-config"),

        config.HomeAppConfig("pinocchio").
            Named("home-app-config").
            Kind("app-config"),

        config.GitRootFile(".pinocchio-profile.yml").
            Named("git-root-local-profile").
            Kind("profile-overlay"),

        config.WorkingDirFile(".pinocchio-profile.yml").
            Named("cwd-local-profile").
            Kind("profile-overlay"),

        config.ExplicitFile(explicit).
            Named("explicit-config-file").
            Kind("explicit-file"),
    ), nil
}
```

### Why use `.pinocchio-profile.yml` instead of `.pinocchio.yaml`?

Because the feature request is specifically about project-local profile behavior, not necessarily a second full app-config convention.

This naming makes the feature discoverable and semantically narrower.

### Open question

Should the repo/cwd files be allowed to contain full config, or only profile-relevant keys?

Recommendation for v1:

- allow the same top-level config structure as the normal config loader
- document that the intended use is project-local profile selection/registries
- if safety becomes a concern later, narrow the schema then

---

## Detailed Phase-by-Phase Implementation Plan

This section is meant to be actionable for an intern.

## Phase 0: Read and orient yourself

### Files to read first

1. `glazed/pkg/config/resolve.go`
2. `glazed/pkg/cmds/sources/load-fields-from-config.go`
3. `glazed/pkg/cmds/fields/parse.go`
4. `glazed/pkg/cmds/fields/field-value.go`
5. `glazed/pkg/cli/helpers.go`
6. `geppetto/pkg/cli/bootstrap/config.go`
7. `geppetto/pkg/cli/bootstrap/profile_selection.go`
8. `geppetto/pkg/cli/bootstrap/engine_settings.go`
9. `geppetto/pkg/cli/bootstrap/inference_debug.go`
10. `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
11. `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go`
12. `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`

### What you should understand before coding

- how config files are currently discovered
- how `sources.FromFiles(...)` applies files in order
- how parse logs are stored on `FieldValue.Log`
- how geppetto resolves profile settings from config/env/defaults/cli
- how pinocchio wraps geppetto bootstrap

---

## Phase 1: Add generic config plan primitives to glazed

### Goal

Create a new plan abstraction without breaking existing callers.

### Recommended new files

- `glazed/pkg/config/plan.go`
- `glazed/pkg/config/plan_sources.go`
- `glazed/pkg/config/plan_report.go`
- optionally `glazed/pkg/config/git.go`

### Keep existing file

- `glazed/pkg/config/resolve.go`

### Suggested strategy

Do **not** delete `ResolveAppConfigPath(...)` yet. Keep it as a simple compatibility helper.

### Tasks

- define `ConfigLayer`
- define `SourceSpec`
- define `ResolvedSource` / `ResolvedConfigFile`
- define `Plan`
- implement built-in source constructors
- implement dedupe logic
- implement explain/report output

### Unit tests to add

- `SystemAppConfig` returns `/etc/<app>/config.yaml` when present
- `XDGAppConfig` uses user config dir
- `HomeAppConfig` uses home dir
- `WorkingDirFile` finds cwd file
- `GitRootFile` finds git-root file and skips if not in repo
- dedupe removes same path discovered twice
- layer order is preserved

### Important design constraint

A plan should be pure discovery/planning logic. It should not parse YAML itself.

---

## Phase 2: Add resolved-file loading path in glazed sources

### Goal

Allow config loading middleware to preserve plan metadata.

### Recommended file to modify

- `glazed/pkg/cmds/sources/load-fields-from-config.go`

### Recommended addition

```go
func FromResolvedFiles(files []config.ResolvedConfigFile, options ...ConfigFileOption) Middleware
```

### Behavior

For each resolved file, append parse metadata:

```go
fields.WithMetadata(map[string]interface{}{
    "config_file":        file.Path,
    "config_index":       file.Index,
    "config_layer":       string(file.Layer),
    "config_source_name": file.SourceName,
    "config_source_kind": file.SourceKind,
})
```

### Why not overload `FromFiles([]string, ...)`?

Because `[]string` cannot carry layer/source metadata cleanly.

### Tests

- parse log contains `config_layer`
- parse log contains `config_source_name`
- metadata survives serialization via `ToSerializableFieldValue`

---

## Phase 3: Integrate config plans into geppetto bootstrap

### Goal

Make bootstrap resolve files via a declarative plan instead of hardcoded path logic.

### Files to modify

- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- possibly `geppetto/pkg/cli/bootstrap/inference_debug.go`

### Suggested changes

#### 1. Extend `AppBootstrapConfig`

Add:

```go
ConfigPlanBuilder func(parsed *values.Values) (*config.Plan, error)
```

#### 2. Replace `ResolveCLIConfigFiles(...)` internals

It should:

- call the plan builder
- resolve the plan
- return both simple `[]string` compatibility form and richer resolved-file form if needed

### Recommended new types

```go
type ResolvedCLIConfigFiles struct {
    Paths  []string
    Files  []config.ResolvedConfigFile
    Report *config.PlanReport
}
```

This makes debugging and future extension easier.

#### 3. Use `sources.FromResolvedFiles(...)`

In both:

- `ResolveCLIProfileSelection(...)`
- `ResolveBaseInferenceSettings(...)`

### Tests

- profile settings resolved from repo-local file
- cwd overrides repo when both exist
- explicit file overrides all previous layers
- inferred parse history contains `config_layer`

---

## Phase 4: Wire Pinocchio to use the new plan

### Goal

Make pinocchio opt into the richer plan with project-local profile sources.

### File to modify

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`

### Suggested change

Replace the current `pinocchioBootstrapConfig()` with one that sets `ConfigPlanBuilder`.

### Example sketch

```go
func pinocchioBootstrapConfig() bootstrap.AppBootstrapConfig {
    return bootstrap.AppBootstrapConfig{
        AppName:          "pinocchio",
        EnvPrefix:        "PINOCCHIO",
        ConfigFileMapper: configFileMapper,
        NewProfileSection: func() (schema.Section, error) {
            return geppettosections.NewProfileSettingsSection()
        },
        BuildBaseSections: func() ([]schema.Section, error) {
            return geppettosections.CreateGeppettoSections()
        },
        ConfigPlanBuilder: pinocchioConfigPlan,
    }
}
```

### Tests to add in pinocchio

- config from `.pinocchio-profile.yml` in cwd is loaded
- config from git root is loaded when cwd does not contain file
- cwd and git root same file dedupes correctly
- printed parsed fields show `config_layer: cwd` or `config_layer: repo`

---

## Phase 5: Improve debug and trace outputs

### Goal

Make the new provenance visible and useful.

### Files to inspect/modify

- `glazed/pkg/cli/helpers.go`
- `geppetto/pkg/cli/bootstrap/inference_debug.go`

### Desired outcome

Both generic parsed-fields dumps and inference-setting debug traces should preserve the enriched config metadata.

### Important note

`glazed/pkg/cli/helpers.go` may need little or no logic change because it already prints metadata when present. Still, tests should be added to lock in expected output.

### Tests

- parsed-fields output includes `config_layer`
- inference debug YAML includes `config_layer` on config-derived steps

---

## Phase 6: Documentation and migration notes

### Files to update

- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- possibly `pinocchio/README.md`
- optionally glazed/geppetto docs if public API is intended for external consumers

### What docs must explain

- new project-local file names
- precedence order
- difference between repo and cwd layers
- how to inspect parsed field history to debug precedence

---

## Suggested Pseudocode for End-to-End Bootstrap

```go
func ResolveCLIProfileSelection(cfg AppBootstrapConfig, parsed *values.Values) (*ResolvedCLIProfileSelection, error) {
    profileSection, err := cfg.NewProfileSection()
    if err != nil {
        return nil, err
    }

    schema_ := schema.NewSchema(schema.WithSections(profileSection))
    resolvedValues := values.New()

    resolvedFiles, err := ResolveCLIConfigFiles(cfg, parsed)
    if err != nil {
        return nil, err
    }

    if err := sources.Execute(
        schema_,
        resolvedValues,
        sources.FromEnv(cfg.normalizedEnvPrefix(), fields.WithSource("env")),
        sources.FromResolvedFiles(
            resolvedFiles.Files,
            sources.WithConfigFileMapper(cfg.ConfigFileMapper),
        ),
        sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
    ); err != nil {
        return nil, err
    }

    if parsed != nil {
        if err := resolvedValues.Merge(parsed); err != nil {
            return nil, err
        }
    }

    profileSettings := ResolveProfileSettings(resolvedValues)
    return &ResolvedCLIProfileSelection{
        ProfileSettings: profileSettings,
        ConfigFiles:     resolvedFiles.Paths,
    }, nil
}
```

---

## Example Debug Output After the Change

### Parsed fields dump

```yaml
profile-settings:
  profile:
    value: local-dev
    log:
      - source: config
        value: team-default
        metadata:
          config_file: /repo/.pinocchio-profile.yml
          config_index: 2
          config_layer: repo
          config_source_name: git-root-local-profile
          config_source_kind: profile-overlay
      - source: config
        value: sandbox
        metadata:
          config_file: /repo/subdir/.pinocchio-profile.yml
          config_index: 3
          config_layer: cwd
          config_source_name: cwd-local-profile
          config_source_kind: profile-overlay
      - source: cli
        value: local-dev
```

### Why this is good

A human can immediately answer:

- repo file set the initial value
- cwd file overrode it
- cli overrode both

That is exactly the level of observability we want.

---

## Alternatives Considered

## Alternative 1: Just add more hardcoded paths to `ResolveAppConfigPath`

### Why rejected

This solves today’s immediate need but makes the API worse. A helper returning a single path is the wrong abstraction for multi-layer config composition.

## Alternative 2: Add `ResolveAppConfigPathsWithLocal(...)` as a one-off helper

### Why partly attractive

It is quick to implement.

### Why still not recommended

It bakes one precedence pattern into one helper and still leaves the caller with an implicit model. The next custom need will cause another helper or more append/prepend branching.

## Alternative 3: Use numeric priorities only

### Why rejected for v1

Works mechanically, but hurts readability.

## Alternative 4: Full DAG ordering with before/after constraints

### Why deferred

Powerful, but likely overkill for the current problem. Named layers are simpler and easier to explain.

---

## Risks and Sharp Edges

## Risk 1: same file discovered by multiple sources

Example:

- cwd is the git root
- both `GitRootFile` and `WorkingDirFile` discover the same path

### Mitigation

Plan must support dedupe by normalized absolute path.

## Risk 2: confusion about merge direction

If file order is not documented carefully, developers may accidentally invert precedence.

### Mitigation

Standardize on one sentence everywhere:

> Config files are applied in list order, low precedence to high precedence; later files win.

## Risk 3: provenance regression during merges

Some flows rebuild parsed values or flatten histories.

### Mitigation

Add tests in both glazed and geppetto that assert `config_layer` survives to final debug output.

## Risk 4: git-root lookup failure modes

Possible cases:

- not in a git repository
- git binary unavailable
- worktree behavior

### Mitigation

For v1, treat git-root source as optional. If git-root discovery fails in a non-fatal way, skip the source and note it in the plan report.

---

## Testing Plan

## Unit tests in glazed

### Config plan tests

- source order matches layer order
- dedupe removes duplicates
- skipped sources are reported
- cwd source discovers only cwd file
- git-root source discovers root file only when in repo

### Source loading tests

- `FromResolvedFiles(...)` attaches metadata
- metadata includes `config_layer`
- metadata includes `config_source_name`

## Integration tests in geppetto

- bootstrap resolves config files via plan
- config order is low -> high precedence
- explicit file wins over cwd/repo/user/system
- parse history reflects all config layers encountered

## Pinocchio tests

- local repo profile file changes selected profile
- local cwd file overrides repo file
- printed parsed fields show layer metadata
- hidden base settings still exclude profile-derived values correctly

---

## Concrete File Reference Map

### Glazed

- `glazed/pkg/config/resolve.go`
  - current single-path helper; keep for compatibility
- `glazed/pkg/cmds/sources/load-fields-from-config.go`
  - current `FromFiles(...)`; add `FromResolvedFiles(...)`
- `glazed/pkg/cmds/fields/parse.go`
  - `ParseStep` definition
- `glazed/pkg/cmds/fields/field-value.go`
  - field value log storage and merge behavior
- `glazed/pkg/cli/helpers.go`
  - parsed field debug printer

### Geppetto

- `geppetto/pkg/cli/bootstrap/config.go`
  - extend bootstrap config with plan builder
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
  - resolve config files from plan
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
  - hidden base settings should also use resolved-file metadata
- `geppetto/pkg/cli/bootstrap/inference_debug.go`
  - verify config-layer metadata survives to trace output

### Pinocchio

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - provide pinocchio-specific plan builder
- `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go`
  - confirm profile-source filtering remains correct
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
  - update docs for new local file behavior and tracing expectations

---

## Intern Implementation Checklist

Use this checklist when implementing.

### Before coding

- [ ] Read the files listed in Phase 0
- [ ] Write down the current precedence order in your own words
- [ ] Confirm how `sources.FromFiles(...)` applies file order

### Glazed phase

- [ ] Add plan types
- [ ] Add source constructors
- [ ] Add plan tests
- [ ] Add `ResolvedConfigFile`
- [ ] Add `FromResolvedFiles(...)`
- [ ] Add metadata tests for `config_layer`

### Geppetto phase

- [ ] Extend `AppBootstrapConfig`
- [ ] Add plan builder plumbing
- [ ] Replace hardcoded path resolution in bootstrap
- [ ] Add trace-focused tests

### Pinocchio phase

- [ ] Build pinocchio plan with repo/cwd sources
- [ ] Add tests for `.pinocchio-profile.yml`
- [ ] Verify parsed field history contains the correct layer
- [ ] Update docs

### Final validation

- [ ] Run affected test packages
- [ ] Manually test cwd vs git-root precedence
- [ ] Manually inspect parsed-fields output
- [ ] Update diary and changelog

---

## Recommended Validation Commands

These are suggested commands for the implementer after code changes.

```bash
# glazed
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
go test ./pkg/config ./pkg/cmds/sources ./pkg/cli/... -count=1

# geppetto
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto
go test ./pkg/cli/bootstrap/... ./pkg/sections/... -count=1

# pinocchio
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./pkg/cmds/... ./cmd/web-chat/... -count=1
```

Manual trace inspection should include whatever command path prints parsed fields or inference settings debug YAML.

---

## Final Recommendation

Implement the new feature as a **declarative config resolution plan in glazed**, consume it from **geppetto bootstrap**, and wire it in **pinocchio** using a pinocchio-specific plan that includes:

- standard system/user config
- git-root `.pinocchio-profile.yml`
- cwd `.pinocchio-profile.yml`
- explicit `--config-file`

At the same time, improve traceability by ensuring every config-derived parse step includes:

- `config_file`
- `config_index`
- `config_layer`
- `config_source_name`
- `config_source_kind`

This yields a design that is:

- easier to read
- easier to extend
- easier to debug
- reusable across go-go-golems apps

---

## Open Questions

- Should `.pinocchio-profile.yml` be allowed to contain full config or only profile-related keys?
- Should git-root discovery shell out to `git` or use a library?
- Should `config_index` replace existing `index` metadata, or should both be emitted temporarily?
- Should we expose plan-report output directly in CLI debug commands?
- Should geppetto bootstrap return the full plan report for higher-level tooling?

---

## References

### Ticket docs

- `analysis/01-local-profile-loading-code-analysis-and-design-options.md`
- `reference/01-diary.md`

### Code references

- `glazed/pkg/config/resolve.go`
- `glazed/pkg/cmds/sources/load-fields-from-config.go`
- `glazed/pkg/cmds/fields/parse.go`
- `glazed/pkg/cmds/fields/field-value.go`
- `glazed/pkg/cli/helpers.go`
- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/inference_debug.go`
- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/parsed_base_settings.go`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
