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

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

type TimelineSnapshotCommand struct {
	*cmds.CommandDescription
}

type TimelineSnapshotSettings struct {
	TimelineDSN     string `glazed.parameter:"timeline-dsn"`
	TimelineDB      string `glazed.parameter:"timeline-db"`
	ConvID          string `glazed.parameter:"conv-id"`
	SinceVersion    uint64 `glazed.parameter:"since-version"`
	Limit           int    `glazed.parameter:"limit"`
	BaseURL         string `glazed.parameter:"base-url"`
	IncludeEntities bool   `glazed.parameter:"include-entities"`
	RawJSON         bool   `glazed.parameter:"raw-json"`
}

func NewTimelineSnapshotCommand() (*TimelineSnapshotCommand, error) {
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
			parameters.WithHelp("Conversation ID to snapshot"),
		),
		parameters.NewParameterDefinition(
			"since-version",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(0),
			parameters.WithHelp("Only include entities after this version"),
		),
		parameters.NewParameterDefinition(
			"limit",
			parameters.ParameterTypeInteger,
			parameters.WithDefault(5000),
			parameters.WithHelp("Limit number of entities (0 = default server limit)"),
		),
		parameters.NewParameterDefinition(
			"base-url",
			parameters.ParameterTypeString,
			parameters.WithDefault(""),
			parameters.WithHelp("Optional HTTP base URL to call /timeline instead of reading SQLite"),
		),
		parameters.NewParameterDefinition(
			"include-entities",
			parameters.ParameterTypeBool,
			parameters.WithDefault(false),
			parameters.WithHelp("Include entities array in output"),
		),
		parameters.NewParameterDefinition(
			"raw-json",
			parameters.ParameterTypeBool,
			parameters.WithDefault(false),
			parameters.WithHelp("Include raw snapshot JSON in output"),
		),
	)

	desc := cmds.NewCommandDescription(
		"snapshot",
		cmds.WithShort("Fetch a timeline snapshot"),
		cmds.WithLong("Fetch a timeline snapshot from SQLite or a remote /timeline endpoint."),
		cmds.WithFlags(flags...),
		cmds.WithLayersList(glazedLayer, commandSettingsLayer),
	)

	return &TimelineSnapshotCommand{CommandDescription: desc}, nil
}

func (c *TimelineSnapshotCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	settings := &TimelineSnapshotSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, settings); err != nil {
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
	resp, err := http.DefaultClient.Do(req)
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
	u.Path = path.Join(strings.TrimRight(u.Path, "/"), "timeline")
	return u.String(), nil
}

func marshalSnapshot(snap *timelinepb.TimelineSnapshotV1) ([]byte, error) {
	return protojson.MarshalOptions{
		EmitUnpopulated: false,
		UseProtoNames:   false,
	}.Marshal(snap)
}

var _ cmds.GlazeCommand = &TimelineSnapshotCommand{}
