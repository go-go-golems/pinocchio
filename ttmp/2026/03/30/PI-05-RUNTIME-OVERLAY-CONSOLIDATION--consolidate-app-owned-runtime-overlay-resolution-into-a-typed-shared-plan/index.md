---
Title: Consolidate app-owned runtime overlay resolution into a typed shared plan
Ticket: PI-05-RUNTIME-OVERLAY-CONSOLIDATION
Status: active
Topics:
    - pinocchio
    - profiles
    - webchat
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/profile_policy.go
      Note: Current stack-aware app-owned runtime merge in Pinocchio web-chat
    - Path: pinocchio/pkg/inference/runtime/profile_runtime.go
      Note: Current app-owned runtime extension types and untyped config surface
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-chat/pkg/profilechat/request_resolver.go
      Note: Leaf-only shared resolver that should migrate to the consolidated plan helper
    - Path: /home/manuel/code/gec/2026-03-16--gec-rag/internal/webchat/resolver.go
      Note: Application-profile plus inference-runtime handwritten merge that should migrate to the consolidated plan helper
ExternalSources: []
Summary: Consolidate the recurring app-owned runtime-overlay pattern into a shared typed plan and merge helper so web-chat apps stop hand-rolling leaf-only or map-heavy runtime resolution logic; implementation is now in place for Pinocchio web-chat and go-go-os-chat, with a typed migration seam in gec-rag.
LastUpdated: 2026-03-30T18:42:00-04:00
WhatFor: Track the framework work needed to turn the current repeated runtime-overlay idiom into a typed, documented, reusable contract across Pinocchio-based web-chat applications.
WhenToUse: Use when designing or implementing shared runtime-plan resolution, app-owned runtime overlays, typed runtime metadata, or migrations away from handwritten request-resolver merge code.
---

# Consolidate app-owned runtime overlay resolution into a typed shared plan

## Overview

This ticket captures the next framework step after the recent `agentmode` rollout work. The current architecture across Pinocchio-based chat apps is consistent in one important way: apps resolve engine settings through Geppetto profiles, then apply app-owned runtime policy such as prompts, middleware, tools, and application-specific overlays before building the final runtime request. That split is correct and already documented at a high level.

What is not yet consolidated is the implementation shape. `pinocchio/cmd/web-chat` now has a stack-aware default overlay plus app-owned runtime merge; `go-go-os-chat` still resolves only the leaf runtime extension; and `gec-rag` has its own second application-profile layer and handwritten merge semantics. All three are solving the same class of problem with different helper code, different merge rules, and too many `map[string]any` or ad hoc metadata payloads.

The goal of this ticket is to define and implement a shared typed runtime-plan contract, a reusable overlay-source model, and a robust merge/fingerprint/provenance story that multiple apps can use without rebuilding the same resolver logic. This should reduce drift, make runtime metadata more typed, and keep open-ended hash maps only where they are actually justified, such as schema-driven middleware config.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

Current findings:

- The pattern is already common across multiple apps.
- The architectural split is documented, but the overlay merge contract is not yet a well-defined shared framework contract.
- The shared helper now lives in `pinocchio/pkg/inference/runtime/runtime_plan.go`.
- `geppetto/pkg/engineprofiles` now exposes typed resolved stack lineage while preserving backward-compatible metadata.
- `pinocchio/cmd/web-chat` and `go-go-os-chat` now consume the shared helper.
- `gec-rag` now has a typed inference-runtime-plan seam and typed fingerprint payload, but still awaits a direct helper migration after a real dependency bump.

## Topics

- pinocchio
- profiles
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
