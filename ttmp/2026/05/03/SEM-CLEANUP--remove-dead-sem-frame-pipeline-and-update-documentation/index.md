---
Title: Remove Dead SEM Frame Pipeline and Update Documentation
Ticket: SEM-CLEANUP
Status: complete
Topics:
    - sem
    - sessionstream
    - timeline
    - cleanup
    - documentation
    - webchat
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/agentmode_chat_feature.go
      Note: Reference FeatureSet implementation (agentmode)
    - Path: cmd/web-chat/reasoning_chat_feature.go
      Note: Active FeatureSet implementation (reasoning)
    - Path: cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts
      Note: Debug UI that imports timelineMapper.ts
    - Path: cmd/web-chat/web/src/sem/registry.test.ts
      Note: Tests for dead TS SEM registry
    - Path: cmd/web-chat/web/src/sem/registry.ts
      Note: Dead TS SEM registry - only used by Storybook stories
    - Path: cmd/web-chat/web/src/sem/timelineMapper.ts
      Note: Active utility to be relocated to debug-ui/ws/
    - Path: cmd/web-chat/web/src/sem/timelinePropsRegistry.ts
      Note: Active utility to be relocated to webchat/
    - Path: cmd/web-chat/web/src/ws/wsManager.ts
      Note: Production frontend WS manager - no SEM dependency
    - Path: pkg/chatapp/features.go
      Note: Replacement FeatureSet interface
    - Path: pkg/doc/tutorials/04-intern-app-owned-middleware-events-timeline-widgets.md
      Note: Obsolete 1041-line SEM pipeline tutorial
    - Path: pkg/sem/registry/registry.go
      Note: Dead Go SEM registry - zero consumers
ExternalSources: []
Summary: 'The SEM (Structured Event Messaging) frame pipeline has been fully replaced by sessionstream + chatapp.FeatureSet. Dead code, dead documentation, and misplaced frontend modules remain. This ticket tracks the complete cleanup: delete dead Go/TS registry code, delete the obsolete intern tutorial, update four stale doc topics, relocate two misplaced TS utilities, and migrate Storybook stories away from SEM.'
LastUpdated: 2026-05-03T10:26:59.135779732-04:00
WhatFor: ""
WhenToUse: ""
---



# Remove Dead SEM Frame Pipeline and Update Documentation

## Overview

The pinocchio webchat previously used a custom SEM frame pipeline to carry structured events from Go backend to React frontend over WebSocket. This pipeline has been completely replaced by the `sessionstream` package combined with `chatapp.FeatureSet`. The migration is done in production, but dead code and stale documentation remain.

This ticket tracks removing all SEM frame artifacts:

- Dead Go package: `pkg/sem/registry/`
- Dead TypeScript modules: `sem/registry.ts`, `sem/registry.test.ts`
- Obsolete tutorial: `04-intern-app-owned-middleware-events-timeline-widgets.md`
- Stale references in four documentation topic files
- Misplaced utilities that need relocation out of `sem/`
- Storybook story that depends on deleted modules

**Key document:** See `design/01-sem-cleanup-architecture-analysis-and-implementation-guide.md` for the full analysis with architecture background, evidence inventory, phased implementation plan, API references, and file-level guidance.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- sem
- sessionstream
- timeline
- cleanup
- documentation
- webchat

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
