---
Title: Project report and remaining Geppetto/Pinocchio legacy cleanup audit
Ticket: PI-CONFIGFILESFUNC-REMOVAL
Status: active
Topics:
    - config
    - glazed
    - pinocchio
    - geppetto
    - cleanup
    - api-design
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/engine_settings.go
      Note: Hidden-base inference settings path duplicates bootstrap config middleware assembly
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/inference_debug.go
      Note: Inference trace path duplicates bootstrap config middleware assembly
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_selection.go
      Note: Bootstrap config-loading path still carries dead FromFiles fallback logic and path-centric compatibility wrappers
    - Path: ../../../../../../../geppetto/pkg/sections/profile_sections.go
      Note: Legacy profile middleware builder duplicates bootstrap flow and should be retired with the older sections helper path
    - Path: ../../../../../../../geppetto/pkg/sections/sections.go
      Note: Legacy Cobra middleware builder duplicated bootstrap logic and still embeds pinocchio-specific config policy
    - Path: cmd/pinocchio/cmds/js.go
      Note: JS command still uses path-list config loading and rebuilds profile registry stack logic outside the main bootstrap path
    - Path: cmd/pinocchio/main.go
      Note: Repository-loading still manually parses layered YAML instead of reusing a typed helper over the plan path
    - Path: pkg/cmds/helpers/parse-helpers.go
      Note: Still exposes a UseViper-shaped manual config/env parser that bypasses the newer bootstrap path
ExternalSources: []
Summary: |
    Combined project report and cleanup audit for the declarative config-plan implementation and follow-up destructive cleanup work. Includes a concrete list of remaining legacy paths in Geppetto and Pinocchio, with file-level evidence and recommended cleanup sequence.
LastUpdated: 2026-04-14T23:40:00-04:00
WhatFor: Preserve the high-level project report in ticket docs and provide a concrete cleanup map for the remaining Geppetto/Pinocchio legacy seams.
WhenToUse: Use when reviewing what was accomplished by the config-plan project and when planning the next cleanup pass in Geppetto and Pinocchio.
---


# Project report and remaining Geppetto/Pinocchio legacy cleanup audit

## Executive summary

This document serves two purposes.

First, it copies the high-level project report for the declarative config-plan work into the ticket stream so the report does not live only in the Obsidian vault. Second, it audits the remaining active Geppetto and Pinocchio code paths for legacy seams that should still be cleaned up if the goal is to remove as much old architecture as possible.

The big picture is now good:

- the active workspace uses declarative config plans
- Pinocchio local profile loading works from git root and cwd
- provenance-aware config metadata survives into parsed field history
- old parser/config/Viper paths have been removed from the active Glazed + local Clay worktree
- Geppetto bootstrap is the primary active integration surface

The remaining work is no longer “finish the migration.” The remaining work is “remove duplicated or compatibility-shaped surfaces that are still hanging around in Geppetto and Pinocchio even though the new architecture is already in place.”

> [!summary]
> The current workspace is already functionally migrated. The highest-value remaining cleanup is:
> 1. delete the duplicated legacy Geppetto section middleware helpers
> 2. remove Pinocchio helper layers that still manually reconstruct config/env loading
> 3. collapse duplicated bootstrap config-loading logic inside Geppetto bootstrap
> 4. eliminate path-centric wrappers where resolved-file or plan-level APIs are now the real source of truth

## Part 1: Project report copied into the ticket stream

### What this project was actually about

The original feature request sounded narrow: support local Pinocchio profile files from the current working directory and the git repository root. But the real architecture problem was broader. The old stack still carried several implicit, path-centric, or Viper-era assumptions:

- hidden config path discovery helpers
- old Cobra parser hooks that returned only `[]string`
- compatibility facades like `pkg/appconfig`
- provenance that often only said “came from config” without explaining the config layer/source rule

Adding repo-local and cwd-local profile files without changing the architecture would have made the system harder to reason about.

The better answer was to replace hidden helper logic with a declarative layered plan model.

### What was implemented

Across Glazed, Geppetto, Pinocchio, and the local Clay worktree, the project delivered:

- a generic declarative config-plan API in Glazed
- plan sources such as:
  - `SystemAppConfig(...)`
  - `HomeAppConfig(...)`
  - `XDGAppConfig(...)`
  - `GitRootFile(...)`
  - `WorkingDirFile(...)`
  - `ExplicitFile(...)`
- provenance-aware config loading via `ResolvedConfigFile`
- standardized parse-step metadata:
  - `config_file`
  - `config_index`
  - `config_layer`
  - `config_source_name`
  - `config_source_kind`
- Geppetto bootstrap integration via `ConfigPlanBuilder`
- Pinocchio policy wiring for `.pinocchio-profile.yml`
- aggressive cleanup of old APIs:
  - `ConfigFilesFunc`
  - `ConfigPath`
  - `pkg/appconfig`
  - `ResolveAppConfigPath(...)`
  - local `clay.InitViper(...)`
- docs and runnable examples for the new plan model

### Architectural ownership after the refactor

The resulting ownership boundaries are clearer than before:

- **Glazed** owns generic config-plan primitives and config-loading middleware.
- **Geppetto** owns bootstrap logic: profile selection, hidden base settings, inference trace/debug.
- **Pinocchio** owns app-specific policy such as local file names and precedence.
- **Clay** is no longer allowed to drag the active workspace back into the old Viper logger/config bootstrap path.

### Final Pinocchio precedence model

The effective low → high precedence order is now:

```text
system -> home -> xdg -> repo -> cwd -> explicit
```

That is visible in code as explicit policy instead of inferred from helper order.

### Key commits already landed

#### Glazed
- `b9628f7` — add declarative config plan primitives
- `0bf7314` — add resolved config file metadata
- `2088c59` — docs and example for declarative config plans
- `0e0f443` — switch Cobra config loading to plans
- `c850f23` — remove `pkg/appconfig`
- `a94d873` — remove legacy app config resolver
- `f13b8df` — add config plan middleware wrappers
- `fcfe018` — teach config plan middleware in docs

#### Geppetto
- `ce7f03d` — integrate declarative config plans
- `8ef6188` — require config plans

#### Pinocchio
- `56bb1f6` — layered local profile plan
- `8765765` — drop no-op parser shims
- `3118d0c` — repository config loading via plans

#### Clay
- `20a8a9d` — remove stale Viper logger dependency
- `84c0ae7` — remove local `InitViper` helper entirely

### Validation status

The active workspace path validated successfully after the Clay fix:

- Glazed config/sources/cli tests
- Geppetto bootstrap tests
- Pinocchio profilebootstrap tests
- Pinocchio command-package tests
- Clay package tests

At this point the migration is functionally done.

## Part 2: Remaining Geppetto/Pinocchio cleanup audit

This section is the actual forward-looking cleanup map.

The main conclusion is:

> the remaining legacy work is not about old deleted APIs still being used directly; it is about duplicated compatibility layers that re-implement pieces of the new architecture and should now be deleted or folded together.

I group the findings into **high-value deletions**, **medium-risk structural cleanup**, and **optional polish**.

---

## High-value deletion candidates

These are the best next cleanup targets because they remove real duplicated architecture rather than merely renaming things.

### 1. Delete Geppetto’s legacy Cobra middleware helpers

**Problem:** `geppetto/pkg/sections/sections.go` and `geppetto/pkg/sections/profile_sections.go` still contain large legacy Cobra middleware builders that duplicate bootstrap behavior, embed Pinocchio-specific config policy, and preserve backward-compatibility fallbacks that no longer match the desired architecture.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go:149-316`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/profile_sections.go:97-264`

**Example:**

```go
// GetCobraCommandGeppettoMiddlewares remains for legacy Cobra middleware
// wiring in existing Geppetto examples.
func GetCobraCommandGeppettoMiddlewares(...) ([]sources.Middleware, error) {
    // bootstrap command settings from Cobra + env + defaults
    // resolve config files once
    // bootstrap profile settings from config + env + Cobra + defaults
    // then rebuild the main middleware chain
}
```

**Why it matters:**
- This code duplicates logic now owned by `geppetto/pkg/cli/bootstrap`.
- It hides a second config-resolution story in Geppetto.
- It bakes Pinocchio-specific policy into a generic Geppetto package.
- It carries explicit backward-compatibility comments and fallback logic, which is exactly the old architecture we have been removing elsewhere.

**Cleanup sketch:**

```text
Step 1: inventory actual callers of GetCobraCommandGeppettoMiddlewares / GetProfileSettingsMiddleware
Step 2: migrate those callers to bootstrap.AppBootstrapConfig + explicit sections
Step 3: delete both helper functions entirely
Step 4: keep CreateGeppettoSections / NewProfileSettingsSection only as schema builders
```

**Priority:** highest

---

### 2. Remove Pinocchio policy leakage from Geppetto `pkg/sections`

**Problem:** even outside the legacy middleware builders, Geppetto `pkg/sections` still embeds Pinocchio-specific behavior like config paths and default profile registry discovery.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go:30-45`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/profile_sections.go:54-65`

**Example:**

```go
func resolvePinocchioConfigFiles(explicit string) ([]glazedConfig.ResolvedConfigFile, error) {
    plan := glazedConfig.NewPlan(...).Add(
        glazedConfig.SystemAppConfig("pinocchio"),
        glazedConfig.HomeAppConfig("pinocchio"),
        glazedConfig.XDGAppConfig("pinocchio"),
        glazedConfig.ExplicitFile(strings.TrimSpace(explicit)),
    )
}

func defaultPinocchioProfileRegistriesIfPresent() string {
    path := filepath.Join(configDir, "pinocchio", "profiles.yaml")
}
```

**Why it matters:**
- This is incorrect ownership: Geppetto should not know the app name `pinocchio` here.
- It makes deletion of the legacy middleware helpers harder because policy and generic schema code are mixed together.
- It blocks clean reuse of the sections package by non-Pinocchio callers.

**Cleanup sketch:**

```text
Move pinocchio-specific default policy helpers into pinocchio/pkg/cmds/profilebootstrap
or delete them entirely as part of deleting the legacy middleware builders.
Keep geppetto/pkg/sections focused on section construction only.
```

**Priority:** highest

---

### 3. Remove Pinocchio’s old helper-based manual layer parser

**Problem:** `pinocchio/pkg/cmds/helpers/parse-helpers.go` still exposes a Viper-shaped/manual parsing helper that reconstructs config loading outside the bootstrap path.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/parse-helpers.go:21-129`
- current active caller:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-chat/main.go:79-89`

**Example:**

```go
type GeppettoLayersHelper struct {
    Profile           string
    ProfileRegistries []string
    ConfigFile        string
    UseViper          bool
}

if helper.UseViper {
    configFiles, err := profilebootstrap.ResolveCLIConfigFiles(nil)
    ...
    sources.FromFile(configPath, ...)
    sources.FromEnv("PINOCCHIO", ...)
}
```

**Why it matters:**
- The type still literally carries `UseViper`, which is a dead conceptual model even though the implementation no longer uses Viper directly.
- It reconstructs config/env/default behavior manually instead of using the current bootstrap path.
- It encourages continued use of path lists + `FromFile(...)` instead of resolved files or plan middleware.
- It adds another Pinocchio-local parsing story that differs from Geppetto bootstrap.

**Cleanup sketch:**

```text
Migrate simple-chat example away from helpers.ParseGeppettoLayers
Replace ParseGeppettoLayers with direct profilebootstrap / bootstrap calls
Delete GeppettoLayersHelper, WithUseViper, and ParseGeppettoLayers
```

**Priority:** highest

---

## Medium-risk structural cleanup

These are worth doing after the high-value deletions because they simplify internal structure and reduce duplication.

### 4. Collapse duplicated config middleware assembly inside Geppetto bootstrap

**Problem:** Geppetto bootstrap currently repeats almost the same “resolve config files -> choose FromResolvedFiles -> execute env/config/defaults” logic in multiple places.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go:67-88`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/engine_settings.go:37-58`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/inference_debug.go:88-109`

**Example:**

```go
configFiles, err := ResolveCLIConfigFilesResolved(cfg, parsed)
...
configMiddleware := sources.FromFiles(configFiles.Paths, ...)
if cfg.ConfigPlanBuilder != nil {
    configMiddleware = sources.FromResolvedFiles(configFiles.Files, ...)
}
```

**Why it matters:**
- It repeats subtle ordering and metadata behavior in three places.
- Future config-loading changes will need to be made in several functions.
- It still carries a dead fallback branch (`FromFiles`) even though `cfg.Validate()` already requires `ConfigPlanBuilder`.

**Cleanup sketch:**

```go
func resolveConfigMiddleware(cfg AppBootstrapConfig, parsed *values.Values) (sources.Middleware, *ResolvedCLIConfigFiles, error) {
    files, err := ResolveCLIConfigFilesResolved(cfg, parsed)
    if err != nil { return nil, nil, err }
    return sources.FromResolvedFiles(files.Files,
        sources.WithConfigFileMapper(cfg.ConfigFileMapper),
        sources.WithParseOptions(fields.WithSource("config")),
    ), files, nil
}
```

Then use that helper in `ResolveCLIProfileSelection`, `ResolveBaseInferenceSettings`, and `BuildInferenceTraceParsedValues`.

**Priority:** medium-high

---

### 5. Remove the dead path-list compatibility branch from Geppetto bootstrap

**Problem:** `AppBootstrapConfig.Validate()` requires `ConfigPlanBuilder`, but the bootstrap code still behaves as if plan-based config loading might be optional.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/config.go:23-41`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go:71-82`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/engine_settings.go:41-52`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/inference_debug.go:92-103`

**Example:**

```go
if c.ConfigPlanBuilder == nil {
    return errors.New("app bootstrap config: config plan builder is required")
}
```

followed by:

```go
configMiddleware := sources.FromFiles(...)
if cfg.ConfigPlanBuilder != nil {
    configMiddleware = sources.FromResolvedFiles(...)
}
```

**Why it matters:**
- This is dead conditional logic.
- It keeps the code looking more compatible than it really is.
- It nudges readers toward the wrong mental model (“maybe path-list config loading is still a valid bootstrap mode”).

**Cleanup sketch:**

```text
Delete the conditional and always build FromResolvedFiles from ResolveCLIConfigFilesResolved.
If path projections are still needed, derive Paths from Files only for return values.
```

**Priority:** medium-high

---

### 6. Remove or shrink path-centric wrapper APIs in `profilebootstrap`

**Problem:** Pinocchio `profilebootstrap` still exports path-centric wrappers that encourage callers to think in `[]string` again.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go:61-70`
- active callers:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go:108-124`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/parse-helpers.go:78-98`

**Example:**

```go
func ResolveCLIConfigFiles(parsed *values.Values) ([]string, error)
func ResolveCLIConfigFilesForExplicit(explicit string) ([]string, error)
func MapPinocchioConfigFile(rawConfig interface{}) ...
```

**Why it matters:**
- These wrappers are not wrong, but they keep a path-list API alive after the rest of the architecture moved to resolved files and plans.
- They made it easy for the JS command and helper package to keep using `FromFiles(...)`/`FromFile(...)` instead of richer plan-aware loading.

**Cleanup sketch:**

```text
Add a Pinocchio-facing ResolveCLIConfigFilesResolved wrapper if needed.
Migrate active callers to resolved files or direct config plan middleware.
Then either:
- delete ResolveCLIConfigFiles / ResolveCLIConfigFilesForExplicit, or
- clearly mark them as compatibility-only and keep them internal.
```

**Priority:** medium

---

### 7. Refactor Pinocchio JS command to stop bypassing provenance-aware loading

**Problem:** `cmd/pinocchio/cmds/js.go` still resolves config files as plain paths and loads them through `FromFiles(...)`, losing the richer resolved-file provenance path that the rest of the stack now uses.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go:108-124`

**Example:**

```go
configFiles, err := profilebootstrap.ResolveCLIConfigFiles(parsedCommandSections)
...
cmd_sources.FromFiles(
    configFiles,
    cmd_sources.WithConfigFileMapper(profilebootstrap.MapPinocchioConfigFile),
)
```

**Why it matters:**
- The JS command is still living on the old path-centric side of the architecture.
- Its parsed history will not be as rich as it could be.
- It perpetuates `ResolveCLIConfigFiles(...)` as an active dependency.

**Cleanup sketch:**

```text
Option A:
- expose ResolveCLIConfigFilesResolved in profilebootstrap
- swap JS command to FromResolvedFiles(...)

Option B:
- use sources.FromConfigPlanBuilder(...) directly in jsCobraMiddlewares
- pull explicit path from parsed command settings just like the main profilebootstrap path
```

**Priority:** medium

---

### 8. Extract profile-registry-chain construction from Pinocchio JS runtime bootstrap

**Problem:** `cmd/pinocchio/cmds/js.go` contains its own registry stack builder even though it already calls `profilebootstrap.ResolveCLIEngineSettings(...)` first.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go:230-314`

**Example:**

```go
resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
...
profileRegistry, defaultResolve, registryCloser, err := loadPinocchioProfileRegistryStackFromSettings(...)
```

**Why it matters:**
- The JS path rebuilds registry-chain state after the main resolution path already handled related profile-selection logic.
- Validation and error semantics are duplicated.
- It makes the JS command a little too special.

**Cleanup sketch:**

```text
Promote a small helper into profilebootstrap or geppetto bootstrap:
BuildProfileRegistryReader(selection ProfileSettings) (...)

Then let both JS and any future runtime consumers share it.
```

**Priority:** medium

---

### 9. Replace manual YAML repository extraction in `cmd/pinocchio/main.go`

**Problem:** Pinocchio main now uses a declarative plan to find config files, but it still manually reads YAML and extracts the top-level `repositories` list in an ad hoc loop.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go:172-216`

**Example:**

```go
for _, file := range files {
    data, err := os.ReadFile(file.Path)
    ...
    var config map[string]interface{}
    if err := yaml.Unmarshal(data, &config); err != nil { ... }
    repos, ok := config["repositories"].([]interface{})
    ...
    repositoryPaths = next
}
```

**Why it matters:**
- This is an app-specific side-channel parser rather than a normal section/middleware path.
- It duplicates YAML reading and top-level extraction logic.
- It currently overwrites `repositoryPaths` each iteration rather than making the precedence rule explicit in a typed way.

**Cleanup sketch:**

```text
Option A: extract a tiny helper that resolves the pinocchio config plan and decodes only the repositories key
Option B: add a typed repository-config decoder that reuses the same config file mapper / precedence story
```

This is less urgent than the duplicated middleware helpers, but it is still an architectural cleanup opportunity.

**Priority:** medium

---

## Optional polish / low-risk cleanup

### 10. Reduce thin Pinocchio helper re-export surface

**Problem:** `pinocchio/pkg/cmds/helpers` contains several files that are now just pass-through wrappers around `profilebootstrap`.

**Where to look:**
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/profile_selection.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/profile_engine_settings.go`
- `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/profile_runtime.go`

**Why it matters:**
- These wrappers are not harmful, but they increase the number of public-looking paths through the same logic.
- Once `parse-helpers.go` is deleted or migrated, the rest of this helper package may be much less necessary.

**Cleanup sketch:**

```text
Inventory actual imports of pinocchio/pkg/cmds/helpers.
If only local code uses them, replace imports with profilebootstrap directly.
Delete wrappers that no longer buy clarity.
```

**Priority:** low-medium

---

## Recommended cleanup sequence

If the goal is to remove as much legacy surface as possible without destabilizing the new architecture, I would do the next pass in this order:

### Phase A: delete the worst duplicated legacy surfaces
1. Migrate `cmd/examples/simple-chat` off `helpers.ParseGeppettoLayers(...)`
2. Delete `pinocchio/pkg/cmds/helpers/parse-helpers.go`
3. Audit callers of `geppetto/pkg/sections.GetCobraCommandGeppettoMiddlewares(...)` and `GetProfileSettingsMiddleware(...)`
4. Migrate or delete those callers
5. Delete both Geppetto legacy middleware builders

### Phase B: simplify the active bootstrap core
6. Extract one shared config middleware helper in `geppetto/pkg/cli/bootstrap`
7. Remove dead `FromFiles(...)` fallback branches there
8. Consider shrinking `ResolvedCLIConfigFiles` to a resolved-first shape

### Phase C: remove path-centric holdouts
9. Migrate `cmd/pinocchio/cmds/js.go` away from `ResolveCLIConfigFiles(...) + FromFiles(...)`
10. Add or use a resolved-file/bootstrap helper instead
11. Delete or reduce the path-centric `profilebootstrap` wrappers if nothing meaningful still needs them

### Phase D: optional ownership cleanup
12. Move or delete Pinocchio-specific policy helpers currently living in Geppetto `pkg/sections`
13. Decide whether `cmd/pinocchio/main.go` repository-config loading deserves a typed decoder helper
14. Shrink remaining thin wrapper packages in Pinocchio if they no longer add value

## Practical “done vs not done” summary

### Done enough to trust the architecture
- declarative config plans are the active architecture
- Geppetto bootstrap is the main active integration path
- Pinocchio local profile policy is explicit and tested
- old removed APIs are gone from current active source

### Not yet cleaned to the minimum possible surface
- Geppetto still ships duplicated legacy section middleware helpers
- Pinocchio still ships a manual helper parser with a `UseViper` flag name
- some active callers still use path lists where resolved files/plans would be the cleaner interface
- a few thin wrapper surfaces remain that mostly forward into `profilebootstrap`

## Recommendation

The next best cleanup project is **not** another big feature. It is a focused destructive pass with a narrow goal:

> remove the duplicated Geppetto section middleware builders and the Pinocchio helper parser layer, then make the JS command consume the resolved-file/bootstrap path like the rest of the app.

That would leave the workspace with one honest config-loading story instead of one primary story plus a few compatibility-shaped side channels.
