# Changelog

## 2026-05-21

- Initial workspace created


## 2026-05-21

Created ticket and intern-ready design for multi-turn stdin/stdout RPC mode.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/01-multi-turn-stdin-stdout-rpc-mode.md — Primary design guide
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary for ticket creation


## 2026-05-21

Uploaded RPC stdin multiturn design bundle to reMarkable at /ai/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/01-multi-turn-stdin-stdout-rpc-mode.md — Uploaded design bundle source


## 2026-05-21

Verified existing multi-turn RPC ticket, refreshed intern orientation in the implementation guide, and prepared bundle for reMarkable upload.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/01-multi-turn-stdin-stdout-rpc-mode.md — Intern-ready stdin RPC design guide
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary Step 2


## 2026-05-21

Implemented first-pass stdin multi-turn RPC mode with request proto, request-id-aware fanout, --stdin-rpc run mode, process-local turn accumulators, tests, and docs.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/doc/general/06-rpc-jsonl-output.md — Docs
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/fanout.go — Request id stamping
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — runStdinRPC implementation
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_stdin_test.go — Tests
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/proto/pinocchio/chatapp/rpc/v1/rpc.proto — RpcRequestLine contract


## 2026-05-21

Recorded stdin RPC implementation commit d6f307a.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary updated with code commit


## 2026-05-21

Added stdin RPC session-isolation test and ran real gpt-5-nano-low subprocess smoke successfully.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_stdin_test.go — Session isolation test
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Smoke-test diary


## 2026-05-21

Recorded session-isolation commit da3b864 and smoke validation in diary.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Commit hash updated

