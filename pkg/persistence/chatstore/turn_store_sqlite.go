package chatstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

type SQLiteTurnStore struct {
	db *sql.DB
}

var _ TurnStore = &SQLiteTurnStore{}

const (
	legacyTurnSnapshotsTable = "turn_snapshots"
	normalizedTurnsTable     = "turns"
)

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

	if err := s.migrateLegacySnapshotTable(); err != nil {
		return err
	}

	stmts := []string{
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
		`CREATE INDEX IF NOT EXISTS turns_by_conv_session ON turns(conv_id, session_id, updated_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS turns_by_session ON turns(session_id, updated_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS blocks_by_kind_role ON blocks(kind, role);`,
		`CREATE INDEX IF NOT EXISTS turn_block_membership_by_turn_phase ON turn_block_membership(conv_id, session_id, turn_id, phase, snapshot_created_at_ms DESC, ordinal);`,
		`CREATE INDEX IF NOT EXISTS turn_block_membership_by_block ON turn_block_membership(block_id, content_hash);`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite turn store: migrate")
		}
	}
	return nil
}

func (s *SQLiteTurnStore) migrateLegacySnapshotTable() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite turn store: db is nil")
	}

	legacyTurnsExists, err := s.tableExists(normalizedTurnsTable)
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: inspect turns table")
	}
	snapshotsExists, err := s.tableExists(legacyTurnSnapshotsTable)
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: inspect turn_snapshots table")
	}
	if legacyTurnsExists && !snapshotsExists {
		legacyHasPayload, err := s.columnExists(normalizedTurnsTable, "payload")
		if err != nil {
			return errors.Wrap(err, "sqlite turn store: inspect legacy turns payload")
		}
		if legacyHasPayload {
			if _, err := s.db.Exec(`ALTER TABLE turns RENAME TO turn_snapshots`); err != nil {
				return errors.Wrap(err, "sqlite turn store: rename turns to turn_snapshots")
			}
			snapshotsExists = true
		}
	}

	if !snapshotsExists {
		if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS turn_snapshots (
		conv_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		turn_id TEXT NOT NULL,
		phase TEXT NOT NULL,
		created_at_ms INTEGER NOT NULL,
		payload TEXT NOT NULL,
		PRIMARY KEY (conv_id, session_id, turn_id, phase, created_at_ms)
	);`); err != nil {
			return errors.Wrap(err, "sqlite turn store: create turn_snapshots table")
		}
	}

	runIDExists, err := s.columnExists(legacyTurnSnapshotsTable, "run_id")
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: inspect run_id column")
	}
	sessionIDExists, err := s.columnExists(legacyTurnSnapshotsTable, "session_id")
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: inspect session_id column")
	}

	if runIDExists && !sessionIDExists {
		if _, err := s.db.Exec(`ALTER TABLE turn_snapshots RENAME COLUMN run_id TO session_id`); err != nil {
			return errors.Wrap(err, "sqlite turn store: rename run_id column")
		}
	}

	legacyStmts := []string{
		`DROP INDEX IF EXISTS turns_by_run;`,
		`DROP INDEX IF EXISTS turn_snapshots_by_run;`,
		`CREATE INDEX IF NOT EXISTS turn_snapshots_by_conv ON turn_snapshots(conv_id, created_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS turn_snapshots_by_session ON turn_snapshots(session_id, created_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS turn_snapshots_by_phase ON turn_snapshots(phase, created_at_ms DESC);`,
	}
	for _, st := range legacyStmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite turn store: migrate legacy turn_snapshots")
		}
	}
	return nil
}

func (s *SQLiteTurnStore) tableExists(table string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("sqlite turn store: db is nil")
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *SQLiteTurnStore) columnExists(table string, column string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("sqlite turn store: db is nil")
	}
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			cid       int
			name      string
			typeName  string
			notNull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
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

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO turn_snapshots(conv_id, session_id, turn_id, phase, created_at_ms, payload)
		VALUES(?, ?, ?, ?, ?, ?)
	`, convID, sessionID, turnID, phase, createdAtMs, payload)
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: insert")
	}
	return nil
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
		clauses = append(clauses, "conv_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.SessionID); v != "" {
		clauses = append(clauses, "session_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Phase); v != "" {
		clauses = append(clauses, "phase = ?")
		args = append(args, v)
	}
	if q.SinceMs > 0 {
		clauses = append(clauses, "created_at_ms >= ?")
		args = append(args, q.SinceMs)
	}

	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT conv_id, session_id, turn_id, phase, created_at_ms, payload
		FROM turn_snapshots
		%s
		ORDER BY created_at_ms DESC
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
		var item TurnSnapshot
		if err := rows.Scan(&item.ConvID, &item.SessionID, &item.TurnID, &item.Phase, &item.CreatedAtMs, &item.Payload); err != nil {
			return nil, err
		}
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
