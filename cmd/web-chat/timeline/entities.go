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

type TimelineEntitiesCommand struct {
	*cmds.CommandDescription
}

type TimelineEntitiesSettings struct {
	TimelineDSN     string `glazed.parameter:"timeline-dsn"`
	TimelineDB      string `glazed.parameter:"timeline-db"`
	ConvID          string `glazed.parameter:"conv-id"`
	Kind            string `glazed.parameter:"kind"`
	EntityID        string `glazed.parameter:"entity-id"`
	VersionMin      int64  `glazed.parameter:"version-min"`
	VersionMax      int64  `glazed.parameter:"version-max"`
	CreatedAfterMs  int64  `glazed.parameter:"created-after-ms"`
	CreatedBeforeMs int64  `glazed.parameter:"created-before-ms"`
	UpdatedAfterMs  int64  `glazed.parameter:"updated-after-ms"`
	UpdatedBeforeMs int64  `glazed.parameter:"updated-before-ms"`
	Limit           int    `glazed.parameter:"limit"`
	Order           string `glazed.parameter:"order"`
	IncludeJSON     bool   `glazed.parameter:"include-json"`
	IncludeSummary  bool   `glazed.parameter:"include-summary"`
}

func NewTimelineEntitiesCommand() (*TimelineEntitiesCommand, error) {
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
			"conv-id",
			parameters.ParameterTypeString,
			parameters.WithHelp("Conversation ID to query"),
		),
		parameters.NewParameterDefinition(
			"kind",
			parameters.ParameterTypeString,
			parameters.WithDefault(""),
			parameters.WithHelp("Filter by entity kind"),
		),
		parameters.NewParameterDefinition(
			"entity-id",
			parameters.ParameterTypeString,
			parameters.WithDefault(""),
			parameters.WithHelp("Filter by entity ID"),
		),
		parameters.NewParameterDefinition(
			"version-min",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Minimum entity version"),
		),
		parameters.NewParameterDefinition(
			"version-max",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Maximum entity version"),
		),
		parameters.NewParameterDefinition(
			"created-after-ms",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Only include entities created after this timestamp (ms)"),
		),
		parameters.NewParameterDefinition(
			"created-before-ms",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Only include entities created before this timestamp (ms)"),
		),
		parameters.NewParameterDefinition(
			"updated-after-ms",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Only include entities updated after this timestamp (ms)"),
		),
		parameters.NewParameterDefinition(
			"updated-before-ms",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Only include entities updated before this timestamp (ms)"),
		),
		parameters.NewParameterDefinition(
			"limit",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(5000),
			parameters.WithHelp("Maximum number of entities to return"),
		),
		parameters.NewParameterDefinition(
			"order",
			parameters.ParameterTypeString,
			parameters.WithDefault("version"),
			parameters.WithHelp("Order by version|created|updated (prefix with - for desc)"),
		),
		parameters.NewParameterDefinition(
			"include-json",
			parameters.ParameterTypeBool,
			parameters.WithDefault(false),
			parameters.WithHelp("Include raw entity JSON"),
		),
		parameters.NewParameterDefinition(
			"include-summary",
			parameters.ParameterTypeBool,
			parameters.WithDefault(true),
			parameters.WithHelp("Include a human summary"),
		),
	)

	desc := cmds.NewCommandDescription(
		"entities",
		cmds.WithShort("List timeline entities for a conversation"),
		cmds.WithLong("List timeline entities with optional filters and ordering."),
		cmds.WithFlags(flags...),
		cmds.WithLayersList(glazedLayer, commandSettingsLayer),
	)

	return &TimelineEntitiesCommand{CommandDescription: desc}, nil
}

func (c *TimelineEntitiesCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	settings := &TimelineEntitiesSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, settings); err != nil {
		return err
	}
	convID := strings.TrimSpace(settings.ConvID)
	if convID == "" {
		return errors.New("conv-id is required")
	}
	order, err := orderClause(settings.Order)
	if err != nil {
		return err
	}

	db, err := openTimelineDB(&StoreSettings{TimelineDSN: settings.TimelineDSN, TimelineDB: settings.TimelineDB})
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	query := strings.Builder{}
	query.WriteString(`
SELECT entity_id, kind, created_at_ms, updated_at_ms, version, entity_json
FROM timeline_entities
WHERE conv_id = ?
`)
	args := []any{convID}

	if settings.Kind != "" {
		query.WriteString("AND kind = ?\n")
		args = append(args, settings.Kind)
	}
	if settings.EntityID != "" {
		query.WriteString("AND entity_id = ?\n")
		args = append(args, settings.EntityID)
	}
	if settings.VersionMin > 0 {
		query.WriteString("AND version >= ?\n")
		args = append(args, settings.VersionMin)
	}
	if settings.VersionMax > 0 {
		query.WriteString("AND version <= ?\n")
		args = append(args, settings.VersionMax)
	}
	if settings.CreatedAfterMs > 0 {
		query.WriteString("AND created_at_ms >= ?\n")
		args = append(args, settings.CreatedAfterMs)
	}
	if settings.CreatedBeforeMs > 0 {
		query.WriteString("AND created_at_ms <= ?\n")
		args = append(args, settings.CreatedBeforeMs)
	}
	if settings.UpdatedAfterMs > 0 {
		query.WriteString("AND updated_at_ms >= ?\n")
		args = append(args, settings.UpdatedAfterMs)
	}
	if settings.UpdatedBeforeMs > 0 {
		query.WriteString("AND updated_at_ms <= ?\n")
		args = append(args, settings.UpdatedBeforeMs)
	}

	query.WriteString("ORDER BY " + order + ", entity_id ASC\n")
	if settings.Limit > 0 {
		query.WriteString("LIMIT ?\n")
		args = append(args, settings.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "timeline entities query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var entityID string
		var kind string
		var createdAt int64
		var updatedAt int64
		var version int64
		var raw string
		if err := rows.Scan(&entityID, &kind, &createdAt, &updatedAt, &version, &raw); err != nil {
			return err
		}

		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("entity_id", entityID),
			types.MRP("kind", kind),
			types.MRP("created_at_ms", createdAt),
			types.MRP("updated_at_ms", updatedAt),
			types.MRP("version", version),
		)

		if settings.IncludeSummary {
			summary, err := summarizeEntity(raw)
			if err != nil {
				row.Set("summary_error", err.Error())
			} else {
				row.Set("summary", summary)
			}
		}
		if settings.IncludeJSON {
			row.Set("entity_json", raw)
		}

		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "timeline entities rows")
	}

	return nil
}

var _ cmds.GlazeCommand = &TimelineEntitiesCommand{}
