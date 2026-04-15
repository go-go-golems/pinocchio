---
Title: Current profile, config, and registry architecture analysis
Ticket: PI-PROFILE-FIRST-CONFIG
Status: active
Topics:
    - config
    - pinocchio
    - profiles
    - design
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/engine_settings.go
      Note: Current base-plus-profile runtime merge path that the new design should preserve initially
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_selection.go
      Note: Current generic bootstrap path for profile control-plane resolution and resolved config files
    - Path: ../../../../../../../geppetto/pkg/engineprofiles/source_chain.go
      Note: Current imported-registry chain behavior and precedence rules
    - Path: ../../../../../../../glazed/pkg/config/plan.go
      Note: Defines the declarative config-plan abstraction and layer-ordered resolved files that the new format must continue to use
    - Path: ../../../../../../../glazed/pkg/config/plan_sources.go
      Note: Shows the built-in source constructors and current app/user/repo/cwd/explicit discovery helpers
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Current Pinocchio config plan
    - Path: pkg/cmds/profilebootstrap/repositories.go
      Note: Evidence that app settings are currently special-cased and should become a first-class app block in the unified document
    - Path: pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md
      Note: Documents the current baseline-plus-profile mental model and runtime switching invariants
ExternalSources: []
Summary: |
    Evidence-backed analysis of the current Pinocchio configuration model, covering layered config plans, Geppetto bootstrap, external engine-profile registries, Pinocchio-specific app settings, and the architectural pressures that motivate a profile-first unified config format.
LastUpdated: 2026-04-14T22:55:00-04:00
WhatFor: |
    Explain the current system clearly enough that a new engineer can understand why the next format change is necessary and which invariants must be preserved.
WhenToUse: Use this analysis before implementing the new config format or when evaluating whether a proposal preserves the key properties of the current system.
---


# Current profile, config, and registry architecture analysis

## Executive Summary

Pinocchio's current configuration system is the result of several rounds of cleanup and consolidation. The low-level file discovery model is in good shape: Glazed config plans now express layered discovery explicitly, Geppetto bootstrap consumes those plans through reusable middleware, and Pinocchio declares its own app policy by choosing the concrete file names and mapper behavior.

The remaining complexity is not in *where files come from* but in *what those files mean*. The current model mixes two runtime configuration ideas:

1. top-level Geppetto inference sections loaded directly from layered config files, and
2. separate engine-profile registries that are selected through `profile-settings.profile` and `profile-settings.profile-registries`.

That split makes the system harder to explain than it needs to be. A new engineer has to understand both direct section-based runtime config and registry-based profile overlays before they can predict how a command gets its final `InferenceSettings`. Local project config also uses a filename that suggests “profile-only override,” even though the file currently participates in a broader config mechanism.

The best next improvement is therefore semantic rather than mechanical: keep the current layered plan mechanism, but replace the split runtime model with one unified document format containing app settings, profile selection, and inline profiles.

## Problem Statement

The immediate design question is:

> How should Pinocchio evolve from “top-level runtime config plus optional external profile registry overlay” to a model that is easier to teach, easier to override locally, and still compatible with Geppetto's profile resolution machinery?

To answer that safely, we need to document the current moving parts and the invariants they already provide.

## Reader Orientation

If you are new to this codebase, there are four major layers to keep straight:

1. **Glazed config discovery** — figures out which files exist and in what precedence order.
2. **Geppetto bootstrap** — interprets config files plus env/defaults, resolves profile selection, and produces final engine settings.
3. **Pinocchio app policy** — chooses actual file names, filters app-specific config, and adds app-owned runtime behavior.
4. **Engine profile registries** — load named engine profiles from YAML or SQLite and merge selected profile settings onto a baseline.

The current system works. The proposal in this ticket exists because it is still more complicated than it needs to be for everyday use and onboarding.

## Current Architecture Map

### 1. Glazed owns layered file discovery

The generic layered file-discovery mechanism lives in:

- `glazed/pkg/config/plan.go`
- `glazed/pkg/config/plan_sources.go`
- `glazed/pkg/doc/topics/27-declarative-config-plans.md`

Important concepts exposed there:

- `ConfigLayer` (`system`, `user`, `repo`, `cwd`, `explicit`)
- `SourceSpec`
- `ResolvedConfigFile`
- `Plan`
- `PlanReport`

This layer answers questions like:

- which files should be searched?
- which layer does each file belong to?
- what order should those files be applied in?
- how can we inspect which file won?

It does **not** decide what the YAML document means. It only produces an ordered set of discovered files plus provenance metadata.

### 2. Geppetto owns generic bootstrap behavior

The current generic bootstrap contract lives in:

- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/config_loading.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/profile_registry.go`

`AppBootstrapConfig` currently requires:

- `AppName`
- `EnvPrefix`
- `ConfigFileMapper`
- `NewProfileSection`
- `BuildBaseSections`
- `ConfigPlanBuilder`

That means Geppetto bootstrap is generic, but it still assumes an application can project each config file into Glazed section maps through `ConfigFileMapper`.

### 3. Pinocchio supplies app policy

Pinocchio's concrete bootstrap policy lives in:

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`
- `pinocchio/pkg/cmds/profilebootstrap/repositories.go`

Today Pinocchio defines a config plan with this effective precedence:

```text
system -> user -> repo -> cwd -> explicit
```

with concrete sources:

1. `/etc/pinocchio/config.yaml`
2. `$HOME/.pinocchio/config.yaml`
3. `${XDG_CONFIG_HOME}/pinocchio/config.yaml`
4. git-root `.pinocchio-profile.yml`
5. cwd `.pinocchio-profile.yml`
6. `--config-file <path>`

The most important current Pinocchio-specific behavior is the config mapper:

- top-level `repositories` is excluded from Geppetto runtime-section parsing,
- everything else that looks like a section map is passed through.

That tells us the current document already contains at least two semantic categories:

- app settings (`repositories`)
- runtime section config (`ai-chat`, `ai-client`, `profile-settings`, etc.)

### 4. External registries own named profile catalogs

The registry interfaces live in:

- `geppetto/pkg/engineprofiles/registry.go`
- `geppetto/pkg/engineprofiles/types.go`
- `geppetto/pkg/engineprofiles/source_chain.go`
- `geppetto/pkg/engineprofiles/slugs.go`

Important types:

- `Registry`
- `RegistryReader`
- `ResolveInput`
- `ResolvedEngineProfile`
- `EngineProfile`
- `EngineProfileRegistry`

Profile registries are already a good abstraction. They know how to:

- parse and validate slugs,
- open YAML or SQLite sources,
- chain multiple sources with precedence,
- resolve profile stacks,
- and merge profile inference settings.

The current redesign question is therefore *not* “do we still need profile registries?” The question is “what role should registries play once local layered config becomes the primary everyday configuration document?”

## Current Runtime Resolution Flow

The current flow can be summarized like this:

```text
config plan resolves files
  -> config files mapped into Glazed sections
  -> hidden base inference settings loaded from config/env/defaults
  -> profile selection loaded from config/env/defaults/CLI
  -> external registry chain opened from profile-registries
  -> selected engine profile resolved
  -> profile inference settings merged onto base
  -> final inference settings used to build engines
```

That model appears in:

- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`

### Current mental model

Today the runtime model is effectively:

```text
baseline + profile overlay = active runtime settings
```

Where:

- **baseline** comes from direct config/env/defaults parsing into Geppetto sections,
- **profile overlay** comes from a selected engine profile loaded from registries.

This is functional, but it has a teaching cost: a newcomer must understand both direct section config and separate profile overlay semantics.

## Current Document Shapes

### 1. Layered config files

Current layered config files can directly contain top-level runtime sections, for example:

```yaml
ai-chat:
  ai-api-type: openai
  ai-engine: gpt-5-mini

profile-settings:
  profile: assistant
  profile-registries:
    - ~/.pinocchio/profiles.yaml
```

Those sections are not a typed unified document. They are simply section-shaped YAML that the config mapper passes through into Glazed's field system.

### 2. Separate app settings path

Pinocchio app repositories are loaded separately via:

- `pinocchio/pkg/cmds/profilebootstrap/repositories.go`

That file currently projects only this shape:

```yaml
repositories:
  - ~/prompts
```

This special case exists because the main config mapper ignores `repositories` when producing Geppetto runtime sections.

### 3. Separate profile registry files

Current external registry files use a schema like:

```yaml
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-4o-mini
  assistant:
    slug: assistant
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
```

Examples live in:

- `pinocchio/examples/js/profiles/basic.yaml`
- `geppetto/examples/js/geppetto/profiles/20-team-agent.yaml`

This means the current user has to keep two different YAML schemas in their head:

- section-shaped config files for direct runtime settings,
- registry-shaped YAML for named profiles.

## Current Profile Selection Model

The currently exposed control-plane fields are:

- `profile-settings.profile`
- `profile-settings.profile-registries`

These are defined in:

- `geppetto/pkg/sections/profile_sections.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`

Current behavior:

- if no `profile-registries` are configured and no profile is requested, bootstrap can proceed with base settings only,
- if a profile is requested but no registries are configured, bootstrap returns a validation error,
- if registries are configured, Geppetto opens a chained registry and resolves the requested profile.

This is internally coherent, but user-facing ergonomics suffer because the “normal config file” does not itself carry a reusable inline profile catalog.

## Current Local Override Story

After the config-plan cleanup, Pinocchio now allows local project overrides through:

- git-root `.pinocchio-profile.yml`
- cwd `.pinocchio-profile.yml`

That was an important step forward. However, the current filename and semantics still expose the old conceptual split:

- the file name implies “profile overlay,”
- the document still contains top-level section config,
- and app settings such as `repositories` are still not part of the same path.

This creates an awkward mismatch:

> the new loader is unified, but the document model still feels split.

## Current Runtime Switching Constraint

The runtime-switching design documented in:

- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/cmd/web-chat/main.go`

preserves a profile-free baseline so that later profile switches do not contaminate the underlying settings.

This matters because any new design must preserve the following invariant:

> switching profiles at runtime should be deterministic and should rebuild from a preserved non-profile baseline, not from the previously active merged settings.

The new document format must therefore keep the distinction between:

- app/base state that should survive profile changes,
- and profile-derived runtime state that should be recomputed.

## Design Pressures Exposed By The Current Code

### Pressure 1: The loader is unified, but the document model is not

The config-plan work already unified discovery. The remaining fragmentation comes from document semantics.

Symptoms:

- runtime sections are loaded directly from config files,
- profiles come from separate registry files,
- app settings are partly separate again,
- and the local override filename still encodes the old mental model.

### Pressure 2: `ConfigFileMapper` is no longer the ideal abstraction for runtime config

`ConfigFileMapper` works well when each file can be mapped independently from raw YAML to section maps.

A profile-first config format changes that. Once runtime settings live under named profiles, the loader needs to:

1. read the whole layered document,
2. merge profile catalogs across layers,
3. determine the effective selected profile,
4. maybe import external registries,
5. and only then project a selected runtime profile into `InferenceSettings`.

That is a **document-first** flow, not a file-by-file section-mapping flow.

### Pressure 3: Registries are useful, but too central for day-to-day config

The registry model is strong for:

- team-shared catalogs,
- reusable profile libraries,
- SQLite-backed profile storage,
- and typed stack resolution.

It is weaker as the primary everyday local configuration interface because a user often just wants:

- “this repo should use profile X,” or
- “this directory should slightly tweak the fast profile,” or
- “this local environment should use a different engine.”

Those are more naturally expressed in the layered config document itself.

### Pressure 4: App settings and runtime settings should stay semantically separate

The current `repositories` special-case loader is strong evidence that not everything belongs in profiles.

The future model should therefore avoid the opposite mistake of shoving every app setting into a profile blob.

A cleaner rule is:

- app/bootstrap settings belong in an app block,
- runtime AI settings belong in profiles.

### Pressure 5: The current user docs are harder than they need to be

The current docs already do a good job explaining the system, but they still need multiple pages to separate:

- hidden base inference settings,
- direct config section loading,
- engine-profile registries,
- local project override files,
- and runtime switching.

That amount of explanation is a sign the model is still richer than necessary for common workflows.

## What Must Be Preserved In Any Redesign

Any new format should preserve the following properties.

### A. Declarative layered discovery

Keep using Glazed config plans.

Why:

- they are explicit,
- testable,
- already adopted,
- and preserve provenance metadata.

### B. Provenance-aware debugging

The new model should still let us answer:

- which file or layer selected the profile?
- where did the active registry list come from?
- which inline profile definition won?
- how did the final settings get built?

The current `ResolvedConfigFile` and parse-step metadata work is too useful to abandon.

### C. Runtime profile switching safety

Do not lose the ability to preserve a non-profile base for later switching.

### D. External registry compatibility

Do not throw away the existing `engineprofiles` investment. YAML and SQLite registries are still valuable as importable catalogs.

### E. App/runtime separation

Do not force `repositories` or other app-owned bootstrap settings into profile payloads.

### F. Clear migration path

There are existing config files, examples, tests, and user habits built around:

- top-level runtime sections such as `ai-chat`,
- `profile-settings.profile`,
- `profile-settings.profile-registries`,
- and `.pinocchio-profile.yml` local overrides.

The redesign should therefore include compatibility and migration phases rather than a one-step break.

## Recommended Direction From This Analysis

The best direction is:

1. keep the current layered config-plan loader,
2. introduce one typed unified config document,
3. keep external registries as optional imported catalogs,
4. move top-level runtime section config into inline profiles,
5. keep app settings in a separate app block,
6. and migrate local override files to the same unified document shape.

In practice, that means replacing the current split model with a document shaped roughly like:

```yaml
app:
  repositories:
    - ~/prompts

profile:
  active: assistant
  registries:
    - ~/.pinocchio/profiles.yaml

profiles:
  default:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini

  assistant:
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5
```

That direction keeps the current engine-profile machinery while making the main config story much simpler.

## Key Insight

The decisive insight from the current architecture is this:

> The recent cleanup already solved the *file resolution* problem. The next ticket should solve the *document semantics* problem.

## References

### Core code

- `glazed/pkg/config/plan.go`
- `glazed/pkg/config/plan_sources.go`
- `glazed/pkg/doc/topics/27-declarative-config-plans.md`
- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/config_loading.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/profile_registry.go`
- `geppetto/pkg/sections/profile_sections.go`
- `geppetto/pkg/sections/sections.go`
- `geppetto/pkg/engineprofiles/registry.go`
- `geppetto/pkg/engineprofiles/types.go`
- `geppetto/pkg/engineprofiles/source_chain.go`
- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/repositories.go`
- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/cmd/pinocchio/cmds/js.go`

### Existing docs/examples

- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
- `geppetto/pkg/doc/tutorials/09-migrating-cli-commands-to-glazed-bootstrap-profile-resolution.md`
- `pinocchio/examples/js/profiles/basic.yaml`
- `geppetto/examples/js/geppetto/profiles/20-team-agent.yaml`
