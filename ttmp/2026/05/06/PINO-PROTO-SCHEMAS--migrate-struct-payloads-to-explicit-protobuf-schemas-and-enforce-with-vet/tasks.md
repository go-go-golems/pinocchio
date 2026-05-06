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
- [ ] Add concrete Pinocchio protobuf messages for AgentMode payloads.
- [ ] Migrate `cmd/web-chat/agentmode_chat_feature.go` away from top-level `structpb.Struct`.
- [ ] Add concrete Pinocchio protobuf messages for reasoning payloads.
- [ ] Migrate `pkg/chatapp/plugins/reasoning.go` away from top-level `structpb.Struct`.
- [ ] Inventory CoinVault widget payload shapes.
- [ ] Replace CoinVault `type + google.protobuf.Struct payload` widget schema with typed messages / `oneof`.
- [ ] Build a real `go/analysis` vet analyzer for sessionstream schema registrations.
- [ ] Wire analyzer into Pinocchio lint/CI/pre-commit validation.
- [ ] Remove the temporary allowlist in `pkg/chatapp/schema_policy_test.go` after migrations.
- [ ] Validate live and hydrated AgentMode, reasoning, and CoinVault widget sessions.
- [ ] Upload final updated documentation bundle to reMarkable.
