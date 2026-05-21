# Changelog

## 2026-05-20

- Initial workspace created


## 2026-05-20

Implemented stdout-first mode selection, --debug-events-jsonl event tracing, TUI fanout teeing, and cumulative streaming text patches

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Mode selection and debug JSONL lifecycle wiring
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmdlayers/helpers.go — New debug-events-jsonl flag
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — Cumulative TUI streaming text patches
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/multi_fanout.go — Multi-target UI fanout for debug traces


## 2026-05-20

Updated RPC JSONL help to document --debug-events-jsonl debug traces

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/doc/general/06-rpc-jsonl-output.md — User-facing debug event log documentation


## 2026-05-20

Added blocking-with-debug path so --debug-events-jsonl records sessionstream events while stdout remains normal text

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Blocking mode now routes through chatapp/sessionstream only when debug event logging is requested
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_sessionstream_finalize_test.go — Verifies text stdout plus JSONL debug file


## 2026-05-20

Retroactively normalized implementation diary to the diary-skill step format before upload

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-SESSIONSTREAM-FINALIZE--finalize-sessionstream-port-and-debug-streaming-visibility/reference/01-implementation-diary.md — Diary rewritten with required prompt context


## 2026-05-20

Uploaded today's design docs and diaries to reMarkable as PIN 20260520 design docs and diaries

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-SESSIONSTREAM-FINALIZE--finalize-sessionstream-port-and-debug-streaming-visibility/reference/01-implementation-diary.md — Records successful reMarkable upload


## 2026-05-20

Addressed PR 153 review findings: TUI backend-finished now waits for run completion and RPC done/error status reflects ChatRunFailed

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — RPC/debug done and error frames derive from recorded run status
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_jsonl_test.go — Tests runtime failure produces failed done status
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/run_status_fanout.go — Records terminal run status from UI events
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — BackendFinishedMsg now follows run terminal events
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout_test.go — Tests segment finish does not end backend


## 2026-05-20

Restored pretty human text printer for reasoning summaries and suppressed duplicate aggregate reasoning YAML

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Selects pretty text printer for default/text output
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/event_printer.go — Pinocchio text printer maps reasoning-summary boundaries to thinking markers
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/event_printer_test.go — Covers reasoning-summary formatting and duplicate aggregate suppression


## 2026-05-20

Restored terminal chat-continuation prompt, made --interactive select the interactive path, verified --chat debug JSONL logging, and made TUI reasoning patches create thinking entities when needed.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Chat continuation and interactive control flow
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — TUI reasoning entity creation


## 2026-05-20

Committed Step 7 interactive chat continuation and TUI reasoning/debug fixes (commit 26962fc).

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Committed chat continuation control flow
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — Committed reasoning patch entity creation


## 2026-05-20

Registered chatapp reasoning/tool-call plugins for command runners and verified Wafer GLM emits visible ChatReasoningPatch streams through --chat debug JSONL.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Command runner plugin registration
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_jsonl_test.go — Reasoning projection regression test


## 2026-05-20

Committed command chatapp plugin registration for reasoning/tool-call projections (commit b24b93e).

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Committed command runner plugin registration
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_jsonl_test.go — Committed ChatReasoningPatch regression coverage


## 2026-05-20

Addressed follow-up PR review comments: documented explicit interactive prompting semantics, emitted failed done frames on RPC/debug terminal startup failures, and hydrated chat continuation UI from the seed result turn.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Follow-up PR review fixes for interactive semantics
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_jsonl_test.go — Regression coverage for done.status failed on RPC startup errors
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_sessionstream_finalize_test.go — Regression coverage for seed-turn hydration snapshots


## 2026-05-20

Recorded follow-up PR review fixes from commit 2094b14 in the implementation diary.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-SESSIONSTREAM-FINALIZE--finalize-sessionstream-port-and-debug-streaming-visibility/reference/01-implementation-diary.md — Step 9 records the follow-up PR review fixes and validation


## 2026-05-21

Fixed continuation TUI startup deadlock by moving Bubble Tea hydration sends into the startup goroutine before p.Run; verified with tmux y-continuation and TAB follow-up.

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Moves continuation hydration Program.Send calls into an asynchronous startup goroutine to avoid pre-Run deadlock
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-SESSIONSTREAM-FINALIZE--finalize-sessionstream-port-and-debug-streaming-visibility/reference/01-implementation-diary.md — Step 10 documents the continuation TUI startup deadlock and tmux validation

