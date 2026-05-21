---
Title: Implementation Diary
Ticket: PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE
Status: active
Topics:
    - pinocchio
    - cleanup
    - timeline
    - sessionstream
    - persistence
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/timeline
      Note: Primary old user-facing command group to delete
    - Path: pkg/persistence/chatstore/timeline_store.go
      Note: Primary old interface to delete
    - Path: proto/sem
    - Path: ttmp/2026/05/21/PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE--remove-legacy-sem-timeline-stack/design-doc/01-removing-the-legacy-sem-timeline-stack.md
      Note: Design produced in Step 1
ExternalSources: []
Summary: Diary for the legacy sem timeline stack deletion design ticket.
LastUpdated: 2026-05-21T15:30:00-04:00
WhatFor: Track the cleanup design that separates the current sessionstream hydration store from the old sem/timeline chatstore stack.
WhenToUse: Read before resuming PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE implementation.
---


# Diary

## Goal

This diary captures the planning work for deleting Pinocchio's legacy `sem` timeline stack. It records the audit that identified the remaining legacy files, why they are safe candidates for removal, and what future implementation needs to preserve.

## Step 1: Create the deletion ticket and write the implementation guide

The goal of this step was to turn the discovery that `chatstore.TimelineStore` is an old timeline stack into a concrete cleanup ticket. The new sessionstream-based runtime already has the correct live timeline persistence abstraction, but old `sem` protobufs, generated files, store implementations, and CLI inspection commands remain in the repo.

I created the ticket and wrote a detailed design guide for a new intern. The guide separates current infrastructure from legacy infrastructure and gives a phased deletion plan that should keep `chatstore.TurnStore` intact while removing the obsolete `sem.timeline` code path.

### Prompt Context

**User prompt (verbatim):** "ok, create a new ticket to delete the legacy timeline stack. Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a new docmgr ticket for deleting the old `sem`/timeline stack, write an intern-ready technical design and implementation guide, store it in the ticket, and upload the document bundle to reMarkable.

**Inferred user intent:** The user wants to remove obsolete timeline code safely after the sessionstream migration, while preserving durable knowledge about what can be deleted, what must remain, and how not to confuse `chatstore.TimelineStore` with `sessionstream.HydrationStore`.

**Commit (code):** N/A — this step created design documentation only.

### What I did

- Created ticket `PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE`.
- Added design doc `design-doc/01-removing-the-legacy-sem-timeline-stack.md`.
- Added this implementation diary.
- Audited remaining `sem` and timeline files with `find` and `rg`.
- Identified the active non-generated references to `sem.timeline`:
  - `pkg/persistence/chatstore/timeline_store*.go`
  - `pkg/persistence/chatstore/timeline_store*_test.go`
  - `cmd/web-chat/timeline/*.go`
- Confirmed current web-chat runtime uses `sessionstream.HydrationStore` through `cmd/web-chat/app/server.go`, not `chatstore.TimelineStore`.
- Wrote a phased deletion plan covering old CLI commands, old stores, generated protobuf outputs, Buf/Makefile cleanup, and validation.

### Why

- Keeping two timeline concepts in the repository is confusing for new persistence work.
- The TUI turns persistence ticket should use `sessionstream.HydrationStore` for timeline persistence, not the older `chatstore.TimelineStore`.
- `proto/sem` and generated `pkg/sem` files appear to remain mostly because the old timeline stack still references them.
- Removing the old stack lowers maintenance burden and makes future docs more accurate.

### What worked

- The audit found a narrow live-code dependency chain:
  - `cmd/web-chat/timeline` and `chatstore.TimelineStore` depend on `pkg/sem/pb/proto/sem/timeline`.
  - current web-chat HTTP export and runtime hydration do not depend on this old store.
- The design can therefore recommend deleting the old command group and old store first, then deleting generated sem files after references are gone.
- The current `chatstore.TurnStore` is separate and can be preserved.

### What didn't work

- No code deletion was attempted in this step.
- The `cmd/web-chat/proto/sem` tree is a separate excluded proto island and needs an additional focused audit before deletion. The design records it as Phase 5 instead of pretending it is automatically safe.

### What I learned

- `chatstore.TimelineStore` and `sessionstream.HydrationStore` have different entity types, APIs, and storage schemas.
- The web-chat runtime has already moved to `sessionstream.HydrationStore`; the older `cmd/web-chat timeline` command group did not move with it.
- The root `buf.gen.yaml` is sem-output-specific and will need adjustment once `proto/sem` is deleted.
- The old generated TypeScript sem outputs appear unused by current frontend code, but typecheck should be the final authority.

### What was tricky to build

The tricky part was deciding what is legacy and what is current inside the same `chatstore` package. `chatstore.TimelineStore` is legacy for live timeline persistence, but `chatstore.TurnStore` is current and should be kept. A broad deletion of the whole package would break turn persistence and conflict with the TUI turns persistence ticket.

The design handles this by making the cleanup target precise: delete `chatstore.TimelineStore`, not `chatstore.TurnStore`; delete `proto/sem`, not `proto/pinocchio/chatapp`; delete old timeline CLI tooling, not sessionstream-backed web-chat export.

### What warrants a second pair of eyes

- Whether any users still rely on `web-chat timeline` commands for old database inspection.
- Whether generated `web/src/sem` or `cmd/web-chat/web/src/sem` files are imported by any path missed by the quick grep.
- Whether `cmd/web-chat/proto/sem` can be deleted in the same PR or should be handled separately.
- Whether `proto-gen-core` should be rewritten to generate `proto/pinocchio` or removed/renamed.

### What should be done in the future

- Implement the deletion plan in a focused cleanup PR.
- If timeline inspection remains useful, create new inspection commands against `sessionstream.HydrationStore` instead of preserving `chatstore.TimelineStore`.
- Coordinate with `PIN-20260521-TUI-TURNS-PERSISTENCE` so CLI helpers are split into current `TurnStore` and `sessionstream.HydrationStore` helpers.

### Code review instructions

- Start with the design doc:
  - `ttmp/2026/05/21/PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE--remove-legacy-sem-timeline-stack/design-doc/01-removing-the-legacy-sem-timeline-stack.md`
- Verify the deletion scope with:
  - `rg "TimelineEntityV2|TimelineSnapshotV2|TimelineStore|NewSQLiteTimelineStore|timelinepb" -n --glob '!ttmp/**'`
  - `rg "pkg/sem/pb|proto/sem|src/sem/pb" -n --glob '!ttmp/**'`
- Review carefully that `pkg/persistence/chatstore/turn_store*.go` remains.
- Validate with targeted tests, `make proto-gen`, `make schema-vet`, full Go tests, and web typecheck/lint.

### Technical details

The current target architecture after deletion is:

```text
Visible timeline persistence:
    sessionstream.HydrationStore

Model-context persistence:
    chatstore.TurnStore

Removed old stack:
    proto/sem
    pkg/sem
    generated sem TS
    chatstore.TimelineStore
    cmd/web-chat timeline commands
```

The old dependency chain to eliminate is:

```text
cmd/web-chat/timeline
  -> chatstore.SQLiteTimelineStore
  -> chatstore.TimelineStore
  -> pkg/sem/pb/proto/sem/timeline
  -> proto/sem/timeline
```
