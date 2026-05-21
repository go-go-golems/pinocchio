# Changelog

## 2026-05-21

- Initial workspace created


## 2026-05-21

Created implementation guide for replacing profile introspection flags with a Glazed pinocchio profiles list command.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-PROFILES-LIST-VERB--add-profiles-list-glazed-command/design-doc/01-implementation-guide.md — Implementation guide
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-PROFILES-LIST-VERB--add-profiles-list-glazed-command/reference/01-implementation-diary.md — Diary


## 2026-05-21

Added a field catalog for the future profiles list command, including default/detailed/full verbosity mappings.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-PROFILES-LIST-VERB--add-profiles-list-glazed-command/design-doc/01-implementation-guide.md — Profiles list field catalog


## 2026-05-21

Expanded profiles list guide with raw inference-setting override fields versus effective resolved settings.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-PROFILES-LIST-VERB--add-profiles-list-glazed-command/design-doc/01-implementation-guide.md — Inference settings override/effective field catalog


## 2026-05-21

Implemented Glazed pinocchio profiles list with raw override and effective inference-setting fields; removed flag-based profile printing UX.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/cmds/profiles/list.go — New list command
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/cmds/profiles/list_test.go — Regression tests
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/main.go — Root command wiring
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Removed flag early exit
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/doc/topics/pinocchio-profile-resolution-and-runtime-switching.md — Docs update

