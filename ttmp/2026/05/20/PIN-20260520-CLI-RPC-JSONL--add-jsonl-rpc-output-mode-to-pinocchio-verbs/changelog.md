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


## 2026-05-20

Phase 1: added protobuf RpcLine contract, generated Go/TS bindings, and protojson round-trip test

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/rpc_proto_test.go — Generated RpcLine protojson round-trip test
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/proto/pinocchio/chatapp/rpc/v1/rpc.proto — New protobuf JSONL line contract


## 2026-05-20

Fixed generated RPC TypeScript binding import ordering after Biome pre-commit failure

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/web-chat/web/src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts — Generated TypeScript binding formatted for Biome
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/ttmp/2026/05/20/PIN-20260520-CLI-RPC-JSONL--add-jsonl-rpc-output-mode-to-pinocchio-verbs/reference/01-investigation-diary.md — Recorded pre-commit failure and fix


## 2026-05-20

Phase 2: added protojson JSONL writer and framing tests for RpcLine

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/writer.go — ProtoJSON JSONL writer implementation
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/writer_test.go — Writer framing and round-trip tests


## 2026-05-20

Phase 3: added sessionstream UIFanout JSONL adapter with snapshot/backend-event helpers and Any payload tests

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/fanout.go — sessionstream UI fanout to protobuf JSONL frames
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/rpc/jsonl/fanout_test.go — fanout


## 2026-05-20

Phase 4: added reusable non-web chatapp runner with sessionstream wiring and tests

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/runner.go — Reusable chatapp/sessionstream runner
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/runner_test.go — Runner submission snapshot and plugin schema tests


## 2026-05-20

Phase 5: added PromptRequest.InitialTurn and runtime seeding tests for rich Pinocchio verb input

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/chat_test.go — InitialTurn runtime behavior tests
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/runtime_inference.go — Runtime seeding from InitialTurn
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/service.go — PromptRequest InitialTurn input

