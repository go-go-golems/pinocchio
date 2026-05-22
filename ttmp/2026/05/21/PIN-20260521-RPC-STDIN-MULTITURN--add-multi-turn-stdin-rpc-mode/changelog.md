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


## 2026-05-21

Implemented and tested cancel-while-running behavior for stdin RPC by allowing cancel requests through while submit runs asynchronously.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — runStdinRPC active-run and cancel handling
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_stdin_test.go — Cancel-while-running test
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary step


## 2026-05-21

Recorded cancel-while-running commit 5f99604 in the diary.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Commit hash updated


## 2026-05-22

Added PR 156 multi-session RPC foundations guide covering request-scoped state, session actors, request-aware fanout, keyed status tracking, and implementation plan.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/fanout.go — Request id stamping review context
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Current concurrency problem context
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/run_status_fanout.go — Status tracking review context
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/02-multi-session-rpc-foundations-and-pr-156-review-response.md — New design guide


## 2026-05-22

Uploaded PR 156 multi-session RPC foundations guide to reMarkable at /ai/2026/05/22/PIN-20260521-RPC-STDIN-MULTITURN.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/02-multi-session-rpc-foundations-and-pr-156-review-response.md — Uploaded guide source


## 2026-05-22

Recorded diary Step 6 for PR 156 multi-session foundations guide and reMarkable upload.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/02-multi-session-rpc-foundations-and-pr-156-review-response.md — Guide referenced by diary
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary Step 6


## 2026-05-22

Recorded docs commit 4124c7e for PR 156 multi-session foundations guide.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Commit hash updated


## 2026-05-22

Corrected diary commit hash after amend to 511051c.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Commit hash corrected


## 2026-05-22

Posted PR 156 comment linking to the multi-session RPC foundations guide.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — PR comment recorded


## 2026-05-22

Added single-session stdin RPC implementation guide choosing one conversation per process for simplicity.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/03-single-session-stdin-rpc-implementation-guide.md — New implementation guide


## 2026-05-22

Recorded single-session guide commit d324461 in diary.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Step 7 commit hash


## 2026-05-22

Implemented single-session stdin RPC semantics: process-bound session id, session_mismatch, session_busy, explicit control-frame request ids, tests, and docs (commit 731eafd).

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/doc/general/06-rpc-jsonl-output.md — User-facing single-session docs
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/fanout.go — Explicit request-id frame helpers
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Single-session stdin RPC implementation
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_stdin_test.go — Single-session tests
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary Step 8


## 2026-05-22

Uploaded single-session stdin RPC guide to reMarkable and validated real strictness/sequential subprocess smokes.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/03-single-session-stdin-rpc-implementation-guide.md — Uploaded guide
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary Step 9


## 2026-05-22

Recorded docs commit 4def508 for single-session completion diary.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Step 9 commit hash


## 2026-05-22

Posted PR 156 comment explaining single-session stdin RPC resolution.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary Step 10


## 2026-05-22

Fixed stdin RPC request-local errors to be non-terminal and added regression coverage.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Request-local terminal=false fixes
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_stdin_test.go — Non-terminal request error regression test
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Diary Step 11


## 2026-05-22

Recorded non-terminal request error fix commit 030385f in diary.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/reference/01-implementation-diary.md — Step 11 commit hash

