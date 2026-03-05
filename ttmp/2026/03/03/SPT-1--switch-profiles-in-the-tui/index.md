---
Title: Switch profiles in the TUI
Ticket: SPT-1
Status: active
Topics:
    - tui
    - profiles
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T16:36:36.987067954-05:00
WhatFor: ""
WhenToUse: ""
---

# Switch profiles in the TUI

## Overview

Add “runtime profile” switching to the terminal chat UI, replacing the current Geppetto/Glazed config-layer middleware approach with explicit **profile + profile-registry** selection.

Core requirements:

- Remove the current “Geppetto middlewares” CLI-layer configuration plumbing and replace it with profile selection (`--profile`) plus registry-source selection (`--profile-registries`).
- Add a `/profile` slash command and a modal picker to switch profiles mid-conversation.
- Persist profile attribution so stored turns/timeline snapshots can answer: “which profile was active for this message/turn?”

## Key Links

- Primary design doc: `design-doc/01-design-profile-switching-in-switch-profiles-tui.md`
- Investigation diary: `reference/01-investigation-diary.md`
- Task checklist: `tasks.md`

## Status

Current status: **active**

## Topics

- tui
- profiles

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
