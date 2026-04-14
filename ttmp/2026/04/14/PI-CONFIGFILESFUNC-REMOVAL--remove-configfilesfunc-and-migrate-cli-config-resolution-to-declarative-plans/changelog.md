# Changelog

## 2026-04-14

- Initial workspace created


## 2026-04-14

Created follow-up cleanup ticket PI-CONFIGFILESFUNC-REMOVAL with an analysis and implementation plan for removing CobraParserConfig.ConfigFilesFunc, migrating current workspace callers to declarative config plans, and assessing whether pkg/appconfig should be modernized or removed instead of preserved.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/analysis/01-configfilesfunc-removal-analysis-and-appconfig-cleanup-plan.md — Primary analysis and implementation-plan document for the cleanup ticket
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/tasks.md — Concrete migration tasks for removing ConfigFilesFunc and deciding the future of pkg/appconfig


## 2026-04-14

Implemented the cleanup requested by the ticket/user: removed CobraParserConfig.ConfigFilesFunc and ConfigPath, added plan-based ConfigPlanBuilder in glazed (commit 0e0f443), removed the pkg/appconfig facade plus its examples and updated active docs (commit c850f23), and simplified Pinocchio command wiring by deleting no-op parser shims that only existed to suppress implicit config loading (commit 8765765). Focused Glazed validation passed; Pinocchio command-package validation is currently blocked by an external clay dependency still importing logging.InitLoggerFromViper.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/appconfig/parser.go — Removed with the pkg/appconfig facade in commit c850f23
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go — Core parser cleanup and plan-based replacement in commit 0e0f443
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra_parser_config_test.go — Regression coverage for new parser semantics in commit 0e0f443
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/24-config-files.md — Current docs updated to teach plan-based CLI config loading in commit c850f23
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main.go — Removed no-op parser shim in commit 8765765
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/reference/01-diary.md — Recorded implementation steps


## 2026-04-14

Completed the final cleanup step: removed glazed/pkg/config/ResolveAppConfigPath (commit a94d873), removed Geppetto's legacy non-plan fallback and made ConfigPlanBuilder mandatory for bootstrap config (commit 8ef6188), and migrated Pinocchio's repository config loading to a declarative plan (commit 3118d0c). Focused Geppetto and Glazed validation passed; Pinocchio top-level command-package validation remains blocked by the external clay dependency still importing logging.InitLoggerFromViper. Also added a follow-up task to introduce FromConfigPlan/FromConfigPlanBuilder middleware wrappers over FromResolvedFiles.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/cli/bootstrap/profile_selection.go — Removed legacy config-file fallback and now requires plan-based resolution in commit 8ef6188
- /home/manuel/workspaces/2026-04-10/pinocchiorc/geppetto/pkg/sections/sections.go — Legacy Geppetto section helper now resolves pinocchio config through a plan in commit 8ef6188
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/config/resolve.go — Deleted the last compatibility app-config resolver in commit a94d873
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/pinocchio/main.go — Repository-loading now uses layered config plans in commit 3118d0c
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/reference/01-diary.md — Recorded Step 4 removing ResolveAppConfigPath and the remaining validation caveats
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/tasks.md — Added a follow-up task for FromConfigPlan middleware wrappers


## 2026-04-14

Added high-level plan middleware wrappers in glazed (commit f13b8df) so ConfigPlanBuilder now delegates through sources.FromConfigPlanBuilder over FromResolvedFiles, and fixed the local clay module's deprecated InitViper path (commit 20a8a9d) so workspace command-package validation no longer fails on the removed Glazed Viper logger symbol. Also cleaned a stale unused import in a Pinocchio test (commit 68994cc).

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/clay/pkg/init.go — Removed the stale dependency on logging.InitLoggerFromViper in commit 20a8a9d
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cli/cobra-parser.go — Cobra parser now uses FromConfigPlanBuilder internally in commit f13b8df
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/cmds/sources/load-fields-from-config.go — Added FromConfigPlan and FromConfigPlanBuilder in commit f13b8df
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/cmd/web-chat/main_profile_registries_test.go — Cleaned unused import after validation unblocked in commit 68994cc
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/reference/01-diary.md — Recorded Step 5 implementing FromConfigPlan middleware and the Clay fix
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/tasks.md — Marked the FromConfigPlan middleware follow-up done


## 2026-04-14

Updated the active Glazed config docs so the new direct plan-loading middleware APIs are discoverable: 24-config-files, 27-declarative-config-plans, the declarative-config-plan example page, the config-files quickstart, and the Viper migration guide now explain when to use FromConfigPlan/FromConfigPlanBuilder versus FromResolvedFiles. Also marked the docs follow-up done in the ticket tasks.

### Related Files

- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/examples/config/01-declarative-config-plan.md — Explained why the example uses FromResolvedFiles and when FromConfigPlan is appropriate
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/24-config-files.md — Added direct FromConfigPlan guidance alongside manual plan.Resolve plus FromResolvedFiles
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/topics/27-declarative-config-plans.md — Documented both manual resolution and direct FromConfigPlan/FromConfigPlanBuilder loading styles
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/config-files-quickstart.md — Added direct middleware framing for ConfigPlanBuilder users
- /home/manuel/workspaces/2026-04-10/pinocchiorc/glazed/pkg/doc/tutorials/migrating-from-viper-to-config-files.md — Added explicit note that a plan can be loaded directly through FromConfigPlan
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/reference/01-diary.md — Recorded Step 6 for the documentation refresh
- /home/manuel/workspaces/2026-04-10/pinocchiorc/pinocchio/ttmp/2026/04/14/PI-CONFIGFILESFUNC-REMOVAL--remove-configfilesfunc-and-migrate-cli-config-resolution-to-declarative-plans/tasks.md — Marked the docs follow-up done

