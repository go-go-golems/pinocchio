package timeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/go-go-golems/pinocchio/pkg/security"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type TimelineSnapshotCommand struct {
	*cmds.CommandDescription
}

type TimelineSnapshotSettings struct {
	TimelineDSN     string `glazed:"timeline-dsn"`
	TimelineDB      string `glazed:"timeline-db"`
	ConvID          string `glazed:"conv-id"`
	SinceVersion    uint64 `glazed:"since-version"`
	Limit           int    `glazed:"limit"`
	BaseURL         string `glazed:"base-url"`
	IncludeEntities bool   `glazed:"include-entities"`
	RawJSON         bool   `glazed:"raw-json"`
}

func NewTimelineSnapshotCommand() (*TimelineSnapshotCommand, error) {
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
			fields.WithHelp("Conversation ID to snapshot"),
		),
		fields.New(
			"since-version",
			fields.TypeInteger,
			fields.WithDefault(0),
			fields.WithHelp("Only include entities after this version"),
		),
		fields.New(
			"limit",
			fields.TypeInteger,
			fields.WithDefault(5000),
			fields.WithHelp("Limit number of entities (0 = default server limit)"),
		),
		fields.New(
			"base-url",
			fields.TypeString,
			fields.WithDefault(""),
			fields.WithHelp("Optional HTTP base URL to call /api/timeline instead of reading SQLite"),
		),
		fields.New(
			"include-entities",
			fields.TypeBool,
			fields.WithDefault(false),
			fields.WithHelp("Include entities array in output"),
		),
		fields.New(
			"raw-json",
			fields.TypeBool,
			fields.WithDefault(false),
			fields.WithHelp("Include raw snapshot JSON in output"),
		),
	)

	desc := cmds.NewCommandDescription(
		"snapshot",
		cmds.WithShort("Fetch a timeline snapshot"),
		cmds.WithLong("Fetch a timeline snapshot from SQLite or a remote /api/timeline endpoint."),
		cmds.WithFlags(flags...),
		cmds.WithSections(glazedLayer, commandSettingsLayer),
	)

	return &TimelineSnapshotCommand{CommandDescription: desc}, nil
}

func (c *TimelineSnapshotCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &TimelineSnapshotSettings{}
	if err := parsedLayers.DecodeSectionInto(values.DefaultSlug, settings); err != nil {
		return err
	}
	convID := strings.TrimSpace(settings.ConvID)
	if convID == "" {
		return errors.New("conv-id is required")
	}

	var snap *timelinepb.TimelineSnapshotV1
	var raw []byte
	var err error
	if settings.BaseURL != "" {
		snap, raw, err = fetchSnapshotHTTP(ctx, settings.BaseURL, convID, settings.SinceVersion, settings.Limit)
	} else {
		snap, raw, err = fetchSnapshotDB(ctx, settings, convID, settings.SinceVersion, settings.Limit)
	}
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("conv_id", snap.GetConvId()),
		types.MRP("version", snap.GetVersion()),
		types.MRP("server_time_ms", snap.GetServerTimeMs()),
		types.MRP("entity_count", len(snap.GetEntities())),
	)

	if settings.IncludeEntities || settings.RawJSON {
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err == nil {
			if settings.IncludeEntities {
				row.Set("entities", payload["entities"])
			}
			if settings.RawJSON {
				row.Set("snapshot_json", payload)
			}
		} else if settings.RawJSON {
			row.Set("snapshot_json", string(raw))
		}
	}

	return gp.AddRow(ctx, row)
}

func fetchSnapshotDB(ctx context.Context, settings *TimelineSnapshotSettings, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, []byte, error) {
	store, err := openTimelineStore(&StoreSettings{TimelineDSN: settings.TimelineDSN, TimelineDB: settings.TimelineDB})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = store.Close() }()

	snap, err := store.GetSnapshot(ctx, convID, sinceVersion, limit)
	if err != nil {
		return nil, nil, err
	}
	raw, err := marshalSnapshot(snap)
	if err != nil {
		return nil, nil, err
	}
	return snap, raw, nil
}

func fetchSnapshotHTTP(ctx context.Context, baseURL, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV1, []byte, error) {
	endpoint, err := timelineEndpoint(baseURL)
	if err != nil {
		return nil, nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid base URL")
	}
	q := u.Query()
	q.Set("conv_id", convID)
	if sinceVersion > 0 {
		q.Set("since_version", fmt.Sprintf("%d", sinceVersion))
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	if err := security.ValidateOutboundURL(u.String(), security.OutboundURLOptions{
		AllowHTTP:          true,
		AllowLocalNetworks: true,
	}); err != nil {
		return nil, nil, errors.Wrap(err, "invalid timeline endpoint URL")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	// #nosec G704 -- URL is validated with ValidateOutboundURL before outbound request.
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, errors.Errorf("timeline HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	var snap timelinepb.TimelineSnapshotV1
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &snap); err != nil {
		return nil, nil, err
	}
	return &snap, body, nil
}

func timelineEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", errors.New("base URL is empty")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(strings.TrimRight(u.Path, "/"), "api", "timeline")
	return u.String(), nil
}

func marshalSnapshot(snap *timelinepb.TimelineSnapshotV1) ([]byte, error) {
	return protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false,
	}.Marshal(snap)
}

var _ cmds.GlazeCommand = &TimelineSnapshotCommand{}
