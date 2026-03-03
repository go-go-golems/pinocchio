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

