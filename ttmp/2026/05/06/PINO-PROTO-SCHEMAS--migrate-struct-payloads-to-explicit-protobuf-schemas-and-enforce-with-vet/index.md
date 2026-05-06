---
Title: Migrate Struct Payloads to Explicit Protobuf Schemas and Enforce with Vet
Ticket: PINO-PROTO-SCHEMAS
Status: active
Topics:
    - protobuf
    - sessionstream
    - webchat
    - linting
    - coinvault
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../2026-03-16--gec-rag/internal/webchat/coinvault_projection_feature.go
      Note: CoinVault sessionstream plugin that registers and projects widget payloads
    - Path: ../../../../../../2026-03-16--gec-rag/proto/coinvault/widgets/v1/widgets.proto
      Note: CoinVault widget schema currently using type plus Struct payload
    - Path: ../../../../../../sessionstream/pkg/sessionstream/projection.go
      Note: UIEvent and TimelineEntity payload contracts
    - Path: ../../../../../../sessionstream/pkg/sessionstream/schema.go
      Note: SchemaRegistry API that registers command/event/UI/timeline protobuf payload schemas
    - Path: cmd/web-chat/agentmode_chat_feature.go
      Note: Current AgentMode plugin using Struct payloads to migrate
    - Path: pkg/chatapp/plugins/reasoning.go
      Note: Current Reasoning plugin using Struct payloads to migrate
    - Path: pkg/chatapp/schema_policy_test.go
      Note: Temporary test-based guardrail to replace with analyzer
    - Path: proto/pinocchio/chatapp/v1/chat.proto
      Note: Pinocchio chatapp protobuf schema source to extend with AgentMode and Reasoning payload types
ExternalSources: []
Summary: ""
LastUpdated: 2026-05-06T15:39:50.362089442-04:00
WhatFor: ""
WhenToUse: ""
---


# Migrate Struct Payloads to Explicit Protobuf Schemas and Enforce with Vet

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- protobuf
- sessionstream
- webchat
- linting
- coinvault

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
