---
Title: Diary
Ticket: PINOCCHIO-TOOLCALL-1
Status: active
Topics: [security, chatapp, backend, runtime]
DocType: reference
Intent: long-term
Owners: [manuel]
RelatedFiles: []
ExternalSources: []
Summary: 'Chronological investigation, design, validation, and delivery record for Pinocchio frontend-tool invocation/result hardening.'
LastUpdated: 2026-08-23T17:25:00-04:00
WhatFor: 'Let an implementer or reviewer retrace why the design binds pending calls to full invocation identity and how to validate the change.'
WhenToUse: 'When implementing, reviewing, resuming, or testing PINOCCHIO-TOOLCALL-1.'
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
