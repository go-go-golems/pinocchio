## Debugging the simple-chat-agent with the Turn/Run/Block SQLite store

This guide explains how to use the embedded SQLite store to inspect and debug what the simple-chat-agent sent to (and received from) the provider, how agent modes and tools behaved, and how to answer questions like "what was the exact system-like prompt?" or "why did the mode switch not apply?".

The store schema, ingestion code, and views live alongside the agent.

- Store code: `pinocchio/cmd/agents/simple-chat-agent/pkg/store/sqlstore.go`
- Schema (embedded): `pinocchio/cmd/agents/simple-chat-agent/pkg/store/schema.sql`
- Views (embedded): `pinocchio/cmd/agents/simple-chat-agent/pkg/store/views.sql`

At runtime the agent writes to `pinocchio/simple-agent.db`.

### What gets stored

The store captures both full snapshots and query-friendly key-value rows:

- Full JSON snapshots per Turn and phase:
  - Table `turn_snapshots(turn_id, phase, created_at, data)`
  - Phases captured by the current agent wiring:
    - `pre_middleware` (before middleware chain)
    - Tool loop phases injected via hooks (from `toolhelpers`): `pre_inference`, `post_inference`, `post_tools`
    - `post_middleware` (after tool execution)

- Normalized tables for fast filtering and joins:
  - `runs` and `run_metadata_kv`
  - `turns` and `turn_kv` with `section` in (`metadata`,`data`)
  - `blocks` and `block_payload_kv`, `block_metadata_kv` keyed by `(block_id, turn_id, phase)`
  - `chat_events` for tool/log/info events with message payloads, plus `run_id`, `turn_id`
  - `tool_registry_snapshots(run_id, turn_id, phase, created_at, tools_json)` capturing the tool registry JSON per turn/phase (debugging aid)

Notes
- All blocks have stable IDs. Constructors assign IDs, and persistence warns and generates an ID if any block is missing one, then writes it back to the Turn for stability across phases.
- In the REPL, “latest” default for blocks is `post_inference`.

### How data is written

The store is invoked from the agent main:

- Snapshots surround the engine call:
  - `SaveTurnSnapshot(ctx, t, "pre_middleware")` before middleware/inference
  - Tool loop hook inside `toolhelpers` captures `pre_inference`, `post_inference`, `post_tools`
  - `SaveTurnSnapshot(ctx, res, "post_middleware")` after tool execution
  - Tool registry JSON is persisted into `turn_kv(data/tool_registry)` and `tool_registry_snapshots`

- Event logging handler:
  - A Watermill router handler calls `store.LogEvent(ctx, e)` for tool calls/results and Info/Log events.

- Tool execution and re-inference:
  - The sqlite tool middleware registers the `sql_query` tool (schema + executor) in the per-turn registry but no longer executes it inline.
  - The standard tool loop (RunToolCallingLoop) detects `tool_call` blocks, executes tools via the registry, appends `tool_use`, then triggers a new inference automatically until done.

### Quickstart: open the database

From the workspace root:

```bash
sqlite3 pinocchio/simple-agent.db
```

List tables and views:

```sql
.tables
.schema v_recent_events
```

### Frequently useful queries

1) What agent mode did we record for each turn? Did we inject the right prompt?

```sql
SELECT * FROM v_turn_modes ORDER BY turn_id;
```

This view shows, per `turn_id`:
- `data_mode`: value of `Turn.Data['agent_mode']`
- `injected_mode`: metadata `agentmode` read from the injected agent-mode block (post phase)

2) Show the actual injected agent-mode prompt text

```sql
SELECT * FROM v_injected_mode_prompts;
```

This returns the full text of the block tagged with `agentmode_tag = agentmode_user_prompt` (post phase). If your mode prompt was wrapped in tags like `<currentMode>...</currentMode>` and `<modeSwitchGuidelines>...</modeSwitchGuidelines>`, you’ll see them here.

3) Recent event feed (tools, info/log)

```sql
SELECT * FROM v_recent_events LIMIT 50;
```

Use this to correlate tool calls and results with `run_id`/`turn_id`, and to see agent-mode Info events (e.g., "agentmode: mode switched").

4) Tool lifecycle by tool_call_id

```sql
SELECT * FROM v_tool_activity ORDER BY last_seen DESC;
5) Inspect sql_query tool calls and results (args/result captured on blocks)

```sql
-- Find recent tool_call/tool_use blocks and show their args/result
WITH recent_turn AS (
  SELECT DISTINCT turn_id FROM block_payload_kv ORDER BY rowid DESC LIMIT 1
), bl AS (
  SELECT b.* FROM blocks b JOIN recent_turn r ON b.turn_id = r.turn_id ORDER BY ord
)
SELECT bl.id, bl.kind, bl.role,
  (SELECT value_text FROM block_payload_kv WHERE block_id=bl.id AND turn_id=bl.turn_id AND key='name' LIMIT 1)  AS tool_name,
  (SELECT value_text FROM block_payload_kv WHERE block_id=bl.id AND turn_id=bl.turn_id AND key='id' LIMIT 1)    AS tool_id,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=bl.id AND turn_id=bl.turn_id AND key='args' LIMIT 1)   AS args,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=bl.id AND turn_id=bl.turn_id AND key='result' LIMIT 1) AS result,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=bl.id AND turn_id=bl.turn_id AND key='text' LIMIT 1)   AS text
FROM bl
WHERE bl.kind IN (2,3,4); -- tool_call/tool_use/llm_text depending on enum values
```

Notes:
- The tool loop writes the tool_use `result` into block payloads. Depending on provider settings, tool result events may or may not be emitted separately in `chat_events`; prefer reading block payloads for ground truth.

6) Verify that the sql_query tool description includes schema and prompts

The middleware precomputes a tool description at startup by:
- Dumping schema via `sqlite_master` (excluding `_prompts`)
- Appending lines from `_prompts(prompt TEXT)` if present

This description is sent to the provider as part of the tool specification but is not stored as a conversation block. To verify:
- Check your application DB (e.g., `pinocchio/anonymized-data.db`) contains `_prompts` rows and expected tables:

```bash
sqlite3 pinocchio/anonymized-data.db "SELECT name FROM sqlite_master WHERE type='table' ORDER BY 1; SELECT COUNT(*) FROM _prompts;"
```

- Confirm early `sql_query` calls succeed and that `result` in block payload lists tables (e.g., `SELECT name FROM sqlite_master WHERE type='table';`).
- For deeper inspection, temporarily enable debug logging around middleware initialization to print the composed tool description.

```

This aggregates the phases for each `tool_id` and shows last result.

7) List tool registry snapshots for a turn

```sql
SELECT phase, created_at, json_array_length(tools_json) AS tool_count
FROM tool_registry_snapshots
WHERE turn_id = :turn_id
ORDER BY id;
```

8) Inspect “latest” (post_inference) blocks for a turn with args/result/text

```sql
WITH b AS (
  SELECT * FROM blocks WHERE turn_id = :turn_id AND EXISTS (
    SELECT 1 FROM block_payload_kv kv WHERE kv.block_id = blocks.id AND kv.turn_id = blocks.turn_id AND kv.phase = 'post_inference'
  ) ORDER BY ord
)
SELECT b.id, b.ord, b.kind, b.role,
  (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name'   AND phase='post_inference' LIMIT 1) AS tool_name,
  (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id'     AND phase='post_inference' LIMIT 1) AS tool_id,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='args'   AND phase='post_inference' LIMIT 1) AS args,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='result' AND phase='post_inference' LIMIT 1) AS result,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='text'   AND phase='post_inference' LIMIT 1) AS text
FROM b;
```

5) Inspect provider-visible prompt text (manual join)

If you need to manually inspect blocks rather than using views:

```sql
-- Find agent-mode prompt blocks (post) and display the raw text payload
SELECT bpk.turn_id, bpk.phase, COALESCE(bpk.value_text, bpk.value_json) AS text
FROM block_payload_kv bpk
JOIN block_metadata_kv bmk
  ON bpk.block_id=bmk.block_id AND bpk.turn_id=bmk.turn_id AND bpk.phase=bmk.phase
WHERE bmk.key='agentmode_tag' AND bmk.value_text='agentmode_user_prompt' AND bpk.key='text'
ORDER BY bpk.rowid;
```

6) Validate RunID/TurnID propagation

```sql
SELECT run_id, turn_id, type, created_at, message
FROM chat_events
ORDER BY id DESC LIMIT 50;
```

You should see a stable `run_id` across the session and a `turn_id` for each inference call.

### Troubleshooting patterns

- Mode resets to default each turn:
  - Check stable RunID: `SELECT DISTINCT run_id FROM chat_events;` If missing or empty, ensure the middleware in `main.go` sets `t.RunID` before inference.
  - Confirm `v_turn_modes` shows the last `data_mode` advancing after a switch.

- Mode switch proposed but not applied:
  - Inspect the last assistant response for YAML and the `agentmode: mode switched` Info event in `v_recent_events`.
  - Verify that the proposed `new_mode` exists in the AgentMode service.

- 400 error about missing tool results:
  - Indicates user input happened while a tool loop was still pending. Gate input while streaming/loop active.

### Schema reference (summary)

Key tables (see `schema.sql` for full DDL):

- `turn_snapshots`: full JSON per phase (`pre`, `post`)
- `turn_kv`: Turn-level KV entries (sections `metadata` or `data`)
- `block_payload_kv`, `block_metadata_kv`: Block-level KV, keyed by `block_id`, `turn_id`, `phase`
- `chat_events`: event stream with `type`, `message`, `tool_name/tool_id/input/result`, and `run_id/turn_id`

### Where ingestion happens in code

- `SaveTurnSnapshot(ctx, t, phase)` in [`sqlstore.go`](../../cmd/agents/simple-chat-agent/pkg/store/sqlstore.go):
  - Ensures `runs` and `turns` exist, stores blocks and their payload/metadata KV per-phase, writes a JSON snapshot, and persists tool registry JSON both in `turn_kv` and `tool_registry_snapshots`.
  - Warns and assigns a new UUID when encountering a block without an ID; the ID is written back to the Turn so later snapshots remain consistent.

- `LogEvent(ctx, e)` in [`sqlstore.go`](../../cmd/agents/simple-chat-agent/pkg/store/sqlstore.go):
  - Serializes tool-call/execute/results and Info/Log events into `chat_events`, preserving raw payload and `run_id/turn_id` from `EventMetadata`.

### Best practices

- Always set stable `RunID` and non-empty `TurnID` prior to inference. This ensures agent-mode persistence and unambiguous debugging.
- Prefer the views for common questions; drop to KV joins for precise payload inspection.
- Use the REPL `/dbg` commands for quick in-app inspection:
  - `/dbg blocks [turn_id] [--phase PHASE] [-v] [--head N|--tail N]`
    - Default phase is `post_inference`. With `-v` it prints the SELECT and ARGS, `turn_id`, `phase`, and shows `tool_call_id` for tool_use.
  - `/dbg tools [turn_id]` lists tool names or prints snapshots; with fallback it shows the exact SELECT used.
- When adding new middlewares, consider adding extra snapshot phases (e.g., `pre_inference`, `post_tools`) to refine visibility.

### Appendix: handy one-liners

```bash
# Tails recent events with run/turn
sqlite3 -readonly pinocchio/simple-agent.db "SELECT id, created_at, type, run_id, turn_id, message FROM chat_events ORDER BY id DESC LIMIT 100;"

# Show last injected mode prompts
sqlite3 -readonly pinocchio/simple-agent.db "SELECT * FROM v_injected_mode_prompts ORDER BY turn_id;"

# Show turn mode alignment (Turn.Data vs injected)
sqlite3 -readonly pinocchio/simple-agent.db "SELECT * FROM v_turn_modes ORDER BY turn_id;"
```

