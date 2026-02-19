.mode column
.headers on

WITH last_final AS (
  SELECT MAX(idx) AS final_idx
  FROM event_log_entries
  WHERE event_type = 'llm.thinking.final'
)
SELECT
  e.idx,
  e.event_type,
  e.event_id,
  e.seq,
  datetime(e.timestamp_ms / 1000.0, 'unixepoch') AS ts_utc
FROM event_log_entries e
JOIN last_final lf ON e.idx > lf.final_idx
ORDER BY e.idx;
