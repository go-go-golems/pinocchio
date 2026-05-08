---
Title: Systematic chat protocol conformance tests for canonical event lifecycles
Ticket: PINO-PROTOCOL-CONFORMANCE
Status: active
Topics:
    - pinocchio
    - chat
    - frontend
    - sessionstream
    - architecture
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/web/src/ws/timelineEvents.ts
      Note: Frontend sparse patch mapper covered by the conformance plan.
    - Path: pinocchio/pkg/chatapp/plugins/toolcall.go
      Note: Tool lifecycle projection covered by the conformance plan.
    - Path: pinocchio/pkg/chatapp/runtime_sink.go
      Note: Core runtime sink whose lifecycle behavior the conformance tests will protect.
    - Path: pinocchio/pkg/ui/timeline_persist.go
      Note: Timeline persistence lifecycle behavior covered by the conformance plan.
ExternalSources: []
Summary: Ticket for designing and implementing systematic protocol conformance tests for Pinocchio canonical chat event lifecycles.
LastUpdated: 2026-05-08T15:45:00-04:00
WhatFor: Use this ticket to coordinate lifecycle invariant tests that prevent reactive edge-case fixes in Pinocchio chat runtime and web-chat frontend.
WhenToUse: Use before modifying chat runtime event handling, projections, persistence, or frontend sparse patch behavior.
---


# Systematic chat protocol conformance tests for canonical event lifecycles

## Overview

This ticket defines a systematic conformance-testing strategy for Pinocchio's canonical chat protocol. It follows the recent Geppetto/Pinocchio event vocabulary cutover and turns review-discovered edge cases into explicit lifecycle invariants and deterministic test matrices.

The core problem is that the chat protocol crosses several boundaries:

1. Geppetto canonical events.
2. Pinocchio runtime event sink.
3. Pinocchio protobuf backend events.
4. sessionstream UI/timeline projections.
5. timeline persistence.
6. web-chat UI event mapping.
7. Redux sparse patch merging.

The primary design doc explains how to test these stages as one protocol rather than as isolated bug fixes.

## Key Links

- [Design guide](./design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md)
- [Investigation diary](./reference/01-investigation-diary.md)
- [Tasks](./tasks.md)
- [Changelog](./changelog.md)

## Status

Current status: **active**.

The design/research deliverable is complete. Implementation tasks remain open.

## Topics

- pinocchio
- chat
- frontend
- sessionstream
- architecture

## Structure

- `design-doc/` - Architecture and implementation guide.
- `reference/` - Investigation diary and future reference material.
- `scripts/` - Future trace extraction/replay helpers.
- `various/` - Future working notes and browser/debug artifacts.
- `archive/` - Deprecated or historical artifacts.
