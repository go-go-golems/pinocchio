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

## 2026-11-22

Phase 2 (057330b): chatapp/sessions (Info, Store, memory and SQLite stores) and chatapp.WithUsageObserver + sessions.RecordUsage (replaces the guide's NewUsagePlugin)

### Related Files

- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/sessions/sessions.go — session metadata types and Store

## 2026-11-22

Phase 3 (9a08f24): pkg/chatapp/httpapi replaces web-chat's appserver: session routes, 409 run_active, stop, SSE transport, auth hooks, Running; serverkit trimmed to snapshot types

### Related Files

- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/httpapi/server.go — server options, route table, transports fanout
- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/httpapi/sse.go — SSE hydration buffer and live delivery

## 2026-11-22

Phase 4 (11da2fa, 7cbef6f): chatapp.WithRunTools + frontendtools.Manager.RunTools (runs get the page's browser tools); frontendtools results are google.protobuf.Value; toolkit/toolexec, toolkit/budget, toolkit/presets

### Related Files

- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/frontendtools/bridge.go — Manager.RunTools wires browser tools into runs
- /home/manuel/workspaces/2026-10-28/slop-computer/pinocchio/pkg/chatapp/toolkit/toolexec/toolexec.go — deadline and result-cut middlewares
