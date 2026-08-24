---
Title: Diary
Ticket: PINOCCHIO-TOOLCALL-1
Status: active
Topics:
    - security
    - chatapp
    - backend
    - runtime
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://cmd/web-chat/internal/appserver/routes_frontend_tools.go
      Note: Stable HTTP mapping for invocation rejection codes (commit 8cdc9af)
    - Path: repo://cmd/web-chat/internal/appserver/server_test.go
      Note: Unsolicited result rejection contract (commit 8cdc9af)
    - Path: repo://pkg/chatapp/frontendtools/manager.go
      Note: Session-bound pending identity and strict result validation (commit 8cdc9af)
    - Path: repo://pkg/chatapp/frontendtools/manager_test.go
      Note: Cross-session, duplicate, mismatch, and unknown-result regressions (commit 8cdc9af)
    - Path: repo://pkg/chatapp/frontendtools/terminal_store.go
      Note: Bounded count/age terminal retention (commit 84c6e63)
    - Path: repo://pkg/chatapp/frontendtools/terminal_store_test.go
      Note: Deterministic count/TTL eviction tests (commit 84c6e63)
ExternalSources: []
Summary: Chronological investigation, design, validation, and delivery record for Pinocchio frontend-tool invocation/result hardening.
LastUpdated: 2026-08-23T17:25:00-04:00
WhatFor: Let an implementer or reviewer retrace why the design binds pending calls to full invocation identity and how to validate the change.
WhenToUse: When implementing, reviewing, resuming, or testing PINOCCHIO-TOOLCALL-1.
---



# Diary

## Goal

Record the evidence and decisions behind the Pinocchio frontend-tool bridge hardening guide, including current behavior, cross-repo contract, failures, validation, and delivery.

## Step 1: Map the bridge and design session-bound idempotent completion

I traced the server half from Geppetto's provider-visible tool call through `BridgeExecutor`, `Manager.Request`, sessionstream publication, `HandleResult`, timeline projection, and return to the parked model run. The design document is intentionally self-contained for an intern: it explains package ownership before proposing new types or tests.

The critical design input is that `Manager.pending` is global and keyed only by tool-call id. A previously executed probe parked a victim-session request and resolved it with an attacker-session result carrying a different tool name. The guide therefore treats session/tool identity as the first containment phase, not as optional defense in depth.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket for each repo and write a detailed design doc for each.\n\nFor each repo:\nCreate  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.\n\n[REMINDER] Output a <summary>...</summary> block at the VERY END of your response. This is mandatory."

**Assistant interpretation:** Create a new Pinocchio ticket with a standalone technical guide for the frontendtools fixes, validate it, and deliver it to reMarkable.

**Inferred user intent:** Hand implementation of the critical bridge fix to a new engineer without requiring them to reconstruct PBUI's prior review session.

### What I did

- Created ticket `PINOCCHIO-TOOLCALL-1` and tasks for writing, validation, and delivery.
- Read `AGENTS.md`, the frontend-tool protobuf, `manager.go`, `bridge.go`, `plugin.go`, and focused tests.
- Mapped current manifest, provider alias, request, result, cancellation, and timeline behavior.
- Wrote the design guide with architecture/sequence/state diagrams, API sketches, locking rules, phased implementation, cross-repo rollout, and a test matrix.

### Why

- A bare provider call id is neither globally unique nor authorization.
- The patch must handle benign cross-session collision as well as malicious result submission.
- Interns need to understand sessionstream and Geppetto ownership before editing a small but security-critical map.

### What worked

- Existing package boundaries make the containment patch local to `pkg/chatapp/frontendtools`.
- Session id is already present on `sessionstream.Command`, so `(session,id)` containment can land before protocol v2.
- Existing alias collision and projection tests provide good patterns for the new table-driven cases.

### What didn't work

N/A during document authoring. No implementation code was changed.

### What I learned

- Old deferred cleanup must compare the exact pending record, not only delete by key; otherwise accidental key reuse can still delete a newer call.
- Durable publication and channel wake-up cannot happen while holding the global manager mutex.
- The terminal ledger must be bounded and distinguish identical retries from conflicting terminal results.

### What was tricky to build

The target design had to separate immediate containment from full protocol identity. Requiring every new field in the first patch would delay the Critical fix; stopping permanently at `(session,id)` would leave run/tool/manifest/executor ambiguity. The guide therefore defines explicit phases and a coordinated v2 protocol rather than a hidden compatibility shim.

### What warrants a second pair of eyes

- The exact ordering of terminal state transition, timeline publication, and parked-channel wake-up.
- Retention limits and sensitive-data policy for terminal result caching.
- Whether executor-targeted delivery belongs in Pinocchio/client filtering or sessionstream transport.

### What should be done in the future

- Implement Phase 0 composite identity and regression tests first.
- Coordinate protocol v2 fields with react-chat and PBUI.
- Run the race detector under concurrent request/result/cancel/manifest load.

### Code review instructions

- Start at `pkg/chatapp/frontendtools/manager.go:25-184`.
- Compare every new key lookup and cleanup to the `InvocationKey` invariants in the guide.
- Run `go test -race ./pkg/chatapp/frontendtools -count=20` before broader chatapp validation.

### Technical details

The minimum safe map shape is:

```go
map[pendingKey]*pendingCall

type pendingKey struct {
    SessionID  sessionstream.SessionId
    ToolCallID string
}
```

The target identity additionally binds message/run, tool name, executor, and manifest generation.

## Step 2: Validate the guide and current package baseline

The finished guide is 737 lines and 3,782 words. Frontmatter and `docmgr doctor` pass, both Mermaid diagrams render, and focused normal/race tests establish a clean current baseline before any implementation change.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Prove the ticket artifact is structurally valid, renderable, and grounded against a passing package baseline.

**Inferred user intent:** Receive an implementation-ready guide rather than an unvalidated draft.

### What I did

- Ran frontmatter validation and `docmgr doctor --ticket PINOCCHIO-TOOLCALL-1`.
- Ran `go test ./pkg/chatapp/frontendtools -count=1`.
- Ran `go test -race ./pkg/chatapp/frontendtools -count=1`.
- Rendered both Mermaid blocks with Mermaid CLI and a no-sandbox Puppeteer config.
- Related manager, bridge, plugin, proto, and focused tests to the design doc.

### Why

- Race behavior is central to the pending-map design.
- Mermaid parse success is required before PDF delivery.
- Relations make the design discoverable from the code it changes.

### What worked

```text
ok github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools
ok github.com/go-go-golems/pinocchio/pkg/chatapp/frontendtools (race)
Doctor: all checks passed
Mermaid: 2/2 PASS
```

### What didn't work

The first renderer command used `mmdc` without an active Node-version PATH:

```text
/bin/bash: mmdc: command not found
```

The rerun used `/home/manuel/.nvm/versions/node/v22.22.1/bin/mmdc`; both diagrams passed.

### What I learned

- The current test baseline passes but lacks the cross-session/collision matrix described in the guide.
- Repository validation does not require modifying production code to establish design evidence.

### What was tricky to build

The renderer needs Chromium `--no-sandbox` in this environment. The corrected command supplied a temporary Puppeteer config without changing document content or weakening syntax validation.

### What warrants a second pair of eyes

- Confirm the full chatapp suite remains green once implementation begins; this validation intentionally targets the affected package.

### What should be done in the future

- Implement the guide phases; no production implementation is part of this documentation ticket step.

### Code review instructions

- Read the design from system orientation through Phase 0 before reviewing pseudocode in isolation.
- Re-run doctor, focused tests, race tests, and Mermaid rendering after edits.

### Technical details

Validation commands are recorded above; the rendering evidence is `various/01-mermaid-render.txt`.

## Step 3: Deliver the guide to a canonical reMarkable path

The initial upload returned success but could not be verified by exact path because parallel rmapi directory creation produced duplicate `23` collections. I repeated dry-run/upload sequentially under the unique `23-deliveries` root and verified the exact guide listing.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Deliver the Pinocchio guide to reMarkable with evidence that the path actually resolves.

**Inferred user intent:** Provide a dependable offline implementation handoff, not merely an upload success line.

### What I did

- Performed initial and canonical dry runs.
- Uploaded the guide sequentially to the canonical unique parent.
- Verified the exact remote listing.
- Added `various/02-remarkable-delivery.md`.

### Why

- The first exact-path listing returned `no matches` despite upload success.
- Duplicate named remote collections cannot be safely resolved or deleted by name.

### What worked

```text
OK: uploaded PINOCCHIO-TOOLCALL-1 Frontend Tool Bridge Guide.pdf -> /ai/2026/08/23-deliveries/PINOCCHIO-TOOLCALL-1
[f] PINOCCHIO-TOOLCALL-1 Frontend Tool Bridge Guide
```

### What didn't work

The first verification failed exactly with:

```text
Error: no matches for 'PINOCCHIO-TOOLCALL-1'
```

Parent listing showed three `[d] 23` entries after warnings `remote tree has changed, refresh the file tree`.

### What I learned

- Serialize cloud directory creation and use unique parents for batch delivery.
- Treat remote listing as a required acceptance check.

### What was tricky to build

The uploader's duplicate detection found the inaccessible first copy and skipped re-upload, while path lookup selected another same-named parent. Changing to an unambiguous parent resolved both behaviors without deleting data.

### What warrants a second pair of eyes

- Optional duplicate collection cleanup should use structured listing IDs and deliberate operator confirmation.

### What should be done in the future

- Read/deliver from the canonical `23-deliveries` path.

### Code review instructions

- Inspect both rendered diagrams and phase/test tables on device.

### Technical details

Canonical path: `/ai/2026/08/23-deliveries/PINOCCHIO-TOOLCALL-1`.

## Step 4: Land session-bound pending identity and strict result containment

I implemented the Critical containment phase as a focused server change. Pending calls are now keyed by `(session_id, tool_call_id)`, duplicate insertion cannot overwrite the first waiter, cleanup compares the exact record pointer, and result handling rejects wrong-session, wrong-tool, unknown, and invalid-status commands before publishing normal timeline events.

The web-chat HTTP boundary now maps stable invocation error codes to 400/404/409 rather than treating every rejection as an internal failure. This interval deliberately does not claim terminal idempotency yet; simultaneous same-key results still require the next phase's atomic pending-to-terminal transition.

### Prompt Context

**User prompt (verbatim):** "Work on PINOCCHIO-TOOLCALL-1, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)\n\n[REMINDER] Output a <summary>...</summary> block at the VERY END of your response. This is mandatory."

**Assistant interpretation:** Implement the Pinocchio-owned phases in focused commits, preserve a continuation-quality diary, and validate each interval.

**Inferred user intent:** Move the security design into reviewed production behavior without losing rationale, failures, or cross-repository sequencing.

**Commit (code):** `8cdc9af1d3597c054566cf0f3a7d50846fba1b8d` — "fix(frontendtools): bind pending calls to sessions"

### What I did

- Replaced `map[string]*pendingCall` with `map[pendingKey]*pendingCall`.
- Added stable `InvocationErrorCode`/`InvocationError` classification.
- Cloned command payloads before canonicalization instead of mutating caller-owned protobufs.
- Validated the five accepted statuses and used trusted pending identity for result events.
- Rejected unknown results instead of projecting them into the conversation timeline.
- Added deterministic tests for same-id/different-session independence, cross-session rejection, wrong tool, invalid status, duplicate insertion, and unknown results.
- Changed web-chat result errors to 400/404/409 and replaced the unsolicited-result projection test.

### Why

- The reproduced attacker-session result could previously wake a victim-session waiter.
- Silent map overwrite and bare-key deferred deletion made benign collisions unsafe.
- Rejected payloads must not look like accepted conversation history.

### What worked

- Focused normal and race tests passed.
- The successful pre-commit rerun passed repository lint, build generation, and `go test ./...`.
- The cross-session test proves the victim remains pending after rejection and accepts its owner's later result.

### What didn't work

The first commit attempt was blocked by the repository-wide test hook:

```text
--- FAIL: TestFrontendToolResultEndpointPublishesTimelineEntity (0.01s)
    server_test.go:141:
        Error:       Not equal:
                     expected: 200
                     actual  : 500
make: *** [Makefile:85: test] Error 1
```

That test intentionally submitted an unsolicited result and expected a timeline entity. I replaced it with `TestFrontendToolResultEndpointRejectsUnsolicitedResult`, added stable HTTP error mapping, reran focused tests, then reran the complete hook successfully.

### What I learned

- The old insecure behavior had become an integration-test contract, so containment requires an explicit behavior-change test rather than only manager unit tests.
- Session mismatch can be diagnosed without exposing another session's tool details by scanning only pending keys for the same call id.
- The pre-commit hook runs generation, frontend build, lint/vet, and the full Go test suite; this is useful but makes focused validation before commit important.

### What was tricky to build

The manager must read immutable pending metadata after unlocking while ensuring old cleanup cannot delete replacement state. Pending records are immutable after insertion, and `removePending` deletes only when `m.pending[key] == pending`. The next phase must atomically remove pending and create terminal state before unlocking; otherwise concurrent results can both publish.

### What warrants a second pair of eyes

- Verify that 404 for unknown results and 409 for identity conflicts are the desired public web-chat semantics.
- Review the O(n) wrong-session diagnostic scan; it is only on rejected unknown keys, but high pending volume may justify a secondary index later.
- Confirm blank tool names should remain accepted at the internal manager boundary; HTTP already requires a tool name.

### What should be done in the future

- Add bounded terminal storage and an atomic pending-to-terminal transition.
- Make cancellation/timeout terminal and reject late browser completions deterministically.
- Coordinate protocol-v2 run/manifest/executor identity before implementing those fields.

### Code review instructions

- Start at `manager.go` types and `HandleResult`, then inspect `Request`/`removePending`.
- Read the cross-session and duplicate tests before reviewing HTTP mapping.
- Validate with `go test -race ./pkg/chatapp/frontendtools -count=1` and `go test ./cmd/web-chat/internal/appserver -count=1`.

### Technical details

```go
type pendingKey struct {
    sessionID  sessionstream.SessionId
    toolCallID string
}

if current := m.pending[pending.key]; current == pending {
    delete(m.pending, pending.key)
}
```

## Step 5: Make completion terminal, bounded, and idempotent

I added the second server-only safety phase. A valid result now atomically moves one invocation from pending state into a bounded terminal store before any publisher or channel operation. Exact retries compare deterministic digests and acknowledge without publishing again; conflicting retries, retained-key reuse, and late results after server cancellation receive distinct stable errors.

Cancellation and deadline expiry now create trusted `cancelled` or `timeout` timeline results. A timeline publication failure after result acceptance completes the parked run with an explicit failed result, while the accepted terminal record prevents a second result from winning.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Continue the implementation in a second coherent interval and record both concurrency semantics and validation evidence.

**Inferred user intent:** Eliminate duplicate/replay completion and cancellation races without mixing in the coordinated protocol-v2 migration.

**Commit (code):** `84c6e63ac4a83c135591bdd890108230e4a7a62e` — "fix(frontendtools): make completion terminal and idempotent"

### What I did

- Added configurable positive terminal count/TTL through `ManagerConfig` and `NewManagerWithConfig`.
- Implemented `boundedTerminalStore` with insertion-order count eviction and age eviction; retry reads do not refresh retention.
- Added deterministic SHA-256 result digests over length-prefixed identity/status/error fields and deterministic protobuf result bytes.
- Made pending-to-terminal transition atomic under `Manager.mu`, then released the lock before publication/channel delivery.
- Added exact retry acknowledgement and `terminal_conflict`, `late_result`, and `key_reuse` errors.
- Terminalized context cancellation/deadline expiry and published trusted terminal events with `context.WithoutCancel`.
- Added deterministic concurrent-delivery tests using channel barriers, plus retention, publication-failure, cancellation, cleanup, and HTTP mapping tests.

### Why

- Phase 0 still allowed two simultaneous valid results to publish before the request's deferred cleanup.
- Network/WebSocket retries need a successful idempotent acknowledgement rather than duplicate effects or false unknown-result errors.
- A cancelled waiter must not leave a key immediately reusable by a late browser result.

### What worked

```text
go test ./pkg/chatapp/frontendtools ./cmd/web-chat/internal/appserver -count=1  # PASS
go test -race ./pkg/chatapp/frontendtools -count=20                           # PASS
pre-commit lint/build/vet + go test ./...                                     # PASS on retry
```

The blocked-publisher test proves the terminal transition happens before external I/O: an identical concurrent retry returns successfully, a conflicting retry is rejected, and only the first path publishes/completes.

### What didn't work

The first commit attempt failed because another repository was already running golangci-lint:

```text
Error: parallel golangci-lint is running
The command is terminated due to an error: parallel golangci-lint is running
make: *** [Makefile:72: lintmax] Error 3
```

The concurrent process was:

```text
/home/manuel/workspaces/2026-07-13/rag-eval-ttc/rag-evaluation-system/.bin/golangci-lint run -v ...
```

I did not terminate it. It exited after observation; the unchanged staged commit then passed the complete hook. The first attempt's `go test ./...` portion had already passed.

### What I learned

- golangci-lint's default process lock is machine-wide enough to block independent repository hooks.
- Terminal reads must not refresh insertion order or attackers/retry storms can pin records indefinitely.
- Marking terminal before publication gives one authoritative winner; publication failure must still wake the model waiter deterministically.

### What was tricky to build

Result hashing cannot hold the global manager lock across expensive protobuf serialization. The implementation deterministically serializes the result before locking, then hashes the small canonical identity fields after the trusted tool name is known. The lock covers only terminal lookup, identity validation, digest comparison, and the pending-to-terminal state move.

Cancellation races require compare-and-set against the exact pending pointer. If result completion already removed pending, cancellation does not replace its terminal state. If cancellation wins, a later browser result sees a context-origin terminal record and receives `late_result`.

### What warrants a second pair of eyes

- Confirm the default 4,096-entry/15-minute terminal retention against deployment retry windows and memory budgets.
- Review the choice that an idempotent retry after first-attempt timeline publication failure receives success; the parked run receives a failed completion and no second publication is attempted.
- Result-size limits are still enforced only by hosting HTTP infrastructure; manager-level payload bounds remain an operations hardening item.

### What should be done in the future

- Add protocol-v2 message/run/manifest/executor/capability identity in a coordinated cross-repository release.
- Add metrics for terminal cache size, eviction, retries, and rejection reasons.
- Decide whether durable recovery should retry failed timeline publication out of band.

### Code review instructions

- Review `terminal_store.go` before the new branches in `HandleResult`.
- Follow one successful result, one identical retry, one cancellation, and one publication failure through the tests.
- Run `go test -race ./pkg/chatapp/frontendtools -count=20`.

### Technical details

The authoritative ordering is:

```text
validate -> lock -> pending-to-terminal -> unlock -> publish -> wake once
```

Terminal retention is bounded by both count and age and does not store arbitrary result payloads—only trusted identity metadata and a SHA-256 digest.
