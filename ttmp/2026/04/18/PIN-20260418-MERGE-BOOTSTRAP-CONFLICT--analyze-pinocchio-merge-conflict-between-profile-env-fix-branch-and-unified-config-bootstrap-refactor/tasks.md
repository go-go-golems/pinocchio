# Tasks

## Phase 1 — Analyze and choose the resolution baseline

- [x] Capture the current merge state (`git status`, unmerged file list, branch-only vs main-only commits).
- [x] Identify which conflicts are architectural hot spots versus low-risk docs/tests.
- [x] Decide and document the resolution rule: use `origin/main` as the baseline for Pinocchio bootstrap/config architecture, then replay only still-missing behavior.

## Phase 2 — Resolve the merge safely

- [x] Resolve helper deletion conflicts by keeping upstream helper removals unless a real active caller proves otherwise:
  - `pkg/cmds/helpers/parse-helpers.go`
  - `pkg/cmds/helpers/profile_selection_test.go`
- [x] Resolve `pkg/cmds/profilebootstrap/profile_selection.go` using the unified-config/configdoc model as the baseline.
- [x] Resolve `pkg/cmds/cobra.go` so command middleware wiring matches the active config-plan/unified-config approach.
- [x] Resolve `cmd/pinocchio/main.go` so repository discovery follows the current unified config model and does not reintroduce deprecated wrapper behavior.
- [x] Resolve runtime consumer conflicts against the new baseline:
  - `cmd/web-chat/main.go`
  - `cmd/pinocchio/cmds/js.go`
  - `cmd/examples/simple-chat/main.go`
  - `cmd/examples/simple-redis-streaming-inference/main.go`
  - `cmd/agents/simple-chat-agent/main.go`
- [x] Resolve test conflicts after active runtime code settles:
  - `cmd/pinocchio/cmds/js_test.go`
  - `cmd/web-chat/main_profile_registries_test.go`
- [x] Resolve docs/vocabulary conflicts last:
  - `README.md`
  - `ttmp/vocabulary.yaml`

## Phase 3 — Reapply branch-specific intent where still needed

- [ ] Verify whether the original `PINOCCHIO_PROFILE` / `--profile` bugfix behavior already survives inside the upstream unified-config architecture.
- [ ] If any branch-specific behavior is missing, replay it in the new architecture instead of reintroducing the old implementation shape.
- [ ] Reconfirm that newer local ticket work is still represented after the merge where applicable:
  - profile-env smoke validation expectations
  - Gemini/auth-related docs context if touched by merged docs
  - HTTP-client debug-trace awareness if touched by merged docs

## Phase 4 — Focused validation after conflict resolution

- [ ] Confirm the working tree has no unresolved conflicts and no accidental resurrected helper files.
- [x] Run focused Pinocchio package validation in the workspace:
  - `cd pinocchio && go test ./pkg/cmds/... ./cmd/pinocchio/... ./cmd/web-chat -count=1`
- [x] Run focused unified-config/bootstrap tests in the workspace:
  - `cd pinocchio && go test ./pkg/cmds/profilebootstrap ./pkg/configdoc -count=1`
- [x] Rebuild the CLI with the workspace modules in effect:
  - `cd pinocchio && go build -o /tmp/pinocchio-merge-check ./cmd/pinocchio`
- [ ] Re-run the original safe profile-selection smoke path:
  - `PINOCCHIO_PROFILE=gemini-2.5-pro /tmp/pinocchio-merge-check code professional hello --print-inference-settings`
- [ ] Inspect the debug output for the expected profile-selection evidence:
  - selected profile slug
  - final engine from the selected profile
  - source/provenance showing profile application
  - `http_client:` decision block still present if using the local Geppetto workspace
- [ ] Re-run a real non-debug runtime smoke path if provider credentials/config are available:
  - `PINOCCHIO_PROFILE=gemini-2.5-pro /tmp/pinocchio-merge-check code professional hello`
- [ ] Re-run a command-discovery smoke path to ensure startup/repository loading still works:
  - `/tmp/pinocchio-merge-check --help`
- [ ] Re-run a JS/bootstrap smoke path if that conflict required manual resolution:
  - `/tmp/pinocchio-merge-check js --help`
  - optional focused script smoke if needed

## Phase 5 — Broader validation and bookkeeping

- [ ] Run broader repo validation only after focused bootstrap tests pass:
  - `cd pinocchio && go test ./... -count=1`
- [ ] Recheck `git status` to ensure only intentional merge-resolution results remain.
- [ ] Update this ticket docs with the final resolution decision, what was kept from upstream, and what was replayed from the branch.
- [ ] Run `docmgr doctor --ticket PIN-20260418-MERGE-BOOTSTRAP-CONFLICT --stale-after 30`.

