---
Title: add pinocchio js command for geppetto runner scripts
Ticket: GP-48-PINOCCHIO-JS-RUNNER
Status: active
Topics:
    - javascript
    - cli
    - pinocchio
    - geppetto
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Add a first-class `pinocchio js` command that runs Geppetto JavaScript runner scripts with Pinocchio config defaults, profile registry resolution, and a small Pinocchio-owned helper surface."
LastUpdated: 2026-03-18T11:49:35.402459603-04:00
WhatFor: "Use this ticket when implementing or reviewing the Pinocchio CLI entrypoint for JavaScript runner scripts, especially where Pinocchio config defaults and profile registries should feel native."
WhenToUse: "Use when adding the `pinocchio js` command, clarifying the JS runtime bootstrap boundary, or validating the supported script authoring model."
---

# add pinocchio js command for geppetto runner scripts

## Overview

This ticket adds a first-class `pinocchio js` command so Pinocchio users can run JavaScript scripts directly against the Geppetto JS API and the new opinionated runner surface.

The command should feel native to Pinocchio:

- it should respect Pinocchio config/env/default loading for hidden base `StepSettings`
- it should use Pinocchio-style profile registry discovery and the existing `--profile-registries` root flag
- it should expose a small Pinocchio-owned helper module or bootstrap surface where that improves script ergonomics
- it should make the common script workflow simple: load profile runtime, create an engine from Pinocchio defaults, run or stream inference

The important architectural constraint is that this command should not fork a parallel JavaScript shell model. It should be a thin Pinocchio-owned bootstrap around the existing Geppetto JS module and runner APIs.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Design Guide**: [design/01-pinocchio-js-runner-design-and-implementation-guide.md](./design/01-pinocchio-js-runner-design-and-implementation-guide.md)
- **Diary**: [reference/01-manuel-investigation-diary.md](./reference/01-manuel-investigation-diary.md)

## Status

Current status: **active**

Implementation has started. The first working slice is landed locally:

- `pinocchio js` exists and is wired into the root CLI
- `require("pinocchio").engines.fromDefaults()` exists
- local smoke coverage works for profile-backed `gp.runner` scripts
- docs/help cleanup is still pending

## Topics

- javascript
- cli
- pinocchio
- geppetto

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
