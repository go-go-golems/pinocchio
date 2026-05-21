# Changelog

## 2026-05-21

- Initial workspace created


## 2026-05-21

Created intern-ready design for persisting TUI chatapp final turns and optional sessionstream timelines

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-TUI-TURNS-PERSISTENCE--persist-turns-in-tui-chatapp/design-doc/01-persisting-turns-in-the-tui-chatapp.md — New design and implementation guide
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-TUI-TURNS-PERSISTENCE--persist-turns-in-tui-chatapp/reference/01-implementation-diary.md — Diary records design step and future review notes


## 2026-05-21

Implemented Phase 1 TUI final-turn persistence and Phase 2 sessionstream timeline DB wiring (commit 94c7b29).

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Wires turns and timeline stores into runChat
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_backend.go — Persists successful final turns from OnFinalTurn

