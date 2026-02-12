package timeline

import (
	"context"
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

type TimelineListCommand struct {
	*cmds.CommandDescription
}

type TimelineListSettings struct {
	TimelineDSN  string `glazed:"timeline-dsn"`
	TimelineDB   string `glazed:"timeline-db"`
	ConvIDPrefix string `glazed:"conv-id-prefix"`
	Limit        int    `glazed:"limit"`
}

func NewTimelineListCommand() (*TimelineListCommand, error) {
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
		"list",
		cmds.WithShort("List conversations in the timeline store"),
		cmds.WithLong("List conversations with version and entity counts from the timeline store."),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TimelineListCommand{CommandDescription: desc}, nil
}

func (c *TimelineListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TimelineListSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}
	db, err := openTimelineDB(&StoreSettings{TimelineDSN: settings.TimelineDSN, TimelineDB: settings.TimelineDB})
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	query := strings.Builder{}
	query.WriteString(`
SELECT
  e.conv_id,
  COALESCE(v.version, MAX(e.version)) AS version,
  COUNT(*) AS entity_count,
  MIN(e.created_at_ms) AS min_created_at_ms,
  MAX(e.updated_at_ms) AS max_updated_at_ms
FROM timeline_entities e
LEFT JOIN timeline_versions v ON v.conv_id = e.conv_id
`)
	args := []any{}
	if settings.ConvIDPrefix != "" {
		query.WriteString("WHERE e.conv_id LIKE ?\n")
		args = append(args, settings.ConvIDPrefix+"%")
	}
	query.WriteString("GROUP BY e.conv_id, v.version\n")
	query.WriteString("ORDER BY e.conv_id ASC\n")
	if settings.Limit > 0 {
		query.WriteString("LIMIT ?\n")
		args = append(args, settings.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "timeline list query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		var version int64
		var entityCount int64
		var minCreated int64
		var maxUpdated int64
		if err := rows.Scan(&convID, &version, &entityCount, &minCreated, &maxUpdated); err != nil {
			return err
		}
		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("version", version),
			types.MRP("entity_count", entityCount),
			types.MRP("min_created_at_ms", minCreated),
			types.MRP("max_updated_at_ms", maxUpdated),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "timeline list rows")
	}

	return nil
}

var _ cmds.GlazeCommand = &TimelineListCommand{}
