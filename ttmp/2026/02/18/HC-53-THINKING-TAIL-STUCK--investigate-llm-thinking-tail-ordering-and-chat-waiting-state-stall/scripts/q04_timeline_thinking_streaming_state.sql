.mode column
.headers on

SELECT
  idx,
  event_id,
  seq,
  message_role AS role,
  message_streaming AS streaming,
  message_content_len AS content_len
FROM event_log_entries
WHERE event_type = 'timeline.upsert'
  AND event_id LIKE '%:thinking'
ORDER BY idx DESC
LIMIT 20;
