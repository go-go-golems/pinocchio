---
Title: Tasks
Ticket: PINO-PROTO-SCHEMAS
Status: active
Topics:
  - protobuf
  - sessionstream
  - linting
DocType: tasks
Intent: implementation checklist
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Task checklist for migrating Struct payloads to typed protobuf schemas and enforcing the policy."
LastUpdated: 2026-05-06T15:45:00-04:00
WhatFor: "Track implementation phases for PINO-PROTO-SCHEMAS."
WhenToUse: "Use when planning or resuming work on the ticket."
---

# Tasks

- [x] Create Pinocchio ticket workspace.
- [x] Write intern-oriented design and implementation guide.
- [x] Create implementation diary.
- [x] Add concrete Pinocchio protobuf messages for AgentMode payloads.
- [x] Migrate `cmd/web-chat/agentmode_chat_feature.go` away from top-level `structpb.Struct`.
- [x] Add concrete Pinocchio protobuf messages for reasoning payloads.
- [x] Migrate `pkg/chatapp/plugins/reasoning.go` away from top-level `structpb.Struct`.
- [x] Inventory CoinVault widget payload shapes.
- [x] Replace CoinVault `type + google.protobuf.Struct payload` widget schema with separate typed protobuf messages and separate event/UI/timeline names per widget; no backwards compatibility shims.
- [x] Build a real `go/analysis` vet analyzer for sessionstream schema registrations.
- [x] Wire analyzer into Pinocchio lint/CI/pre-commit validation.
- [x] Remove the temporary allowlist in `pkg/chatapp/schema_policy_test.go` after migrations.
- [x] Validate live and hydrated AgentMode, reasoning, and CoinVault widget sessions.
- [x] Upload final updated documentation bundle to reMarkable.
