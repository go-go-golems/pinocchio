package chatstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type SQLiteTimelineStore struct {
	db *sql.DB
}

var _ TimelineStore = &SQLiteTimelineStore{}

func NewSQLiteTimelineStore(dsn string) (*SQLiteTimelineStore, error) {
	if dsn == "" {
		return nil, errors.New("sqlite timeline store: empty dsn")
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	s := &SQLiteTimelineStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteTimelineStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteTimelineStore) migrate() error {
	if s == nil || s.db == nil {
		return errors.New("sqlite timeline store: db is nil")
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS timeline_versions (
		  conv_id TEXT PRIMARY KEY,
		  version INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS timeline_entities (
		  conv_id TEXT NOT NULL,
		  entity_id TEXT NOT NULL,
		  kind TEXT NOT NULL,
		  created_at_ms INTEGER NOT NULL,
		  updated_at_ms INTEGER NOT NULL,
		  version INTEGER NOT NULL,
		  entity_json TEXT NOT NULL,
		  PRIMARY KEY (conv_id, entity_id)
		);`,
		`CREATE INDEX IF NOT EXISTS timeline_entities_by_version
		  ON timeline_entities(conv_id, version);`,
		`CREATE INDEX IF NOT EXISTS timeline_entities_by_created
		  ON timeline_entities(conv_id, created_at_ms);`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite timeline store: migrate")
		}
	}
	return nil
}

func (s *SQLiteTimelineStore) Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV1) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite timeline store: db is nil")
	}
	if convID == "" {
		return errors.New("sqlite timeline store: convID is empty")
	}
	if version == 0 {
		return errors.New("sqlite timeline store: version is 0")
	}
	if entity == nil {
		return errors.New("sqlite timeline store: entity is nil")
	}
	if entity.Id == "" {
		return errors.New("sqlite timeline store: entity.id is empty")
	}
	if entity.Kind == "" {
		return errors.New("sqlite timeline store: entity.kind is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current int64
	_ = tx.QueryRowContext(ctx, `SELECT version FROM timeline_versions WHERE conv_id = ?`, convID).Scan(&current)
	newVersion := current
	if version > uint64(current) {
		newVersion = int64(version)
	}

	var existingCreated int64
	err = tx.QueryRowContext(ctx, `SELECT created_at_ms FROM timeline_entities WHERE conv_id = ? AND entity_id = ?`, convID, entity.Id).
		Scan(&existingCreated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	createdAt := existingCreated
	if createdAt == 0 {
		if entity.CreatedAtMs > 0 {
			createdAt = entity.CreatedAtMs
		} else {
			createdAt = now
		}
	}

	entity.CreatedAtMs = createdAt
	entity.UpdatedAtMs = now

	entityJSON, err := protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false, // protojson lowerCamelCase
	}.Marshal(entity)
	if err != nil {
		return errors.Wrap(err, "sqlite timeline store: marshal entity")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO timeline_entities(conv_id, entity_id, kind, created_at_ms, updated_at_ms, version, entity_json)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conv_id, entity_id) DO UPDATE SET
		  kind = excluded.kind,
		  updated_at_ms = excluded.updated_at_ms,
		  version = excluded.version,
		  entity_json = excluded.entity_json
	`, convID, entity.Id, entity.Kind, createdAt, now, int64(version), string(entityJSON)); err != nil {
		return errors.Wrap(err, "sqlite timeline store: upsert entity")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO timeline_versions(conv_id, version)
		VALUES(?, ?)
		ON CONFLICT(conv_id) DO UPDATE SET version = excluded.version
	`, convID, newVersion); err != nil {
		return errors.Wrap(err, "sqlite timeline store: upsert version")
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteTimelineStore) GetSnapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite timeline store: db is nil")
	}
	if convID == "" {
		return nil, errors.New("sqlite timeline store: convID is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 5000
	}

	var current int64
	_ = s.db.QueryRowContext(ctx, `SELECT version FROM timeline_versions WHERE conv_id = ?`, convID).Scan(&current)

	var query string
	var args []any
	if sinceVersion == 0 {
		// Full snapshot in stable projection order (version).
		query = `
			SELECT entity_json
			FROM timeline_entities
			WHERE conv_id = ?
			ORDER BY version ASC, entity_id ASC
			LIMIT ?
		`
		args = []any{convID, limit}
	} else {
		// Incremental updates ordered by projection version.
		query = `
			SELECT entity_json
			FROM timeline_entities
			WHERE conv_id = ? AND version > ?
			ORDER BY version ASC
			LIMIT ?
		`
		args = []any{convID, sinceVersion, limit}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite timeline store: query snapshot")
	}
	defer func() { _ = rows.Close() }()

	entities := make([]*timelinepb.TimelineEntityV1, 0, 128)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var e timelinepb.TimelineEntityV1
		if err := (protojson.UnmarshalOptions{
			DiscardUnknown: true,
		}).Unmarshal([]byte(raw), &e); err != nil {
			return nil, errors.Wrap(err, "sqlite timeline store: unmarshal entity")
		}
		entities = append(entities, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &timelinepb.TimelineSnapshotV1{
		ConvId:       convID,
		Version:      uint64(current),
		ServerTimeMs: time.Now().UnixMilli(),
		Entities:     entities,
	}, nil
}

func SQLiteTimelineDSNForFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("sqlite timeline store: empty path")
	}
	// WAL for concurrent readers + writer. busy_timeout to avoid transient SQLITE_BUSY.
	return fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", path), nil
}
