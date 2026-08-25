# Changelog

## 2026-08-23

- Initial workspace created


## 2026-08-23

Wrote and validated the frontend-tool bridge invocation identity/result completion guide; frontmatter/doctor, normal/race tests, and 2 Mermaid renders pass

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/ttmp/2026/08/23/PINOCCHIO-TOOLCALL-1--bind-frontend-tool-calls-to-sessions-and-make-completion-idempotent/design-doc/01-pinocchio-frontend-tool-bridge-hardening-invocation-identity-result-validation-implementation-guide.md — Primary intern implementation guide
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/ttmp/2026/08/23/PINOCCHIO-TOOLCALL-1--bind-frontend-tool-calls-to-sessions-and-make-completion-idempotent/reference/01-diary.md — Investigation and validation record


## 2026-08-23

Dry-ran, uploaded, and verified the Pinocchio guide at /ai/2026/08/23-deliveries/PINOCCHIO-TOOLCALL-1; recorded rmapi duplicate-parent recovery

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/ttmp/2026/08/23/PINOCCHIO-TOOLCALL-1--bind-frontend-tool-calls-to-sessions-and-make-completion-idempotent/reference/01-diary.md — Delivery failure/recovery record
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/ttmp/2026/08/23/PINOCCHIO-TOOLCALL-1--bind-frontend-tool-calls-to-sessions-and-make-completion-idempotent/various/02-remarkable-delivery.md — Canonical upload and listing evidence


## 2026-08-24

Phase 0: bound pending frontend calls to session/call identity, added strict result validation and stable HTTP rejection mapping (commit 8cdc9af)

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/manager.go — Critical containment implementation
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/manager_test.go — Security and collision regression matrix


## 2026-08-24

Phase 1: added atomic terminal completion, identical-retry idempotency, cancellation/timeout terminalization, bounded retention, and concurrency regressions (commit 84c6e63)

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/manager.go — Atomic pending-to-terminal completion
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/manager_test.go — Replay, cancellation, publication failure, and concurrency matrix
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/terminal_store.go — Bounded terminal ledger


## 2026-08-24

Completed server-only Phase 0/1 validation: focused tests, 20x race suite, chatapp suite, make build, lint/vet, frontend build, and full repository tests pass (commit d1e1741)

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/bridge_test.go — Terminal status bridge coverage
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/ttmp/2026/08/23/PINOCCHIO-TOOLCALL-1--bind-frontend-tool-calls-to-sessions-and-make-completion-idempotent/design-doc/01-pinocchio-frontend-tool-bridge-hardening-invocation-identity-result-validation-implementation-guide.md — Implementation status and remaining coordinated phases


## 2026-08-24

Addressed PR 207 review: bounded cancellation publication and terminal cancelled/timeout card rendering (commit c9e8255)

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/cmd/web-chat/web/src/features/web-chat/cards/ToolCallCard/ToolCallCard.tsx — Shared terminal-status handling
- /home/manuel/workspaces/2026-08-20/add-pbui-agent/pinocchio/pkg/chatapp/frontendtools/manager.go — Fresh five-second publication deadline


## 2026-08-25

Narrowed the immediate executor phase to the authoritative client, connection, and assignment tuple; timed leases and automatic takeover remain deferred.

### Related Files

- /home/manuel/workspaces/2026-08-20/add-pbui-agent/react-chat/ttmp/2026/08/23/REACT-CHAT-TOOL-RUNTIME-1--make-browser-tool-execution-idempotent-single-owner-and-manifest-safe/design-doc/02-concise-frontend-tool-executor-ownership-protocol.md — Authoritative cross-repository protocol

