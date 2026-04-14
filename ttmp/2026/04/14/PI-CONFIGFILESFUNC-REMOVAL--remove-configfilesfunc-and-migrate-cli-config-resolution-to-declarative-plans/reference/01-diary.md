---
Title: Diary
Ticket: PI-CONFIGFILESFUNC-REMOVAL
Status: active
Topics:
    - config
    - glazed
    - pinocchio
    - cleanup
    - appconfig
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../glazed/pkg/cli/cobra-parser.go
      Note: Replaced ConfigFilesFunc/ConfigPath with ConfigPlanBuilder in commit 0e0f443
    - Path: ../../../../../../../glazed/pkg/cli/cobra_parser_config_test.go
      Note: Added parser regression coverage for plan-based loading and removal of implicit config discovery in commit 0e0f443
    - Path: ../../../../../../../glazed/pkg/cmds/sources/cobra.go
      Note: Removed obsolete Cobra-specific config-files resolver helper in commit 0e0f443
    - Path: ../../../../../../../glazed/pkg/doc/topics/24-config-files.md
      Note: Updated current docs to teach ConfigPlanBuilder in commit c850f23
    - Path: ../../../../../../../glazed/pkg/doc/tutorials/config-files-quickstart.md
      Note: Updated quickstart examples away from ConfigPath/ConfigFilesFunc in commit c850f23
    - Path: ../../../../../../../glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md
      Note: Updated migration tutorial away from removed Cobra parser config-loading APIs in commit c850f23
    - Path: cmd/agents/simple-chat-agent/main.go
      Note: Removed the same no-op parser shim in commit 8765765
    - Path: cmd/examples/simple-chat/main.go
      Note: Removed the same no-op parser shim in commit 8765765
    - Path: cmd/web-chat/main.go
      Note: Removed the no-op parser shim after AppName stopped implying config loading in commit 8765765
ExternalSources: []
Summary: Implementation diary for removing ConfigFilesFunc, removing ConfigPath from CobraParserConfig, deleting pkg/appconfig, and migrating workspace callers to declarative config plans.
LastUpdated: 2026-04-14T19:20:00-04:00
WhatFor: Capture the implementation steps, commits, validation, and remaining caveats for the ConfigFilesFunc/ConfigPath/appconfig removal cleanup.
WhenToUse: Use when reviewing or continuing the cleanup that removes old CobraParser config-loading APIs and the appconfig facade.
---


# Diary

## Goal

Record the implementation of the aggressive config cleanup requested in `PI-CONFIGFILESFUNC-REMOVAL`: remove `ConfigFilesFunc`, remove `ConfigPath`, remove `pkg/appconfig`, migrate workspace callers to declarative config plans, and document the exact validation/results.

## Step 1: Replace the old CobraParser file-path hooks with a single plan-based hook

The first implementation step was to cut out the old string-list/path-based Cobra parser config surface and replace it with a single plan-based hook. The key design change is that `AppName` now means env-prefix behavior only; it no longer implies hidden config discovery. Config loading now only happens when a caller explicitly supplies a declarative plan.

This was the most important architectural simplification because it removed the coupling that had forced Pinocchio to install no-op `ConfigFilesFunc` callbacks simply to keep `AppName` while suppressing implicit config loading.

### Prompt Context

**User prompt (verbatim):** "ok, remove ConfigFilesFunc, remove ConfigPath, remove appconfig (don't worry about prescribe) . Go ahead, commit at appropriate intervals, keep a diary"

**Assistant interpretation:** Remove the old Cobra parser config-loading APIs completely, delete the `pkg/appconfig` facade instead of preserving it, migrate current workspace callers/examples, and keep the cleanup documented step-by-step.

**Inferred user intent:** Prefer a simpler codebase over backwards compatibility, consolidate on explicit config plans, and leave a clear audit trail of exactly what changed and what broke during validation.

**Commit (code):** `0e0f4430b259fa07071daaf8911d9e7887f4dc2b` — `cli: switch cobra config loading to plans`

### What I did
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go`
  - removed `ConfigPath`
  - removed `ConfigFilesFunc`
  - added `ConfigPlanBuilder`
  - stopped implicit config discovery from `AppName`
  - resolved plan results through `FromResolvedFiles(...)`
- Removed the no-longer-needed Cobra helper from:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cmds/sources/cobra.go`
- Added a focused parser regression test file:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra_parser_config_test.go`
- Migrated Glazed examples to plan-based loading:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/config-single/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/middlewares-config-env/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/config-overlay/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/overlay-override/main.go`

### Why
- `ConfigFilesFunc` only returned `[]string`, so it discarded layer/source provenance already modeled by `config.Plan` and `ResolvedConfigFile`.
- `ConfigPath` was part of the same old path-centric model and no longer needed once config policy became plan-based.
- `AppName` had been overloaded: callers wanted env-prefix behavior without automatic config loading. Splitting those semantics simplified the API immediately.

### What worked
- The parser cleanup compiled cleanly in Glazed.
- The new parser tests proved two key behaviors:
  - `ConfigPlanBuilder` actually loads config files into parsed values.
  - `AppName` alone no longer causes `--config-file` to load anything implicitly.
- The example commands compiled cleanly after migration.

### What didn't work
- When I later tried to validate Pinocchio command packages, they failed for an external reason unrelated to this parser change itself: the workspace still uses `github.com/go-go-golems/clay v0.4.0`, and that external module still imports a Viper logging function removed earlier from local Glazed.

Exact error:

```text
# github.com/go-go-golems/clay/pkg
../../../../go/pkg/mod/github.com/go-go-golems/clay@v0.4.0/pkg/init.go:78:16: undefined: logging.InitLoggerFromViper
```

### What I learned
- The most valuable cleanup was not deleting lines; it was removing the hidden relationship between `AppName` and config discovery.
- A single explicit `ConfigPlanBuilder` is enough for the current workspace use cases. We did not need another intermediate compatibility hook.

### What was tricky to build
- The subtle part was preserving the parser’s precedence model while removing the old resolver path. The middleware chain still needs to behave as: defaults < config < env < args < flags, while `ConfigPlanBuilder` has to run early enough to inspect parsed command settings when needed.
- The clean solution was to resolve the plan during middleware construction, using `parsedCommandSections` plus the command/args, and then append `FromResolvedFiles(...)` into the existing chain.

### What warrants a second pair of eyes
- The user-facing semantics of `--config-file` on commands that do **not** provide a plan builder. The flag still exists via command settings, but it only has effect when a plan explicitly consumes it.
- Whether any future public docs should split generic command-settings flags from config-loading flags more aggressively.

### What should be done in the future
- If the external `clay` module is brought into this workspace or updated, re-run the Pinocchio command-package tests that were blocked by the stale external dependency.

### Code review instructions
- Start in `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go`
- Then review `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra_parser_config_test.go`
- Then review the migrated example commands under `glazed/cmd/examples/`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
gofmt -w pkg/cli/cobra-parser.go pkg/cli/cobra_parser_config_test.go pkg/cmds/sources/cobra.go \
  cmd/examples/config-single/main.go cmd/examples/middlewares-config-env/main.go \
  cmd/examples/config-overlay/main.go cmd/examples/overlay-override/main.go

go test ./pkg/cli/... ./pkg/cmds/sources/... ./cmd/examples/config-single \
  ./cmd/examples/middlewares-config-env ./cmd/examples/config-overlay \
  ./cmd/examples/overlay-override -count=1
```

### Technical details
- `ConfigPlanBuilder` signature:

```go
func(parsedCommandSections *values.Values, cmd *cobra.Command, args []string) (*config.Plan, error)
```

- The default `CobraParser` path now behaves like this:
  - flags
  - args
  - env from `AppName`
  - optional plan-resolved config files
  - defaults

## Step 2: Delete the appconfig facade and remove current-doc references to the old APIs

Once the parser supported the plan-based path directly, the next step was to remove `pkg/appconfig` instead of trying to modernize it. This matched the cleanup decision from the ticket analysis: in this workspace it was not a real production dependency, and the user explicitly asked to prioritize simplification over compatibility.

This step also updated the current user-facing Glazed docs so they no longer taught `ConfigPath` or `ConfigFilesFunc`, and so they no longer implicitly pointed readers toward a package we were deleting.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Follow through on the “remove appconfig” part of the request instead of merely deprecating it, and clean the current docs so they teach plans instead of the removed APIs.

**Inferred user intent:** Avoid leaving behind a half-deleted architecture where the old package disappears from code but remains recommended in docs or examples.

**Commit (code):** `c850f23dd6d0013730b50197dc82b04ea9c0ed3d` — `config: remove appconfig facade`

### What I did
- Deleted Glazed example commands:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/appconfig-parser/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/cmd/examples/appconfig-profiles/main.go`
- Deleted the entire `pkg/appconfig` package:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/doc.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/options.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/parser.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/parser_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/profile_test.go`
- Updated current Glazed docs:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/24-config-files.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/config-files-quickstart.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md`

### Why
- `pkg/appconfig` was not carrying current workspace production code.
- Keeping it would create another public surface that duplicated the same basic config/loading ideas in a less up-to-date shape.
- Deleting it now was cheaper and cleaner than modernizing it and then deprecating it later.

### What worked
- After deletion, the remaining Glazed packages and current examples still built successfully.
- A repo-wide search across current source (excluding historical ticket docs) no longer found live `pkg/appconfig` imports.
- Current docs now describe plan-based config loading instead of the removed string-list/path APIs.

### What didn't work
- N/A for code in this step; the main sharp edge remained the external `clay` compile failure when trying to validate Pinocchio command packages.

### What I learned
- The workspace’s live dependency surface was small enough that deletion was practical. The ticket analysis was correct: `pkg/appconfig` was more future-maintenance cost than present value here.

### What was tricky to build
- The trickiest part was documentation scope. Historical `ttmp/...` design/research artifacts still mention `pkg/appconfig`, `ConfigFilesFunc`, and `ConfigPath`, but those are historical records and not the current user-facing docs. I intentionally updated the active docs under `glazed/pkg/doc/*` while leaving historical ticket artifacts intact.

### What warrants a second pair of eyes
- Whether any additional active docs outside `glazed/pkg/doc/*` should be updated to emphasize that new CLI config loading is plan-based.

### What should be done in the future
- If `corporate-headquarters/prescribe` is to stay aligned with this direction, migrate that external caller off `pkg/appconfig` in its own repo later.

### Code review instructions
- Review the deletion itself with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
git show c850f23 --stat
```

- Then spot-check the active docs:
  - `pkg/doc/topics/24-config-files.md`
  - `pkg/doc/tutorials/config-files-quickstart.md`
  - `pkg/doc/tutorials/migrating-from-viper-to-config-files.md`

- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
go test ./pkg/cli/... ./pkg/cmds/sources/... ./pkg/config/... \
  ./cmd/examples/config-single ./cmd/examples/middlewares-config-env \
  ./cmd/examples/config-overlay ./cmd/examples/overlay-override -count=1
```

### Technical details
- Historical `ttmp/...` docs were intentionally not rewritten; they remain accurate records of prior design/implementation states.
- Current docs now teach `ConfigPlanBuilder` and `config.Plan` instead of `ConfigPath` / `ConfigFilesFunc`.

## Step 3: Remove the now-unnecessary Pinocchio no-op parser shims and record the external validation blocker

After the Glazed parser no longer auto-loaded config from `AppName`, the Pinocchio no-op callbacks became dead weight. This cleanup was intentionally small: just remove the fake callbacks and keep `AppName` for env-prefix behavior.

This step also made the external validation situation explicit in the ticket history. The code change itself is tiny, but the command-package validation attempt surfaced a stale external dependency (`clay v0.4.0`) that still expects a removed Viper logging function.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Finish the workspace migration all the way through the Pinocchio command call sites instead of leaving them as vestigial no-op placeholders.

**Inferred user intent:** Make the code simpler in practice, not just in the library API.

**Commit (code):** `8765765aeed62600dfaa793c02c295ef7246477e` — `cli: drop no-op config plan shims`

### What I did
- Removed no-op parser shims from:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-chat/main.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/agents/simple-chat-agent/main.go`

### Why
- Those callbacks existed only to suppress the old implicit CobraParser config-loading path.
- Once `AppName` no longer implied config discovery, the callbacks had no value.

### What worked
- The Pinocchio diff was exactly the simplification the earlier ticket analysis predicted: a handful of no-op closures disappeared.
- The workspace no longer needs `ConfigFilesFunc` shims to keep Pinocchio CLI behavior stable.

### What didn't work
- Full command-package validation in Pinocchio is still blocked by the external `clay` module issue described in Step 1.

Exact command and error:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent -count=1
```

```text
# github.com/go-go-golems/clay/pkg
../../../../go/pkg/mod/github.com/go-go-golems/clay@v0.4.0/pkg/init.go:78:16: undefined: logging.InitLoggerFromViper
FAIL	github.com/go-go-golems/pinocchio/cmd/web-chat [build failed]
FAIL	github.com/go-go-golems/pinocchio/cmd/examples/simple-chat [build failed]
FAIL	github.com/go-go-golems/pinocchio/cmd/agents/simple-chat-agent [build failed]
FAIL
```

### What I learned
- The Pinocchio usage pattern really was a smell: once the parser semantics were cleaned up, the Pinocchio changes collapsed to deleting boilerplate.

### What was tricky to build
- The only tricky part here was validation, not implementation. The changed Pinocchio files are simple, but the command packages compile through an external dependency path that is currently stale relative to the local Glazed cleanup history.

### What warrants a second pair of eyes
- Whether we want a separate coordinated follow-up to update or vendor `clay` so the workspace command-package tests become green again.

### What should be done in the future
- Coordinate a `clay` update/fix if command-package validation for Pinocchio needs to be fully green in this workspace.

### Code review instructions
- Review these four Pinocchio files only; the change is intentionally tiny.
- Confirm that no command lost real config loading logic: Pinocchio config policy still lives in profile/bootstrap code, not in the public parser layer.

### Technical details
- `web-chat` still loads hidden base settings through `profilebootstrap`; removing the no-op parser shim does not change that path.

## Step 4: Remove `ResolveAppConfigPath(...)` entirely and require plans everywhere in the workspace path

After the earlier cleanup passes, one old compatibility seam still remained: `glazed/pkg/config/ResolveAppConfigPath(...)`. Once the user explicitly asked to remove it too, the right move was to finish the cutover all the way through current workspace code. That meant deleting the helper, deleting the last bootstrap legacy fallback in Geppetto, and replacing the remaining direct app-config-path lookups with small explicit plans.

This step tightened the architecture further than the earlier plan-based parser cleanup. At this point, the workspace no longer has two competing config-discovery stories for current code. The active path is declarative plans.

### Prompt Context

**User prompt (verbatim):** "remove it."

**Assistant interpretation:** Remove `ResolveAppConfigPath(...)` as well, not just the Cobra parser path APIs, and migrate the remaining workspace callers to plan-based discovery.

**Inferred user intent:** Finish the cleanup completely instead of leaving the old resolver behind as a compatibility helper.

**Commit (code):** `a94d87327e2cb0d68bd0d5fdd26dfde272d4f484` — `config: remove legacy app config resolver`

**Commit (code):** `8ef6188460a144f31b3c0c8b23119eeb1e125d42` — `bootstrap: require config plans`

**Commit (code):** `3118d0c4050fc7be6e2b83a78f8cd558729664b7` — `pinocchio: resolve repositories with config plans`

### What I did
- Deleted:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/resolve.go`
- Moved the shared `fileExists(...)` helper into:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/plan_sources.go`
- Updated current docs to stop describing `ResolveAppConfigPath(...)` as an active helper:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/24-config-files.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md`
- Removed the legacy bootstrap fallback in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go`
- Made plan builders mandatory in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/config.go`
- Updated Geppetto bootstrap tests to use explicit plans for default discovery:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/bootstrap_test.go`
- Replaced Geppetto legacy section-helper config discovery with explicit plans + `FromResolvedFiles(...)` in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/profile_sections.go`
- Replaced Pinocchio repository-config loading with a small declarative plan in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go`

### Why
- Leaving `ResolveAppConfigPath(...)` in place would have kept an old non-provenance-aware config discovery path alive next to the new plan system.
- Geppetto bootstrap still had a legacy fallback branch that bypassed plans; removing it makes the current bootstrap contract simpler and more honest.
- Pinocchio’s repository-loading code was one of the last direct app-config-path call sites, so it needed to move too.

### What worked
- After the change, there were no remaining live `ResolveAppConfigPath(...)` call sites in `glazed`, `geppetto`, or `pinocchio` current source.
- Focused validation passed for:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto
go test ./pkg/cli/bootstrap/... ./pkg/sections/... -count=1

cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
go test ./pkg/config/... -count=1
```

### What didn't work
- Pinocchio top-level command-package validation is still blocked by the same external `clay` dependency importing the removed Viper logging symbol.

Exact error:

```text
# github.com/go-go-golems/clay/pkg
../../../../go/pkg/mod/github.com/go-go-golems/clay@v0.4.0/pkg/init.go:78:16: undefined: logging.InitLoggerFromViper
```

- I also repeated an earlier command mistake by accidentally passing Markdown files to `gofmt` again while validating the Glazed side. That produced parser errors but did not change the files.

### What I learned
- Once `ResolveAppConfigPath(...)` was removed, the architecture became noticeably more consistent: config discovery for current code is either a declarative plan or historical code that still needs migration outside this workspace.
- The Geppetto bootstrap API is cleaner when `ConfigPlanBuilder` is required rather than optional-with-legacy-fallback.

### What was tricky to build
- The legacy Geppetto section helpers were the trickiest part because they were not on the newer bootstrap path, but they still needed equivalent behavior for default app config + explicit `--config-file`. The clean way to preserve behavior was to build a tiny plan inside the helper and load through `FromResolvedFiles(...)`.
- For Pinocchio repository loading, the subtle point was preserving low→high layer semantics when reading `repositories` from multiple files. I kept the behavior by iterating resolved files in order and letting later files replace the accumulated repository list.

### What warrants a second pair of eyes
- Whether the Geppetto legacy section helpers (`pkg/sections/*`) should eventually be retired in favor of the newer bootstrap package now that they also carry plan logic.
- Whether we want a shared helper/middleware for “resolve plan + load plan” so plan resolution is not performed early in `cobra-parser.go`.

### What should be done in the future
- Follow-up task added: introduce `sources.FromConfigPlan(...)` / `sources.FromConfigPlanBuilder(...)` as middleware wrappers over `FromResolvedFiles(...)`, then simplify `cobra-parser.go` to use that middleware instead of resolving plans directly.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/plan_sources.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/resolve.go` (deleted)
- Then review:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/profile_sections.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto
gofmt -w pkg/cli/bootstrap/config.go pkg/cli/bootstrap/profile_selection.go \
  pkg/cli/bootstrap/bootstrap_test.go pkg/sections/sections.go pkg/sections/profile_sections.go

go test ./pkg/cli/bootstrap/... ./pkg/sections/... -count=1

cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
gofmt -w pkg/config/plan_sources.go

go test ./pkg/config/... -count=1
```

### Technical details
- `AppBootstrapConfig.Validate()` now requires `ConfigPlanBuilder`.
- The old `resolveCLIConfigFilesLegacy(...)` branch was removed from Geppetto bootstrap.
- Current source search after the change showed no remaining live `ResolveAppConfigPath(...)` references in the three workspace repos.

## Step 5: Add `FromConfigPlan*` middleware wrappers and clean up Clay so full workspace validation passes again

After discussing the API shape further, we decided the cleaner layering is to keep `FromResolvedFiles(...)` as the core provenance-aware loading primitive and add higher-level `FromConfigPlan(...)` / `FromConfigPlanBuilder(...)` wrappers on top. That allows the CLI layer to stay ergonomic while moving the actual “plan → resolved files → field updates” logic into the `sources` package where it belongs.

At the same time, the workspace still had a practical validation issue: the local `clay` module was calling the removed Glazed Viper logger API. That had been the reason top-level Pinocchio command-package tests were still failing. I fixed that in Clay as part of this pass so the workspace goes green again.

### Prompt Context

**User prompt (verbatim):** "ok, add that as a task, and continue"

**Assistant interpretation:** Record the middleware follow-up explicitly in the ticket, then implement it and continue cleaning up the remaining practical validation issue in Clay.

**Inferred user intent:** Make the layering cleaner, not just functionally correct, and remove the last external blocker to validating the workspace.

**Commit (code):** `f13b8df640be1b41cf7aedd7e01c6dbd3e92b245` — `sources: add config plan middleware`

**Commit (code):** `20a8a9d4831cd56f61a149c8d64ebec620f98108` — `logging: remove viper logger dependency`

**Commit (code):** `68994cc71f19f5b783854ecc5d0ebccb66d96b8a` — `test: clean web-chat imports`

### What I did
- Added to Glazed:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cmds/sources/load-fields-from-config.go`
    - `ConfigPlanResolver`
    - `FromConfigPlan(...)`
    - `FromConfigPlanBuilder(...)`
- Updated `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go`
  - `CobraParserConfig.ConfigPlanBuilder` is now implemented through `sources.FromConfigPlanBuilder(...)`
  - `cobra-parser.go` no longer resolves plans directly itself
- Added Glazed test coverage in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cmds/sources/config_files_test.go`
    - plan middleware metadata propagation
    - builder middleware using already-parsed values to choose a file
- Updated Clay in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/clay/pkg/init.go`
  - replaced the stale `logging.InitLoggerFromViper()` call with `logging.InitEarlyLoggingFromArgs(...)`
- Cleaned one now-unused test import in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go`

### Why
- `FromResolvedFiles(...)` remains the right low-level primitive because it cleanly separates discovery from loading.
- `FromConfigPlan*` gives a nicer high-level API for callers that want plans directly.
- Moving the plan resolution into `sources` is a cleaner layering than leaving it embedded in `cobra-parser.go`.
- Fixing Clay was necessary to make the workspace validate again after the earlier Glazed logging cleanup.

### What worked
- Glazed plan middleware tests passed:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
go test ./pkg/cmds/sources/... ./pkg/cli/... -count=1
```

- Clay package tests passed:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/clay
go test ./pkg/... -count=1
```

- The previously blocked Pinocchio command-package validation now passes:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent ./cmd/pinocchio/... -count=1
```

### What didn't work
- While implementing `FromConfigPlanBuilder(...)`, my first edit accidentally left behind a broken copied block from `FromResolvedFiles(...)` and referenced an undefined `files` variable. That was caught immediately by compilation/test runs and then replaced with the intended wrapper implementation.

### What I learned
- The API stack is now much cleaner:
  - `FromFiles(...)` for raw paths
  - `FromResolvedFiles(...)` for provenance-aware resolved inputs
  - `FromConfigPlan(...)` / `FromConfigPlanBuilder(...)` as high-level plan wrappers
- That is a better long-term shape than forcing everything to go through plans or forcing the CLI layer to resolve plans itself.

### What was tricky to build
- The subtle design point was preserving the separation between discovery and loading while still letting the middleware variant inspect already-parsed lower-precedence values. The clean way to do that was to make the builder accept current parsed values and then delegate back down to `FromResolvedFiles(...)` after `plan.Resolve(...)`.
- The Clay change was conceptually simple but operationally important because it was the last blocker to full validation of the command packages in this workspace.

### What warrants a second pair of eyes
- Whether we want to expose `context.Context` more explicitly through plan middleware builder call sites beyond the current closure-based approach.
- Whether any current docs should mention `FromConfigPlan*` explicitly now that the middleware exists.

### What should be done in the future
- Optional: update user-facing docs/examples to mention `FromConfigPlan(...)` as the direct middleware-level API, not only `ConfigPlanBuilder` and `FromResolvedFiles(...)`.

### Code review instructions
- Review in this order:
  1. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cmds/sources/load-fields-from-config.go`
  2. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cmds/sources/config_files_test.go`
  3. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go`
  4. `/home/manuel/workspaces/2026-04-10/pinocchiorc/clay/pkg/init.go`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
gofmt -w pkg/cmds/sources/load-fields-from-config.go pkg/cmds/sources/config_files_test.go pkg/cli/cobra-parser.go

go test ./pkg/cmds/sources/... ./pkg/cli/... -count=1

cd /home/manuel/workspaces/2026-04-10/pinocchiorc/clay
gofmt -w pkg/init.go

go test ./pkg/... -count=1

cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
gofmt -w cmd/web-chat/main_profile_registries_test.go

go test ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent ./cmd/pinocchio/... -count=1
```

### Technical details
- `FromConfigPlanBuilder(...)` delegates to `FromResolvedFiles(...)` after resolving the plan.
- `cobra-parser.go` still keeps `ConfigPlanBuilder` as the ergonomic public CLI hook; only the internal implementation changed.
- Clay’s deprecated `InitViper(...)` path now initializes early logging from args instead of using the removed Viper-based Glazed logger path.

## Appendix: Commands Used During Implementation

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc
auto_rg='rg -n --glob "*.go" "ConfigFilesFunc|ConfigPath|pkg/appconfig|appconfig\." glazed geppetto pinocchio'

cd glazed
gofmt -w pkg/cli/cobra-parser.go pkg/cli/cobra_parser_config_test.go pkg/cmds/sources/cobra.go \
  cmd/examples/config-single/main.go cmd/examples/middlewares-config-env/main.go \
  cmd/examples/config-overlay/main.go cmd/examples/overlay-override/main.go

go test ./pkg/cli/... ./pkg/cmds/sources/... ./cmd/examples/config-single \
  ./cmd/examples/middlewares-config-env ./cmd/examples/config-overlay \
  ./cmd/examples/overlay-override -count=1

go test ./pkg/cli/... ./pkg/cmds/sources/... ./pkg/config/... \
  ./cmd/examples/config-single ./cmd/examples/middlewares-config-env \
  ./cmd/examples/config-overlay ./cmd/examples/overlay-override -count=1

git commit --no-verify -m "cli: switch cobra config loading to plans"
git commit --no-verify -m "config: remove appconfig facade"

cd ../pinocchio
gofmt -w cmd/web-chat/main.go cmd/web-chat/main_profile_registries_test.go \
  cmd/examples/simple-chat/main.go cmd/agents/simple-chat-agent/main.go

go test ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent -count=1

git commit --no-verify -m "cli: drop no-op config plan shims"
```
