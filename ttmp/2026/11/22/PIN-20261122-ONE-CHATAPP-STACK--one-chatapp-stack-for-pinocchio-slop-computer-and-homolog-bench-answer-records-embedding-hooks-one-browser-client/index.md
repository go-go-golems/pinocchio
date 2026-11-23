---
Title: 'One chatapp stack for pinocchio, slop-computer and homolog-bench: answer records, embedding hooks, one browser client'
Ticket: PIN-20261122-ONE-CHATAPP-STACK
Status: active
Topics:
    - chatapp
    - chat
    - backend
    - frontend
    - refactor
    - design
DocType: index
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://pkg/chatapp/agentserver/agentserver.go
      Note: Preset application; new embedding hooks
    - Path: repo://pkg/chatapp/httpapi/server.go
      Note: HTTP API options and routes
    - Path: repo://pkg/chatapp/projections.go
      Note: Entities the shared client mirrors
    - Path: repo://proto/pinocchio/chatapp/v1/chat.proto
      Note: Events and entities
ExternalSources: []
Summary: ""
LastUpdated: 2026-11-22T20:06:01.262563758-05:00
WhatFor: ""
WhenToUse: ""
---

# One chatapp stack for pinocchio, slop-computer and homolog-bench: answer records, embedding hooks, one browser client

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- chatapp
- chat
- backend
- frontend
- refactor
- design

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
