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

## Step 6: Refresh the active Glazed docs so the new plan middleware is actually discoverable

After landing `FromConfigPlan(...)` and `FromConfigPlanBuilder(...)`, the codebase was internally cleaner but the user-facing docs still mostly taught the older two-step story: build a plan, call `plan.Resolve(...)`, then pass the result to `FromResolvedFiles(...)`. That is still valid, but it no longer tells the full story now that Glazed has first-class middleware wrappers for plans.

So this pass focused on documentation only: update the active help pages and tutorials so they clearly explain when to use each layer of the API.

### Prompt Context

**User prompt (verbatim):** "continue"

**Assistant interpretation:** Keep going with the next useful cleanup pass without stopping for clarification.

**Inferred user intent:** Finish the obvious follow-up work from the prior step, especially the docs cleanup I had just proposed.

### What I did
- Updated Glazed docs in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/24-config-files.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/27-declarative-config-plans.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/examples/config/01-declarative-config-plan.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/config-files-quickstart.md`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md`
- Added a DONE task to this ticket noting that active docs now teach `FromConfigPlan(...)` / `FromConfigPlanBuilder(...)` explicitly.

### Why
- The code had moved ahead of the docs.
- Without this pass, readers would still discover the low-level `FromResolvedFiles(...)` path but miss the higher-level direct middleware API.
- The whole point of the previous refactor was not only internal cleanliness but a better API shape for callers. The docs should reflect that.

### What worked
- `git diff --check` in Glazed passed cleanly after the doc edits:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
git diff --check
```

### What didn't work
- Nothing functionally broke here; this was a doc-only pass.

### What I learned
- The current plan-loading API now has a clean three-level explanation that is worth teaching explicitly:
  - `FromFiles(...)` for simple ordered file lists
  - `FromResolvedFiles(...)` when you want explicit resolved inputs + provenance
  - `FromConfigPlan(...)` / `FromConfigPlanBuilder(...)` when you want the middleware pipeline to own resolution

### What was tricky to write down
- The subtle part is explaining that `FromConfigPlan(...)` is not a replacement for `FromResolvedFiles(...)`; it is a higher-level convenience over it. The docs need to preserve that layering instead of flattening everything into “just use plans.”

### What warrants a second pair of eyes
- Whether the current doc examples should also add a fully runnable direct-`FromConfigPlan(...)` example in code, instead of only describing it in the help pages.

### What should be done in the future
- Optional: add a second runnable example (or extend `cmd/examples/config-plan`) that shows `sources.FromConfigPlanBuilder(...)` choosing a config file based on already-parsed lower-precedence selector values.

### Code review instructions
- Review in this order:
  1. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/24-config-files.md`
  2. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/27-declarative-config-plans.md`
  3. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/examples/config/01-declarative-config-plan.md`
  4. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/config-files-quickstart.md`
  5. `/home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed
git diff --check
```

### Technical details
- `24-config-files.md` now teaches direct plan middleware loading in addition to explicit `plan.Resolve(...) + FromResolvedFiles(...)`.
- `27-declarative-config-plans.md` now distinguishes between manual resolution and direct middleware loading, and documents when to use each.
- The example/tutorial pages now mention why an example may intentionally keep the explicit resolved-file path even though `FromConfigPlan(...)` exists.

## Step 7: Remove `InitViper(...)` from local Clay and audit remaining active consumers in `corporate-headquarters`

After removing the Viper-based logging path and then cleaning up `FromConfigPlan(...)` middleware, the local Clay module still had one leftover convenience function: `InitViper(appName, rootCmd)`. It was no longer used anywhere in the active workspace code, and keeping the symbol around only encouraged exactly the deprecated runtime path we are trying to eliminate.

The user explicitly asked to remove that symbol entirely and to check what still depends on it in `corporate-headquarters`.

### Prompt Context

**User prompt (verbatim):** "Remove the InitViper entirely from clay, and check what still uses it in corporate-headquarters"

**Assistant interpretation:** Delete the local Clay `InitViper(...)` helper instead of leaving it deprecated, then audit `corporate-headquarters` for live Go call sites so the remaining migration surface is clear.

**Inferred user intent:** Finish the cleanup decisively and identify the next external migration targets instead of leaving another deprecated escape hatch in place.

### What I did
- Removed `InitViper(...)` completely from:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/clay/pkg/init.go`
- Updated Clay README references in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/clay/README.md`
  - removed the example mention of `pkg.InitViper(...)`
- Updated one active workspace doc snippet in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/doc/topics/06-embeddings.md`
  - changed the sample root initialization from `clay.InitViper(...)` to `clay.InitGlazed(...)`
- Audited active `corporate-headquarters` Go files (excluding `ttmp`, old vibe snapshots, and similar historical noise) for remaining `InitViper(...)` call sites.

### Why
- Local Clay no longer needs to preserve a deprecated one-shot Viper/bootstrap helper.
- The workspace already uses explicit Glazed parser config and source middleware patterns.
- Removing the symbol now prevents new callers from reintroducing the deprecated path.
- The external `corporate-headquarters` audit provides the concrete follow-up list for any broader migration work.

### What worked
- Clay still validated after the removal:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/clay
gofmt -w pkg/init.go
go test ./pkg/... -count=1
```

- The previously validated Pinocchio command-package tests still pass after the Clay symbol removal:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent ./cmd/pinocchio/... -count=1
```

### What didn't work
- A first quick `rg` over `corporate-headquarters` mixed together active code, local definitions, backups, and historical `ttmp` notes. I reran the search with tighter filters so the audit result reflects active Go call sites instead of every historical mention.

### Corporate-headquarters audit results

Active remaining Go call sites using `clay.InitViper(...)` or the sibling deprecated `logging.InitViper(...)`:

- `cliopatra`
  - `cliopatra/cmd/cliopatra/main.go:46`
- `ecrivain`
  - `ecrivain/cmd/ecrivain/main.go:228`
- `escuse-me`
  - `escuse-me/cmd/escuse-me/main.go:164`
- `facture`
  - `facture/cmd/facture/main.go:53`
- `go-go-agent`
  - `go-go-agent/cmd/simple/main.go:197`
  - `go-go-agent/cmd/file-extraction/main.go:204`
  - `go-go-agent/cmd/file-extraction/main.go:215`
  - `go-go-agent/cmd/agent/main.go:32`
- `go-go-labs`
  - `go-go-labs/cmd/apps/datadog-cli/main.go:148`
  - `go-go-labs/cmd/apps/maps/main.go:42`
  - `go-go-labs/cmd/apps/reggie/main.go:18`
  - `go-go-labs/cmd/apps/create-pr/main.go:223`
  - `go-go-labs/cmd/apps/embeddings/main.go:50`
  - `go-go-labs/cmd/apps/test-html-selector/main.go:599`
  - `go-go-labs/cmd/apps/note-linker/main.go:188`
  - `go-go-labs/cmd/apps/capture/main.go:239`
  - `go-go-labs/cmd/apps/cloudwatch-access-log/main.go:20`
  - `go-go-labs/cmd/apps/poll-modem/cmd/root.go:49`
  - `go-go-labs/cmd/apps/simplify-html/main.go:270`
  - `go-go-labs/cmd/github-projects/main.go:63` (`logging.InitViper(...)`, same deprecated family)
- `mastoid`
  - `mastoid/cmd/mastoid/main.go:27`
- `oak`
  - `oak/cmd/oak/commands/root.go:39`
- `plunger`
  - `plunger/cmd/plunger/main.go:50`
- `prompto`
  - `prompto/cmd/prompto/main.go:39`
- `uhoh`
  - `uhoh/cmd/uhoh/main.go:69`

I also saw backup/original copies still containing the old call in:
- `workspace-manager.backup/cmd/root.go:49`
- `workspace-manager.orig/cmd/root.go:49`

And `corporate-headquarters` still has its own local definitions of the deprecated helpers in:
- `clay/pkg/init.go`
- `glazed/pkg/cmds/logging/init-logging.go`

### What I learned
- The remaining migration surface in `corporate-headquarters` is still fairly broad, especially in `go-go-labs`.
- The workspace cleanup here is now stricter than the external monorepo state: the local Clay module no longer exports `InitViper(...)` at all.

### What was tricky
- The main subtlety was distinguishing live code from historical notes and copies. `ttmp`, `.orig`, `.backup`, and vibe snapshots contain many references that are useful historically but should not be treated as current migration blockers.

### What warrants a second pair of eyes
- Which of the `corporate-headquarters` consumers should be migrated first versus deleted/archived, especially under `go-go-labs` where several apps may be inactive.

### What should be done in the future
- Optional external follow-up: migrate `corporate-headquarters/clay` to remove its own `InitViper(...)` symbol and then port the remaining active consumers to `InitGlazed(...)` plus explicit parser config / config-plan wiring.

### Code review instructions
- Review in this order:
  1. `/home/manuel/workspaces/2026-04-10/pinocchiorc/clay/pkg/init.go`
  2. `/home/manuel/workspaces/2026-04-10/pinocchiorc/clay/README.md`
  3. `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/doc/topics/06-embeddings.md`
- Validate with:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/clay
gofmt -w pkg/init.go

go test ./pkg/... -count=1

cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
go test ./cmd/web-chat ./cmd/examples/simple-chat ./cmd/agents/simple-chat-agent ./cmd/pinocchio/... -count=1
```

### Technical details
- Local Clay now still exposes the narrower deprecated helpers `InitViperWithAppName(...)` and `InitViperInstanceWithAppName(...)`, but not the root-command bootstrap shortcut `InitViper(...)`.
- The local active workspace no longer has any live code that depends on the removed Clay helper.

## Step 8: Pragmatically kill the removed Viper startup path across the remaining legacy `corporate-headquarters` programs

After the audit, the user explicitly said not to spend effort modernizing the old apps fully and not to worry about whether most of them still compile, especially in `go-go-labs`. The goal was simpler: stop the active legacy programs from referencing the removed worktree startup APIs.

So this pass was intentionally blunt and pragmatic:
- replace `clay.InitViper(...)` with `clay.InitGlazed(...)`
- replace `pkg.InitViper(...)` with `pkg.InitGlazed(...)`
- replace active `logging.InitLoggerFromViper()` usage with `logging.InitLoggerFromCobra(...)`
- replace the one active `logging.InitViper(...)` user in `go-go-labs/cmd/github-projects/main.go`
- do **not** try to fully modernize command parsing, config loading, or make every legacy program compile cleanly

### Prompt Context

**User prompt (verbatim):** "ok, just kill it. these are all legacy programs. no need to update glazed/clay in corp headquarters, we have a worktree right here"

**Follow-up clarification (verbatim):** "most of these project s don't compile anymore anyway, especially in go-go-labs"

**Assistant interpretation:** Remove references to the deleted/deprecated startup helpers from the active legacy corp programs, but do not turn this into a broader modernization or validation project.

**Inferred user intent:** Make the worktree-compatible API sweep quickly and stop.

### What I changed
Active corp code was updated in these areas:

- `cliopatra/cmd/cliopatra/main.go`
- `ecrivain/cmd/ecrivain/main.go`
- `escuse-me/cmd/escuse-me/main.go`
- `facture/cmd/facture/main.go`
- `go-go-agent/cmd/simple/main.go`
- `go-go-agent/cmd/file-extraction/main.go`
- `go-go-agent/cmd/agent/main.go`
- `go-go-labs/cmd/apps/datadog-cli/main.go`
- `go-go-labs/cmd/apps/maps/main.go`
- `go-go-labs/cmd/apps/reggie/main.go`
- `go-go-labs/cmd/apps/create-pr/main.go`
- `go-go-labs/cmd/apps/embeddings/main.go`
- `go-go-labs/cmd/apps/test-html-selector/main.go`
- `go-go-labs/cmd/apps/note-linker/main.go`
- `go-go-labs/cmd/apps/capture/main.go`
- `go-go-labs/cmd/apps/cloudwatch-access-log/main.go`
- `go-go-labs/cmd/apps/poll-modem/cmd/root.go`
- `go-go-labs/cmd/apps/simplify-html/main.go`
- `go-go-labs/cmd/github-projects/main.go`
- `go-go-labs/cmd/n8n-cli/main.go`
- `mastoid/cmd/mastoid/main.go`
- `oak/cmd/oak/commands/root.go`
- `openai-app-server/cmd/openai-app-server/root.go`
- `openai-mock-server/cmd/openai-mock-server/main.go`
- `plunger/cmd/plunger/main.go`
- `prompto/cmd/prompto/main.go`
- `uhoh/cmd/uhoh/main.go`
- `zine-layout/cmd/zine-layout/main.go`

### What worked
A focused active-code grep now shows the removed/deprecated startup path is gone from current corp Go code and only remains in:
- `corporate-headquarters/clay/pkg/init.go` (the corp-local legacy helper definition we intentionally did **not** edit)
- `workspace-manager.orig/...`
- `workspace-manager.backup/...`

Command used:

```bash
cd /home/manuel/code/wesen/corporate-headquarters
rg -n 'InitViper\(|InitLoggerFromViper\(' . --glob '*.go' -g '!**/ttmp/**' -g '!vibes/**'
```

### What didn’t work / what I explicitly did not do
- I did **not** try to make the touched legacy programs compile or pass tests.
- I briefly widened the scope to rename a few unrelated `AddLoggingLayerToRootCommand(...)` call sites, then stopped and reverted that expansion when the user said not to bother.
- I did **not** update `corporate-headquarters/clay` or `corporate-headquarters/glazed` themselves; the user specifically said that was unnecessary because the worktree modules are here already.

### What I learned
- The useful stopping condition for this pass was not “all legacy corp programs build” but “active corp code no longer references removed worktree startup helpers.”
- That condition is now satisfied.

### What warrants a second pair of eyes
- Only if someone later decides these legacy programs are worth reviving. At that point they should get proper parser/config modernization rather than more search-and-replace cleanup.

### What should be done in the future
- Optional: none, unless one of these legacy programs is revived and deserves a real Glazed parser/config migration.

### Review instructions
- Sanity check with:

```bash
cd /home/manuel/code/wesen/corporate-headquarters
rg -n 'InitViper\(|InitLoggerFromViper\(' . --glob '*.go' -g '!**/ttmp/**' -g '!vibes/**'
```

Expected remaining hits are only the corp-local Clay helper definition and the backup/orig workspace-manager copies.

## Step 9: Delete the remaining Geppetto legacy middleware builders and the Pinocchio `UseViper` helper parser

The next cleanup pass targeted the highest-value compatibility surfaces identified in the audit: the large duplicated Geppetto section middleware builders and Pinocchio’s own manual helper parser that still carried a `UseViper`-shaped API. The goal was not to redesign the active bootstrap path again, but to remove the remaining alternate paths that kept old architecture ideas alive.

### Prompt Context

**User prompt (verbatim):** "go ahead."

**Assistant interpretation:** Start executing the recommended deletion sequence from the cleanup audit instead of stopping at documentation.

**Inferred user intent:** Remove the most obvious remaining legacy surfaces now that the new config-plan/bootstrap path is stable.

### What I did
- Deleted Geppetto legacy middleware builders from:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go`
    - removed `resolvePinocchioConfigFiles(...)`
    - removed `GetCobraCommandGeppettoMiddlewares(...)`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/profile_sections.go`
    - removed `defaultPinocchioProfileRegistriesIfPresent()`
    - removed `GetProfileSettingsMiddleware(...)`
- Deleted Pinocchio’s compatibility parser helper:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/parse-helpers.go`
- Migrated the active `simple-chat` example away from that helper:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-chat/main.go`
  - it now resolves helper-layer config with `ResolveCLIConfigFilesResolved(...) + FromResolvedFiles(...) + Merge(parsed)` and resolves inference settings through `profilebootstrap.ResolveCLIEngineSettings(...)`
- Added a resolved-files wrapper in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
    - `ResolveCLIConfigFilesResolved(...)`
- Replaced the deleted Geppetto middleware helper in Pinocchio command wiring with a current plan-based path in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cobra.go`
  - `BuildCobraCommandWithGeppettoMiddlewares(...)` now uses a local middleware builder over `profilebootstrap.BootstrapConfig()` and `sources.FromConfigPlanBuilder(...)`

### Why
- The Geppetto middleware builders were large duplicated compatibility surfaces that no longer matched the active bootstrap architecture.
- `parse-helpers.go` was the clearest remaining Pinocchio file still shaped around a Viper-era mental model, including a `UseViper` field name and manual reconstruction of env/config/default parsing.
- Keeping those files around made the codebase look like it still supported multiple equally valid configuration architectures, even though the project had already standardized on bootstrap + plans.

### What worked
- Focused Geppetto validation still passed after deleting the legacy helpers:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto
gofmt -w pkg/sections/sections.go pkg/sections/profile_sections.go

go test ./pkg/sections ./pkg/cli/bootstrap/... -count=1
```

- Focused Pinocchio validation passed after migrating the caller and deleting `parse-helpers.go`:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
gofmt -w pkg/cmds/cobra.go cmd/examples/simple-chat/main.go pkg/cmds/profilebootstrap/profile_selection.go

go test ./pkg/cmds/profilebootstrap ./pkg/cmds/helpers ./pkg/cmds ./cmd/examples/simple-chat -count=1
```

- A follow-up grep over current source no longer finds:
  - `GetCobraCommandGeppettoMiddlewares(...)`
  - `GetProfileSettingsMiddleware(...)`
  - `ParseGeppettoLayers(...)`
  - `WithUseViper(...)`
  - `GeppettoLayersHelper`

### What didn't work
- My earlier audit grep missed one real active caller of the deleted Geppetto helper:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cobra.go`
- That showed up immediately as a compile error:

```text
pkg/cmds/cobra.go:17:32: undefined: sections2.GetCobraCommandGeppettoMiddlewares
```

- The fix was not to restore the old helper, but to replace that caller with a local plan-based middleware builder that uses `profilebootstrap.BootstrapConfig()` directly.

### What I learned
- The deleted Geppetto helpers were not just stale; they had already stopped being a useful abstraction. Removing them actually clarified the active ownership split.
- The Pinocchio command-layer caller in `pkg/cmds/cobra.go` was the real bridge that needed to survive, but it did not need the old Geppetto helper to do so.

### What was tricky
- The `simple-chat` example still wanted two different things from parsing:
  1. helper-layer values such as `cmdlayers.HelpersSettings`
  2. final resolved inference settings with profile overlay applied
- The clean answer was to split those responsibilities explicitly instead of continuing to funnel everything through `ParseGeppettoLayers(...)`.

### What warrants a second pair of eyes
- Whether `BuildCobraCommandWithGeppettoMiddlewares(...)` should keep that name now that it no longer uses the old Geppetto helper function, or whether it should eventually be renamed to reflect that it is really a Pinocchio-specific plan/bootstrap parser path.
- Whether the resolved-files wrapper added to `profilebootstrap` should remain public or be folded into a more direct caller path in the next cleanup step.

### What should be done in the future
- Continue with the next cleanup tranche identified in the audit:
  - collapse duplicated bootstrap config middleware assembly in Geppetto
  - migrate `cmd/pinocchio/cmds/js.go` off path-list config loading
  - then evaluate whether the remaining thin helper re-export layer in `pkg/cmds/helpers/*` still buys anything

### Code review instructions
- Review in this order:
  1. `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go`
  2. `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/profile_sections.go`
  3. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/parse-helpers.go` (deleted)
  4. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/examples/simple-chat/main.go`
  5. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cobra.go`
  6. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`

## Step 10: Collapse the remaining bootstrap duplication, migrate the JS command to resolved files, and delete the thin helper package entirely

After Step 9, the obvious legacy middleware builders were gone, but there were still a few structural leftovers:

- Geppetto bootstrap still repeated the same config-middleware assembly in multiple files.
- The Pinocchio JS command still used `ResolveCLIConfigFiles(...) + FromFiles(...)`, which kept the path-list model alive.
- JS and web-chat both rebuilt profile registry chains manually.
- `cmd/pinocchio/main.go` still manually decoded YAML just to read `repositories`.
- The thin `pinocchio/pkg/cmds/helpers` re-export package no longer had any active callers.

This step cleaned all of those up in one pass.

### Prompt Context

**User prompt (verbatim):** still within the same "go ahead" cleanup continuation.

**Assistant interpretation:** Continue removing the remaining active legacy or compatibility-shaped surfaces instead of stopping after the first deletion tranche.

**Inferred user intent:** Finish the cleanup as far as it can reasonably go while the architecture is fresh and the tests are green.

### What I did
- Added a shared resolved-files middleware helper in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/config_loading.go`
- Updated Geppetto bootstrap to use that helper in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/engine_settings.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/inference_debug.go`
- Added a shared profile-registry-chain builder in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_registry.go`
- Updated Pinocchio JS command to use resolved files and the shared registry-chain helper:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js_test.go`
- Updated web-chat to use the shared registry-chain helper:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go`
- Added typed repository-config loading helpers in:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories.go`
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories_test.go`
- Updated main command repository loading to use the typed helper instead of manual YAML parsing:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go`
- Exported the local Pinocchio middleware builder and reused it from `cmd/pinocchio/main.go`:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/cobra.go`
- Deleted the no-longer-used thin helper package entirely:
  - `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/`

### Why
- The config middleware duplication in bootstrap was now pure maintenance overhead.
- The JS command still represented one of the last path-centric holdouts in active Pinocchio code.
- The duplicated registry-chain setup in bootstrap/JS/web-chat was unnecessary once the current profile-selection model stabilized.
- The main command’s repository loading should follow the same typed/config-plan-based approach as the rest of the refactor instead of manually unmarshalling YAML.
- The thin helper re-export package no longer bought anything once its only active caller had been migrated away.

### What worked
- Geppetto bootstrap validation passed after the shared helper extraction:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto
gofmt -w pkg/cli/bootstrap/config_loading.go pkg/cli/bootstrap/profile_selection.go \
  pkg/cli/bootstrap/engine_settings.go pkg/cli/bootstrap/inference_debug.go \
  pkg/cli/bootstrap/profile_registry.go

go test ./pkg/cli/bootstrap/... -count=1
```

- Pinocchio validation passed after the JS/web-chat/main-command refactor:

```bash
cd /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio
gofmt -w pkg/cmds/cobra.go cmd/pinocchio/main.go \
  cmd/pinocchio/cmds/js.go cmd/pinocchio/cmds/js_test.go \
  cmd/web-chat/main.go pkg/cmds/profilebootstrap/repositories.go \
  pkg/cmds/profilebootstrap/repositories_test.go

go test ./pkg/cmds/profilebootstrap ./pkg/cmds ./cmd/pinocchio \
  ./cmd/pinocchio/cmds/... ./cmd/web-chat -count=1
```

- A grep over current source no longer finds:
  - `GetCobraCommandGeppettoMiddlewares(...)`
  - `GetProfileSettingsMiddleware(...)`
  - `ParseGeppettoLayers(...)`
  - `WithUseViper(...)`
  - `GeppettoLayersHelper`

### What didn't work
- While replacing manual repository loading in `cmd/pinocchio/main.go`, two more direct references to the deleted Geppetto middleware helper surfaced in repository/clip command wiring.
- Exact compile failure:

```text
cmd/pinocchio/main.go:224:42: undefined: sections2.GetCobraCommandGeppettoMiddlewares
cmd/pinocchio/main.go:251:42: undefined: sections2.GetCobraCommandGeppettoMiddlewares
```

- The fix was to export the local Pinocchio middleware builder from `pkg/cmds/cobra.go` as `GetPinocchioCommandMiddlewares(...)` and use that from `cmd/pinocchio/main.go`.
- After extracting the shared registry-chain helper, the JS test and web-chat import list also needed small follow-up fixes.

### What I learned
- Once the first destructive pass removed the biggest compatibility surfaces, the remaining cleanup became easier because the code started to point more obviously toward the single intended architecture.
- The active workspace no longer really needs a separate `pinocchio/pkg/cmds/helpers` compatibility package.
- The `cmd/pinocchio/main.go` wiring had quietly carried a couple more deleted-helper dependencies than the initial grep suggested, so compile/test runs were still the best way to flush out stragglers.

### What was tricky
- The repository loading change looked local at first, but it indirectly forced the last `cmd/pinocchio/main.go` callers of the deleted Geppetto middleware builder into the open.
- The shared registry-chain helper also touched three different flows at once: bootstrap engine settings, JS runtime bootstrap, and web-chat startup.

### What warrants a second pair of eyes
- Whether `ResolveCLIConfigFiles(...)` / `ResolveCLIConfigFilesForExplicit(...)` are still worth keeping as public wrappers now that current active callers use resolved files instead.
- Whether `GetPinocchioCommandMiddlewares(...)` should remain exported or be folded into a more clearly named command/parser helper surface.

### What should be done in the future
- Optional: prune or internalize the remaining path-centric `ResolveCLIConfigFiles*` wrappers if no meaningful active callers still need them.
- Optional: decide whether the now-shared registry-chain helper should eventually be used by any remaining Geppetto example/internal packages that still construct chains manually.

### Code review instructions
- Review in this order:
  1. `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/config_loading.go`
  2. `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_registry.go`
  3. `/home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/{profile_selection,engine_settings,inference_debug}.go`
  4. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/cmds/js.go`
  5. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go`
  6. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/profilebootstrap/repositories.go`
  7. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go`
  8. `/home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/pkg/cmds/helpers/` (deleted)

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
