---
Title: Implementation diary
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
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological diary for replacing split selection/runtime bootstrap APIs with a canonical runtime API."
LastUpdated: 2026-04-18T16:50:00-04:00
WhatFor: "Record the implementation sequence, validation commands, and reasoning for the canonical runtime API refactor."
WhenToUse: "Use when reviewing, resuming, or auditing this refactor."
---

# Diary

## Goal

Record the implementation of the canonical profile runtime API refactor that removes selection wrappers and makes runtime resolution the single source of truth.

## Step 1: Create the ticket, define the target API, and start implementation

The user asked for the clean version, not a soft migration. I started by creating a dedicated ticket so the refactor has its own design, tasks, and diary rather than being hidden inside earlier profile-fix and merge-conflict tickets. The design focus is to remove the selection/runtime split, remove Pinocchio wrapper aliases, and make the runtime object the only authoritative profile bootstrap result.

This step intentionally happens before code changes. The API break is large enough that the design and task list need to exist first so the implementation can be reviewed as a deliberate contract change instead of a series of unrelated edits.

### Prompt Context

**User prompt (verbatim):** "ok, create a ticket and add a detailed implementation document and add detailed tasks to the ticket, and then implement them. No backwards compatibility, no wrappers, just the clean new API."

**Assistant interpretation:** Create a fresh docmgr ticket for the API cleanup, write a concrete implementation plan and task list, then perform the refactor directly without compatibility shims.

**Inferred user intent:** Replace the ambiguous bootstrap API surface now, not later, and make the resulting contract simple enough that callers cannot accidentally choose the wrong abstraction.

**Commit (code):** pending

### What I did
- Created ticket `PIN-20260418-CANONICAL-PROFILE-RUNTIME-API`.
- Added a design doc for the target API.
- Added this diary.
- Prepared to update the ticket tasks and implement the refactor in Geppetto and Pinocchio.

### Why
- The user explicitly requested a ticket plus detailed implementation docs before code changes.
- This refactor changes public bootstrap contracts in two repos, so the reasoning should be captured before edits begin.

### What worked
- `docmgr ticket create-ticket` created the workspace cleanly.
- `docmgr doc add` created the design and diary documents.

### What didn't work
- N/A

### What I learned
- The cleanest API will require removing concrete type aliasing between Pinocchio and Geppetto, not just renaming one function.

### What was tricky to build
- The tricky part is not implementation mechanics; it is choosing boundaries that are actually clean. Geppetto owns generic registry runtime resolution, while Pinocchio owns unified config documents and inline profiles. The final API has to respect that split without reintroducing selection wrappers.

### What warrants a second pair of eyes
- The exact shape of the new inference-debug input contract.
- Whether any external callers outside this workspace still depend on the old Geppetto bootstrap symbols.

### What should be done in the future
- Implement the refactor.
- Update tests/docs.
- Run focused and broad validation.

### Code review instructions
- Start with the design doc: `../design-doc/01-canonical-profile-runtime-api-without-selection-wrappers.md`
- Then review the implementation commits in Geppetto and Pinocchio.

### Technical details
- Ticket path: `ttmp/2026/04/18/PIN-20260418-CANONICAL-PROFILE-RUNTIME-API--replace-split-profile-selection-runtime-apis-with-a-canonical-profile-runtime-api`
- Key code areas expected to change:
  - `geppetto/pkg/cli/bootstrap/*`
  - `pinocchio/pkg/cmds/profilebootstrap/*`
  - `pinocchio/pkg/cmds/cmd.go`
  - `pinocchio/cmd/web-chat/main.go`
  - `pinocchio/cmd/pinocchio/cmds/js.go`

## Step 2: Remove the split APIs, migrate call sites, and validate the canonical runtime path

I implemented the refactor directly instead of staging a compatibility layer. In Geppetto, the public selection-only contract is gone. `ResolveCLIProfileRuntime(...)` now directly owns profile-settings resolution, implicit default registry fallback, and the registry chain. `ResolvedCLIEngineSettings` no longer duplicates `ProfileSelection`, and inference-debug now consumes a smaller resolved-inference payload instead of a concrete bootstrap wrapper.

In Pinocchio, I removed the public split between `ResolveCLIProfileSelection(...)`, `ResolveUnifiedConfig(...)`, and `ResolveUnifiedProfileRegistryChain(...)`. The public API now centers on `ResolveCLIProfileRuntime(...)`, and the engine-settings result is Pinocchio-owned instead of a Geppetto alias. The command runner, web-chat, and JS bootstrap paths now consume runtime-first data. Focused tests, broad tests, an isolated fallback smoke run, and a real runtime smoke run all passed after one round of test-fixup.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Implement the clean runtime-first API right away and update everything that still depends on the split selection/runtime contract.

**Inferred user intent:** Make the API impossible to misuse by deleting the misleading public selection layer instead of merely renaming it.

**Commit (code):** `0947110` — `bootstrap: remove split profile selection api`

**Commit (code):** `3632afd` — `bootstrap: adopt canonical profile runtime api`

### What I did
- Geppetto:
  - removed `ResolvedCLIProfileSelection`
  - removed `ResolveCLIProfileSelection(...)`
  - removed `ResolveEngineProfileSettings(...)`
  - refactored `ResolveCLIProfileRuntime(...)` to resolve settings directly
  - added `PrepareProfileSettingsForRuntime(...)` for clean shared fallback normalization
  - removed `ProfileSelection` from `ResolvedCLIEngineSettings`
  - changed inference-debug to accept `ResolvedInferenceTrace`
  - updated bootstrap tests and the migration tutorial
- Pinocchio:
  - replaced the public split helper surface with one `ResolveCLIProfileRuntime(...)`
  - moved unified-config merging into a private config-runtime helper
  - made Pinocchio own `ResolvedCLIEngineSettings`
  - migrated `pkg/cmds/cmd.go`, `cmd/web-chat/main.go`, and `cmd/pinocchio/cmds/js.go`
  - updated tests to assert runtime-first behavior instead of selection-only behavior
- Validation:
  - `cd geppetto && go test ./pkg/cli/bootstrap -count=1`
  - `cd pinocchio && go test ./pkg/cmds/profilebootstrap ./pkg/cmds ./cmd/pinocchio/cmds ./cmd/web-chat -count=1`
  - `cd pinocchio && go test ./... -count=1`
  - `cd pinocchio && go build -o /tmp/pinocchio-profile-runtime ./cmd/pinocchio`
  - isolated fallback smoke using a temporary `XDG_CONFIG_HOME` with only `profiles.yaml`
  - real runtime smoke with `PINOCCHIO_PROFILE=gemini-2.5-pro`

### Why
- The old API shape let callers validate the wrong abstraction.
- Runtime resolution is the only layer that can honestly answer whether a selected profile is executable.
- Pinocchio’s old aliases made ownership and contracts blurrier than necessary.

### What worked
- The canonical runtime API compiled and passed focused tests quickly once the call sites were migrated.
- The isolated smoke run proved the implicit `${XDG_CONFIG_HOME}/pinocchio/profiles.yaml` fallback still works through the new runtime-first API.
- The real runtime smoke also worked after the refactor.

### What didn't work
- The first geppetto runtime test failed because the runtime API now opens configured registry files, so the old env-prefix test needed a real temp registry file instead of just a string path.
- The first layering tests in both Geppetto and Pinocchio failed because they previously only asserted profile-selection precedence without providing any resolvable runtime source. I fixed those tests by giving them actual registries or inline profiles so they validate the runtime contract honestly.
- The real runtime smoke still showed alias warnings from external prompt repositories (`corporate-headquarters`, `ttc`, `wesen-misc`), but the command itself succeeded. Those warnings are outside this API refactor.

### What I learned
- Removing the selection API forces tests to become more honest. If a test wants to validate runtime behavior, it must provide a resolvable runtime source.
- The clean boundary is: Geppetto owns generic registry-runtime normalization; Pinocchio owns unified documents and inline profile composition.

### What was tricky to build
- The sharp edge was implicit fallback plus inline profiles. Pinocchio cannot simply call the generic Geppetto runtime constructor too early, because Geppetto would reject `profile != ""` with no registries before Pinocchio has a chance to contribute inline profiles. The clean fix was to separate shared profile-settings preparation (including default fallback normalization) from Pinocchio’s inline-profile-aware registry composition.

### What warrants a second pair of eyes
- Whether any downstream repos outside this workspace still import the removed Geppetto symbols.
- Whether the inference-debug helper should eventually accept an interface rather than a small struct, or whether the current small-struct contract is the right amount of explicitness.

### What should be done in the future
- Optional: scan external prompt repositories for the still-unmigrated nested alias warnings surfaced by the real runtime smoke.
- Optional: add a dedicated integration test that shells out to a temporary `profiles.yaml`-only setup so the fallback contract is guarded at the CLI level.

### Code review instructions
- Start in Geppetto:
  - `geppetto/pkg/cli/bootstrap/profile_runtime.go`
  - `geppetto/pkg/cli/bootstrap/profile_selection.go`
  - `geppetto/pkg/cli/bootstrap/engine_settings.go`
  - `geppetto/pkg/cli/bootstrap/inference_debug.go`
- Then review Pinocchio:
  - `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`
  - `pinocchio/pkg/cmds/profilebootstrap/engine_settings.go`
  - `pinocchio/pkg/cmds/cmd.go`
  - `pinocchio/cmd/web-chat/main.go`
  - `pinocchio/cmd/pinocchio/cmds/js.go`
- Re-run:
  - `cd geppetto && go test ./pkg/cli/bootstrap -count=1`
  - `cd pinocchio && go test ./... -count=1`
  - `cd pinocchio && go build -o /tmp/pinocchio-profile-runtime ./cmd/pinocchio`
  - run an isolated `profiles.yaml` fallback smoke

### Technical details
- New authoritative shared object in Geppetto: `ResolvedCLIProfileRuntime`
- New authoritative Pinocchio object: `ResolvedCLIProfileRuntime`
- New inference-debug payload: `ResolvedInferenceTrace`
- Removed public split APIs:
  - `ResolveCLIProfileSelection(...)`
  - `ResolveEngineProfileSettings(...)`
  - Pinocchio’s public `ResolveUnifiedConfig(...)`
  - Pinocchio’s public `ResolveUnifiedProfileRegistryChain(...)`
