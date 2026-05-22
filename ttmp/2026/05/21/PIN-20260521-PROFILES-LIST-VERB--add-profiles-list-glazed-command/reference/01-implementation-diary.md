---
Title: Implementation Diary
Ticket: PIN-20260521-PROFILES-LIST-VERB
Status: active
Topics:
    - pinocchio
    - profiles
    - cli
    - glazed
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/cli/bootstrap/profile_introspection.go
      Note: Source of the current no-header text renderer and profile report structs
    - Path: cmd/pinocchio/cmds/clip.go
      Note: Existing simple Glazed command example
    - Path: cmd/pinocchio/cmds/profiles/list.go
      Note: Glazed profiles list command with override/effective settings
    - Path: cmd/pinocchio/cmds/profiles/list_test.go
      Note: Profiles list regression tests
    - Path: cmd/pinocchio/cmds/profiles/root.go
      Note: Profiles command group
    - Path: cmd/pinocchio/cmds/tokens/helpers.go
      Note: Existing command group with Glazed subcommands
    - Path: cmd/pinocchio/main.go
      Note: |-
        Future root command wiring point
        Root wiring and removal of profile introspection flags
    - Path: pkg/cmds/cmd.go
      Note: Removed dynamic command print-profiles early exit
    - Path: pkg/cmds/profilebootstrap/profile_introspection.go
      Note: Existing Pinocchio-aware report builder from the flag experiment
    - Path: pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md
      Note: Updated user docs for profiles list
ExternalSources: []
Summary: Diary for designing a first-class `pinocchio profiles list` Glazed command.
LastUpdated: 2026-05-21T18:05:00-04:00
WhatFor: Track why the profile listing UX moved from early-exit flags to a dedicated Glazed verb.
WhenToUse: Read before implementing or reviewing `pinocchio profiles list`.
---


# Diary

## Goal

This diary captures the design pivot from profile introspection flags to a dedicated `pinocchio profiles list` command. It also records the answer to why the existing text output appears to show `default` in the first column repeatedly.

## Step 1: Create the profiles list verb ticket and implementation guide

The user pointed out that the current text report is confusing because rows appear to start with `default`, and asked for column headers. They then clarified that the better user-facing shape is not flags at all, but a first-class `pinocchio profiles list` Glazed command with a `--verbosity default,detailed,full` control.

I created a new docmgr ticket and wrote an implementation guide focused on replacing the flag-based UX with a Glazed command. The guide keeps the useful Pinocchio-aware profile resolution adapter but moves the user interaction model to rows, columns, headers, and structured Glazed output.

### Prompt Context

**User prompt (verbatim):** "why does it say default in the first column always? Add column headers.

Actually, what we should add instead of the flags is a `pinocchio profiles list` verb (use glazed command), with a --verbosity default,detailed,full flag to show more parameters . Create a new docmgr ticket and add an implementation guide."

**Assistant interpretation:** Explain the confusing current profile row layout, then create a new ticket and guide for replacing the flag-based profile introspection UX with a Glazed `profiles list` command.

**Inferred user intent:** The user wants a clearer, discoverable, table-oriented profile listing command rather than hidden early-exit flags on unrelated verbs.

**Commit (code):** N/A — this step created a design ticket and implementation guide only.

### What I did

- Created ticket `PIN-20260521-PROFILES-LIST-VERB`.
- Added design document `design-doc/01-implementation-guide.md`.
- Replaced `tasks.md` with implementation tasks.
- Added this implementation diary.
- Inspected the current Geppetto renderer to identify why the first visible profile column often says `default`.

### Why

- The flag approach works mechanically but is not the right UX for a user asking to list profiles.
- Glazed rows naturally provide column headers and structured output.
- A `profiles` command group can grow future verbs such as `show`, `resolve`, and `validate`.

### What worked

- The current renderer clearly shows the source of confusion: it prints marker columns followed by `profile.Registry` and `profile.Slug` with no headers.
- Existing Pinocchio/Glazed command examples provide enough patterns for a concrete implementation guide.

### What didn't work

- No code was changed in this step.
- The currently committed flag-based implementation remains in the branch until the new command is implemented or the flag work is reverted/superseded.

### What I learned

- The first text value that often says `default` is the registry slug, not necessarily the profile's default status.
- The row also has marker characters before it: `>` for selected and `*` for default. Those markers are terse and make the unheaded output harder to read.

### What was tricky to build

The tricky part was not the ticket creation; it was phrasing the guide so implementers preserve the correct Pinocchio-specific profile resolution behavior while changing the surface area. The earlier flag implementation added a useful `profilebootstrap.BuildProfileRegistryReport` adapter. The new command should reuse or move that logic rather than falling back to generic Geppetto-only profile loading.

### What warrants a second pair of eyes

- Whether the old flags should be fully removed immediately or left temporarily as undocumented compatibility during the transition.
- The exact `--verbosity full` columns, especially how to represent nested redacted merged settings in Glazed rows.
- Whether `profiles list` should live in a new `cmd/pinocchio/cmds/profiles/` package or a flat `cmd/pinocchio/cmds/profiles.go` file.

### What should be done in the future

- Implement the guide.
- Replace flag tests with `profiles list` command tests.
- Update user-facing documentation once the command exists.

### Code review instructions

- Start with the design doc for expected UX and output columns.
- During implementation, review the command wiring in `cmd/pinocchio/main.go` and the Glazed command definition.
- Validate that table output has headers and JSON output has stable field names.

### Technical details

Current confusing renderer excerpt from `geppetto/pkg/cli/bootstrap/profile_introspection.go`:

```go
fmt.Fprintf(w, "  %s%s %-14s %-24s model=%-16s api=%-18s %s\n",
    selected,
    marker,
    profile.Registry,
    profile.Slug,
    firstNonEmpty(profile.Model, "-"),
    firstNonEmpty(profile.APIType, "-"),
    desc,
)
```

The proposed replacement is one Glazed row per profile with explicit fields:

```text
selected default registry profile display_name model api_type description
```

## Step 2: Implement `pinocchio profiles list`

This step implemented the design as a real Glazed command and removed the flag-based user-facing path. The new command lists one row per profile, so table output has headers and structured output can be produced through Glazed's normal `--output` and `--fields` controls.

The implementation also distinguishes raw profile overrides from effective settings. Raw `override_*` fields come from the profile's own `EngineProfile.InferenceSettings`; effective fields come from resolving the profile stack. This lets operators answer both "what does this profile set?" and "what values will be active after inheritance?"

### Prompt Context

**User prompt (verbatim):** "ok, update the guide, then tasks, then implement, commit at appropriate intervals, and keep a diary"

**Assistant interpretation:** Update the design/tasks to include inference override/effective settings, implement the Glazed command, remove the flag-based UX, validate, commit, and keep the diary current.

**Inferred user intent:** The user wants the profile listing UX implemented now, with enough introspection detail to debug important inference settings such as `chat.engine` and `reasoning_effort`.

**Commit (code):** 1b4aedb — "profiles: add list command"

### What I did

- Added `cmd/pinocchio/cmds/profiles/root.go` with a `profiles` command group.
- Added `cmd/pinocchio/cmds/profiles/list.go` with a Glazed `list` command.
- Registered `profiles` in `cmd/pinocchio/main.go`.
- Removed root `NewProfileIntrospectionSection()` flag wiring from `cmd/pinocchio/main.go`.
- Removed dynamic command `--print-profiles` schema injection and early-exit handling from `pkg/cmds/cmd.go`.
- Removed the previous flag-path test file `pkg/cmds/profile_introspection_test.go`.
- Added `cmd/pinocchio/cmds/profiles/list_test.go` covering:
  - raw override fields;
  - effective inherited fields;
  - selected marker;
  - full JSON-ish settings fields;
  - invalid verbosity validation.
- Updated profile-resolution docs to document `pinocchio profiles list` instead of `--print-profiles`.

### Why

- A dedicated command is clearer than an early-exit flag on unrelated verbs.
- Glazed table output provides headers, which solves the confusing unlabeled `default` registry column.
- Operators need to inspect both profile-declared overrides and inherited/effective settings.

### What worked

- Targeted tests passed:

```bash
go test ./cmd/pinocchio/cmds/profiles ./pkg/cmds ./pkg/cmds/profilebootstrap -count=1
```

- Manual JSON smoke showed selected/default rows and override/effective fields:

```bash
go run ./cmd/pinocchio profiles list \
  --profile-registries $tmp/profiles.yaml \
  --profile mini \
  --verbosity detailed \
  --output json
```

- Manual table smoke showed headers:

```bash
go run ./cmd/pinocchio profiles list --profile-registries $tmp/profiles.yaml
```

The table started with explicit headers:

```text
selected | default | registry | profile | display_name | effective_chat_engine | effective_chat_api_type | reasoning_effort | description
```

### What didn't work

- First compile attempt used `profilebootstrap.ProfileRegistrySummaryReport` and `profilebootstrap.ProfileSummaryReport`, but those aliases did not exist. The compiler error was:

```text
undefined: profilebootstrap.ProfileRegistrySummaryReport
undefined: profilebootstrap.ProfileSummaryReport
```

I fixed this by using the Geppetto bootstrap report types directly for summary structs while still using the Pinocchio wrapper type alias for the top-level report.

### What I learned

- The existing Pinocchio-aware report builder is useful, but the Glazed command needs raw `EngineProfile` values too. Summary rows alone only expose `model` and `api_type` and cannot answer which inference settings a profile explicitly overrides.
- YAML marshaling is preferable for flattening `InferenceSettings` paths because the structs use YAML tags for profile file names such as `chat.engine` and `inference.reasoning_effort`.

### What was tricky to build

The subtle part was keeping raw and effective fields separate. Raw override fields must come from `registry.GetEngineProfile(...).InferenceSettings`; effective fields must come from `registry.ResolveEngineProfile(...)`. If the implementation used only the resolved settings, inherited values would look like explicit overrides. If it used only raw settings, inherited API type and default reasoning settings would disappear.

Another subtle part was replacing, not duplicating, the UX. The earlier flag path was mechanically valid, but leaving it wired would preserve the less clear early-exit command semantics the new ticket is intended to avoid.

### What warrants a second pair of eyes

- Whether full-mode nested fields (`resolution_metadata`, `merged_inference_settings`) render acceptably in all Glazed output modes.
- Whether default output should include `effective_chat_api_type` or keep the table narrower.
- Whether `override_settings_json` should be a string or a nested object for JSON output.

### What should be done in the future

- Consider adding `pinocchio profiles show <profile>` for profile-specific full details without repeating full nested settings on every row.
- Consider adding `pinocchio profiles sources` for registry source diagnostics.

### Code review instructions

- Start with `cmd/pinocchio/cmds/profiles/list.go` and review `buildProfileRow`, `extractOverrideSummary`, and `extractEffectiveSummary`.
- Verify root wiring in `cmd/pinocchio/main.go`.
- Verify the old flag path was removed from `pkg/cmds/cmd.go`.
- Validate with:
  - `go test ./cmd/pinocchio/cmds/profiles ./pkg/cmds ./pkg/cmds/profilebootstrap -count=1`
  - `go run ./cmd/pinocchio profiles list --profile-registries ./examples/js/profiles/basic.yaml --verbosity detailed`
  - `go test ./... -count=1`

### Technical details

The key invariant is:

```text
raw EngineProfile.InferenceSettings -> override_* fields
resolved EngineProfile stack          -> effective_* fields
```

The command currently emits default columns:

```text
selected, default, registry, profile, display_name,
effective_chat_engine, effective_chat_api_type, reasoning_effort, description
```

## Step 3: Add `pinocchio profiles show`

The list command is useful for comparing profiles, but full nested settings are awkward when repeated on every row. This step adds a focused `show` verb that emits exactly one profile row using the same raw override and effective setting extraction logic as `profiles list`.

The new verb accepts a plain profile slug, a `registry/profile` reference, or no argument. With no argument, it shows the selected/default profile. This keeps quick operator usage short while still supporting explicit registry disambiguation.

### Prompt Context

**User prompt (verbatim):** "add the show verb."

**Assistant interpretation:** Add `pinocchio profiles show` to inspect a single profile in detail using the existing Glazed profile introspection infrastructure.

**Inferred user intent:** The user wants a better place for full profile details than repeating nested settings for every row in `profiles list`.

**Commit (code):** 41be9ce — "profiles: add show command"

### What I did

- Added `cmd/pinocchio/cmds/profiles/show.go`.
- Wired `show` into `cmd/pinocchio/cmds/profiles/root.go`.
- Added `cmd/pinocchio/cmds/profiles/show_test.go`.
- Updated user docs to mention `pinocchio profiles show`.
- Updated ticket tasks and design guide references.

### Why

- `profiles list --verbosity full` can be too wide for table output.
- A focused `show` command lets users inspect one profile's overrides, resolved effective settings, lineage, and settings JSON through normal Glazed output.

### What worked

- `profiles show workspace/mini --profile-registries $tmp/profiles.yaml --output json` emitted exactly one row.
- The row included raw override fields such as `override_chat_engine` and `override_inference_reasoning_effort`.
- The row included inherited effective fields such as `effective_chat_api_type`.

### What didn't work

- N/A in implementation; the initial approach reused the existing row builder cleanly.

### What I learned

- The extraction split from Step 2 made `show` small: the command only needed target resolution and one-row emission.

### What was tricky to build

The main detail was target resolution. The command accepts either `profile`, `registry/profile`, or no argument. The implementation resolves those inputs against the selected/default registry and profile from the report so the default behavior matches `profiles list` selection semantics.

### What warrants a second pair of eyes

- Whether `profiles show` should default to `detailed` or `full`. It currently defaults to `detailed` to avoid very wide table output while still showing override/effective fields.
- Whether the positional argument should be renamed from `profile-ref` to `profile` in help output.

### What should be done in the future

- Consider adding examples in embedded help docs if a dedicated profiles help page is added.

### Code review instructions

- Review `cmd/pinocchio/cmds/profiles/show.go` target resolution and row reuse.
- Validate with:
  - `go test ./cmd/pinocchio/cmds/profiles -count=1`
  - `go run ./cmd/pinocchio profiles show workspace/mini --profile-registries $tmp/profiles.yaml --output json`

### Technical details

Supported forms:

```bash
pinocchio profiles show
pinocchio profiles show mini --registry workspace
pinocchio profiles show workspace/mini
```

## Step 4: Address PR review for `profiles show --registry`

A PR review pointed out that `profiles show --registry <other>` with no profile argument combined the requested registry with the globally selected/default profile slug. That could show the wrong profile or fail if the selected profile slug did not exist in the requested registry.

I changed target resolution so an explicit registry with an omitted profile uses that registry's own default profile. The no-argument case still uses the globally selected/default profile, and explicit `registry/profile` references still win over `--registry`.

### Prompt Context

**User prompt (verbatim):** "Address PR review comments: https://github.com/go-go-golems/pinocchio/pull/155"

**Assistant interpretation:** Fetch PR review comments, identify requested changes, implement fixes, validate, and document the work.

**Inferred user intent:** The user wants the automated review feedback resolved cleanly on the PR branch.

**Commit (code):** 0c96569 — "profiles: fix show registry default resolution"

### What I did

- Fetched PR 155 inline comments via GitHub CLI.
- Updated `resolveShowTarget` in `cmd/pinocchio/cmds/profiles/show.go`.
- Added `defaultProfileForRegistry` helper.
- Added tests for:
  - `profiles show --registry other` choosing `other` registry's default profile;
  - no registry/no profile still choosing the selected profile.

### Why

- An explicit registry should not be paired with a profile slug selected in another registry when the user omits the profile argument.
- The command should be predictable for multi-registry setups.

### What worked

- Targeted tests passed:

```bash
go test ./cmd/pinocchio/cmds/profiles -count=1
go test ./cmd/pinocchio/cmds/profiles ./pkg/cmds ./pkg/cmds/profilebootstrap -count=1
```

### What didn't work

- N/A. The review comment was precise and the target-resolution logic was isolated.

### What I learned

- `--registry` without a profile should be interpreted as "show this registry's default profile", not "show the global selected profile in this registry".

### What was tricky to build

The key distinction is whether the registry was explicit. If the registry is explicit and the profile is omitted, derive the profile from that registry. If neither is explicit, use the selected/default global target.

### What warrants a second pair of eyes

- Whether `profiles show --registry other` should error when `other` has no default profile, or whether it should fall back to global selected profile. The current behavior errors, which is safer.

### What should be done in the future

- N/A.

### Code review instructions

- Review `resolveShowTarget` in `cmd/pinocchio/cmds/profiles/show.go`.
- Review the new target-resolution tests in `cmd/pinocchio/cmds/profiles/show_test.go`.

### Technical details

Resolution now follows:

```text
registry/profile argument -> exact target
profile + --registry      -> requested registry + profile
--registry only           -> requested registry + that registry's default profile
no target flags/args      -> selected/default registry + selected/default profile
```
