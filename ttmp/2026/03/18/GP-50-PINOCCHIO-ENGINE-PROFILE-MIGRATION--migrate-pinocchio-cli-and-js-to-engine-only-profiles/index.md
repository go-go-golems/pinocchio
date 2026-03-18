---
Title: Migrate Pinocchio CLI and JS to engine-only profiles
Ticket: GP-50-PINOCCHIO-ENGINE-PROFILE-MIGRATION
Status: active
Topics:
    - pinocchio
    - migration
    - config
    - js-bindings
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-18T16:22:21.315677251-04:00
WhatFor: ""
WhenToUse: ""
---

# Migrate Pinocchio CLI and JS to engine-only profiles

## Overview

This ticket migrates Pinocchio downstream of Geppetto's engine-profile hard cut. The immediate focus is the `cmd/pinocchio` binary, especially repository-loaded commands such as `pinocchio code unix hello` and the `pinocchio js` command. Web chat comes later because it still has a genuinely app-level multi-profile runtime model.

The migration goal is to make Pinocchio resolve engine profiles into final `InferenceSettings` before command execution, while treating prompts, middlewares, tools, and runtime identity as Pinocchio-owned concerns. That means Pinocchio must stop depending on Geppetto's removed mixed runtime/profile model and must reformat any remaining profile YAML fixtures and user-facing examples.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- pinocchio
- engineprofiles
- cli
- javascript

## Tasks

See [tasks.md](./tasks.md) for the current task list.

The current implementation order is:

1. Fix shared CLI inference-settings bootstrap so engine profiles actually affect command execution.
2. Migrate `pinocchio js` to the new engine-profile model and restore a real inference example.
3. Add or update a profile-registry conversion script for old mixed-runtime YAML.
4. Hand web chat off to a separate app-runtime migration plan rather than forcing it into the CLI/JS cutover.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
