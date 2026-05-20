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


## 2026-05-20

Phase 6: wired Pinocchio CLI --rpc/--output jsonl through chatapp/sessionstream protobuf JSONL

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — RPC JSONL run mode implementation
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd_rpc_jsonl_test.go — CLI RPC JSONL integration tests
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmdlayers/helpers.go — --rpc flag and jsonl output choice
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/run/context.go — RunModeRPCJSONL and UI RPC setting


## 2026-05-20

Committed Phase 6 CLI RPC integration as cfaf7fb

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Phase 6 committed in cfaf7fb


## 2026-05-20

Phase 7: added Bubble Tea sessionstream UIFanout adapter and snapshot hydration tests

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — Bubble Tea chatapp UIFanout adapter
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout_test.go — Adapter tests for streaming


## 2026-05-20

Committed Phase 7 preparatory Bubble Tea adapter as 72a3d17

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — Phase 7 adapter committed in 72a3d17


## 2026-05-20

Ran real tmux smoke tests for RPC JSONL and TUI with gpt-5-nano-low and gpt-5-mini; removed unused TUI wrapper APIs

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Removed stale compatibility comment
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_fanout.go — Removed NewChatAppUIFanoutForProgram wrapper
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/runtime/builder.go — Removed unused handler-factory and BuildComponents wrapper APIs


## 2026-05-20

Wired command TUI chat mode to chatapp/sessionstream, added multiturn backend, and validated TAB submission in tmux with gpt-5 profiles

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/chatapp/runtime_inference.go — Fallback assistant text publishes only current-run assistant blocks
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/cmd.go — Command chat mode now uses chatapp runner and TUI fanout
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/chatapp_backend.go — Chatapp-backed Bubble Tea backend with multiturn snapshot reconstruction
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/fanout_proxy.go — Proxy fanout for Bubble Tea program construction order
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/runtime/builder.go — Removed unused transitional raw-handler builder


## 2026-05-20

Removed leftover unused command TUI profile-switch/seed helper files after migrating command chat mode

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/profile_switch_events.go — Removed unused profile-switch event helpers
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/cmds/seed_emit.go — Removed unused raw seed emission helpers


## 2026-05-20

Removed switch-profiles-tui, profileswitch package, raw simple-chat TUI backend/forwarder, and related scripts/docs references

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/switch-profiles-tui — Removed standalone switch-profiles TUI command
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/backend.go — Removed raw simple-chat TUI backend and StepChatForwardFunc
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/profileswitch — Removed runtime TUI profile-switch package
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/scripts — Removed switch-profile TUI smoke scripts


## 2026-05-20

Phase 8: added RPC JSONL help page, removed StepTimelinePersistFunc raw persistence helper, and ran final targeted validation/schema-vet

### Related Files

- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/cmd/pinocchio/doc/general/06-rpc-jsonl-output.md — User-facing RPC JSONL help page
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/timeline_persist.go — Removed raw UI-topic persistence helper
- /home/manuel/workspaces/2026-05-20/pinocchio-structured-data-cli/pinocchio/pkg/ui/timeline_persist_test.go — Removed tests for deleted raw persistence helper


## 2026-05-20

Committed and pushed Phase 8 docs/cleanup as c4af742 docs: finish RPC JSONL phase eight

