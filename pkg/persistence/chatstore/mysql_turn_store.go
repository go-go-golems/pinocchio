package chatstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/geppetto/pkg/turns/serde"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

// MySQLTurnStore is a MySQL/Aurora implementation of TurnStore. It is a
// dialect translation of SQLiteTurnStore: the same three tables (turns, blocks,
// turn_block_membership), the same columns, and the same single-transaction
// Save (upsert turn -> replace membership rowset -> upsert blocks + membership).
// Its schema is component-versioned independently from sessionstream hydration.
const mysqlTurnSchemaVersion int64 = 1

const mysqlTurnSchemaComponent = "chatstore.turns"

// MySQLTurnStore is selected only by an explicit MySQL StoreSpec in the
// serverkit composition layer.
//
// The shared chatstore helpers (block content hashing, JSON marshaling,
// metadata extraction, payload YAML reconstruction) are reused unchanged, so
// the only differences from SQLiteTurnStore are the SQL statements and the
// MySQL connection pool. SQLite-isms translated:
//
//   - INSERT ... ON CONFLICT(cols) DO UPDATE SET excluded.x -> INSERT ... AS new
//     ON DUPLICATE KEY UPDATE x = new.x (existing row via table-qualified name)
//   - INSERT OR REPLACE INTO -> INSERT ... ON DUPLICATE KEY UPDATE
//   - single sqlite connection -> bounded MySQL pool
type MySQLTurnStore struct {
	db *sql.DB
}

var _ TurnStore = &MySQLTurnStore{}

// NewMySQLTurnStore opens a bounded MySQL pool, migrates the three-table schema
// if needed, and returns a TurnStore. The dsn must be a go-sql-driver/mysql DSN
// with parseTime=true.
func NewMySQLTurnStore(ctx context.Context, dsn string) (*MySQLTurnStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("mysql turn store: empty dsn")
	}
	db, err := openChatstoreMySQLPool(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "mysql turn store: open db")
	}
	s := &MySQLTurnStore{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, errors.Wrap(err, "mysql turn store: migrate")
	}
	return s, nil
}

func (s *MySQLTurnStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *MySQLTurnStore) migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("mysql turn store: db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	versionTableExists, err := s.tableExists(ctx, "pinocchio_schema_version")
	if err != nil {
		return errors.Wrap(err, "inspect schema version table")
	}
	managedTablesExist, err := s.anyManagedTurnTableExists(ctx)
	if err != nil {
		return errors.Wrap(err, "inspect turn tables")
	}

	if !versionTableExists {
		if managedTablesExist {
			return errors.New("mysql turn store: unversioned prototype schema detected; recreate the database or migrate it explicitly")
		}
		if _, err := s.db.ExecContext(ctx, `
			CREATE TABLE pinocchio_schema_version (
				component VARBINARY(64) NOT NULL PRIMARY KEY,
				schema_version BIGINT NOT NULL
			) ENGINE=InnoDB;
		`); err != nil {
			return errors.Wrap(err, "create schema version table")
		}
		for _, st := range mysqlTurnCreateTableStatements {
			if _, err := s.db.ExecContext(ctx, st); err != nil {
				return errors.Wrap(err, "create turn schema")
			}
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO pinocchio_schema_version(component, schema_version) VALUES(?, ?)
		`, mysqlTurnSchemaComponent, mysqlTurnSchemaVersion); err != nil {
			return errors.Wrap(err, "record turn schema version")
		}
		return nil
	}

	var version int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT schema_version FROM pinocchio_schema_version WHERE component = ?
	`, mysqlTurnSchemaComponent).Scan(&version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("mysql turn store: schema version component chatstore.turns is missing")
		}
		return errors.Wrap(err, "read turn schema version")
	}
	if version != mysqlTurnSchemaVersion {
		return errors.Errorf("mysql turn store: unsupported chatstore.turns schema version %d (want %d)", version, mysqlTurnSchemaVersion)
	}
	for _, table := range []string{"turns", "blocks", "turn_block_membership"} {
		exists, err := s.tableExists(ctx, table)
		if err != nil {
			return errors.Wrapf(err, "inspect managed table %s", table)
		}
		if !exists {
			return errors.Errorf("mysql turn store: schema version %d is recorded but managed table %s is missing", version, table)
		}
	}
	return nil
}

var mysqlTurnCreateTableStatements = []string{
	`CREATE TABLE turns (
		conv_id VARBINARY(255) NOT NULL,
		session_id VARBINARY(255) NOT NULL,
		turn_id VARBINARY(255) NOT NULL,
		turn_created_at_ms BIGINT NOT NULL,
		turn_metadata_json MEDIUMTEXT NOT NULL,
		turn_data_json MEDIUMTEXT NOT NULL,
		runtime_key VARBINARY(255) NOT NULL DEFAULT '',
		inference_id VARBINARY(255) NOT NULL DEFAULT '',
		updated_at_ms BIGINT NOT NULL,
		PRIMARY KEY (conv_id, session_id, turn_id),
		KEY turns_by_conv_session (conv_id, session_id, updated_at_ms),
		KEY turns_by_session (session_id, updated_at_ms),
		KEY turns_by_conv_runtime (conv_id, runtime_key, updated_at_ms),
		KEY turns_by_conv_inference (conv_id, inference_id, updated_at_ms)
	) ENGINE=InnoDB;`,
	`CREATE TABLE blocks (
		block_id VARBINARY(255) NOT NULL,
		content_hash VARBINARY(64) NOT NULL,
		hash_algorithm VARBINARY(64) NOT NULL DEFAULT 'sha256-canonical-json-v1',
		kind VARBINARY(128) NOT NULL,
		role VARBINARY(128) NOT NULL DEFAULT '',
		payload_json MEDIUMTEXT NOT NULL,
		block_metadata_json MEDIUMTEXT NOT NULL,
		first_seen_at_ms BIGINT NOT NULL,
		PRIMARY KEY (block_id, content_hash),
		KEY blocks_by_kind_role (kind, role)
	) ENGINE=InnoDB;`,
	`CREATE TABLE turn_block_membership (
		conv_id VARBINARY(255) NOT NULL,
		session_id VARBINARY(255) NOT NULL,
		turn_id VARBINARY(255) NOT NULL,
		phase VARBINARY(64) NOT NULL,
		snapshot_created_at_ms BIGINT NOT NULL,
		ordinal INT NOT NULL,
		block_id VARBINARY(255) NOT NULL,
		content_hash VARBINARY(64) NOT NULL,
		PRIMARY KEY (conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal),
		KEY tmem_by_block (block_id, content_hash)
	) ENGINE=InnoDB;`,
}

func (s *MySQLTurnStore) tableExists(ctx context.Context, table string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?
	`, table).Scan(&count)
	return count > 0, err
}

func (s *MySQLTurnStore) anyManagedTurnTableExists(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_name IN ('turns', 'blocks', 'turn_block_membership')
	`).Scan(&count)
	return count > 0, err
}

func validateMySQLOpaqueField(name, value string, maxBytes int) error {
	if len([]byte(value)) > maxBytes {
		return errors.Errorf("mysql turn store: %s exceeds %d-byte limit", name, maxBytes)
	}
	return nil
}

func (s *MySQLTurnStore) Save(ctx context.Context, convID, sessionID, turnID, phase string, createdAtMs int64, payload string, opts TurnSaveOptions) error {
	if s == nil || s.db == nil {
		return errors.New("mysql turn store: db is nil")
	}
	if strings.TrimSpace(convID) == "" {
		return errors.New("mysql turn store: convID is empty")
	}
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("mysql turn store: sessionID is empty")
	}
	if strings.TrimSpace(turnID) == "" {
		return errors.New("mysql turn store: turnID is empty")
	}
	if strings.TrimSpace(phase) == "" {
		return errors.New("mysql turn store: phase is empty")
	}
	if ctx == nil {
		return errors.New("mysql turn store: ctx is nil")
	}
	if err := validateMySQLOpaqueField("convID", convID, 255); err != nil {
		return err
	}
	if err := validateMySQLOpaqueField("sessionID", sessionID, 255); err != nil {
		return err
	}
	if err := validateMySQLOpaqueField("turnID", turnID, 255); err != nil {
		return err
	}
	if err := validateMySQLOpaqueField("phase", phase, 64); err != nil {
		return err
	}
	if err := validateMySQLOpaqueField("runtimeKey", opts.RuntimeKey, 255); err != nil {
		return err
	}
	if err := validateMySQLOpaqueField("inferenceID", opts.InferenceID, 255); err != nil {
		return err
	}
	if createdAtMs <= 0 {
		createdAtMs = time.Now().UnixMilli()
	}

	t, err := serde.FromYAML([]byte(payload))
	if err != nil {
		return errors.Wrap(err, "mysql turn store: parse payload yaml")
	}
	if t == nil {
		return errors.New("mysql turn store: parse payload yaml: decoded nil turn")
	}

	row := snapshotBackfillRow{
		convID:      convID,
		sessionID:   sessionID,
		turnID:      turnID,
		phase:       phase,
		runtimeKey:  opts.RuntimeKey,
		inferenceID: opts.InferenceID,
		createdAtMs: createdAtMs,
	}
	if row.runtimeKey == "" {
		row.runtimeKey = runtimeKeyFromTurnMetadata(t.Metadata)
	}
	if row.inferenceID == "" {
		row.inferenceID = inferenceIDFromTurnMetadata(t.Metadata)
	}
	if err := validateMySQLOpaqueField("runtimeKey", row.runtimeKey, 255); err != nil {
		return err
	}
	if err := validateMySQLOpaqueField("inferenceID", row.inferenceID, 255); err != nil {
		return err
	}
	_, err = s.persistNormalizedSnapshot(ctx, row, t)
	return err
}

// persistNormalizedSnapshot mirrors SQLiteTurnStore.persistNormalizedSnapshot
// with MySQL SQL. It runs in one transaction: upsert the turn row, delete the
// prior membership rowset for this (turn, phase, snapshot), then upsert blocks
// and insert membership rows. On any error the transaction rolls back.
func (s *MySQLTurnStore) persistNormalizedSnapshot(ctx context.Context, row snapshotBackfillRow, t *turns.Turn) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.Wrap(err, "mysql turn store: begin tx")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	turnID := row.turnID
	if t.ID != "" {
		turnID = t.ID
	}
	if turnID == "" {
		turnID = "turn"
	}
	if err := validateMySQLOpaqueField("turnID", turnID, 255); err != nil {
		return 0, err
	}

	turnMetadataJSON, err := marshalJSONObject(turnMetadataToMap(t.Metadata))
	if err != nil {
		return 0, errors.Wrap(err, "mysql turn store: marshal turn metadata")
	}
	turnDataJSON, err := marshalJSONObject(turnDataToMap(t.Data))
	if err != nil {
		return 0, errors.Wrap(err, "mysql turn store: marshal turn data")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO turns(
			conv_id, session_id, turn_id, turn_created_at_ms, turn_metadata_json, turn_data_json, runtime_key, inference_id, updated_at_ms
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?) AS new
		ON DUPLICATE KEY UPDATE
			turn_created_at_ms = LEAST(turns.turn_created_at_ms, new.turn_created_at_ms),
			turn_metadata_json = new.turn_metadata_json,
			turn_data_json = new.turn_data_json,
			runtime_key = CASE
				WHEN new.runtime_key <> '' THEN new.runtime_key
				ELSE turns.runtime_key
			END,
			inference_id = CASE
				WHEN new.inference_id <> '' THEN new.inference_id
				ELSE turns.inference_id
			END,
			updated_at_ms = GREATEST(turns.updated_at_ms, new.updated_at_ms)
	`, row.convID, row.sessionID, turnID, row.createdAtMs, turnMetadataJSON, turnDataJSON, row.runtimeKey, row.inferenceID, row.createdAtMs); err != nil {
		return 0, errors.Wrap(err, "mysql turn store: upsert turns row")
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM turn_block_membership
		WHERE conv_id = ? AND session_id = ? AND turn_id = ? AND phase = ? AND snapshot_created_at_ms = ?
	`, row.convID, row.sessionID, turnID, row.phase, row.createdAtMs); err != nil {
		return 0, errors.Wrap(err, "mysql turn store: clear existing membership rowset")
	}

	membershipInserted := 0
	for i, block := range t.Blocks {
		blockID := normalizeMySQLBlockID(block.ID, turnID, i)
		if err := validateMySQLOpaqueField("blockID", blockID, 255); err != nil {
			return 0, err
		}
		payloadMap := cloneStringAnyMap(block.Payload)
		blockMetadata := blockMetadataToMap(block.Metadata)
		if err := validateMySQLOpaqueField("kind", block.Kind.String(), 128); err != nil {
			return 0, err
		}
		if err := validateMySQLOpaqueField("role", block.Role, 128); err != nil {
			return 0, err
		}

		contentHash, err := ComputeBlockContentHash(block.Kind.String(), block.Role, payloadMap, blockMetadata)
		if err != nil {
			return 0, errors.Wrap(err, "mysql turn store: compute block content hash")
		}
		payloadJSON, err := marshalJSONObject(payloadMap)
		if err != nil {
			return 0, errors.Wrap(err, "mysql turn store: marshal block payload")
		}
		blockMetadataJSON, err := marshalJSONObject(blockMetadata)
		if err != nil {
			return 0, errors.Wrap(err, "mysql turn store: marshal block metadata")
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO blocks(
				block_id, content_hash, hash_algorithm, kind, role, payload_json, block_metadata_json, first_seen_at_ms
			)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?) AS new
			ON DUPLICATE KEY UPDATE
				kind = new.kind,
				role = new.role,
				payload_json = new.payload_json,
				block_metadata_json = new.block_metadata_json,
				first_seen_at_ms = LEAST(blocks.first_seen_at_ms, new.first_seen_at_ms)
		`, blockID, contentHash, BlockContentHashAlgorithmV1, block.Kind.String(), block.Role, payloadJSON, blockMetadataJSON, row.createdAtMs); err != nil {
			return 0, errors.Wrap(err, "mysql turn store: upsert blocks row")
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO turn_block_membership(
				conv_id, session_id, turn_id, phase, snapshot_created_at_ms, ordinal, block_id, content_hash
			)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				block_id = VALUES(block_id),
				content_hash = VALUES(content_hash)
		`, row.convID, row.sessionID, turnID, row.phase, row.createdAtMs, i, blockID, contentHash); err != nil {
			return 0, errors.Wrap(err, "mysql turn store: insert turn_block_membership")
		}
		membershipInserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, errors.Wrap(err, "mysql turn store: commit tx")
	}
	committed = true
	return membershipInserted, nil
}

func (s *MySQLTurnStore) List(ctx context.Context, q TurnQuery) ([]TurnSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("mysql turn store: db is nil")
	}
	if strings.TrimSpace(q.ConvID) == "" && strings.TrimSpace(q.SessionID) == "" {
		return nil, errors.New("mysql turn store: convID or sessionID required")
	}
	if ctx == nil {
		return nil, errors.New("mysql turn store: ctx is nil")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 200
	}

	clauses := []string{}
	args := []any{}
	if v := q.ConvID; strings.TrimSpace(v) != "" {
		clauses = append(clauses, "m.conv_id = ?")
		args = append(args, v)
	}
	if v := q.SessionID; strings.TrimSpace(v) != "" {
		clauses = append(clauses, "m.session_id = ?")
		args = append(args, v)
	}
	if v := q.Phase; strings.TrimSpace(v) != "" {
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

	// #nosec G201 -- where only interpolates constant clause fragments; values remain parameterized in args.
	query := fmt.Sprintf(`
		SELECT
			m.conv_id,
			m.session_id,
			m.turn_id,
			m.phase,
			m.snapshot_created_at_ms,
			COALESCE(MAX(t.runtime_key), '') AS runtime_key,
			COALESCE(MAX(t.inference_id), '') AS inference_id,
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
		return nil, errors.Wrap(err, "mysql turn store: query")
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
			&item.RuntimeKey,
			&item.InferenceID,
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

func (s *MySQLTurnStore) LoadLatestTurn(ctx context.Context, convID, phase string) (*TurnSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("mysql turn store: db is nil")
	}
	if strings.TrimSpace(convID) == "" {
		return nil, errors.New("mysql turn store: convID is empty")
	}
	if ctx == nil {
		return nil, errors.New("mysql turn store: ctx is nil")
	}
	clauses := []string{"m.conv_id = ?"}
	args := []any{convID}
	if phase != "" {
		clauses = append(clauses, "m.phase = ?")
		args = append(args, phase)
	}
	where := "WHERE " + strings.Join(clauses, " AND ")

	// #nosec G201 -- where only interpolates constant clause fragments; values remain parameterized in args.
	query := fmt.Sprintf(`
		SELECT
			m.conv_id,
			m.session_id,
			m.turn_id,
			m.phase,
			m.snapshot_created_at_ms,
			COALESCE(MAX(t.runtime_key), '') AS runtime_key,
			COALESCE(MAX(t.inference_id), '') AS inference_id,
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
		LIMIT 1
	`, where)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "mysql turn store: load latest turn")
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, nil
	}

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
		&item.RuntimeKey,
		&item.InferenceID,
		&turnMetadataJSON,
		&turnDataJSON,
	); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
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
	return &item, nil
}

func (s *MySQLTurnStore) loadSnapshotBlocks(ctx context.Context, convID string, sessionID string, turnID string, phase string, snapshotCreatedAtMs int64) ([]map[string]any, error) {
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
		return nil, errors.Wrap(err, "mysql turn store: query snapshot blocks")
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
			return nil, errors.Wrap(err, "mysql turn store: scan snapshot block")
		}
		_ = ordinal
		payloadMap, err := parseJSONObject(payloadJSON)
		if err != nil {
			return nil, errors.Wrap(err, "mysql turn store: parse block payload json")
		}
		metadataMap, err := parseJSONObject(blockMetadataJSON)
		if err != nil {
			return nil, errors.Wrap(err, "mysql turn store: parse block metadata json")
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
		return nil, errors.Wrap(err, "mysql turn store: iterate snapshot blocks")
	}
	return blocks, nil
}

func normalizeMySQLBlockID(blockID string, turnID string, ordinal int) string {
	if blockID != "" {
		return blockID
	}
	return fmt.Sprintf("%s#%d", turnID, ordinal)
}

// openChatstoreMySQLPool opens a bounded database/sql pool for the mysql
// driver. Pool sizing matches the design (20 open / 10 idle / 5m lifetime).
func openChatstoreMySQLPool(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)
	return db, nil
}
