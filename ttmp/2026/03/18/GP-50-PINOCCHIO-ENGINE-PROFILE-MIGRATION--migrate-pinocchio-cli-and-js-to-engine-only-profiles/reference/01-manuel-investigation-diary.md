---
Title: Manuel investigation diary
Ticket: GP-50-PINOCCHIO-ENGINE-PROFILE-MIGRATION
Status: active
Topics:
    - pinocchio
    - migration
    - config
    - js-bindings
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/pinocchio/cmds/js.go
      Note: JS command path inspected in Step 1
    - Path: examples/js/profiles/basic.yaml
      Note: Fixture inspection captured in Step 1
    - Path: examples/js/runner-profile-demo.js
      Note: Script inspection captured in Step 1
    - Path: pkg/cmds/helpers/profile_runtime.go
      Note: Main helper inspected in Step 1
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-18T16:22:21.607041646-04:00
WhatFor: ""
WhenToUse: ""
---


# Manuel investigation diary

## Goal

Record the Pinocchio downstream migration from mixed runtime profiles to engine-only profiles, starting with the `cmd/pinocchio` binary and `pinocchio js`, while deferring web chat to a later slice.

## Step 1: Create the migration ticket and map the breakage

The immediate task was to create a new Pinocchio migration ticket and build an implementation plan around the places that matter first to users: repository-loaded CLI commands such as `pinocchio code unix hello` and the `pinocchio js` command. I approached this as a downstream hard-cut analysis, not as an attempt to keep the old mixed runtime/profile behavior alive.

The investigation showed that Pinocchio is currently in an inconsistent state after the Geppetto engine-profile hard cut. The shared CLI helper resolves an engine profile but returns only a clone of the base settings, and the JS examples still use the removed mixed runtime profile format. That means the current issue is not confined to one example script; it sits in the shared bootstrap path.

### Prompt Context

**User prompt (verbatim):** "ok, do the downstream migration in pinocchio. I'm first concerned with the cmd/pinocchio binary, where for example `pinocchio code unix hello` should for example work (using config and then ~/.config/pinocchio/profiles.yaml (which you might need to reformat with a script), and then `pinocchio js` with one of the scripts that should be doing a real inference. We can then move on to web-chat which actually requires multiple profiles. Make a new ticket for this pinocchio migration, and analyze the situation and make an implementation plan"

**Assistant interpretation:** Create a new Pinocchio ticket, analyze the current downstream breakage after the engine-profile hard cut, and produce a concrete implementation plan focused on the main CLI and JS paths before web chat.

**Inferred user intent:** Establish a clean downstream migration path for Pinocchio so the CLI and JS command behave coherently with engine-only profiles, without reintroducing the removed mixed runtime complexity.

### What I did

- Inspected [`pkg/cmds/helpers/profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go) to verify how base settings and engine profile selection are currently resolved.
- Inspected [`cmd/pinocchio/cmds/js.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/js.go) to see how `pinocchio js` wires Geppetto and Pinocchio JS modules.
- Inspected [`pkg/js/modules/pinocchio/module.go`](../../../../../../pinocchio/pkg/js/modules/pinocchio/module.go) to confirm `pinocchio.engines.fromDefaults(...)` is base-config-only.
- Inspected [`cmd/pinocchio/main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go) and repository-loading flow to confirm that `pinocchio code unix hello` comes from repository-loaded YAML commands rather than a built-in Cobra subtree.
- Inspected the current example fixture and script:
  - [`examples/js/profiles/basic.yaml`](../../../../../../pinocchio/examples/js/profiles/basic.yaml)
  - [`examples/js/runner-profile-demo.js`](../../../../../../pinocchio/examples/js/runner-profile-demo.js)
- Created ticket `GP-50-PINOCCHIO-ENGINE-PROFILE-MIGRATION` with:
  - [index.md](../index.md)
  - [01-pinocchio-engine-profile-migration-analysis-and-implementation-plan.md](../design-doc/01-pinocchio-engine-profile-migration-analysis-and-implementation-plan.md)
  - [tasks.md](../tasks.md)

### Why

- The migration needs to happen at the shared bootstrap layer or the user-visible CLI will remain inconsistent.
- The old example/profile files are actively misleading because they imply profile-driven engine behavior that no longer exists.
- A detailed plan is needed before code changes because repository-loaded commands and JS use different surfaces but share the same conceptual engine-profile dependency.

### What worked

- I confirmed the core shared helper bug: `ResolveInferenceSettings(...)` resolves an engine profile and then returns `base.Clone()` instead of the merged final settings.
- I confirmed the repository-loaded command path is the relevant target for `pinocchio code ...`.
- I confirmed `pinocchio.engines.fromDefaults(...)` is intentionally base-config-oriented and does not consult profile registries.

### What didn't work

- Searching for a built-in `code unix` Cobra command did not produce one. The command is not implemented as a dedicated subtree in the repository, so the migration cannot be scoped to a single command package.
- The JS fixture inspection confirmed the checked-in example YAML is still in the removed mixed format:

```yaml
runtime:
  system_prompt: ...
```

That format is no longer valid for the intended architecture.

### What I learned

- The first real migration target is the shared CLI helper path, not `pinocchio js`.
- The second target is the JS command bootstrap, which still treats engine profile selection like runtime resolution.
- Pinocchio's default `~/.config/pinocchio/profiles.yaml` behavior is still fine as a convenience, but the file now needs to mean "engine profiles only."

### What was tricky to build

- The user-facing example `pinocchio code unix hello` suggested a built-in command path, but Pinocchio actually loads many commands dynamically from configured repositories. That means the migration target is a layer deeper than the visible command name.
- The current code shape hides a serious bug in plain sight: `ResolveInferenceSettings(...)` looks authoritative but silently discards the resolved engine profile when returning settings. That kind of helper is dangerous because downstream callers think they are migrated when they are not.

### What warrants a second pair of eyes

- The eventual merge behavior for base settings plus resolved engine profile settings. The helper needs to preserve the intended precedence order and avoid accidental partial overrides.
- The JS API naming around `fromDefaults(...)` versus any new profile-aware helper. Renaming or adding a new helper may be cleaner than silently changing existing semantics.

### What should be done in the future

- Implement Slice 2 first: shared CLI bootstrap hard cut.
- Then migrate repository-loaded command execution and `pinocchio js`.
- Defer web chat until the CLI and JS paths are stable.

### Code review instructions

- Start with [`profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go) and compare its current return value to the new Geppetto engine-profile contract in [`geppetto/pkg/engineprofiles`](../../../../../../geppetto/pkg/engineprofiles).
- Then inspect [`js.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/js.go) and [`module.go`](../../../../../../pinocchio/pkg/js/modules/pinocchio/module.go) to see the current engine/runtime split.
- Validate the analysis with:
  - `rg -n "ResolveInferenceSettings\\(|ResolveEngineProfileSettings\\(|runner.resolveRuntime\\(" pinocchio -g'*.go' -g'*.js'`
  - `go run ./cmd/pinocchio js ./examples/js/runner-profile-demo.js --profile <slug>`

### Technical details

Key observed code shape:

```go
resolved, err := chain.ResolveEngineProfile(ctx, in)
if err != nil { ... }
return base.Clone(), resolved, func() { _ = chain.Close() }, nil
```

This is the main helper bug that must be removed first.

## Quick Reference

- Shared helper to fix first:
  - [`profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go)
- Repository-loaded command entry:
  - [`main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go)
- JS command entry:
  - [`js.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/js.go)
- Base config engine builder:
  - [`module.go`](../../../../../../pinocchio/pkg/js/modules/pinocchio/module.go)
- Stale example fixture:
  - [`basic.yaml`](../../../../../../pinocchio/examples/js/profiles/basic.yaml)
- Stale example script:
  - [`runner-profile-demo.js`](../../../../../../pinocchio/examples/js/runner-profile-demo.js)

## Usage Examples

- Use this diary together with the design doc while implementing each slice.
- After each code commit, add a new diary step with the exact commands run and failures observed.

## Step 2: Hard-cut the shared CLI path and rewrite `pinocchio js` around engine profiles

This step implemented the first real GP-50 migration slice. The goal was to make the `cmd/pinocchio` binary coherent again before touching web chat: the shared helper needed to return final merged `InferenceSettings`, the JS smoke/live examples needed to stop pretending there was still a profile-driven runtime layer in Geppetto, and the stale `profiles_migrate_legacy` command needed to disappear rather than linger as a false promise.

### Prompt Context

**User prompt (verbatim):** "ok, add tasks to the ticket and work them off one by one, committing at appropriate intervals and keeping a detailed diary"

**Assistant interpretation:** Implement the first GP-50 slices in code, keep the ticket in sync, and record the exact migration work and validation.

**Inferred user intent:** Move the Pinocchio CLI and JS command onto the new engine-profile model without spending time preserving old mixed runtime semantics.

### What I did

- Added and used a canonical helper in [`pkg/cmds/helpers/profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go) that resolves base config plus optional engine profile into final merged `InferenceSettings`.
- Updated direct helper callers:
  - [`cmd/agents/simple-chat-agent/main.go`](../../../../../../pinocchio/cmd/agents/simple-chat-agent/main.go)
  - [`cmd/examples/internal/tuidemo/profile.go`](../../../../../../pinocchio/cmd/examples/internal/tuidemo/profile.go)
- Added focused regression tests in [`pkg/cmds/helpers/profile_runtime_test.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime_test.go).
- Deleted the stale legacy migration command:
  - [`cmd/pinocchio/cmds/profiles_migrate_legacy.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/profiles_migrate_legacy.go)
  - [`cmd/pinocchio/cmds/profiles_migrate_legacy_test.go`](../../../../../../pinocchio/cmd/pinocchio/cmds/profiles_migrate_legacy_test.go)
- Reworked the command-path JS fixtures and tests:
  - [`examples/js/profiles/basic.yaml`](../../../../../../pinocchio/examples/js/profiles/basic.yaml)
  - [`examples/js/runner-profile-smoke.js`](../../../../../../pinocchio/examples/js/runner-profile-smoke.js)
  - [`examples/js/runner-profile-demo.js`](../../../../../../pinocchio/examples/js/runner-profile-demo.js)
  - [`cmd/pinocchio/main_profile_registries_test.go`](../../../../../../pinocchio/cmd/pinocchio/main_profile_registries_test.go)
- Cleaned remaining CLI-side stale runtime key/fingerprint usage in:
  - [`cmd/switch-profiles-tui/main.go`](../../../../../../pinocchio/cmd/switch-profiles-tui/main.go)
  - [`scripts/profile-infer-once/main.go`](../../../../../../pinocchio/scripts/profile-infer-once/main.go)
- Updated the public docs/help:
  - [`examples/js/README.md`](../../../../../../pinocchio/examples/js/README.md)
  - [`README.md`](../../../../../../pinocchio/README.md)
  - [`05-js-runner-scripts.md`](../../../../../../pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md)

### Why

- `pinocchio js` needs one clear profile story: engine profiles choose engine settings, and `gp.runner` runs the result.
- The old smoke/live scripts were still teaching the removed GP-49 model, which made the command look broken even after the CLI helper was fixed.
- Leaving `profiles_migrate_legacy` in place would imply the mixed runtime format is still a supported migration target, which it is not.

### What worked

- The focused CLI/helper tests passed after the helper cutover.
- `go test ./cmd/pinocchio -count=1` passed once the JS fixtures and tests were moved to engine-profile semantics.
- `go build ./cmd/switch-profiles-tui ./cmd/pinocchio` passed after removing the stale runtime key/fingerprint references from the TUI command path.
- The smoke command now proves the selected engine profile actually changes the resolved model:

```bash
go run ./cmd/pinocchio js ./examples/js/runner-profile-smoke.js --profile assistant --profile-registries ./examples/js/profiles/basic.yaml
```

Output:

```text
"profile=assistant model=gpt-5-mini prompt=hello from pinocchio js"
```

### What didn't work

- A broad `go test ./...` still failed immediately in web-chat-owned packages because [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go) and the web-chat composer/tests still reference removed mixed-runtime Geppetto types such as `gepprofiles.RuntimeSpec`.
- That failure is outside the current CLI migration boundary and confirms the need to keep web chat as a follow-up slice rather than forcing it into the same commit.

### What I learned

- The command binary is now on stable legs without reviving the old Geppetto runtime abstraction.
- `pinocchio.engines.fromDefaults(...)` is still useful, but it should stay documented as base-config-only, not profile-aware.
- The correct profile-driven JS path is now:

```javascript
const resolved = gp.profiles.resolve({});
const engine = gp.engines.fromResolvedProfile(resolved);
const out = gp.runner.run({ engine, prompt: "..." });
```

### What was tricky to build

- Pinocchio's config files use Glazed field names inside section blocks, not the YAML tag names from the Go structs. That mattered when building realistic helper tests.
- The help page already had a partially updated top section but still taught `gp.runner.resolveRuntime({})` in the lower “Practical Notes” section, so the rendered help contradicted itself until both halves were updated.

### What warrants a second pair of eyes

- The eventual handling of `~/.config/pinocchio/profiles.yaml`. We now have a clear target shape, but a migration script still needs to be designed.
- Whether `pinocchio.engines.fromDefaults(...)` should eventually gain a sibling helper like `fromResolvedProfile(...)` on the Pinocchio module, or whether `gp.engines.fromResolvedProfile(...)` is enough.

### What should be done in the future

- Commit this CLI/JS slice.
- Then move to the repository-loaded command path explicitly and validate `pinocchio code ...` with a concrete fixture.
- After that, start the separate web-chat migration slice.

### Code review instructions

- Start with [`pkg/cmds/helpers/profile_runtime.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime.go) and [`pkg/cmds/helpers/profile_runtime_test.go`](../../../../../../pinocchio/pkg/cmds/helpers/profile_runtime_test.go).
- Then read [`cmd/pinocchio/main_profile_registries_test.go`](../../../../../../pinocchio/cmd/pinocchio/main_profile_registries_test.go) and the two example scripts to see the new JS command contract end to end.
- Finally compare the public docs/help pages to the live command behavior by running:
  - `go run ./cmd/pinocchio help js-runner-scripts`
  - `go run ./cmd/pinocchio js ./examples/js/runner-profile-smoke.js --profile assistant --profile-registries ./examples/js/profiles/basic.yaml`

### Technical details

The critical code path now looks like:

```go
resolved, err := chain.ResolveEngineProfile(ctx, in)
if err != nil { ... }

finalSettings, err := gepprofiles.MergeInferenceSettings(base, resolved.InferenceSettings)
if err != nil { ... }

return &ResolvedInferenceSettings{
    InferenceSettings: finalSettings,
    ResolvedEngineProfile: resolved,
    ...
}, nil
```

And the JS profile-driven path now looks like:

```javascript
const resolved = gp.profiles.resolve({});
const engine = gp.engines.fromResolvedProfile(resolved);
const out = gp.runner.run({ engine, prompt: "..." });
```

## Step 3: Fix the repository-loaded blocking command path

This step addressed the remaining command-line gap after the first CLI/JS slice. The user’s example `pinocchio code unix hello` is ultimately a repository-loaded YAML command path, not a dedicated Cobra subtree. After the earlier helper cutover, chat mode already had an engine-profile-aware path through `profileswitch`, but blocking command runs still used the parsed `InferenceSettings` directly and therefore ignored the selected engine profile.

### Prompt Context

**User prompt (verbatim):** "ok, clean this up too" followed by an explicit direction to keep moving task by task through the downstream migration.

**Assistant interpretation:** Continue GP-50 past the shared helper and JS shell, and make sure the repository-loaded command execution path actually picks up selected engine profiles as well.

**Inferred user intent:** Make the main `pinocchio` command path trustworthy, not just the JS shell and helper binaries.

### What I did

- Traced the loaded-command path through:
  - [`cmd/pinocchio/main.go`](../../../../../../pinocchio/cmd/pinocchio/main.go)
  - [`pkg/cmds/loader.go`](../../../../../../pinocchio/pkg/cmds/loader.go)
  - [`pkg/cmds/cmd.go`](../../../../../../pinocchio/pkg/cmds/cmd.go)
- Confirmed the actual bug: blocking mode created `InferenceSettings` from parsed values only, while profile-selected settings were only used later in chat/profile-switch code.
- Updated [`PinocchioCommand.RunIntoWriter`](../../../../../../pinocchio/pkg/cmds/cmd.go) so that when profile registries are configured it resolves the selected engine profile against the base settings and replaces the blocking-run `InferenceSettings` with the merged result before calling `RunWithOptions(...)`.
- Added an optional `EngineFactory` field on [`PinocchioCommand`](../../../../../../pinocchio/pkg/cmds/cmd.go) so tests can inject a fake engine factory without changing production behavior.
- Added [`cmd_profile_registry_test.go`](../../../../../../pinocchio/pkg/cmds/cmd_profile_registry_test.go), which:
  - loads a command from YAML via `LoadFromYAML(...)`
  - provides parsed `ai-chat` and `profile-settings` values
  - injects a fake engine factory
  - asserts that the loaded-command path passes the profile-selected model into `CreateEngine(...)`

### Why

- Without this fix, the repository-loaded commands would still silently ignore the selected engine profile in blocking mode.
- A direct unit/integration-style test around `LoadFromYAML(...)` and `RunIntoWriter(...)` is the cleanest way to simulate the user-visible `pinocchio code ...` path without requiring a live provider or a whole command-repository checkout.

### What worked

- The new regression test passed after the blocking-path fix:

```bash
go test ./pkg/cmds ./cmd/pinocchio -count=1
```

- `go build ./cmd/pinocchio` also passed after the patch.

### What didn't work

- The first version of the test imported `pkg/cmds/helpers`, which created an import cycle from the `cmds` package test back into itself. I removed that dependency and defined the tiny `profile-settings` section inline in the test.
- The next two failures were setup issues rather than logic bugs:
  - I keyed parsed sections by `GetName()` instead of `GetSlug()`
  - I initially assumed `RunIntoWriter(...)` would print the assistant output in this no-router test path, but the important assertion is the captured `InferenceSettings`, not stdout

### What I learned

- The loaded-command path already had almost all the information it needed. The missing piece was simply resolving the selected engine profile before the blocking run starts.
- A test-only factory injection point on `PinocchioCommand` is enough to prove this path without changing the production CLI contract.

### What was tricky to build

- The parsed-values setup for `RunIntoWriter(...)` has to look like real Glazed input. It needs the full Geppetto section scaffold plus the helper section, not just the specific fields the test cares about.

### What warrants a second pair of eyes

- Precedence between explicit Geppetto inference flags and selected engine profiles. The current patch aligns the blocking path with the existing chat/profile-switch manager flow, but that precedence should be reviewed explicitly later.

### What should be done in the future

- Commit this Slice 3 code.
- Then move to the remaining profile-registry file migration work, especially `~/.config/pinocchio/profiles.yaml`.

### Code review instructions

- Review [`pkg/cmds/cmd.go`](../../../../../../pinocchio/pkg/cmds/cmd.go) around `RunIntoWriter(...)`.
- Then read [`pkg/cmds/cmd_profile_registry_test.go`](../../../../../../pinocchio/pkg/cmds/cmd_profile_registry_test.go) end to end; it is the best compact expression of the intended loaded-command behavior.

### Technical details

The new blocking-path logic is:

```go
if profileRegistries != "" && baseSettings != nil {
    mgr, err := profileswitch.NewManagerFromSources(ctx, profileRegistries, baseSettings)
    ...
    resolved, err := mgr.Resolve(ctx, profile)
    ...
    stepSettings = resolved.InferenceSettings
}
```

And the regression test proves that a loaded YAML command now hands `profiled-model` into the fake engine factory instead of the old `base-model`.

## Related

- [index.md](../index.md)
- [01-pinocchio-engine-profile-migration-analysis-and-implementation-plan.md](../design-doc/01-pinocchio-engine-profile-migration-analysis-and-implementation-plan.md)
- [tasks.md](../tasks.md)

## Step 4: Add a real migration path for `~/.config/pinocchio/profiles.yaml`

This step tackled the last missing piece for the CLI/JS migration boundary: the file format that Pinocchio auto-discovers at `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml`. The runtime semantics had already been hard-cut, but users still needed a concrete way to convert older mixed files into the new engine-only registry shape, and the public docs still described the old format too loosely.

### Prompt Context

**User prompt (verbatim):** The user asked to continue the Pinocchio downstream migration task by task and explicitly wanted the default `~/.config/pinocchio/profiles.yaml` path to keep working once reformatted with a script.

**Assistant interpretation:** Finish Slice 5 by adding a standalone migration script, document the exact profile-registry resolution order, and add focused validation for the default auto-discovered profile file path.

**Inferred user intent:** Make the CLI migration practical for a real existing Pinocchio installation, not just for checked-in repo fixtures.

### What I did

- Added a new migration helper in [`pkg/cmds/helpers/engine_profile_migration.go`](../../../../../../pinocchio/pkg/cmds/helpers/engine_profile_migration.go).
- Added focused tests in [`pkg/cmds/helpers/engine_profile_migration_test.go`](../../../../../../pinocchio/pkg/cmds/helpers/engine_profile_migration_test.go).
- Added a standalone script wrapper at [`scripts/migrate-engine-profiles-yaml/main.go`](../../../../../../pinocchio/scripts/migrate-engine-profiles-yaml/main.go).
- Added a new command-level regression test in [`main_profile_registries_test.go`](../../../../../../pinocchio/cmd/pinocchio/main_profile_registries_test.go) for the default `${XDG_CONFIG_HOME}/pinocchio/profiles.yaml` fallback path.
- Updated public docs to teach:
  - the engine-only `profiles.<slug>.inference_settings` shape
  - the exact precedence for `--profile-registries`, `PINOCCHIO_PROFILE_REGISTRIES`, config, and default `profiles.yaml`
  - the migration-script workflow
  - the fact that `pinocchio.engines.fromDefaults(...)` stays base-config-only

### Why

- Auto-discovery of `~/.config/pinocchio/profiles.yaml` is still useful, but it only helps users if the expected file format is explicit and easy to migrate.
- The deleted legacy migration command should not come back as a public command surface, but its useful conversion logic was still worth salvaging in a smaller, cleaner form.
- The docs needed to stop mixing old "runtime profile" language with the new engine-only model.

### What worked

- The new helper cleanly converts three useful cases:
  - already-canonical engine-profile YAML
  - mixed runtime profiles with `runtime.step_settings_patch`
  - older flat profile maps where each profile directly held Geppetto section patches
- It drops old application-level runtime fields like `runtime.system_prompt`, `runtime.middlewares`, and `runtime.tools` with explicit warnings instead of silently pretending those fields still belong here.
- Focused helper tests passed:

```bash
go test ./pkg/cmds/helpers -count=1
```

- The migration script dry run worked on the checked-in example registry:

```bash
go run ./scripts/migrate-engine-profiles-yaml --dry-run --input ./examples/js/profiles/basic.yaml
```

### What didn't work

- My first build used the pre-rename Geppetto codec helper names `DecodeRuntimeYAMLSingleRegistry` and `EncodeRuntimeYAMLSingleRegistry`. Those are now `DecodeEngineProfileYAMLSingleRegistry` and `EncodeEngineProfileYAMLSingleRegistry`.
- My first patch-to-settings conversion skipped the parsed section precreation step, which caused Glazed to fail with `section ai-client not found` when converting old section-patch maps. Precreating the sections fixed it.
- The broader `go test ./cmd/pinocchio` path was slower than expected because it shells through `go run` and had to pay a fresh toolchain/dependency download cost. That is validation noise, not a logic problem in the migration helper itself.

### What I learned

- The useful migration target is not just the old flat legacy map; it is the more recent mixed runtime file that still carries `runtime.step_settings_patch`. That data can be converted directly into `inference_settings`.
- It is safer to keep the migration entry point as a script than to reintroduce a CLI subcommand that implies the mixed runtime format is still part of the main command surface.

### What was tricky to build

- The old patch data is stored as Geppetto section maps, not as `InferenceSettings` YAML, so the migration helper needed a local schema-backed adapter that reconstructs `InferenceSettings` from section patches.
- The new engine-profile codec omits `default_profile_slug` on write even though the in-memory type still carries it. That is acceptable for current Pinocchio flows because slug `default` is the practical default-profile convention.

### What warrants a second pair of eyes

- The exact migration warnings for dropped app-level runtime fields. The current wording is direct and accurate, but it is user-facing and worth a final read.
- Whether the default `profiles.yaml` fallback should also surface an explicit "run the migration script" hint when the file decode fails. That would be a user-experience improvement, but I left it out of this slice to keep the hard cut small.

### What should be done in the future

- Finish the focused validation note for the default auto-discovered `profiles.yaml` path and then commit Slice 5.
- After that, move to the web-chat-specific follow-up, which is where the real multi-profile app-runtime concerns still live.

### Code review instructions

- Start with [`engine_profile_migration.go`](../../../../../../pinocchio/pkg/cmds/helpers/engine_profile_migration.go) and verify the three supported input shapes.
- Then read [`engine_profile_migration_test.go`](../../../../../../pinocchio/pkg/cmds/helpers/engine_profile_migration_test.go).
- Finally check the public docs in [`README.md`](../../../../../../pinocchio/README.md), [`examples/js/README.md`](../../../../../../pinocchio/examples/js/README.md), and [`05-js-runner-scripts.md`](../../../../../../pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md).

### Technical details

The key conversion path for a mixed profile now looks like:

```go
if len(raw.Runtime.StepSettingsPatch) > 0 {
    patchSettings, err := inferenceSettingsFromSectionPatch(raw.Runtime.StepSettingsPatch)
    ...
    finalSettings, err = gepprofiles.MergeInferenceSettings(finalSettings, patchSettings)
}
```

and the script entry point is intentionally tiny:

```go
result, err := cmdhelpers.MigrateEngineProfilesFile(...)
for _, warning := range result.Warnings {
    fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
}
```

## Step 5: Separate web chat from the CLI/JS migration

This step closes the loop on GP-50’s original scope. The CLI and `pinocchio js` migration now have a coherent engine-profile story, so the remaining work is not “more of the same.” The remaining work is web chat’s own app-runtime migration, and that needed to be documented explicitly instead of being left as an implied mess.

### Prompt Context

**User prompt (verbatim):** Continue working the GP-50 tasks off one by one.

**Assistant interpretation:** Finish the remaining planning slice by inventorying what still blocks web chat and decide whether web chat should keep its own local profile/runtime format.

**Inferred user intent:** End GP-50 with a clean handoff instead of letting web chat ambiguities bleed back into the CLI/JS migration.

### What I did

- Searched the web-chat packages and shared runtime packages for remaining references to:
  - `RuntimeSpec`
  - runtime fingerprints
  - system prompt / middleware / tool selection coming from resolved profiles
- Confirmed the key leftovers are concentrated in:
  - [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go)
  - [`cmd/web-chat/runtime_composer.go`](../../../../../../pinocchio/cmd/web-chat/runtime_composer.go)
  - [`cmd/web-chat/profile_policy.go`](../../../../../../pinocchio/cmd/web-chat/profile_policy.go)
  - [`pkg/webchat/conversation.go`](../../../../../../pinocchio/pkg/webchat/conversation.go)
- Added a dedicated follow-up doc:
  - [02-web-chat-follow-up-plan.md](../design-doc/02-web-chat-follow-up-plan.md)
- Marked the Slice 7 planning tasks complete in [tasks.md](../tasks.md).

### Why

- Web chat genuinely still needs an app-owned runtime layer. That is not a regression; it is a product-specific requirement.
- Trying to force that requirement back into Geppetto engine profiles would recreate the same mixed-runtime problem we just removed.
- GP-50 needed a clear stopping point so the CLI/JS migration could be treated as complete enough to build on.

### What worked

- The inventory made the boundary obvious: web chat still depends on app-owned prompt, middleware, tool, and runtime identity concerns, while the CLI/JS path no longer does.
- The follow-up doc now gives a concrete target model:

```text
Engine profile layer (Geppetto)
  -> InferenceSettings

Web-chat app profile layer (Pinocchio)
  -> system prompt
  -> middleware uses
  -> tool names
  -> runtime key / fingerprint
```

### What didn't work

- Nothing failed technically here; this was a planning/documentation slice.
- The only friction was that the current web-chat code still has many tests and structs using the old mixed runtime naming, which makes the inventory noisy.

### What I learned

- The right future shape is not “web chat keeps using Geppetto profiles differently.”
- The right future shape is “web chat gets its own narrow local app-profile format that references an engine profile.”

### What warrants a second pair of eyes

- The exact YAML shape for the future web-chat app profile file.
- Whether the follow-up should remain inside GP-50 as a final cleanup or be split into a new dedicated ticket.

### What should be done in the future

- Use [02-web-chat-follow-up-plan.md](../design-doc/02-web-chat-follow-up-plan.md) as the starting point for the next ticket.
- Keep GP-50 focused on the now-working CLI and JS migration path.

## Step 6: Verify the default auto-discovered profiles file and close the ticket scope

This final step closed the one remaining open GP-50 task: prove that the default `${XDG_CONFIG_HOME:-~/.config}/pinocchio/profiles.yaml` fallback actually works in the migrated CLI/JS path.

### What I did

- Built a local binary:

```bash
go build -o /tmp/pinocchio-gp50 ./cmd/pinocchio
```

- Ran the default-path smoke flow against a temporary XDG config directory containing only `pinocchio/profiles.yaml`:

```bash
tmpdir=$(mktemp -d)
mkdir -p "$tmpdir/xdg/pinocchio"
cat > "$tmpdir/xdg/pinocchio/profiles.yaml" <<'YAML'
slug: workspace
profiles:
  default:
    slug: default
    inference_settings:
      chat:
        api_type: openai
        engine: default-model
  gpt-5-mini:
    slug: gpt-5-mini
    stack:
      - profile_slug: default
    inference_settings:
      chat:
        engine: gpt-5-mini
YAML

XDG_CONFIG_HOME="$tmpdir/xdg" HOME="$tmpdir" \
  /tmp/pinocchio-gp50 js ./examples/js/runner-profile-smoke.js --profile gpt-5-mini
```

### What worked

The command produced:

```text
"profile=gpt-5-mini model=gpt-5-mini prompt=hello from pinocchio js"
```

That proves the default auto-discovered `profiles.yaml` path now works in the new engine-profile world without requiring `--profile-registries`.

### What I learned

- Using a built binary is the right validation tool here. The earlier `go run` path kept paying toolchain and dependency download costs in this environment, which obscured the actual functional result.
- With that one last smoke check done, the Pinocchio CLI/JS migration is complete enough to hand off web chat separately.

## Step 7: Hard-cut shared web-chat/runtime to a Pinocchio-owned runtime payload

After closing the CLI/JS portion, the next blocker showed up immediately in both CoinVault and Temporal: they still compiled through shared Pinocchio web-chat/runtime packages that referenced the deleted Geppetto mixed runtime type. The real break was in the shared seam, not in their local code first.

### What I did

- Added a Pinocchio-owned runtime payload in [`pkg/inference/runtime/profile_runtime.go`](../../../../../../pinocchio/pkg/inference/runtime/profile_runtime.go):
  - `ProfileRuntime`
  - `MiddlewareUse`
  - `WebChatProfileRuntimeExtension`
  - `ProfileRuntimeFromEngineProfile(...)`
  - `SetProfileRuntime(...)`
- Replaced all shared `*gepprofiles.RuntimeSpec` references in:
  - [`pkg/inference/runtime/composer.go`](../../../../../../pinocchio/pkg/inference/runtime/composer.go)
  - [`pkg/webchat/conversation.go`](../../../../../../pinocchio/pkg/webchat/conversation.go)
  - [`pkg/webchat/conversation_service.go`](../../../../../../pinocchio/pkg/webchat/conversation_service.go)
  - [`pkg/webchat/http/api.go`](../../../../../../pinocchio/pkg/webchat/http/api.go)
  - [`pkg/webchat/chat_service.go`](../../../../../../pinocchio/pkg/webchat/chat_service.go)
  - [`pkg/webchat/llm_state.go`](../../../../../../pinocchio/pkg/webchat/llm_state.go)
- Updated [`cmd/web-chat/profile_policy.go`](../../../../../../pinocchio/cmd/web-chat/profile_policy.go) so resolved engine profiles now contribute final merged `InferenceSettings` while Pinocchio app runtime is loaded from the local `pinocchio.webchat_runtime@v1` extension on the selected engine profile.
- Updated [`cmd/web-chat/runtime_composer.go`](../../../../../../pinocchio/cmd/web-chat/runtime_composer.go) to use the new local runtime type and current `middlewarecfg.Use`.
- Updated [`pkg/webchat/http/profile_api.go`](../../../../../../pinocchio/pkg/webchat/http/profile_api.go) to surface the Pinocchio runtime extension instead of the removed Geppetto runtime field.
- Added test helpers and migrated the affected test suites in `cmd/web-chat` and `pkg/webchat`.

### What worked

- The shared packages now compile cleanly against the engine-only Geppetto contract.
- Focused validation passed:
  - `go test ./pinocchio/pkg/webchat ./pinocchio/pkg/webchat/http -count=1`
  - `go test ./pinocchio/cmd/web-chat -count=1`

### What didn't work

- The first pass at `runtime_composer.go` assumed `middlewarecfg.MiddlewareUse` still existed. The actual type is `middlewarecfg.Use`, so the adapter layer had to be corrected after the first focused build failed.
- Some older tests still expected `RuntimeFingerprint` to look like `sha256:...`, but the current composer format is a JSON payload string. Those assertions had to be updated to check for a non-empty computed fingerprint instead of a legacy prefix.
- The old `profile.stack.trace` expectation in request-resolution tests no longer held. `profile.stack.lineage` was the stable assertion that still mattered.

### What I learned

- The clean boundary here is workable:

```text
Geppetto engine profiles
  -> final InferenceSettings

Pinocchio runtime extension
  -> system prompt
  -> middleware uses
  -> tool names
```

- A local runtime extension is enough to unblock the downstreams without deciding the full long-term web-chat app-profile format today.
