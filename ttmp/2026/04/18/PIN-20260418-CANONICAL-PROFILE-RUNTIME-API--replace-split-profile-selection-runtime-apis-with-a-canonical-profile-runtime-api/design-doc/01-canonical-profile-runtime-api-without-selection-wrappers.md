---
Title: Canonical profile runtime API without selection wrappers
Ticket: PIN-20260418-CANONICAL-PROFILE-RUNTIME-API
Status: active
Topics:
    - pinocchio
    - geppetto
    - bootstrap
    - configuration
    - runtime
    - cli
    - design
    - cleanup
    - migration
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Replace the split selection/runtime bootstrap API surface with a single canonical runtime resolver and remove legacy wrapper APIs."
LastUpdated: 2026-04-18T16:50:00-04:00
WhatFor: "Plan the clean API that resolves profile intent, registry fallback, inline profiles, and final engine settings from one canonical runtime path."
WhenToUse: "Use when refactoring or reviewing Geppetto/Pinocchio profile bootstrap APIs and when validating that profile selection and runtime resolution cannot drift apart."
---

# Canonical profile runtime API without selection wrappers

## Executive Summary

The current bootstrap surface is split between a lightweight “selection” API and a heavier “runtime” API. That split lets different callers observe different slices of truth. A caller can see `profile=...` with an empty registry list from the selection API while a deeper runtime path still succeeds because inline profiles or implicit fallback sources are considered later.

This ticket replaces that split with one canonical runtime resolver per layer. Geppetto keeps a single registry-based runtime API. Pinocchio keeps a single unified-config runtime API that includes config documents, inline profiles, imported registries, and the composed registry chain. There is no backward compatibility layer, no legacy wrapper function, and no public “selection-only” resolver. Any lightweight view of profile state must be derived from the canonical runtime object.

## Problem Statement

### Current symptoms

1. **Two public stories for profile resolution**
   - Geppetto exposes `ResolveCLIProfileSelection(...)` and `ResolveCLIProfileRuntime(...)`.
   - Pinocchio exposes `ResolveCLIProfileSelection(...)`, `ResolveUnifiedConfig(...)`, `ResolveUnifiedProfileRegistryChain(...)`, and `ResolveCLIEngineSettings(...)`.

2. **Selection is weaker than runtime**
   - Selection reports only the effective profile slug and explicit registry list.
   - Runtime may additionally consider implicit default registry fallback and/or inline `profiles:` loaded from unified config documents.

3. **Callers can accidentally choose the wrong abstraction**
   - Tests and commands can validate the selection object and incorrectly conclude that runtime resolution cannot succeed.
   - The API shape itself invites drift because “selection” sounds authoritative even though it is not the full runtime contract.

4. **Pinocchio still carries wrapper-style aliases over Geppetto types**
   - `type ResolvedCLIProfileSelection = bootstrap.ResolvedCLIProfileSelection`
   - `type ResolvedCLIEngineSettings = bootstrap.ResolvedCLIEngineSettings`
   - helper functions that only forward into Geppetto plus extra Pinocchio-only logic elsewhere

### Why this is bad

- API consumers must memorize hidden semantics instead of reading them from the type system.
- Tests can accidentally bless an incomplete layer.
- Refactors are harder because there is no single canonical object to thread through the system.
- Wrapper aliases make ownership ambiguous and discourage clean boundaries.

## Goals

- Replace split selection/runtime APIs with one canonical runtime resolver in each layer.
- Remove public selection-only resolvers from the affected bootstrap surface.
- Remove Pinocchio wrapper aliases over Geppetto runtime/engine-setting structs.
- Preserve the actual desired behavior:
  - explicit profile selection
  - implicit `${XDG_CONFIG_HOME}/<app>/profiles.yaml` fallback for registry-based flows
  - inline `profiles:` from Pinocchio unified config documents
  - composed imported + inline registry resolution
- Keep inference debug output working without depending on a concrete engine-settings wrapper type.
- Update tests and docs to describe the new API directly.

## Non-Goals

- No backward compatibility shims.
- No deprecation wrappers.
- No attempt to keep `ResolveCLIProfileSelection(...)` or `ResolvedCLIProfileSelection` alive.
- No migration of unrelated webchat runtime policy APIs.

## Proposed Solution

## 1. Geppetto: one canonical registry-based runtime API

### New public shape

```go
type ResolvedCLIProfileRuntime struct {
    ProfileSettings     ProfileSettings
    ConfigFiles         []string
    ProfileRegistryChain *ResolvedProfileRegistryChain
    Close               func()
}
```

### Public entry points that remain

- `NewCLISelectionValues(...)`
- `ResolveCLIConfigFilesResolved(...)`
- `ResolveCLIProfileRuntime(...)`
- `ResolveCLIEngineSettings(...)`
- inference-debug helpers

### Public entry points removed

- `ResolveCLIProfileSelection(...)`
- `ResolveEngineProfileSettings(...)`
- `ResolvedCLIProfileSelection`

### Behavioral contract

`ResolveCLIProfileRuntime(...)` becomes the only public API that answers:
- what profile was selected
- which config files were used
- which registries are active after implicit fallback
- which registry chain is available for engine-profile resolution

There is no separate selection object. The canonical runtime object directly owns `ProfileSettings`.

## 2. Pinocchio: one canonical unified-config runtime API

### New public shape

```go
type ResolvedCLIProfileRuntime struct {
    ProfileSettings      ProfileSettings
    ConfigFiles          *ResolvedCLIConfigFiles
    Documents            *configdoc.ResolvedDocuments
    Effective            *configdoc.Document
    ProfileRegistryChain *bootstrap.ResolvedProfileRegistryChain
    Close                func()
}
```

### Public entry points that remain

- `BootstrapConfig()`
- `NewCLISelectionValues(...)`
- `ResolveCLIConfigFilesResolved(...)`
- `ResolveBaseInferenceSettings(...)`
- `ResolveCLIProfileRuntime(...)`
- `ResolveCLIEngineSettings(...)`
- `ResolveRepositoryPaths()`

### Public entry points removed

- `ResolveCLIProfileSelection(...)`
- `ResolveUnifiedConfig(...)`
- `ResolveUnifiedProfileRegistryChain(...)`
- `ResolveEngineProfileSettings(...)`
- Pinocchio type aliases over Geppetto selection/engine-setting structs

### Behavioral contract

Pinocchio’s canonical runtime API must directly answer:
- which config documents were resolved
- what the effective merged unified config document is
- which profile slug is active after unified config + explicit overrides
- which registry sources are active after implicit default fallback
- whether inline profiles participate in the final registry chain
- which registry chain is used for actual runtime resolution

Callers should never need to manually compose config resolution plus registry-chain resolution.

## 3. Engine settings structures stop carrying duplicate selection wrappers

### Geppetto

`ResolvedCLIEngineSettings` becomes:

```go
type ResolvedCLIEngineSettings struct {
    BaseInferenceSettings  *aisettings.InferenceSettings
    FinalInferenceSettings *aisettings.InferenceSettings
    ProfileRuntime         *ResolvedCLIProfileRuntime
    ResolvedEngineProfile  *gepprofiles.ResolvedEngineProfile
    ConfigFiles            []string
    Close                  func()
}
```

### Pinocchio

Pinocchio defines its own engine-settings result with the same clean intent instead of aliasing Geppetto’s struct.

## 4. Inference debug helpers accept a minimal resolution payload

Today inference-debug helpers depend on `*ResolvedCLIEngineSettings` even though they only need:
- final inference settings
- resolved engine profile

Replace that concrete dependency with a smaller dedicated input struct, for example:

```go
type ResolvedInferenceTrace struct {
    FinalInferenceSettings *aisettings.InferenceSettings
    ResolvedEngineProfile  *gepprofiles.ResolvedEngineProfile
}
```

That lets both Geppetto and Pinocchio pass clean engine-resolution results without cross-package aliasing.

## Design Decisions

### Decision: remove selection APIs entirely

**Why:** A public selection-only API invites misuse because it looks authoritative but is not the full runtime truth.

### Decision: keep Geppetto generic and Pinocchio app-aware

**Why:** Geppetto should own generic registry-based runtime resolution. Pinocchio should own unified config documents and inline profiles. The clean boundary is not “selection vs runtime”; it is “generic registry runtime vs app-specific unified-config runtime.”

### Decision: no backward compatibility

**Why:** The user explicitly requested a clean API with no wrappers. Retaining deprecated names would preserve the ambiguity we are trying to remove.

### Decision: inference-debug helpers should depend on data, not a package-specific result wrapper

**Why:** The debug path only needs resolved inference data, not a specific bootstrap result type.

## Alternatives Considered

### Alternative A: keep selection API and rename it to `ResolveCLIProfileIntent(...)`

This would improve naming but would still preserve two public resolution paths. Rejected because it does not fully eliminate drift.

### Alternative B: keep Pinocchio wrapper aliases and only enrich comments/docs

Rejected because the code shape would remain confusing and future callers would still choose the wrong abstraction.

### Alternative C: move Pinocchio unified config document semantics into Geppetto

Rejected for now. Inline Pinocchio config documents and repository metadata are app-owned concerns.

## Implementation Plan

### Phase 1: documentation + ticket scaffolding

- Create the ticket workspace.
- Add this design doc.
- Add a detailed task list.
- Add an implementation diary.

### Phase 2: Geppetto API cleanup

- Refactor `ResolveCLIProfileRuntime(...)` to own profile-settings resolution directly.
- Remove `ResolvedCLIProfileSelection` and related public helper functions.
- Update `ResolvedCLIEngineSettings` to carry only `ProfileRuntime`.
- Introduce the smaller inference-debug input type and update call sites.
- Update tests and docs in `geppetto/pkg/cli/bootstrap` and migration tutorials.

### Phase 3: Pinocchio API cleanup

- Replace exported unified-config helper split with one `ResolveCLIProfileRuntime(...)`.
- Make old exported config/registry-chain helpers private or remove them.
- Define Pinocchio-owned `ResolvedCLIEngineSettings` instead of aliasing Geppetto’s struct.
- Update command call sites (`cmd.go`, JS runner, web-chat, repository loading) to consume the canonical runtime API.

### Phase 4: tests and validation

- Update unit tests to validate the runtime object directly.
- Add/adjust tests for:
  - implicit default registry fallback
  - inline profile-only resolution
  - config-file precedence
  - engine settings still preserving final runtime behavior
- Run focused package tests in Geppetto and Pinocchio.
- Run a real Pinocchio smoke path with `PINOCCHIO_PROFILE=... --print-inference-settings`.

### Phase 5: docs + final cleanup

- Update affected public docs/tutorials to the new API names.
- Record the implementation in the ticket diary/changelog.
- Run `docmgr doctor`.

## API Sketches

### Geppetto before

```go
ResolveCLIProfileSelection(...)
ResolveCLIProfileRuntime(...)
ResolveCLIEngineSettings(...)
```

### Geppetto after

```go
ResolveCLIProfileRuntime(...)
ResolveCLIEngineSettings(...)
```

### Pinocchio before

```go
ResolveCLIProfileSelection(...)
ResolveUnifiedConfig(...)
ResolveUnifiedProfileRegistryChain(...)
ResolveCLIEngineSettings(...)
```

### Pinocchio after

```go
ResolveCLIProfileRuntime(...)
ResolveCLIEngineSettings(...)
ResolveRepositoryPaths(...)
```

## Validation Plan

### Geppetto

```bash
cd geppetto && go test ./pkg/cli/bootstrap -count=1
```

### Pinocchio focused

```bash
cd pinocchio && go test ./pkg/cmds/... ./cmd/pinocchio/... ./cmd/web-chat -count=1
```

### Pinocchio broad

```bash
cd pinocchio && go test ./... -count=1
cd pinocchio && go build -o /tmp/pinocchio-profile-runtime ./cmd/pinocchio
PINOCCHIO_PROFILE=gemini-2.5-pro /tmp/pinocchio-profile-runtime code professional hello --print-inference-settings
```

## Risks

- Public API breakage across Geppetto and Pinocchio callers.
- Hidden assumptions in tests or docs that still refer to `ProfileSelection`.
- Debug helper signature changes may affect more than one package.

## Review Checklist

- No public `ResolveCLIProfileSelection(...)` remains.
- No public `ResolvedCLIProfileSelection` remains.
- Pinocchio no longer aliases Geppetto engine-settings results.
- Engine settings and debug output still work.
- Implicit fallback and inline profile semantics are tested through the canonical runtime API.
