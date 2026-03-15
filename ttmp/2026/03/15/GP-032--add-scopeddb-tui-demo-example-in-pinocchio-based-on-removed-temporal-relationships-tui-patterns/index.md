---
Title: add scopeddb tui demo example in pinocchio based on removed temporal-relationships tui patterns
Ticket: GP-032
Status: active
Topics:
    - pinocchio
    - tui
    - sqlite
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/cmd/switch-profiles-tui/main.go
      Note: Main current TUI reference used for the recommendation.
    - Path: /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/ui/backends/toolloop/backend.go
      Note: Reusable backend that the future demo should build on.
    - Path: /home/manuel/workspaces/2026-03-02/deliver-mento-1/geppetto/pkg/inference/tools/scopeddb/schema.go
      Note: Scopeddb dataset builder API that the demo should teach.
ExternalSources: []
Summary: Research ticket recommending a new dedicated Pinocchio Bubble Tea example for demonstrating Geppetto scopeddb tools with fake data.
LastUpdated: 2026-03-15T18:00:00-04:00
WhatFor: Capture the analysis, implementation plan, and supporting diary for adding a scopeddb TUI demo to Pinocchio.
WhenToUse: Use when implementing or reviewing a future `scopeddb-tui-demo` example in Pinocchio.
---

# add scopeddb tui demo example in pinocchio based on removed temporal-relationships tui patterns

## Overview

This ticket recommends adding a new dedicated example binary in Pinocchio for demonstrating `geppetto/pkg/inference/tools/scopeddb` inside a Bubble Tea interface. The guide in this ticket explains why the removed temporal-relationships TUIs are still useful reference material, why the modern implementation should instead build on current Pinocchio TUI primitives, and how an intern should implement the example.

## Key Links

- Primary design doc: `design-doc/01-scopeddb-tui-demo-analysis-design-and-intern-implementation-guide.md`
- Diary: `reference/01-investigation-diary.md`
- Tasks: `tasks.md`
- Changelog: `changelog.md`

## Status

Current status: **active**

Analysis and ticket authoring are complete. Implementation is intentionally left as future work.

## Topics

- pinocchio
- tui
- sqlite

## Recommendation Summary

Create:

```text
pinocchio/cmd/examples/scopeddb-tui-demo/main.go
```

Do not:

- add the first demo to `cmd/web-chat`,
- revive a production `tui` command family,
- or copy the removed temporal-relationships TUI code directly.

## Tasks

See [tasks.md](./tasks.md) for completed research work and the proposed implementation backlog.

## Changelog

See [changelog.md](./changelog.md) for the chronological summary of ticket updates.

## Structure

- `design-doc/` contains the intern-facing architecture and implementation guide.
- `reference/` contains the investigation diary.
- `tasks.md` tracks completed research steps and future implementation work.
- `changelog.md` records ticket-level changes.
