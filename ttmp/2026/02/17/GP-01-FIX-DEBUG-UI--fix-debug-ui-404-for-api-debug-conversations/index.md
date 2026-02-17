---
Title: Fix debug UI 404 for /api/debug/conversations
Ticket: GP-01-FIX-DEBUG-UI
Status: active
Topics:
    - bug
    - analysis
    - chat
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Investigate and fix debug-ui 404s for /api/debug/conversations across debug-gating, root-prefix, and dev-proxy configurations.
LastUpdated: 2026-02-17T10:45:00-05:00
WhatFor: Track diagnosis and implementation tasks for GP-01-FIX-DEBUG-UI.
WhenToUse: Use when implementing or reviewing fixes for debug endpoint reachability in cmd/web-chat.
---

# Fix debug UI 404 for /api/debug/conversations

## Overview

This ticket captures investigation and remediation work for a debug UI 404 on `/api/debug/conversations`.

Initial findings show three plausible failure modes:
- debug routes disabled unless `--debug-api` is set
- frontend absolute `/api/debug/*` calls breaking when backend is mounted under `--root /chat`
- Vite proxy defaulting to `:8080` when backend runs on `:8081`

See analysis and diary docs for command-level evidence.

## Investigation Docs

- [Analysis: Debug UI 404 Investigation](./analysis/01-debug-ui-404-investigation-api-debug-conversations.md)
- [Diary](./diary/01-diary.md)

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- bug
- analysis
- chat
- backend

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
