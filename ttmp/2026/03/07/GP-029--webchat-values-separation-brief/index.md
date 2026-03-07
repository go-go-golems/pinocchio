---
Title: webchat values separation brief
Ticket: GP-029
Status: closed
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/doc/topics/webchat-values-separation-migration-guide.md
      Note: Dedicated migration guide for moving embeddings from parsed-values constructors to explicit dependency-injected constructors
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Command wiring now also needs to move to strict profile-registry-driven runtime configuration
    - Path: pinocchio/cmd/web-chat/runtime_composer.go
      Note: Web-chat runtime composer should stop taking AI step settings from parsed values
    - Path: pinocchio/pkg/webchat/router.go
      Note: Primary code surface for the Values separation refactor
    - Path: pinocchio/pkg/webchat/stream_backend.go
      Note: Secondary code surface because Redis config parsing currently happens here
    - Path: pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/design-doc/01-webchat-values-separation-brief.md
      Note: Main handoff brief for the refactor
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-07T14:53:59-05:00
WhatFor: Use this ticket as the handoff package for separating Glazed values parsing from Pinocchio webchat router construction.
WhenToUse: Use when implementing or reviewing the Router API cleanup and the dependency-injected embedding boundary for webchat.
---



# webchat values separation brief

## Overview

This ticket now tracks the implementation of the Pinocchio webchat Values-separation refactor: move Glazed `*values.Values` parsing out of `pkg/webchat` core router and server construction while preserving the current architecture where applications own `/chat` semantics and Pinocchio owns the generic SEM/timeline/websocket machinery.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Design Brief**: [design-doc/01-webchat-values-separation-brief.md](./design-doc/01-webchat-values-separation-brief.md)
- **Diary**: [reference/01-diary.md](./reference/01-diary.md)

## Status

Current status: **closed**

## Topics

- webchat
- backend
- http
- runtime

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
