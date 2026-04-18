# Changelog

## 2026-04-18

- Initial workspace created

## 2026-04-18

Added the first merge-conflict assessment document, documented why `origin/main` should be treated as the baseline for the bootstrap/config subsystem, and expanded the ticket task list into explicit fix and validation phases.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go — Primary bootstrap seam where branch-side and upstream unified-config work collide
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/main.go — Root startup path with the highest conflict density and repository-loading semantics
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/web-chat/main.go — Representative runtime consumer already moved by upstream onto unified profile bootstrap
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/cobra.go — Parser/middleware seam that must align with the final config-plan model
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-MERGE-BOOTSTRAP-CONFLICT--analyze-pinocchio-merge-conflict-between-profile-env-fix-branch-and-unified-config-bootstrap-refactor/analysis/01-merge-conflict-assessment-profile-env-fix-branch-versus-unified-config-bootstrap-refactor.md — Main assessment document for this ticket

## 2026-04-18

Added a diary plus a file-by-file merge-resolution runbook, including explicit `ours`/`theirs` guidance, staged validation commands, and commit tranche boundaries for the actual merge work.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-MERGE-BOOTSTRAP-CONFLICT--analyze-pinocchio-merge-conflict-between-profile-env-fix-branch-and-unified-config-bootstrap-refactor/reference/01-diary.md — Chronological record of the merge assessment and planning work
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-MERGE-BOOTSTRAP-CONFLICT--analyze-pinocchio-merge-conflict-between-profile-env-fix-branch-and-unified-config-bootstrap-refactor/playbooks/01-file-by-file-merge-resolution-runbook.md — Operational runbook for resolving the live conflict set in reviewable tranches
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go — Highest-risk control-plane conflict that anchors the runbook ordering
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/main.go — Startup/repository-loading conflict used to define the first commit boundary

## 2026-04-18

Resolved the repo-level vocabulary YAML conflict, completed Stage A of the merge runbook by taking the upstream baseline for the control-plane files and upstream helper deletions, and fixed one exposed profilebootstrap fixture to the newer `inference_settings.api` schema so focused Stage A tests pass again.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/vocabulary.yaml — Conflict repaired by merging both new topic sets into valid YAML
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/profilebootstrap/profile_selection.go — Resolved to the upstream unified-config baseline as part of Stage A
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/cobra.go — Resolved to the upstream middleware-based parser shape as part of Stage A
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/main.go — Resolved to the upstream repository-loading baseline as part of Stage A
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/pkg/cmds/profilebootstrap/local_profile_plan_test.go — Updated fixture to the new `inference_settings.api` schema after Stage A validation exposed the stale shape

## 2026-04-18

Resolved the remaining Stage B conflicts to the upstream unified-config baseline, reran focused runtime/cmd tests plus a Pinocchio CLI build, and left the profile-env regression question as an explicit follow-up instead of silently changing the merged baseline semantics inside the conflict resolution step.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/web-chat/main.go — Runtime consumer resolved to the unified-config baseline and validated with focused tests
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/pinocchio/cmds/js.go — JS runtime bootstrap resolved to the upstream unified-config baseline
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/web-chat/main_profile_registries_test.go — Restored the honest upstream baseline expectation so the merge checkpoint reflects current code behavior
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/README.md — Kept the newer operator-facing `inference_settings.api` hard-cut note while resolving the docs conflict
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/cmd/examples/simple-chat/main.go — Example command resolved to the upstream parser/config baseline

## 2026-04-18

Validated the merged baseline end-to-end, confirmed the original `PINOCCHIO_PROFILE=... --print-inference-settings` smoke path still works after the merge checkpoint, ran the full Pinocchio test suite, and recorded one remaining semantic follow-up around selection-layer fallback visibility versus successful runtime resolution.

### Related Files

- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-MERGE-BOOTSTRAP-CONFLICT--analyze-pinocchio-merge-conflict-between-profile-env-fix-branch-and-unified-config-bootstrap-refactor/reference/01-diary.md — Diary updated with post-merge validation commands, smoke results, and the merge checkpoint commit
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/ttmp/2026/04/18/PIN-20260418-MERGE-BOOTSTRAP-CONFLICT--analyze-pinocchio-merge-conflict-between-profile-env-fix-branch-and-unified-config-bootstrap-refactor/tasks.md — Validation tasks marked done after full test/build/help/smoke coverage
- /home/manuel/workspaces/2026-04-18/fix-piniocchio-profile-env/pinocchio/README.md — User-facing profile-loading docs kept in sync with the merged baseline and current profile YAML hard cut

