# Changelog

## 2026-02-14

- Initial workspace created

## 2026-02-15

Simplified GP-021 design and task plan to a minimal first implementation: channels-only websocket subscriptions, global conversation `seq` (with filtered-client gaps allowed), and explicit deferral of turn-snapshot websocket streaming.

### Related Files

- pinocchio/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/design/01-websocket-broadcast-refactor-analysis-brainstorm-and-design.md — Removed `ws_profile`/typed debug-option complexity and clarified channel-only defaults
- pinocchio/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/tasks.md — Updated task list to channel-only filtering and explicit follow-up deferral for `debug.turn_snapshot`


## 2026-02-14

Added deep websocket broadcast architecture analysis, experiments, and refactor design with phased migration plan.

### Related Files

- geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/design/01-websocket-broadcast-refactor-analysis-brainstorm-and-design.md — Primary 5+ page design/brainstorm artifact
- geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/reference/01-diary.md — Diary with experiment and decision trail
- geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/scripts/01-trace-ws-broadcast-paths.sh — Experiment script
- geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/scripts/02-inventory-ws-protocol-surface.sh — Experiment script
- geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/scripts/03-hookability-audit.sh — Experiment script


## 2026-02-14

Updated design doc with explicit sink->stream->projector dataflow, ownership/reference map, and two-signal turn snapshot gating (producer intent + consumer subscription).

### Related Files

- geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/design/01-websocket-broadcast-refactor-analysis-brainstorm-and-design.md — Added sections 2.6/2.7/9/10 and updated migration/testing tasks


## 2026-02-14

Refreshed GP-021 design + inventory scripts after webchat runtime cleanup: replaced stale BuildEngineFromReq/profile assumptions with ConversationRequestResolver + RuntimeComposer + runtime query flow, and refined snapshot channel section toward persist-first reference events.

### Related Files

- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/design/01-websocket-broadcast-refactor-analysis-brainstorm-and-design.md — Updated architecture text and snapshot emission design
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/scripts/02-inventory-ws-protocol-surface.sh — Updated grep probes for resolver/runtime path
- /home/manuel/workspaces/2026-02-13/mv-debug-ui-geppetto/geppetto/ttmp/2026/02/14/GP-021-WEBSOCKET-BROADCAST-REFACTOR--refactor-websocket-broadcast-architecture-and-extension-hooks/scripts/03-hookability-audit.sh — Updated hook audit probes for resolver/composer options
