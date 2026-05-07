package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

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
