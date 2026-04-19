---
Title: File-by-file merge resolution runbook
Ticket: PIN-20260418-MERGE-BOOTSTRAP-CONFLICT
Status: active
Topics:
    - pinocchio
    - merge
    - configuration
    - bootstrap
    - profiles
DocType: playbooks
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/pinocchio/cmds/js.go
      Note: Runbook marks this as a high-risk manual merge starting from upstream unified-config baseline
    - Path: cmd/pinocchio/main.go
      Note: Runbook defines manual merge guidance and repository-loading checks here
    - Path: cmd/web-chat/main.go
      Note: Runbook marks this as a high-risk runtime consumer merge after the control plane settles
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Runbook uses this file as the anchor for the control-plane resolution strategy
    - Path: ttmp/vocabulary.yaml
      Note: Runbook notes this conflict blocks docmgr doctor until YAML is repaired
ExternalSources: []
Summary: Operational runbook for resolving the active Pinocchio merge conflict, including file-by-file guidance, recommended git commands, and commit boundaries.
LastUpdated: 2026-04-18T16:10:57.079639573-04:00
WhatFor: ""
WhenToUse: ""
---


# File-by-file merge resolution runbook

## Goal

Resolve the active merge conflict in `pinocchio` intentionally and in reviewable tranches, using `origin/main` as the baseline for the bootstrap/config subsystem and replaying only still-missing branch behavior afterward.

## Ground rules

1. In this in-progress merge, `ours` = `task/fix-piniocchio-profile-env`, `theirs` = `origin/main`.
2. For the bootstrap/config subsystem, prefer `theirs` when upstream clearly supersedes the older implementation shape.
3. Do not resurrect deleted helper files unless an active, still-intended caller requires them.
4. Do not commit the whole merge in one shot.
5. Use the local workspace (`go.work`) for validation so local Geppetto/Glazed/Sqleton/Clay code is in effect.

## Commit plan

### Commit 1 — control plane

Target files:
- `pkg/cmds/helpers/parse-helpers.go`
- `pkg/cmds/helpers/profile_selection_test.go`
- `pkg/cmds/profilebootstrap/profile_selection.go`
- `pkg/cmds/cobra.go`
- `cmd/pinocchio/main.go`

Goal:
- resolve the architecture-defining bootstrap/config/repository startup layer first
- ensure the repo builds far enough that downstream runtime consumers can be resolved against a stable baseline

### Commit 2 — runtime consumers and tests

Target files:
- `cmd/web-chat/main.go`
- `cmd/web-chat/main_profile_registries_test.go`
- `cmd/pinocchio/cmds/js.go`
- `cmd/pinocchio/cmds/js_test.go`
- `cmd/examples/simple-chat/main.go`
- `cmd/examples/simple-redis-streaming-inference/main.go`
- `cmd/agents/simple-chat-agent/main.go`

Goal:
- align active command/runtime entrypoints with the resolved control plane
- keep example and test behavior consistent with the final control-plane shape

### Commit 3 — docs/bookkeeping/final cleanup

Target files:
- `README.md`
- `ttmp/vocabulary.yaml`
- any ticket/docs files updated during resolution

Goal:
- reconcile user-facing docs and vocabulary after code behavior is settled
- run the full validation set and record the results

## File-by-file resolution map

## 1. Keep upstream deletions

### `pkg/cmds/helpers/parse-helpers.go`

Recommendation: **take `theirs` (deleted)**

Reason:
- `origin/main` already deleted the helper parser path in `170afd7`.
- Keeping the branch version would resurrect a path the upstream architecture intentionally removed.

Command:

```bash
git rm pkg/cmds/helpers/parse-helpers.go
```

### `pkg/cmds/helpers/profile_selection_test.go`

Recommendation: **take `theirs` (deleted)**

Reason:
- This test targets helper behavior that upstream deliberately removed in `2801b6b`.
- Reviving the test strongly suggests reviving the deleted helper API, which is the wrong direction.

Command:

```bash
git rm pkg/cmds/helpers/profile_selection_test.go
```

## 2. Resolve bootstrap/control-plane files with upstream as baseline

### `pkg/cmds/profilebootstrap/profile_selection.go`

Recommendation: **manual merge, starting from `theirs`**

Baseline to keep from `theirs`:
- `ResolvedUnifiedConfig`
- `ResolveUnifiedConfig(...)`
- `ResolveUnifiedProfileRegistryChain(...)`
- `configdoc` imports and unified-config control plane
- `pinocchioConfigPlanBuilder(...)` with repo/cwd/explicit local override layers

Branch-side pieces to consider reapplying only if still missing:
- none by default; first verify whether the original bugfix intent already survives in the unified-config model

Suggested workflow:

```bash
git checkout --theirs pkg/cmds/profilebootstrap/profile_selection.go
# then inspect manually before git add
```

Manual review checklist:
- confirm no branch-side `ResolveCLIProfileRuntime(...)` wrapper is required by current callers
- confirm the final plan builder still supports the intended explicit config-file path
- confirm implicit profile/env selection semantics are not lost when moving fully to unified config

### `pkg/cmds/cobra.go`

Recommendation: **take `theirs` with a light manual review**

Baseline to keep:
- `GetPinocchioCommandMiddlewares(...)`
- `sources.FromConfigPlanBuilder(...)`
- `BuildCobraCommandWithGeppettoMiddlewares(...)` using `MiddlewaresFunc: GetPinocchioCommandMiddlewares`

Do **not** restore:
- the branch-side parser-config shim that reintroduced `AppName + ConfigPlanBuilder` here if upstream already uses the dedicated middleware builder

Suggested workflow:

```bash
git checkout --theirs pkg/cmds/cobra.go
# inspect, then git add
```

### `cmd/pinocchio/main.go`

Recommendation: **manual merge, starting from `theirs`**

Baseline to keep from `theirs`:
- `ResolveRepositoryPaths()`
- unified config repository loading
- `cli.WithCobraMiddlewaresFunc(cmds.GetPinocchioCommandMiddlewares)`

Discard from branch unless proven necessary:
- manual loop over resolved config files reading top-level `repositories`
- older parser-config shim in `pinocchioParserConfig()`

Suggested workflow:

```bash
git checkout --theirs cmd/pinocchio/main.go
# manually re-add only clearly still-needed branch documentation/comments if applicable
```

Manual review checklist:
- confirm repository loading still matches the intended startup model
- confirm command discovery still uses the same middleware/config semantics as active commands
- confirm no stale import or helper remains from the old repository-loading path

## 3. Resolve runtime consumers against the settled control plane

### `cmd/web-chat/main.go`

Recommendation: **manual merge, starting from `theirs`**

Baseline to keep:
- `ResolveUnifiedConfig(...)`
- `ResolveUnifiedProfileRegistryChain(...)`
- no command-local manual registry-source parsing

Discard:
- branch-side older `ResolveCLIProfileRuntime(...)` flow if the unified-config path already covers the behavior

Command:

```bash
git checkout --theirs cmd/web-chat/main.go
# then manually inspect for any missing behavior
```

### `cmd/pinocchio/cmds/js.go`

Recommendation: **manual merge, starting from `theirs`**

Baseline to keep:
- unified-config bootstrap path
- `ResolveUnifiedConfig(...)`
- `ResolveUnifiedProfileRegistryChain(...)` if current JS runtime bootstrap uses it

Branch-side behavior to re-check, not automatically reapply:
- selected-profile reader wrapping
- any convenience around runtime close handling

This file deserves extra care because it had heavy churn on both sides.

Command:

```bash
git checkout --theirs cmd/pinocchio/cmds/js.go
# then compare against branch intent before git add
```

### `cmd/examples/simple-chat/main.go`

Recommendation: **take `theirs` unless compile/test review shows a missing active dependency**

Reason:
- upstream already cleaned older helper/config-plan shims here
- examples should follow the active public shape, not preserve transitional branch code

### `cmd/examples/simple-redis-streaming-inference/main.go`

Recommendation: **take `theirs`**

Reason:
- the upstream version already points examples at the newer Pinocchio middleware path

### `cmd/agents/simple-chat-agent/main.go`

Recommendation: **take `theirs`**

Reason:
- this appears to be only a no-op parser-config shim conflict (`ConfigFilesFunc`/`ConfigPlanBuilder` vs upstream removal)
- keep the simplified upstream version

## 4. Resolve tests after code settles

### `cmd/pinocchio/cmds/js_test.go`

Recommendation: **manual merge, but bias toward `theirs`**

Reason:
- the branch test reflects the older `ResolveCLIProfileRuntime(...)` shape
- upstream tests should likely assert unified-config behavior instead

### `cmd/web-chat/main_profile_registries_test.go`

Recommendation: **manual merge**

Keep from `theirs`:
- inline profile / unified config assertions
- newer resolved-config expectations

Keep from branch only if still true after code resolution:
- implicit fallback expectations that are still part of the intended behavior

## 5. Resolve docs last

### `README.md`

Recommendation: **manual merge**

Keep from `theirs`:
- unified config language (`profile.active`, current example paths)
- corrected relative example link

Keep from branch if still current after code resolution:
- `inference_settings.api` hard-cut note
- any repository-loading explanation that still matches the final merged code

### `ttmp/vocabulary.yaml`

Recommendation: **manual merge**

Keep both sets of new vocabulary terms if all are still useful:
- branch additions: `bootstrap`, `configuration`, `runtime`, `cli`, `aliases`
- upstream additions: `design`, `sqleton`, `migration`, `cleanup`

Be careful with YAML indentation. This file is currently blocking `docmgr doctor`.

## Recommended command sequence

### Stage A — bootstrap/control plane

```bash
# deletions
git rm pkg/cmds/helpers/parse-helpers.go
git rm pkg/cmds/helpers/profile_selection_test.go

# upstream-baseline files
git checkout --theirs pkg/cmds/profilebootstrap/profile_selection.go
git checkout --theirs pkg/cmds/cobra.go
git checkout --theirs cmd/pinocchio/main.go

# manually inspect/edit, then stage
git add pkg/cmds/profilebootstrap/profile_selection.go pkg/cmds/cobra.go cmd/pinocchio/main.go
```

Validation target before Commit 1:

```bash
cd pinocchio
go test ./pkg/cmds/profilebootstrap ./pkg/configdoc ./pkg/cmds -count=1
```

### Stage B — runtime consumers/tests

```bash
git checkout --theirs cmd/web-chat/main.go
git checkout --theirs cmd/pinocchio/cmds/js.go
git checkout --theirs cmd/examples/simple-chat/main.go
git checkout --theirs cmd/examples/simple-redis-streaming-inference/main.go
git checkout --theirs cmd/agents/simple-chat-agent/main.go

# tests need manual review after code settles
git checkout --theirs cmd/web-chat/main_profile_registries_test.go
git checkout --theirs cmd/pinocchio/cmds/js_test.go
```

Validation target before Commit 2:

```bash
cd pinocchio
go test ./cmd/web-chat ./cmd/pinocchio/cmds ./pkg/cmds/... -count=1
go build -o /tmp/pinocchio-merge-check ./cmd/pinocchio
```

### Stage C — docs/final validation

```bash
# manual merge
$EDITOR README.md
$EDITOR ttmp/vocabulary.yaml
```

Validation target before Commit 3:

```bash
cd pinocchio
go test ./... -count=1
PINOCCHIO_PROFILE=gemini-2.5-pro /tmp/pinocchio-merge-check code professional hello --print-inference-settings
/tmp/pinocchio-merge-check --help
/tmp/pinocchio-merge-check js --help
docmgr doctor --ticket PIN-20260418-MERGE-BOOTSTRAP-CONFLICT --stale-after 30
```

## Commit discipline

Because the user asked for commits at appropriate intervals, use these boundaries:

1. **Commit 1:** control-plane resolution compiles and focused bootstrap/config tests pass
2. **Commit 2:** runtime consumers + focused command tests pass
3. **Commit 3:** docs/final cleanup + broader validation recorded

Until the first tranche is resolved, no normal Git commit is possible because the repository still contains unmerged paths. That is expected; do not force a partial workaround commit outside the merge.

## Quick risk summary

Highest-risk files:
- `pkg/cmds/profilebootstrap/profile_selection.go`
- `cmd/pinocchio/main.go`
- `cmd/web-chat/main.go`
- `cmd/pinocchio/cmds/js.go`

Lowest-risk files:
- `cmd/agents/simple-chat-agent/main.go`
- `cmd/examples/simple-redis-streaming-inference/main.go`
- helper deletions (once accepted)

## Usage example

If you are resuming this work later, start here:

1. Read the assessment doc.
2. Use this runbook to resolve Stage A first.
3. Do **not** touch docs until Stage B is green.
4. Only run `docmgr doctor` after `ttmp/vocabulary.yaml` is conflict-free.

