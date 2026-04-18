---
Title: Profile-first unified config format and migration design
Ticket: PI-PROFILE-FIRST-CONFIG
Status: active
Topics:
    - config
    - pinocchio
    - profiles
    - design
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_registry.go
      Note: Current external-registry bootstrap helper that should evolve toward imported-catalog composition
    - Path: ../../../../../../../geppetto/pkg/engineprofiles/registry.go
      Note: The proposed inline-profile bridge intentionally reuses the existing registry abstraction instead of bypassing it
    - Path: ../../../../../../../geppetto/pkg/engineprofiles/types.go
      Note: Defines the engine profile and registry shapes that the new inline profiles should mirror closely
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Current local file policy and mapper-driven bootstrap path that the design proposes to replace with a document-first path
    - Path: pkg/cmds/profilebootstrap/repositories.go
      Note: Current separate app-settings loader that the design proposes to fold into app.repositories in the unified document
    - Path: pkg/doc/topics/webchat-profile-registry.md
      Note: Shows how web-chat currently depends on external profile registries and why imported registries must remain supported
ExternalSources: []
Summary: |
    Proposed target architecture for a unified Pinocchio config document that contains app settings, profile selection, and inline profiles while keeping external engine-profile registries as optional imported catalogs and preserving the existing layered Glazed config-plan discovery model.
LastUpdated: 2026-04-14T22:55:00-04:00
WhatFor: |
    Act as the primary implementation reference for the future config-format migration, including schema, responsibilities, merge semantics, breaking-change rollout, testing, and implementation phases.
WhenToUse: Use this document when implementing or reviewing the profile-first config redesign or when deciding how inline profiles should interact with external profile registries and local config layers.
---


# Profile-first unified config format and migration design

## Executive Summary

This document proposes the next major simplification of Pinocchio configuration.

The loader should stay exactly where the recent cleanup left it:

- Glazed owns declarative layered config discovery.
- Geppetto owns reusable bootstrap and engine-profile resolution primitives.
- Pinocchio owns app-specific config policy and user-facing file conventions.

What should change is the config *document* shape.

The recommended future model is:

1. one layered Pinocchio config document,
2. one place for app settings,
3. one place for profile selection and imported catalogs,
4. one place for inline profiles,
5. and no long-term top-level runtime section config such as `ai-chat`.

The proposed target shape is:

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
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini

  assistant:
    display_name: Assistant
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5
```

This is a profile-first config model, not a “make everything a profile” model.

- `app` is still app-owned.
- `profile` is a control plane.
- `profiles` is where runtime AI behavior lives.
- external registries survive as imported catalogs, not as the only place profiles can exist.

## Problem Statement

The current model splits runtime behavior across two systems:

- top-level Geppetto section config in ordinary config files,
- and named engine-profile overlays loaded from external registries.

This creates several practical problems:

1. it is hard to explain to newcomers,
2. it makes local override files conceptually confusing,
3. it requires separate reasoning about config files and profile registries,
4. it keeps app settings (`repositories`) on a partly separate path,
5. and it leaves `ConfigFileMapper` doing work that becomes less natural once runtime settings are profile-based.

The goal is to simplify the user-facing model without discarding the strong parts of the current system.

## Design Goals

### Primary goals

1. **One layered config story**
   - User, repo, cwd, and explicit config should all participate in one coherent model.

2. **One user-facing document schema**
   - Users should not have to keep section-shaped config and registry-shaped YAML equally central in their heads.

3. **Profiles should own runtime AI settings**
   - Long-term runtime fields such as `ai-chat` should not live at the top level of the config document.

4. **App settings should stay separate from profiles**
   - `repositories` and similar non-runtime settings should not be forced into runtime profile payloads.

5. **External registries should remain valuable**
   - YAML and SQLite profile catalogs should stay supported for team/shared use cases.

6. **Migration should be explicit rather than compatibility-heavy**
   - The redesign may ship a migration guide or migration verb, but runtime compatibility shims are not required.

### Non-goals

1. **Do not replace Glazed config plans**
   - Layered file discovery is already in a good state.

2. **Do not remove Geppetto engine-profile resolution**
   - That subsystem already solves real problems well.

3. **Do not collapse app settings and runtime settings into one untyped blob**
   - The design should become simpler, not less structured.

4. **Do not force a new precedence model for CLI/env/runtime overrides unless necessary**
   - The format redesign should minimize unrelated semantic churn.

## Proposed Terms

To reduce confusion, the design should use the following language consistently.

### Unified config document

A layered Pinocchio YAML document containing:

- `app`
- `profile`
- `profiles`

### App settings

Non-profile Pinocchio settings such as:

- `repositories`
- future local host/server/bootstrap settings

### Profile control plane

Selection/import settings that determine *which* profile catalog is available and *which* profile is active:

- `profile.active`
- `profile.registries`

### Inline profiles

Named profile definitions in the unified document under:

- `profiles`

### Imported registries

External profile catalogs referenced through:

- `profile.registries`

These keep the current engine-profile registry shape and transport options.

## Proposed Target Schema

## 1. Top-level shape

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
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini

  assistant:
    display_name: Assistant
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5
```

## 2. `app` block

Purpose:

- application-owned settings that should not be treated as runtime AI profile payloads.

Initial contents:

```yaml
app:
  repositories:
    - ~/prompts
```

Future candidates may include:

- server/bootstrap defaults,
- local UI defaults,
- host-specific runtime settings that do not belong in engine profiles.

## 3. `profile` block

Purpose:

- select the active profile,
- specify external profile catalogs.

Proposed initial shape:

```yaml
profile:
  active: assistant
  registries:
    - ~/.pinocchio/profiles.yaml
```

Notes:

- `active` is the config-file equivalent of the current `profile-settings.profile`.
- `registries` is the config-file equivalent of the current `profile-settings.profile-registries`.
- CLI flags can keep their current names (`--profile`, `--profile-registries`) and be mapped into this control plane even if the file format itself is breaking.

## 4. `profiles` block

Purpose:

- define inline runtime profiles directly in the layered config document.

Recommended shape:

```yaml
profiles:
  default:
    display_name: Default
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini

  assistant:
    display_name: Assistant
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5
```

The key design choice is to make each inline profile structurally close to `engineprofiles.EngineProfile`.

That avoids inventing a second completely different profile shape.

### Recommended inline-profile contract

Each `profiles.<slug>` entry should support at least:

- `display_name`
- `description`
- `stack`
- `inference_settings`
- `extensions`
- optional metadata fields if useful later

Internally this should be easy to project into `engineprofiles.EngineProfile`.

## Why This Schema Is Better

### It matches how people think

Users naturally think in terms of:

- app settings,
- which profile is active,
- and what profiles exist.

They do not naturally think in terms of:

- top-level `ai-chat` section config,
- plus a second profile catalog somewhere else,
- plus a special-case app setting path.

### It makes local overrides obvious

A repo-local or cwd-local config file can now simply say:

```yaml
profile:
  active: local-dev

profiles:
  local-dev:
    inference_settings:
      chat:
        api_type: openai-compatible
        engine: llama-local
```

No separate registry file is required for common local workflows.

### It preserves external reuse

Team/shared registries still work:

```yaml
profile:
  active: analyst
  registries:
    - ~/.pinocchio/team-profiles.yaml
```

That keeps the existing engine-profile registry system valuable.

## Proposed Runtime Semantics

The safest first implementation is to preserve the current high-level runtime idea:

```text
base + selected profile overlay = final runtime settings
```

The difference is that the selected profile can now come from:

- inline `profiles`,
- imported registries,
- or both.

### Recommended resolution pipeline

```text
resolve layered files
  -> decode and merge unified config documents
  -> extract app settings
  -> extract profile control plane (active + registries)
  -> load imported registries
  -> build synthetic inline registry from merged profiles
  -> combine imported registries + inline registry into one chain
  -> resolve selected profile
  -> merge selected profile inference settings onto base inference settings
```

### Why preserve the current base-plus-profile idea first

This minimizes unrelated churn in:

- Geppetto bootstrap,
- runtime profile switching,
- web-chat startup,
- JS runtime bootstrap,
- and current engine construction.

A separate future ticket can revisit whether direct CLI/env runtime fields should become *post-profile manual overrides*. That is a valid question, but it is not required to land the new document format.

## Proposed Layering Rules

## 1. File-layer precedence remains unchanged

Low to high:

```text
system -> user -> repo -> cwd -> explicit
```

That continues to come from Glazed config plans.

## 2. Unified config document precedence

The unified document should be merged in the same low-to-high order as resolved files.

High-level rules:

- scalar values: last writer wins
- `app.repositories`: append in layer order, dedupe, preserve stable order
- `profile.active`: last writer wins
- `profile.registries`: later layer replaces earlier list
- `profiles.<slug>`: see below

## 3. Inline profile merge semantics

This is the most important merge-design choice.

### Recommendation

For the first implementation, merge same-slug inline profiles *field by field* across layers rather than replacing the whole profile entry.

Why:

- it matches the spirit of config overlays,
- it makes repo/cwd local tweaks ergonomic,
- it avoids forcing users to duplicate an entire profile definition just to change one field.

Recommended profile-level merge behavior:

- `display_name`, `description`: last writer wins
- `stack`: later layer replaces earlier stack entirely
- `inference_settings`: merge with the existing `MergeInferenceSettings` logic
- `extensions`: deep merge maps, last writer wins on scalar leaves
- metadata: last writer wins unless a clearer rule emerges during implementation

### Example

Low layer:

```yaml
profiles:
  assistant:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5
```

Higher repo-local layer:

```yaml
profiles:
  assistant:
    inference_settings:
      chat:
        engine: gpt-5-mini
```

Effective merged inline profile:

```yaml
profiles:
  assistant:
    inference_settings:
      chat:
        api_type: openai
        engine: gpt-5-mini
```

This is the ergonomic behavior users expect from layered config.

## Proposed Registry Composition Model

The registry system should survive, but its role changes.

### Current role

Registries are central to profile resolution.

### Proposed role

Registries become optional imported catalogs.

### Composition rule

Build a final profile catalog from:

1. imported registries from `profile.registries`
2. one synthetic inline registry built from merged `profiles`

### Precedence rule

The synthetic inline registry should have highest precedence for same-slug lookup.

That means local or user config can override imported shared profile slugs intuitively.

### Implementation sketch

```text
imported registry chain (existing Geppetto code)
  + synthetic inline registry (new)
  = final profile resolution registry
```

### Why a synthetic registry is the right bridge

It lets the new config document reuse the current engine-profile resolver instead of creating a second parallel profile-resolution engine.

## Proposed Canonical Local Filename

### Recommendation

Adopt:

- `.pinocchio.yml`

as the long-term canonical local override filename.

### Why

Once the local file can contain:

- `app`,
- `profile`,
- and `profiles`,

then `.pinocchio-profile.yml` becomes misleading. It implies the file is only about profile overlay, which would no longer be true.

### Breaking-change strategy

Treat the filename change as part of the explicit format cutover.

Suggested behavior:

- support `.pinocchio.yml` as the canonical filename,
- do not load `.pinocchio-profile.yml` in the new runtime path,
- fail with a clear error if the old filename is encountered in a context that now expects the new format,
- optionally provide a migration command to rewrite old files into the new filename and schema.

That keeps the runtime implementation simpler and avoids long-lived dual-format ambiguity.

## Proposed API Direction

The main architectural consequence of this design is that config loading becomes **document-first** rather than **file-by-file section-map first**.

### Why the current `ConfigFileMapper` seam is insufficient

Today, each config file can be mapped independently into section maps.

In the proposed model, runtime settings depend on the *effective merged document* and the *selected profile*, which may require imported registries.

That means the loader needs to do this:

```text
resolve files
  -> load documents
  -> merge documents
  -> resolve selected profile
  -> project results into app settings / profile settings / inference settings
```

not this:

```text
for each file:
  raw yaml -> section map
```

### Recommended new responsibilities

#### Glazed

Keep unchanged:

- file resolution plans,
- provenance-bearing `ResolvedConfigFile` values,
- plan reports.

#### Geppetto

Add generic helpers for:

- combining imported registries with an inline synthetic registry,
- resolving profile control-plane state from a document-derived structure,
- projecting selected profile results into bootstrap outputs.

#### Pinocchio

Own the unified document schema and migration.

This includes:

- typed config-document structs,
- document merge logic,
- app block decoding,
- strict rejection of old config shapes,
- local filename policy.

## Proposed Breaking-Change Rollout

A staged rollout is still recommended, but it should be explicit rather than compatibility-heavy.

### Phase 1: land the new format as the only supported runtime format

Support only:

- `app`
- `profile`
- `profiles`
- the new canonical local filename

Do not add runtime support for legacy top-level `ai-chat` style config, legacy `profile-settings`, or legacy `.pinocchio-profile.yml`.

### Phase 2: fail loudly and helpfully on old format

Emit clear validation errors when old-format files are encountered, including:

- top-level runtime sections such as `ai-chat`
- top-level `profile-settings`
- old local override filenames

### Phase 3: provide migration help outside the runtime path

If migration assistance is needed, provide it through:

- a migration guide,
- or a one-shot migration command such as `pinocchio config migrate`

That keeps the runtime implementation clean while still giving users a path forward.

## Risks

### Risk 1: merge semantics for inline profiles become too magical

Mitigation:

- document the merge rules explicitly,
- keep `stack` replacement behavior simple,
- add focused tests for same-slug layered merges.

### Risk 2: bootstrap layering becomes harder to reason about

Mitigation:

- preserve the current high-level base-plus-profile runtime model in phase 1,
- keep the document loader typed and explicit,
- provide explain/debug output for effective config document and effective profile catalog.

### Risk 3: provenance becomes weaker

Mitigation:

- keep `ResolvedConfigFile` and plan reports,
- add document-level provenance structures for `app`, `profile.active`, `profile.registries`, and `profiles.<slug>`.

### Risk 4: web-chat/runtime switching regressions

Mitigation:

- do not rewrite runtime switching semantics in the same change,
- reuse the current preserved-base pattern,
- add explicit switching tests for inline profiles and imported profiles.

### Risk 5: too much of the design becomes Pinocchio-specific

Mitigation:

- keep the unified document schema in Pinocchio,
- but extract reusable registry-composition helpers into Geppetto when they are genuinely generic.

## Alternatives Considered

## Alternative A: keep the current model and just improve docs

Rejected because the current user-facing model is genuinely more complex than necessary. Documentation can soften that complexity, but it does not remove it.

## Alternative B: make *all* config into profiles

Example rejected shape:

```yaml
profiles:
  default:
    repositories:
      - ~/prompts
    ai-chat:
      ...
```

Rejected because:

- app settings and runtime settings have different semantics,
- changing profiles should not quietly change unrelated app bootstrap state unless explicitly intended,
- current `repositories` special handling is evidence that an app/runtime split is real.

## Alternative C: keep top-level runtime config and make registries optional only

Rejected because it leaves the biggest teaching problem in place: two equally central runtime models.

## Alternative D: delete external registries entirely

Rejected because external registries still solve legitimate team/shared-catalog problems and already have a good generic implementation.

## Implementation Plan

### Phase 0: research output complete

This ticket itself is Phase 0.

Deliverables already written:

- current-state analysis
- design document
- implementation guide
- diary

### Phase 1: typed document model in Pinocchio

Add a typed schema package, for example:

- `pinocchio/pkg/configdoc`

Suggested initial types:

```go
type Document struct {
    App      AppBlock                 `yaml:"app"`
    Profile  ProfileBlock             `yaml:"profile"`
    Profiles map[string]*InlineProfile `yaml:"profiles"`

    LegacyRuntimeSections map[string]map[string]any `yaml:"-"`
}

type AppBlock struct {
    Repositories []string `yaml:"repositories"`
}

type ProfileBlock struct {
    Active     string   `yaml:"active"`
    Registries []string `yaml:"registries"`
}

type InlineProfile struct {
    DisplayName       string                 `yaml:"display_name,omitempty"`
    Description       string                 `yaml:"description,omitempty"`
    Stack             []engineprofiles.EngineProfileRef `yaml:"stack,omitempty"`
    InferenceSettings *settings.InferenceSettings       `yaml:"inference_settings,omitempty"`
    Extensions        map[string]any         `yaml:"extensions,omitempty"`
}
```

### Phase 2: document loader and merger

Add a typed loader that:

1. accepts `[]ResolvedConfigFile`,
2. loads each YAML document,
3. merges them in layer order,
4. keeps provenance per merged block/profile.

### Phase 3: synthetic inline registry bridge

Add logic that converts merged inline profiles into a synthetic `engineprofiles.EngineProfileRegistry`.

### Phase 4: bootstrap integration

Change Pinocchio bootstrap so runtime config is derived from the unified document rather than direct file-to-section mapping.

### Phase 5: migration tooling and failure messaging

Add clear errors, docs, and optional migration tooling rather than compatibility adapters.

### Phase 6: docs/examples migration

Update:

- Pinocchio docs
- Geppetto migration tutorial
- example configs
- local profile docs
- web-chat docs

### Phase 7: post-cutover cleanup

Remove any temporary migration-only helpers that are no longer needed once the new format and optional migration tooling are stable.

## Testing Strategy

### Unit tests

1. document decode tests
2. document merge tests
3. same-slug inline profile merge tests
4. old-format rejection tests
5. synthetic inline registry tests
6. imported-plus-inline registry precedence tests

### Integration tests

1. repo/cwd/explicit layered local config selection
2. profile resolution from inline profiles only
3. profile resolution from imported registries only
4. inline override of imported same-slug profile
5. app repositories loaded from unified document
6. runtime switching using preserved base plus resolved inline profile

### Command/package tests

At minimum revalidate:

- `geppetto/pkg/cli/bootstrap/...`
- `pinocchio/pkg/cmds/profilebootstrap/...`
- `pinocchio/pkg/cmds/...`
- `pinocchio/cmd/web-chat`
- `pinocchio/cmd/pinocchio/cmds/...`
- representative example commands

## Open Questions

1. Should the config-file control-plane key be `profile.registries` or should the new design already rename it to `profile.imports`?
   - Recommendation: keep `registries` in phase 1 for minimal churn; rename later if still desirable.

2. Should direct CLI runtime flags override profile payloads or continue acting as part of the base that a selected profile may override?
   - Recommendation: preserve current behavior in the format migration; revisit only in a separate semantics ticket.

3. Should the synthetic inline registry get a stable user-visible slug?
   - Recommendation: yes, but keep it internal unless debugging output needs it.

4. Should same-slug inline profiles merge field-by-field or be full replacement?
   - Recommendation: field-by-field, with explicit replacement for `stack`.

## References

### Primary references

- `glazed/pkg/config/plan.go`
- `glazed/pkg/config/plan_sources.go`
- `glazed/pkg/doc/topics/27-declarative-config-plans.md`
- `geppetto/pkg/cli/bootstrap/config.go`
- `geppetto/pkg/cli/bootstrap/profile_selection.go`
- `geppetto/pkg/cli/bootstrap/engine_settings.go`
- `geppetto/pkg/cli/bootstrap/profile_registry.go`
- `geppetto/pkg/engineprofiles/registry.go`
- `geppetto/pkg/engineprofiles/types.go`
- `geppetto/pkg/engineprofiles/source_chain.go`
- `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
- `pinocchio/pkg/cmds/profilebootstrap/repositories.go`
- `pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`

### Companion documents in this ticket

- `analysis/01-current-profile-config-and-registry-architecture-analysis.md`
- `reference/01-implementation-guide-for-the-profile-first-config-format.md`
- `reference/02-investigation-diary.md`
