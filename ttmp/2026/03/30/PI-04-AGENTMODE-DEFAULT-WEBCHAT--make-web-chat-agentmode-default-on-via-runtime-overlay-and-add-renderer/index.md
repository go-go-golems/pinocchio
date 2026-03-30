---
Title: Make web-chat agentmode default-on via runtime overlay and add renderer
Ticket: PI-04-AGENTMODE-DEFAULT-WEBCHAT
Status: active
Topics:
    - pinocchio
    - webchat
    - agentmode
    - profiles
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/main.go
      Note: Dead enable-agentmode flag and server construction
    - Path: pinocchio/cmd/web-chat/profile_policy.go
      Note: Chat endpoint profile resolution, runtime extraction, and merge point
    - Path: pinocchio/pkg/inference/runtime/profile_runtime.go
      Note: App-owned runtime extension and MiddlewareUse.Enabled field
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: agent.mode to agent_mode entity mapping
    - Path: pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts
      Note: Missing dedicated agent_mode renderer registration
ExternalSources: []
Summary: Follow-up ticket to remove the dead web-chat enable-agentmode flag, inject agentmode as a default web-chat runtime middleware with per-profile enabled:false opt-out, enable it across effective profile resolution, and add a dedicated web-chat renderer for agent_mode entities. Implementation completed in commits 9d30b0d and 0da28f9.
LastUpdated: 2026-03-30T14:31:00-04:00
WhatFor: Plan and implement default-on web-chat agentmode activation through runtime-policy overlay rather than CLI flags, while preserving per-profile opt-out and improving the frontend rendering path.
WhenToUse: Use when implementing or reviewing how web-chat should enable agentmode by default across profiles and how agent_mode entities should appear in the chat timeline.
---

# Make web-chat agentmode default-on via runtime overlay and add renderer

## Overview

This ticket covers the next cleanup and productization step after the initial agentmode structuredsink work. Web-chat currently has working agentmode middleware, working structured extraction, and working SEM entity creation, but it still has a dead `--enable-agentmode` flag, it only enables agentmode when each profile explicitly opts in, and it lacks a dedicated `agent_mode` renderer in the chat UI. The intent of this ticket is to make agentmode default-on for web-chat through a proper runtime-policy overlay, preserve per-profile `enabled: false` opt-out semantics, and add a renderer so the resulting entities have an intentional UI.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

Current implementation findings:

- `--enable-agentmode` was removed in commit `9d30b0d`.
- The `/chat` endpoint now merges a default app-owned web-chat runtime with the resolved profile runtime stack.
- Runtime merging is stack-aware through `profile.stack.lineage`, so app-owned runtime policy follows registry inheritance rather than only reading the leaf profile extension.
- `agent.mode` SEM frames already become `agent_mode` entities in frontend state, and those entities now render through a dedicated builtin card added in commit `0da28f9`.

## Topics

- pinocchio
- webchat
- agentmode
- profiles

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
