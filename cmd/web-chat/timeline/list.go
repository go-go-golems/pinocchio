package timeline

import (
	"context"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type TimelineListCommand struct {
	*cmds.CommandDescription
}

type TimelineListSettings struct {
	TimelineDSN  string `glazed.parameter:"timeline-dsn"`
	TimelineDB   string `glazed.parameter:"timeline-db"`
	ConvIDPrefix string `glazed.parameter:"conv-id-prefix"`
	Limit        int    `glazed.parameter:"limit"`
}

func NewTimelineListCommand() (*TimelineListCommand, error) {
	glazedLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, err
	}
	commandSettingsLayer, err := cli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}

	flags := append(timelineStoreFlagDefs(),
		parameters.NewParameterDefinition(
			"conv-id-prefix",
			parameters.ParameterTypeString,
			parameters.WithDefault(""),
			parameters.WithHelp("Filter conversations by prefix"),
		),
		parameters.NewParameterDefinition(
			"limit",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(200),
			parameters.WithHelp("Limit number of conversations (0 = no limit)"),
		),
	)

	desc := cmds.NewCommandDescription(
		"list",
		cmds.WithShort("List conversations in the timeline store"),
		cmds.WithLong("List conversations with version and entity counts from the timeline store."),
		cmds.WithFlags(flags...),
		cmds.WithLayersList(glazedLayer, commandSettingsLayer),
	)

	return &TimelineListCommand{CommandDescription: desc}, nil
}

func (c *TimelineListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	settings := &TimelineListSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, settings); err != nil {
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
