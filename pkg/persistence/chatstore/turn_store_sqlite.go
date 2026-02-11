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

	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS turns (
		conv_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		turn_id TEXT NOT NULL,
		phase TEXT NOT NULL,
		created_at_ms INTEGER NOT NULL,
		payload TEXT NOT NULL,
		PRIMARY KEY (conv_id, session_id, turn_id, phase, created_at_ms)
	);`); err != nil {
		return errors.Wrap(err, "sqlite turn store: create table")
	}

	runIDExists, err := s.columnExists("turns", "run_id")
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: inspect run_id column")
	}
	sessionIDExists, err := s.columnExists("turns", "session_id")
	if err != nil {
		return errors.Wrap(err, "sqlite turn store: inspect session_id column")
	}

	if runIDExists && !sessionIDExists {
		if _, err := s.db.Exec(`ALTER TABLE turns RENAME COLUMN run_id TO session_id`); err != nil {
			return errors.Wrap(err, "sqlite turn store: rename run_id column")
		}
	}

	stmts := []string{
		`DROP INDEX IF EXISTS turns_by_run;`,
		`CREATE INDEX IF NOT EXISTS turns_by_conv ON turns(conv_id, created_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS turns_by_session ON turns(session_id, created_at_ms DESC);`,
		`CREATE INDEX IF NOT EXISTS turns_by_phase ON turns(phase, created_at_ms DESC);`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite turn store: migrate")
		}
	}
	return nil
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
		ctx = context.Background()
	}
	if createdAtMs <= 0 {
		createdAtMs = time.Now().UnixMilli()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO turns(conv_id, session_id, turn_id, phase, created_at_ms, payload)
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
		ctx = context.Background()
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
		FROM turns
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
