package chatstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
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

func (s *SQLiteTimelineStore) UpsertConversation(ctx context.Context, record ConversationRecord) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite timeline store: db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UnixMilli()
	record = normalizeConversationRecord(record, now)
	if record.ConvID == "" {
		return errors.New("sqlite timeline store: convID is empty")
	}
	lastSeenVersion, err := uint64ToInt64(record.LastSeenVersion)
	if err != nil {
		return errors.Wrap(err, "sqlite timeline store: last_seen_version overflow")
	}
	hasTimeline := int64(0)
	if record.HasTimeline {
		hasTimeline = 1
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO timeline_conversations (
			conv_id, session_id, runtime_key, created_at_ms, last_activity_ms,
			last_seen_version, has_timeline, status, last_error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conv_id) DO UPDATE SET
			session_id = CASE
				WHEN excluded.session_id <> '' THEN excluded.session_id
				ELSE timeline_conversations.session_id
			END,
			runtime_key = CASE
				WHEN excluded.runtime_key <> '' THEN excluded.runtime_key
				ELSE timeline_conversations.runtime_key
			END,
			created_at_ms = CASE
				WHEN timeline_conversations.created_at_ms > 0 THEN timeline_conversations.created_at_ms
				ELSE excluded.created_at_ms
			END,
			last_activity_ms = CASE
				WHEN excluded.last_activity_ms > timeline_conversations.last_activity_ms THEN excluded.last_activity_ms
				ELSE timeline_conversations.last_activity_ms
			END,
			last_seen_version = CASE
				WHEN excluded.last_seen_version > timeline_conversations.last_seen_version THEN excluded.last_seen_version
				ELSE timeline_conversations.last_seen_version
			END,
			has_timeline = CASE
				WHEN excluded.has_timeline = 1 OR timeline_conversations.has_timeline = 1 THEN 1
				ELSE 0
			END,
			status = CASE
				WHEN excluded.status <> '' THEN excluded.status
				ELSE timeline_conversations.status
			END,
			last_error = CASE
				WHEN excluded.last_error <> '' THEN excluded.last_error
				ELSE timeline_conversations.last_error
			END
	`, record.ConvID, record.SessionID, record.RuntimeKey, record.CreatedAtMs, record.LastActivityMs, lastSeenVersion, hasTimeline, record.Status, record.LastError)
	if err != nil {
		return errors.Wrap(err, "sqlite timeline store: upsert conversation")
	}
	return nil
}

func (s *SQLiteTimelineStore) GetConversation(ctx context.Context, convID string) (ConversationRecord, bool, error) {
	if s == nil || s.db == nil {
		return ConversationRecord{}, false, errors.New("sqlite timeline store: db is nil")
	}
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return ConversationRecord{}, false, errors.New("sqlite timeline store: convID is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		record          ConversationRecord
		lastSeenVersion int64
		hasTimeline     int64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT conv_id, session_id, runtime_key, created_at_ms, last_activity_ms,
		       last_seen_version, has_timeline, status, last_error
		FROM timeline_conversations
		WHERE conv_id = ?
	`, convID).Scan(
		&record.ConvID,
		&record.SessionID,
		&record.RuntimeKey,
		&record.CreatedAtMs,
		&record.LastActivityMs,
		&lastSeenVersion,
		&hasTimeline,
		&record.Status,
		&record.LastError,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ConversationRecord{}, false, nil
	}
	if err != nil {
		return ConversationRecord{}, false, errors.Wrap(err, "sqlite timeline store: get conversation")
	}
	lastSeenU64, err := int64ToUint64(lastSeenVersion)
	if err != nil {
		return ConversationRecord{}, false, errors.Wrap(err, "sqlite timeline store: invalid conversation version")
	}
	record.LastSeenVersion = lastSeenU64
	record.HasTimeline = hasTimeline == 1
	if record.Status == "" {
		record.Status = "active"
	}
	return record, true, nil
}

func (s *SQLiteTimelineStore) ListConversations(ctx context.Context, limit int, sinceMs int64) ([]ConversationRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("sqlite timeline store: db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 200
	}

	query := `
		SELECT conv_id, session_id, runtime_key, created_at_ms, last_activity_ms,
		       last_seen_version, has_timeline, status, last_error
		FROM timeline_conversations
	`
	args := make([]any, 0, 2)
	if sinceMs > 0 {
		query += ` WHERE last_activity_ms >= ?`
		args = append(args, sinceMs)
	}
	query += ` ORDER BY last_activity_ms DESC, conv_id ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite timeline store: list conversations")
	}
	defer func() { _ = rows.Close() }()

	records := make([]ConversationRecord, 0, limit)
	for rows.Next() {
		var (
			record          ConversationRecord
			lastSeenVersion int64
			hasTimeline     int64
		)
		if err := rows.Scan(
			&record.ConvID,
			&record.SessionID,
			&record.RuntimeKey,
			&record.CreatedAtMs,
			&record.LastActivityMs,
			&lastSeenVersion,
			&hasTimeline,
			&record.Status,
			&record.LastError,
		); err != nil {
			return nil, errors.Wrap(err, "sqlite timeline store: scan conversation")
		}
		lastSeenU64, err := int64ToUint64(lastSeenVersion)
		if err != nil {
			return nil, errors.Wrap(err, "sqlite timeline store: invalid conversation version")
		}
		record.LastSeenVersion = lastSeenU64
		record.HasTimeline = hasTimeline == 1
		if record.Status == "" {
			record.Status = "active"
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "sqlite timeline store: iterate conversations")
	}
	return records, nil
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
		`CREATE TABLE IF NOT EXISTS timeline_conversations (
		  conv_id TEXT PRIMARY KEY,
		  session_id TEXT NOT NULL,
		  runtime_key TEXT NOT NULL DEFAULT '',
		  created_at_ms INTEGER NOT NULL,
		  last_activity_ms INTEGER NOT NULL,
		  last_seen_version INTEGER NOT NULL DEFAULT 0,
		  has_timeline INTEGER NOT NULL DEFAULT 1,
		  status TEXT NOT NULL DEFAULT 'active',
		  last_error TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS timeline_conversations_by_last_activity
		  ON timeline_conversations(last_activity_ms DESC, conv_id ASC);`,
		`CREATE INDEX IF NOT EXISTS timeline_conversations_by_session
		  ON timeline_conversations(session_id);`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return errors.Wrap(err, "sqlite timeline store: migrate")
		}
	}
	return nil
}

func (s *SQLiteTimelineStore) Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV2) error {
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
	versionI64, err := uint64ToInt64(version)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current int64
	_ = tx.QueryRowContext(ctx, `SELECT version FROM timeline_versions WHERE conv_id = ?`, convID).Scan(&current)
	newVersion := current
	if versionI64 > current {
		newVersion = versionI64
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
	`, convID, entity.Id, entity.Kind, createdAt, now, versionI64, string(entityJSON)); err != nil {
		return errors.Wrap(err, "sqlite timeline store: upsert entity")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO timeline_versions(conv_id, version)
		VALUES(?, ?)
		ON CONFLICT(conv_id) DO UPDATE SET version = excluded.version
	`, convID, newVersion); err != nil {
		return errors.Wrap(err, "sqlite timeline store: upsert version")
	}

	// Keep conversation index progression in sync with entity/version upserts.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO timeline_conversations (
			conv_id, session_id, runtime_key, created_at_ms, last_activity_ms,
			last_seen_version, has_timeline, status, last_error
		) VALUES (?, '', '', ?, ?, ?, 1, 'active', '')
		ON CONFLICT(conv_id) DO UPDATE SET
			created_at_ms = CASE
				WHEN timeline_conversations.created_at_ms > 0 THEN timeline_conversations.created_at_ms
				ELSE excluded.created_at_ms
			END,
			last_activity_ms = CASE
				WHEN excluded.last_activity_ms > timeline_conversations.last_activity_ms THEN excluded.last_activity_ms
				ELSE timeline_conversations.last_activity_ms
			END,
			last_seen_version = CASE
				WHEN excluded.last_seen_version > timeline_conversations.last_seen_version THEN excluded.last_seen_version
				ELSE timeline_conversations.last_seen_version
			END,
			has_timeline = 1
	`, convID, now, now, newVersion); err != nil {
		return errors.Wrap(err, "sqlite timeline store: upsert conversation progress")
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteTimelineStore) GetSnapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV2, error) {
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

	entities := make([]*timelinepb.TimelineEntityV2, 0, 128)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var e timelinepb.TimelineEntityV2
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

	versionU64, err := int64ToUint64(current)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite timeline store: invalid snapshot version")
	}

	return &timelinepb.TimelineSnapshotV2{
		ConvId:       convID,
		Version:      versionU64,
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

func uint64ToInt64(v uint64) (int64, error) {
	if v > math.MaxInt64 {
		return 0, errors.Errorf("value %d overflows int64", v)
	}
	return int64(v), nil
}

func int64ToUint64(v int64) (uint64, error) {
	if v < 0 {
		return 0, errors.Errorf("value %d cannot be represented as uint64", v)
	}
	return uint64(v), nil
}
