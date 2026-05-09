package app

import (
	"context"
	"database/sql"
	"time"
)

func insertGeppettoRecord(ctx context.Context, db *sql.DB, id int64, timestamp time.Time, sessionID string, rec *GeppettoDebugRecord) error {
	recordSessionID := firstNonEmptyString(rec.SessionID, sessionID)
	usageJSON := mustJSON(rec.Usage)
	if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_records(record_id, ts, kind, provider, model, session_id, run_id, inference_id, turn_id, message_id, stage, event_type, info_message, provider_call_id, provider_call_index, response_id, item_id, output_index, summary_index, choice_index, content_block_index, stream_kind, correlation_key, parent_correlation_key, tool_call_id, tool_call_index, segment_id, segment_index, segment_type, segment_status, text_len, stop_reason, finish_class, duration_ms, has_tool_calls, usage_json, delta_len, normalized_delta_len, buffer_len, error, object_json, event_json, metadata_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, timestamp.Format(time.RFC3339Nano), rec.Kind, rec.Provider, rec.Model, recordSessionID, rec.RunID, rec.InferenceID, rec.TurnID, rec.MessageID, rec.Stage, rec.EventType, rec.InfoMessage, rec.ProviderCallID, nullableIntPtr(rec.ProviderCallIndex), rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), nullableIntPtr(rec.ChoiceIndex), nullableIntPtr(rec.ContentBlockIndex), rec.StreamKind, rec.CorrelationKey, rec.ParentCorrelationKey, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), rec.SegmentID, nullableIntPtr(rec.SegmentIndex), rec.SegmentType, rec.SegmentStatus, rec.TextLen, rec.StopReason, rec.FinishClass, nullableInt64Ptr(rec.DurationMs), boolInt(rec.HasToolCalls), usageJSON, rec.DeltaLen, rec.NormalizedDeltaLen, rec.BufferLen, rec.Error, mustJSON(rec.ObjectJSON), mustJSON(rec.EventJSON), mustJSON(rec.MetadataJSON)); err != nil {
		return err
	}
	if rec.ObjectJSON != nil {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_provider_events(record_id, provider_event_type, response_id, item_id, output_index, summary_index, choice_index, stream_kind, correlation_key, parent_correlation_key, tool_call_id, tool_call_index, provider_call_id, provider_call_index, object_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.EventType, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), nullableIntPtr(rec.ChoiceIndex), rec.StreamKind, rec.CorrelationKey, rec.ParentCorrelationKey, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), rec.ProviderCallID, nullableIntPtr(rec.ProviderCallIndex), mustJSON(rec.ObjectJSON)); err != nil {
			return err
		}
	}
	if rec.EventJSON != nil || rec.MetadataJSON != nil {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_emitted_events(record_id, geppetto_event_type, info_message, response_id, item_id, output_index, summary_index, choice_index, stream_kind, correlation_key, parent_correlation_key, tool_call_id, tool_call_index, provider_call_id, provider_call_index, segment_id, segment_index, segment_type, segment_status, event_json, metadata_json) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, rec.EventType, rec.InfoMessage, rec.ResponseID, rec.ItemID, nullableIntPtr(rec.OutputIndex), nullableIntPtr(rec.SummaryIndex), nullableIntPtr(rec.ChoiceIndex), rec.StreamKind, rec.CorrelationKey, rec.ParentCorrelationKey, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), rec.ProviderCallID, nullableIntPtr(rec.ProviderCallIndex), rec.SegmentID, nullableIntPtr(rec.SegmentIndex), rec.SegmentType, rec.SegmentStatus, mustJSON(rec.EventJSON), mustJSON(rec.MetadataJSON)); err != nil {
			return err
		}
	}
	if rec.Kind == "provider_call_result" || rec.Stage == "provider_call_result_finalized" {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_inference_results(record_id, ts, provider, model, session_id, run_id, inference_id, turn_id, provider_call_id, provider_call_index, response_id, stop_reason, finish_class, duration_ms, has_tool_calls, usage_json, correlation_key, parent_correlation_key) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, timestamp.Format(time.RFC3339Nano), rec.Provider, rec.Model, recordSessionID, rec.RunID, rec.InferenceID, rec.TurnID, rec.ProviderCallID, nullableIntPtr(rec.ProviderCallIndex), rec.ResponseID, rec.StopReason, rec.FinishClass, nullableInt64Ptr(rec.DurationMs), boolInt(rec.HasToolCalls), usageJSON, rec.CorrelationKey, rec.ParentCorrelationKey); err != nil {
			return err
		}
	}
	if rec.Kind == "segment" || rec.Stage == "segment_started" || rec.Stage == "segment_updated" || rec.Stage == "segment_finished" {
		if _, err := db.ExecContext(ctx, `INSERT INTO geppetto_segments(record_id, ts, provider, model, session_id, run_id, inference_id, turn_id, message_id, provider_call_id, provider_call_index, response_id, item_id, segment_id, segment_index, segment_type, stream_kind, segment_status, text_len, tool_call_id, tool_call_index, correlation_key, parent_correlation_key, event_type, stage) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, timestamp.Format(time.RFC3339Nano), rec.Provider, rec.Model, recordSessionID, rec.RunID, rec.InferenceID, rec.TurnID, rec.MessageID, rec.ProviderCallID, nullableIntPtr(rec.ProviderCallIndex), rec.ResponseID, rec.ItemID, rec.SegmentID, nullableIntPtr(rec.SegmentIndex), rec.SegmentType, rec.StreamKind, rec.SegmentStatus, rec.TextLen, rec.ToolCallID, nullableIntPtr(rec.ToolCallIndex), rec.CorrelationKey, rec.ParentCorrelationKey, rec.EventType, rec.Stage); err != nil {
			return err
		}
	}
	return nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nullableInt64Ptr(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
