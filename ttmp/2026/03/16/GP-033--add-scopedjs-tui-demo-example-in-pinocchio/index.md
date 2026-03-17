---
Title: Add scopedjs TUI demo example in pinocchio
Ticket: GP-033
Status: active
Topics:
    - pinocchio
    - ui
    - js-bindings
    - tools
    - architecture
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/main.go
      Note: Existing TUI example wiring that the new scopedjs demo should mirror closely
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/cmd/examples/scopeddb-tui-demo/renderers.go
      Note: Existing custom tool-call and tool-result timeline renderer pattern
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/pkg/ui/backends/toolloop/backend.go
      Note: Pinocchio backend that drives the tool loop and Bubble Tea model
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/pinocchio/pkg/ui/forwarders/agent/forwarder.go
      Note: Event to timeline forwarder used by the demo UI
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/tool.go
      Note: Reusable registration surface the new demo exists to teach
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/pkg/inference/tools/scopedjs/eval.go
      Note: Eval input and output contract that the renderer should visualize
    - Path: /home/manuel/workspaces/2026-03-15/add-scoped-js/geppetto/cmd/examples/scopedjs-dbserver/main.go
      Note: Small runnable scopedjs example that should be lifted into a full Pinocchio TUI demo
ExternalSources: []
Summary: Planning ticket for a Pinocchio Bubble Tea demo that teaches the new Geppetto scopedjs package through a fake but concrete project-ops runtime with files, db-style data, Obsidian-style note helpers, and route registration.
LastUpdated: 2026-03-16T15:18:00-04:00
WhatFor: Capture the analysis, architecture, implementation plan, and onboarding guide for a new Pinocchio example that demonstrates composed scoped JavaScript tools in a TUI.
WhenToUse: Use when implementing or reviewing a future `scopedjs-tui-demo` example in Pinocchio or when onboarding to the Pinocchio plus Geppetto demo architecture.
---

# Add scopedjs TUI demo example in pinocchio

## Overview

This ticket scopes a dedicated Pinocchio example binary that demonstrates `geppetto/pkg/inference/tools/scopedjs` in the same role that `cmd/examples/scopeddb-tui-demo` serves for `scopeddb`: a small but realistic Bubble Tea application that teaches the package by showing a full prompt-to-tool-to-rendered-result workflow.

The recommended demo is a scoped "project workspace ops" assistant. The LLM gets one tool such as `eval_project_ops`, backed by a prepared JavaScript runtime with:

- `fs` for reading and writing files inside a temp workspace,
- a scoped `db` global for fake tasks and notes,
- a fake `obsidian` module for note creation metadata,
- a fake `webserver` module that records routes instead of opening sockets,
- and bootstrap helpers that make the runtime feel like one coherent environment.

The goal is not to ship production modules. The goal is to make the new reusable `scopedjs` package easy to understand, easy to review, and easy to demo live in the terminal.

## Key Links

- **Primary implementation plan**: `analysis/01-scopedjs-tui-demo-recommendation-and-implementation-plan.md`
- **Primary design guide**: `design/01-scopedjs-tui-demo-analysis-design-and-intern-implementation-guide.md`
- **Investigation diary**: `reference/01-investigation-diary.md`
- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

The ticket currently contains planning and design only. No Pinocchio code has been changed yet.

## Topics

- pinocchio
- ui
- js-bindings
- tools
- architecture

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- analysis/ - Recommendation summary and detailed phase-by-phase implementation plan
- design/ - Architecture and design documents
- reference/ - Investigation diary and supporting reference notes
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
