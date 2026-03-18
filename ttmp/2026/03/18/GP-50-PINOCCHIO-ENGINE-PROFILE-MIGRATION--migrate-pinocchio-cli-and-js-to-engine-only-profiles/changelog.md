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

## 2026-03-18

Implemented the repository-loaded command follow-up slice. `PinocchioCommand.RunIntoWriter(...)` now resolves the selected engine profile for blocking runs as well, so YAML-loaded commands no longer ignore profile-selected engine settings. Added a fake-factory regression test that loads a command from YAML and proves the merged engine-profile settings reach `CreateEngine(...)`.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/cmd.go — blocking command path now resolves engine-profile-selected settings
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/cmd_profile_registry_test.go — regression test for loaded YAML commands

## 2026-03-18

Implemented the profile-registry migration slice. Added a standalone migration script for old `~/.config/pinocchio/profiles.yaml`, added helper tests that convert mixed runtime profiles and older flat profile maps into engine-only `inference_settings`, added a regression test for the default auto-discovered profiles file path, and rewrote the public docs to describe the real engine-profile shape and precedence order.

### Related Files

- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/engine_profile_migration.go — core migration helper for mixed and legacy profile YAML
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/pkg/cmds/helpers/engine_profile_migration_test.go — focused conversion tests
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/scripts/migrate_engine_profiles_yaml.go — standalone user-facing migration script
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/main_profile_registries_test.go — default `~/.config/pinocchio/profiles.yaml` regression test
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/README.md — updated engine-profile docs and migration instructions
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/examples/js/README.md — updated JS example docs
- /home/manuel/workspaces/2026-03-17/add-opinionated-apis/pinocchio/cmd/pinocchio/doc/general/05-js-runner-scripts.md — updated help page
