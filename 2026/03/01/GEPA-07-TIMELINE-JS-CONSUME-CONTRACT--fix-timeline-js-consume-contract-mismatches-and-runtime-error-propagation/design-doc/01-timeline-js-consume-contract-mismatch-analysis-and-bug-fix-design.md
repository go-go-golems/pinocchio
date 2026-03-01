---
Title: Timeline JS Consume Contract Mismatch Analysis and Bug Fix Design
Ticket: GEPA-07-TIMELINE-JS-CONSUME-CONTRACT
Status: active
Topics:
    - gepa
    - pinocchio
    - sem
    - goja
    - bug
    - architecture
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/web-chat/llm_delta_projection_harness_test.go
      Note: Harness evidence for consume behavior on llm.delta
    - Path: pkg/doc/topics/13-js-api-reference.md
      Note: Documented consume contract and execution ordering
    - Path: pkg/webchat/timeline_handlers_builtin.go
      Note: chat.message builtin projection handler registration
    - Path: pkg/webchat/timeline_js_runtime.go
      Note: Reducer return normalization and synthetic-entity fallback behavior
    - Path: pkg/webchat/timeline_js_runtime_test.go
      Note: Current JS runtime consume/non-consume behavior coverage
    - Path: pkg/webchat/timeline_projector.go
      Note: Projection gate that currently drops runtime errors when handled=false
    - Path: pkg/webchat/timeline_registry.go
      Note: Handler/runtime ordering and handled-error semantics
    - Path: ttmp/2026/02/26/GEPA-03-EVENT-STREAMING--investigate-js-vm-event-streaming-to-web-frontend-and-sem/design-doc/01-gepa-event-streaming-architecture-investigation.md
      Note: Background architecture and module-based runtime context
    - Path: ttmp/2026/02/26/GEPA-06-JS-SEM-REDUCERS-HANDLERS--investigate-javascript-registered-sem-reducers-and-event-handlers/design-doc/02-cross-repo-js-sem-runtime-implementation-design.md
      Note: Baseline contract and normalization semantics from GEPA-06
ExternalSources: []
Summary: 'Detailed root-cause analysis and implementation design for three contract mismatches in the timeline JS runtime: consume-only reducer output normalization, runtime-vs-builtin ordering, and runtime error propagation semantics.'
LastUpdated: 2026-03-01T07:05:00-05:00
WhatFor: Onboard new engineers and provide a concrete bug-fix blueprint that restores documented runtime behavior.
WhenToUse: Use when implementing or reviewing timeline JS runtime consume semantics and projection ordering/error behavior.
---


# 1. Executive Summary

This document analyzes three review findings against the timeline JS runtime integration in `pkg/webchat` and provides a concrete bug-fix design.

The findings are valid:

1. A reducer return object with only `{ consume: true }` can incorrectly create a synthetic timeline entity.
2. Built-in handler effects (notably `chat.message`) run before runtime consume decisions, so `consume: true` cannot suppress those built-ins.
3. Runtime errors can be dropped silently due to ambiguous `(handled, err)` control-flow semantics.

Impact:

1. Unexpected upserts can overwrite timeline rows (`event.id` collision) and violate script author intent.
2. Runtime contract documented in project docs is not uniformly true for all event types.
3. Runtime failures become hard to debug and can produce inconsistent mixed projection states.

This is not a full architecture failure. It is a contract-drift and phase-boundary design mismatch that can be corrected with targeted changes to runtime normalization and handler orchestration.

---

# 2. Problem Statement and Scope

## 2.1 Problem

We need backend behavior to match the documented and ticketed contract:

1. JS reducers may be additive (`consume: false`) or replacement (`consume: true`).
2. `consume: true` should skip built-in projection for that frame.
3. Runtime execution errors should not be silently ignored.

Current code violates those expectations in specific branches.

## 2.2 In Scope

1. `pkg/webchat/timeline_js_runtime.go`
2. `pkg/webchat/timeline_registry.go`
3. `pkg/webchat/timeline_projector.go`
4. `pkg/webchat` tests and `cmd/web-chat` harness tests
5. Runtime contract docs where needed

## 2.3 Out of Scope

1. New product features beyond contract restoration
2. Full redesign of timeline projection subsystem
3. Cross-repo refactor extraction into separate module

---

# 3. Background From GEPA-03 and GEPA-06

## 3.1 Relevant contract in GEPA-06 design

`GEPA-06` implementation design states:

1. Reducers may set `consume=true` to suppress built-in projection.
2. Supported reducer return forms include `true`, `false`, `{consume:true}`, entity object, entity array, and `{consume, upserts}`.
3. Dispatch order is handlers -> reducers -> sink upserts -> consume decision back to pipeline.

Evidence:

1. `ttmp/2026/02/26/GEPA-06.../design-doc/02-cross-repo-js-sem-runtime-implementation-design.md:120-129`
2. `.../design-doc/02-...md:340-375`
3. `.../design-doc/02-...md:381-418`

## 3.2 Runtime docs currently promise this behavior

`pkg/doc/topics/13-js-api-reference.md` says:

1. `consume: true` skips built-in projection.
2. Runtime execution order ends with consume suppressing built-ins.

Evidence:

1. `pkg/doc/topics/13-js-api-reference.md:119-139`

Conclusion: the intended contract is explicit and stable enough to treat deviations as bugs.

---

# 4. Current-State Architecture (Evidence-Backed)

## 4.1 Projection entrypoint

`TimelineProjector.ApplySemFrame`:

1. decodes SEM envelope,
2. calls `handleTimelineHandlers(...)`,
3. if `handled == true`, returns early,
4. otherwise continues to switch-based built-ins for `llm.*`, tool, planning, etc.

Evidence: `pkg/webchat/timeline_projector.go:82-126`.

## 4.2 Handler/runtime registry path

`handleTimelineHandlers` currently does:

1. Run registered handler list `timelineHandlers[ev.Type]` first.
2. Run optional runtime bridge next.
3. Return handled/error with mixed logic.

Evidence: `pkg/webchat/timeline_registry.go:64-94`.

## 4.3 Built-in handler registration

`chat.message` built-in is registered as a timeline handler (not in projector switch):

1. `registerBuiltinTimelineHandlers()` calls `RegisterTimelineHandler("chat.message", builtinChatMessageTimelineHandler)`.
2. That handler upserts message snapshots directly.

Evidence: `pkg/webchat/timeline_handlers_builtin.go:10-23`.

## 4.4 Reducer return normalization

`decodeReducerReturn` behavior:

1. Handles booleans and arrays.
2. For objects: reads `consume`, then:
   - if `upserts` exists, decode it,
   - else tries to decode the entire object as entity.

Evidence: `pkg/webchat/timeline_js_runtime.go:340-370`.

`decodeTimelineEntity` defaults:

1. `id` defaults to `event.id`.
2. `kind` defaults to `js.timeline.entity`.

Evidence: `pkg/webchat/timeline_js_runtime.go:387-398`.

These defaults are useful for entity-ish objects but unsafe for consume-only control objects.

---

# 5. Detailed Analysis of the Three Findings

## 5.1 Finding A: consume-only object treated as upsert object

### Observed behavior

For reducer output `{ consume: true }`:

1. `consume := toBool(m["consume"])` is true.
2. `upserts` key is absent.
3. Code falls through to `decodeTimelineEntity(m, ev, now)`.
4. Because defaults exist, entity is synthesized with:
   - `id = event.id`
   - `kind = "js.timeline.entity"`

Evidence:

1. `pkg/webchat/timeline_js_runtime.go:353-368`
2. `pkg/webchat/timeline_js_runtime.go:387-398`

### Why this is wrong

Contract says `{consume:true}` is a control result with no implied upsert. Synthesis introduces side effects.

### Risk

1. Accidental row overwrite if `event.id` collides with primary message/tool entity.
2. Hard-to-explain timeline artifacts with default kind.
3. Silent contract break for script authors.

---

## 5.2 Finding B: runtime consumes too late for handler-backed built-ins

### Observed behavior

In `handleTimelineHandlers`:

1. list handlers execute first (`for _, h := range list`).
2. runtime executes after list.

Evidence: `pkg/webchat/timeline_registry.go:73-87`.

Because `chat.message` builtin is in list handlers (`timeline_handlers_builtin.go:10-23`), it executes before runtime reducers can set consume.

### Why this is wrong

Docs and GEPA design assert `consume:true` suppresses built-ins. That should include all built-in projection paths, not only projector-switch branches.

### Risk

1. Inconsistent semantics by event type:
   - `llm.delta` may be suppressible,
   - `chat.message` not suppressible.
2. Surprising mixed projections for replacement reducers.

---

## 5.3 Finding C: runtime error can be ignored when handled=false

### Observed behavior

`handleTimelineHandlers` can return `(handled=false, err!=nil)` from runtime:

```go
if handled || err != nil {
    return handled, err
}
```

Evidence: `pkg/webchat/timeline_registry.go:85-87`.

`ApplySemFrame` only returns the error when `handled` is true:

```go
if handled, err := handleTimelineHandlers(...); handled {
    return err
}
```

Evidence: `pkg/webchat/timeline_projector.go:116-117`.

If handled is false and err non-nil, projector continues to built-ins and error is dropped.

### Why this is wrong

Errors should not disappear based on a boolean intended for consume semantics.

### Risk

1. Runtime failures are masked.
2. Debugging and incident triage become difficult.
3. Built-ins continue after runtime failure, producing partial/contradictory state.

---

# 6. Is This a Base Design Issue?

## 6.1 Short answer

Partially.

1. Finding A is primarily an implementation bug in normalization.
2. Findings B and C reveal a design seam issue:
   - built-ins exist in two phases,
   - consume/error semantics are represented by one ambiguous `(handled, err)` pair.

## 6.2 Design seam description

Current architecture merges three concepts into one boolean:

1. "any handler matched"
2. "consume event / skip built-ins"
3. "error state"

When these are conflated, ordering and propagation bugs become easy.

## 6.3 Recommended design direction

Separate concepts explicitly in registry pipeline return type.

---

# 7. Proposed Fix Design

## 7.1 Design goals

1. Preserve existing additive behavior where intended.
2. Make consume semantics uniform for handler-backed and switch-based built-ins.
3. Ensure runtime errors are never silently dropped.
4. Keep public script API unchanged.

## 7.2 Contract model after fix

For each SEM event:

1. Runtime executes first (handlers + reducers).
2. Runtime may emit upserts and set `consume`.
3. If runtime errored, event handling fails (or at minimum is surfaced and skipped deterministically; see alternatives).
4. If `consume` is true, no built-in handlers/switch branches run.
5. If `consume` is false, built-ins run.

## 7.3 API/flow refactor (internal)

Replace ambiguous `(handled bool, err error)` in registry path with explicit result:

```go
type TimelineDispatchResult struct {
    Consumed bool
    Matched  bool
}
```

Alternative minimal change if we want low churn:

1. Keep signature but redefine behavior:
   - runtime err always returns `handled=true` to force projector to surface error.
2. This is less clean and easier to regress later.

Preferred: explicit struct.

## 7.4 Ordering fix

In `handleTimelineHandlers`:

1. evaluate runtime first,
2. if runtime returns consume -> short-circuit built-in handlers,
3. else run list handlers.

Pseudo-flow:

```go
if runtime != nil {
    consumed, err := runtime.HandleSemEvent(...)
    if err != nil {
        return TimelineDispatchResult{Consumed: false, Matched: true}, err
    }
    if consumed {
        return TimelineDispatchResult{Consumed: true, Matched: true}, nil
    }
}

matchedList := false
for _, h := range list {
    matchedList = true
    if err := h(...); err != nil {
        return TimelineDispatchResult{Consumed: true, Matched: true}, err
    }
}
return TimelineDispatchResult{Consumed: matchedList, Matched: matchedList}, nil
```

Note:

1. Returning `Consumed=true` for list-handled preserves existing behavior where handler-backed events skip projector switch.
2. Runtime consume now genuinely suppresses handler-backed built-ins.

## 7.5 Normalization fix for consume-only control objects

In `decodeReducerReturn` object branch:

1. if object has `consume` key and has no `upserts`, treat as control-only output,
2. return `(consume, nil)` without entity decode fallback.

Pseudo:

```go
consume := toBool(m["consume"])
_, hasConsume := m["consume"]
_, hasUpserts := m["upserts"]

if hasUpserts {
    // existing decode logic
}

if hasConsume {
    return consume, nil
}

if looksLikeEntity(m) {
    return consume, []Entity{decodeEntity(m)}
}

return consume, nil
```

Key point: add `looksLikeEntity` guard before decoding fallback.

Suggested `looksLikeEntity` condition:

1. true if any of `id`, `kind`, `props`, `meta`, `created_at_ms`, `createdAtMs`, `updated_at_ms`, `updatedAtMs` is present.
2. false for pure control objects.

## 7.6 Error propagation fix

Change projector call-site to always propagate errors regardless of consume/handled flags.

Current:

```go
if handled, err := handleTimelineHandlers(...); handled {
    return err
}
```

Preferred:

```go
result, err := handleTimelineHandlers(...)
if err != nil {
    return err
}
if result.Consumed {
    return nil
}
```

This removes error-dropping class entirely.

---

# 8. File-by-File Implementation Plan

## Phase 1: Control-flow and normalization fixes

1. `pkg/webchat/timeline_js_runtime.go`
   - adjust `decodeReducerReturn` control-object handling,
   - add `looksLikeTimelineEntityMap` helper,
   - keep current entity decoding defaults for entity-like objects.

2. `pkg/webchat/timeline_registry.go`
   - refactor `handleTimelineHandlers` ordering (runtime first),
   - replace ambiguous return semantics with explicit dispatch result (or minimally force runtime errors to handled=true).

3. `pkg/webchat/timeline_projector.go`
   - propagate errors unconditionally,
   - branch on explicit consume signal.

## Phase 2: Tests

1. `pkg/webchat/timeline_js_runtime_test.go`
   - add consume-only reducer test ensuring no synthetic entity upsert.

2. New/updated tests around registry/projector integration (suggest `pkg/webchat/timeline_registry_runtime_contract_test.go`):
   - runtime consume suppresses `chat.message` built-in handler,
   - runtime error with handled=false equivalent is surfaced,
   - non-consuming runtime preserves existing built-in behavior.

3. `cmd/web-chat/llm_delta_projection_harness_test.go`
   - keep existing consume/non-consume behavior checks,
   - optionally add `chat.message` harness path if practical.

## Phase 3: Documentation updates

1. `pkg/doc/topics/13-js-api-reference.md`
2. `pkg/doc/topics/14-js-api-user-guide.md`
3. `cmd/web-chat/README.md`

Confirm docs match actual ordering and consume semantics after fixes.

---

# 9. Test Strategy (Detailed)

## 9.1 Unit matrix for reducer normalization

Cases for `decodeReducerReturn`:

1. `nil` -> consume false, no upserts.
2. `true` -> consume true, no upserts.
3. `false` -> consume false, no upserts.
4. `{consume:true}` -> consume true, no upserts.
5. `{consume:false}` -> consume false, no upserts.
6. entity map without consume/upserts -> one upsert.
7. `{consume:true, upserts:[...]}` -> consume true, upserts decoded.
8. `{consume:true, upserts:{...}}` -> consume true, single upsert.
9. malformed upserts type -> consume respected, no upserts.

## 9.2 Behavior tests for ordering

1. Register runtime reducer for `chat.message` returning `{consume:true}` and side upsert.
2. Send `chat.message` frame.
3. Assert built-in message projection did not run.
4. Assert runtime side upsert exists.

## 9.3 Error propagation tests

1. Inject runtime that returns error and consumed=false.
2. `ApplySemFrame` must return error.
3. Assert no further built-in writes occur for that frame.

## 9.4 Regression tests

1. Existing `llm.delta` consume harness still passes.
2. Existing non-consume additive behavior still passes.

Validation commands:

```bash
go test ./pkg/webchat -run Timeline -count=1
go test ./cmd/web-chat -run LLMDeltaProjectionHarness -count=1
make build
```

---

# 10. Alternatives and Tradeoffs

## Option A (recommended): explicit dispatch result + runtime-first ordering

Pros:

1. Clean semantics.
2. Removes error-dropping edge class.
3. Aligns with docs and GEPA design.

Cons:

1. Slightly broader internal refactor.

## Option B: minimal patch without new result type

Approach:

1. Runtime first,
2. force runtime errors to return handled=true,
3. fix consume-only normalization.

Pros:

1. Smaller patch surface.

Cons:

1. Boolean remains overloaded.
2. Future regressions more likely.

Recommendation: Option A.

---

# 11. Migration and Compatibility Notes

1. No JS API shape changes required.
2. Behavior change is corrective:
   - consume-only objects stop creating synthetic entities,
   - consume works consistently for handler-backed built-ins.
3. Some scripts that accidentally relied on buggy synthetic entity creation may observe changed output. This is acceptable and should be called out in release notes.

---

# 12. Intern Implementation Runbook

Use this sequence when implementing:

1. Read current control flow in:
   - `pkg/webchat/timeline_projector.go`
   - `pkg/webchat/timeline_registry.go`
   - `pkg/webchat/timeline_js_runtime.go`
2. Implement normalization fix first (smallest, safest).
3. Add unit tests for normalization before touching ordering.
4. Refactor registry ordering and explicit result semantics.
5. Update projector call-site to propagate errors unconditionally.
6. Add integration tests for:
   - consume suppressing `chat.message`,
   - runtime error propagation.
7. Run targeted tests, then broader package tests.
8. Update docs to reflect exact runtime behavior.

Code review checklist:

1. Does `{consume:true}` produce zero upserts unless `upserts` provided?
2. Can runtime consume suppress `chat.message` built-in handler?
3. Are runtime errors surfaced from `ApplySemFrame` in all branches?
4. Do existing consume/non-consume llm harness tests still pass?
5. Is documentation updated and line-accurate?

---

# 13. Risks and Mitigations

1. Risk: behavior changes for scripts depending on buggy fallback.
   - Mitigation: document in changelog; provide migration examples.
2. Risk: ordering refactor impacts app-owned handlers.
   - Mitigation: add tests for app-owned handler event types.
3. Risk: stricter error propagation may surface noisy runtime issues.
   - Mitigation: keep callback-level throw containment; only propagate host/runtime execution errors.

---

# 14. Open Questions

1. Should runtime host execution errors fail the entire frame hard, or be downgraded to warnings in production mode?
2. Should control-object parsing accept string booleans (`"true"`) or stay strict boolean-only for `consume`?
3. Do we want explicit metrics counters for:
   - runtime consumed events,
   - runtime errors,
   - control-only returns?

---

# 15. Concrete References

## Core implementation

1. `pkg/webchat/timeline_projector.go:82-126`
2. `pkg/webchat/timeline_registry.go:20-95`
3. `pkg/webchat/timeline_js_runtime.go:219-370`
4. `pkg/webchat/timeline_js_runtime.go:387-425`
5. `pkg/webchat/timeline_handlers_builtin.go:10-23`

## Tests and harness

1. `pkg/webchat/timeline_js_runtime_test.go:20-140`
2. `pkg/webchat/timeline_projector_test.go:130-151`
3. `cmd/web-chat/llm_delta_projection_harness_test.go:235-290`

## Docs and ticket context

1. `pkg/doc/topics/13-js-api-reference.md:119-139`
2. `pkg/doc/topics/14-js-api-user-guide.md:100-130`
3. `cmd/web-chat/README.md:124-146`
4. `ttmp/2026/02/26/GEPA-06-JS-SEM-REDUCERS-HANDLERS--investigate-javascript-registered-sem-reducers-and-event-handlers/design-doc/02-cross-repo-js-sem-runtime-implementation-design.md:120-129`
5. `.../design-doc/02-...md:340-418`
