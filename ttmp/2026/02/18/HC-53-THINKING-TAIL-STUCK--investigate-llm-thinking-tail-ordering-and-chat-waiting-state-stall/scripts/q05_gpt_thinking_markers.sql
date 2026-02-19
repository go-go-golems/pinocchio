.mode column
.headers on

SELECT
  line_no,
  ts_iso,
  level,
  module_ref,
  message,
  kv_json
FROM gpt_log_lines
WHERE raw_line LIKE '%thinking-ended%'
   OR raw_line LIKE '%reasoning-summary%'
   OR raw_line LIKE '%EventThinkingPartial%'
ORDER BY line_no;
