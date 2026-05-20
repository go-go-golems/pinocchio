# Changelog

## 2026-05-20

- Initial workspace created.
- Created design document for JSONL/RPC output mode for Pinocchio CLI verbs.
- Created chronological investigation diary.
- Documented current command loading, helper flag, event router, and printer architecture.
- Proposed `--rpc` plus `--output jsonl`, a versioned JSONL envelope, event-kind mapping, implementation phases, and tests.
- Related key Pinocchio and Geppetto source files to the ticket.
- Validated the ticket with `docmgr doctor --ticket PIN-20260520-CLI-RPC-JSONL --stale-after 30`.
- Uploaded the final design bundle to reMarkable under `/ai/2026/05/20/PIN-20260520-CLI-RPC-JSONL`.

## 2026-05-20

Created intern-ready JSONL/RPC output mode design and investigation diary

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/01-jsonl-rpc-output-mode-for-pinocchio-cli-verbs.md — Primary design deliverable
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/reference/01-investigation-diary.md — Chronological diary deliverable


## 2026-05-20

Added follow-up design to unify CLI/TUI/RPC streams on sessionstream and chatapp instead of duplicating raw Geppetto event mapping

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md — Follow-up design deliverable
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/reference/01-investigation-diary.md — Updated diary with sessionstream/chatapp investigation


## 2026-05-20

Updated unified chatapp/sessionstream design to make the JSONL RPC line format protobuf-defined via a proposed pinocchio.chatapp.rpc.v1.RpcLine envelope

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/design-doc/02-unify-pinocchio-cli-tui-and-rpc-streams-on-sessionstream-chatapp.md — Updated protobuf-defined JSONL boundary
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/reference/01-investigation-diary.md — Recorded protobuf boundary update


## 2026-05-20

Expanded ticket tasks into phased implementation plan and marked Phase 1 as active

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/reference/01-investigation-diary.md — Diary step for phased task planning
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/tasks.md — Phased implementation checklist

