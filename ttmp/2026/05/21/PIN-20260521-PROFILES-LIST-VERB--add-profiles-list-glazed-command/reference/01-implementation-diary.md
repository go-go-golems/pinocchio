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
    - Path: cmd/pinocchio/main.go
      Note: Future root command wiring point
    - Path: cmd/pinocchio/cmds/clip.go
      Note: Existing simple Glazed command example
    - Path: cmd/pinocchio/cmds/tokens/helpers.go
      Note: Existing command group with Glazed subcommands
    - Path: pkg/cmds/profilebootstrap/profile_introspection.go
      Note: Existing Pinocchio-aware report builder from the flag experiment
    - Path: geppetto/pkg/cli/bootstrap/profile_introspection.go
      Note: Source of the current no-header text renderer and profile report structs
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
