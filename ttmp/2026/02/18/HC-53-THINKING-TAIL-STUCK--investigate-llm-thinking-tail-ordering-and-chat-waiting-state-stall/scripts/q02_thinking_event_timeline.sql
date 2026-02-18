.mode column
.headers on

SELECT
  idx,
  datetime(timestamp_ms / 1000.0, 'unixepoch') AS ts_utc,
  event_type,
  event_id,
  seq
FROM event_log_entries
WHERE event_type LIKE 'llm.thinking.%'
ORDER BY idx;
