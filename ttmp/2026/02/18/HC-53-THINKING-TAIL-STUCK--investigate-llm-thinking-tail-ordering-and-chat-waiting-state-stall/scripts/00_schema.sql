PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

DROP TABLE IF EXISTS event_log_entries;
DROP TABLE IF EXISTS gpt_log_lines;

CREATE TABLE event_log_entries (
  idx INTEGER PRIMARY KEY,
  entry_id TEXT,
  timestamp_ms INTEGER,
  event_type TEXT,
  event_id TEXT,
  family TEXT,
  summary TEXT,
  sem INTEGER,
  seq TEXT,
  stream_id TEXT,
  conv_id TEXT,
  message_role TEXT,
  message_streaming INTEGER,
  message_content_len INTEGER,
  raw_payload_json TEXT,
  event_data_json TEXT,
  event_metadata_json TEXT
);

CREATE INDEX idx_event_log_entries_event_type ON event_log_entries(event_type);
CREATE INDEX idx_event_log_entries_event_id ON event_log_entries(event_id);
CREATE INDEX idx_event_log_entries_timestamp ON event_log_entries(timestamp_ms);

CREATE TABLE gpt_log_lines (
  line_no INTEGER PRIMARY KEY,
  ts_iso TEXT,
  level TEXT,
  module_ref TEXT,
  message TEXT,
  kv_json TEXT,
  raw_line TEXT
);

CREATE INDEX idx_gpt_log_lines_ts ON gpt_log_lines(ts_iso);
CREATE INDEX idx_gpt_log_lines_level ON gpt_log_lines(level);
