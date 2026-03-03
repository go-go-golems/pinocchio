---
Title: Reusable Pinocchio TUI (third-party package)
Ticket: PI-01-REUSABLE-PINOCCHIO-TUI
Status: active
Topics:
    - tui
    - pinocchio
    - refactor
    - thirdparty
    - bobatea
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/chat/model.go
      Note: Bobatea chat model extension points
    - Path: pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go
      Note: Agent/tool-loop backend candidate for extraction
    - Path: pinocchio/pkg/ui/backend.go
      Note: Backend + event forwarder for basic chat TUI
    - Path: pinocchio/pkg/ui/runtime/builder.go
      Note: Primary reusable chat TUI builder
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T07:58:16.856582438-05:00
WhatFor: ""
WhenToUse: ""
---


# Reusable Pinocchio TUI (third-party package)

## Overview

Goal: enable a third-party Go package to ship its own Bubble Tea TUI “version of Pinocchio” without importing `pinocchio/cmd/...`, by documenting and (if needed) refactoring the existing Pinocchio/Geppetto/Bobatea TUI runtime building blocks.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Primary design doc**: `design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md`
- **Clean-break unified design doc (this request)**: `design-doc/02-unified-pinocchio-tui-simple-chat-agent-tool-loop-as-reusable-primitives.md`
- **Diary**: `reference/01-diary.md`
- **Copy/paste recipes**: `reference/02-third-party-pinocchio-tui-copy-paste-recipes.md`
- **Existing upstream guide**: `pinocchio/pkg/doc/topics/01-chat-builder-guide.md`

## Status

Current status: **active**

## Topics

- tui
- pinocchio
- refactor
- thirdparty
- bobatea

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
