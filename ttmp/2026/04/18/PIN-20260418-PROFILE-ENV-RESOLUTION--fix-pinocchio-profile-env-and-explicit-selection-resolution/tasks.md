# Tasks

## Phase 0 — Ticket / investigation baseline

- [x] Reproduce the profile-selection failure and capture the error output
- [x] Map the current profile/bootstrap architecture and docs mismatch
- [x] Create the first design doc and investigation diary
- [x] Create the second Geppetto-first assessment doc
- [x] Upload the research bundle to reMarkable

## Phase 1 — Restore shared bootstrap behavior in Geppetto

- [x] Add shared implicit default registry discovery in `geppetto/pkg/cli/bootstrap` based on app name / XDG `profiles.yaml`
- [x] Update `ResolveCLIProfileSelection(...)` so implicit registry sources are injected before shared validation runs
- [x] Add a shared Geppetto helper that resolves `profile selection + registry chain` together so callers stop reimplementing chain loading
- [x] Refactor `ResolveCLIEngineSettings(...)` / `ResolveCLIEngineSettingsFromBase(...)` to reuse the shared profile-runtime helper
- [x] Update Geppetto bootstrap tests to assert the restored implicit fallback behavior
- [x] Run targeted Geppetto bootstrap tests
- [x] Commit the Geppetto shared-bootstrap phase

## Phase 2 — Remove duplicated registry logic from Pinocchio callers

- [x] Fix Pinocchio `profilebootstrap` wrappers so the repo builds cleanly against the current shared Geppetto bootstrap API
- [x] Refactor `pinocchio/cmd/web-chat/main.go` to consume shared Geppetto profile-runtime resolution instead of validating/loading registries locally
- [x] Refactor `pinocchio/cmd/pinocchio/cmds/js.go` to consume shared Geppetto profile-runtime resolution instead of reopening registry chains locally
- [x] Refactor `pinocchio/pkg/cmds/helpers/parse-helpers.go` to use shared profile selection instead of re-reading env vars / revalidating registries manually
- [x] Update Pinocchio tests that currently assert “no implicit registry fallback”
- [x] Run targeted Pinocchio tests for `profilebootstrap`, helpers, `web-chat`, and `cmd/pinocchio/cmds`
- [x] Run broader `go test ./...` validation for the Pinocchio repo and fix stale parser/bootstrap call sites
- [x] Commit the Pinocchio consumer-cleanup phase

## Phase 3 — Docs, diary, and validation

- [x] Update current docs/tests that still describe the strict “no implicit registry fallback” model
- [x] Update the ticket diary with implementation steps, exact commands, errors, and commit hashes
- [x] Update ticket changelog / file relations / task state
- [x] Run `docmgr doctor` and fix any issues
- [x] Refresh the reMarkable bundle with the implementation-phase docs
- [x] Commit the ticket/docs phase
