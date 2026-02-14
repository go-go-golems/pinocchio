package turns

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

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

type TurnsBackfillCommand struct {
	*cmds.CommandDescription
}

type TurnsBackfillSettings struct {
	TurnsDSN  string `glazed:"turns-dsn"`
	TurnsDB   string `glazed:"turns-db"`
	ConvID    string `glazed:"conv-id"`
	SessionID string `glazed:"session-id"`
	Limit     int    `glazed:"limit"`
	DryRun    bool   `glazed:"dry-run"`
}

func NewTurnsBackfillCommand() (*TurnsBackfillCommand, error) {
	glazedLayer, err := settings.NewGlazedSection()
	if err != nil {
		return nil, err
	}
	commandSettingsLayer, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	desc := cmds.NewCommandDescription(
		"backfill",
		cmds.WithShort("Backfill normalized turns/blocks tables from legacy snapshots"),
		cmds.WithLong("Parse turn_snapshots.payload YAML rows and write normalized rows into turns, blocks, and turn_block_membership."),
		cmds.WithFlags(
			fields.New(
				"turns-dsn",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("SQLite DSN for turn store (preferred over turns-db)"),
			),
			fields.New(
				"turns-db",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("SQLite DB file path for turn store (DSN derived with WAL/busy_timeout)"),
			),
			fields.New(
				"conv-id",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Optional conversation filter for backfill"),
			),
			fields.New(
				"session-id",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Optional session filter for backfill"),
			),
			fields.New(
				"limit",
				fields.TypeInteger,
				fields.WithDefault(0),
				fields.WithHelp("Optional max number of snapshot rows to process (0 = all)"),
			),
			fields.New(
				"dry-run",
				fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Parse and count rows without writing normalized tables"),
			),
		),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TurnsBackfillCommand{CommandDescription: desc}, nil
}

func (c *TurnsBackfillCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TurnsBackfillSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}

	dsn, err := resolveTurnsDSN(settings)
	if err != nil {
		return err
	}
	store, err := chatstore.NewSQLiteTurnStore(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	res, err := store.BackfillNormalizedFromSnapshots(ctx, chatstore.TurnBackfillOptions{
		ConvID:    strings.TrimSpace(settings.ConvID),
		SessionID: strings.TrimSpace(settings.SessionID),
		Limit:     settings.Limit,
		DryRun:    settings.DryRun,
	})
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("dry_run", settings.DryRun),
		types.MRP("snapshots_scanned", res.SnapshotsScanned),
		types.MRP("snapshots_backfilled", res.SnapshotsBackfilled),
		types.MRP("blocks_scanned", res.BlocksScanned),
		types.MRP("turn_rows_upserted", res.TurnRowsUpserted),
		types.MRP("block_rows_upserted", res.BlockRowsUpserted),
		types.MRP("membership_inserted", res.MembershipInserted),
		types.MRP("parse_errors", res.ParseErrors),
	)
	return gp.AddRow(ctx, row)
}

func resolveTurnsDSN(settings *TurnsBackfillSettings) (string, error) {
	if settings == nil {
		return "", errors.New("turn backfill settings are nil")
	}
	if v := strings.TrimSpace(settings.TurnsDSN); v != "" {
		return v, nil
	}
	if strings.TrimSpace(settings.TurnsDB) == "" {
		return "", errors.New("turn store not configured (set --turns-dsn or --turns-db)")
	}
	return chatstore.SQLiteTurnDSNForFile(settings.TurnsDB)
}

var _ cmds.GlazeCommand = &TurnsBackfillCommand{}
