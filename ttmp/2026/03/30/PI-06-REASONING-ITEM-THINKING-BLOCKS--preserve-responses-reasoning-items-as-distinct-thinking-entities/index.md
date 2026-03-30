---
Title: Preserve Responses reasoning items as distinct thinking entities
Ticket: PI-06-REASONING-ITEM-THINKING-BLOCKS
Status: active
Topics:
    - pinocchio
    - geppetto
    - webchat
    - open-responses
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/steps/ai/openai_responses/engine.go
      Note: Responses streaming engine currently tracks reasoning item ids internally but collapses live thinking text into one cumulative stream.
    - Path: geppetto/pkg/steps/ai/openai/engine_openai.go
      Note: Older OpenAI streaming engine also emits a single flattened thinking stream without item boundaries.
    - Path: pinocchio/pkg/webchat/sem_translator.go
      Note: SEM translation currently hard-codes one thinking entity id per inference using baseID plus :thinking.
    - Path: pinocchio/pkg/webchat/timeline_projector.go
      Note: Timeline projector is already capable of handling multiple message ids, but current tests and assumptions are single-thinking-stream oriented.
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: Frontend SEM registry already supports multiple llm thinking entities as long as ids are distinct.
ExternalSources: []
Summary: Design the long-term fix for malformed thinking markdown and flattened reasoning streams by preserving OpenAI Responses reasoning item boundaries end to end, so each provider reasoning item becomes its own thinking entity in SEM and the web-chat timeline.
LastUpdated: 2026-03-30T16:54:10-04:00
WhatFor: Track the multi-layer refactor needed to stop flattening all Responses reasoning output into a single thinking card and instead model provider reasoning items as first-class streamed entities.
WhenToUse: Use when changing reasoning-stream semantics, SEM identity, timeline entity modeling, or OpenAI Responses thinking persistence in Geppetto and Pinocchio.
---

# Preserve Responses reasoning items as distinct thinking entities

## Overview

This ticket covers the long-term structural fix for a bug that currently surfaces as malformed markdown in the web-chat "Thinking" card. The immediate symptom is that section-like reasoning fragments such as `**Crafting a concise response**` can become glued to the end of the previous paragraph. The short-term patch can repair markdown boundaries, but the deeper issue is architectural: the OpenAI Responses provider emits multiple reasoning items, while the current Geppetto plus Pinocchio pipeline flattens them into a single cumulative thinking entity per inference.

The provider is already giving the system meaningful structure. In the Responses streaming protocol, `response.output_item.added` tells us when a new reasoning item begins, and `response.output_item.done` tells us when that item ends. Geppetto keeps some per-item state for persistence, but the live stream that reaches the web UI is still collapsed into one `:thinking` id. That destroys item boundaries and forces the UI to treat several independent reasoning phases as one endless message.

The goal of this ticket is to preserve provider reasoning-item identity through the whole streaming stack. That means the engine, event model, SEM translator, and timeline projection code should stop pretending there is only one thinking stream per inference. Once that structural change exists, the UI will naturally render multiple thinking segments instead of one flattened card, and markdown boundaries will stop depending on heuristics.

## Key Links

- **Main design guide**: [design-doc/01-intern-guide-to-preserving-responses-reasoning-items-as-distinct-thinking-entities.md](./design-doc/01-intern-guide-to-preserving-responses-reasoning-items-as-distinct-thinking-entities.md)
- **Documentation inventory and source notes**: [reference/01-documentation-and-source-inventory.md](./reference/01-documentation-and-source-inventory.md)
- **Task plan**: [tasks.md](./tasks.md)
- **Changelog**: [changelog.md](./changelog.md)

## Status

Current status: **active**

Current findings:

- The malformed markdown is already present in live `llm.thinking.delta` payloads before the frontend renders them.
- The OpenAI Responses engine tracks `currentReasoningItemID` internally, so the provider is already exposing the boundaries we need.
- The SEM translator currently discards reasoning item identity and emits one fixed thinking id per inference.
- The web-chat projector and frontend registry are already mostly compatible with multiple thinking entities as long as they receive distinct ids.
- The short-term fix should be an engine-level markdown-boundary normalization. The long-term fix should be per-reasoning-item entity modeling.

## Structure

- design-doc/ - Detailed architecture and implementation guide for the long-term fix
- reference/ - Source inventory and documentation notes
- playbooks/ - Reserved for validation runbooks if needed later
- scripts/ - Reserved for one-off analysis scripts if needed later
- various/ - Working notes
- archive/ - Deprecated ticket artifacts
