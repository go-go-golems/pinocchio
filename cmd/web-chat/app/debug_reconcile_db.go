package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func (r *StreamDebugRecorder) BuildSQLiteReconcileDB(ctx context.Context, sessionID string, body io.Reader, provider DebugDataProvider) ([]byte, error) {
	frontendRecords, err := parseFrontendLogUpload(body)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "pinocchio-stream-debug-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "stream-debug.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	if err := createDebugSQLiteSchema(ctx, db); err != nil {
		return nil, err
	}
	backendRecords := r.Records(sessionID, "")
	if err := insertDebugSQLiteMeta(ctx, db, sessionID, len(backendRecords), len(frontendRecords)); err != nil {
		return nil, err
	}
	if err := insertBackendDebugRecords(ctx, db, backendRecords); err != nil {
		return nil, err
	}
	if err := insertFrontendDebugRecords(ctx, db, frontendRecords); err != nil {
		return nil, err
	}
	if provider != nil {
		if err := insertTimelineEntities(ctx, db, provider, sessionID); err != nil {
			return nil, fmt.Errorf("insert timeline: %w", err)
		}
		if err := insertTurns(ctx, db, provider, sessionID); err != nil {
			return nil, fmt.Errorf("insert turns: %w", err)
		}
	}
	if err := createDebugSQLiteViews(ctx, db); err != nil {
		return nil, fmt.Errorf("create views: %w", err)
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
