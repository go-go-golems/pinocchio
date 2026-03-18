# Changelog

## 2026-03-18

- Initial workspace created


## 2026-03-18

Created the Pinocchio downstream engine-profile migration ticket, documented the CLI and JS breakage, and wrote a phased implementation plan focused on repository-loaded commands and pinocchio js.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/cmds/js.go — Current JS bootstrap shape analyzed for migration
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go — Current shared helper bug identified in analysis

## 2026-03-18

Implemented the first downstream migration slice for the Pinocchio CLI and JS command. The shared helper now returns final merged `InferenceSettings`, direct CLI callers were updated, the legacy `profiles_migrate_legacy` command was deleted, the `pinocchio js` examples/tests were rewritten to the engine-profile model, and the command help/docs now teach `gp.profiles.resolve({})` plus `gp.engines.fromResolvedProfile(...)` instead of the removed mixed runtime path.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime.go — canonical final-settings helper
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/profile_runtime_test.go — focused merge-path tests
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main_profile_registries_test.go — JS command regression coverage
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-demo.js — real inference example now uses engine profiles directly
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/runner-profile-smoke.js — deterministic smoke script now validates profile-selected engine settings
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/profiles/basic.yaml — fixture rewritten to engine-profile YAML
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md — help page updated to the new split
