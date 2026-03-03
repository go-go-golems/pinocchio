# Changelog

## 2026-03-03

- Initial workspace created


## 2026-03-03

Step 1: scaffold PI-02 ticket workspace, tasks, implementation plan, and diary (commit 5c866ec)

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/ttmp/2026/03/03/PI-02--extract-common-tui/reference/01-diary.md — Record PI-02 Step 1
- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/ttmp/2026/03/03/PI-02--extract-common-tui/tasks.md — Task list for extraction work


## 2026-03-03

Step 2: extract ToolLoopBackend out of cmd/ into pinocchio/pkg/ui/backends/toolloop (commit 17b2100f9224add43db713d8033d2fec621109d0)

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/cmd/agents/simple-chat-agent/main.go — Updated to use extracted pkg backend
- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/ui/backends/toolloop/backend.go — New reusable tool-loop backend + temporary forwarder


## 2026-03-03

Step 3: extract agent UI forwarder into pinocchio/pkg/ui/forwarders/agent and update wiring (commit 3a224057b0a7011f5ee52050061941c6ca509ae6)

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/ui/backends/toolloop/backend.go — Backend now only runs tool loop; projection moved out
- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/ui/forwarders/agent/forwarder.go — Agent event→timeline forwarder


## 2026-03-03

Step 4: tmux smoke run of simple-chat-agent in help-mode (no DB/credentials side effects)

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/cmd/agents/simple-chat-agent/main.go — Smoke-verified command still starts post-refactor


## 2026-03-03

Ticket closed


## 2026-03-03

Step 5: add Glazed help pages for TUI integration (guide + playbook) and validate via pinocchio help (commit dbbacec0daaf6db2be33f5798d59f8fcb72f65e6)

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md — New TUI integration ops/debugging playbook
- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md — New intern-first TUI integration guide


## 2026-03-03

Step 6: upload bundled TUI integration guide + playbook to reMarkable under /ai/2026/03/03/PI-02--extract-common-tui

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md — Included in reMarkable bundle
- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md — Included in reMarkable bundle


## 2026-03-03

Docs follow-up: add explicit import-path hints to all TUI integration code snippets (commit e6a87ec3778039ed32e931182597b3f23bde86b6)

### Related Files

- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/doc/topics/pinocchio-tui-integration-playbook.md — Now includes import-path hints above each snippet
- /home/manuel/workspaces/2026-03-03/extract-tui-pinocchio/pinocchio/pkg/doc/tutorials/06-tui-integration-guide.md — Now includes import-path hints above each snippet

