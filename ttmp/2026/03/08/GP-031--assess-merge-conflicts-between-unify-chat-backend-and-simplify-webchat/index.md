---
Title: Assess merge conflicts between unify-chat-backend and simplify-webchat
Ticket: GP-031
Status: active
Topics:
    - backend
    - pinocchio
    - refactor
    - webchat
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go
      Note: Workspace impact assessment depends on shared chat mounting surface.
    - Path: ../../../../../../../os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go
      Note: Workspace impact assessment depends on shared resolver contract evidence.
    - Path: ../../../../../../../os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go
      Note: web-agent-example uses the stable handler-first server surface.
    - Path: ../../../../../../../os-openai-app-server/wesen-os/cmd/wesen-os-launcher/main_integration_test.go
      Note: Launcher integration tests prove selector/debug contract alignment.
    - Path: ../../../../../../../os-openai-app-server/wesen-os/go.work
      Note: Workspace overlay determines whether local pinocchio branch changes affect builds.
    - Path: ../../../../../../../os-openai-app-server/wesen-os/pkg/assistantbackendmodule/module.go
      Note: Assistant module consumes the shared go-go-os-chat embedding layer.
    - Path: pkg/webchat/server.go
      Note: Pushed branch still keeps compatibility seams simplify-webchat removed.
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-08T16:31:31.815718567-04:00
WhatFor: ""
WhenToUse: ""
---


# Assess merge conflicts between unify-chat-backend and simplify-webchat

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- backend
- pinocchio
- refactor
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
