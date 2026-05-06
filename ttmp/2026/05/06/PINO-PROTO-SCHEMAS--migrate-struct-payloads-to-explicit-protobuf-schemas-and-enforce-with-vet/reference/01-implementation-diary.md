---
Title: Implementation Diary
Ticket: PINO-PROTO-SCHEMAS
Status: active
Topics:
  - protobuf
  - sessionstream
  - linting
DocType: reference
Intent: chronological work log
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological notes for the explicit protobuf payload migration and vet analyzer ticket."
LastUpdated: 2026-05-06T15:45:00-04:00
WhatFor: "Use to resume work and understand what changed, what failed, and what remains."
WhenToUse: "Before continuing implementation on PINO-PROTO-SCHEMAS."
---

# Implementation Diary

## 2026-05-06 — Ticket creation and design guide

Created ticket `PINO-PROTO-SCHEMAS` in the Pinocchio `ttmp` tree after the AgentMode hydration investigation showed a concrete failure mode from generic `google.protobuf.Struct` payloads.

What was observed:

- Live UI-event payloads already used a frontend Struct-unwrapping helper.
- Hydrated timeline snapshot payloads did not use that helper.
- The `AgentMode` timeline entity was stored as a top-level `google.protobuf.Struct` with the actual fields under JSON `value`.
- The frontend looked for `payload.data`, found none, and rendered `No analysis`.

Decision captured in the design document:

- All sessionstream commands, backend events, UI events, and timeline entities need concrete feature-owned protobuf messages.
- This includes app-specific payloads.
- `google.protobuf.Struct` is acceptable only inside a typed message for intentionally open-ended sub-data.
- The temporary source-scanning `_test.go` guardrail should be replaced by a real `go/analysis` vet tool.

Artifacts created:

- `design/01-explicit-protobuf-payloads-and-vet-enforcement.md`

Important caveat:

- `docmgr` defaults to the workspace-level `.ttmp.yaml`, which points at the gec-rag `ttmp` root. For this ticket, commands must use the absolute Pinocchio root:
  `/home/manuel/workspaces/2026-05-02/use-sessionstream-coinvault/pinocchio/ttmp`

## 2026-05-06 — CoinVault widget schema decision update

Updated the design after review: CoinVault widgets should not use a single wrapper `oneof` as the main durable widget contract. Instead, each widget gets its own protobuf messages and its own sessionstream backend event, UI event, and timeline entity kind.

Rationale:

- `sessionstream` already has event names and timeline entity kinds, so those names should be the primary dispatch mechanism.
- The frontend renderer registry naturally dispatches by entity kind.
- Separate widget messages avoid a mega-wrapper that grows every time a widget is added.
- Each widget can evolve independently.

Also clarified that this migration does not need backwards compatibility shims for old local `Struct` payloads. Reset local smoke DBs or perform an explicit one-off repair outside the app runtime if old sessions must be inspected.

## 2026-05-06 — Implemented typed payload migration and schema vet tool

Implemented the no-backwards-compatibility migration for the current known payload debt.

Pinocchio changes:

- Added concrete protobuf messages in `proto/pinocchio/chatapp/v1/chat.proto`:
  - `ReasoningUpdate`
  - `AgentModePreviewUpdate`
  - `AgentModeCommittedUpdate`
  - `AgentModePreviewCleared`
  - `AgentModeEntity`
- Regenerated Go protobuf code.
- Migrated `pkg/chatapp/plugins/reasoning.go` to publish/project `ReasoningUpdate` instead of `structpb.Struct`.
- Migrated `cmd/web-chat/agentmode_chat_feature.go` to publish/project typed AgentMode messages and a flattened `AgentModeEntity`.
- Removed the temporary policy-test allowlist.
- Added real analyzer package `pkg/analysis/sessionstreamschema` and vettool command `cmd/tools/pinocchio-lint`.
- Added `make schema-vet` for `go vet -vettool=/tmp/pinocchio-lint ./cmd/... ./pkg/...`.

CoinVault changes:

- Replaced `CoinVaultWidgetUpsert` / `CoinVaultWidgetEntity` with per-widget protobuf messages in `proto/coinvault/widgets/v1/widgets.proto`.
- Regenerated Go and TypeScript protobuf code.
- Migrated `internal/webchat/coinvault_projection_feature.go` to register separate event/UI/timeline names per widget:
  - inventory cards
  - inventory table
  - stats row
  - stock alert
  - projection error
- Updated frontend protobuf parsing to dispatch by widget-specific event/entity kind.

Validation run:

```text
cd pinocchio && go test ./pkg/chatapp ./pkg/chatapp/plugins ./cmd/web-chat ./pkg/analysis/sessionstreamschema -count=1
cd pinocchio && make schema-vet
cd pinocchio/cmd/web-chat/web && npx vitest run src/ws/wsManager.test.ts && npm run typecheck
cd 2026-03-16--gec-rag && go test ./internal/webchat ./internal/projectionlookup ./internal/projectionblocks -count=1
cd 2026-03-16--gec-rag/web && npm run typecheck && npm run test:unit -- src/ws/parsing.test.ts
```

Smoke test:

- Started a fresh pinocchio web-chat server on `:8092` with empty `/tmp/pinocchio-proto-smoke` timeline/turn DBs.
- Created session `131e9d08-78f4-44f6-9aca-f9512914e6a6`.
- Submitted a prompt through the HTTP API.
- OpenAI Responses returned HTTP 200 SSE.
- Session reached `finished` with hydrated `ChatMessage` user, thinking, assistant text, and final thinking entities.

No backwards compatibility shims were added. Old local smoke databases with Struct payloads should be reset if they need to be used with the new schema.
