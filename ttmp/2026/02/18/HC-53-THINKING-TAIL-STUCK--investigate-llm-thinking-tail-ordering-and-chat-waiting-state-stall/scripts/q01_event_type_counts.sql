.mode column
.headers on

SELECT
  event_type,
  COUNT(*) AS n
FROM event_log_entries
GROUP BY event_type
ORDER BY n DESC, event_type;
