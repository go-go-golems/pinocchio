package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	chatexport "github.com/go-go-golems/pinocchio/pkg/chatapp/export"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"

	_ "github.com/mattn/go-sqlite3"
)

// DebugTimelineProvider fetches timeline snapshot data for the reconcile DB.
type DebugTimelineProvider interface {
	ExportTimelineEntities(ctx context.Context, sessionID string) ([]DebugTimelineEntity, error)
}

// DebugTurnsProvider fetches turns data for the reconcile DB.
type DebugTurnsProvider interface {
	ExportTurnsList(ctx context.Context, sessionID string) ([]DebugTurn, error)
}

// DebugTimelineEntity is a flat timeline entity row for the reconcile DB.
type DebugTimelineEntity struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	CreatedOrdinal   uint64 `json:"createdOrdinal"`
	LastEventOrdinal uint64 `json:"lastEventOrdinal"`
	Tombstone        bool   `json:"tombstone"`
	PayloadType      string `json:"payloadType,omitempty"`
	Payload          string `json:"payload,omitempty"`
}

// DebugTurn is a flat turn row for the reconcile DB.
type DebugTurn struct {
	ConvID      string `json:"convId"`
	SessionID   string `json:"sessionId"`
	TurnID      string `json:"turnId"`
	Phase       string `json:"phase"`
	RuntimeKey  string `json:"runtimeKey,omitempty"`
	InferenceID string `json:"inferenceId,omitempty"`
	CreatedAtMs int64  `json:"createdAtMs"`
	CreatedAt   string `json:"createdAt,omitempty"`
	Payload     string `json:"payload"`
}

// DebugDataProvider combines timeline and turns providers.
type DebugDataProvider interface {
	DebugTimelineProvider
	DebugTurnsProvider
}

type frontendLogUpload struct {
	Records []map[string]any `json:"records"`
}

func (r *StreamDebugRecorder) BuildSQLiteReconcileDB(ctx context.Context, sessionID string, body io.Reader, provider DebugDataProvider) ([]byte, error) {
	frontendRecords, err := parseFrontendLogUpload(body)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "pinocchio-stream-debug-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "stream-debug.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	if err := createDebugSQLiteSchema(ctx, db); err != nil {
		return nil, err
	}
	backendRecords := r.Records(sessionID, "")
	if err := insertDebugSQLiteMeta(ctx, db, sessionID, len(backendRecords), len(frontendRecords)); err != nil {
		return nil, err
	}
	if err := insertBackendDebugRecords(ctx, db, backendRecords); err != nil {
		return nil, err
	}
	if err := insertFrontendDebugRecords(ctx, db, frontendRecords); err != nil {
		return nil, err
	}
	if provider != nil {
		if err := insertTimelineEntities(ctx, db, provider, sessionID); err != nil {
			return nil, fmt.Errorf("insert timeline: %w", err)
		}
		if err := insertTurns(ctx, db, provider, sessionID); err != nil {
			return nil, fmt.Errorf("insert turns: %w", err)
		}
	}
	if err := createDebugSQLiteViews(ctx, db); err != nil {
		return nil, fmt.Errorf("create views: %w", err)
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func parseFrontendLogUpload(body io.Reader) ([]map[string]any, error) {
	if body == nil {
		return nil, fmt.Errorf("missing frontend log body")
	}
	var raw any
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode frontend log upload: %w", err)
	}
	if obj, ok := raw.(map[string]any); ok {
		raw = obj["records"]
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("frontend log upload must be an array or object with records array")
	}
	out := make([]map[string]any, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("frontend log record %d is not an object", i)
		}
		out = append(out, obj)
	}
	return out, nil
}

func createDebugSQLiteSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE backend_records (id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, ts TEXT, session_id TEXT, connection_id TEXT, ordinal INTEGER, raw_json TEXT NOT NULL)`,
		`CREATE TABLE backend_pipeline (record_id INTEGER PRIMARY KEY REFERENCES backend_records(id), mode TEXT, event_name TEXT, event_type TEXT, event_appended INTEGER, view_ordinal INTEGER, timeline_cursor_advanced INTEGER, append_error TEXT, session_error TEXT, view_error TEXT, ui_projection_error TEXT, timeline_projection_error TEXT, apply_error TEXT, cursor_error TEXT, fanout_error TEXT)`,
		`CREATE TABLE backend_pipeline_ui_events (id INTEGER PRIMARY KEY AUTOINCREMENT, record_id INTEGER NOT NULL REFERENCES backend_records(id), source TEXT NOT NULL, name TEXT, payload_type TEXT, payload_json TEXT)`,
		`CREATE TABLE backend_pipeline_entities (id INTEGER PRIMARY KEY AUTOINCREMENT, record_id INTEGER NOT NULL REFERENCES backend_records(id), source TEXT NOT NULL, kind TEXT, entity_id TEXT, created_ordinal INTEGER, last_event_ordinal INTEGER, tombstone INTEGER, payload_type TEXT, payload_json TEXT)`,
		`CREATE TABLE backend_transport (record_id INTEGER PRIMARY KEY REFERENCES backend_records(id), stage TEXT, direction TEXT, frame_type TEXT, event_name TEXT, payload_type TEXT, since_snapshot_ordinal INTEGER, snapshot_ordinal INTEGER, snapshot_entity_count INTEGER, fanout_event_count INTEGER, raw_bytes INTEGER, queue_len INTEGER, queue_cap INTEGER, error TEXT)`,
		`CREATE TABLE backend_transport_snapshot_entities (id INTEGER PRIMARY KEY AUTOINCREMENT, record_id INTEGER NOT NULL REFERENCES backend_records(id), kind TEXT, entity_id TEXT, created_ordinal INTEGER, last_event_ordinal INTEGER, payload_type TEXT, tombstone INTEGER)`,
		`CREATE TABLE geppetto_records (record_id INTEGER PRIMARY KEY REFERENCES backend_records(id), ts TEXT, provider TEXT, model TEXT, session_id TEXT, inference_id TEXT, turn_id TEXT, message_id TEXT, stage TEXT NOT NULL, event_type TEXT, info_message TEXT, response_id TEXT, item_id TEXT, output_index INTEGER, summary_index INTEGER, delta_len INTEGER, normalized_delta_len INTEGER, buffer_len INTEGER, error TEXT, object_json TEXT, event_json TEXT, metadata_json TEXT)`,
		`CREATE TABLE geppetto_provider_events (record_id INTEGER PRIMARY KEY REFERENCES geppetto_records(record_id), provider_event_type TEXT, response_id TEXT, item_id TEXT, output_index INTEGER, summary_index INTEGER, object_json TEXT)`,
		`CREATE TABLE geppetto_emitted_events (record_id INTEGER PRIMARY KEY REFERENCES geppetto_records(record_id), geppetto_event_type TEXT, info_message TEXT, response_id TEXT, item_id TEXT, output_index INTEGER, summary_index INTEGER, event_json TEXT, metadata_json TEXT)`,
		`CREATE INDEX idx_geppetto_records_session_stage ON geppetto_records(session_id, stage)`,
		`CREATE INDEX idx_geppetto_records_item ON geppetto_records(item_id)`,
		`CREATE INDEX idx_geppetto_records_event_type ON geppetto_records(event_type)`,
		`CREATE TABLE frontend_records (id INTEGER PRIMARY KEY, type TEXT NOT NULL, ts_ms INTEGER, ts_iso TEXT, session_id TEXT, ordinal INTEGER, raw_json TEXT NOT NULL)`,
		`CREATE TABLE frontend_raw_ws (record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id), size INTEGER, preview TEXT, raw TEXT)`,
		`CREATE TABLE frontend_parsed_frames (record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id), frame_type TEXT, name TEXT, payload_type TEXT, frame_json TEXT)`,
		`CREATE TABLE frontend_snapshots (record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id), entity_count INTEGER, dropped_count INTEGER)`,
		`CREATE TABLE frontend_snapshot_entities (id INTEGER PRIMARY KEY AUTOINCREMENT, record_id INTEGER NOT NULL REFERENCES frontend_records(id), raw_kind TEXT, raw_id TEXT, mapped_kind TEXT, mapped_id TEXT, dropped INTEGER)`,
		`CREATE TABLE frontend_ui_events (record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id), name TEXT, message_id TEXT, mutation_json TEXT)`,
		`CREATE TABLE frontend_lifecycle (record_id INTEGER PRIMARY KEY REFERENCES frontend_records(id), event TEXT)`,
		`CREATE INDEX idx_backend_records_session_ordinal ON backend_records(session_id, ordinal)`,
		`CREATE INDEX idx_backend_transport_stage ON backend_transport(stage)`,
		`CREATE INDEX idx_frontend_records_session_ordinal ON frontend_records(session_id, ordinal)`,
		`CREATE INDEX idx_frontend_records_type ON frontend_records(type)`,

		// Timeline and turns tables — persisted data alongside event evidence.
		`CREATE TABLE timeline_entities (id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, entity_id TEXT NOT NULL, created_ordinal INTEGER NOT NULL, last_event_ordinal INTEGER NOT NULL, tombstone INTEGER NOT NULL DEFAULT 0, payload_type TEXT, payload_json TEXT)`,
		`CREATE INDEX idx_timeline_entities_kind ON timeline_entities(kind)`,
		`CREATE INDEX idx_timeline_entities_entity_id ON timeline_entities(entity_id)`,

		`CREATE TABLE turns (id INTEGER PRIMARY KEY AUTOINCREMENT, conv_id TEXT NOT NULL, session_id TEXT NOT NULL, turn_id TEXT NOT NULL, phase TEXT NOT NULL, runtime_key TEXT, inference_id TEXT, created_at_ms INTEGER, created_at TEXT, payload_json TEXT NOT NULL)`,
		`CREATE INDEX idx_turns_session_id ON turns(session_id)`,
		`CREATE INDEX idx_turns_conv_id ON turns(conv_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func createDebugSQLiteViews(ctx context.Context, db *sql.DB) error {
	views := []string{
		// Backend pipeline fanout ordinals that never reached WebSocket transport.
		`CREATE VIEW missing_transport_fanout AS
		 SELECT bp.record_id, bp.event_name, br.ordinal
		   FROM backend_pipeline bp
		   JOIN backend_records br ON br.id = bp.record_id
		   WHERE bp.fanout_error = ''
		     AND br.ordinal != ''
		     AND NOT EXISTS (
		       SELECT 1 FROM backend_transport bt
		         JOIN backend_records br2 ON br2.id = bt.record_id
		        WHERE bt.stage = 'fanout_started' AND br2.ordinal = br.ordinal
		     )`,

		// Backend transport fanout ordinals with no corresponding pipeline record.
		`CREATE VIEW extra_transport_fanout AS
		 SELECT bt.record_id, bt.stage, br.ordinal
		   FROM backend_transport bt
		   JOIN backend_records br ON br.id = bt.record_id
		  WHERE bt.stage = 'fanout_started'
		    AND br.ordinal != ''
		    AND NOT EXISTS (
		      SELECT 1 FROM backend_pipeline bp
		        JOIN backend_records br2 ON br2.id = bp.record_id
		       WHERE br2.ordinal = br.ordinal
		    )`,

		// Backend pipeline events with errors.
		`CREATE VIEW backend_pipeline_errors AS
		 SELECT br.ordinal, bp.event_name,
		        bp.append_error, bp.view_error,
		        bp.ui_projection_error, bp.timeline_projection_error,
		        bp.apply_error, bp.cursor_error, bp.fanout_error
		   FROM backend_pipeline bp
		   JOIN backend_records br ON br.id = bp.record_id
		  WHERE COALESCE(bp.append_error, bp.view_error, bp.ui_projection_error,
		                 bp.timeline_projection_error, bp.apply_error, bp.cursor_error, bp.fanout_error, '') != ''`,

		// Backend transport events with errors.
		`CREATE VIEW backend_transport_errors AS
		 SELECT br.ordinal, bt.stage, bt.frame_type, bt.error
		   FROM backend_transport bt
		   JOIN backend_records br ON br.id = bt.record_id
		  WHERE bt.error IS NOT NULL AND bt.error != ''`,

		// Geppetto reasoning/provider sequence for OpenAI Responses debugging.
		`CREATE VIEW geppetto_reasoning_sequence AS
		 SELECT record_id, ts, stage, event_type, info_message, message_id, response_id, item_id,
		        output_index, summary_index, delta_len, normalized_delta_len, buffer_len, error
		   FROM geppetto_records
		  WHERE COALESCE(event_type, '') LIKE '%reasoning%'
		     OR COALESCE(event_type, '') LIKE '%summary%'
		     OR COALESCE(info_message, '') LIKE '%thinking%'
		     OR COALESCE(info_message, '') LIKE '%reasoning%'
		  ORDER BY ts, record_id`,

		// Summary-related Geppetto records missing provider item IDs.
		`CREATE VIEW geppetto_summary_without_item_id AS
		 SELECT *
		   FROM geppetto_records
		  WHERE (COALESCE(event_type, '') LIKE '%summary%'
		     OR COALESCE(info_message, '') LIKE '%summary%')
		    AND COALESCE(item_id, '') = ''`,

		// Geppetto publish errors.
		`CREATE VIEW geppetto_publish_errors AS
		 SELECT *
		   FROM geppetto_records
		  WHERE stage = 'geppetto_publish_error'
		     OR COALESCE(error, '') != ''`,

		// Provider records next to emitted Geppetto events by provider item id.
		`CREATE VIEW geppetto_provider_to_emitted AS
		 SELECT p.record_id AS provider_record_id,
		        p.provider_event_type,
		        p.response_id,
		        p.item_id,
		        e.record_id AS emitted_record_id,
		        e.geppetto_event_type,
		        e.info_message
		   FROM geppetto_provider_events p
		   LEFT JOIN geppetto_emitted_events e
		     ON COALESCE(e.item_id, '') = COALESCE(p.item_id, '')
		    AND COALESCE(e.item_id, '') != ''`,

		// Provider reasoning deltas correlated through Geppetto publish records,
		// backend Sessionstream ordinals, frontend parsed frames, UI mutations, and
		// persisted timeline entities. This is row-order/chunk based until provider
		// IDs are propagated into frontend ReasoningUpdate payloads.
		`CREATE VIEW geppetto_reasoning_to_frontend AS
		 WITH
		 provider_delta AS (
		   SELECT row_number() OVER (ORDER BY record_id) AS rn,
		          record_id AS provider_record_id,
		          response_id,
		          item_id,
		          output_index,
		          summary_index,
		          json_extract(object_json, '$.delta') AS provider_delta
		     FROM geppetto_records
		    WHERE stage = 'provider_normalize_delta'
		      AND event_type = 'response.reasoning_summary_text.delta'
		 ),
		 geppetto_delta AS (
		   SELECT row_number() OVER (ORDER BY record_id) AS rn,
		          record_id AS geppetto_event_record_id,
		          json_extract(event_json, '$.delta') AS geppetto_delta,
		          message_id AS geppetto_message_id
		     FROM geppetto_records
		    WHERE stage = 'geppetto_publish_done'
		      AND event_type = 'partial-thinking'
		 ),
		 backend_reasoning AS (
		   SELECT row_number() OVER (ORDER BY CAST(br.ordinal AS INTEGER)) AS rn,
		          br.ordinal AS backend_ordinal,
		          bp.event_name AS backend_event_name,
		          json_extract(bpue.payload_json, '$.messageId') AS backend_message_id,
		          json_extract(bpue.payload_json, '$.chunk') AS backend_chunk
		     FROM backend_pipeline bp
		     JOIN backend_records br ON br.id = bp.record_id
		     JOIN backend_pipeline_ui_events bpue ON bpue.record_id = br.id
		    WHERE bp.event_name = 'ChatReasoningDelta'
		      AND bpue.source = 'uiEvents'
		 ),
		 frontend_reasoning AS (
		   SELECT row_number() OVER (ORDER BY CAST(fr.ordinal AS INTEGER)) AS rn,
		          fr.ordinal AS frontend_ordinal,
		          fpf.name AS frontend_event_name,
		          json_extract(fpf.frame_json, '$.payload.messageId') AS frontend_message_id,
		          json_extract(fpf.frame_json, '$.payload.chunk') AS frontend_chunk
		     FROM frontend_parsed_frames fpf
		     JOIN frontend_records fr ON fr.id = fpf.record_id
		    WHERE fpf.name = 'ChatReasoningAppended'
		 ),
		 frontend_mutation AS (
		   SELECT fr.ordinal,
		          fui.name AS frontend_ui_event_name,
		          fui.message_id AS ui_mutation_message_id
		     FROM frontend_ui_events fui
		     JOIN frontend_records fr ON fr.id = fui.record_id
		    WHERE fui.name = 'ChatReasoningAppended'
		 )
		 SELECT pd.rn,
		        pd.provider_record_id,
		        pd.response_id,
		        pd.item_id AS provider_item_id,
		        pd.output_index,
		        pd.summary_index,
		        pd.provider_delta,
		        gd.geppetto_event_record_id,
		        gd.geppetto_delta,
		        gd.geppetto_message_id,
		        br.backend_ordinal,
		        br.backend_event_name,
		        br.backend_message_id,
		        br.backend_chunk,
		        fr.frontend_ordinal,
		        fr.frontend_event_name,
		        fr.frontend_message_id,
		        fr.frontend_chunk,
		        fm.frontend_ui_event_name,
		        fm.ui_mutation_message_id,
		        te.entity_id AS timeline_entity_id,
		        te.created_ordinal AS timeline_created_ordinal,
		        te.last_event_ordinal AS timeline_last_event_ordinal
		   FROM provider_delta pd
		   JOIN geppetto_delta gd ON gd.rn = pd.rn
		   JOIN backend_reasoning br ON br.rn = pd.rn
		   JOIN frontend_reasoning fr ON fr.rn = pd.rn
		   LEFT JOIN frontend_mutation fm ON fm.ordinal = fr.frontend_ordinal
		   LEFT JOIN timeline_entities te ON te.entity_id = fr.frontend_message_id`,

		// Frontend parsed frames with no corresponding UI event mutation.
		`CREATE VIEW frontend_parsed_no_mutation AS
		 SELECT pf.record_id, pf.frame_type, pf.name, fr.ordinal
		   FROM frontend_parsed_frames pf
		   JOIN frontend_records fr ON fr.id = pf.record_id
		  WHERE pf.frame_type = 'ui-event'
		    AND NOT EXISTS (
		      SELECT 1 FROM frontend_ui_events fue
		       WHERE fue.record_id = pf.record_id
		    )`,

		// Frontend snapshot entities that were dropped during hydration.
		`CREATE VIEW frontend_dropped_entities AS
		 SELECT fse.raw_kind, fse.raw_id, fse.mapped_kind, fse.mapped_id
		   FROM frontend_snapshot_entities fse
		  WHERE fse.dropped = 1`,

		// Timeline entities that are tombstoned.
		`CREATE VIEW tombstoned_entities AS
		 SELECT kind, entity_id, created_ordinal, last_event_ordinal, payload_type
		   FROM timeline_entities
		  WHERE tombstone = 1`,

		// Delivery chain: pipeline fanout -> transport fanout -> frontend parsed.
		`CREATE VIEW delivery_chain AS
		 SELECT br.ordinal,
		        bp.event_name AS pipeline_event,
		        CASE WHEN EXISTS (
		          SELECT 1 FROM backend_transport bt
		            JOIN backend_records br2 ON br2.id = bt.record_id
		           WHERE bt.stage = 'fanout_started' AND br2.ordinal = br.ordinal
		        ) THEN 'yes' ELSE 'no' END AS transport_fanout,
		        CASE WHEN EXISTS (
		          SELECT 1 FROM frontend_parsed_frames fpf
		            JOIN frontend_records fr ON fr.id = fpf.record_id
		           WHERE fr.ordinal = br.ordinal
		        ) THEN 'yes' ELSE 'no' END AS frontend_parsed
		   FROM backend_pipeline bp
		   JOIN backend_records br ON br.id = bp.record_id
		  WHERE br.ordinal != ''
		  ORDER BY CAST(br.ordinal AS INTEGER)`,

		// Per-entity timeline state: entity kind counts.
		`CREATE VIEW entity_kind_summary AS
		 SELECT kind, COUNT(*) AS count,
		        SUM(CASE WHEN tombstone = 0 THEN 1 ELSE 0 END) AS alive,
		        SUM(CASE WHEN tombstone = 1 THEN 1 ELSE 0 END) AS tombstoned
		   FROM timeline_entities
		  GROUP BY kind`,
	}
	for _, view := range views {
		if _, err := db.ExecContext(ctx, view); err != nil {
			return err
		}
	}
	return nil
}

func insertDebugSQLiteMeta(ctx context.Context, db *sql.DB, sessionID string, backendCount, frontendCount int) error {
	entries := map[string]string{
		"schema_version":        "pinocchio-stream-debug-sqlite-v1",
		"session_id":            sessionID,
		"created_at":            time.Now().UTC().Format(time.RFC3339Nano),
		"backend_record_count":  strconv.Itoa(backendCount),
		"frontend_record_count": strconv.Itoa(frontendCount),
		"geppetto_record_count": strconv.Itoa(0),
	}
	for k, v := range entries {
		if _, err := db.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES(?, ?)`, k, v); err != nil {
			return err
		}
	}
	return nil
}

func insertBackendDebugRecords(ctx context.Context, db *sql.DB, records []DebugRecord) error {
	geppettoCount := 0
	for _, rec := range records {
		raw := mustJSON(rec)
		res, err := db.ExecContext(ctx, `INSERT INTO backend_records(kind, ts, session_id, connection_id, ordinal, raw_json) VALUES(?, ?, ?, ?, ?, ?)`, rec.Kind, rec.Timestamp.Format(time.RFC3339Nano), rec.SessionID, rec.ConnectionID, nullableInt(rec.Ordinal), raw)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if rec.Pipeline != nil {
			if err := insertBackendPipeline(ctx, db, id, rec.Pipeline); err != nil {
				return err
			}
		}
		if rec.Transport != nil {
			if err := insertBackendTransport(ctx, db, id, rec.Transport); err != nil {
				return err
			}
		}
		if rec.Geppetto != nil {
			geppettoCount++
			if err := insertGeppettoRecord(ctx, db, id, rec.Timestamp, rec.SessionID, rec.Geppetto); err != nil {
				return err
			}
		}
	}
	_, err := db.ExecContext(ctx, `INSERT OR REPLACE INTO meta(key, value) VALUES('geppetto_record_count', ?)`, strconv.Itoa(geppettoCount))
	return err
}

func insertBackendPipeline(ctx context.Context, db *sql.DB, id int64, rec *PipelineDebugRecord) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO backend_pipeline(record_id, mode, event_name, event_type, event_appended, view_ordinal, timeline_cursor_advanced, append_error, session_error, view_error, ui_projection_error, timeline_projection_error, apply_error, cursor_error, fanout_error) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.Mode, rec.Event, rec.EventTyp, boolInt(rec.EventAppended), nullableInt(rec.ViewOrdinal), boolInt(rec.TimelineCursorAdvanced), rec.AppendError, rec.SessionError, rec.ViewError, rec.UIProjectionError, rec.TimelineProjectionError, rec.ApplyError, rec.CursorError, rec.FanoutError); err != nil {
		return err
	}
	for _, ev := range rec.UIEvents {
		if err := insertBackendPipelineUIEvent(ctx, db, id, "uiEvents", ev); err != nil {
			return err
		}
	}
	for _, ev := range rec.FanoutEvents {
		if err := insertBackendPipelineUIEvent(ctx, db, id, "fanoutEvents", ev); err != nil {
			return err
		}
	}
	for _, ent := range rec.TimelineEntities {
		if err := insertBackendPipelineEntity(ctx, db, id, "timelineEntities", ent); err != nil {
			return err
		}
	}
	for _, ent := range rec.AppliedEntities {
		if err := insertBackendPipelineEntity(ctx, db, id, "appliedEntities", ent); err != nil {
			return err
		}
	}
	return nil
}

func insertBackendPipelineUIEvent(ctx context.Context, db *sql.DB, id int64, source string, ev UIEventDebug) error {
	_, err := db.ExecContext(ctx, `INSERT INTO backend_pipeline_ui_events(record_id, source, name, payload_type, payload_json) VALUES(?, ?, ?, ?, ?)`, id, source, ev.Name, ev.PayloadType, mustJSON(ev.Payload))
	return err
}

func insertBackendPipelineEntity(ctx context.Context, db *sql.DB, id int64, source string, ent TimelineEntityDebug) error {
	_, err := db.ExecContext(ctx, `INSERT INTO backend_pipeline_entities(record_id, source, kind, entity_id, created_ordinal, last_event_ordinal, tombstone, payload_type, payload_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, source, ent.Kind, ent.ID, nullableInt(ent.CreatedOrdinal), nullableInt(ent.LastEventOrdinal), boolInt(ent.Tombstone), ent.PayloadType, mustJSON(ent.Payload))
	return err
}

func insertBackendTransport(ctx context.Context, db *sql.DB, id int64, rec *TransportDebugRecord) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO backend_transport(record_id, stage, direction, frame_type, event_name, payload_type, since_snapshot_ordinal, snapshot_ordinal, snapshot_entity_count, fanout_event_count, raw_bytes, queue_len, queue_cap, error) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.Stage, rec.Direction, rec.FrameType, rec.EventName, rec.PayloadType, nullableInt(rec.SinceSnapshotOrdinal), nullableInt(rec.SnapshotOrdinal), rec.SnapshotEntityCount, rec.FanoutEventCount, rec.RawBytes, rec.QueueLen, rec.QueueCap, rec.Error); err != nil {
		return err
	}
	for _, ent := range rec.SnapshotEntities {
		if _, err := db.ExecContext(ctx, `INSERT INTO backend_transport_snapshot_entities(record_id, kind, entity_id, created_ordinal, last_event_ordinal, payload_type, tombstone) VALUES(?, ?, ?, ?, ?, ?, ?)`, id, ent.Kind, ent.ID, nullableInt(ent.CreatedOrdinal), nullableInt(ent.LastEventOrdinal), ent.PayloadType, boolInt(ent.Tombstone)); err != nil {
			return err
		}
	}
	return nil
}

func insertGeppettoRecord(ctx context.Context, db *sql.DB, id int64, timestamp time.Time, sessionID string, rec *GeppettoDebugRecord) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_records(record_id, ts, provider, model, session_id, inference_id, turn_id, message_id, stage, event_type, info_message, response_id, item_id, output_index, summary_index, delta_len, normalized_delta_len, buffer_len, error, object_json, event_json, metadata_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, timestamp.Format(time.RFC3339Nano), rec.Provider, rec.Model, sessionID, rec.InferenceID, rec.TurnID, rec.MessageID, rec.Stage, rec.EventType, rec.InfoMessage, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), rec.DeltaLen, rec.NormalizedDeltaLen, rec.BufferLen, rec.Error, mustJSON(rec.ObjectJSON), mustJSON(rec.EventJSON), mustJSON(rec.MetadataJSON)); err != nil {
		return err
	}
	if rec.ObjectJSON != nil {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_provider_events(record_id, provider_event_type, response_id, item_id, output_index, summary_index, object_json) VALUES(?, ?, ?, ?, ?, ?, ?)`, id, rec.EventType, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), mustJSON(rec.ObjectJSON)); err != nil {
			return err
		}
	}
	if rec.EventJSON != nil || rec.MetadataJSON != nil {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_emitted_events(record_id, geppetto_event_type, info_message, response_id, item_id, output_index, summary_index, event_json, metadata_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.EventType, rec.InfoMessage, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), mustJSON(rec.EventJSON), mustJSON(rec.MetadataJSON)); err != nil {
			return err
		}
	}
	return nil
}

func insertFrontendDebugRecords(ctx context.Context, db *sql.DB, records []map[string]any) error {
	for i, rec := range records {
		id := int64FromAny(rec["id"])
		if id == 0 {
			id = int64(i + 1)
		}
		typ := stringFromAny(rec["type"])
		if typ == "" {
			typ = "unknown"
		}
		tsMs := int64FromAny(rec["timestamp"])
		tsISO := ""
		if tsMs > 0 {
			tsISO = time.UnixMilli(tsMs).UTC().Format(time.RFC3339Nano)
		}
		raw := mustJSON(rec)
		if _, err := db.ExecContext(ctx, `INSERT INTO frontend_records(id, type, ts_ms, ts_iso, session_id, ordinal, raw_json) VALUES(?, ?, ?, ?, ?, ?, ?)`, id, typ, tsMs, tsISO, stringFromAny(rec["sessionId"]), nullableIntFromAny(rec["ordinal"]), raw); err != nil {
			return err
		}
		if err := insertFrontendTypedRecord(ctx, db, id, typ, rec); err != nil {
			return err
		}
	}
	return nil
}

func insertFrontendTypedRecord(ctx context.Context, db *sql.DB, id int64, typ string, rec map[string]any) error {
	switch typ {
	case "raw-ws":
		_, err := db.ExecContext(ctx, `INSERT INTO frontend_raw_ws(record_id, size, preview, raw) VALUES(?, ?, ?, ?)`, id, int64FromAny(rec["size"]), stringFromAny(rec["preview"]), stringFromAny(rec["raw"]))
		return err
	case "parsed-frame":
		_, err := db.ExecContext(ctx, `INSERT INTO frontend_parsed_frames(record_id, frame_type, name, payload_type, frame_json) VALUES(?, ?, ?, ?, ?)`, id, stringFromAny(rec["frameType"]), stringFromAny(rec["name"]), stringFromAny(rec["payloadType"]), mustJSON(rec["frame"]))
		return err
	case "snapshot":
		if _, err := db.ExecContext(ctx, `INSERT INTO frontend_snapshots(record_id, entity_count, dropped_count) VALUES(?, ?, ?)`, id, int64FromAny(rec["entityCount"]), int64FromAny(rec["droppedCount"])); err != nil {
			return err
		}
		for _, item := range arrayFromAny(rec["entities"]) {
			obj, _ := item.(map[string]any)
			if _, err := db.ExecContext(ctx, `INSERT INTO frontend_snapshot_entities(record_id, raw_kind, raw_id, mapped_kind, mapped_id, dropped) VALUES(?, ?, ?, ?, ?, ?)`, id, stringFromAny(obj["rawKind"]), stringFromAny(obj["rawId"]), stringFromAny(obj["mappedKind"]), stringFromAny(obj["mappedId"]), boolInt(boolFromAny(obj["dropped"]))); err != nil {
				return err
			}
		}
		return nil
	case "ui-event":
		_, err := db.ExecContext(ctx, `INSERT INTO frontend_ui_events(record_id, name, message_id, mutation_json) VALUES(?, ?, ?, ?)`, id, stringFromAny(rec["name"]), stringFromAny(rec["messageId"]), mustJSON(rec["mutation"]))
		return err
	case "ws-lifecycle":
		_, err := db.ExecContext(ctx, `INSERT INTO frontend_lifecycle(record_id, event) VALUES(?, ?)`, id, stringFromAny(rec["event"]))
		return err
	default:
		return nil
	}
}

func insertTimelineEntities(ctx context.Context, db *sql.DB, provider DebugTimelineProvider, sessionID string) error {
	entities, err := provider.ExportTimelineEntities(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, ent := range entities {
		_, err := db.ExecContext(ctx,
			`INSERT INTO timeline_entities(kind, entity_id, created_ordinal, last_event_ordinal, tombstone, payload_type, payload_json) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			ent.Kind, ent.ID, ent.CreatedOrdinal, ent.LastEventOrdinal, boolInt(ent.Tombstone), ent.PayloadType, ent.Payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertTurns(ctx context.Context, db *sql.DB, provider DebugTurnsProvider, sessionID string) error {
	turns, err := provider.ExportTurnsList(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, turn := range turns {
		_, err := db.ExecContext(ctx,
			`INSERT INTO turns(conv_id, session_id, turn_id, phase, runtime_key, inference_id, created_at_ms, created_at, payload_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			turn.ConvID, turn.SessionID, turn.TurnID, turn.Phase, turn.RuntimeKey, turn.InferenceID, turn.CreatedAtMs, turn.CreatedAt, turn.Payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func mustJSON(v any) string {
	if v == nil {
		return "null"
	}
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(body)
}

// exportDataProvider adapts the app's Service and TurnStore to DebugDataProvider.
type exportDataProvider struct {
	snapshotProvider chatexport.SnapshotProvider
	turnStore        chatstore.TurnStore
}

func newExportDataProvider(snapshotProvider chatexport.SnapshotProvider, turnStore chatstore.TurnStore) *exportDataProvider {
	if snapshotProvider == nil && turnStore == nil {
		return nil
	}
	return &exportDataProvider{snapshotProvider: snapshotProvider, turnStore: turnStore}
}

func (p *exportDataProvider) ExportTimelineEntities(ctx context.Context, sessionID string) ([]DebugTimelineEntity, error) {
	if p == nil || p.snapshotProvider == nil {
		return nil, nil
	}
	snap, err := p.snapshotProvider.Snapshot(ctx, sessionstream.SessionId(sessionID))
	if err != nil {
		return nil, err
	}
	entities := make([]DebugTimelineEntity, 0, len(snap.Entities))
	for _, ent := range snap.Entities {
		entities = append(entities, DebugTimelineEntity{
			Kind:             ent.Kind,
			ID:               ent.Id,
			CreatedOrdinal:   ent.CreatedOrdinal,
			LastEventOrdinal: ent.LastEventOrdinal,
			Tombstone:        ent.Tombstone,
			PayloadType:      protoType(ent.Payload),
			Payload:          mustJSON(encodeProtoJSON(ent.Payload)),
		})
	}
	return entities, nil
}

func (p *exportDataProvider) ExportTurnsList(ctx context.Context, sessionID string) ([]DebugTurn, error) {
	if p == nil || p.turnStore == nil {
		return nil, nil
	}
	turns, err := p.turnStore.List(ctx, chatstore.TurnQuery{ConvID: sessionID})
	if err != nil {
		return nil, err
	}
	out := make([]DebugTurn, 0, len(turns))
	for _, turn := range turns {
		out = append(out, DebugTurn{
			ConvID:      turn.ConvID,
			SessionID:   turn.SessionID,
			TurnID:      turn.TurnID,
			Phase:       turn.Phase,
			RuntimeKey:  turn.RuntimeKey,
			InferenceID: turn.InferenceID,
			CreatedAtMs: turn.CreatedAtMs,
			Payload:     turn.Payload,
		})
	}
	return out, nil
}

func nullableIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(s string) any {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return v
}

func nullableIntFromAny(v any) any {
	i := int64FromAny(v)
	if i == 0 {
		return nil
	}
	return i
}

func int64FromAny(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		out, _ := typed.Int64()
		return out
	case string:
		out, _ := strconv.ParseInt(typed, 10, 64)
		return out
	default:
		return 0
	}
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func boolFromAny(v any) bool {
	b, _ := v.(bool)
	return b
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func arrayFromAny(v any) []any {
	items, _ := v.([]any)
	return items
}
