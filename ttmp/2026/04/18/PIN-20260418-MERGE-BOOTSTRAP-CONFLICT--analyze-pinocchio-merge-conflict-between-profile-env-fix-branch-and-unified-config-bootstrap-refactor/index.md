---
Title: Analyze pinocchio merge conflict between profile-env fix branch and unified config/bootstrap refactor
Ticket: PIN-20260418-MERGE-BOOTSTRAP-CONFLICT
Status: active
Topics:
    - pinocchio
    - merge
    - configuration
    - bootstrap
    - profiles
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-04-18T16:07:02.501279685-04:00
WhatFor: ""
WhenToUse: ""
---

# Analyze pinocchio merge conflict between profile-env fix branch and unified config/bootstrap refactor

## Overview

This ticket captures the assessment of the active merge conflict between `task/fix-piniocchio-profile-env` and `origin/main` in the Pinocchio repo. The goal is to resolve the conflict intentionally instead of mechanically by choosing the right architectural baseline, identifying which conflicted files are actually superseded by upstream work, and turning the result into an explicit fix + validation checklist.

## Key Links

- [Analysis: merge conflict assessment](./analysis/01-merge-conflict-assessment-profile-env-fix-branch-versus-unified-config-bootstrap-refactor.md)
- [Diary](./reference/01-diary.md)
- [Runbook: file-by-file merge resolution](./playbooks/01-file-by-file-merge-resolution-runbook.md)
- [Task list](./tasks.md)
- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- pinocchio
- merge
- configuration
- bootstrap
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
