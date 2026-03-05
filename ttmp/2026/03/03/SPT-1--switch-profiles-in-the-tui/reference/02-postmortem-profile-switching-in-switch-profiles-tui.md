---
Title: 'Postmortem: profile switching in switch-profiles-tui'
Ticket: SPT-1
Status: active
Topics:
    - tui
    - profiles
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: bobatea/pkg/chat/model.go
      Note: Submit/intercept path and completion cleanup
    - Path: pinocchio/cmd/switch-profiles-tui/main.go
      Note: Final TUI wiring
    - Path: pinocchio/cmd/switch-profiles-tui/persistence.go
      Note: Turn snapshot persistence
    - Path: pinocchio/pkg/ui/profileswitch/backend.go
      Note: Profile-aware backend (session.Builder swapping
    - Path: pinocchio/pkg/ui/timeline_persist.go
      Note: Timeline persistence and attribution
    - Path: pinocchio/scripts/switch-profiles-tui-smoke-and-verify.sh
      Note: Regression harness
ExternalSources: []
Summary: ""
LastUpdated: 2026-03-03T19:14:53.367743756-05:00
WhatFor: ""
WhenToUse: ""
---


# Postmortem: profile switching in switch-profiles-tui

## Goal

Provide an intern-friendly, end-to-end retrospective of how “profile switching in the TUI” was designed, implemented, debugged, and validated. This includes the final architecture, the runbook, the persistence contract, and the main reliability issues we hit (and why).

## Context

This ticket adds profile switching to a Bubble Tea chat TUI using the Geppetto “profile registries” system. The deliverable is a runnable CLI (`switch-profiles-tui`) that:

- Loads profiles from a registry stack (`--profile-registries`).
- Picks an initial profile (`--profile` optional).
- Allows switching profiles at runtime via `/profile` and a modal picker.
- Runs *real provider inference* (using `/tmp/profile-registry.yaml` in smoke tests).
- Persists:
  - **turn snapshots** (SQLite turns DB) with `runtime_key` and `inference_id` per turn,
  - **timeline projection** (SQLite timeline DB) with runtime/profile attribution on assistant messages,
  - explicit **profile switch markers** in the timeline.

Repositories involved (this workspace is a multi-repo `go.work`):

- `bobatea/`: Bubble Tea chat UI model (submit path, keybindings, header view, interceptor hook).
- `geppetto/`: inference engines, profiles domain model, events, and provider integration.
- `pinocchio/`: application wiring, persistence stores, UI forwarders/persisters, and the new TUI command.

Key “domain model” concepts:

- **Profile**: named config that maps to an “effective runtime” (provider model, system prompt, runtime patches).
- **Registry stack**: a chain of sources (YAML/SQLite) that produce a single merged profile registry.
- **Resolved runtime**: the effective step settings + metadata derived from (registry stack + selected profile).
- **Turn persistence**: stores the canonical serialized `turns.Turn` snapshot; includes runtime attribution.
- **Timeline persistence**: stores a projection for UI hydration/replay (entities like “assistant message”).

## Quick Reference

### Runbook (local)

**1) Ensure a working registry exists**

- Required: `/tmp/profile-registry.yaml` must exist and contain a usable profile registry stack with working provider credentials.
- This file contains secrets; do not print or commit it.

**2) End-to-end smoke + persistence verification (tmux automation + real inference)**

From `pinocchio/`:

```bash
./scripts/switch-profiles-tui-smoke-and-verify.sh
```

This script:
- Verifies startup fails when zero profiles load.
- Runs the TUI in tmux with `--profile-registries /tmp/profile-registry.yaml`.
- Submits a prompt under profile A.
- Runs `/profile <profileB>`.
- Submits a second prompt under profile B.
- Queries SQLite and asserts runtime/profile boundaries.

**3) Manual run**

```bash
go run ./cmd/switch-profiles-tui \
  --profile-registries /tmp/profile-registry.yaml \
  --profile mento-haiku-4.5 \
  --conv-id demo-1 \
  --timeline-db /tmp/demo-1.timeline.db \
  --turns-db /tmp/demo-1.turns.db
```

In the TUI:
- Type a prompt and press `tab` to submit.
- Type `/profile` to open a modal picker.
- Or `/profile mento-sonnet-4.6` to switch directly.

### Persistence contract (what we assert)

**Turns DB** (`--turns-db`):
- Table: `turns`
- Expected after two prompts:
  - 2 rows for the same `conv_id`
  - `runtime_key` differs across the two turns
  - `inference_id` is non-empty (real inference happened)

**Timeline DB** (`--timeline-db`):
- Table: `timeline_entities`
- Expected after two prompts + one switch:
  - ≥ 2 assistant `message` entities with:
    - `props.runtime_key`
    - `props["profile.slug"]`
    - `props["profile.registry"]`
    - `props.streaming=false` (final)
  - ≥ 1 `profile_switch` entity with:
    - `props.from`, `props.to`
    - `props.runtime_key`, `props.runtime_fingerprint`
    - `props["profile.slug"]` (target)

### File entry points (where to start reading)

- TUI command: `pinocchio/cmd/switch-profiles-tui/main.go`
- Profile manager/backend: `pinocchio/pkg/ui/profileswitch/`
- Turn persistence: `pinocchio/cmd/switch-profiles-tui/persistence.go`
- Timeline persistence: `pinocchio/pkg/ui/timeline_persist.go`
- Runtime attribution propagation (events): `geppetto/pkg/steps/ai/runtimeattrib/runtimeattrib.go`
- UI chat model: `bobatea/pkg/chat/model.go`
- Scripts:
  - `pinocchio/scripts/switch-profiles-tui-smoke-and-verify.sh`
  - `pinocchio/scripts/switch-profiles-tui-tmux-smoke.sh`
  - `pinocchio/scripts/switch-profiles-tui-verify-persistence.sh`
  - `pinocchio/scripts/switch-profiles-tui-startup-fail.sh`

## Usage Examples

### “I changed something; did I break profile switching?”

Run:

```bash
cd pinocchio
./scripts/switch-profiles-tui-smoke-and-verify.sh
```

If it fails:
- The script prints a tmux pane tail and SQLite counts.
- Re-run with a different log level:

```bash
LOG_LEVEL=debug ./scripts/switch-profiles-tui-smoke-and-verify.sh
```

### “Does inference actually happen with this profile?”

Run a single inference (no TUI):

```bash
cd pinocchio
go run ./scripts/profile-infer-once.go --profile mento-haiku-4.5
```

This prints:
- selected profile + runtime key + runtime fingerprint
- assistant text output (e.g. `OK.`)

## What Actually Happened (intern narrative)

### What shipped (feature summary)

We shipped a new runnable TUI command that makes profiles the selection knob:

- `/profile` (modal picker) and `/profile <slug>` (direct switch).
- Startup fails if `--profile-registries` loads zero profiles.
- Turn persistence stores `runtime_key` and `inference_id` per turn.
- Timeline persistence stores runtime/profile attribution on assistant messages.
- Timeline contains explicit `profile_switch` entities marking when a switch occurred.

### Where it got tricky (big picture)

The “hard parts” were not the UI for switching; they were:

1) **Event routing reliability under streaming** (Watermill + in-memory pubsub).
2) **Shutdown ordering and context cancellation** (Bubble Tea UI cleanup vs persister completion).
3) **SQLite write contention** (multiple goroutines writing timeline entities concurrently).

### Key failures + root causes + fixes

#### 1) Watermill hang / inference stall

**Symptom**
- tmux smoke automation would hang (TUI appeared stuck mid-run) or inference would stop progressing.

**Root cause**
- `gochannel` pubsub can block publishing until subscriber ACK by default (or with the wrong config).
- If the inference path publishes events and the publish blocks waiting for ACKs, and the subscribers are not actively draining (or are stalled behind UI dispatch), you can deadlock the streaming loop.

**Fix**
- In `pinocchio/cmd/switch-profiles-tui/main.go`, the router uses `gochannel.NewGoChannel` with:
  - `OutputChannelBuffer: 256`
  - `BlockPublishUntilSubscriberAck: false`

This makes publishing non-blocking and adds buffering for bursts.

#### 2) “UI shows an answer but turns DB is empty”

**Symptom**
- The assistant response appears in the UI, but `turns` table stays empty (scripts time out waiting for persisted turns).

**Root cause**
- `bobatea/pkg/chat/model.go` called `backend.Kill()` during completion cleanup.
- That cancellation could happen after UI saw “final”, but before the inference pipeline finished persisting the final turn snapshot.

**Fix**
- `bobatea/pkg/chat/model.go` no longer calls `backend.Kill()` when it receives `BackendFinishedMsg`.
- This allows the backend/persister to finish cleanly without being canceled by UI cleanup.

#### 3) SQLite “database is locked” / missing timeline entities

**Symptom**
- Intermittent “database is locked” errors, and timeline entities (especially under streaming) sometimes failed to land.

**Root cause**
- SQLite allows only one writer at a time; concurrent `Upsert` calls from multiple goroutines collide.
- Under streaming, timeline persistence can write frequently (partials) while other code paths (profile switch marker) also write.

**Fix**
- Serialize timeline store `Upsert` calls in-process (a mutex wrapper store in the TUI command).
- Also serialize writes in the timeline persistence handler.

#### 4) “context canceled” in timeline persistence (message contexts)

**Symptom**
- Timeline persistence logs `context canceled` during streaming and sometimes misses final entities.

**Root cause**
- Watermill message contexts (`msg.Context()`) can be canceled unexpectedly depending on ack/teardown ordering.
- Using `msg.Context()` directly as the SQLite write context makes persistence vulnerable to cancellation unrelated to “we actually want to stop writing”.

**Fix**
- Timeline persistence uses a detached bounded context (`context.WithTimeout(context.Background(), 2*time.Second)`) for writes.

### Why we persisted profile switches directly (in addition to EventInfo)

We still publish `EventInfo("profile-switched")`, but the persistence guarantee for switch markers is implemented by directly upserting a `profile_switch` entity to the timeline store:

- It’s simple and deterministic.
- It uses the same version counter (`atomic.Uint64`) as the timeline persister so ordering remains monotonic within the conversation timeline.

This avoids relying on the event→handler→persistence chain for correctness in the most timing-sensitive path.

## Commit Map (for archeology)

This work spans multiple repos in the workspace:

- `bobatea`:
  - `f0ba314` submit interceptor + header hook
  - `34b05be` stop killing backend on completion (persistence reliability)
- `geppetto`:
  - `ae962af` propagate runtime/profile attribution into events from turns
- `pinocchio`:
  - `a9500a3` add `switch-profiles-tui` command + profile backend
  - `322d2c0` persist runtime attribution + `profile_switch` markers + tests
  - `25a524d` initial scripts for tmux smoke + persistence verification
  - `ba0623d` harden router + persistence (buffered gochannel, locks, direct switch persist)
  - `4f719cd` scripts: startup-fail + improved smoke/verify + infer helper

## Lessons / Guidance for the Next Intern

- Treat **contexts** as a first-class part of the design. A “final event” is not always “all side effects finished”.
- Avoid binding persistence to the lifecycle of a transport message context.
- Assume SQLite will be unhappy with concurrent writers unless you serialize or configure busy timeouts intentionally.
- Keep a single “easy to run” script that proves the feature end-to-end (tmux + DB checks) and iterate until it is reliable.

## Related

- Design doc: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/design-doc/01-design-profile-switching-in-switch-profiles-tui.md`
- Diary: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/01-investigation-diary.md`
- Concurrency deep dive: `ttmp/2026/03/03/SPT-1--switch-profiles-in-the-tui/reference/03-event-context-concurrency-watermill-bubble-tea-and-sqlite.md`

## Related

<!-- Link to related documents or resources -->
