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

## Step 2: Delete the legacy timeline stack

This step implemented the cleanup plan from the design document. The deletion removed the old `sem` protobuf tree, the generated sem Go and TypeScript outputs, the old `chatstore.TimelineStore` implementations, and the `web-chat timeline` command group that inspected the old SQLite timeline schema.

The important preservation rule was that only the legacy timeline stack was removed. `chatstore.TurnStore` remains in place because it is still the current storage seam for serialized final `turns.Turn` snapshots and is needed by web-chat and the upcoming TUI turns persistence work.

### Prompt Context

**User prompt (verbatim):** "Ok, create tasks (if you haven't yet), then kill all the old timeline stuff. Keep a diary as you work."

**Assistant interpretation:** Ensure the cleanup ticket has concrete tasks, implement the legacy timeline deletion now, and update the implementation diary while doing the work.

**Inferred user intent:** The user wants the old `sem`/timeline stack removed rather than merely documented, with ticket bookkeeping updated so later readers can understand what was deleted and why.

**Commit (code):** 051ce27 — "cleanup: remove legacy sem timeline stack"

### What I did

- Confirmed tasks already existed and then updated them after implementation.
- Deleted the old web-chat timeline CLI command group:
  - `cmd/web-chat/timeline/`
- Removed root command wiring from `cmd/web-chat/main.go`:
  - removed `timelinecmd` import;
  - removed `timelinecmd.AddToRootCommand(root)`.
- Deleted legacy timeline store code and tests:
  - `pkg/persistence/chatstore/timeline_store.go`
  - `pkg/persistence/chatstore/timeline_store_memory.go`
  - `pkg/persistence/chatstore/timeline_store_memory_test.go`
  - `pkg/persistence/chatstore/timeline_store_sqlite.go`
  - `pkg/persistence/chatstore/timeline_store_sqlite_test.go`
- Rewrote `pkg/cmds/chat_persistence.go` so it only opens `chatstore.TurnStore` via `openCLITurnStore` and no longer mentions `chatstore.TimelineStore`.
- Updated `pkg/cmds/chat_persistence_test.go` to test turns-only store opening and preserve `cliTurnStorePersister` coverage.
- Deleted sem source and generated outputs:
  - `proto/sem/`
  - `pkg/sem/`
  - `web/src/sem/`
  - `cmd/web-chat/web/src/sem/`
- Deleted the unused excluded web-chat proto island:
  - `cmd/web-chat/proto/`
- Updated generation/tooling configuration:
  - removed `buf.gen.yaml` because it only generated sem outputs;
  - updated `Makefile` so `proto-gen-core` uses `buf.chatapp.gen.yaml` and `buf.chatapp.web.gen.yaml` for `proto/pinocchio`;
  - removed the `proto-gen-web-chat` target that only generated the removed excluded proto island;
  - removed `pkg/sem/pb` from the gosec exclude list;
  - removed sem generated directory ignores from `cmd/web-chat/web/biome.json`.
- Updated frontend architecture docs to remove the historical `sem/pb` directory from the documented live frontend shape.
- Verified that no live-code references remain for:
  - `pkg/sem/pb`
  - `web/src/sem/pb`
  - `cmd/web-chat/web/src/sem/pb`
  - `proto/sem`
  - `cmd/web-chat/proto`
  - `TimelineStore`
  - `TimelineEntityV2`
  - `TimelineSnapshotV2`
  - `timelinepb`
  - `cmd/web-chat/timeline`

### Why

- The old stack used `sem.timeline.TimelineEntityV2` and `sem.timeline.TimelineSnapshotV2`, which are not the current `sessionstream` runtime types.
- New runtime timeline persistence should use `sessionstream.HydrationStore`, not `chatstore.TimelineStore`.
- Keeping the old CLI commands and generated protobuf trees made the repository appear to have two active timeline systems.
- Removing the old stack makes the current architecture easier to understand: visible timeline state belongs to `sessionstream`; model-context turns belong to `chatstore.TurnStore`.

### What worked

- The initial grep audit was accurate: once `cmd/web-chat/timeline` and `chatstore.TimelineStore` were removed, the `sem` protobuf tree and generated outputs had no live-code users.
- Targeted Go tests passed:
  - `go test ./pkg/persistence/chatstore ./pkg/cmds ./cmd/web-chat/... -count=1`
- Protobuf generation and schema vet passed after Makefile updates:
  - `make proto-gen`
  - `make schema-vet`
- Full Go tests passed:
  - `go test ./... -count=1`
- Frontend typecheck passed:
  - `npm run typecheck` from `cmd/web-chat/web`.
- Frontend lint passed after formatting generated chatapp TypeScript imports with Biome:
  - `npx --yes @biomejs/biome@2.3.8 check --write src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts src/chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb.ts`
  - `npm run lint` from `cmd/web-chat/web`.

### What didn't work

- The first frontend lint run failed after `make proto-gen` regenerated chatapp TypeScript files with import ordering that Biome rejected.
- Exact failure:
  - command: `cd cmd/web-chat/web && npm run lint`
  - files: `src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts` and `src/chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb.ts`
  - error: `The imports and exports are not sorted.`
- Fix:
  - ran Biome write mode on the two generated chatapp TypeScript files;
  - reran `npm run lint`, which passed.

### What I learned

- The root `buf.gen.yaml` was entirely sem-specific and could be removed once `proto/sem` was gone.
- Active chatapp protobuf generation already has dedicated templates:
  - `buf.chatapp.gen.yaml`
  - `buf.chatapp.web.gen.yaml`
- The excluded `cmd/web-chat/proto/sem` island had no non-proto consumers and was safe to remove with the legacy timeline cleanup.
- `pkg/doc/topics/webchat-frontend-architecture.md` had already described `sem/pb` as historical; after deletion it needed to be removed from the directory tree entirely.

### What was tricky to build

The tricky part was deleting aggressively without deleting the current persistence substrate. `pkg/persistence/chatstore` contained both the old timeline store and the current turn store. Removing the whole package would have broken web-chat turn persistence and the planned TUI turns persistence work.

The solution was to remove only `timeline_store*` files and keep `turn_store*` files. `pkg/cmds/chat_persistence.go` was then rewritten to expose a turns-only `openCLITurnStore` helper. This preserves the current `cliTurnStorePersister` path while removing the old timeline store dependency.

### What warrants a second pair of eyes

- Whether any external users relied on the removed `web-chat timeline` command group for old database inspection.
- Whether `proto-gen-core` should remain named `core` now that it only generates active Pinocchio chatapp protos.
- Whether generated chatapp TypeScript files should be excluded from Biome or consistently post-processed after `make proto-gen`.
- Whether replacement sessionstream hydration inspection commands should be added in a follow-up.

### What should be done in the future

- If timeline inspection is still needed, add new tooling against `sessionstream.HydrationStore` rather than restoring `chatstore.TimelineStore`.
- Coordinate with `PIN-20260521-TUI-TURNS-PERSISTENCE` so future TUI `--timeline-db` support opens a `sessionstream` SQLite hydration store.
- Consider renaming Makefile proto targets to make it clear they generate chatapp protobuf outputs.

### Code review instructions

- Start with deletion boundaries:
  - confirm `pkg/persistence/chatstore/turn_store*.go` remain;
  - confirm `pkg/persistence/chatstore/timeline_store*.go` are gone;
  - confirm `proto/pinocchio/chatapp/*` and `pkg/chatapp/pb/*` remain;
  - confirm `proto/sem`, `pkg/sem`, `web/src/sem`, and `cmd/web-chat/web/src/sem` are gone.
- Review `pkg/cmds/chat_persistence.go` to ensure turns DB behavior still works and no timeline store API remains.
- Review `Makefile` and `buf.yaml` to ensure proto generation no longer references deleted sem paths.
- Validate with:
  - `go test ./pkg/persistence/chatstore ./pkg/cmds ./cmd/web-chat/... -count=1`
  - `make proto-gen`
  - `make schema-vet`
  - `go test ./... -count=1`
  - `cd cmd/web-chat/web && npm run typecheck && npm run lint`

### Technical details

The removed dependency chain was:

```text
cmd/web-chat/timeline
  -> chatstore.SQLiteTimelineStore
  -> chatstore.TimelineStore
  -> pkg/sem/pb/proto/sem/timeline
  -> proto/sem/timeline
```

The retained current persistence split is:

```text
visible UI timeline:
  sessionstream.HydrationStore

model-context final turns:
  chatstore.TurnStore
```

The final reference audit command was:

```bash
rg "pkg/sem/pb|web/src/sem/pb|cmd/web-chat/web/src/sem/pb|proto/sem|cmd/web-chat/proto|src/sem/pb|TimelineStore|NewSQLiteTimelineStore|NewInMemoryTimelineStore|TimelineEntityV2|TimelineSnapshotV2|timelinepb|cmd/web-chat/timeline" -n --glob '!ttmp/**'
```

It returned no live-code matches.
