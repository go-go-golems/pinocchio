# Changelog

## 2025-12-18

- Initial workspace created


## 2025-12-18

Fixed all typecheck linting errors related to typed map keys (TurnDataKey, TurnMetadataKey, BlockMetadataKey). Added conversion helpers in sqlstore.go and fixed type usage across multiple files.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go — Fixed types
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/agents/simple-chat-agent/pkg/store/sqlstore.go — Added conversion helpers
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/examples/simple-chat/main.go — Fixed types
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/pkg/middlewares/sqlitetool/middleware.go — Fixed types
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/pkg/webchat/conversation.go — Fixed types
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/pkg/webchat/router.go — Fixed types


## 2025-12-18

Created analysis document explaining turnsdatalint const-only key enforcement and options to fix remaining violations (Option A: direct access recommended)

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/ttmp/2025/12/18/001-FIX-KEY-TAG-LINTING--fix-key-tag-linting-errors/analysis/01-turnsdatalint-why-dynamic-keys-conversions-fail-options-to-fix-pinocchio-geppetto.md — Analysis document


## 2025-12-18

Updated analysis document: propose relaxing turnsdatalint from const-only to typed-key enforcement (allows conversions/variables/parameters while still rejecting raw string literals)

## 2025-12-18

Fixed compilation breakage in the simple-chat agent backend caused by an incomplete `turn` → `Turn` field rename (`b.turn` references left behind). Also updated the one remaining call site to set initial turn data via `backend.Turn.Data[...]` directly (const key), matching the intended “direct access” workaround.

### Related Files

- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go — Replace `b.turn` with `b.Turn` and update tool loop turn pointer handling (commit `ee8bf085a1cefea9a73eeceadf4afe9aae453668`)
- /home/manuel/workspaces/2025-11-18/fix-pinocchio-profiles/pinocchio/cmd/agents/simple-chat-agent/main.go — Set initial server tools data via `backend.Turn.Data[turns.DataKeyResponsesServerTools]` (commit `ee8bf085a1cefea9a73eeceadf4afe9aae453668`)


## 2026-02-14

Ticket closed

