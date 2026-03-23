---
Title: Restore web-agent-example and dependent apps against current runner-first webchat architecture
Ticket: APP-12-WEBCHAT-RUNNER-FIRST-REBASE
Status: active
Topics:
    - backend
    - webchat
    - runner
    - apps
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go.work
      Note: Root workspace overlay restored sibling-module visibility and linked app coverage.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/go.work
      Note: Nested workspace needed a Go version bump to remain compatible with the updated local pinocchio checkout.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/web-agent-example/cmd/web-agent-example/main.go
      Note: Minimal downstream embedder already uses the current app-owned handler surface.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/chatservice/component.go
      Note: Reusable embedding layer mounts the same current webchat routes.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-os-chat/pkg/profilechat/request_resolver.go
      Note: Strict resolver already matches the modern profile/registry contract.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/server.go
      Note: Runner-first server surface proves that downstream apps should own /chat and /ws mounting.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/router.go
      Note: Deps-first router and ChatService/ConversationService split are the architectural baseline.
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/pinocchio/pkg/webchat/chat_service.go
      Note: ChatService remains a real orchestration boundary and must not be collapsed away.
ExternalSources: []
Summary: Restores local downstream compilation after the pinocchio rebase by repairing the root and nested Go workspaces instead of changing application source, and documents the runner-first webchat architecture for future maintainers.
LastUpdated: 2026-03-08T19:22:12.710186633-04:00
WhatFor: ""
WhenToUse: ""
---

# Restore web-agent-example and dependent apps against current runner-first webchat architecture

## Overview

This ticket documents and fixes the local multi-module breakage introduced after updating the `pinocchio` checkout to the current runner-first webchat architecture while reverting simplify-webchat-only changes. The key finding is that the downstream callers were already written against the correct architecture; the real regressions were in Go workspace composition.

The fix restores a root `go.work`, updates `wesen-os/go.work` to `go 1.26.1`, mirrors the local `go-go-os-chat v0.0.0` replacement semantics already used by `wesen-os`, and validates the downstream compile surface across `web-agent-example`, `go-go-os-chat`, `wesen-os`, and linked app modules.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Main Guide**: [design-doc/01-runner-first-webchat-rebase-analysis-and-implementation-guide.md](./design-doc/01-runner-first-webchat-rebase-analysis-and-implementation-guide.md)
- **Diary**: [reference/01-investigation-diary.md](./reference/01-investigation-diary.md)
- **Validation Script**: [scripts/validate-workspace-builds.sh](./scripts/validate-workspace-builds.sh)

## Status

Current status: **active**

## Topics

- backend
- webchat
- runner
- apps

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
