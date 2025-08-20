## Proposal: REPL Slash Commands for Debugging simple-chat-agent (Turn/Run/Blocks)

Audience: Developers using the `simple-chat-agent` who want fast, in-REPL visibility into what was sent to and received from the provider, the current agent mode, tools executed, and raw Turn/Block payloads captured by the SQLite store.

References
- REPL component: `bobatea/docs/repl.md`
- Debugging store: `pinocchio/cmd/agents/simple-chat-agent/pkg/store/sqlstore.go`
- Existing debugging guide: `pinocchio/ttmp/2025-08-10/09-debugging-the-simple-chat-agent-with-the-turn-run-block-sqlite-store.md`

### Goals
- Provide `/` commands in the REPL to inspect recent Runs/Turns, blocks (including tool_call/tool_use), and event logs without leaving the app.
- Surface the same insights we currently get via ad-hoc SQL, but with curated, readable output.
- Keep the system reactive: run these queries against the live `simple-agent.db` in read-only mode.

### Command Catalog

- `/dbg help`: Show available debug commands and short descriptions.
- `/dbg runs [N]`: List last N runs with creation time and metadata presence. Default N=10.
- `/dbg turns [run_id] [N]`: List last N turns for a given run (or latest run if omitted): `turn_id`, timestamps.
- `/dbg last-turn`: Show the last turn id and key facts (mode, number of blocks, presence of tool calls/uses).
- `/dbg blocks [turn_id]`: Pretty list all blocks of a turn with kind/role and short payload previews.
- `/dbg toolcalls [turn_id]`: Show tool_call and tool_use pairs for a turn: id, name, args, result (truncated).
- `/dbg events [N]`: Show last N entries from `v_recent_events` (or directly `chat_events`) with type and message.
- `/dbg mode [turn_id]`: Show `v_turn_modes` alignment for a turn (Turn.Data agent_mode vs injected metadata).
- `/dbg injected-mode-prompts [N]`: Show last N entries of `v_injected_mode_prompts` (truncated text).
- `/dbg schema`: Print the current application DB schema as seen by the sqlite tool middleware (via `sqlite_master`, excluding `_prompts`).
- `/dbg prompts`: Print `_prompts` entries from the transaction DB (first 120 chars each).

Notes
- All textual payloads should be truncated to reasonable lengths (e.g., 160 chars) to keep the REPL readable.
- When a parameter is omitted, use sensible defaults (latest run/turn; N=10).

### UX and Output
- Commands return formatted plain text suitable for REPL display (no TUI tables needed).
- Prefer compact lines, one entry per line; indent wrapped lines.
- For JSON payloads, optionally pretty-print a compact single-line preview.

### Architecture and Integration

- Registration
  - Add commands immediately after REPL model creation in `pinocchio/cmd/agents/simple-chat-agent/main.go` where we already construct `replModel := repl.NewModel(evaluator, replCfg)`.
  - Use `replModel.AddCustomCommand(name, handler)` as described in `repl.md`.

- Dependencies
  - Reuse the existing `*storepkg.SQLiteStore` (variable `snapshotStore`) or open a read-only handle to `pinocchio/simple-agent.db` when executing commands.
  - For schema/prompts, read from the transaction DB (`pinocchio/anonymized-data.db`) in read-only mode.

- Safety and Concurrency
  - Use read-only queries.
  - Store writes are only performed by the agent flow; debug commands must never mutate state.

### Implementation Sketch

1) Wiring (after creating `replModel`)

```go
// main.go (after replModel := repl.NewModel(...))
registerDebugCommands(replModel, snapshotStore)
```

2) Command registration

```go
// file: pinocchio/cmd/agents/simple-chat-agent/pkg/ui/debug_commands.go
func registerDebugCommands(m *repl.Model, s *store.SQLiteStore) {
  m.AddCustomCommand("dbg", func(args []string) tea.Cmd {
    return func() tea.Msg {
      out, err := runDebugCommand(args, s)
      return repl.EvaluationCompleteMsg{Input: "/dbg " + strings.Join(args, " "), Output: out, Error: err}
    }
  })
}
```

3) Command router

```go
func runDebugCommand(args []string, s *store.SQLiteStore) (string, error) {
  if len(args) == 0 || args[0] == "help" { return helpText(), nil }
  switch args[0] {
  case "runs": return listRuns(s, takeN(args, 1, 10))
  case "turns": return listTurns(s, argOrEmpty(args, 1), takeN(args, 2, 10))
  case "last-turn": return showLastTurn(s)
  case "blocks": return showBlocks(s, mustArg(args, 1))
  case "toolcalls": return showToolCalls(s, mustArg(args, 1))
  case "events": return listEvents(s, takeN(args, 1, 20))
  case "mode": return showMode(s, mustArg(args, 1))
  case "injected-mode-prompts": return listInjectedPrompts(s, takeN(args, 1, 5))
  case "schema": return printAppSchema()
  case "prompts": return printAppPrompts()
  default:
    return "unknown: /dbg " + strings.Join(args, " "), nil
  }
}
```

4) SQL snippets (executed via the store’s `db` or ad-hoc read-only connections)

- Runs
```sql
SELECT id, created_at FROM runs ORDER BY created_at DESC LIMIT :N;
```

- Turns (per run)
```sql
SELECT id AS turn_id, created_at FROM turns WHERE run_id = :run_id ORDER BY created_at DESC LIMIT :N;
```

- Last turn id
```sql
SELECT turn_id FROM block_payload_kv ORDER BY rowid DESC LIMIT 1;
```

- Blocks of a turn (with previews)
```sql
WITH b AS (
  SELECT * FROM blocks WHERE turn_id = :turn_id ORDER BY ord
)
SELECT b.id, b.kind, b.role,
  (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) AS tool_name,
  (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id' LIMIT 1) AS tool_id,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='args' LIMIT 1) AS args,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='text' LIMIT 1) AS text
FROM b;
```

- Tool calls in a turn (args/result)
```sql
WITH b AS (
  SELECT * FROM blocks WHERE turn_id = :turn_id ORDER BY ord
)
SELECT b.id,
  (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) AS name,
  (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id' LIMIT 1) AS id,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='args' LIMIT 1) AS args,
  (SELECT COALESCE(value_text, value_json) FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='result' LIMIT 1) AS result
FROM b
WHERE (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='name' LIMIT 1) IS NOT NULL
   OR (SELECT value_text FROM block_payload_kv WHERE block_id=b.id AND turn_id=b.turn_id AND key='id' LIMIT 1) IS NOT NULL;
```

- Recent events
```sql
SELECT id, created_at, type, message, tool_name, tool_id FROM chat_events ORDER BY id DESC LIMIT :N;
```

- Mode alignment
```sql
SELECT * FROM v_turn_modes WHERE turn_id = :turn_id;
```

- Injected mode prompts
```sql
SELECT * FROM v_injected_mode_prompts ORDER BY rowid DESC LIMIT :N;
```

- App DB schema (exclude `_prompts`) and prompts (transaction DB)
```sql
-- schema
SELECT sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != '_prompts' ORDER BY name;
-- prompts
SELECT substr(prompt,1,120) FROM _prompts LIMIT 20;
```

### Data Sources
- `pinocchio/simple-agent.db`: All Turn/Block snapshots and `chat_events` (see `sqlstore.go`).
- `pinocchio/anonymized-data.db`: Application (transaction) DB for `sql_query` tool schema and `_prompts`.

### Error Handling
- If a query fails, display a short, single-line error and suggest `/dbg help`.
- For empty results, display “No records found”.

### Performance
- Cap lists with default limits; support an optional `[N]` parameter.
- Avoid loading full JSON snapshots unless explicitly requested in the future (could add `/dbg snapshot [turn_id] [phase]`).

### Security and Safety
- All commands must be read-only.
- Do not print personally identifying or sensitive payloads; truncate aggressively by default.

### Next Steps (TODO)
- [ ] Implement `registerDebugCommands` and `runDebugCommand` under `pkg/ui/`.
- [ ] Add read-only helpers to execute SQL against `simple-agent.db` and `anonymized-data.db`.
- [ ] Format outputs compactly (truncate 160 chars; indent wrapped lines).
- [ ] Add `/dbg snapshot [turn_id] [phase]` to print raw JSON snapshot (optional, gated behind a confirmation prompt or a `--full` flag).
- [ ] Add `/dbg tool <tool_id>` to aggregate all events for a specific tool call (uses `events.ToolEventAggregator`).
- [ ] Document commands in the in-app `/dbg help` and the debugging guide.


