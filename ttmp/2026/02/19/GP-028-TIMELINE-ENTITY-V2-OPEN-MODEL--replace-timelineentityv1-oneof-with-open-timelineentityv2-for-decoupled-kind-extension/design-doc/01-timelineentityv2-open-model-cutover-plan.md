---
Title: TimelineEntityV2 Open Model Cutover Plan
Ticket: GP-028-TIMELINE-ENTITY-V2-OPEN-MODEL
Status: active
Topics:
    - architecture
    - backend
    - frontend
    - timeline
    - webchat
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: Frontend timeline.upsert decode and dispatch
    - Path: pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts
      Note: |-
        Timeline payload mapping to UI entity props
        Frontend oneof mapper replacement with kind+props mapping
    - Path: pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts
      Note: Versioned upsert semantics in Redux timeline state
    - Path: pinocchio/cmd/web-chat/web/src/ws/wsManager.ts
      Note: Hydration + buffered replay behavior
    - Path: pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go
      Note: SQLite upsert path needing timeline_conversations LastSeenVersion update
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: ConversationRecord builder currently missing LastSeenVersion population
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Service-level timeline.upsert emission path
    - Path: pinocchio/pkg/webchat/timeline_projector.go
      Note: |-
        Backend SEM to timeline projection writer path
        Projection writer path to migrate to TimelineEntityV2
    - Path: pinocchio/pkg/webchat/timeline_upsert.go
      Note: Router timeline.upsert emission path
    - Path: pinocchio/proto/sem/timeline/transport.proto
      Note: |-
        Current TimelineEntityV1 closed oneof contract to replace
        Current closed oneof timeline schema targeted for V2 open model
ExternalSources: []
Summary: Hard-cutover design for replacing TimelineEntityV1 closed oneof snapshots with open TimelineEntityV2 kind/props model so new domain kinds can ship without touching pinocchio transport proto again.
LastUpdated: 2026-02-19T10:58:00-05:00
WhatFor: Define the pinocchio-side architectural change needed to decouple future custom timeline kinds from core schema churn.
WhenToUse: Use when implementing timeline transport changes, frontend mapping updates, and projection pipeline cutover in pinocchio webchat.
---


# TimelineEntityV2 Open Model Cutover Plan

## Executive Summary

Pinocchio webchat currently uses `TimelineEntityV1` with a closed `oneof snapshot` in `proto/sem/timeline/transport.proto`. That shape forces pinocchio schema edits whenever a new custom timeline payload family is introduced. This is the coupling we want to remove.

This ticket proposes a pinocchio-core refactor: replace the V1 closed-snapshot model with a new open `TimelineEntityV2` model keyed by `kind` and carrying flexible `props` data. The cutover is explicit and non-backward-compatible. After this one-time migration, future domain kinds (for example `hypercard_widget` and `hypercard_card`) can be introduced by backend projector registration and frontend renderer registration only, without modifying pinocchio transport proto again.

Target outcome:

1. `timeline.upsert` remains the canonical stream.
2. New kinds do not require pinocchio proto `oneof` edits.
3. Frontend mapper complexity drops (no growing oneof-case switch).
4. Hydration/replay determinism is preserved.

## Problem Statement

The current timeline contract has a structural scalability problem:

1. `TimelineEntityV1` encodes payload variants in a closed `oneof snapshot`.
2. Each new domain family requires adding a new message type and new `oneof` case in core pinocchio schema.
3. Codegen churn hits backend and frontend every time.
4. Ownership boundaries blur because app/domain teams must modify pinocchio core contracts.

This creates architectural friction:

1. Pinocchio core changes are required for app-specific semantics.
2. Deployment risk increases for unrelated teams because core transport is edited frequently.
3. Frontend mapping accumulates a case-by-case translation layer.
4. Extensibility claims are weakened by hard schema coupling.

## Proposed Solution

Use a new open timeline entity transport model and cut over fully.

### 1) Introduce TimelineEntityV2 contract

Define V2 messages in `proto/sem/timeline/transport.proto` (or sibling `transport_v2.proto`) and make them canonical:

```proto
message TimelineEntityV2 {
  string id = 1;
  string kind = 2;
  int64 created_at_ms = 3;
  int64 updated_at_ms = 4;

  // Canonical open payload used by renderer mapping.
  google.protobuf.Struct props = 10;

  // Optional typed payload for advanced consumers.
  google.protobuf.Any typed = 11;

  // Optional diagnostic/trace metadata.
  map<string, string> meta = 12;
}

message TimelineUpsertV2 {
  string conv_id = 1;
  uint64 version = 2;
  TimelineEntityV2 entity = 3;
}

message TimelineSnapshotV2 {
  string conv_id = 1;
  uint64 version = 2;
  int64 server_time_ms = 3;
  repeated TimelineEntityV2 entities = 10;
}
```

Design intent:

1. `kind` is the renderer dispatch key.
2. `props` is the stable generic payload contract.
3. `typed` is optional and does not block unknown-kind rendering.
4. `meta` carries lightweight non-rendering info.

### 2) Keep event envelope shape stable

Keep SEM envelope and `event.type = "timeline.upsert"` stable. Replace payload schema from `TimelineUpsertV1` to `TimelineUpsertV2`.

This minimizes websocket routing changes and limits blast radius.

### 3) Backend projector writes V2 directly

Update `TimelineProjector` and custom timeline handlers to create `TimelineEntityV2` with:

1. deterministic `id`
2. explicit `kind`
3. structured `props`
4. optional `typed` only when needed

No V1 oneof snapshot construction should remain.

### 4) Frontend maps V2 directly to Redux entities

Update frontend registry/mapper to decode `TimelineUpsertV2` and map:

1. `entity.kind` -> `TimelineEntity.kind`
2. `entity.props` -> `TimelineEntity.props`
3. timestamps/version as before

Oneof-case mapping logic is removed.

### 5) Hard cutover, no compatibility layer

Do not keep dual V1/V2 decode paths or alias shims.

1. Backend emits only V2.
2. Frontend decodes only V2.
3. tests updated to V2 expectations.

This reduces permanent complexity and enforces clean contract ownership.

## End-to-End Flow After Cutover

```text
runtime event
  -> sem translator (typed event.data)
  -> timeline projector handler
  -> TimelineEntityV2 {id, kind, props, typed?, meta?}
  -> timeline store upsert(version)
  -> timeline.upsert(event.data = TimelineUpsertV2)
  -> ws manager hydrate + replay
  -> frontend decode TimelineUpsertV2
  -> timeline slice upsert
  -> ChatTimeline renderer dispatch by kind
```

## Design Decisions

1. Open model over closed oneof:
- Decision: use `kind + props` as canonical payload transport.
- Rationale: removes recurring core schema edits for new kinds.

2. Keep `timeline.upsert` event name:
- Decision: preserve SEM event type.
- Rationale: avoids unnecessary route/protocol churn.

3. Hard cutover (no backwards compatibility):
- Decision: remove V1 path instead of dual support.
- Rationale: avoid long-lived migration complexity and divergent semantics.

4. Optional typed payload (`Any`):
- Decision: allow advanced typed consumers without enforcing oneof growth.
- Rationale: preserve future strong typing use-cases while keeping transport open.

5. Frontend dispatch by `kind` only:
- Decision: renderer selection must be by entity kind.
- Rationale: predictable plugin registration and simpler mapper code.

## Extension Rule (No Future Proto Edits)

After this cutover, adding a new timeline kind must not require editing `pinocchio/proto/sem/timeline/transport.proto`.

Required steps for a new domain kind:

1. Backend projector (or custom timeline handler) emits:
- `TimelineEntityV2.kind = "<new_kind>"`
- `TimelineEntityV2.props = { ...domain payload... }`
2. Frontend optional props normalization:
- register normalizer in `cmd/web-chat/web/src/sem/timelinePropsRegistry.ts`
3. Frontend renderer:
- register renderer via `registerTimelineRenderer("<new_kind>", Renderer)` from `cmd/web-chat/web/src/webchat/rendererRegistry.ts`

Explicit non-requirement:

1. No new transport protobuf messages.
2. No `oneof` expansion.
3. No pinocchio-core schema churn for app-owned kinds.

## Alternatives Considered

1. Keep V1 and add more oneof cases:
- Rejected: preserves coupling and repeats same problem.

2. Use `tool_result.customKind` for all future custom kinds:
- Rejected: hides domain semantics in generic envelope and creates renderer hacks.

3. Keep V1 and add a single `custom` oneof case:
- Rejected for now: better than many oneof cases, but still keeps V1-specific branching and mixed mental model.

4. JSON-only ad hoc envelope outside protobuf:
- Rejected: loses schema discipline and degrades type tooling.

## Implementation Plan

### Phase 1: Proto contract cutover in pinocchio

1. Define `TimelineEntityV2`, `TimelineUpsertV2`, `TimelineSnapshotV2`.
2. Add required imports for `Struct` and `Any`.
3. Remove V1 messages from active use and update generated code.

Deliverable:
- protobuf artifacts generated for Go and TS with V2 messages.

### Phase 2: Backend projector/store/wire update

1. Update projector output construction to V2 payload shape.
2. Update timeline upsert emission in router/service to serialize V2.
3. Update hydration service API response to return `TimelineSnapshotV2`.
4. Fix conversation index persistence so `LastSeenVersion` tracks real timeline progression:
- populate `LastSeenVersion` in `buildConversationRecord` (`pkg/webchat/conversation.go`)
- update SQLite upsert path to advance `timeline_conversations.last_seen_version` during timeline writes.

Deliverable:
- backend emits and serves only V2 timeline payloads.

### Phase 3: Frontend decode and mapper update

1. Update `cmd/web-chat/web/src/sem/registry.ts` to decode `TimelineUpsertV2`.
2. Replace oneof mapper in `timelineMapper.ts` with direct `kind + props` mapping.
3. Validate `timelineSlice` version semantics unchanged.

Deliverable:
- frontend renders timeline from V2 without oneof branches.

### Phase 4: Test migration

1. Update backend tests for projector and wire payload assertions.
2. Update frontend tests for mapper and registry decode behavior.
3. Add hydration/replay integration tests proving determinism under V2.

Deliverable:
- full green test suite for V2-only pipeline.

### Phase 5: Cleanup

1. Remove dead V1 mapping helpers and comments.
2. Remove stale docs mentioning oneof snapshot extension flow.
3. Publish extension guidance: new kinds only require projector + renderer registration.

Deliverable:
- no V1 timeline contract references in active code paths.

## Detailed Impact Map (Pinocchio)

Backend protocol + projection:

1. `pinocchio/proto/sem/timeline/transport.proto`
2. `pinocchio/pkg/webchat/timeline_projector.go`
3. `pinocchio/pkg/webchat/timeline_upsert.go`
4. `pinocchio/pkg/webchat/conversation_service.go`
5. `pinocchio/pkg/webchat/conversation.go`
6. `pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go`

Frontend decode + render pipeline:

1. `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
2. `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts`
3. `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
4. `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`
5. `pinocchio/cmd/web-chat/web/src/webchat/ChatWidget.tsx`
6. `pinocchio/cmd/web-chat/web/src/webchat/components/Timeline.tsx`
7. `pinocchio/cmd/web-chat/web/src/sem/timelinePropsRegistry.ts`
8. `pinocchio/cmd/web-chat/web/src/webchat/rendererRegistry.ts`

Debug/UI paths:

1. `pinocchio/cmd/web-chat/web/src/debug-ui/ws/debugTimelineWsManager.ts`
2. any debug-ui components decoding timeline proto snapshots.

## Validation and Acceptance Criteria

1. Backend emits valid `TimelineUpsertV2` on every timeline upsert.
2. `/api/timeline` returns `TimelineSnapshotV2` only.
3. Browser hydrates and replays without duplicate/stale entities.
4. Unknown kinds render safely via fallback renderer.
5. A new domain kind can be added without editing pinocchio `transport.proto`.
6. Conversation debug listing persists non-zero `LastSeenVersion` across restart for active timeline conversations.

## Open Questions

1. Should `typed` be mandatory for some kind families, or strictly optional always?
2. Should `meta` include reserved keys (for example source stream or producer namespace)?
3. Should we move V1 messages to an archived proto file for historical traceability?

## References

- `pinocchio/proto/sem/timeline/transport.proto`
- `pinocchio/pkg/webchat/timeline_projector.go`
- `pinocchio/pkg/webchat/timeline_upsert.go`
- `pinocchio/pkg/webchat/conversation_service.go`
- `pinocchio/pkg/webchat/conversation.go`
- `pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go`
- `pinocchio/cmd/web-chat/web/src/sem/registry.ts`
- `pinocchio/cmd/web-chat/web/src/sem/timelineMapper.ts`
- `pinocchio/cmd/web-chat/web/src/ws/wsManager.ts`
- `pinocchio/cmd/web-chat/web/src/store/timelineSlice.ts`
