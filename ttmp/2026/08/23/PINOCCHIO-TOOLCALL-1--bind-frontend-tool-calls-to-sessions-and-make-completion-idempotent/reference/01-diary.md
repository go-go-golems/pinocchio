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
    - Path: repo://Makefile
      Note: Reproducible generated TypeScript formatting (commit 7279126)
    - Path: repo://cmd/web-chat/internal/appserver/routes_frontend_tools.go
      Note: |-
        Stable HTTP mapping for invocation rejection codes (commit 8cdc9af)
        Strict HTTP acknowledgement and result boundary (commit 7279126)
    - Path: repo://cmd/web-chat/internal/appserver/server_test.go
      Note: |-
        Unsolicited result rejection contract (commit 8cdc9af)
        HTTP regressions and race-safe runtime observation (commit 7279126)
    - Path: repo://cmd/web-chat/web/src/features/web-chat/WebChatApp/ProviderToolCallRenderer.tsx
      Note: Single actionable frontend-tool routing boundary (04b5479)
    - Path: repo://cmd/web-chat/web/src/features/web-chat/cards/ToolCallCard/ToolCallCard.test.tsx
      Note: Cancelled and timeout card rendering regressions (commit c9e8255)
    - Path: repo://cmd/web-chat/web/src/features/web-chat/cards/ToolCallCard/ToolCallCard.tsx
      Note: |-
        Terminal cards suppress stale human controls (commit c9e8255)
        Strict assigned human controls (b056b6a)
        Generic timeline projection is now read-only (04b5479)
    - Path: repo://cmd/web-chat/web/src/features/web-chat/extensions/pinocchio-timeline-adapters/pinocchioTimelineAdapters.ts
      Note: Live and hydrated executor provenance hotfix (b056b6a)
    - Path: repo://cmd/web-chat/web/src/ws/frontendTools.test.ts
      Note: Accepted terminal-status contract tests (commit c9e8255)
    - Path: repo://cmd/web-chat/web/src/ws/frontendTools.ts
      Note: Canonical terminal frontend-tool status predicate (commit c9e8255)
    - Path: repo://pkg/chatapp/frontendtools/bridge_test.go
      Note: All non-success terminal statuses map to model-visible errors (commit d1e1741)
    - Path: repo://pkg/chatapp/frontendtools/executor_test.go
      Note: Assignment state-machine and race regressions (commit 7279126)
    - Path: repo://pkg/chatapp/frontendtools/manager.go
      Note: |-
        Session-bound pending identity and strict result validation (commit 8cdc9af)
        Bounded detached cancellation publication context (commit c9e8255)
        Atomic assignment pending and terminal ownership (commit 7279126)
    - Path: repo://pkg/chatapp/frontendtools/manager_test.go
      Note: |-
        Cross-session, duplicate, mismatch, and unknown-result regressions (commit 8cdc9af)
        Blocking-publisher cancellation deadline regression (commit c9e8255)
    - Path: repo://pkg/chatapp/frontendtools/plugin.go
      Note: Durable executor provenance (commit 7279126)
    - Path: repo://pkg/chatapp/frontendtools/terminal_store.go
      Note: Bounded count/age terminal retention (commit 84c6e63)
    - Path: repo://pkg/chatapp/frontendtools/terminal_store_test.go
      Note: Deterministic count/TTL eviction tests (commit 84c6e63)
    - Path: repo://proto/pinocchio/chatapp/frontendtools/v1/frontend_tool.proto
      Note: Executor identity wire contract (commit 7279126)
    - Path: ws://react-chat/ttmp/2026/08/23/REACT-CHAT-TOOL-RUNTIME-1--make-browser-tool-execution-idempotent-single-owner-and-manifest-safe/design-doc/02-concise-frontend-tool-executor-ownership-protocol.md
      Note: Cross-repository design adopted in Diary Step 8
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

## Step 6: Audit bridge behavior and complete the server-only validation gate

I performed a final pass against the design's Phase 0/1 test matrix, removed terminal fields that were stored but never read, and added a table-driven bridge test for every accepted non-success status. Focused, 20-iteration race, chatapp-wide, build, lint/vet, frontend generation, and repository-wide tests all pass.

The design now has an implementation-status section that names the landed commits, actual error codes, retention defaults, and the explicit Phase 2–4 boundary. The ticket remains active because protocol-v2 and executor ownership require coordinated PBUI/react-chat changes.

### Prompt Context

**User prompt (verbatim):** (same as Step 4)

**Assistant interpretation:** Close out the safe Pinocchio-only implementation slice with broad validation and accurate continuation docs.

**Inferred user intent:** Leave reviewable commits and a clear account of what is fixed versus what still depends on other repositories.

**Commit (code):** `d1e1741d90210701c357415c51db7b92cb455f6d` — "test(frontendtools): cover terminal bridge statuses"

### What I did

- Removed unused `messageID` and `status` fields from terminal records.
- Added bridge tests for `failed`, `cancelled`, `denied`, and `timeout` model-error mapping.
- Re-ran the 20-iteration race suite after the cleanup.
- Ran `make build` and `go test ./pkg/chatapp/... -count=1` explicitly.
- Added implementation status and residual scope to the design guide.

### Why

- The ticket forbids dead code/data and requires an intern to distinguish implemented containment from future protocol work.
- Manager-level status acceptance is insufficient unless the bridge consistently translates every non-success terminal result for Geppetto.

### What worked

```text
go test ./pkg/chatapp/frontendtools -count=1             # PASS
go test -race ./pkg/chatapp/frontendtools -count=20      # PASS
go test ./pkg/chatapp/... -count=1                       # PASS
make build                                                # PASS
pre-commit: generation + frontend build + lint/vet + ./... # PASS
```

### What didn't work

N/A in this interval. The Vite build continued to emit its pre-existing non-fatal warning that `app-config.js` cannot be bundled without `type="module"`; build output completed successfully.

### What I learned

- The default bridge already had correct generic non-success behavior, but the contract was untested outside one manager-level denied result.
- The server-only slice is independently releasable; message/run/manifest/executor identity is not, because current consumers cannot send those fields.

### What was tricky to build

The validation boundary had to avoid claiming the full ticket design was implemented. `(session, call)` plus bounded retention closes the reproduced cross-session defect and duplicate-completion races, but after terminal eviction v1 still cannot distinguish a very late same-session reused call id from a new invocation. Protocol-v2 identity/capability is the required durable solution.

### What warrants a second pair of eyes

- Review the residual same-session/id reuse window after terminal eviction.
- Confirm the protocol-v2 rollout order before adding protobuf fields.
- Decide whether terminal retention configuration belongs in web-chat/PBUI command-line configuration or can remain at defaults initially.

### What should be done in the future

- Implement task `7xay` only with matching chat-provider/PBUI protocol changes.
- Implement client-scoped manifests/executor leases under task `bu4j` after protocol identity exists.
- Add operations metrics and result-size policy in the later hardening phase.

### Code review instructions

- Review commits in order: `8cdc9af`, `84c6e63`, then `d1e1741`.
- Start with the manager regression names, then follow the corresponding state transitions in `manager.go`.
- Re-run the five validation commands above; inspect `git status --short` afterward for generated noise.

### Technical details

Implemented HTTP outcomes are 400 for invalid status, 404 for unknown result, 410 for late result, 409 for identity/terminal/key conflicts, and 500 for operational publication failure.

## Step 7: Bound cancellation publication and close terminal UI states

PR 207 review identified two integration gaps in cancellation handling. A detached cancellation-event publication could wait indefinitely after the request context ended, and the web card did not recognize the server's new `cancelled` and `timeout` states as terminal.

I bounded detached terminal publication to five seconds and added a context-aware blocking-publisher regression. I also centralized the browser terminal-status predicate, added `timeout` to the shared result type, and directly verified that cancelled/timed-out confirmation cards render as done without Approve or Deny controls.

### Prompt Context

**User prompt (verbatim):** "Address code review issues: https://github.com/go-go-golems/pinocchio/pull/207

[REMINDER] Output a <summary>...</summary> block at the VERY END of your response. This is mandatory."

**Assistant interpretation:** Inspect every PR 207 review thread, implement the actionable backend and frontend fixes, test them at the package and repository levels, and update the ticket record.

**Inferred user intent:** Make PR 207 safe to merge by resolving the review findings with executable regression coverage rather than comment-only responses.

**Commit (code):** `c9e8255aa8bf58d881b25a88e1f4e3e94ec6a8aa` — "fix(frontendtools): bound cancellation publication"

### What I did

- Queried PR reviews and inline comments through `gh api` and found two P2 findings.
- Added a five-second detached-context deadline around cancellation/timeout timeline publication.
- Added a publisher that blocks until `ctx.Done()` and verified a cancelled request still returns while retaining terminal late-result rejection.
- Added a canonical TypeScript predicate for all five accepted terminal statuses.
- Updated `ToolCallCard` to hide human controls for `cancelled` and `timeout`.
- Added predicate and rendered-card regression tests.
- Ran focused race tests, all frontend tests/checks, and the full pre-commit pipeline.

### Why

- Cancellation must bound the request's remaining work; detaching cancellation without adding a new deadline defeats that invariant.
- A backend terminal transition and its UI completion predicate are one cross-layer contract. Leaving approval controls active invites guaranteed HTTP 410 responses.

### What worked

```text
go test ./pkg/chatapp/frontendtools -count=1          # PASS
go test -race ./pkg/chatapp/frontendtools -count=20   # PASS
frontend npm test                                      # 36 PASS
frontend typecheck + Biome lint                        # PASS
pre-commit generation/build/lint/vet/go test ./...     # PASS
```

### What didn't work

`gh pr view 207 --comments` failed because GitHub's GraphQL response still references deprecated classic project cards:

```text
GraphQL: Projects (classic) is being deprecated in favor of the new Projects experience
```

I recovered by querying `/pulls/207/comments` and `/pulls/207/reviews` through `gh api --paginate`, which returned both inline findings and the parent review without using the failing field.

### What I learned

- `context.WithoutCancel` is appropriate for best-effort terminal publication only when immediately wrapped in a fresh bounded context.
- Pinocchio's status union had already included `cancelled` but omitted `timeout`, while the renderer independently hard-coded only three terminal states; one shared predicate prevents another drift.
- Static React rendering is sufficient to verify that dead approval controls are absent without adding a browser testing dependency.

### What was tricky to build

Terminalization deliberately mutates pending state before external I/O, so a publication timeout must not roll the invocation back. The regression therefore verifies both escape behavior and the durable semantic outcome: `Request` returns `context.Canceled`, publication observes `context.DeadlineExceeded`, and a subsequent browser success remains a `late_result`.

The test publisher also needed synchronization around its observed error so the 20-iteration race suite could validate the assertion without introducing a test-only data race.

### What warrants a second pair of eyes

- Confirm five seconds is an appropriate operational bound for the current sessionstream storage/fanout path.
- Confirm terminal publication failures should remain returned as wrapped cancellation errors rather than moving to asynchronous observability.
- Review whether other timeline renderers duplicate terminal-status lists outside this card.

### What should be done in the future

- Add publication timeout/error metrics when the control-plane observability phase lands.
- Carry terminal status as a generated protocol enum in protocol v2 instead of maintaining a TypeScript string union.

### Code review instructions

- Start at `Manager.terminalizeContext` and `TestManagerContextCancellationPublicationIsBounded`.
- Then review `isTerminalFrontendToolResultStatus` and the static-render card test.
- Re-run the five validation commands listed above.

### Technical details

The bounded context preserves request values with `context.WithoutCancel(ctx)` but replaces cancellation and the expired request deadline using `context.WithTimeout(..., 5*time.Second)`. Terminal state continues to win before publication, preserving the existing idempotency contract.

## Step 8: Narrow executor ownership to the concise assignment tuple

This design interval updated Pinocchio's implementation guide to point at the new authoritative cross-repository protocol. The immediate server work is no longer coupled to timed leases or every aspirational protocol-v2 field: Pinocchio will assign and bind a client, connection, and assignment epoch through manifest acceptance, pending calls, results, terminal digests, and timeline entities.

No Go implementation changed in this step. The value is a precise boundary for the next implementation phase and an explicit statement that in-flight calls are never silently reassigned.

### Prompt Context

**User prompt (verbatim):** (same as react-chat Diary Step 10)

**Assistant interpretation:** Update the server companion guide so Pinocchio implementers follow the concise accepted contract rather than the earlier full lease proposal.

**Inferred user intent:** Keep all three repository plans coherent and immediately implementable.

**Commit (docs):** `042c2fdef5ee5beb131dac81a38caaafe441b1eb` — "PINOCCHIO-TOOLCALL-1: adopt concise executor protocol"

### What I did

- Reviewed `frontend_tool.proto`, `manager.go`, and `plugin.go` against the new contract.
- Updated the Pinocchio guide's status and Phase 2 heading/scope.
- Assigned atomic manifest acknowledgement, pending executor capture, strict result comparison, terminal digesting, and projection ownership to Pinocchio.

### Why

- Pinocchio is the only layer that can select one server-visible executor and bind it to pending completion state.

### What worked

- Existing manager locking, bounded terminal state, context terminalization, and Hub publisher APIs provide suitable extension points.

### What didn't work

- The old guide's full run/manifest/capability/lease target was too broad for the requested first release. It is retained as future context but superseded for immediate executor work.

### What I learned

- PBUI can call a new acknowledgement-returning manager operation with its Hub as publisher, avoiding a submit-then-query race.

### What was tricky to build

Manifest publication failure needs compare-before-rollback so a failed older publish cannot restore state over a newer accepted assignment.

### What warrants a second pair of eyes

- Review stable error codes and HTTP mapping for missing/mismatched executor identity.
- Review same-revision semantic manifest comparison and deterministic encoding.

### What should be done in the future

- Implement the concise schema/manager/projection changes and race matrix before publishing the next Pinocchio version.

### Code review instructions

- Read react-chat design 02, then trace Pinocchio `HandleManifest`, `Request`, `HandleResult`, `terminalizeContext`, and plugin projection.

### Technical details

The accepted tuple is `(client_instance_id, connection_id, assignment_id)`. Timed expiry and automatic pending-call takeover are deliberately absent.

## Step 9: Implement server-owned executor assignment

This phase implemented the concise executor tuple at Pinocchio's authoritative boundary. Client-scoped manifests now receive a server-generated assignment; requests capture it immutably; results, retries, cancellation, terminal digests, live events, and durable entities all carry and validate the same tuple. A changed connection owns future calls only, while already-pending calls retain their original executor.

The built-in web-chat HTTP adapter now returns the exact manifest acknowledgement through one manager operation instead of submitting and then querying mutable state. The phase also made generated TypeScript formatting part of deterministic protocol generation and repaired a pre-existing test-only race exposed by the required race run.

### Prompt Context

**User prompt (verbatim):** `go ahead, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill). Print out a brutalist work slip with the plan / different phases for the ticket. then before stsarting a phase, plrint a split about the phase, and print one when the phase is done.`

**Assistant interpretation:** Implement the concise executor protocol across repositories in gated phases, print an overall plan plus start/completion slips, commit coherent intervals, and record exact implementation/validation evidence in the ticket diaries.

**Inferred user intent:** Ship the feature rather than leave it as design, while preserving reviewability and refusing to claim a phase complete until its protocol, tests, generated artifacts, and release boundary are verified.

**Commit (code):** `7279126741694247c2e3a78e5e80a6be6dc994b8` — "feat(frontendtools): assign one browser executor"

### What I did

- Added generated `FrontendToolExecutor` protobuf identity and fields on manifest, request, result, event, and timeline entity messages.
- Implemented serialized `Manager.AcceptManifest` with strict identity bounds, monotonic per-connection revisions, idempotent same-revision equality, server UUID assignments, assignment retention/replacement, exact acknowledgement, and publication rollback.
- Replaced bare session manifests with immutable assigned manifests.
- Captured executor identity in pending and terminal calls and included it in deterministic result digests.
- Rejected missing/mismatched result executors with stable error codes.
- Preserved executor provenance through context cancellation and plugin projection.
- Updated bridge lookups and all existing frontend-tool tests for strict manifests/results.
- Added `executor_test.go` covering assignment transitions, revision conflict/regression, rollback, in-flight ownership immutability, mismatch rejection, and concurrent acceptance.
- Updated Pinocchio's web-chat HTTP adapter to use exact acknowledgement and strict result identity.
- Made generated TypeScript Biome formatting part of `proto-gen-core`.
- Replaced unsynchronized test observation pointers with a channel, making the appserver race suite clean.

### Why

- Browser filtering is only trustworthy for honest concurrency when the server first creates a unique assignment and binds pending completion to it.
- A manifest POST followed by a separate state query can acknowledge the wrong racing tab.
- Executor provenance must survive snapshot hydration or a reload can turn an old request into an unassigned executable call.

### What worked

- `make proto-gen-core` produced identical diff hashes on consecutive runs: `b2b899224ded92b81e5772bb340ad9bbcee7630816e577fe2ad2a0f63e48a8c4`.
- Focused frontend-tools tests and race tests passed.
- Full uncached `GOWORK=off go test ./... -count=1` passed.
- `GOWORK=off make build` passed.
- The successful pre-commit gate passed Go generation/build, golangci-lint, custom vet analyzers, full tests, web typecheck, and web lint.

### What didn't work

1. The first focused compile failed exactly with:

   ```text
   pkg/chatapp/frontendtools/bridge.go:167:32: manifest.Tools undefined (type *assignedManifest has no field or method Tools)
   pkg/chatapp/frontendtools/bridge.go:239:32: manifest.Tools undefined (type *assignedManifest has no field or method Tools)
   ```

   `RegisterManifestTools` and `ResolveProviderToolName` still referenced the old map value. Both now read the immutable assigned manifest's `updated.Tools`.

2. The next focused run failed because legacy tests posted manifests/results without strict identity and direct requests lacked an available assigned manifest. Test helpers now install deterministic client/connection/assignment fixtures, and bridge tests echo the request assignment.

3. A new executor test used `require.Equal` on protobuf messages. Protobuf runtime message-state initialization made semantically equal messages appear structurally different with a huge internal diff. The test now uses `proto.Equal`.

4. The first full race command exposed a pre-existing appserver test race between an inference goroutine writing `**turns.Turn` and the test polling/reading it. The exact race began at `runtimeBackedTestEngine.RunInference` (`server_test.go:40`) versus `TestSubmitAndSnapshot_WiresSessionIDAndTurnStoreIntoRuntime` (`server_test.go:374`). Replacing the shared pointers/poll loop with one buffered `runtimeObservation` channel made the rerun pass.

5. The first commit gate failed web lint because regenerated Buf TypeScript imports were not Biome-sorted:

   ```text
   The imports and exports are not sorted.
   Found 4 errors.
   make: *** [Makefile:95: web-lint] Error 1
   ```

   Adding a pinned Biome formatting step to `proto-gen-core` made generation reproducible and the second complete commit gate passed.

### What I learned

- Assignment event publication and HTTP acknowledgement need one serialized state transition; otherwise event order and returned ownership can diverge.
- Same-connection manifest updates should retain assignment, while a different connection always starts a new epoch.
- Generated-source reproducibility includes repository formatting requirements, not merely raw generator stability.
- Required race validation can expose unrelated test harness defects even when production synchronization is correct.

### What was tricky to build

Manifest acceptance combines ordering, idempotency, rollback, and external publication. `manifestMu` serializes acceptance through publication; `Manager.mu` protects state; a failed publication restores the previous pointer only if its candidate is still current. Pending calls clone the assignment under the manager lock before publishing. Later ownership changes cannot mutate that clone, and terminal retries compare executor before digest/idempotency handling.

The reconnect semantics also required distinguishing current assignment from pending assignment. `Manager.Request` captures current ownership, but `HandleResult` validates against pending/terminal ownership rather than the manager's latest manifest, so an old owner can legitimately finish its old call after a new connection owns future calls.

### What warrants a second pair of eyes

- Review `AcceptManifest` lock ordering, publication rollback, and global manifest serialization.
- Review strict revision equality based on deterministic descriptor bytes.
- Confirm `executor_unavailable` should cover both missing assignment and unavailable tool at request time.
- Review HTTP status mappings and the explicit honest-client—not cryptographic authorization—boundary.
- Review the generated-file formatting step and unrelated generator-version churn committed with the schema.

### What should be done in the future

- Publish this Pinocchio contract before react-chat consumes it.
- Add cryptographic/channel-bound executor proof only if hostile authorized tabs become part of the threat model.
- Do not add timed reassignment of pending effects without an explicit effect-replay policy.

### Code review instructions

1. Start with `frontend_tool.proto`, then `Manager.AcceptManifest` and `Manager.Request`.
2. Trace `pending.executor` through `HandleResult`, `terminalizeContext`, terminal digest, and plugin projection.
3. Review `executor_test.go` as the state-machine specification.
4. Review appserver manifest acknowledgement and result error mapping.
5. Run:

   ```text
   make proto-gen-core
   GOWORK=off go test ./pkg/chatapp/frontendtools -count=1
   GOWORK=off go test -race ./pkg/chatapp/frontendtools ./cmd/web-chat/internal/appserver -count=1
   GOWORK=off go test ./... -count=1
   GOWORK=off make build
   ```

### Technical details

The full executor tuple is bounded to 128 bytes per opaque field. Assignment IDs use UUIDs by default and an injected generator in tests. Identical same-revision manifests publish no duplicate event. Different connections may start at any revision because revisions are connection-scoped. Result digest order is client, connection, assignment, call, tool, status, error, then deterministic result bytes.

## Step 10: Merge and publish Pinocchio v0.11.15

This phase moved the server contract from a workspace commit to an immutable downstream-consumable module release. The full local push gate and all eleven GitHub checks passed, PR 208 merged, and lightweight tag `v0.11.15` now resolves through the public Go module proxy to the authoritative merge commit.

The release preserves upstream-first ordering: react-chat can now implement against a real module version rather than a workspace replacement or unpublished branch.

### Prompt Context

**User prompt (verbatim):** (same as Step 9)

**Assistant interpretation:** Treat publication as its own gated phase and do not begin the browser runtime until the server contract is merged, tagged, and independently resolvable.

**Inferred user intent:** Keep cross-repository integration reproducible and prevent consumers from depending on local workspace state.

### What I did

- Pushed `task/add-pbui-agent` through the complete pre-push gate.
- Opened and documented PR 208.
- Waited for Buf, full tests, webchat, lint, MySQL, GoSec, govulncheck, secret scan, dependency review, CodeQL Analyze, and CodeQL checks.
- Merged PR 208 to upstream `main`.
- Tagged merge commit `806f449f0ac8a7ee9c52d459103e29ec8f4b533e` as `v0.11.15` and pushed the tag to upstream.
- Verified `GOWORK=off GOPROXY=https://proxy.golang.org go list -m -json github.com/go-go-golems/pinocchio@v0.11.15`.

### Why

- React-chat and PBUI release validation must consume the immutable server API outside `go.work`.

### What worked

- PR: `https://github.com/go-go-golems/pinocchio/pull/208` merged at `2026-08-25T21:40:16Z`.
- All eleven GitHub checks passed.
- The public Go proxy immediately resolved `v0.11.15` to merge commit `806f449`.
- The pre-push gate included GoSec with zero issues, govulncheck with no called vulnerabilities, full tests, lint, web checks, and a GoReleaser snapshot.

### What didn't work

- N/A. The branch push, review gate, merge, tag push, and proxy resolution all succeeded.

### What I learned

- The repository uses lightweight module tags on authoritative merge commits; preserving that convention avoids unnecessary tag-object differences.

### What was tricky to build

The release had to tag the upstream merge commit rather than the feature branch head. GitHub created merge commit `806f449`, while implementation/docs heads were `7279126`/`e7ed165`; tagging the branch head would have omitted the canonical merge topology used by prior releases.

### What warrants a second pair of eyes

- Confirm downstream modules select `v0.11.15` with `GOWORK=off` and do not accidentally resolve the sibling workspace checkout.

### What should be done in the future

- Consume `v0.11.15` in react-chat and PBUI.
- Keep tag `v0.11.15` immutable.

### Code review instructions

1. Inspect PR 208's merged diff and checks.
2. Run the exact proxy query above.
3. Confirm `Origin.Hash` is `806f449f0ac8a7ee9c52d459103e29ec8f4b533e` and `Ref` is `refs/tags/v0.11.15`.

### Technical details

The release timestamp reported by the Go proxy is `2026-08-25T21:40:15Z`. No compatibility fallback was introduced; manifests/results without complete executor identity are rejected by the new contract.

## Step 11: Reopen publication after late PR 208 review

Codex review arrived after PR 208 had already merged and identified two valid P1 defects. The first allowed an unpublished candidate manifest to become visible to a concurrent `Request`; a failed publication could therefore strand a pending call on an assignment the browser never acknowledged. The second made Pinocchio's built-in human approval card incompatible with strict result validation because its timeline projection and POST omitted the request executor.

Both defects are fixed and validated in commit `b056b6a`, pushed as PR 210. The PR is intentionally left open for Manuel to merge. Published `v0.11.15` is now treated as superseded-for-consumption; downstream integration must wait for a new immutable hotfix tag after PR 210 is merged.

### Prompt Context

**User prompt (verbatim):** `Address code review issues in https://github.com/go-go-golems/pinocchio/pull/208 btw.\n\nAnd do not merge by yourself, let me do that.`

**Assistant interpretation:** Address every open PR 208 review thread with tests and a follow-up PR, respond and resolve the threads, but leave the follow-up unmerged for the maintainer.

**Inferred user intent:** Preserve a human merge gate and do not let release momentum bypass correctness feedback.

**Commit (code):** `b056b6aa37c5f8d412e934549ac184e34b01d304` — "fix(frontendtools): commit assignment before exposure"

### What I did

- Queried all PR 208 review threads and read both late P1 comments.
- Changed `AcceptManifest` so candidate state is installed only after event publication succeeds; no rollback-visible window remains.
- Added a blocking-publication regression proving a concurrent request captures the prior committed assignment.
- Added a Pinocchio-specific frontend-tool timeline adapter that retains executor identity for live and hydrated cards.
- Required complete executor provenance before rendering built-in human controls.
- Forwarded executor identity through `submitFrontendToolResult`.
- Added adapter, card, and result-transport tests.
- Ran full Go tests/build, focused races, web typecheck, 16 focused tests, Biome lint, and the complete pre-commit/pre-push gates.
- Pushed PR 210 and left it unmerged.
- Replied to and resolved both PR 208 review threads with exact commit/PR evidence.

### Why

- A request must never capture assignment state whose publication/acknowledgement can still fail.
- Strict server validation must be rolled out with every first-party result producer, including Pinocchio's own human card.

### What worked

- Focused Go race checks passed.
- Web typecheck, 16 focused tests, and lint passed.
- Full uncached Go tests and `make build` passed.
- Complete push gate passed GoSec (0), govulncheck, lint, tests, web checks, and GoReleaser snapshot.
- PR 210 is open at `https://github.com/go-go-golems/pinocchio/pull/210`.

### What didn't work

- The Phase 2 completion slip and `v0.11.15` release occurred before the automated review posted at `2026-08-25T21:41:34Z`. That release contains the two reviewed defects and must not be selected by downstream consumers.
- PR 208 had already been merged before the user's no-self-merge instruction arrived. No PR was merged by the assistant after that instruction; PR 210 remains open.

### What I learned

- Publishing before installing candidate state is simpler and safer than install-then-rollback when readers do not participate in the acceptance lock.
- Strict protocol migrations require auditing every producer, not only the principal PBUI/react-chat consumer path.
- Review arrival can lag CI/merge; future release phases should include an explicit review-settling check before merge/tagging when the maintainer wants a human gate.

### What was tricky to build

The hotfix needed to preserve old assignment availability during a blocked candidate publication. `manifestMu` still serializes manifest acceptance, while requests continue reading the committed map under `Manager.mu`; because the candidate is not installed until publication returns successfully, requests safely use the old executor without blocking on external I/O.

The built-in UI used chat-provider `0.2.1`, whose generic frontend-tool adapter drops the new executor field. A higher-priority Pinocchio adapter now carries executor provenance without waiting for the future chat-provider `0.6.0` release.

### What warrants a second pair of eyes

- Review whether publishing the event immediately before installing manager state creates any observer that can synchronously initiate a server request during the tiny commit interval; ordinary browser message flow still waits for the HTTP acknowledgement.
- Review the Pinocchio-specific adapter priority relative to chat-provider's generic adapter.
- Confirm PR 210 should merge before assigning a hotfix tag.

### What should be done in the future

- Manuel merges PR 210.
- Tag the resulting upstream merge commit with a new immutable patch version; never rewrite `v0.11.15`.
- Downstream repositories consume only the hotfix version.

### Code review instructions

1. Review PR 210 and commit `b056b6a`.
2. Start with `TestRequestCannotObserveManifestBeforePublicationCommits`.
3. Trace executor from projected entity to `ToolCallCard` and `submitFrontendToolResult`.
4. Confirm both PR 208 threads have responses and are resolved.

### Technical details

The two original review comments are `discussion_r3857682073` and `discussion_r3857682087`. Replies are `discussion_r3857798761` and `discussion_r3857798935`. No new release tag was created for the hotfix because the maintainer merge is pending.

## Step 12: Remove the duplicate UI authority exposed by PR 210 review

Further review correctly showed that forwarding a broadcast request's executor tuple into a generic approval card did not establish local ownership. Every tab could copy the same visible tuple. This was not another missing-field bug; it exposed a duplicated-authority design pattern: the timeline card independently inferred actionability and posted results outside chat-provider's assignment-aware runtime.

The fix removes that authority rather than adding another local identity store. Generic timeline cards are now read-only. All automatic and human frontend interactions must pass through `ToolCallOutlet` and the single chat-provider `ToolRuntime`, which owns assignment matching, human completion CAS, cancellation, terminal retention, and retries. Hydrated session identity now comes from the adapter's snapshot context as intended.

### Prompt Context

**User prompt (verbatim):** `Address further https://github.com/go-go-golems/pinocchio/pull/210 . Is this somehow a design pattern issue, if you take a step back? Is therea better architecture that would address these issues?`

**Assistant interpretation:** Fix both new review findings, but step back from incremental field threading and remove the second browser completion authority that keeps generating ownership bugs.

**Inferred user intent:** Prefer a durable architecture with one execution state machine over repeated local patches that make every renderer responsible for protocol correctness.

**Commit (code):** `04b54790affa7ba7292503000229b22fd66da811` — "fix(web-chat): centralize frontend tool authority"

### What I did

- Removed direct result submission and heuristic approval controls from generic `ToolCallCard`.
- Removed Pinocchio web-chat's bespoke `submitFrontendToolResult` function.
- Kept timeline cards read-only even when input resembles an approval and carries an executor tuple.
- Routed all frontend modes through `ProviderToolCallRenderer -> ToolCallOutlet`; added numeric and enum mode normalization.
- Preserved executor provenance in the Pinocchio adapter for the shared runtime/outlet.
- Read hydrated session ID from `SnapshotProjectionContext.sessionId`.
- Added mode classification, read-only card, and hydrated context tests.
- Passed web typecheck, 23 focused tests, Biome lint, and the complete pre-push gate.
- Replied to and resolved both PR 210 threads; left PR 210 unmerged.
- Added the single-authority decision to the authoritative concise protocol design.

### Why

- Broadcast executor fields are provenance, not proof that the current tab owns them.
- Only the runtime has local acknowledged assignment state and can compare it before rendering/completing.
- UI projection should describe what happened; it should not independently become a command authority based on payload shape.

### What worked

- Commit `04b5479` deletes more duplicated authority code than it adds: 130 lines removed, 37 added.
- Web typecheck and lint pass.
- Four focused files / 23 tests pass.
- The complete push gate passes Go build/lint/vet, GoSec, govulncheck, full tests, web checks, and release snapshot.
- Both PR 210 review threads are resolved with architectural explanations.

### What didn't work

- The prior `b056b6a` approach only checked that executor fields were non-empty. Because all session subscribers see the same broadcast fields, every tab could still render and submit. It correctly transported provenance but incorrectly treated provenance as local authority.

### What I learned

- “Carry the field everywhere” is not sufficient when authority depends on private local state. The renderer must ask the authority-owning runtime, not reconstruct eligibility from the event.
- Timeline projection and command execution are separate responsibilities. Combining them produces confused-deputy behavior.
- Removing a legacy interaction is safer than maintaining two partially synchronized state machines during a strict protocol migration.

### What was tricky to build

Pinocchio's provider renderer already routed recognized frontend modes into `ToolCallOutlet`, but numeric enum values could miss the string-based `includes('FRONTEND')` test and fall into the generic card. Explicitly normalizing both protobuf JSON enum names and numeric values closes that accidental bypass. Unknown/unregistered tools remain visible through the outlet's read-only fallback.

### What warrants a second pair of eyes

- Confirm all frontend mode encodings are covered by `isFrontendToolMode`.
- Confirm no other card/helper posts frontend results outside `ToolRuntime`.
- Review whether Pinocchio should upgrade its embedded chat-provider only after executor-aware `0.6.0` is published.

### What should be done in the future

- Register supported built-in human tools explicitly through chat-provider rather than inferring approval UI from arbitrary input fields.
- Add an architectural guard/test that no timeline renderer imports a frontend result submission transport.

### Code review instructions

1. Review the deletion in `ToolCallCard` and `frontendTools.ts` first.
2. Review `ProviderToolCallRenderer` as the single routing boundary.
3. Verify hydrated `sessionId` comes from projection context.
4. Search for `/tools/results` and ensure only the shared runtime/client transport owns browser submission after the eventual package upgrade.

### Technical details

The new PR 210 comments are `discussion_r3857844822` and `discussion_r3857844830`; replies are `discussion_r3857898379` and `discussion_r3857898685`. Both threads are resolved. PR 210 remains open for maintainer merge.
