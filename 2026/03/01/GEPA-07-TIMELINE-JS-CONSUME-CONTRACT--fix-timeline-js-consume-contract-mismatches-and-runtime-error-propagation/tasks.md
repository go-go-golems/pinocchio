# Tasks

## TODO

- [x] Create ticket workspace with `docmgr --root` under `pinocchio`.
- [x] Read GEPA-03 and GEPA-06 context and map contract expectations.
- [x] Analyze three flagged issues with line-anchored evidence from runtime/registry/projector code.
- [x] Write intern-focused design doc with root-cause analysis and phased bug-fix plan.
- [x] Write investigation diary with commands, observations, and decisions.
- [x] Run `docmgr doctor` for ticket quality checks.
- [x] Upload deliverable bundle to reMarkable (dry-run + real upload + remote verification).
- [x] Task 1: Fix consume-only reducer normalization so `{consume:true}` does not create synthetic upserts.
- [x] Task 2: Add/extend tests for consume-only reducer output contract.
- [x] Task 3: Reorder runtime execution before list handlers so `consume:true` can suppress handler-backed built-ins.
- [x] Task 4: Fix runtime error propagation so runtime errors are not silently dropped when consume/handled is false.
- [x] Task 5: Add/extend tests for handler-backed consume suppression and runtime error propagation.
- [x] Task 6: Run validation (`go test ./pkg/webchat`, targeted `cmd/web-chat` harness, `make build`) and update docs/diary/changelog.
