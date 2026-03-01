# Changelog

## 2026-03-01

- Initial workspace created
- Added primary analysis and bug-fix design doc:
  - `design-doc/01-timeline-js-consume-contract-mismatch-analysis-and-bug-fix-design.md`
- Added chronological investigation diary:
  - `reference/01-investigation-diary.md`
- Captured line-anchored evidence for three runtime contract mismatches:
  - consume-only reducer object causing synthetic upserts,
  - runtime consume not suppressing handler-backed built-ins,
  - runtime errors dropped when handled=false.
- Added phased implementation and test strategy focused on intern onboarding.

## 2026-03-01

Added detailed intern-focused analysis and bug-fix design for timeline JS consume contract mismatches; documented fixes for consume-only normalization, runtime-first consume semantics, and runtime error propagation.

### Related Files

- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/2026/03/01/GEPA-07-TIMELINE-JS-CONSUME-CONTRACT--fix-timeline-js-consume-contract-mismatches-and-runtime-error-propagation/design-doc/01-timeline-js-consume-contract-mismatch-analysis-and-bug-fix-design.md — Primary analysis and implementation blueprint
- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/2026/03/01/GEPA-07-TIMELINE-JS-CONSUME-CONTRACT--fix-timeline-js-consume-contract-mismatches-and-runtime-error-propagation/reference/01-investigation-diary.md — Chronological evidence and decision log
- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/pkg/webchat/timeline_js_runtime.go — Normalization behavior evidence
- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/pkg/webchat/timeline_projector.go — ApplySemFrame gating evidence
- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/pkg/webchat/timeline_registry.go — Ordering and error-propagation evidence

## 2026-03-01

Implemented GEPA-07 runtime contract fixes and tests in sequential task commits.

### Commits

- `1c2b444` — `webchat: treat consume-only reducer objects as control signals`
- `3ac8382` — `webchat: add regression test for consume-only reducer returns`
- `b7db579` — `webchat: run runtime before handlers and propagate runtime failures`

### Validation

- `go test ./pkg/webchat -count=1` ✅
- `go test ./cmd/web-chat -run LLMDeltaProjectionHarness -count=1` ✅
- `make build` ✅

### Related Files

- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/pkg/webchat/timeline_js_runtime.go — Consume-only normalization fix
- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/pkg/webchat/timeline_registry.go — Runtime-first ordering + runtime error propagation fix
- /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/pinocchio/pkg/webchat/timeline_js_runtime_test.go — Regression tests for consume-only, chat.message suppression, and runtime error propagation
