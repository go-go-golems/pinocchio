---
Title: 'PR #200 CI fixes and review analysis'
Ticket: PIN-20260819-PR200-CI-FIXES
Status: active
Topics:
    - ci
    - security
    - persistence
    - review
DocType: design-doc
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/glazed/pkg/help/cmd/export.go:glazed help export migrated from --format sqlite to --export-mode sqlite
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/pinocchio/.github/workflows/release.yml:Publish docs export_command still used --format sqlite after glazed migrated to --export-mode
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/pinocchio/go.mod:Declared github.com/moby/go-archive v0.2.0 (vulnerable, GHSA-hfg8-hc9c-6c3h)
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/pinocchio/pkg/chatapp/serverkit/stores.go:MemoryTurnStore.Save dedup key omitted CreatedAtMs, collapsing per-timestamp snapshots
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/pinocchio/pkg/chatapp/serverkit/stores_test.go:Added regression test for per-timestamp snapshot preservation
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/pinocchio/pkg/persistence/chatstore/mysql_turn_store.go:MySQL turn_block_membership PK includes snapshot_created_at_ms (identity reference)
    - Path: /home/manuel/workspaces/2026-08-13/ragkit-coinvault-mysql/pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go:SQLite turn_block_membership PK includes snapshot_created_at_ms (identity reference)
    - Path: abs:///home/manuel/code/wesen/go-go-golems/flowkit/cmd/flowkit/main.go
      Note: New minimal glazed CLI so flowkit help export works (PR flowkit#8)
    - Path: abs:///home/manuel/code/wesen/go-go-golems/flowkit/doc_embed.go
      Note: Embeds docs/ for the flowkit help system (PR flowkit#8)
    - Path: abs:///home/manuel/code/wesen/go-go-golems/go-go-wm/.github/workflows/release.yaml
      Note: cmd/XXX -> cmd/go-go-wm + --export-mode sqlite (PR go-go-wm#3)
    - Path: abs:///home/manuel/code/wesen/go-go-golems/go-go-wm/pkg/cmds/query.go
      Note: settings.NewGlazedSchema -> NewStructuredOutputSection migration for glazed v1.4.3 (PR go-go-wm#3)
    - Path: abs:///home/manuel/code/wesen/go-go-golems/infra-tooling/templates/github/publish-docsctl.template.yml
      Note: Template flag --format sqlite -> --export-mode sqlite (PR infra-tooling#33)
    - Path: repo://.github/workflows/release.yml
      Note: Changed export_command from --format sqlite to --export-mode sqlite (Fix 1)
    - Path: repo://go.mod
      Note: Bumped moby/go-archive 0.2.0 -> 0.3.3 to clear GHSA-hfg8-hc9c-6c3h (Fix 2)
    - Path: repo://pkg/chatapp/serverkit/stores.go
      Note: Added CreatedAtMs to MemoryTurnStore.Save dedup key (Fix 3)
    - Path: repo://pkg/chatapp/serverkit/stores_test.go
      Note: Added TestMemoryTurnStorePreservesSnapshotsPerTimestamp regression test
    - Path: repo://pkg/persistence/chatstore/mysql_turn_store.go
      Note: MySQL turn_block_membership PK includes snapshot_created_at_ms (identity reference for Fix 3)
    - Path: repo://pkg/persistence/chatstore/turn_store_sqlite.go
      Note: SQLite turn_block_membership PK includes snapshot_created_at_ms (identity reference for Fix 3)
    - Path: ws://glazed/pkg/help/cmd/export.go
      Note: glazed help export --export-mode definition (root cause for Fix 1)
ExternalSources:
    - https://github.com/go-go-golems/pinocchio/pull/200
    - https://github.com/go-go-golems/pinocchio/actions/runs/32199365530/job/95911030970
    - https://github.com/advisories/GHSA-hfg8-hc9c-6c3h
Summary: 'Three PR #200 blockers: glazed docs export must use --export-mode sqlite, moby/go-archive must be bumped past GHSA-hfg8-hc9c-6c3h, and the in-memory turn store must key snapshots on CreatedAtMs to match SQLite/MySQL.'
LastUpdated: 2026-08-19T16:20:00-04:00
WhatFor: 'Fixing the Publish docs, Dependency Review, and Codex P2 review blockers on pinocchio PR #200'
WhenToUse: Reference for the exact CI fixes and the in-memory snapshot identity correction
---













# PR #200 CI fixes and review analysis

## 1. Executive summary

PR #200 ("feat(persistence): Add MySQL backend for chat history and timelines", branch
`task/pinocchio-pr197-sessionstream-v012`) has three blockers preventing merge:

1. **Publish docs / publish-docs (fail)** — the `Export Glazed help SQLite database` step
   fails because glazed's `help export` command migrated its SQLite output from
   `--format sqlite` to `--export-mode sqlite`. The workflow still passes the old
   flag, which glazed now rejects with `Argument format has invalid choice sqlite`.
2. **Dependency Review (fail)** — the PR introduces `github.com/moby/go-archive@0.2.0`
   (transitively via `testcontainers-go` for the new disposable MySQL harness), which
   is flagged at **high** severity by `GHSA-hfg8-hc9c-6c3h` ("Crafted tar archive can
   write outside the extraction directory"). Patched in `0.3.0`; latest is `0.3.3`.
3. **Codex review (P2 comment)** — `pkg/chatapp/serverkit/stores.go` `MemoryTurnStore.Save`
   deduplicates snapshots on `(ConvID, SessionID, TurnID, Phase)` and **omits
   `CreatedAtMs`**, so repeated saves of the same turn/phase at different timestamps
   replace earlier snapshots. SQLite and MySQL key snapshots on
   `(conv_id, session_id, turn_id, phase, snapshot_created_at_ms)`, so the in-memory
   backend silently collapses history relative to the durable backends.

All three are addressed here. GoSec Security Scan, govulncheck, CodeQL, lint, test,
buf, and the MySQL disposable integration tests already pass and are not in scope.

## 2. Problem statement and scope

### In scope
- `.github/workflows/release.yml` `publish-docs` job `export_command`.
- `go.mod` / `go.sum` bump of `github.com/moby/go-archive` (and the transitive
  `klauspost/compress`, `moby/sys/user`, `moby/sys/userns` upgrades `go get` pulls in).
- `pkg/chatapp/serverkit/stores.go` `MemoryTurnStore.Save` identity semantics.
- A regression test in `pkg/chatapp/serverkit/stores_test.go`.

### Out of scope
- The non-failing `cpuguy83/dockercfg` OpenSSF Scorecard warning (2.9 < 3); it is a
  warning only and `fail-on-severity: high` does not fail on scorecard warnings.
- Re-architecting the durable backends; their identity model is the reference, not
  the thing being changed.
- GoSec excludes (`G101,G304,...`) — already passing.

## 3. Current-state architecture (evidence-based)

### 3.1 Docs export pipeline

`release.yml` `publish-docs` reuses the shared
`go-go-golems/infra-tooling/.github/workflows/publish-docsctl.yml@main` workflow and
passes an `export_command` that builds the help SQLite DB:

```yaml
export_command: GOWORK=off go run ./cmd/pinocchio help export --format sqlite --output-path .docsctl/help.sqlite
```

`cmd/pinocchio/main.go` registers the glazed help tree via
`help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)` (line 159), which calls
`AddExportCommand(helpCmd, hs)` → `NewExportCommand`. In glazed
(`pkg/help/cmd/export.go`) the export command defines `--export-mode` with values
`glazed | files | sqlite` (default `glazed`) and a separate `--format` (the Glazed
tabular output format: json/csv/table/yaml). The `sqlite` value was moved from
`--format` to `--export-mode` in glazed commit `cdb5537` ("Cleanup flags and
middlewares"), which is contained in tags `v1.4.0`–`v1.4.3`. Pinocchio requires
`glazed v1.4.3`, so the published module the CI builds against (`GOWORK=off`) already
only accepts `--export-mode sqlite`.

### 3.2 Turn snapshot identity across backends

`pkg/persistence/chatstore/turn_store.go` defines:

```go
type TurnSnapshot struct {
    ConvID, SessionID, TurnID, Phase string
    RuntimeKey, InferenceID          string
    CreatedAtMs                      int64
    Payload                          string
}
```

- **SQLite** (`turn_store_sqlite.go`): `turn_block_membership` table
  `PRIMARY KEY (conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal)`
  (line 101). `Save` deletes membership rows for the exact
  `(conv, session, turn, phase, snapshot_created_at_ms)` and re-inserts (lines 331–332),
  i.e. snapshot identity **includes the timestamp**; different timestamps are kept.
- **MySQL** (`mysql_turn_store.go`): same composite key including
  `snapshot_created_at_ms` (line 173); `INSERT ... ON DUPLICATE KEY UPDATE` (line 320)
  preserves distinct timestamps.
- **In-memory** (`pkg/chatapp/serverkit/stores.go`): `MemoryTurnStore.Save` matched on
  `(ConvID, SessionID, TurnID, Phase)` only and replaced the first match, dropping
  `CreatedAtMs` from the identity. This is the divergence the review flagged.

## 4. Gap analysis

| # | Blocker | Root cause | Gap |
|---|---------|-----------|-----|
| 1 | Publish docs fail | `--format sqlite` removed from export mode | Workflow uses pre-`v1.4.0` invocation |
| 2 | Dependency Review fail | `moby/go-archive@0.2.0` introduced via testcontainers | Indirect dep not pinned past the advisory |
| 3 | Codex P2 review | In-memory dedup omits `CreatedAtMs` | Backend identity mismatch vs SQLite/MySQL |

## 5. Proposed solution

### Fix 1 — Docs export flag
Change `release.yml`:
```diff
- export_command: GOWORK=off go run ./cmd/pinocchio help export --format sqlite --output-path .docsctl/help.sqlite
+ export_command: GOWORK=off go run ./cmd/pinocchio help export --export-mode sqlite --output-path .docsctl/help.sqlite
```

### Fix 2 — Vulnerable indirect dependency
Bump the indirect dep (CI uses `GOWORK=off`, so `go.mod`/`go.sum` must be
self-consistent without the workspace):
```bash
GOWORK=off go get github.com/moby/go-archive@v0.3.3
GOWORK=off go mod tidy
```
Resulting `go.mod` delta: `moby/go-archive 0.2.0 → 0.3.3`, plus transitive
`klauspost/compress 1.18.6 → 1.18.7`, `moby/sys/user 0.4.0 → 0.4.1`, and a new
`moby/sys/userns 0.1.0` indirect.

### Fix 3 — In-memory snapshot identity
Include `CreatedAtMs` in the dedup key:
```diff
- if ... && s.turns[i].Phase == snap.Phase {
+ if ... && s.turns[i].Phase == snap.Phase && s.turns[i].CreatedAtMs == snap.CreatedAtMs {
      s.turns[i] = snap
      return nil
  }
```
Semantics now match the durable backends: an exact `(conv, session, turn, phase,
createdAtMs)` re-save replaces (idempotent, like SQLite's DELETE+reinsert for that
`snapshot_created_at_ms`); a save at a new timestamp appends a distinct snapshot.

### Regression test
`TestMemoryTurnStorePreservesSnapshotsPerTimestamp` saves the same
`(conv, session, turn, phase)` at `t=100` and `t=200`, asserts `List` returns both,
then re-saves `t=100` and asserts it replaces only that row.

## 6. Decision records

### DR-1: `--export-mode sqlite` vs pinning an older glazed
- **Context:** glazed moved `sqlite` from `--format` to `--export-mode`.
- **Options:** (a) update the workflow flag; (b) downgrade pinocchio's glazed to a
  pre-`cdb5537` tag.
- **Decision:** (a) update the flag.
- **Rationale:** `--export-mode` is the supported, released API in `v1.4.3`; downgrading
  glazed would lose other fixes and is not sustainable.
- **Consequences:** workflow matches current glazed; future glazed changes to the
  export command will need the same attention.
- **Status:** accepted.

### DR-2: Bump to `v0.3.3` rather than the minimum patched `v0.3.0`
- **Context:** Advisory patches in `0.3.0`; `0.3.3` is the latest.
- **Decision:** bump to `v0.3.3`.
- **Rationale:** take the latest patch line of the same minor to avoid immediate
  re-flagging; `go mod tidy` keeps the graph minimal. The transitive
  `klauspost/compress` / `moby/sys/*` upgrades are required by `v0.3.3` and compile/test
  clean.
- **Consequences:** slightly newer transitive deps; validated by build + tests.
- **Status:** accepted.

### DR-3: Include `CreatedAtMs` in the in-memory key (not "append always")
- **Context:** Review asked to "preserve every in-memory turn snapshot".
- **Options:** (a) add `CreatedAtMs` to the dedup key; (b) always append (never replace).
- **Decision:** (a) include `CreatedAtMs`.
- **Rationale:** the durable backends replace an exact `(…, snapshot_created_at_ms)`
  re-save (idempotent persistence), not append-duplicate it. Matching that semantics
  keeps the in-memory backend behaviorally consistent and prevents unbounded row
  growth on retries of the same timestamp.
- **Consequences:** re-saving the same timestamp replaces; new timestamps are kept.
- **Status:** accepted.

## 7. Phased implementation plan (file-level)

1. `pinocchio/.github/workflows/release.yml` — swap `--format sqlite` → `--export-mode sqlite`.
2. `pinocchio/go.mod` / `go.sum` — `GOWORK=off go get github.com/moby/go-archive@v0.3.3 && go mod tidy`.
3. `pinocchio/pkg/chatapp/serverkit/stores.go` — add `CreatedAtMs` to the dedup condition.
4. `pinocchio/pkg/chatapp/serverkit/stores_test.go` — add `TestMemoryTurnStorePreservesSnapshotsPerTimestamp`.
5. Validate: `gofmt`, `go test ./pkg/chatapp/... ./pkg/persistence/... ./pkg/testsupport/...`,
   `go vet`, `GOWORK=off go build ./...`.

## 8. Testing and validation strategy

- **Fix 1:** reproduce CI locally with `GOWORK=off go run ./cmd/pinocchio help export
  --export-mode sqlite --output-path /tmp/help.sqlite` → produces a `sections` table;
  the old `--format sqlite` reproduces the CI failure (`Argument format has invalid
  choice sqlite`, exit 1, no file).
- **Fix 2:** `GOWORK=off go build ./...` and `go test ./pkg/persistence/...
  ./pkg/testsupport/...` (testcontainers-using packages) pass; `go mod tidy` is a
  no-op diff after the bump.
- **Fix 3:** new unit test passes; existing `TestMemoryTurnStoreLoadLatestFinalTurn`
  still passes (it uses distinct turn IDs, so identity is unaffected).

## 9. Risks, alternatives, and open questions

- **Risk:** `moby/sys/userns` is a new transitive; if a future Go version or a
  testcontainers constraint rejects it, re-pin. Low likelihood; build is green.
- **Risk:** the `cpuguy83/dockercfg` scorecard warning could become a failure if the
  workflow later sets `fail-on-scorecard`. Not addressed here (warning only).
- **Open question:** whether the in-memory store should also offer a "prune by
  timestamp" helper to bound growth across long-lived sessions. Out of scope; the
  durable backends already cap via the composite key.
- **Alternative considered:** moving the export invocation into a pinned
  `make docs-export` target so the workflow and local dev share one source of truth.
  Deferred — out of scope for unblocking PR #200.

## 10. References (key files)

- `pinocchio/.github/workflows/release.yml` (publish-docs job)
- `pinocchio/.github/workflows/dependency-scanning.yml` (fail-on-severity: high)
- `pinocchio/go.mod` (moby/go-archive indirect)
- `pinocchio/pkg/chatapp/serverkit/stores.go` (MemoryTurnStore)
- `pinocchio/pkg/persistence/chatstore/turn_store_sqlite.go` (identity reference)
- `pinocchio/pkg/persistence/chatstore/mysql_turn_store.go` (identity reference)
- `glazed/pkg/help/cmd/export.go` (--export-mode definition)
- Advisory: https://github.com/advisories/GHSA-hfg8-hc9c-6c3h
