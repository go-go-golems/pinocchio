# Changelog

## 2026-05-21

- Initial workspace created


## 2026-05-21

Created design for removing legacy sem timeline stack while preserving current TurnStore and sessionstream hydration

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE--remove-legacy-sem-timeline-stack/design-doc/01-removing-the-legacy-sem-timeline-stack.md — New deletion design and implementation guide
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-REMOVE-LEGACY-SEM-TIMELINE--remove-legacy-sem-timeline-stack/reference/01-implementation-diary.md — Diary records audit and design step


## 2026-05-21

Removed legacy sem timeline stack: old web-chat timeline CLI, chatstore TimelineStore, proto/sem generated outputs, and sem generation tooling

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/Makefile — Proto and security targets no longer reference removed sem outputs
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/buf.yaml — Root Buf config no longer excludes removed web-chat proto island
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/chat_persistence.go — Rewritten to keep only current TurnStore opening
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/doc/topics/webchat-frontend-architecture.md — Frontend architecture docs no longer list historical sem generated types


## 2026-05-21

Step 2: removed legacy sem timeline stack (commit 051ce27)

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/Makefile — Proto generation now targets active chatapp schemas
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/chat_persistence.go — Turns-only persistence helper after legacy timeline removal

