---
Title: Investigate noisy nested prompt alias warnings during Pinocchio command repository load
Ticket: PIN-20260418-PROMPT-ALIAS-RESOLUTION
Status: active
Topics:
    - pinocchio
    - glazed
    - aliases
    - bootstrap
    - cli
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-18T14:48:11.715914433-04:00
WhatFor: ""
WhenToUse: ""
---

# Investigate noisy nested prompt alias warnings during Pinocchio command repository load

## Overview

This ticket tracks the noisy startup warnings emitted while Pinocchio loads nested prompt aliases such as:

- `alias concise-doc (prefix: [code go], source ...) for go not found`

Initial investigation shows this is **not** a command-loading-order problem. Commands are inserted before aliases. The current failure comes from how nested alias files inherit their directory path as parents, while alias resolution then looks up `alias.Parents + aliasFor`, producing paths like `code go go` instead of the real target path `code go`.

The main goal of this ticket is to decide and enforce the intended contract for nested aliases:

- should alias files only target commands under the same prefix, or
- should nested alias files be allowed to refer to the parent command one level up?

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- pinocchio
- glazed
- aliases
- bootstrap
- cli

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
