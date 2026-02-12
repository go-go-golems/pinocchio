package timeline

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type TimelineVerifyCommand struct {
	*cmds.CommandDescription
}

type TimelineVerifySettings struct {
	TimelineDSN  string `glazed:"timeline-dsn"`
	TimelineDB   string `glazed:"timeline-db"`
	ConvIDPrefix string `glazed:"conv-id-prefix"`
	Limit        int    `glazed:"limit"`
	CheckJSON    bool   `glazed:"check-json"`
	EmitOK       bool   `glazed:"emit-ok"`
}

func NewTimelineVerifyCommand() (*TimelineVerifyCommand, error) {
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
			fields.WithHelp("Maximum number of issues to return (0 = no limit)"),
		),
		fields.New(
			"check-json",
			fields.TypeBool,
			fields.WithDefault(false),
			fields.WithHelp("Attempt to parse entity_json and report errors"),
		),
		fields.New(
			"emit-ok",
			fields.TypeBool,
			fields.WithDefault(false),
			fields.WithHelp("Emit a single ok row when no issues are found"),
		),
	)

	desc := cmds.NewCommandDescription(
		"verify",
		cmds.WithShort("Run consistency checks on timeline persistence"),
		cmds.WithLong("Check for version mismatches, missing metadata, and malformed JSON."),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TimelineVerifyCommand{CommandDescription: desc}, nil
}

func (c *TimelineVerifyCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TimelineVerifySettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}

	db, err := openTimelineDB(&StoreSettings{TimelineDSN: settings.TimelineDSN, TimelineDB: settings.TimelineDB})
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	limit := settings.Limit
	issueCount := 0
	addIssue := func(row types.Row) error {
		if limit > 0 && issueCount >= limit {
			return errLimitReached
		}
		issueCount++
		return gp.AddRow(ctx, row)
	}

	if err := checkMissingVersions(ctx, db, settings.ConvIDPrefix, addIssue); err != nil {
		if err != errLimitReached {
			return err
		}
		return nil
	}
	if err := checkVersionMismatch(ctx, db, settings.ConvIDPrefix, addIssue); err != nil {
		if err != errLimitReached {
			return err
		}
		return nil
	}
	if err := checkTimestampOrder(ctx, db, settings.ConvIDPrefix, addIssue); err != nil {
		if err != errLimitReached {
			return err
		}
		return nil
	}
	if err := checkEmptyKinds(ctx, db, settings.ConvIDPrefix, addIssue); err != nil {
		if err != errLimitReached {
			return err
		}
		return nil
	}
	if settings.CheckJSON {
		if err := checkJSON(ctx, db, settings.ConvIDPrefix, addIssue); err != nil {
			if err != errLimitReached {
				return err
			}
			return nil
		}
	}

	if issueCount == 0 && settings.EmitOK {
		row := types.NewRow(
			types.MRP("status", "ok"),
			types.MRP("issues", 0),
		)
		return gp.AddRow(ctx, row)
	}

	return nil
}

var errLimitReached = errors.New("issue limit reached")

func checkMissingVersions(ctx context.Context, db *sql.DB, prefix string, addIssue func(types.Row) error) error {
	query := strings.Builder{}
	query.WriteString(`
SELECT e.conv_id, COUNT(*) AS entity_count
FROM timeline_entities e
LEFT JOIN timeline_versions v ON v.conv_id = e.conv_id
WHERE v.conv_id IS NULL
`)
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("AND e.conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}
	query.WriteString("GROUP BY e.conv_id\n")

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "missing versions query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		var count int64
		if err := rows.Scan(&convID, &count); err != nil {
			return err
		}
		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("issue", "missing_version_row"),
			types.MRP("details", map[string]any{"entity_count": count}),
		)
		if err := addIssue(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "missing versions rows")
	}
	return nil
}

func checkVersionMismatch(ctx context.Context, db *sql.DB, prefix string, addIssue func(types.Row) error) error {
	query := strings.Builder{}
	query.WriteString(`
SELECT e.conv_id, MAX(e.version) AS max_version, v.version AS current_version
FROM timeline_entities e
JOIN timeline_versions v ON v.conv_id = e.conv_id
`)
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("WHERE e.conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}
	query.WriteString("GROUP BY e.conv_id, v.version\n")
	query.WriteString("HAVING max_version != v.version\n")

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "version mismatch query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		var maxVersion int64
		var currentVersion int64
		if err := rows.Scan(&convID, &maxVersion, &currentVersion); err != nil {
			return err
		}
		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("issue", "version_mismatch"),
			types.MRP("details", map[string]any{"max_entity_version": maxVersion, "current_version": currentVersion}),
		)
		if err := addIssue(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "version mismatch rows")
	}
	return nil
}

func checkTimestampOrder(ctx context.Context, db *sql.DB, prefix string, addIssue func(types.Row) error) error {
	query := strings.Builder{}
	query.WriteString(`
SELECT conv_id, entity_id, created_at_ms, updated_at_ms
FROM timeline_entities
WHERE updated_at_ms < created_at_ms
`)
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("AND conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "timestamp order query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		var entityID string
		var createdAt int64
		var updatedAt int64
		if err := rows.Scan(&convID, &entityID, &createdAt, &updatedAt); err != nil {
			return err
		}
		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("entity_id", entityID),
			types.MRP("issue", "updated_before_created"),
			types.MRP("details", map[string]any{"created_at_ms": createdAt, "updated_at_ms": updatedAt}),
		)
		if err := addIssue(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "timestamp order rows")
	}
	return nil
}

func checkEmptyKinds(ctx context.Context, db *sql.DB, prefix string, addIssue func(types.Row) error) error {
	query := strings.Builder{}
	query.WriteString(`
SELECT conv_id, entity_id
FROM timeline_entities
WHERE kind = ''
`)
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("AND conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "empty kind query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		var entityID string
		if err := rows.Scan(&convID, &entityID); err != nil {
			return err
		}
		row := types.NewRow(
			types.MRP("conv_id", convID),
			types.MRP("entity_id", entityID),
			types.MRP("issue", "empty_kind"),
		)
		if err := addIssue(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "empty kind rows")
	}
	return nil
}

func checkJSON(ctx context.Context, db *sql.DB, prefix string, addIssue func(types.Row) error) error {
	query := strings.Builder{}
	query.WriteString(`
SELECT conv_id, entity_id, entity_json
FROM timeline_entities
`)
	args := []any{}
	if strings.TrimSpace(prefix) != "" {
		query.WriteString("WHERE conv_id LIKE ?\n")
		args = append(args, prefix+"%")
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return errors.Wrap(err, "json check query failed")
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		var entityID string
		var raw string
		if err := rows.Scan(&convID, &entityID, &raw); err != nil {
			return err
		}
		var entity timelinepb.TimelineEntityV1
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(raw), &entity); err != nil {
			row := types.NewRow(
				types.MRP("conv_id", convID),
				types.MRP("entity_id", entityID),
				types.MRP("issue", "json_parse_error"),
				types.MRP("details", map[string]any{"error": err.Error()}),
			)
			if err := addIssue(row); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "json check rows")
	}
	return nil
}

var _ cmds.GlazeCommand = &TimelineVerifyCommand{}
