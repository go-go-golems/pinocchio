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
  - [`scripts/profile-infer-once.go`](../../../../../../pinocchio/scripts/profile-infer-once.go)
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

## Related

- [index.md](../index.md)
- [01-pinocchio-engine-profile-migration-analysis-and-implementation-plan.md](../design-doc/01-pinocchio-engine-profile-migration-analysis-and-implementation-plan.md)
- [tasks.md](../tasks.md)
