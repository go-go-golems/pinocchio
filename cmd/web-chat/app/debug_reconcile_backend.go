package app

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

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
