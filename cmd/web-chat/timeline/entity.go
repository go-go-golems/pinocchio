package timeline

import (
	"context"
	"database/sql"
	stderrors "errors"
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

type TimelineEntityCommand struct {
	*cmds.CommandDescription
}

type TimelineEntitySettings struct {
	TimelineDSN    string `glazed:"timeline-dsn"`
	TimelineDB     string `glazed:"timeline-db"`
	ConvID         string `glazed:"conv-id"`
	EntityID       string `glazed:"entity-id"`
	IncludeJSON    bool   `glazed:"include-json"`
	IncludeSummary bool   `glazed:"include-summary"`
}

func NewTimelineEntityCommand() (*TimelineEntityCommand, error) {
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
			fields.WithHelp("Conversation ID"),
		),
		fields.New(
			"entity-id",
			fields.TypeString,
			fields.WithHelp("Entity ID to fetch"),
		),
		fields.New(
			"include-json",
			fields.TypeBool,
			fields.WithDefault(true),
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
		"entity",
		cmds.WithShort("Fetch a single timeline entity"),
		cmds.WithLong("Fetch a single timeline entity by ID."),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TimelineEntityCommand{CommandDescription: desc}, nil
}

func (c *TimelineEntityCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TimelineEntitySettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}
	convID := strings.TrimSpace(settings.ConvID)
	if convID == "" {
		return errors.New("conv-id is required")
	}
	entityID := strings.TrimSpace(settings.EntityID)
	if entityID == "" {
		return errors.New("entity-id is required")
	}

	db, err := openTimelineDB(&StoreSettings{TimelineDSN: settings.TimelineDSN, TimelineDB: settings.TimelineDB})
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	row, err := fetchEntityRow(ctx, db, convID, entityID)
	if err != nil {
		return err
	}

	out := types.NewRow(
		types.MRP("conv_id", convID),
		types.MRP("entity_id", row.entityID),
		types.MRP("kind", row.kind),
		types.MRP("created_at_ms", row.createdAt),
		types.MRP("updated_at_ms", row.updatedAt),
		types.MRP("version", row.version),
	)

	if settings.IncludeSummary {
		summary, err := summarizeEntity(row.rawJSON)
		if err != nil {
			out.Set("summary_error", err.Error())
		} else {
			out.Set("summary", summary)
		}
	}
	if settings.IncludeJSON {
		out.Set("entity_json", row.rawJSON)
	}

	return gp.AddRow(ctx, out)
}

type entityRow struct {
	entityID  string
	kind      string
	createdAt int64
	updatedAt int64
	version   int64
	rawJSON   string
}

func fetchEntityRow(ctx context.Context, db *sql.DB, convID, entityID string) (*entityRow, error) {
	var row entityRow
	query := `
SELECT entity_id, kind, created_at_ms, updated_at_ms, version, entity_json
FROM timeline_entities
WHERE conv_id = ? AND entity_id = ?
LIMIT 1
`
	if err := db.QueryRowContext(ctx, query, convID, entityID).Scan(&row.entityID, &row.kind, &row.createdAt, &row.updatedAt, &row.version, &row.rawJSON); err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return nil, errors.Errorf("entity not found: %s", entityID)
		}
		return nil, err
	}
	return &row, nil
}

var _ cmds.GlazeCommand = &TimelineEntityCommand{}
