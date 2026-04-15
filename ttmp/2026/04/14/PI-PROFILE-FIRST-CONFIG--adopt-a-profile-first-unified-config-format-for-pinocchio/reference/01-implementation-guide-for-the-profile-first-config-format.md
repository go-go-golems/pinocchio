---
Title: Implementation guide for the profile-first config format
Ticket: PI-PROFILE-FIRST-CONFIG
Status: active
Topics:
    - config
    - pinocchio
    - profiles
    - design
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/config.go
      Note: Implementation guide explains how AppBootstrapConfig may need a new document-aware seam
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/engine_settings.go
      Note: Implementation guide preserves the current base-plus-profile merge path for the first migration phase
    - Path: ../../../../../../../geppetto/pkg/engineprofiles/source_chain.go
      Note: Implementation guide reuses the current imported-registry chain instead of inventing another resolution engine
    - Path: ../../../../../../../glazed/pkg/config/plan.go
      Note: Implementation guide references the current resolved-file and plan APIs that should remain unchanged
    - Path: cmd/web-chat/main.go
      Note: Implementation guide highlights web-chat as a sensitive consumer of profile selection and registry access
    - Path: pkg/cmds/cmd.go
      Note: Implementation guide calls out runtime-switching and command bootstrap behaviors that must not regress during migration
    - Path: pkg/configdoc/load.go
      Note: First implementation tranche adds strict YAML decode with KnownFields and document validation
    - Path: pkg/configdoc/load_test.go
      Note: First implementation tranche adds focused decode and old-format rejection tests
    - Path: pkg/configdoc/types.go
      Note: First implementation tranche adds the typed unified config document structs and local filename policy
ExternalSources: []
Summary: |
    Step-by-step implementation guide for adding the new profile-first unified config format, written for an unfamiliar intern and organized by responsibility, phases, key APIs, merge rules, file targets, testing, and review strategy.
LastUpdated: 2026-04-14T22:55:00-04:00
WhatFor: |
    Provide a concrete coding guide for a future implementation pass, with enough orientation that a new contributor can work safely without rediscovering the current architecture from scratch.
WhenToUse: Use this document when implementing the new config format or reviewing PRs that introduce the unified document loader, inline profile catalog, or breaking-change migration tooling.
---



# Implementation guide for the profile-first config format

## Goal

This guide explains how to implement the proposed profile-first Pinocchio config format safely and incrementally.

It is written for a new intern who may be comfortable with Go but unfamiliar with:

- Glazed config plans,
- Geppetto bootstrap,
- engine-profile registries,
- Pinocchio runtime switching,
- and the recent config cleanup work.

The goal is not merely to list files to edit. The goal is to make the system understandable enough that future changes still respect the current architecture.

## Context

The current system has already solved the *layered discovery* problem. File resolution is explicit and reusable. The next job is to change the *document shape* and the *runtime interpretation* of config files.

The proposed format is:

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

This should eventually replace the need to put runtime section config such as `ai-chat` at the top level of ordinary config files.

## First Principles

Before touching code, internalize these rules.

### Rule 1: Do not rewrite file discovery

Keep using:

- `glazed/pkg/config/plan.go`
- `glazed/pkg/config/plan_sources.go`

The new work should reuse the existing plan machinery and its precedence metadata.

### Rule 2: Do not delete the engine-profile registry subsystem

Keep using:

- `geppetto/pkg/engineprofiles/*`

The new inline `profiles` block should *bridge into* that subsystem, not replace it.

### Rule 3: App settings and runtime settings are different

- `app.repositories` is not a profile.
- `profiles.*.inference_settings` is a profile.

Do not merge them into one untyped map.

### Rule 4: Prefer typed document loading over generic raw maps

The new config shape should be decoded into typed Go structs early.

### Rule 5: Preserve runtime switching invariants

Anything that changes profile resolution must still let Pinocchio rebuild active runtime state from a preserved non-profile baseline.

## System Map

This is the minimum architecture map you should understand before implementing.

### Glazed layer

Files:

- `glazed/pkg/config/plan.go`
- `glazed/pkg/config/plan_sources.go`
- `glazed/pkg/cmds/sources/load-fields-from-config.go`

Responsibility:

- discover files in precedence order,
- preserve file/layer/source metadata,
- optionally load resolved files into field/value history.

### Geppetto layer

Files:

- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/profile_registry.go`
- `geppetto/pkg/engineprofiles/*`

Responsibility:

- interpret app bootstrap config,
- resolve profile control-plane state,
- open and compose registries,
- merge selected profile onto base inference settings,
- create final engine settings.

### Pinocchio layer

Files:

- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`
- `pinocchio/pkg/cmds/profilebootstrap/repositories.go`
- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/cmd/pinocchio/cmds/js.go`

Responsibility:

- define concrete config policy,
- choose local filename conventions,
- own app-specific document schema,
- preserve app/runtime-switch behavior.

## Quick Reference

## Key current APIs

### File discovery and provenance

```go
type ResolvedConfigFile struct {
    Path       string
    Layer      ConfigLayer
    SourceName string
    SourceKind string
    Index      int
}
```

Used by:

- `glazed/pkg/config/plan.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`

### Bootstrap config contract

```go
type AppBootstrapConfig struct {
    AppName           string
    EnvPrefix         string
    ConfigFileMapper  sources.ConfigFileMapper
    NewProfileSection func() (schema.Section, error)
    BuildBaseSections func() ([]schema.Section, error)
    ConfigPlanBuilder ConfigPlanBuilder
}
```

Important note:

- this contract may need a new typed-document seam because `ConfigFileMapper` is no longer the natural abstraction once runtime settings live under `profiles`.

### Profile registry interfaces

```go
type ResolveInput struct {
    RegistrySlug      RegistrySlug
    EngineProfileSlug EngineProfileSlug
}

type ResolvedEngineProfile struct {
    RegistrySlug      RegistrySlug
    EngineProfileSlug EngineProfileSlug
    InferenceSettings *settings.InferenceSettings
    StackLineage      []ResolvedProfileStackEntry
    Metadata          map[string]any
}
```

### Current profile control plane

```go
type ProfileSettings struct {
    Profile           string   `glazed:"profile"`
    ProfileRegistries []string `glazed:"profile-registries"`
}
```

The new document should conceptually map:

- `profile.active` -> `Profile`
- `profile.registries` -> `ProfileRegistries`

## Recommended new types

These are suggested starting points, not fixed law.

```go
package configdoc

type Document struct {
    App      AppBlock                  `yaml:"app"`
    Profile  ProfileBlock              `yaml:"profile"`
    Profiles map[string]*InlineProfile `yaml:"profiles"`
}

type AppBlock struct {
    Repositories []string `yaml:"repositories,omitempty"`
}

type ProfileBlock struct {
    Active     string   `yaml:"active,omitempty"`
    Registries []string `yaml:"registries,omitempty"`
}

type InlineProfile struct {
    DisplayName       string                             `yaml:"display_name,omitempty"`
    Description       string                             `yaml:"description,omitempty"`
    Stack             []engineprofiles.EngineProfileRef  `yaml:"stack,omitempty"`
    InferenceSettings *settings.InferenceSettings        `yaml:"inference_settings,omitempty"`
    Extensions        map[string]any                     `yaml:"extensions,omitempty"`
}
```

## Suggested package plan

### New Pinocchio package

Recommended new package:

- `pinocchio/pkg/configdoc`

Suggested files:

- `types.go` — typed document structs
- `load.go` — YAML decode helpers
- `merge.go` — layered document merge logic
- `profiles.go` — inline profile conversion helpers
- `provenance.go` — optional provenance structures for explain/debug
- `*_test.go` — focused unit tests

### Possible Geppetto additions

Only extract to Geppetto what is genuinely generic:

- helper to build a composite registry from imported registries plus a synthetic inline registry
- maybe a new bootstrap seam for document-derived profile control and inline catalog input

Do **not** prematurely move the entire unified config document into Geppetto. The app block and local filename policy are Pinocchio-specific.

## Recommended Phase Order

## Phase 1: Add a typed config document package

### Objective

Create a safe place to define, decode, and merge the new format without immediately changing command bootstrap.

### Files to add

- `pinocchio/pkg/configdoc/types.go`
- `pinocchio/pkg/configdoc/load.go`
- `pinocchio/pkg/configdoc/merge.go`
- `pinocchio/pkg/configdoc/types_test.go`
- `pinocchio/pkg/configdoc/merge_test.go`
- `pinocchio/pkg/configdoc/load_test.go`

### What to implement

1. typed YAML decode with strict field checking
2. validation for obvious structural mistakes
3. layered merge logic
4. explicit rejection of old config shapes and legacy local filenames

### Pseudocode

```go
func LoadDocument(path string) (*Document, error) {
    raw := readYAML(path)
    doc := &Document{}
    if err := yaml.Unmarshal(raw, doc); err != nil {
        return nil, err
    }
    return doc, nil
}

func MergeDocuments(low, high *Document) *Document {
    // last writer wins for scalars
    // app.repositories appends in layer order with dedupe
    // profile.registries replaces as a control-plane list
    // merge same-slug profiles field-by-field
}
```

### Review checklist

- no bootstrap code changed yet
- tests fully describe merge rules
- old-format rejection is explicit and tested

## Phase 2: Add an inline-profile registry bridge

### Objective

Convert merged inline profiles into something Geppetto already knows how to resolve.

### Best approach

Build a synthetic `engineprofiles.EngineProfileRegistry` from inline `profiles`.

### Why

That lets current registry resolution, stack resolution, and inference-settings merge logic stay reusable.

### Suggested functions

```go
func InlineProfilesToRegistry(doc *Document, slug engineprofiles.RegistrySlug) (*engineprofiles.EngineProfileRegistry, error)
func BuildInlineRegistryChain(doc *Document, imported engineprofiles.Registry) (engineprofiles.Registry, error)
```

### Pseudocode

```go
func InlineProfilesToRegistry(doc *Document) (*engineprofiles.EngineProfileRegistry, error) {
    reg := &engineprofiles.EngineProfileRegistry{
        Slug:     engineprofiles.MustRegistrySlug("config-inline"),
        Profiles: map[engineprofiles.EngineProfileSlug]*engineprofiles.EngineProfile{},
    }
    for rawSlug, p := range doc.Profiles {
        slug := parseProfileSlug(rawSlug)
        reg.Profiles[slug] = &engineprofiles.EngineProfile{
            Slug:              slug,
            DisplayName:       p.DisplayName,
            Description:       p.Description,
            Stack:             p.Stack,
            InferenceSettings: clone(p.InferenceSettings),
            Extensions:        deepCopy(p.Extensions),
        }
    }
    return reg, nil
}
```

### Precedence rule to implement

Inline profiles should win over imported profiles when the same slug is resolved without an explicit registry slug.

### Review checklist

- no duplicate slug ambiguity left unresolved
- imported registries still behave the same
- inline registry precedence is clearly documented in tests

## Phase 3: Add a document-first resolver

### Objective

Replace the current “file-by-file config mapper into runtime sections” path with a document-first resolution flow.

### Why this phase exists

This is the architectural center of the migration.

The new format cannot be handled purely by `ConfigFileMapper` because runtime settings live inside named profiles and require whole-document state.

### Suggested new return type

```go
type ResolvedUnifiedConfig struct {
    Files         []config.ResolvedConfigFile
    Report        *config.PlanReport
    EffectiveDoc  *configdoc.Document
    AppSettings   configdoc.AppBlock
    ProfileActive string
    Registries    []string
}
```

### Suggested new helpers

Possibly in Pinocchio first:

```go
func ResolveUnifiedConfig(parsed *values.Values) (*ResolvedUnifiedConfig, error)
func ResolveUnifiedProfileSelection(parsed *values.Values) (*ResolvedUnifiedProfileSelection, error)
```

### Implementation notes

- still call the existing plan builder
- resolve files first
- load and merge the effective document
- only then derive profile control-plane state and inline profiles

## Phase 4: Fold repositories into the unified document

### Objective

Remove the special-case repository loader path.

### Current file to replace

- `pinocchio/pkg/cmds/profilebootstrap/repositories.go`

### Future behavior

`app.repositories` should be loaded from the same layered document as the rest of Pinocchio config.

### Why this matters

This is one of the cleanest simplifications the new format can deliver.

After this change, the statement becomes true:

> One layered config document controls both app settings and runtime profile configuration.

## Phase 5: Integrate with Geppetto bootstrap

### Objective

Make `ResolveCLIProfileSelection(...)` and `ResolveCLIEngineSettings(...)` consume the new document-derived control plane and inline profile catalog.

### Possible approaches

#### Option A: add a second bootstrap seam

Keep `AppBootstrapConfig` mostly intact, but add a new document-aware path for apps that need it.

Example direction:

```go
type AppBootstrapConfig struct {
    ...
    ResolveUnifiedConfig func(parsed *values.Values) (*ResolvedUnifiedConfig, error)
}
```

#### Option B: generalize bootstrap away from `ConfigFileMapper`

This is architecturally cleaner long-term, but more invasive.

### Recommendation

Start with Option A for lower migration risk, then collapse old/new seams after the format migration stabilizes.

## Phase 6: Migrate runtime consumers

### Pinocchio command/runtime consumers to inspect carefully

- `pinocchio/pkg/cmds/cmd.go`
- `pinocchio/cmd/web-chat/main.go`
- `pinocchio/cmd/pinocchio/cmds/js.go`
- `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`

### What to preserve

- hidden base settings logic
- profile-free baseline reconstruction for runtime switching
- JS runtime bootstrap behavior
- web-chat registry access and current-profile APIs

### What should change

- profile selection should be able to come from inline profiles in the main config doc
- runtime config should no longer depend on top-level `ai-chat` in config files once migration is complete

## Phase 7: Breaking-change handling and migration tooling

### Old inputs should fail loudly

Reject rather than reinterpret:

- top-level runtime sections such as `ai-chat`
- top-level `profile-settings`
- old local filename `.pinocchio-profile.yml`

### Filename policy

Canonical local filename:

- `.pinocchio.yml`

Recommended behavior:

- load only the new filename in the new runtime path
- emit a clear error when the old filename is encountered

### Optional migration tool

If migration assistance is needed, add a one-shot command such as:

```text
pinocchio config migrate
```

That command can rewrite old config into the new schema without forcing the runtime implementation to support both formats.

## Diagrams

## Current runtime path

```text
resolved config files
  -> ConfigFileMapper per file
  -> direct runtime section parsing
  -> hidden base inference settings
  -> profile-settings selects external registries
  -> external registry resolves engine profile
  -> final settings
```

## Target runtime path

```text
resolved config files
  -> load typed documents
  -> merge one effective unified document
  -> extract app block + profile control plane + inline profiles
  -> load imported registries
  -> create synthetic inline registry
  -> compose final registry view
  -> resolve selected profile
  -> merge selected profile onto base inference settings
  -> final settings
```

## Validation Checklist

Use this checklist during implementation.

### Unit-test checklist

- [ ] document decode works for new format
- [ ] document merge preserves layer precedence
- [ ] same-slug profile overlay works as designed
- [ ] old top-level runtime sections fail clearly
- [ ] old `profile-settings` fails clearly
- [ ] old local filename fails clearly
- [ ] synthetic inline registry resolves profiles correctly
- [ ] inline profiles override imported same-slug profiles

### Integration-test checklist

- [ ] repo-local config can select an inline profile
- [ ] cwd-local config can override repo-selected profile
- [ ] explicit `--config-file` can override repo/cwd `profile.active`
- [ ] `app.repositories` is resolved from the unified document
- [ ] JS runtime bootstrap can resolve inline profiles
- [ ] web-chat can list/use inline profiles through the composed registry path
- [ ] runtime switching still rebuilds from preserved base settings

### Documentation checklist

- [ ] Pinocchio user docs teach the new schema
- [ ] Geppetto migration tutorial no longer centers the old top-level runtime config shape
- [ ] examples include both inline-only and imported-registry-plus-inline cases
- [ ] breaking-change migration guidance is documented clearly

## Usage Examples

### Example 1: inline-only local project profile

```yaml
app:
  repositories:
    - ~/prompts

profile:
  active: local-dev

profiles:
  local-dev:
    inference_settings:
      chat:
        api_type: openai-compatible
        engine: llama-local
```

### Example 2: team registry plus local same-slug override

```yaml
profile:
  active: assistant
  registries:
    - ~/.pinocchio/team-profiles.yaml

profiles:
  assistant:
    inference_settings:
      chat:
        engine: gpt-5-mini
```

Expected behavior:

- imported registry supplies the broader `assistant` shape,
- local inline profile overrides the engine field,
- inline profile has highest same-slug precedence.

### Example 3: app settings plus imported profile catalog

```yaml
app:
  repositories:
    - ~/prompts
    - ~/team/prompts

profile:
  active: analyst
  registries:
    - ~/.pinocchio/team-profiles.yaml
```

This is the “team shared catalog” use case.

## What To Read Before Coding

Read these in order:

1. `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
2. `geppetto/pkg/cli/bootstrap/profile_selection.go`
3. `geppetto/pkg/cli/bootstrap/engine_settings.go`
4. `geppetto/pkg/cli/bootstrap/profile_registry.go`
5. `geppetto/pkg/engineprofiles/registry.go`
6. `geppetto/pkg/engineprofiles/types.go`
7. `geppetto/pkg/engineprofiles/source_chain.go`
8. `pinocchio/pkg/cmds/profilebootstrap/repositories.go`
9. `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
10. this ticket’s analysis and design docs

## Common Mistakes To Avoid

1. **Do not implement the new format as another raw `map[string]any` pipeline.**
   Use typed structs.

2. **Do not bypass the registry resolver for inline profiles.**
   Reuse `engineprofiles` by bridging into it.

3. **Do not silently invent precedence.**
   Write tests first for same-slug profile merges and inline-versus-imported precedence.

4. **Do not break runtime switching in the same refactor.**
   Preserve the existing base/profile separation first.

5. **Do not quietly support old and new local filenames in the runtime path at the same time.**

## Related

- `analysis/01-current-profile-config-and-registry-architecture-analysis.md`
- `design-doc/01-profile-first-unified-config-format-and-migration-design.md`
- `reference/02-investigation-diary.md`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
