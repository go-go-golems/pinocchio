---
Title: Replace TimelineEntityV1 oneof with open TimelineEntityV2 for decoupled kind extension
Ticket: GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL
Status: completed
Topics:
    - architecture
    - backend
    - frontend
    - timeline
    - webchat
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/proto/sem/timeline/transport.proto
      Note: Existing TimelineEntityV1 closed oneof transport schema targeted for V2 replacement
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/timeline_projector.go
      Note: Backend timeline projection writer to migrate to TimelineEntityV2
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts
      Note: Frontend oneof mapper to replace with kind+props mapping
    - Path: /home/manuel/workspaces/2026-02-14/hypercard-add-webchat/pinocchio/pkg/webchat/conversation.go
      Note: Conversation index persistence path with LastSeenVersion gap called out in ticket tasks
ExternalSources: []
Summary: Pinocchio-side plan for hard cutover from TimelineEntityV1 oneof snapshots to open TimelineEntityV2 kind/props transport, plus conversation-index version persistence fix.
LastUpdated: 2026-02-19T11:24:43-05:00
WhatFor: Track architecture, implementation tasks, and acceptance criteria for decoupling future custom timeline kinds from core pinocchio transport schema edits.
WhenToUse: Use when implementing or reviewing timeline transport refactor and related persistence correctness fixes in pinocchio.
---

# Replace TimelineEntityV1 oneof with open TimelineEntityV2 for decoupled kind extension

## Overview

This ticket defines a pinocchio-core architectural refactor:

1. Replace `TimelineEntityV1` closed oneof snapshots with an open `TimelineEntityV2` model.
2. Keep `timeline.upsert` as canonical wire event while changing payload shape.
3. Remove V1 compatibility paths (hard cutover).
4. Fix conversation index version persistence (`LastSeenVersion`) so debug metadata remains accurate after restart.

## Key Links

- Design plan: `design-doc/01-timelineentityv2-open-model-cutover-plan.md`
- Tasks: `tasks.md`
- Changelog: `changelog.md`

## Status

Current status: **completed**

## Topics

- architecture
- backend
- frontend
- protobuf
- timeline
- webchat

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
