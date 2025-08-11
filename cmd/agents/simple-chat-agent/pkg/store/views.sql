-- Debugging Views for simple-agent
--
-- This file defines virtual views to quickly inspect the most relevant
-- information while debugging. All views are read-only.

-- View: v_turn_modes
-- Purpose: Show, per turn, the agent mode recorded in Turn.Data, and the
--          injected agent-mode prompt metadata captured on the mode block.
-- Columns:
--   turn_id: Turn identifier
--   data_mode: Value of Turn.Data['agent_mode'] if present
--   injected_mode: Metadata 'agentmode' taken from the injected agent mode user block (post phase)
CREATE VIEW IF NOT EXISTS v_turn_modes AS
SELECT
  t.id AS turn_id,
  MAX(CASE WHEN tk.section='data' AND tk.key='agent_mode' THEN COALESCE(tk.value_text, tk.value_json) END) AS data_mode,
  MAX(CASE WHEN bmk.key='agentmode' AND bmk.phase='post' THEN COALESCE(bmk.value_text, bmk.value_json) END) AS injected_mode
FROM turns t
LEFT JOIN turn_kv tk ON tk.turn_id = t.id
LEFT JOIN block_metadata_kv bmk ON bmk.turn_id = t.id
GROUP BY t.id;

-- View: v_injected_mode_prompts
-- Purpose: Return the actual injected mode block text for each turn.
-- Columns:
--   turn_id, prompt_text
CREATE VIEW IF NOT EXISTS v_injected_mode_prompts AS
SELECT bpk.turn_id AS turn_id,
       MAX(COALESCE(bpk.value_text, bpk.value_json)) AS prompt_text
FROM block_payload_kv bpk
JOIN block_metadata_kv bmk
  ON bpk.block_id=bmk.block_id AND bpk.turn_id=bmk.turn_id AND bpk.phase=bmk.phase
WHERE bmk.key='agentmode_tag' AND bmk.value_text='agentmode_user_prompt' AND bpk.key='text'
GROUP BY bpk.turn_id;

-- View: v_recent_events
-- Purpose: Latest N events with useful fields for quick inspection.
-- Note: Limit in SELECT as needed.
CREATE VIEW IF NOT EXISTS v_recent_events AS
SELECT id, created_at, type, level, message, tool_name, tool_id, input, result, run_id, turn_id
FROM chat_events
ORDER BY id DESC;

-- View: v_tool_activity
-- Purpose: Aggregate tool lifecycle by tool_id within a turn.
CREATE VIEW IF NOT EXISTS v_tool_activity AS
SELECT
  tool_id,
  MIN(created_at) AS first_seen,
  MAX(created_at) AS last_seen,
  MIN(CASE WHEN type='tool-call' THEN 1 ELSE NULL END) AS called,
  MIN(CASE WHEN type='tool-call-execute' THEN 1 ELSE NULL END) AS executed,
  MAX(CASE WHEN type IN ('tool-result','tool-call-execution-result') THEN result END) AS last_result,
  MAX(run_id) AS run_id,
  MAX(turn_id) AS turn_id
FROM chat_events
WHERE tool_id IS NOT NULL AND tool_id <> ''
GROUP BY tool_id;

-- View: v_turns_with_modes
-- Purpose: Summary per turn with run_id, mode from Turn.Data, injected mode, and last mode Info event
CREATE VIEW IF NOT EXISTS v_turns_with_modes AS
SELECT
  t.run_id,
  t.id AS turn_id,
  MAX(CASE WHEN tk.section='data' AND tk.key='agent_mode' THEN COALESCE(tk.value_text, tk.value_json) END) AS data_mode,
  MAX(CASE WHEN bmk.key='agentmode' AND bmk.phase='post' THEN COALESCE(bmk.value_text, bmk.value_json) END) AS injected_mode,
  (
    SELECT e.message FROM chat_events e
    WHERE e.turn_id = t.id AND e.type = 'info' AND e.message LIKE 'agentmode:%'
    ORDER BY e.id DESC LIMIT 1
  ) AS last_mode_event
FROM turns t
LEFT JOIN turn_kv tk ON tk.turn_id = t.id
LEFT JOIN block_metadata_kv bmk ON bmk.turn_id = t.id
GROUP BY t.id;

-- View: v_provider_messages
-- Purpose: Extract user and assistant texts per turn from block_payload_kv
CREATE VIEW IF NOT EXISTS v_provider_messages AS
SELECT
  t.id AS turn_id,
  MAX(CASE WHEN b.kind=0 AND bpk.key='text' THEN COALESCE(bpk.value_text, bpk.value_json) END) AS user_text,
  MAX(CASE WHEN b.kind=1 AND bpk.key='text' THEN COALESCE(bpk.value_text, bpk.value_json) END) AS assistant_text
FROM turns t
LEFT JOIN blocks b ON b.turn_id = t.id
LEFT JOIN block_payload_kv bpk ON bpk.block_id = b.id AND bpk.turn_id = b.turn_id AND bpk.phase='post'
GROUP BY t.id;


