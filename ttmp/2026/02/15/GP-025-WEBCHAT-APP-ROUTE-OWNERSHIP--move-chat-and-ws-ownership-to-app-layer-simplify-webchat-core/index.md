---
Title: Move /chat and /ws ownership to app layer; simplify webchat core
Ticket: GP-025-WEBCHAT-APP-ROUTE-OWNERSHIP
Status: complete
Topics:
    - architecture
    - webchat
    - refactor
    - api
    - routing
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Move route ownership out of `pkg/webchat` and refactor webchat core into reusable conversation/runtime primitives with lower abstraction overhead.
LastUpdated: 2026-02-15T17:46:56.602209312-05:00
WhatFor: Track analysis and implementation planning for route ownership inversion and router simplification.
WhenToUse: Use when designing or implementing the GP-025 webchat architecture cutover.
---


# Move /chat and /ws ownership to app layer; simplify webchat core

## Overview
GP-025 analyzes how to simplify webchat architecture by moving `/chat` and `/ws` route ownership to end applications and reducing `pkg/webchat` to a toolkit of reusable primitives.

The primary architectural thesis is that current complexity is caused by centralized route ownership plus many extension seams. A cleaner model is explicit app-level route handling and a smaller core API.

## Key Links

- Design analysis: `design/01-webchat-toolkit-app-owned-routes-analysis.md`
- Detailed diary: `reference/01-diary.md`
- Tasks: `tasks.md`
- Changelog: `changelog.md`

## Status
Current status: **active**

## Topics

- architecture
- webchat
- refactor
- api
- routing

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
