# Changelog

## 2025-12-18

- Initial workspace created


## 2025-12-18

Step 1: Fixed flag parsing issue - added check for --help flag before manual ParseFlags call to allow cobra to handle help naturally

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/pinocchio/main.go — Fixed initialization sequence


## 2025-12-18

Step 2: Add early logging flagset debug dump + filter args to only logging flags (so early init honors --log-level without breaking cobra parsing)

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/pinocchio/main.go — Early logging init now parses only logging-related args and can print parsed values


## 2025-12-18

Code: 4da6556 — pinocchio: pre-parse logging flags before command loading

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/pinocchio/main.go — Early logging init pre-parses logging flags; adds --debug-early-flagset dump; fixes help behavior


## 2025-12-18

Code: 2b1ac84 — pinocchio: use glazed InitEarlyLoggingFromArgs, remove duplicate code and debug flag

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/pinocchio/main.go — Removed duplicate early logging code


## 2026-02-14

Ticket closed


## 2026-02-14

Ticket closed

