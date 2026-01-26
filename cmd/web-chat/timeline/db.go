package timeline

import (
	"database/sql"

	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
)

type StoreSettings struct {
	TimelineDSN string `glazed.parameter:"timeline-dsn"`
	TimelineDB  string `glazed.parameter:"timeline-db"`
}

func timelineStoreFlagDefs() []*parameters.ParameterDefinition {
	return []*parameters.ParameterDefinition{
		parameters.NewParameterDefinition(
			"timeline-dsn",
			parameters.ParameterTypeString,
			parameters.WithDefault(""),
			parameters.WithHelp("SQLite DSN for durable timeline snapshots (preferred over timeline-db)"),
		),
		parameters.NewParameterDefinition(
			"timeline-db",
			parameters.ParameterTypeString,
			parameters.WithDefault(""),
			parameters.WithHelp("SQLite DB file path for durable timeline snapshots (DSN derived with WAL/busy_timeout)"),
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
	return webchat.SQLiteTimelineDSNForFile(s.TimelineDB)
}

func openTimelineDB(settings *StoreSettings) (*sql.DB, error) {
	dsn, err := settings.resolveTimelineDSN()
	if err != nil {
		return nil, err
	}
	return sql.Open("sqlite3", dsn)
}

func openTimelineStore(settings *StoreSettings) (*webchat.SQLiteTimelineStore, error) {
	dsn, err := settings.resolveTimelineDSN()
	if err != nil {
		return nil, err
	}
	return webchat.NewSQLiteTimelineStore(dsn)
}
