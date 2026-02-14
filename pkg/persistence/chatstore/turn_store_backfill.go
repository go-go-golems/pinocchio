package chatstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	"github.com/pkg/errors"
)

type TurnBackfillOptions struct {
	ConvID    string
	SessionID string
	Limit     int
	DryRun    bool
}

type TurnBackfillResult struct {
	SnapshotsScanned    int `json:"snapshots_scanned"`
	SnapshotsBackfilled int `json:"snapshots_backfilled"`
	BlocksScanned       int `json:"blocks_scanned"`
	BlockRowsUpserted   int `json:"block_rows_upserted"`
	TurnRowsUpserted    int `json:"turn_rows_upserted"`
	MembershipInserted  int `json:"membership_inserted"`
	ParseErrors         int `json:"parse_errors"`
}

type snapshotBackfillRow struct {
	convID      string
	sessionID   string
	turnID      string
	phase       string
	createdAtMs int64
	payload     string
}

// BackfillNormalizedFromSnapshots parses legacy snapshot payloads and writes normalized rows.
func (s *SQLiteTurnStore) BackfillNormalizedFromSnapshots(ctx context.Context, opts TurnBackfillOptions) (*TurnBackfillResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite turn store: db is nil")
	}
	if ctx == nil {
		return nil, errors.New("sqlite turn store: ctx is nil")
	}

	clauses := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if v := strings.TrimSpace(opts.ConvID); v != "" {
		clauses = append(clauses, "conv_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(opts.SessionID); v != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, v)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT conv_id, session_id, turn_id, phase, created_at_ms, payload
		FROM %s
		%s
		ORDER BY created_at_ms ASC
	`, legacyTurnSnapshotsTable, where)
	if opts.Limit > 0 {
		query += "\nLIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: query snapshots for backfill")
	}
	defer func() { _ = rows.Close() }()

	result := &TurnBackfillResult{}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: begin backfill tx")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for rows.Next() {
		var row snapshotBackfillRow
		if err := rows.Scan(&row.convID, &row.sessionID, &row.turnID, &row.phase, &row.createdAtMs, &row.payload); err != nil {
			return nil, errors.Wrap(err, "sqlite turn store: scan snapshot row")
		}
		result.SnapshotsScanned++

		t, err := serde.FromYAML([]byte(row.payload))
		if err != nil || t == nil {
			result.ParseErrors++
			continue
		}

		turnID := strings.TrimSpace(row.turnID)
		if tid := strings.TrimSpace(t.ID); tid != "" {
			turnID = tid
		}
		if turnID == "" {
			turnID = "turn"
		}

		turnMetadataJSON, err := marshalJSONObject(turnMetadataToMap(t.Metadata))
		if err != nil {
			return nil, errors.Wrap(err, "sqlite turn store: marshal turn metadata")
		}
		turnDataJSON, err := marshalJSONObject(turnDataToMap(t.Data))
		if err != nil {
			return nil, errors.Wrap(err, "sqlite turn store: marshal turn data")
		}

		if !opts.DryRun {
			if err := backfillUpsertTurnRow(ctx, tx, row, turnID, turnMetadataJSON, turnDataJSON); err != nil {
				return nil, err
			}
			result.TurnRowsUpserted++
		}

		if !opts.DryRun {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM turn_block_membership
				WHERE conv_id = ? AND session_id = ? AND turn_id = ? AND phase = ? AND snapshot_created_at_ms = ?
			`, row.convID, row.sessionID, turnID, row.phase, row.createdAtMs); err != nil {
				return nil, errors.Wrap(err, "sqlite turn store: clear existing membership rowset")
			}
		}

		for i, block := range t.Blocks {
			result.BlocksScanned++
			blockID := normalizeBlockID(block.ID, turnID, i)
			payloadMap := cloneStringAnyMap(block.Payload)
			blockMetadata := blockMetadataToMap(block.Metadata)

			contentHash, err := ComputeBlockContentHash(block.Kind.String(), block.Role, payloadMap, blockMetadata)
			if err != nil {
				return nil, errors.Wrap(err, "sqlite turn store: compute block content hash")
			}
			payloadJSON, err := marshalJSONObject(payloadMap)
			if err != nil {
				return nil, errors.Wrap(err, "sqlite turn store: marshal block payload")
			}
			blockMetadataJSON, err := marshalJSONObject(blockMetadata)
			if err != nil {
				return nil, errors.Wrap(err, "sqlite turn store: marshal block metadata")
			}

			if !opts.DryRun {
				if err := backfillUpsertBlockRow(ctx, tx, row, blockID, contentHash, block.Kind.String(), block.Role, payloadJSON, blockMetadataJSON); err != nil {
					return nil, err
				}
				result.BlockRowsUpserted++

				if _, err := tx.ExecContext(ctx, `
					INSERT OR REPLACE INTO turn_block_membership(
						conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal, block_id, content_hash
					) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
				`, row.convID, row.sessionID, turnID, row.phase, row.createdAtMs, i, blockID, contentHash); err != nil {
					return nil, errors.Wrap(err, "sqlite turn store: insert turn_block_membership")
				}
				result.MembershipInserted++
			}
		}

		result.SnapshotsBackfilled++
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: snapshot row iteration")
	}

	if opts.DryRun {
		return result, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: commit backfill tx")
	}
	committed = true
	return result, nil
}

func backfillUpsertTurnRow(ctx context.Context, tx *sql.Tx, row snapshotBackfillRow, turnID string, metadataJSON string, dataJSON string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO turns(
			conv_id, session_id, turn_id, turn_created_at_ms, turn_metadata_json, turn_data_json, updated_at_ms
		)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conv_id, session_id, turn_id) DO UPDATE SET
			turn_created_at_ms = MIN(turns.turn_created_at_ms, excluded.turn_created_at_ms),
			turn_metadata_json = excluded.turn_metadata_json,
			turn_data_json = excluded.turn_data_json,
			updated_at_ms = MAX(turns.updated_at_ms, excluded.updated_at_ms)
	`, row.convID, row.sessionID, turnID, row.createdAtMs, metadataJSON, dataJSON, row.createdAtMs); err != nil {
		return errors.Wrap(err, "sqlite turn store: upsert turns row")
	}
	return nil
}

func backfillUpsertBlockRow(ctx context.Context, tx *sql.Tx, row snapshotBackfillRow, blockID string, contentHash string, kind string, role string, payloadJSON string, metadataJSON string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO blocks(
			block_id, content_hash, hash_algorithm, kind, role, payload_json, block_metadata_json, first_seen_at_ms
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(block_id, content_hash) DO UPDATE SET
			kind = excluded.kind,
			role = excluded.role,
			payload_json = excluded.payload_json,
			block_metadata_json = excluded.block_metadata_json,
			first_seen_at_ms = MIN(blocks.first_seen_at_ms, excluded.first_seen_at_ms)
	`, blockID, contentHash, BlockContentHashAlgorithmV1, strings.TrimSpace(kind), strings.TrimSpace(role), payloadJSON, metadataJSON, row.createdAtMs); err != nil {
		return errors.Wrap(err, "sqlite turn store: upsert blocks row")
	}
	return nil
}

func normalizeBlockID(blockID string, turnID string, ordinal int) string {
	id := strings.TrimSpace(blockID)
	if id != "" {
		return id
	}
	return fmt.Sprintf("%s#%d", strings.TrimSpace(turnID), ordinal)
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func turnMetadataToMap(metadata turns.Metadata) map[string]any {
	out := map[string]any{}
	metadata.Range(func(key turns.TurnMetadataKey, value any) bool {
		out[string(key)] = value
		return true
	})
	return out
}

func turnDataToMap(data turns.Data) map[string]any {
	out := map[string]any{}
	data.Range(func(key turns.TurnDataKey, value any) bool {
		out[string(key)] = value
		return true
	})
	return out
}

func blockMetadataToMap(metadata turns.BlockMetadata) map[string]any {
	out := map[string]any{}
	metadata.Range(func(key turns.BlockMetadataKey, value any) bool {
		out[string(key)] = value
		return true
	})
	return out
}

func marshalJSONObject(v map[string]any) (string, error) {
	if len(v) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
