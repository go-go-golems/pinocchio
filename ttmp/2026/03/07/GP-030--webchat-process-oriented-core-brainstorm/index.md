---
Title: webchat process-oriented core brainstorm
Ticket: GP-030
Status: active
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Primary code surface if startup is generalized beyond an LLM loop
    - Path: pinocchio/pkg/webchat/router.go
      Note: Primary code surface for keeping transport generic while adjusting startup boundaries
    - Path: pinocchio/ttmp/2026/03/07/GP-030--webchat-process-oriented-core-brainstorm/design-doc/01-webchat-runner-architecture-and-process-oriented-startup-refactor-plan.md
      Note: Detailed implementation guide capturing the current recommendation for Conversation plus Runner
    - Path: pinocchio/ttmp/2026/03/07/GP-030--webchat-process-oriented-core-brainstorm/reference/01-webchat-process-core-brainstorm.md
      Note: Main record of the current architecture discussion
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-07T16:40:00-05:00
WhatFor: Use this ticket to capture and continue the discussion about evolving Pinocchio webchat toward a more generic process-oriented core.
WhenToUse: Use when reviewing or extending the brainstorm around Conversation vs Process abstractions, runner ownership, and generic SEM transport boundaries.
---



# webchat process-oriented core brainstorm

## Overview

This ticket stores the current brainstorm and follow-up design work around a possible Pinocchio refactor: keep the generic `conv_id` transport, websocket, timeline hydration, and SEM projection machinery, but move startup semantics toward an app-owned `Runner` model while keeping `Conversation` as the transport identity.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Brainstorm**: [reference/01-webchat-process-core-brainstorm.md](./reference/01-webchat-process-core-brainstorm.md)
- **Design Guide**: [design-doc/01-webchat-runner-architecture-and-process-oriented-startup-refactor-plan.md](./design-doc/01-webchat-runner-architecture-and-process-oriented-startup-refactor-plan.md)
- **Diary**: [reference/01-diary.md](./reference/01-diary.md)
- **Postmortem**: [reference/02-runner-rebuild-postmortem.md](./reference/02-runner-rebuild-postmortem.md)

## Status

Current status: **active**

## Topics

- webchat
- backend
- pinocchio
- refactor

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design-doc/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
