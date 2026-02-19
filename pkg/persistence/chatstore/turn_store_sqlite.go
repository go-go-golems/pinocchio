package chatstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type SQLiteTurnStore struct {
	db *sql.DB
}

var _ TurnStore = &SQLiteTurnStore{}

var sqliteSchemaIntrospectionTables = map[string]struct{}{
	"turns":                 {},
	"blocks":                {},
	"turn_block_membership": {},
}

type snapshotBackfillRow struct {
	convID      string
	sessionID   string
	turnID      string
	phase       string
	createdAtMs int64
}

func NewSQLiteTurnStore(dsn string) (*SQLiteTurnStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("sqlite turn store: empty dsn")
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	s := &SQLiteTurnStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteTurnStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteTurnStore) migrate() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite turn store: db is nil")
	}

	createTableStmts := []string{
		`CREATE TABLE IF NOT EXISTS turns (
			conv_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			turn_created_at_ms INTEGER NOT NULL,
			turn_metadata_json TEXT NOT NULL DEFAULT '{}',
			turn_data_json TEXT NOT NULL DEFAULT '{}',
			updated_at_ms INTEGER NOT NULL,
			PRIMARY KEY (conv_id, session_id, turn_id)
		);`,
		`CREATE TABLE IF NOT EXISTS blocks (
			block_id TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			hash_algorithm TEXT NOT NULL DEFAULT 'sha256-canonical-json-v1',
			kind TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			block_metadata_json TEXT NOT NULL DEFAULT '{}',
			first_seen_at_ms INTEGER NOT NULL,
			PRIMARY KEY (block_id, content_hash)
		);`,
		`CREATE TABLE IF NOT EXISTS turn_block_membership (
			conv_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			phase TEXT NOT NULL,
			snapshot_created_at_ms INTEGER NOT NULL,
			ordinal INTEGER NOT NULL,
			block_id TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			PRIMARY KEY (conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal),
			FOREIGN KEY (conv_id, session_id, turn_id) REFERENCES turns(conv_id, session_id, turn_id) ON DELETE CASCADE,
			FOREIGN KEY (block_id, content_hash) REFERENCES blocks(block_id, content_hash)
		);`,
	}
	for _, st := range createTableStmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite turn store: migrate")
		}
	}

	if err := s.ensureTurnsTableColumns(); err != nil {
		return errors.Wrap(err, "sqlite turn store: ensure turns columns")
	}

	createIndexStmts := []string{
		`CREATE INDEX IF NOT EXISTS turns_by_conv_session ON turns(conv_id, session_id, updated_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS turns_by_session ON turns(session_id, updated_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS blocks_by_kind_role ON blocks(kind, role);`,
		`CREATE INDEX IF NOT EXISTS turn_block_membership_by_turn_phase ON turn_block_membership(conv_id, session_id, turn_id, phase, snapshot_created_at_ms DESC, ordinal);`,
		`CREATE INDEX IF NOT EXISTS turn_block_membership_by_block ON turn_block_membership(block_id, content_hash);`,
	}
	for _, st := range createIndexStmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite turn store: migrate")
		}
	}

	return nil
}

func (s *SQLiteTurnStore) ensureTurnsTableColumns() error {
	cols, err := s.tableColumns("turns")
	if err != nil {
		return err
	}

	if !cols["turn_created_at_ms"] {
		if _, err := s.db.Exec(`ALTER TABLE turns ADD COLUMN turn_created_at_ms INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		if cols["created_at_ms"] {
			if _, err := s.db.Exec(`UPDATE turns SET turn_created_at_ms = CASE WHEN turn_created_at_ms > 0 THEN turn_created_at_ms ELSE COALESCE(created_at_ms, 0) END`); err != nil {
				return err
			}
		}
	}

	if !cols["turn_metadata_json"] {
		if _, err := s.db.Exec(`ALTER TABLE turns ADD COLUMN turn_metadata_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !cols["turn_data_json"] {
		if _, err := s.db.Exec(`ALTER TABLE turns ADD COLUMN turn_data_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	if !cols["updated_at_ms"] {
		if _, err := s.db.Exec(`ALTER TABLE turns ADD COLUMN updated_at_ms INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`
		UPDATE turns
		SET updated_at_ms = CASE
			WHEN COALESCE(updated_at_ms, 0) > 0 THEN updated_at_ms
			WHEN COALESCE(turn_created_at_ms, 0) > 0 THEN turn_created_at_ms
			ELSE CAST(strftime('%s','now') AS INTEGER) * 1000
		END
	`); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteTurnStore) tableColumns(table string) (map[string]bool, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite turn store: db is nil")
	}
	table, err := normalizeSQLiteIntrospectionTable(table)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[strings.ToLower(strings.TrimSpace(name))] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeSQLiteIntrospectionTable(table string) (string, error) {
	table = strings.ToLower(strings.TrimSpace(table))
	if _, ok := sqliteSchemaIntrospectionTables[table]; !ok {
		return "", errors.Errorf("sqlite turn store: unsupported table for schema introspection: %q", table)
	}
	return table, nil
}

func (s *SQLiteTurnStore) Save(ctx context.Context, convID, sessionID, turnID, phase string, createdAtMs int64, payload string) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite turn store: db is nil")
	}
	if strings.TrimSpace(convID) == "" {
		return errors.New("sqlite turn store: convID is empty")
	}
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("sqlite turn store: sessionID is empty")
	}
	if strings.TrimSpace(turnID) == "" {
		return errors.New("sqlite turn store: turnID is empty")
	}
	if strings.TrimSpace(phase) == "" {
		return errors.New("sqlite turn store: phase is empty")
	}
	if ctx == nil {
		return errors.New("sqlite turn store: ctx is nil")
	}
	if createdAtMs <= 0 {
		createdAtMs = time.Now().UnixMilli()
	}

	t, err := serde.FromYAML([]byte(payload))
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: parse payload yaml")
	}
	if t == nil {
		return errors.New("sqlite turn store: parse payload yaml: decoded nil turn")
	}

	row := snapshotBackfillRow{
		convID:      convID,
		sessionID:   sessionID,
		turnID:      turnID,
		phase:       phase,
		createdAtMs: createdAtMs,
	}
	_, err = s.persistNormalizedSnapshot(ctx, row, t)
	return err
}

func (s *SQLiteTurnStore) persistNormalizedSnapshot(ctx context.Context, row snapshotBackfillRow, t *turns.Turn) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.Wrap(err, "sqlite turn store: begin tx")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	turnID := strings.TrimSpace(row.turnID)
	if tid := strings.TrimSpace(t.ID); tid != "" {
		turnID = tid
	}
	if turnID == "" {
		turnID = "turn"
	}

	turnMetadataJSON, err := marshalJSONObject(turnMetadataToMap(t.Metadata))
	if err != nil {
		return 0, errors.Wrap(err, "sqlite turn store: marshal turn metadata")
	}
	turnDataJSON, err := marshalJSONObject(turnDataToMap(t.Data))
	if err != nil {
		return 0, errors.Wrap(err, "sqlite turn store: marshal turn data")
	}

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
	`, row.convID, row.sessionID, turnID, row.createdAtMs, turnMetadataJSON, turnDataJSON, row.createdAtMs); err != nil {
		return 0, errors.Wrap(err, "sqlite turn store: upsert turns row")
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM turn_block_membership
		WHERE conv_id = ? AND session_id = ? AND turn_id = ? AND phase = ? AND snapshot_created_at_ms = ?
	`, row.convID, row.sessionID, turnID, row.phase, row.createdAtMs); err != nil {
		return 0, errors.Wrap(err, "sqlite turn store: clear existing membership rowset")
	}

	membershipInserted := 0
	for i, block := range t.Blocks {
		blockID := normalizeBlockID(block.ID, turnID, i)
		payloadMap := cloneStringAnyMap(block.Payload)
		blockMetadata := blockMetadataToMap(block.Metadata)

		contentHash, err := ComputeBlockContentHash(block.Kind.String(), block.Role, payloadMap, blockMetadata)
		if err != nil {
			return 0, errors.Wrap(err, "sqlite turn store: compute block content hash")
		}
		payloadJSON, err := marshalJSONObject(payloadMap)
		if err != nil {
			return 0, errors.Wrap(err, "sqlite turn store: marshal block payload")
		}
		blockMetadataJSON, err := marshalJSONObject(blockMetadata)
		if err != nil {
			return 0, errors.Wrap(err, "sqlite turn store: marshal block metadata")
		}

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
		`, blockID, contentHash, BlockContentHashAlgorithmV1, strings.TrimSpace(block.Kind.String()), strings.TrimSpace(block.Role), payloadJSON, blockMetadataJSON, row.createdAtMs); err != nil {
			return 0, errors.Wrap(err, "sqlite turn store: upsert blocks row")
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO turn_block_membership(
				conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal, block_id, content_hash
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		`, row.convID, row.sessionID, turnID, row.phase, row.createdAtMs, i, blockID, contentHash); err != nil {
			return 0, errors.Wrap(err, "sqlite turn store: insert turn_block_membership")
		}
		membershipInserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, errors.Wrap(err, "sqlite turn store: commit tx")
	}
	committed = true
	return membershipInserted, nil
}

func (s *SQLiteTurnStore) List(ctx context.Context, q TurnQuery) ([]TurnSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite turn store: db is nil")
	}
	if strings.TrimSpace(q.ConvID) == "" && strings.TrimSpace(q.SessionID) == "" {
		return nil, errors.New("sqlite turn store: convID or sessionID required")
	}
	if ctx == nil {
		return nil, errors.New("sqlite turn store: ctx is nil")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 200
	}

	clauses := []string{}
	args := []any{}
	if v := strings.TrimSpace(q.ConvID); v != "" {
		clauses = append(clauses, "m.conv_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.SessionID); v != "" {
		clauses = append(clauses, "m.session_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Phase); v != "" {
		clauses = append(clauses, "m.phase = ?")
		args = append(args, v)
	}
	if q.SinceMs > 0 {
		clauses = append(clauses, "m.snapshot_created_at_ms >= ?")
		args = append(args, q.SinceMs)
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			m.conv_id,
			m.session_id,
			m.turn_id,
			m.phase,
			m.snapshot_created_at_ms,
			COALESCE(MAX(t.turn_metadata_json), '{}') AS turn_metadata_json,
			COALESCE(MAX(t.turn_data_json), '{}') AS turn_data_json
		FROM turn_block_membership m
		LEFT JOIN turns t
			ON t.conv_id = m.conv_id
			AND t.session_id = m.session_id
			AND t.turn_id = m.turn_id
		%s
		GROUP BY
			m.conv_id,
			m.session_id,
			m.turn_id,
			m.phase,
			m.snapshot_created_at_ms
		ORDER BY m.snapshot_created_at_ms DESC
		LIMIT ?
	`, where)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: query")
	}
	defer func() { _ = rows.Close() }()

	items := []TurnSnapshot{}
	for rows.Next() {
		var (
			item             TurnSnapshot
			turnMetadataJSON string
			turnDataJSON     string
		)
		if err := rows.Scan(
			&item.ConvID,
			&item.SessionID,
			&item.TurnID,
			&item.Phase,
			&item.CreatedAtMs,
			&turnMetadataJSON,
			&turnDataJSON,
		); err != nil {
			return nil, err
		}

		blockRows, err := s.loadSnapshotBlocks(ctx, item.ConvID, item.SessionID, item.TurnID, item.Phase, item.CreatedAtMs)
		if err != nil {
			return nil, err
		}
		payload, err := buildTurnPayloadYAML(item.TurnID, blockRows, turnMetadataJSON, turnDataJSON)
		if err != nil {
			return nil, err
		}
		item.Payload = payload
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func SQLiteTurnDSNForFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("sqlite turn store: empty path")
	}
	return fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", path), nil
}

func (s *SQLiteTurnStore) loadSnapshotBlocks(ctx context.Context, convID string, sessionID string, turnID string, phase string, snapshotCreatedAtMs int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			m.ordinal,
			b.block_id,
			b.kind,
			b.role,
			COALESCE(b.payload_json, '{}') AS payload_json,
			COALESCE(b.block_metadata_json, '{}') AS block_metadata_json
		FROM turn_block_membership m
		JOIN blocks b
			ON b.block_id = m.block_id
			AND b.content_hash = m.content_hash
		WHERE
			m.conv_id = ?
			AND m.session_id = ?
			AND m.turn_id = ?
			AND m.phase = ?
			AND m.snapshot_created_at_ms = ?
		ORDER BY m.ordinal ASC
	`, convID, sessionID, turnID, phase, snapshotCreatedAtMs)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: query snapshot blocks")
	}
	defer func() { _ = rows.Close() }()

	blocks := make([]map[string]any, 0, 16)
	for rows.Next() {
		var (
			ordinal           int
			blockID           string
			kind              string
			role              string
			payloadJSON       string
			blockMetadataJSON string
		)
		if err := rows.Scan(&ordinal, &blockID, &kind, &role, &payloadJSON, &blockMetadataJSON); err != nil {
			return nil, errors.Wrap(err, "sqlite turn store: scan snapshot block")
		}
		_ = ordinal
		payloadMap, err := parseJSONObject(payloadJSON)
		if err != nil {
			return nil, errors.Wrap(err, "sqlite turn store: parse block payload json")
		}
		metadataMap, err := parseJSONObject(blockMetadataJSON)
		if err != nil {
			return nil, errors.Wrap(err, "sqlite turn store: parse block metadata json")
		}
		block := map[string]any{
			"id":   blockID,
			"kind": kind,
		}
		if strings.TrimSpace(role) != "" {
			block["role"] = role
		}
		if len(payloadMap) > 0 {
			block["payload"] = payloadMap
		}
		if len(metadataMap) > 0 {
			block["metadata"] = metadataMap
		}
		blocks = append(blocks, block)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "sqlite turn store: iterate snapshot blocks")
	}
	return blocks, nil
}

func buildTurnPayloadYAML(turnID string, blocks []map[string]any, turnMetadataJSON string, turnDataJSON string) (string, error) {
	payload := map[string]any{
		"id":     turnID,
		"blocks": blocks,
	}
	turnMetadata, err := parseJSONObject(turnMetadataJSON)
	if err != nil {
		return "", err
	}
	if len(turnMetadata) > 0 {
		payload["metadata"] = turnMetadata
	}
	turnData, err := parseJSONObject(turnDataJSON)
	if err != nil {
		return "", err
	}
	if len(turnData) > 0 {
		payload["data"] = turnData
	}
	b, err := yaml.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseJSONObject(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
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
