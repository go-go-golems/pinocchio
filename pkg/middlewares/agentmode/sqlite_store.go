package agentmode

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements Store using a SQLite database.
type SQLiteStore struct{ db *sql.DB }

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS agent_mode_changes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT,
  turn_id TEXT,
  from_mode TEXT,
  to_mode TEXT,
  analysis TEXT,
  at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_mode_changes_session_id_at ON agent_mode_changes(session_id, at);
`)
	if err != nil {
		return err
	}

	if hasRunID, err := s.columnExists("agent_mode_changes", "run_id"); err != nil {
		return err
	} else if hasRunID {
		if hasSessionID, err := s.columnExists("agent_mode_changes", "session_id"); err != nil {
			return err
		} else if !hasSessionID {
			if _, err := s.db.Exec(`ALTER TABLE agent_mode_changes RENAME COLUMN run_id TO session_id`); err != nil {
				return err
			}
		}
	}

	if _, err := s.db.Exec(`DROP INDEX IF EXISTS idx_agent_mode_changes_run_id_at`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_mode_changes_session_id_at ON agent_mode_changes(session_id, at)`); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) columnExists(table string, column string) (bool, error) {
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

func (s *SQLiteStore) GetCurrentMode(ctx context.Context, sessionID string) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT to_mode FROM agent_mode_changes WHERE session_id = ? ORDER BY at DESC, id DESC LIMIT 1`, sessionID)
	var mode string
	switch err := row.Scan(&mode); err {
	case nil:
		return mode, nil
	case sql.ErrNoRows:
		return "", nil
	default:
		return "", err
	}
}

func (s *SQLiteStore) RecordModeChange(ctx context.Context, change ModeChange) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_mode_changes (session_id, turn_id, from_mode, to_mode, analysis, at) VALUES (?, ?, ?, ?, ?, ?)`,
		change.SessionID, change.TurnID, change.FromMode, change.ToMode, change.Analysis, change.At.Format(time.RFC3339Nano))
	return err
}
