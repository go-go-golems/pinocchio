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
