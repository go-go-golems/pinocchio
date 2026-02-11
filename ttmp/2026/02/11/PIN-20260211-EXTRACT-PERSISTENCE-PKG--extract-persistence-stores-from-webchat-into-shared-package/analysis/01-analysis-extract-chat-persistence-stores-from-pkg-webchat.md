---
Title: 'Analysis: Extract chat persistence stores from pkg/webchat'
Ticket: PIN-20260211-EXTRACT-PERSISTENCE-PKG
Status: active
Topics:
    - pinocchio
    - backend
    - refactor
    - persistence
    - analysis
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/timeline/db.go
      Note: Updated to consume extracted chatstore package
    - Path: pkg/persistence/chatstore/timeline_store.go
      Note: Extracted shared timeline store interface
    - Path: pkg/persistence/chatstore/timeline_store_sqlite.go
      Note: Extracted shared sqlite timeline implementation
    - Path: pkg/persistence/chatstore/turn_store.go
      Note: Extracted shared turn store interface
    - Path: pkg/persistence/chatstore/turn_store_sqlite.go
      Note: Extracted shared sqlite turn implementation
    - Path: pkg/persistence/chatstore/turn_store_sqlite_test.go
      Note: New shared turn-store sqlite test coverage
    - Path: pkg/webchat/router.go
      Note: Current consumer that will import extracted package
    - Path: pkg/webchat/router_options.go
      Note: Router options using store types that must be updated during extraction
    - Path: pkg/webchat/timeline_store.go
      Note: Current timeline store interface that should move to shared persistence package
    - Path: pkg/webchat/timeline_store_memory.go
      Note: Current in-memory timeline fallback candidate for extraction
    - Path: pkg/webchat/timeline_store_sqlite.go
      Note: Current sqlite timeline implementation to extract
    - Path: pkg/webchat/turn_store.go
      Note: Current turn store interface to extract
    - Path: pkg/webchat/turn_store_sqlite.go
      Note: Current sqlite turn implementation to extract
ExternalSources: []
Summary: Explains why chat persistence code should move out of pkg/webchat, proposes target package structure, and defines a low-risk migration plan with compatibility shims.
LastUpdated: 2026-02-11T12:05:00-05:00
WhatFor: Guide a refactor that makes persistence reusable from CLI and web surfaces without webchat coupling.
WhenToUse: Use when implementing or reviewing extraction of turn/timeline stores into a shared package.
---



# Analysis

## Problem statement

The SQLite persistence primitives used for chat history and timeline hydration currently live under `pkg/webchat`, even though they are conceptually generic storage components. This creates awkward coupling when `cmd/pinocchio` (terminal chat path) wants to reuse persistence, because CLI code ends up importing a package named and scoped around web concerns.

Current generic code is in:

- `pkg/webchat/timeline_store.go`
- `pkg/webchat/timeline_store_sqlite.go`
- `pkg/webchat/timeline_store_memory.go`
- `pkg/webchat/turn_store.go`
- `pkg/webchat/turn_store_sqlite.go`

These files do not depend on HTTP handlers, websocket lifecycle, or router behavior. They are storage abstractions plus SQLite implementations. Keeping them in `pkg/webchat` increases conceptual friction and makes reuse feel like layering violation.

## Why extraction is worth doing

1. Improves dependency clarity.
- Storage package should not look web-specific when used by CLI and other backends.

2. Reduces import confusion.
- `cmd/pinocchio` importing `pkg/webchat` for database stores is surprising for reviewers and future maintainers.

3. Enables clean reuse.
- Future consumers (batch tools, replay commands, diagnostics) can use persistence without pulling in webchat namespace.

4. Makes ownership explicit.
- Persistence code can evolve on its own roadmap (schema changes, retention helpers, backends) with clearer boundaries.

## What should and should not be extracted

## Extract (phase 1)

1. Interfaces and data structs:
- `TimelineStore`
- `TurnStore`
- `TurnSnapshot`
- `TurnQuery`

2. SQLite implementations:
- `SQLiteTimelineStore`
- `SQLiteTurnStore`
- DSN helpers (`SQLiteTimelineDSNForFile`, `SQLiteTurnDSNForFile`)

3. In-memory timeline store (if needed by webchat fallback and tests):
- `InMemoryTimelineStore`

4. Store-focused tests:
- `timeline_store_sqlite_test.go`
- `timeline_store_memory_test.go`
- add a missing/parallel turn-store sqlite test in the new package.

## Keep in `pkg/webchat`

1. Router wiring and lifecycle:
- `router.go`, `server.go`, `conversation.go`

2. Projection logic tied to webchat event flow:
- `timeline_projector.go`
- `sem_translator.go`

3. Conversation-scoped adapters that currently reference webchat structs:
- `turn_persister.go` (or rewrite later as generic helper with no `Conversation` dependency)

## Target package options

## Option A: `pkg/persistence/chatstore` (recommended)

Pros:
- Clear semantic scope (`persistence` + `chatstore`).
- Room for future additions (migrations, retention, export/import helpers).
- Avoids naming collisions with other store concepts.

Cons:
- Slightly longer import paths.

## Option B: `pkg/chatstore`

Pros:
- Short import path.

Cons:
- Less explicit that this is persistence/storage focused.

## Option C: `pkg/stores/chat`

Pros:
- Generic store grouping.

Cons:
- Might conflict with existing package naming conventions in repo.

Recommendation: choose Option A (`pkg/persistence/chatstore`) for long-term clarity.

## Compatibility strategy

A hard move can break consumers that currently import `pkg/webchat` store symbols. To reduce risk, use a two-step strategy.

## Step 1: add new package + compatibility shims

1. Create `pkg/persistence/chatstore` with extracted code.
2. Keep thin aliases/wrappers in `pkg/webchat` for one transition cycle:
- type aliases for interfaces/structs.
- wrapper constructors forwarding to new package.

Example shape in `pkg/webchat` (transitional):

```go
package webchat

import "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"

type TimelineStore = chatstore.TimelineStore
type TurnStore = chatstore.TurnStore

func NewSQLiteTimelineStore(dsn string) (*chatstore.SQLiteTimelineStore, error) {
    return chatstore.NewSQLiteTimelineStore(dsn)
}
```

3. Update internal imports gradually to use new package directly.

## Step 2: remove shims in follow-up

After internal and external callers migrate, remove wrappers from `pkg/webchat`.

## Migration plan (implementation order)

1. Create package `pkg/persistence/chatstore`.
2. Move/copy store files and adjust package names/imports.
3. Move/copy store tests and make sure they pass in new location.
4. Add temporary shims in `pkg/webchat`.
5. Update webchat router/options/conversation imports to new package.
6. Update CLI runtime code to import new package directly.
7. Run full test suite for affected packages.
8. Open follow-up ticket to remove compatibility shims once stable.

## Risk analysis

1. API breakage risk.
- Mitigation: compatibility shims in `pkg/webchat`.

2. Import cycles risk.
- Mitigation: ensure new package has no dependency on `pkg/webchat`; only shared dependencies (protobuf, sql, errors).

3. Behavior drift risk (schema or DSN differences).
- Mitigation: move code mostly unchanged first; avoid logic edits during extraction PR.

4. Test coverage gaps for turn store.
- Mitigation: add explicit turn-store sqlite tests in new package during extraction.

5. Refactor scope creep.
- Mitigation: keep projector/router changes out of extraction PR unless required by imports.

## Validation checklist for extraction PR

1. `go test ./pkg/persistence/chatstore/...` passes.
2. Existing webchat store tests still pass (or replaced equivalents in new package).
3. `go test ./pkg/webchat/...` passes with new imports.
4. `go test ./pkg/cmds/... ./pkg/ui/...` passes if CLI integration already exists.
5. Grep check: no storage-type definitions duplicated across packages after move.

## Expected outcome

After this extraction:

- `cmd/pinocchio` and `cmd/web-chat` can both consume persistence via a neutral package.
- Storage logic has a clearer home and cleaner dependency graph.
- Future persistence features can be added once in a shared package instead of feeling “owned by webchat.”

## Execution notes (implemented in this ticket)

This ticket was implemented as a hard-cut migration (no compatibility shims), per direction:

1. Added new package: `pkg/persistence/chatstore`.
2. Moved timeline + turn store interfaces and SQLite implementations there.
3. Added/ported tests there, including a dedicated `turn_store_sqlite_test.go`.
4. Updated `pkg/webchat` and `cmd/web-chat/timeline` imports to use `chatstore`.
5. Removed old store files from `pkg/webchat` entirely.

Implication: any external consumer importing old `pkg/webchat` store symbols must migrate to `pkg/persistence/chatstore` imports.

## Suggested follow-up tickets

1. Remove compatibility shims from `pkg/webchat` after one release cycle.
2. Extract any remaining generic persistence helpers (for turn persisting and retention) into the same shared package.
3. Add small developer docs showing canonical imports and migration notes.
