package app

import (
	"context"
	"database/sql"
	"time"
)

func insertGeppettoRecord(ctx context.Context, db *sql.DB, id int64, timestamp time.Time, sessionID string, rec *GeppettoDebugRecord) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_records(record_id, ts, provider, model, session_id, inference_id, turn_id, message_id, stage, event_type, info_message, response_id, item_id, output_index, summary_index, choice_index, stream_kind, correlation_key, tool_call_id, tool_call_index, delta_len, normalized_delta_len, buffer_len, error, object_json, event_json, metadata_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, timestamp.Format(time.RFC3339Nano), rec.Provider, rec.Model, sessionID, rec.InferenceID, rec.TurnID, rec.MessageID, rec.Stage, rec.EventType, rec.InfoMessage, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), nullableIntPtr(rec.ChoiceIndex), rec.StreamKind, rec.CorrelationKey, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), rec.DeltaLen, rec.NormalizedDeltaLen, rec.BufferLen, rec.Error, mustJSON(rec.ObjectJSON), mustJSON(rec.EventJSON), mustJSON(rec.MetadataJSON)); err != nil {
		return err
	}
	if rec.ObjectJSON != nil {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_provider_events(record_id, provider_event_type, response_id, item_id, output_index, summary_index, choice_index, stream_kind, correlation_key, tool_call_id, tool_call_index, object_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.EventType, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), nullableIntPtr(rec.ChoiceIndex), rec.StreamKind, rec.CorrelationKey, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), mustJSON(rec.ObjectJSON)); err != nil {
			return err
		}
	}
	if rec.EventJSON != nil || rec.MetadataJSON != nil {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_emitted_events(record_id, geppetto_event_type, info_message, response_id, item_id, output_index, summary_index, choice_index, stream_kind, correlation_key, tool_call_id, tool_call_index, event_json, metadata_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.EventType, rec.InfoMessage, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), nullableIntPtr(rec.ChoiceIndex), rec.StreamKind, rec.CorrelationKey, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), mustJSON(rec.EventJSON), mustJSON(rec.MetadataJSON)); err != nil {
			return err
		}
	}
	return nil
}
