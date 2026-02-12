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

type TimelineEntitiesCommand struct {
	*cmds.CommandDescription
}

type TimelineEntitiesSettings struct {
	TimelineDSN     string `glazed:"timeline-dsn"`
	TimelineDB      string `glazed:"timeline-db"`
	ConvID          string `glazed:"conv-id"`
	Kind            string `glazed:"kind"`
	EntityID        string `glazed:"entity-id"`
	VersionMin      int64  `glazed:"version-min"`
	VersionMax      int64  `glazed:"version-max"`
	CreatedAfterMs  int64  `glazed:"created-after-ms"`
	CreatedBeforeMs int64  `glazed:"created-before-ms"`
	UpdatedAfterMs  int64  `glazed:"updated-after-ms"`
	UpdatedBeforeMs int64  `glazed:"updated-before-ms"`
	Limit           int    `glazed:"limit"`
	Order           string `glazed:"order"`
	IncludeJSON     bool   `glazed:"include-json"`
	IncludeSummary  bool   `glazed:"include-summary"`
}

func NewTimelineEntitiesCommand() (*TimelineEntitiesCommand, error) {
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
			"conv-id",
			fields.TypeString,
			fields.WithHelp("Conversation ID to query"),
		),
		fields.New(
			"kind",
			fields.TypeString,
			fields.WithDefault(""),
			fields.WithHelp("Filter by entity kind"),
		),
		fields.New(
			"entity-id",
			fields.TypeString,
			fields.WithDefault(""),
			fields.WithHelp("Filter by entity ID"),
		),
		fields.New(
			"version-min",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Minimum entity version"),
		),
		fields.New(
			"version-max",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Maximum entity version"),
		),
		fields.New(
			"created-after-ms",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Only include entities created after this timestamp (ms)"),
		),
		fields.New(
			"created-before-ms",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Only include entities created before this timestamp (ms)"),
		),
		fields.New(
			"updated-after-ms",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Only include entities updated after this timestamp (ms)"),
		),
		fields.New(
			"updated-before-ms",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Only include entities updated before this timestamp (ms)"),
		),
		fields.New(
			"limit",
			fields.TypeInteger,
			fields.WithDefault(5000),
			fields.WithHelp("Maximum number of entities to return"),
		),
		fields.New(
			"order",
			fields.TypeString,
			fields.WithDefault("version"),
			fields.WithHelp("Order by version|created|updated (prefix with - for desc)"),
		),
		fields.New(
			"include-json",
			fields.TypeBool,
			fields.WithDefault(false),
			fields.WithHelp("Include raw entity JSON"),
		),
		fields.New(
			"include-summary",
			fields.TypeBool,
			fields.WithDefault(true),
			fields.WithHelp("Include a human summary"),
		),
	)

	desc := cmds.NewCommandDescription(
		"entities",
		cmds.WithShort("List timeline entities for a conversation"),
		cmds.WithLong("List timeline entities with optional filters and ordering."),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TimelineEntitiesCommand{CommandDescription: desc}, nil
}

func (c *TimelineEntitiesCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TimelineEntitiesSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
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
