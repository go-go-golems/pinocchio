---
Title: Implementation plan
Ticket: PI-02
Status: active
Topics:
    - pinocchio
    - tui
    - refactor
    - thirdparty
    - bobatea
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/agents/simple-chat-agent/main.go
      Note: Call site to update to extracted pkg backend/forwarder
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go
      Note: |-
        Source of ToolLoopBackend + agent forwarder to extract
        Former location of ToolLoopBackend (moved to pkg/
    - Path: pinocchio/pkg/ui/backend.go
      Note: Existing StepChatForwardFunc reference when designing agent forwarder semantics
    - Path: pinocchio/pkg/ui/backends/toolloop/backend.go
      Note: Extracted ToolLoopBackend (and current agent forwarder) now lives here
    - Path: pinocchio/pkg/ui/forwarders/agent/forwarder.go
      Note: Extracted agent forwarder (event→timeline mapping)
    - Path: pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md
      Note: Primary extraction plan (Phase 1/2) used as spec
    - Path: pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/design-doc/02-unified-pinocchio-tui-simple-chat-agent-tool-loop-as-reusable-primitives.md
      Note: Larger unification proposal (explicitly out of scope for this ticket)
ExternalSources: []
Summary: Minimal refactor plan to extract Pinocchio’s agent/tool-loop TUI backend + UI forwarder from cmd/ into reusable pkg/ packages.
LastUpdated: 2026-03-03T10:32:40.27306123-05:00
WhatFor: ""
WhenToUse: ""
---




# Implementation plan

## Executive Summary

Extract the “agent/tool-loop chat” backend and its rich “Geppetto event → Bobatea timeline entity” forwarder from `pinocchio/cmd/agents/simple-chat-agent/pkg/...` into `pinocchio/pkg/...` so other commands (and third-party packages) can reuse the same TUI primitives without importing `cmd/`.

This ticket intentionally scopes to the minimal, behavior-preserving extraction described in the existing PI-01 design docs, not the larger “unify all TUI tracks” clean-break proposal.

## Problem Statement

Pinocchio already has a reusable “simple chat” runtime surface (`pinocchio/pkg/ui/runtime` + `pinocchio/pkg/ui`), but the more complete “agent/tool-loop chat” implementation is still trapped under `pinocchio/cmd/...`:

- `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go` (tool-loop session backend + UI forwarder)

Third-party packages should not import `cmd/` paths, and other Pinocchio commands shouldn’t need to copy/paste this logic. The goal is to move the reusable pieces to `pinocchio/pkg/...` and update the existing command to consume them.

## Proposed Solution

### Phase 1: Extract tool-loop backend

- Move `ToolLoopBackend` from:
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`
  - into: `pinocchio/pkg/ui/backends/toolloop/backend.go`
- Keep behavior stable:
  - still uses `geppetto/pkg/inference/session.Session` + `enginebuilder`,
  - still returns `boba_chat.BackendFinishedMsg{}` only when the tool loop finishes,
  - still exposes `CurrentTurn()` for seeding `Turn.Data` (needed for `--server-tools`).

### Phase 2: Extract agent UI forwarder

- Move the event→timeline mapping logic (currently `ToolLoopBackend.MakeUIForwarder`) into a reusable package:
  - `pinocchio/pkg/ui/forwarders/agent` (name may be adjusted during implementation)
- The extracted forwarder must:
  - forward all the agent-specific event types (tool calls/results, logs, agent mode, web search),
  - **not** send `BackendFinishedMsg` on provider “final/error/interrupt” events (tool loop may continue),
  - ack Watermill messages promptly (preserve current ack semantics).

### Phase 3: Update command

- Update `pinocchio/cmd/agents/simple-chat-agent/main.go` to import the new `pkg` backend/forwarder (no cmd-only imports for those pieces).
- Keep higher-level UI composition (host/overlay/tools) as-is for now; this ticket is focused on extracting the common backend/forwarder layer.

## Design Decisions

- **Minimal extraction first**: follow PI-01’s Phase 1/2 plan (extract backend and forwarder) instead of a full TUI unification refactor.
- **Preserve current behavior**: avoid redesigning mapping semantics; refactor by moving code + lightly shaping APIs to fit `pkg/`.
- **Keep existing “simple chat” APIs stable**: do not touch `pinocchio/pkg/ui/runtime.ChatBuilder` or `pinocchio/pkg/ui.StepChatForwardFunc` unless required for compilation.

## Alternatives Considered

- **Clean-break “unified TUI” (`pinocchio/pkg/tui`) refactor** (proposed in PI-01 doc 02): valuable, but too large for this ticket’s initial extraction and would create lots of churn.
- **Leave as-is and let third parties import `cmd/`**: rejected; `cmd/` is conventionally binary-internal and not a stable library surface.
- **Copy/paste extraction into downstream projects**: rejected; guarantees drift and repeated bugfixes.

## Implementation Plan

1. Create `pinocchio/pkg/ui/backends/toolloop` and move `ToolLoopBackend` there.
2. Create `pinocchio/pkg/ui/forwarders/agent` and move the `MakeUIForwarder` implementation there.
3. Update `simple-chat-agent` to use the new packages.
4. Run `gofmt` and `go test ./...` for the `pinocchio` module and fix compile/test failures.
5. Smoke-run `simple-chat-agent` via tmux if feasible in this environment (non-interactive sanity).
6. Update PI-02 diary and changelog with commit hashes and relate changed files.

## Open Questions

- Should the extracted forwarder package be `pinocchio/pkg/ui/forwarders/agent` or `pinocchio/pkg/ui/forwarders` with options? (Prefer the minimal packaging that reads clearly and doesn’t over-abstract.)
- Do we also want to share some mapping code with `pinocchio/pkg/ui.StepChatForwardFunc`, or keep them separate for now?

## References

Primary design references (already in repo):

- `pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md`
- `pinocchio/ttmp/2026/03/03/PI-01-REUSABLE-PINOCCHIO-TUI--reusable-pinocchio-tui-third-party-package/design-doc/02-unified-pinocchio-tui-simple-chat-agent-tool-loop-as-reusable-primitives.md`
