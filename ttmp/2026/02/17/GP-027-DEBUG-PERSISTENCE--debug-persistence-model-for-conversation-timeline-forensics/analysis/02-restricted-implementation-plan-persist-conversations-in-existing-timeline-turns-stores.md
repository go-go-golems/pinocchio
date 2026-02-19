---
Title: 'Restricted Implementation Plan: Persist Conversations in Existing Timeline/Turns Stores'
Ticket: GP-027-DEBUG-PERSISTENCE
Status: active
Topics:
    - debugging
    - persistence
    - backend
    - webchat
    - sqlite
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/persistence/chatstore/timeline_store.go
      Note: Timeline store interface extension point
    - Path: pinocchio/pkg/persistence/chatstore/timeline_store_memory.go
      Note: |-
        In-memory timeline store parity implementation
        In-memory parity for conversation index APIs
    - Path: pinocchio/pkg/persistence/chatstore/timeline_store_sqlite.go
      Note: |-
        Timeline SQLite schema migration target for conversation index
        Migration and query implementation for timeline_conversations table
    - Path: pinocchio/pkg/persistence/chatstore/turn_store.go
      Note: Turn-side enrichment options for per-conversation summaries
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: |-
        Conversation lifecycle touch points for persistence writes
        Lifecycle touch points where conversation index writes should occur
    - Path: pinocchio/pkg/webchat/router_debug_routes.go
      Note: |-
        `/api/debug/conversations` currently lists only live in-memory conversations
        Target endpoint merge behavior for live and persisted conversations
ExternalSources: []
Summary: Narrow plan to persist conversation-level records with minimal schema/API changes, enabling historical debug UI listing.
LastUpdated: 2026-02-17T15:01:00-05:00
WhatFor: Define a minimal implementation that lets debug UI discover past conversations without full observability expansion.
WhenToUse: Use when implementing the first persistence increment after GP-01 to remove live-memory-only debug conversation listing.
---


# Restricted Implementation Plan: Persist Conversations in Existing Timeline/Turns Stores

## Objective

Enable debug UI to show past conversations even after process restart, using the existing timeline and turns persistence setup.

Primary outcome:

- `/api/debug/conversations` returns both live and persisted conversation records.
- Selecting a historical conversation can load timeline data (`/api/debug/timeline`/`/api/timeline`) and turns data (`/api/debug/turns`) when available.

## Scope (Intentionally Narrow)

In scope:

- Add a persistent conversation index to the timeline store (SQLite + in-memory implementation parity).
- Write conversation metadata from `ConvManager` lifecycle points.
- Read/merge persisted records in debug conversation endpoints.
- Optional enrichment from turns store for turn count/last turn timestamp.

Out of scope:

- Raw event-log persistence.
- Full tracing/span instrumentation.
- Provider/tool detailed persistence expansion.
- Cross-database SQL joins via ATTACH.

## Current State

- `/api/debug/conversations` reads only `ConvManager.conns` in memory.
- Timeline store persists per-conversation entities but has no conversation index/list API.
- Turn store persists turn snapshots keyed by `conv_id`/`session_id`, but no conversation-list API.

Resulting gap:

- Historical conversations disappear from debug UI once no longer live.

## Proposed Data Model (MVP)

Add `ConversationRecord` persisted in timeline store.

Required fields:

- `conv_id` (PK)
- `session_id` (latest known)
- `runtime_key`
- `created_at_ms`
- `last_activity_ms`
- `last_seen_version` (timeline version watermark)
- `has_timeline` (bool)
- `source` (`live`, `persisted`, `merged`) computed at read time, not stored

Optional now (recommended):

- `last_error` (short string)
- `status` (`active|idle|closed|evicted|error`)

## Store Contract Changes

Extend `TimelineStore` with conversation index methods.

- `UpsertConversation(ctx, rec ConversationRecord) error`
- `GetConversation(ctx, convID string) (ConversationRecord, bool, error)`
- `ListConversations(ctx, limit int, sinceMs int64) ([]ConversationRecord, error)`

Keep existing methods untouched:

- `Upsert(ctx, convID, version, entity)`
- `GetSnapshot(ctx, convID, sinceVersion, limit)`

## SQLite Timeline Migration

In `timeline_store_sqlite.go`, add:

- table `timeline_conversations`
- indexes on `last_activity_ms DESC` and `session_id`

Proposed schema:

```sql
CREATE TABLE IF NOT EXISTS timeline_conversations (
  conv_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  runtime_key TEXT NOT NULL DEFAULT '',
  created_at_ms INTEGER NOT NULL,
  last_activity_ms INTEGER NOT NULL,
  last_seen_version INTEGER NOT NULL DEFAULT 0,
  has_timeline INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'active',
  last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS timeline_conversations_by_last_activity
  ON timeline_conversations(last_activity_ms DESC);
CREATE INDEX IF NOT EXISTS timeline_conversations_by_session
  ON timeline_conversations(session_id);
```

## In-Memory Timeline Store Parity

In `timeline_store_memory.go`:

- add `map[string]ConversationRecord`
- implement the same three methods for test/dev parity
- preserve sort order by `last_activity_ms DESC`

## Write Path Integration

Write/update conversation records at these points in `conversation.go`:

- On `GetOrCreate` when a conversation is created:
- initialize `created_at_ms` and `last_activity_ms`
- On any `touchLocked(now)` activity path:
- update `last_activity_ms`
- On timeline projector upsert callback:
- update `last_seen_version`

Practical approach:

- Add a small helper in `ConvManager` that computes `ConversationRecord` from `Conversation` and calls `timelineStore.UpsertConversation(...)` best-effort.
- Do not fail chat flow on index write errors; log warnings.

## Read Path and API Behavior

Update `router_debug_routes.go`:

- `/api/debug/conversations`:
- load live map from `ConvManager`
- load persisted list from `timelineStore.ListConversations`
- merge by `conv_id`
- if live and persisted both exist, mark source `merged` and prefer live volatile fields (`active_sockets`, `stream_running`, queue depth)
- keep historical records even when not live (`active_sockets=0`, `stream_running=false`)

- `/api/debug/conversations/:id`:
- fallback to persisted `GetConversation` when not found live
- include `has_timeline_source=true` if persisted timeline record exists

## Turns Join/Enrichment Strategy

Question: do we need turns DB conversation info for joining?

Answer for MVP: mostly no schema change needed.

- turns already include `conv_id` + `session_id`
- API can enrich conversation summaries by querying turns store per `conv_id` (or batched helper)

Recommended minimal enhancement:

- add optional `TurnStore` helper method later:
- `ListConversationHeads(ctx, limit int) ([]TurnConversationHead, error)`
- this avoids N+1 queries and provides `turn_count`, `last_turn_at_ms`

But this helper is not required for first delivery; historical listing can ship from timeline conversation index alone.

## Backward Compatibility and Risk

- Existing timeline and turn APIs remain unchanged.
- New store methods require updating stub/mock timeline stores in tests.
- Migration adds one table only; low risk.
- Failure mode is graceful: if conversation index read fails, endpoint can still return live conversations.

## Test Plan

Backend unit/integration tests:

1. `SQLiteTimelineStore` migration creates `timeline_conversations`.
2. Upsert/list/get conversation records round-trip correctly.
3. `/api/debug/conversations` returns persisted-only conversations when live map is empty.
4. merge behavior for live + persisted same `conv_id`.
5. `/api/debug/conversations/:id` returns persisted detail when not live.

Regression checks:

- existing timeline snapshot tests still pass.
- existing debug route enable/disable tests still pass.

## Implementation Sequence

1. Add new conversation record type + timeline store interface methods.
2. Implement SQLite and in-memory timeline conversation index.
3. Wire conversation writes from `ConvManager` lifecycle.
4. Wire debug API merge/read behavior.
5. Add tests.
6. Add docs/runbook updates for expected persisted conversation behavior.

## Acceptance Criteria

- After restart, historical conversations still appear in debug UI list.
- Opening a historical conversation can hydrate timeline/turns when corresponding data exists.
- No regression in live conversation listing fields.
- No chat runtime failures if persistence write fails (warning-only path).
