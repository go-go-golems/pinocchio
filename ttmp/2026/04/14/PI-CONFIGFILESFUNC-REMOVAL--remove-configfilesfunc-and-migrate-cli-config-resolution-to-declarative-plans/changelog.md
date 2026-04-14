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

