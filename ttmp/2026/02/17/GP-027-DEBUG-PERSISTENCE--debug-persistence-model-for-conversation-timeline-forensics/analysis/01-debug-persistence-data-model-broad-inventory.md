---
Title: Debug Persistence Data Model (Broad Inventory)
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
      Note: |-
        Timeline store contract that may be expanded for conversation indexing
        Current timeline persistence interface boundary
    - Path: pinocchio/pkg/persistence/chatstore/turn_store.go
      Note: |-
        Turn persistence contract and potential enrichment join points
        Turn persistence interface for conversation enrichment
    - Path: pinocchio/pkg/webchat/router_debug_routes.go
      Note: |-
        Current debug conversation endpoints and response shape
        Defines current live-memory debug conversation listing endpoints
    - Path: pinocchio/pkg/webchat/timeline_projector.go
      Note: |-
        Timeline projection write path and event-derived entities
        Projection write path and event-to-entity mapping
ExternalSources: []
Summary: Catalog of conversation, timeline, turn, event, transport, runtime, and failure data that can be persisted for forensic debugging.
LastUpdated: 2026-02-17T15:01:00-05:00
WhatFor: Define the full candidate debug persistence surface before narrowing implementation scope.
WhenToUse: Use when deciding what to persist for deep debugging, replayability, and incident forensics.
---


# Debug Persistence Data Model (Broad Inventory)

## Purpose

This document captures the broadest useful set of data we can persist for debugging across chat runtime, timeline projection, turn snapshots, transport boundaries, middleware internals, and system/runtime failures.

This is a capability inventory, not an MVP scope.

## Design Principles

- Correlate everything with stable IDs.
- Preserve both logical time (event sequence/version) and wall-clock time.
- Keep enough raw data for replay/forensics, but default to redacted payloads.
- Separate high-volume append-only event logs from low-volume indexed metadata.
- Support both live-debug use cases and post-mortem/historical queries.

## Identity and Correlation Keys

Persist these keys across stores/APIs:

- `conv_id`
- `session_id`
- `turn_id`
- `run_id`
- `request_id` / `idempotency_key`
- `stream_id`
- `event_id`
- `seq` (stream-local monotonic order)
- `version` (projection state version)
- `trace_id` / `span_id` (future tracing integration)

Time fields:

- `occurred_at_ms` (source event time)
- `ingested_at_ms` (persistence write time)
- `updated_at_ms` (record update time)

## Data Domains

### Conversation Domain

Primary record for conversation discovery and triage:

- identity: `conv_id`, `session_id`, `runtime_key`, `runtime_fingerprint`, `profile_slug`
- lifecycle: `created_at_ms`, `last_activity_ms`, `closed_at_ms`, `evicted_at_ms`
- status: `active|idle|closed|evicted|error`
- runtime: `active_request_key`, `stream_running`, websocket counts (current + high-water)
- buffering: queue depth (current + high-water), buffered event count
- capabilities: `has_timeline`, `has_turns`, `has_raw_events`
- incident markers: last error code/class/message, last panic flag

### Timeline Projection Domain

For hydration/view reconstruction:

- timeline entities (already present)
- entity metadata: `entity_id`, `kind`, `created_at_ms`, `updated_at_ms`, first/last seen versions
- projection internals: projection handler version, upsert latency, drop/throttle counters
- projection integrity: version gaps, duplicate entity writes, decode/transform failures

### Turn Domain

For authored + post-inference state analysis:

- identity: `conv_id`, `session_id`, `turn_id`, `phase`
- payloads: serialized turn snapshots per phase
- phase timings: first seen, finalization time, phase-to-phase latency
- block-level fingerprints: content hashes, block lineage, reused blocks
- parse quality: parse errors, canonicalization warnings

### Event Log Domain (Raw + Parsed)

For root-cause and replay:

- raw SEM envelope bytes/text
- parsed envelope fields: `type`, `id`, `seq`, `stream_id`, `data`
- source path: websocket, internal bus, redis subscriber, replay
- decode status/errors
- ordering anomalies: duplicate seq, gaps, out-of-order markers

### Provider / Model Domain

For LLM request debugging:

- provider/model/version
- request/response payload hashes (optionally redacted body)
- token usage (`input`, `output`, cache/reasoning where available)
- finish reason, retries, backoff cause
- latency breakdown (queue, provider, postprocess)

### Tooling Domain

For tool-call diagnostics:

- tool call identity + tool version
- input args (raw + redacted variants)
- output/result (raw + redacted variants)
- status, start/end/duration, retries, timeout/cancel
- side effects references (files touched, db writes, outbound requests)

### Middleware / Agent Domain

For behavior explainability:

- middleware chain and per-step duration
- mode/routing decisions and rationale metadata
- prompt transforms (pre/post)
- guardrail/moderation decisions
- planning/router confidence and branch selection

### Transport/API Domain

For client/server boundary failures:

- HTTP request method/path/status/latency/size
- origin/user-agent/cors/auth outcomes
- websocket connect/disconnect reasons
- ping/pong latency metrics
- route mount prefix in effect (`/` vs `/chat`)

### Runtime/System Domain (Lower-Level)

For infrastructure-level debugging:

- build SHA/version, process id, uptime
- memory/CPU snapshots, goroutine count, GC pause stats
- open FD count, sqlite busy/lock rates
- panic/recover entries with stack traces
- dependency health (redis/message bus/provider reachability)

### Error Taxonomy

Normalize failures for fast filtering:

- `error_code` (stable)
- `error_class` (`validation`, `provider`, `tool`, `projection`, `storage`, `transport`, `runtime`)
- user-safe message
- raw cause chain + stack trace
- correlated IDs (`conv_id`, `turn_id`, `event_id`, `trace_id`)

## Storage Layers

- Conversation index: low-volume, indexed for listing/filtering/sorting.
- Timeline entities: medium-volume projection state for UI hydration.
- Turn snapshots: medium-volume forensic snapshots by phase.
- Raw event log: high-volume append-only (retention + compaction needed).
- Metrics/diagnostics: rollup tables for fast dashboards.

## Retention and Privacy

- Default to redacted payload storage for prompts/tool inputs/results.
- Keep optional raw payload mode behind explicit debug flag.
- Attach data sensitivity tags per field.
- Define retention tiers:
- hot (recent incidents, full fidelity)
- warm (summaries + partial payloads)
- cold/archive (compressed + restricted access)

## Query Patterns to Support

- List conversations by recent activity, including non-live historical sessions.
- Open a conversation and inspect timeline + turns + events in one joined view.
- Filter by failure class/tool name/model/provider/status.
- Jump from any UI event back to raw envelope and correlated turn/model/tool rows.
- Reconstruct a replay bundle for deterministic repro.

## Suggested Rollout (Broad)

1. Conversation index persistence (first unlock for historical discovery).
2. Better turn/timeline aggregation APIs for conversation summaries.
3. Optional raw event log persistence.
4. Provider/tool/middleware telemetry persistence.
5. Replay and incident package export.
