---
Title: 'Merge conflict assessment: profile-env fix branch versus unified config/bootstrap refactor'
Ticket: PIN-20260418-MERGE-BOOTSTRAP-CONFLICT
Status: active
Topics:
    - pinocchio
    - merge
    - configuration
    - bootstrap
    - profiles
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/pinocchio/main.go
      Note: Highest-density merge conflict with repository-loading semantics and parser wiring decisions
    - Path: cmd/web-chat/main.go
      Note: Representative runtime consumer already moved upstream onto unified profile bootstrap
    - Path: pkg/cmds/cobra.go
      Note: Command middleware seam that must match the final config-plan architecture
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Primary bootstrap seam where branch-side profile-env work collides with upstream unified-config bootstrap
    - Path: pkg/configdoc/resolved.go
      Note: Upstream unified-config document loading layer that should be treated as the baseline
ExternalSources: []
Summary: Assesses the active merge conflict between the profile-env fix branch and origin/main, identifies which side should be treated as the architectural baseline, and turns the result into concrete fix and validation tasks.
LastUpdated: 2026-04-18T16:07:20.802629088-04:00
WhatFor: ""
WhenToUse: ""
---


# Merge conflict assessment

## Executive summary

This merge is noisy, but it is not fundamentally ambiguous. The hard part is not understanding what happened; the hard part is resisting the temptation to preserve both implementations at the same time.

The branch being merged (`task/fix-piniocchio-profile-env`) changed the Pinocchio bootstrap/profile-resolution seam to centralize profile-registry handling in shared Geppetto bootstrap helpers and to remove duplicated command-local registry loading. Meanwhile, `origin/main` landed a larger Pinocchio-owned refactor in the same area: declarative config plans, unified config documents under `pkg/configdoc`, local override layers, repository resolution from unified config, and deletion of older helper paths.

The result is a merge that looks severe textually but is tractable architecturally. The safest resolution strategy is:

1. treat `origin/main` as the baseline for the Pinocchio config/bootstrap architecture,
2. keep the newer unified-config and helper-deletion work,
3. reapply only the branch-specific bugfix intent that is still missing after that baseline is in place,
4. validate against the local workspace (`go.work`) so local Geppetto/Glazed/Clay changes are actually in play.

## Current repository state

### Branch relationship

At merge time:

- Current branch: `task/fix-piniocchio-profile-env`
- Merge target already in progress: `origin/main`
- Unmerged files: 14

Branch-only commits relevant to this conflict:

- `6d2c944` — `bootstrap: remove duplicated profile registry loading`
- `a3e6603` — docs for the above
- `57726ad` — docs + smoke validation
- `66203ab`, `e4291da` — prompt alias ticket/docs and alias-path migration
- `a7ac181`, `d90c14f` — newer docs-only commits from the current ticket work

Main-only commits relevant to the same seam:

- `56bb1f6` — `profilebootstrap: add layered local config plan`
- `3118d0c` — `pinocchio: resolve repositories with config plans`
- `170afd7` — `cmds: drop helper config parser`
- `2801b6b` — `cmds: remove remaining helper paths`
- `703288e` — `profilebootstrap: remove path list wrappers`
- `c6afd24` — `bootstrap: switch profile resolution to unified config`
- `6cf9b41` — `web-chat: use unified profile bootstrap`
- plus the `pkg/configdoc/*` tranche and accompanying docs/tests

### Unmerged files

Code/config conflicts:

- `cmd/agents/simple-chat-agent/main.go`
- `cmd/examples/simple-chat/main.go`
- `cmd/examples/simple-redis-streaming-inference/main.go`
- `cmd/pinocchio/cmds/js.go`
- `cmd/pinocchio/main.go`
- `cmd/web-chat/main.go`
- `pkg/cmds/cobra.go`
- `pkg/cmds/profilebootstrap/profile_selection.go`
- `pkg/cmds/helpers/parse-helpers.go` (`deleted by them`)
- `pkg/cmds/helpers/profile_selection_test.go` (`deleted by them`)

Tests/docs conflicts:

- `README.md`
- `cmd/pinocchio/cmds/js_test.go`
- `cmd/web-chat/main_profile_registries_test.go`
- `ttmp/vocabulary.yaml`

### Conflict concentration

The biggest conflicts by marker density are:

1. `cmd/pinocchio/main.go`
2. `cmd/web-chat/main.go`
3. `cmd/pinocchio/cmds/js.go`
4. `pkg/cmds/cobra.go`
5. `pkg/cmds/profilebootstrap/profile_selection.go`

That is exactly where the architectural seam lives, so the conflict is concentrated rather than scattered.

## Why the merge is hard

## 1. Both sides changed the same abstraction boundary

The feature branch assumed this shape:

- Geppetto owns the shared profile/bootstrap contract.
- Pinocchio provides app identity and config-file mapping.
- Commands should stop validating/loading registries locally.
- `ResolveCLIProfileRuntime(...)` is the shared entry point for commands that need both selection and registry chain.

`origin/main` now assumes a broader Pinocchio-owned control plane:

- Pinocchio owns a unified config document model (`pkg/configdoc`).
- Config and profile selection are resolved through layered config plans and merged documents.
- Inline profiles and imported registries can be composed in Pinocchio before Geppetto resolves the actual engine profile.
- Repository loading is derived from the unified config document.
- Older helper paths were removed as dead ends.

These are not independent changes. They overlap directly.

## 2. Upstream went further than the branch

The branch changed a narrower set of runtime consumers. `origin/main` changed the entire control plane around them. In practice that means many branch-side edits are now superseded, even when the intent was correct.

Examples:

- Branch-side `ResolveCLIProfileRuntime(...)` wrapper work is still conceptually useful, but `origin/main` moved profile selection through `ResolveUnifiedConfig(...)` and `ResolveUnifiedProfileRegistryChain(...)`.
- Branch-side repository/config merge clarification in `cmd/pinocchio/main.go` is now partially obsolete because upstream added `ResolveRepositoryPaths()` and unified app config resolution.
- Branch-side cleanup of `pkg/cmds/helpers/*` overlaps with upstream helper deletion. Trying to preserve both is likely to resurrect dead APIs.

## 3. The workspace still matters

The local workspace `go.work` includes:

- `./geppetto`
- `./glazed`
- `./pinocchio`
- `./sqleton`
- local Clay

That means local validation will not behave like a pure released-module build. This is good for integration testing, but it also means the merge must be checked with workspace-aware commands so that unreleased local changes in Geppetto/Glazed/Clay are actually exercised.

## Severity assessment

### Textual severity

High.

There are many conflicts, and the largest ones are in the highest-churn bootstrap/control-plane files.

### Architectural ambiguity

Medium.

The code does not point in two equally valid directions. `origin/main` is clearly the more recent and broader architecture for Pinocchio. The ambiguity is mostly about how much branch-specific behavior still needs to be replayed, not about which architecture should win.

### Recovery difficulty

Moderate, not extreme.

This is not a rewrite-from-scratch situation. It is a deliberate-baseline merge:

- accept `origin/main` where it clearly supersedes branch code,
- manually integrate only the still-missing bugfix intent,
- run focused validation before attempting broader repo validation.

## Recommended merge strategy

## Rule 1: prefer architectural replacement over hunk preservation

Do **not** try to keep both sides of the old and new bootstrap flow.

Concretely:

- Do not resurrect deleted helper files just because the branch changed them.
- Do not restore path-list config wrapper usage where `origin/main` intentionally removed it.
- Do not keep branch-local registry-loading patterns if upstream already moved the call site onto unified-config bootstrap.

## Rule 2: take `origin/main` as the baseline in the bootstrap/config subsystem

For the active conflict set, the upstream side should generally win for:

- `pkg/cmds/profilebootstrap/profile_selection.go`
- `cmd/pinocchio/main.go`
- `cmd/web-chat/main.go`
- `pkg/cmds/cobra.go`
- most example commands
- helper deletions

Then inspect whether the feature branch still carries behavior not represented in that upstream baseline.

## Rule 3: replay intent, not implementation shape

The feature branch’s valuable intent is:

- profile/env selection should not require commands to manually load registries,
- shared bootstrap behavior should be used consistently,
- docs/tests should prove the original `PINOCCHIO_PROFILE` scenario,
- recent Gemini/debug/docs work should be retained where it still applies.

That intent should be replayed into the unified-config shape rather than copied mechanically from branch files.

## File-by-file resolution guidance

### Likely take `theirs` almost wholesale

These look structurally superseded by upstream:

- `pkg/cmds/helpers/parse-helpers.go` → keep deleted unless a still-active caller proves otherwise
- `pkg/cmds/helpers/profile_selection_test.go` → keep deleted unless the helper layer is intentionally restored
- `cmd/agents/simple-chat-agent/main.go`
- `cmd/examples/simple-chat/main.go`
- `cmd/examples/simple-redis-streaming-inference/main.go`
- `pkg/cmds/cobra.go`

Why: upstream already removed older helper paths and shifted command parsing to the newer config-plan/unified-config model.

### Take `theirs` as baseline, then review for missing branch behavior

These need manual review after starting from upstream:

- `pkg/cmds/profilebootstrap/profile_selection.go`
- `cmd/pinocchio/main.go`
- `cmd/web-chat/main.go`
- `cmd/pinocchio/cmds/js.go`
- `cmd/pinocchio/cmds/js_test.go`
- `cmd/web-chat/main_profile_registries_test.go`

Why: these are the exact files where branch-side profile bootstrap fixes and upstream unified-config refactors both touched runtime behavior.

### Docs/manual merge

- `README.md`
- `ttmp/vocabulary.yaml`

Low technical risk. Manual merge should preserve current operator guidance and current ticket vocabulary additions.

## Key technical questions to answer during resolution

1. Does `origin/main` already preserve the fixed `PINOCCHIO_PROFILE` / `--profile` scenario once merged with local Geppetto? If yes, do not replay older branch wiring just because it was part of the original fix.
2. Does `origin/main` preserve the newer local profile/unified config semantics in `pkg/configdoc`? If yes, treat those semantics as authoritative.
3. Are any branch-side test assertions still testing real active APIs, or were they targeting APIs deleted upstream? Remove the latter instead of reviving them.
4. After the merge, are command-entrypoint middlewares loading config through the same plan semantics as repository resolution and unified config document loading? If not, reconcile that before broader validation.
5. Are local workspace versions of Geppetto/Glazed/Clay being used during validation? If not, the validation result is incomplete.

## Recommended execution order

1. Resolve the helper deletions first.
2. Resolve `pkg/cmds/profilebootstrap/profile_selection.go` next, because the rest of the runtime wiring depends on that shape.
3. Resolve `pkg/cmds/cobra.go` and `cmd/pinocchio/main.go` together, because they define parser/bootstrap/repository startup behavior.
4. Resolve runtime consumers next:
   - `cmd/web-chat/main.go`
   - `cmd/pinocchio/cmds/js.go`
   - examples/agents
5. Resolve tests after the active code paths settle.
6. Merge docs/vocabulary last.
7. Run focused validation before any full-repo validation sweep.

## What would make this merge go badly

These are the failure modes to avoid:

- keeping both old helper paths and the new unified-config paths,
- mixing path-list wrappers back into a codebase that intentionally removed them,
- restoring command-local registry loading in web-chat/JS/examples,
- validating only with released module versions instead of the workspace,
- treating passing tests in one command package as proof that repository startup + command discovery + unified profile bootstrap are all coherent.

## Bottom line

The merge is substantial but not alarming. The codebase is not split between two unrelated designs; it is split between an older narrower fix and a newer broader upstream refactor of the same subsystem.

That means the right response is not to stitch both together. The right response is to let the newer unified-config architecture win, then explicitly verify that the original profile-selection bugfix and the newer local workspace changes still hold inside that architecture.

