# Changelog

## 2026-11-21

- Initial workspace created

## 2026-11-21

Ticket created; intern design guide written (current system, mechanism-vs-content rule, target design, phases, decisions)

### Related Files

- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/ttmp/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP--merge-agentd-into-chatapp-one-agent-runtime-mechanism-in-the-core-content-left-to-the-caller/design-doc/01-intern-guide-merging-agentd-into-chatapp-one-agent-runtime-mechanism-in-the-core.md — the guide

## 2026-11-21

Phase 1 (0ac7b5e): chatapp runs are mechanism only — ComposedRuntime Loop/ToolConfig/RoundGuard, busy policy, outcome codes, PromptRequest.Blocks, every run's turn kept (unanswered calls closed), chatapp content defaults and demo removed

### Related Files

- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/run_outcome.go — outcome decision, classifier, closing unanswered calls
- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/runtime_inference.go — run lifecycle
