package timeline

import (
	"database/sql"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

type StoreSettings struct {
	TimelineDSN string `glazed:"timeline-dsn"`
	TimelineDB  string `glazed:"timeline-db"`
}

func timelineStoreFlagDefs() []*fields.Definition {
	return []*fields.Definition{
		fields.New(
			"timeline-dsn",
			fields.TypeString,
			fields.WithDefault(""),
			fields.WithHelp("SQLite DSN for durable timeline snapshots (preferred over timeline-db)"),
		),
		fields.New(
			"timeline-db",
			fields.TypeString,
			fields.WithDefault(""),
			fields.WithHelp("SQLite DB file path for durable timeline snapshots (DSN derived with WAL/busy_timeout)"),
		),
	}
}

func (s *StoreSettings) resolveTimelineDSN() (string, error) {
	if s == nil {
		return "", errors.New("timeline store settings are nil")
	}
	if s.TimelineDSN != "" {
		return s.TimelineDSN, nil
	}
	if s.TimelineDB == "" {
		return "", errors.New("timeline store not configured (set --timeline-dsn or --timeline-db)")
	}
	return chatstore.SQLiteTimelineDSNForFile(s.TimelineDB)
}

func openTimelineDB(settings *StoreSettings) (*sql.DB, error) {
	dsn, err := settings.resolveTimelineDSN()
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite3", dsn)
}

func openTimelineStore(settings *StoreSettings) (*chatstore.SQLiteTimelineStore, error) {
	dsn, err := settings.resolveTimelineDSN()
	if err != nil {
		return nil, err
	}
	return chatstore.NewSQLiteTimelineStore(dsn)
}
