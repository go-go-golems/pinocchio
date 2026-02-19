.mode column
.headers on

WITH finals AS (
  SELECT
    idx AS final_idx,
    event_id,
    seq AS final_seq,
    timestamp_ms AS final_ts
  FROM event_log_entries
  WHERE event_type = 'llm.thinking.final'
),
post AS (
  SELECT
    f.final_idx,
    f.event_id AS thinking_id,
    f.final_seq,
    f.final_ts,
    (
      SELECT e2.idx
      FROM event_log_entries e2
      WHERE e2.idx > f.final_idx
        AND e2.event_type = 'llm.thinking.delta'
        AND e2.event_id = f.event_id
      ORDER BY e2.idx
      LIMIT 1
    ) AS post_delta_idx
  FROM finals f
)
SELECT
  p.final_idx,
  p.thinking_id,
  p.final_seq,
  datetime(p.final_ts / 1000.0, 'unixepoch') AS final_ts_utc,
  e.idx AS post_delta_idx,
  e.seq AS post_delta_seq,
  datetime(e.timestamp_ms / 1000.0, 'unixepoch') AS post_delta_ts_utc
FROM post p
JOIN event_log_entries e ON e.idx = p.post_delta_idx
ORDER BY p.final_idx;
