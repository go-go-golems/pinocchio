---
Title: Reusable Pinocchio Web Chat (Go server + React frontend)
Ticket: PI-02-REUSABLE-PINOCCHIO-WEBCHAT
Status: active
Topics:
    - webchat
    - react
    - frontend
    - pinocchio
    - refactor
    - thirdparty
    - websocket
    - http-api
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat
      Note: Example app wiring + React UI source (primary extraction source)
    - Path: pinocchio/pkg/webchat
      Note: Reusable webchat backend core (primary reuse target)
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T08:19:04.261542278-05:00
WhatFor: ""
WhenToUse: ""
---


# Reusable Pinocchio Web Chat (Go server + React frontend)

## Overview

Goal: enable a third-party Go module/app to reuse Pinocchio’s **web-chat** end-to-end: the Go backend (HTTP + WebSocket + persistence + SEM translation) and the React frontend (widgets, state, and extension points), without importing `pinocchio/cmd/...`.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Primary design doc**: `design-doc/01-reusable-pinocchio-web-chat-analysis-extraction-guide.md`
- **Diary**: `reference/01-diary.md`
- **Copy/paste recipes**: `reference/02-third-party-web-chat-reuse-copy-paste-recipes.md`
- **Existing upstream docs**: `pinocchio/pkg/doc/topics/webchat-overview.md`, `pinocchio/pkg/doc/topics/webchat-framework-guide.md`, `pinocchio/pkg/doc/topics/webchat-frontend-architecture.md`

## Status

Current status: **active**

## Topics

- webchat
- react
- frontend
- pinocchio
- refactor
- thirdparty
- websocket
- http-api

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
