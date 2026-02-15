# Changelog

## 2025-12-18

- Initial workspace created


## 2025-12-18

Step 1: Migrated all deprecated Viper usage to Glazed config middlewares. Fixed 8 files, eliminated all SA1019 deprecation warnings. make lint now passes with 0 issues.

### Related Files

- /home/manuel/workspaces/2025-12-01/integrate-moments-persistence/pinocchio/cmd/pinocchio/main.go — Replaced InitViper and InitLoggerFromViper
- /home/manuel/workspaces/2025-12-01/integrate-moments-persistence/pinocchio/pkg/cmds/helpers/parse-helpers.go — Replaced GatherFlagsFromViper with config middlewares


## 2026-02-14

Ticket closed

