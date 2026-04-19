---
Title: Replace split profile selection/runtime APIs with a canonical profile runtime API
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
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/pinocchio/cmds/js.go
      Note: JS runner bootstrap now consumes canonical profile runtime data instead of rebuilding registry state separately
    - Path: cmd/web-chat/main.go
      Note: Web-chat command migrated from split unified-config helpers to the canonical profile runtime API
    - Path: cmd/web-chat/main_profile_registries_test.go
      Note: Runtime-first tests now validate fallback and inline-profile semantics through the new public API
    - Path: pkg/cmds/cmd.go
      Note: Main command runner consumes the old selection object and must migrate to runtime-first data
    - Path: pkg/cmds/profilebootstrap/engine_settings.go
      Note: Pinocchio engine settings currently reconstruct runtime state from split helpers
    - Path: pkg/cmds/profilebootstrap/profile_selection.go
      Note: Pinocchio unified-config bootstrap split that will collapse into one runtime API
ExternalSources: []
Summary: Track the clean API refactor that removes selection wrappers and makes runtime resolution the single source of truth for profile bootstrap state.
LastUpdated: 2026-04-18T16:50:00-04:00
WhatFor: Coordinate design, implementation, validation, and review for the canonical profile runtime API refactor across Geppetto and Pinocchio.
WhenToUse: Use when implementing, reviewing, or resuming the canonical profile runtime API cleanup.
---





# Replace split profile selection/runtime APIs with a canonical profile runtime API

## Overview

This ticket tracks a deliberate API cleanup across Geppetto and Pinocchio. The current bootstrap surface splits profile resolution into a lightweight selection view and a deeper runtime view. That makes it too easy for callers and tests to read an incomplete answer and assume it is authoritative. The target state is a clean runtime-first API with no backward compatibility wrappers and no public selection-only resolver.

## Key Links

- Design: [design-doc/01-canonical-profile-runtime-api-without-selection-wrappers.md](./design-doc/01-canonical-profile-runtime-api-without-selection-wrappers.md)
- Diary: [reference/01-implementation-diary.md](./reference/01-implementation-diary.md)
- Tasks: [tasks.md](./tasks.md)
- Changelog: [changelog.md](./changelog.md)

## Current status

Current status: **active**

### Completed so far

- Ticket workspace created.
- Detailed design document added.
- Implementation diary started.
- Geppetto bootstrap API cleaned up to use a single canonical profile-runtime resolver.
- Pinocchio bootstrap API cleaned up to use a single canonical runtime-first resolver.
- Main Pinocchio call sites and tests migrated.
- Focused tests, broad tests, build, isolated fallback smoke, and real runtime smoke all passed.

### In progress

- Final ticket bookkeeping and review.

## Scope

### In scope

- Geppetto bootstrap runtime API cleanup
- Pinocchio unified-config runtime API cleanup
- engine-settings contract cleanup
- inference-debug contract cleanup
- call-site/test/doc migration

### Out of scope

- backward compatibility shims
- unrelated webchat runtime policy refactors
- external prompt repository alias cleanup

## Tasks

See [tasks.md](./tasks.md) for the detailed implementation checklist.

## Structure

- `design-doc/` — architecture and implementation plan
- `reference/` — diary and review context
- `playbooks/` — optional runbooks if the refactor needs one
- `scripts/` — temporary validation helpers if needed
