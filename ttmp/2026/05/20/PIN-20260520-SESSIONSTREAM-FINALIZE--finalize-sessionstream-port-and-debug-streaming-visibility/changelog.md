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

