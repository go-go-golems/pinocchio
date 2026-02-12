package timeline

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type TimelineStatsCommand struct {
	*cmds.CommandDescription
}

type TimelineStatsSettings struct {
	TimelineDSN  string `glazed:"timeline-dsn"`
	TimelineDB   string `glazed:"timeline-db"`
	ConvIDPrefix string `glazed:"conv-id-prefix"`
	Limit        int    `glazed:"limit"`
}

func NewTimelineStatsCommand() (*TimelineStatsCommand, error) {
	glazedLayer, err := settings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	commandSettingsLayer, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	flags := append(timelineStoreFlagDefs(),
		fields.New(
			"conv-id-prefix",
			fields.TypeString,
			fields.WithDefault(""),
			fields.WithHelp("Filter conversations by prefix"),
		),
		fields.New(
			"limit",
			fields.TypeInteger,
			fields.WithDefault(200),
			fields.WithHelp("Limit number of conversations (0 = no limit)"),
		),
	)

	desc := cmds.NewCommandDescription(
		"stats",
		cmds.WithShort("Summarize timeline entities per conversation"),
		cmds.WithLong("Aggregate entity counts and timestamps per conversation."),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TimelineStatsCommand{CommandDescription: desc}, nil
}

func (c *TimelineStatsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TimelineStatsSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}

	db, err := openTimelineDB(&StoreSettings{TimelineDSN: settings.TimelineDSN, TimelineDB: settings.TimelineDB})
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	versionMap, err := fetchTimelineVersions(ctx, db, settings.ConvIDPrefix)
	if err != nil {
		return err
	}
	stats, err := fetchTimelineStats(ctx, db, settings.ConvIDPrefix)
	if err != nil {
		return err
	}

	for convID, version := range versionMap {
		if _, ok := stats[convID]; !ok {
			stats[convID] = &convStats{convID: convID, version: version, kindCounts: map[string]int64{}}
		}
	}

	convIDs := make([]string, 0, len(stats))
	for convID := range stats {
		convIDs = append(convIDs, convID)
	}
	sort.Strings(convIDs)
	if settings.Limit > 0 && len(convIDs) > settings.Limit {
		convIDs = convIDs[:settings.Limit]
	}

	for _, convID := range convIDs {
		st := stats[convID]
		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("version", st.version),
			types.MRP("entity_count", st.total),
			types.MRP("min_created_at_ms", st.minCreated),
			types.MRP("max_updated_at_ms", st.maxUpdated),
			types.MRP("kind_counts", st.kindCounts),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

type convStats struct {
	convID     string
	version    int64
	total      int64
	minCreated int64
	maxUpdated int64
	kindCounts map[string]int64
}

func fetchTimelineVersions(ctx context.Context, db *sql.DB, prefix string) (map[string]int64, error) {
	query := strings.Builder{}
	query.WriteString("SELECT conv_id, version FROM timeline_versions\n")
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("WHERE conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}
	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, errors.Wrap(err, "timeline versions query failed")
	}
	defer func() { _ = rows.Close() }()

	versions := map[string]int64{}
	for rows.Next() {
		var convID string
		var version int64
		if err := rows.Scan(&convID, &version); err != nil {
			return nil, err
		}
		versions[convID] = version
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "timeline versions rows")
	}

	return versions, nil
}

func fetchTimelineStats(ctx context.Context, db *sql.DB, prefix string) (map[string]*convStats, error) {
	query := strings.Builder{}
	query.WriteString(`
SELECT conv_id, kind, COUNT(*) AS count, MIN(created_at_ms) AS min_created, MAX(updated_at_ms) AS max_updated
FROM timeline_entities
`)
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("WHERE conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}
	query.WriteString("GROUP BY conv_id, kind\n")

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, errors.Wrap(err, "timeline stats query failed")
	}
	defer func() { _ = rows.Close() }()

	stats := map[string]*convStats{}
	for rows.Next() {
		var convID string
		var kind string
		var count int64
		var minCreated int64
		var maxUpdated int64
		if err := rows.Scan(&convID, &kind, &count, &minCreated, &maxUpdated); err != nil {
			return nil, err
		}
		st := stats[convID]
		if st == nil {
			st = &convStats{convID: convID, kindCounts: map[string]int64{}}
			stats[convID] = st
		}
		st.kindCounts[kind] = count
		st.total += count
		if st.minCreated == 0 || (minCreated > 0 && minCreated < st.minCreated) {
			st.minCreated = minCreated
		}
		if maxUpdated > st.maxUpdated {
			st.maxUpdated = maxUpdated
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "timeline stats rows")
	}

	return stats, nil
}

var _ cmds.GlazeCommand = &TimelineStatsCommand{}
