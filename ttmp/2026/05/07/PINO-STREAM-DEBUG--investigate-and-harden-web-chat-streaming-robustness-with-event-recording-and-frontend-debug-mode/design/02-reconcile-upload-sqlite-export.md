---
Title: Reconcile Upload SQLite Export Design
Ticket: PINO-STREAM-DEBUG
Status: active
Topics:
  - streaming-robustness
  - event-recording
  - frontend-debug
  - websocket
  - hydration
  - sessionstream
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
  - ../../../../cmd/web-chat/app/debug_recorder.go
  - ../../../../cmd/web-chat/app/server_debug.go
  - ../../../../cmd/web-chat/app/debug_reconcile_db.go
  - ../../../../cmd/web-chat/web/src/ws/streamDebug.ts
ExternalSources: []
Summary: Design for uploading frontend stream debug logs and returning a SQLite database containing backend and frontend debug records in schematized tables.
LastUpdated: 2026-05-07T00:00:00-04:00
WhatFor: Guide implementation of a reconcile/upload endpoint that packages raw and parsed debug evidence into a SQLite database for incremental analysis.
WhenToUse: Use when extending Pinocchio stream diagnostics beyond JSON endpoints into queryable/debuggable SQLite artifacts.
---

# Reconcile Upload SQLite Export Design

## Executive summary

The existing debug APIs expose backend records as JSON and the frontend debug overlay can export browser records as JSON. That is enough for manual inspection, but it is awkward for incremental analysis. Every new question requires writing a new ad-hoc JSON traversal.

This design adds a **reconcile/upload SQLite export endpoint**:

```text
POST /api/debug/sessions/{sessionId}/reconcile/upload
Content-Type: application/json
Accept: application/vnd.sqlite3
```

The request body contains frontend debug records exported by `pinocchio.debugStream`. The response is a downloadable `sqlite.db` file containing:

- backend Sessionstream observer records already held by the Pinocchio debug recorder;
- uploaded frontend records;
- parsed/indexed tables for common fields such as ordinals, frame types, event names, connection IDs, snapshot entity counts, mutations, and lifecycle events;
- raw JSON for every record so future analysis can be added without losing data.

This endpoint does **not** attempt to solve all reconciliation logic in the first pass. It only parses logs and stores them in an appropriate schema. Future passes can add views, analysis tables, and derived reports without changing the upload format.

## Why SQLite instead of another JSON response

A single debug session can contain thousands of records. JSON is fine for transport, but poor for iterative investigation. SQLite gives us:

- stable, inspectable tables;
- ad-hoc SQL queries with `sqlite3`, DuckDB, Datasette, or scripts;
- appendable schema evolution;
- indexes over ordinals, record types, event names, and connection IDs;
- ability to add derived views later without changing the raw evidence tables.

The goal is to make a debug bundle that an intern can download, open, and query:

```bash
sqlite3 stream-debug.sqlite
.tables
SELECT ordinal, name FROM frontend_ui_events ORDER BY ordinal;
SELECT * FROM backend_transport WHERE stage = 'fanout_no_targets';
```

## Data sources

The database contains two families of data.

### Backend records

Backend records come from `StreamDebugRecorder` in `cmd/web-chat/app/debug_recorder.go`. They are already normalized into `DebugRecord` values:

```text
DebugRecord
  kind = pipeline | transport
  timestamp
  sessionId
  connectionId
  ordinal
  pipeline  -> PipelineDebugRecord
  transport -> TransportDebugRecord
```

### Frontend records

Frontend records are uploaded from `cmd/web-chat/web/src/ws/streamDebug.ts`. They are intentionally flexible JSON records:

```text
raw-ws
parsed-frame
snapshot
ui-event
ws-lifecycle
```

The upload endpoint must accept both common shapes:

```json
{ "records": [ ... ] }
```

and:

```json
[ ... ]
```

This makes the endpoint easy to use from the browser panel, curl, or a local script.

## Endpoint contract

### Request

```text
POST /api/debug/sessions/{sessionId}/reconcile/upload
Content-Type: application/json
```

Body:

```json
{
  "records": [
    {
      "id": 1,
      "timestamp": 1770000000000,
      "type": "parsed-frame",
      "sessionId": "sess-1",
      "frameType": "ui-event",
      "ordinal": "42",
      "name": "ChatMessageAppended",
      "frame": { }
    }
  ]
}
```

### Response

```text
200 OK
Content-Type: application/vnd.sqlite3
Content-Disposition: attachment; filename="pinocchio-stream-debug-{sessionId}.sqlite"
```

Body: SQLite database bytes.

### Failure cases

| Status | Cause |
|--------|-------|
| `400` | Malformed JSON upload. |
| `404` | Debug API disabled or route not found. |
| `405` | Non-POST method for upload endpoint. |
| `500` | SQLite export construction failed. |

## Schema overview

The first schema version is intentionally simple: raw tables plus enough parsed columns for useful queries.

```text
meta
backend_records
backend_pipeline
backend_pipeline_ui_events
backend_pipeline_entities
backend_transport
backend_transport_snapshot_entities
frontend_records
frontend_raw_ws
frontend_parsed_frames
frontend_snapshots
frontend_snapshot_entities
frontend_ui_events
frontend_lifecycle
```

Every source record is stored as raw JSON. Parsed tables reference source records by ID.

## Schema detail

### `meta`

```sql
CREATE TABLE meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

Example keys:

- `schema_version`
- `session_id`
- `created_at`
- `backend_record_count`
- `frontend_record_count`

### `backend_records`

```sql
CREATE TABLE backend_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    ts TEXT,
    session_id TEXT,
    connection_id TEXT,
    ordinal INTEGER,
    raw_json TEXT NOT NULL
);
```

This is the root table for all backend records.

### `backend_pipeline`

```sql
CREATE TABLE backend_pipeline (
    record_id INTEGER PRIMARY KEY REFERENCES backend_records(id),
    mode TEXT,
    event_name TEXT,
    event_type TEXT,
    event_appended INTEGER,
    view_ordinal INTEGER,
    timeline_cursor_advanced INTEGER,
    append_error TEXT,
    session_error TEXT,
    view_error TEXT,
    ui_projection_error TEXT,
    timeline_projection_error TEXT,
    apply_error TEXT,
    cursor_error TEXT,
    fanout_error TEXT
);
```

### `backend_pipeline_ui_events`

```sql
CREATE TABLE backend_pipeline_ui_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    record_id INTEGER NOT NULL REFERENCES backend_records(id),
    source TEXT NOT NULL, -- uiEvents | fanoutEvents
    name TEXT,
    payload_type TEXT,
    payload_json TEXT
);
```

### `backend_pipeline_entities`

```sql
CREATE TABLE backend_pipeline_entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    record_id INTEGER NOT NULL REFERENCES backend_records(id),
    source TEXT NOT NULL, -- timelineEntities | appliedEntities
    kind TEXT,
    entity_id TEXT,
    created_ordinal INTEGER,
    last_event_ordinal INTEGER,
    tombstone INTEGER,
    payload_type TEXT,
    payload_json TEXT
);
```

### `backend_transport`

```sql
CREATE TABLE backend_transport (
    record_id INTEGER PRIMARY KEY REFERENCES backend_records(id),
    stage TEXT,
    direction TEXT,
    frame_type TEXT,
    event_name TEXT,
    payload_type TEXT,
    since_snapshot_ordinal INTEGER,
    snapshot_ordinal INTEGER,
    snapshot_entity_count INTEGER,
    fanout_event_count INTEGER,
    raw_bytes INTEGER,
    queue_len INTEGER,
    queue_cap INTEGER,
    error TEXT
);
```

### `backend_transport_snapshot_entities`

```sql
CREATE TABLE backend_transport_snapshot_entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    record_id INTEGER NOT NULL REFERENCES backend_records(id),
    kind TEXT,
    entity_id TEXT,
    created_ordinal INTEGER,
    last_event_ordinal INTEGER,
    payload_type TEXT,
    tombstone INTEGER
);
```

### `frontend_records`

```sql
CREATE TABLE frontend_records (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    ts_ms INTEGER,
    ts_iso TEXT,
    session_id TEXT,
    ordinal INTEGER,
    raw_json TEXT NOT NULL
);
```

The frontend record ID is the browser-side ring-buffer ID if present. If missing, the server assigns an incrementing ID.

### Frontend parsed tables

```sql
CREATE TABLE frontend_raw_ws (
    record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id),
    size INTEGER,
    preview TEXT,
    raw TEXT
);

CREATE TABLE frontend_parsed_frames (
    record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id),
    frame_type TEXT,
    name TEXT,
    payload_type TEXT,
    frame_json TEXT
);

CREATE TABLE frontend_snapshots (
    record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id),
    entity_count INTEGER,
    dropped_count INTEGER
);

CREATE TABLE frontend_snapshot_entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    record_id INTEGER NOT NULL REFERENCES frontend_records(id),
    raw_kind TEXT,
    raw_id TEXT,
    mapped_kind TEXT,
    mapped_id TEXT,
    dropped INTEGER
);

CREATE TABLE frontend_ui_events (
    record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id),
    name TEXT,
    message_id TEXT,
    mutation_json TEXT
);

CREATE TABLE frontend_lifecycle (
    record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id),
    event TEXT
);
```

## Implementation plan

### Phase 1: Design and route shape

- Add this design document to `PINO-STREAM-DEBUG`.
- Add tasks for upload SQLite export.
- Extend the debug route parser to allow nested action `reconcile/upload`.

### Phase 2: SQLite builder

Add a new Go file:

```text
cmd/web-chat/app/debug_reconcile_db.go
```

Responsibilities:

- parse upload JSON into `[]map[string]any`;
- create a temporary SQLite DB;
- create schema;
- insert backend records from `StreamDebugRecorder.Records(sessionID, "")`;
- insert frontend records from upload;
- return database bytes;
- clean up temp file.

### Phase 3: Endpoint

Add to `HandleDebugRoutes`:

```go
case "reconcile/upload":
    if r.Method != http.MethodPost { ... }
    body, err := s.debugRecorder.BuildSQLiteReconcileDB(r.Context(), sessionID, r.Body)
    if err != nil { ... }
    w.Header().Set("Content-Type", "application/vnd.sqlite3")
    w.Header().Set("Content-Disposition", ...)
    w.Write(body)
```

### Phase 4: Tests

Add an app test that:

1. starts a debug-enabled server;
2. produces backend records by opening a WebSocket and submitting a message;
3. POSTs a small frontend log upload;
4. opens the returned SQLite DB;
5. asserts rows exist in:
   - `backend_records`
   - `backend_pipeline`
   - `backend_transport`
   - `frontend_records`
   - `frontend_parsed_frames`
   - `frontend_ui_events`

## Future analysis ideas

Once the data is in SQLite, add views incrementally:

```sql
CREATE VIEW missing_frontend_ordinals AS ...;
CREATE VIEW ui_event_delivery_chain AS ...;
CREATE VIEW snapshot_dropped_entities AS ...;
```

We can then add CLI presets or dashboard queries without changing the upload endpoint.

## Timeline and turns data (implemented)

In addition to observer event records, the SQLite artifact now includes two tables for persisted state:

### timeline_entities

The durable timeline entities from the session's current snapshot. These represent what the system *persisted* — messages, reasoning blocks, agent mode cards, tool calls, and tool results.

```sql
CREATE TABLE timeline_entities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    created_ordinal INTEGER NOT NULL,
    last_event_ordinal INTEGER NOT NULL,
    tombstone INTEGER NOT NULL DEFAULT 0,
    payload_type TEXT,
    payload_json TEXT
);
```

### turns

The accumulated turn snapshots from the turn store. Each turn is a full accumulator snapshot containing the serialized conversation state at that point.

```sql
CREATE TABLE turns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conv_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    turn_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    runtime_key TEXT,
    inference_id TEXT,
    created_at_ms INTEGER,
    created_at TEXT,
    payload_json TEXT NOT NULL
);
```

These tables are populated by a `DebugDataProvider` interface, implemented by `exportDataProvider` which bridges `chatapp.Service` (for timeline snapshots) and `TurnStore` (for turns). When the provider is nil (e.g., test server without turn store), these tables are empty but still exist in the schema.

This means a single SQLite file now contains:
1. Backend observer records (what happened)
2. Frontend debug records (what the browser saw)
3. Timeline entities (what was persisted)
4. Turns (what was accumulated)

## Closing guidance

The endpoint should be boring and reliable. It should not try to be clever about every possible analysis. The important property is that it preserves raw evidence while adding enough schema to make common questions queryable. Analysis can grow over time; lost raw records cannot be recovered.
