package app

import (
	"context"
	"database/sql"
)

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
