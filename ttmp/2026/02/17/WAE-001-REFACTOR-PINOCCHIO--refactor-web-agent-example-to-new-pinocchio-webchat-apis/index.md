---
Title: Refactor web-agent-example to new Pinocchio webchat APIs
Ticket: WAE-001-REFACTOR-PINOCCHIO
Status: complete
Topics:
    - chat
    - backend
    - refactor
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/17/WAE-001-REFACTOR-PINOCCHIO--refactor-web-agent-example-to-new-pinocchio-webchat-apis/analysis/01-web-agent-example-migration-to-new-pinocchio-webchat-api.md
      Note: Full migration analysis and implementation blueprint
    - Path: /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/pinocchio/ttmp/2026/02/17/WAE-001-REFACTOR-PINOCCHIO--refactor-web-agent-example-to-new-pinocchio-webchat-apis/reference/01-diary.md
      Note: Detailed step-by-step diary
ExternalSources: []
Summary: Analysis ticket for migrating web-agent-example to the post-refactor Pinocchio webchat API boundaries.
LastUpdated: 2026-02-17T00:00:00-05:00
WhatFor: Document migration path and architectural context for external webchat consumers.
WhenToUse: Use before implementing web-agent-example refactor against new runtime/http API package boundaries.
---

# Refactor web-agent-example to new Pinocchio webchat APIs

## Overview

This ticket documents how to migrate `web-agent-example` after recent `pinocchio` webchat API refactors.

The primary artifact is a long-form analysis that:

1. Explains the current `cmd/web-chat` architecture and API boundaries in detail.
2. Defines a concrete migration plan for `web-agent-example` as an external consumer.

## Key Links

- Analysis: `analysis/01-web-agent-example-migration-to-new-pinocchio-webchat-api.md`
- Diary: `reference/01-diary.md`
- Tasks: `tasks.md`
- Changelog: `changelog.md`

## Status

Current status: **complete**

## Topics

- chat
- backend
- refactor

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.
