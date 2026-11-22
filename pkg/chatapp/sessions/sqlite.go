package sessions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // database/sql driver "sqlite3"
)

var errEmptyID = errors.New("sessions: session id is empty")

const sqliteColumns = "id, owner, labels, title, created_ms, updated_ms, usage, data"

// SQLiteStore keeps sessions in a SQLite database (table chatapp_sessions).
// Updates are serialized in-process, which is sufficient for one server
// process per database file.
type SQLiteStore struct {
	db  *sql.DB
	mu  sync.Mutex
	now func() time.Time
}

var _ Store = (*SQLiteStore)(nil)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS chatapp_sessions (
	id         TEXT PRIMARY KEY,
	owner      TEXT NOT NULL DEFAULT '',
	labels     TEXT NOT NULL DEFAULT '{}',
	title      TEXT NOT NULL DEFAULT '',
	created_ms INTEGER NOT NULL,
	updated_ms INTEGER NOT NULL,
	usage      TEXT NOT NULL DEFAULT '{}',
	data       TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS chatapp_sessions_owner ON chatapp_sessions(owner, created_ms);
`

// NewSQLiteStore opens (and creates if needed) a store at dsn, e.g.
// "file:/path/sessions.db?_journal_mode=WAL&_busy_timeout=5000".
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) Create(ctx context.Context, info Info) error {
	if strings.TrimSpace(info.ID) == "" {
		return errEmptyID
	}
	now := s.now()
	if info.Created.IsZero() {
		info.Created = now
	}
	if info.Updated.IsZero() {
		info.Updated = info.Created
	}
	labels, usage, err := encodeInfo(info)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chatapp_sessions WHERE id = ?`, info.ID).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return ErrExists
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO chatapp_sessions (id, owner, labels, title, created_ms, updated_ms, usage, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		info.ID, info.Owner, labels, info.Title, info.Created.UnixMilli(), info.Updated.UnixMilli(), usage, string(info.Data))
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (Info, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+sqliteColumns+` FROM chatapp_sessions WHERE id = ?`, id)
	info, err := scanInfo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Info{}, ErrNotFound
	}
	return info, err
}

func (s *SQLiteStore) List(ctx context.Context, f Filter) ([]Info, error) {
	query := `SELECT ` + sqliteColumns + ` FROM chatapp_sessions`
	args := []any{}
	if f.Owner != "" {
		query += ` WHERE owner = ?`
		args = append(args, f.Owner)
	}
	query += ` ORDER BY created_ms DESC, id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Info{}
	for rows.Next() {
		info, err := scanInfo(rows)
		if err != nil {
			return nil, err
		}
		if !f.Matches(info) {
			continue
		}
		out = append(out, info)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Update(ctx context.Context, id string, fn func(*Info) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := fn(&info); err != nil {
		return err
	}
	info.ID = id
	info.Updated = s.now()
	labels, usage, err := encodeInfo(info)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE chatapp_sessions SET owner = ?, labels = ?, title = ?, updated_ms = ?, usage = ?, data = ? WHERE id = ?`,
		info.Owner, labels, info.Title, info.Updated.UnixMilli(), usage, string(info.Data), id)
	return err
}

func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM chatapp_sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInfo(row rowScanner) (Info, error) {
	var (
		info                 Info
		labels, usage, data  string
		createdMs, updatedMs int64
	)
	if err := row.Scan(&info.ID, &info.Owner, &labels, &info.Title, &createdMs, &updatedMs, &usage, &data); err != nil {
		return Info{}, err
	}
	if data != "" {
		info.Data = json.RawMessage(data)
	}
	info.Created = time.UnixMilli(createdMs).UTC()
	info.Updated = time.UnixMilli(updatedMs).UTC()
	if err := json.Unmarshal([]byte(labels), &info.Labels); err != nil {
		return Info{}, err
	}
	if len(info.Labels) == 0 {
		info.Labels = nil
	}
	if err := json.Unmarshal([]byte(usage), &info.Usage); err != nil {
		return Info{}, err
	}
	return info, nil
}

func encodeInfo(info Info) (string, string, error) {
	labels := info.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	lb, err := json.Marshal(labels)
	if err != nil {
		return "", "", err
	}
	ub, err := json.Marshal(info.Usage)
	if err != nil {
		return "", "", err
	}
	return string(lb), string(ub), nil
}
