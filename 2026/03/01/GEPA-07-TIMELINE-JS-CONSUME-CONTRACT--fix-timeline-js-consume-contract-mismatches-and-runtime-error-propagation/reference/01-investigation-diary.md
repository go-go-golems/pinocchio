---
Title: Investigation Diary
Ticket: GEPA-07-TIMELINE-JS-CONSUME-CONTRACT
Status: active
Topics:
    - gepa
    - pinocchio
    - sem
    - goja
    - bug
    - architecture
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: 2026/03/01/GEPA-07-TIMELINE-JS-CONSUME-CONTRACT--fix-timeline-js-consume-contract-mismatches-and-runtime-error-propagation/design-doc/01-timeline-js-consume-contract-mismatch-analysis-and-bug-fix-design.md
      Note: Primary design output produced from this diary
    - Path: pkg/webchat/timeline_js_runtime.go
      Note: Investigated reducer normalization behavior
    - Path: pkg/webchat/timeline_projector.go
      Note: Investigated handled/error gating behavior
    - Path: pkg/webchat/timeline_registry.go
      Note: Investigated runtime and built-in ordering
ExternalSources: []
Summary: Chronological log of analysis steps, commands, observations, and decisions for timeline JS consume contract mismatch investigation.
LastUpdated: 2026-03-01T07:05:30-05:00
WhatFor: Provide reproducible investigation context for future contributors and reviewers.
WhenToUse: Use when reviewing why the bug-fix design was chosen and how findings were validated.
---


# Investigation Diary

Ticket: `GEPA-07-TIMELINE-JS-CONSUME-CONTRACT`
Date: `2026-03-01`

## 1. Objective

Create a new ticket-local, intern-friendly analysis and bug-fix design document explaining three flagged issues:

1. consume-only reducer output becoming an unintended upsert,
2. runtime consume not suppressing handler-backed built-ins,
3. runtime errors dropped when `handled=false`.

## 2. Chronological Log

## Step 1: Loaded workflow skills and tool capabilities

Commands:

```bash
sed -n '1,260p' /home/manuel/.codex/skills/ticket-research-docmgr-remarkable/SKILL.md
sed -n '1,260p' /home/manuel/.codex/skills/remarkable-upload/SKILL.md
docmgr --help
```

What worked:

1. Confirmed required workflow (ticket creation, evidence gathering, doctor validation, remarkable upload with dry-run first).
2. Confirmed available `docmgr` and `remarquee` command surfaces.

## Step 2: Verified docmgr root behavior and current ticket inventory

Commands:

```bash
docmgr ticket --help
docmgr list tickets --help
docmgr --root /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio status --summary-only
docmgr list tickets --root /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio --with-glaze-output --output csv --fields ticket,title,status,path --with-headers=false
```

What worked:

1. Verified `--root` works as requested.
2. Confirmed `GEPA-03` and `GEPA-06` tickets exist and are active.

## Step 3: Created new ticket workspace and docs

Commands:

```bash
docmgr ticket create-ticket --root /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio --ticket GEPA-07-TIMELINE-JS-CONSUME-CONTRACT --title "Fix timeline JS consume contract mismatches and runtime error propagation" --topics gepa,webchat,sem,timeline,js-runtime,bugfix-design
docmgr doc add --root /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio --ticket GEPA-07-TIMELINE-JS-CONSUME-CONTRACT --doc-type design-doc --title "Timeline JS Consume Contract Mismatch Analysis and Bug Fix Design"
docmgr doc add --root /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio --ticket GEPA-07-TIMELINE-JS-CONSUME-CONTRACT --doc-type reference --title "Investigation Diary"
```

What worked:

1. New ticket and document scaffolding were created.

What was tricky:

1. Initial assumption was that output would land under `pinocchio/ttmp/...`.
2. With `--root` set to repo root, output landed under `pinocchio/2026/...`.

Failure observed:

```text
ls: cannot access '/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/ttmp/2026/03/01/...': No such file or directory
```

Resolution:

1. Located actual path with:

```bash
find /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio -maxdepth 5 -type d -name 'GEPA-07-TIMELINE-JS-CONSUME-CONTRACT--*'
```

## Step 4: Collected code and contract evidence for all three findings

Commands (selection):

```bash
sed -n '320,430p' pkg/webchat/timeline_js_runtime.go
sed -n '1,220p' pkg/webchat/timeline_registry.go
sed -n '1,240p' pkg/webchat/timeline_projector.go
sed -n '1,220p' pkg/webchat/timeline_handlers_builtin.go
nl -ba pkg/webchat/timeline_js_runtime.go | sed -n '340,390p'
nl -ba pkg/webchat/timeline_registry.go | sed -n '60,110p'
nl -ba pkg/webchat/timeline_projector.go | sed -n '104,126p'
nl -ba pkg/webchat/timeline_handlers_builtin.go | sed -n '1,40p'
```

What worked:

1. Confirmed consume-only object fallthrough into `decodeTimelineEntity`.
2. Confirmed runtime executes after handler list.
3. Confirmed runtime errors can be returned with `handled=false` and then dropped by projector gate.

## Step 5: Cross-checked GEPA-03/GEPA-06 and runtime docs

Commands (selection):

```bash
sed -n '100,210p' ttmp/2026/02/26/GEPA-06-.../design-doc/02-cross-repo-js-sem-runtime-implementation-design.md
sed -n '330,430p' ttmp/2026/02/26/GEPA-06-.../design-doc/02-cross-repo-js-sem-runtime-implementation-design.md
nl -ba pkg/doc/topics/13-js-api-reference.md | sed -n '116,146p'
```

What worked:

1. Verified written contract for `consume` semantics and normalization matrix.
2. Verified mismatch is against explicit documented behavior, not a subjective interpretation.

## Step 6: Confirmed current package tests status

Command:

```bash
go test ./pkg/webchat
```

Result:

```text
ok   github.com/go-go-golems/pinocchio/pkg/webchat (cached)
```

Interpretation:

1. Existing tests do not currently catch all three mismatch classes.
2. Additional tests are required as part of bug fix work.

## Step 7: Authored detailed design doc

Actions:

1. Wrote comprehensive analysis and fix design in `design-doc/01-...md`.
2. Included intern runbook, pseudocode, phased implementation plan, and test matrix.

## 3. Key Decisions and Rationale

1. Treat issue A as normalization bug, not API redesign.
2. Treat issues B/C as design seam mismatch requiring clearer pipeline semantics.
3. Recommend explicit dispatch result type to avoid overloading a single boolean for multiple meanings.

## 4. What Worked Well

1. Existing docs already define intended behavior clearly.
2. Existing tests and harness provide a base to extend for regression coverage.
3. Code path is localized to three files (`timeline_js_runtime.go`, `timeline_registry.go`, `timeline_projector.go`).

## 5. What Did Not Work / Friction

1. `docmgr --root` output location was initially assumed wrong (`ttmp` vs direct root-relative).
2. No existing dedicated test file for registry/runtime consume suppression on handler-backed built-ins.

## 6. What Was Tricky to Build

1. Distinguishing implementation bug from architectural issue without over-correcting design.
2. Preserving current behavior for non-consuming flows while making consume semantics uniform.
3. Designing error propagation that is strict enough for debuggability without changing callback-level containment policy.

## 7. Code Review Instructions

Reviewers should validate:

1. consume-only object no longer generates synthetic `js.timeline.entity` upserts.
2. runtime consume can suppress `chat.message` built-in handler projection.
3. runtime errors are surfaced from projector regardless of consume/handled flags.
4. existing llm consume/non-consume behavior remains unchanged.

## 8. Suggested Next Validation Pass (when code changes are implemented)

```bash
go test ./pkg/webchat -run Timeline -count=1
go test ./cmd/web-chat -run LLMDeltaProjectionHarness -count=1
make build
```

## Step 8: Convert Design Into Executable Implementation Tasks

I converted the ticket from analysis-only state into an implementation checklist with task-by-task sequencing. The goal is to execute and commit each behavior fix independently so regressions are easier to isolate in review.

I split the work so normalization, ordering, and error propagation are independently testable, then appended explicit validation as the final step. This allows us to move through the ticket one task at a time and keep commit history easy for an intern to replay.

### Prompt Context

**User prompt (verbatim):** "alright, create tasks to implement that ticket and then work through the tasks one by one, committing as you go, keeping a diary."

**Assistant interpretation:** The user wants execution to proceed now, not just planning, with atomic task completion and commits.

**Inferred user intent:** Ensure the ticket is implemented in a disciplined, auditable workflow with diary continuity.

### What I did
- Updated `tasks.md` to define six concrete implementation tasks.
- Sequenced tasks to isolate risk:
  1. consume-only normalization,
  2. normalization tests,
  3. runtime ordering,
  4. runtime error propagation,
  5. behavior tests,
  6. validation and docs.

### Why
- A decomposed checklist makes one-commit-per-task practical and reviewable.
- This structure prevents mixing multiple semantics changes in one patch.

### What worked
- Existing ticket structure was already in place, so only task granularity needed expansion.

### What didn't work
- N/A

### What I learned
- The best cut points are by behavior contract, not by file.

### What was tricky to build
- Choosing task boundaries that map cleanly to both code changes and test intent.

### What warrants a second pair of eyes
- Task sequencing assumes runtime ordering and error semantics can be changed without breaking existing harness assumptions.

### What should be done in the future
- Keep this per-contract task decomposition pattern for similar runtime behavior tickets.

### Code review instructions
- Verify `tasks.md` now reflects implementation-ready, atomic units.
- Confirm ordering of tasks matches risk reduction strategy.

### Technical details
- Updated file:
  - `2026/03/01/GEPA-07-TIMELINE-JS-CONSUME-CONTRACT--fix-timeline-js-consume-contract-mismatches-and-runtime-error-propagation/tasks.md`

## Step 9: Task 1 Implementation - Consume-Only Normalization Fix

I implemented the first behavior fix in `timeline_js_runtime.go`: consume-only object returns are now treated as control-only outputs and no longer auto-upsert synthetic entities. This directly addresses the contract mismatch for `{consume:true}`.

The key implementation detail is that object returns with a `consume` key now short-circuit as control returns unless the object explicitly looks like an entity payload. This preserves current flexibility for explicit entity objects while removing the accidental fallthrough path.

### Prompt Context

**User prompt (verbatim):** (see Step 8)

**Assistant interpretation:** Start executing the implementation tasks with one commit per completed task and keep diary continuity.

**Inferred user intent:** Resolve the runtime contract bugs incrementally and audibly.

### What I did
- Updated `pkg/webchat/timeline_js_runtime.go`:
  - Added `hasConsume` handling in `decodeReducerReturn`.
  - Added control-object short-circuit for consume-only returns.
  - Added helper `looksLikeTimelineEntityMap` to allow explicit entity payloads with `consume`.
- Ran targeted tests:
  - `go test ./pkg/webchat -run TestJSTimelineRuntime -count=1`

### Why
- Prevent accidental synthetic upserts from control-only reducer outputs.
- Keep backwards compatibility for explicit entity payloads.

### What worked
- Targeted JS runtime tests passed after change.
- Change stayed localized to reducer return normalization path.

### What didn't work
- N/A

### What I learned
- Default `decodeTimelineEntity` fallback is safe only when the input map is known to be entity-like.

### What was tricky to build
- Preserving explicit entity-return flexibility without reintroducing control-object fallthrough.

### What warrants a second pair of eyes
- The `looksLikeTimelineEntityMap` key heuristic should be reviewed to ensure it matches intended reducer API flexibility.

### What should be done in the future
- Add explicit normalization matrix tests to prevent regressions in object-return interpretation.

### Code review instructions
- Start at `pkg/webchat/timeline_js_runtime.go` in `decodeReducerReturn`.
- Validate that `{consume:true}` produces no upsert unless entity fields are explicitly present.
- Re-run: `go test ./pkg/webchat -run TestJSTimelineRuntime -count=1`.

### Technical details
- Updated file:
  - `pkg/webchat/timeline_js_runtime.go`
- Validation command:
  - `go test ./pkg/webchat -run TestJSTimelineRuntime -count=1`
