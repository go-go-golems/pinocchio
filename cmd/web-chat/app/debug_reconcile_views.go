package app

import (
	"context"
	"database/sql"
)

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
		// persisted timeline entities. ReasoningUpdate payloads now expose provider
		// IDs directly; row order still disambiguates multiple deltas for one item.
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
		          json_extract(bpue.payload_json, '$.provider') AS backend_provider,
		          json_extract(bpue.payload_json, '$.responseId') AS backend_response_id,
		          json_extract(bpue.payload_json, '$.itemId') AS backend_item_id,
		          json_extract(bpue.payload_json, '$.outputIndex') AS backend_output_index,
		          json_extract(bpue.payload_json, '$.summaryIndex') AS backend_summary_index,
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
		          json_extract(fpf.frame_json, '$.payload.provider') AS frontend_provider,
		          json_extract(fpf.frame_json, '$.payload.responseId') AS frontend_response_id,
		          json_extract(fpf.frame_json, '$.payload.itemId') AS frontend_item_id,
		          json_extract(fpf.frame_json, '$.payload.outputIndex') AS frontend_output_index,
		          json_extract(fpf.frame_json, '$.payload.summaryIndex') AS frontend_summary_index,
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
		        br.backend_provider,
		        br.backend_response_id,
		        br.backend_item_id,
		        br.backend_output_index,
		        br.backend_summary_index,
		        br.backend_chunk,
		        fr.frontend_ordinal,
		        fr.frontend_event_name,
		        fr.frontend_message_id,
		        fr.frontend_provider,
		        fr.frontend_response_id,
		        fr.frontend_item_id,
		        fr.frontend_output_index,
		        fr.frontend_summary_index,
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
