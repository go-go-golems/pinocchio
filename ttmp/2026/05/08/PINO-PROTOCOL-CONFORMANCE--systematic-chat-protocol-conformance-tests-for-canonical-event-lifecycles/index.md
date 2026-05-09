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
Summary: Ticket for designing and implementing systematic provider-to-browser protocol conformance tests for canonical chat event lifecycles.
LastUpdated: 2026-05-08T20:58:00-04:00
WhatFor: Use this ticket to coordinate lifecycle invariant tests that prevent reactive edge-case fixes in Geppetto provider adapters, Pinocchio chat runtime, and web-chat frontend.
WhenToUse: Use before modifying provider adapters, chat runtime event handling, projections, persistence, or frontend sparse patch behavior.
---


# Systematic chat protocol conformance tests for canonical event lifecycles

## Overview

This ticket defines a systematic conformance-testing strategy for the provider-to-browser canonical chat protocol. It follows the recent Geppetto/Pinocchio event vocabulary cutover and turns review-discovered edge cases into explicit lifecycle invariants and deterministic test matrices.

The core problem is that the chat protocol crosses several boundaries:

1. Provider-native stream events from OpenAI Responses, OpenAI-compatible Chat Completions, Claude, and Gemini.
2. Geppetto provider adapter normalization into canonical events.
3. Pinocchio runtime event sink.
4. Pinocchio protobuf backend events.
5. sessionstream UI/timeline projections.
6. timeline persistence.
7. web-chat UI event mapping.
8. Redux sparse patch merging.

The primary design docs explain how to test these stages as one protocol rather than as isolated bug fixes.

## Key Links

- [Design guide](./design-doc/01-chat-protocol-conformance-analysis-and-implementation-guide.md)
- [OpenAI Chat Completions stream reducer refactor](./design-doc/04-openai-chat-stream-reducer-refactor.md)
- [OpenAI Responses stream refactor](./design-doc/05-openai-responses-stream-refactor.md)
- [Static analysis guide](./design-doc/02-static-analysis-for-protocol-conformance.md) — reference only; not an implementation target for this ticket.
- [Finite-state model guide](./design-doc/03-finite-state-model-for-protocol-conformance.md) — reference only; not an implementation target for this ticket.
- [Investigation diary](./reference/01-investigation-diary.md)
- [Tasks](./tasks.md)
- [Changelog](./changelog.md)

## Status

Current status: **active**.

The design/research deliverable is complete. The OpenAI Chat Completions stream reducer refactor and table-driven tests are implemented in Geppetto. Current focus is adopting the same consume/complete/state pattern in OpenAI Responses before adding broader provider-normalization tests. Static-analysis and model-checking implementation are explicitly out of scope for this ticket.

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
