-- Schema for simple-agent debug storage
-- Tables capture runs, turns, blocks, snapshots and chat events.

-- Runs
CREATE TABLE IF NOT EXISTS runs(
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  metadata TEXT
);

-- Per-run metadata kv for fast querying
CREATE TABLE IF NOT EXISTS run_metadata_kv(
  run_id TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT,
  value_text TEXT,
  value_json TEXT,
  PRIMARY KEY (run_id, key),
  FOREIGN KEY(run_id) REFERENCES runs(id)
);

-- Turns
CREATE TABLE IF NOT EXISTS turns(
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  metadata TEXT,
  FOREIGN KEY(run_id) REFERENCES runs(id)
);

-- Turn kv (section is 'metadata' or 'data')
CREATE TABLE IF NOT EXISTS turn_kv(
  turn_id TEXT NOT NULL,
  section TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT,
  value_text TEXT,
  value_json TEXT,
  PRIMARY KEY (turn_id, section, key),
  FOREIGN KEY(turn_id) REFERENCES turns(id)
);

-- Blocks (logical units within a turn)
CREATE TABLE IF NOT EXISTS blocks(
  id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  ord INTEGER NOT NULL,
  kind INTEGER NOT NULL,
  role TEXT,
  created_at TEXT NOT NULL,
  PRIMARY KEY (id, turn_id),
  FOREIGN KEY(turn_id) REFERENCES turns(id)
);

-- Block payload kv (phase is 'pre' or 'post')
CREATE TABLE IF NOT EXISTS block_payload_kv(
  block_id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  phase TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT,
  value_text TEXT,
  value_json TEXT,
  PRIMARY KEY (block_id, turn_id, phase, key),
  FOREIGN KEY(turn_id) REFERENCES turns(id)
);

-- Block metadata kv
CREATE TABLE IF NOT EXISTS block_metadata_kv(
  block_id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  phase TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT,
  value_text TEXT,
  value_json TEXT,
  PRIMARY KEY (block_id, turn_id, phase, key),
  FOREIGN KEY(turn_id) REFERENCES turns(id)
);

-- Full JSON snapshots for convenience
CREATE TABLE IF NOT EXISTS turn_snapshots(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  turn_id TEXT NOT NULL,
  phase TEXT NOT NULL,
  created_at TEXT NOT NULL,
  data TEXT NOT NULL,
  FOREIGN KEY(turn_id) REFERENCES turns(id)
);

-- Event log (tool calls, results, info/log)
CREATE TABLE IF NOT EXISTS chat_events(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT NOT NULL,
  type TEXT NOT NULL,
  message TEXT,
  level TEXT,
  tool_name TEXT,
  tool_id TEXT,
  input TEXT,
  result TEXT,
  data_json TEXT,
  payload_json TEXT,
  run_id TEXT,
  turn_id TEXT
);

-- Tool registry snapshots per turn/phase (debugging aid)
CREATE TABLE IF NOT EXISTS tool_registry_snapshots(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  turn_id TEXT NOT NULL,
  phase TEXT NOT NULL,
  created_at TEXT NOT NULL,
  tools_json TEXT NOT NULL
);

-- Optional prompts table read by sqlitetool middleware (if present)
CREATE TABLE IF NOT EXISTS _prompts (
  prompt TEXT NOT NULL
);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS idx_turn_kv_key ON turn_kv(turn_id, section, key);
CREATE INDEX IF NOT EXISTS idx_bpk_key ON block_payload_kv(turn_id, phase, key);
CREATE INDEX IF NOT EXISTS idx_bmk_key ON block_metadata_kv(turn_id, phase, key);
CREATE INDEX IF NOT EXISTS idx_events_time ON chat_events(type, created_at);


