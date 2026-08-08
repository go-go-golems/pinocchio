package profiles

import (
	"context"
	"fmt"
	"sort"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aistepssettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
	"github.com/pkg/errors"
)

// DebugCommand reports the source, stack, and final settings selected for a
// Pinocchio engine profile. It intentionally reports credential presence only;
// it never emits credential values.
type DebugCommand struct {
	*cmds.CommandDescription
}

// DebugSettings controls the safe settings detail included by profiles debug.
type DebugSettings struct {
	IncludeSettings bool `glazed:"include-settings"`
}

var _ cmds.GlazeCommand = (*DebugCommand)(nil)

// NewDebugCommand creates the profile-resolution diagnostic command.
func NewDebugCommand() (*DebugCommand, error) {
	commandSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	geppettoSections, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, err
	}

	return &DebugCommand{
		CommandDescription: cmds.NewCommandDescription(
			"debug",
			cmds.WithShort("Debug profile sources, stacking, and effective credential presence"),
			cmds.WithLong(`Resolve the selected Pinocchio profile exactly as a runtime does.

The command reports registry sources, profile-stack order, and the base, stack-merged,
and final effective settings. Credential values are never printed: each API-key field is
reported only as present or empty.

Examples:
  pinocchio profiles debug --profile researcher
  pinocchio profiles debug --profile-registries ~/.config/pinocchio/profiles.yaml,./profiles.yaml --profile ttc-live-luna-codex
  pinocchio profiles debug --profile assistant --include-settings --format json
`),
			cmds.WithFlags(
				fields.New(
					"include-settings",
					fields.TypeBool,
					fields.WithDefault(false),
					fields.WithHelp("Include redacted settings JSON for each resolution stage"),
				),
			),
			cmds.WithSections(append([]schema.Section{commandSettingsSection}, geppettoSections...)...),
		),
	}, nil
}

func (c *DebugCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *values.Values,
	gp middlewares.Processor,
) error {
	settings := &DebugSettings{}
	if parsedLayers != nil {
		if err := parsedLayers.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
			return errors.Wrap(err, "decode profiles debug settings")
		}
	}

	resolved, err := profilebootstrap.ResolveCLIEngineSettings(ctx, parsedLayers)
	if err != nil {
		return errors.Wrap(err, "resolve profile engine settings")
	}
	if resolved != nil && resolved.Close != nil {
		defer resolved.Close()
	}
	if resolved == nil || resolved.ProfileRuntime == nil || resolved.ProfileRuntime.ProfileRegistryChain == nil || resolved.ProfileRuntime.ProfileRegistryChain.Registry == nil {
		return fmt.Errorf("no profile registry configured")
	}

	for _, source := range debugSourceRows(resolved.ProfileRuntime) {
		if err := gp.AddRow(ctx, source); err != nil {
			return err
		}
	}

	profile := resolved.ResolvedEngineProfile
	if profile == nil {
		return fmt.Errorf("selected profile did not resolve")
	}
	registry := resolved.ProfileRuntime.ProfileRegistryChain.Registry
	for _, lineage := range profile.StackLineage {
		layer, err := registry.GetEngineProfile(ctx, lineage.RegistrySlug, lineage.EngineProfileSlug)
		if err != nil {
			return errors.Wrapf(err, "load profile-stack layer %s/%s", lineage.RegistrySlug, lineage.EngineProfileSlug)
		}
		row := debugProfileLayerRow(lineage, layer.InferenceSettings, settings.IncludeSettings)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	if err := gp.AddRow(ctx, debugSettingsRow("profile-stack-effective", profile.RegistrySlug.String(), profile.EngineProfileSlug.String(), profile.InferenceSettings, settings.IncludeSettings)); err != nil {
		return err
	}
	if err := gp.AddRow(ctx, debugSettingsRow("base", "", "", resolved.BaseInferenceSettings, settings.IncludeSettings)); err != nil {
		return err
	}
	return gp.AddRow(ctx, debugSettingsRow("final", profile.RegistrySlug.String(), profile.EngineProfileSlug.String(), resolved.FinalInferenceSettings, settings.IncludeSettings))
}

func debugSourceRows(runtime *profilebootstrap.ResolvedCLIProfileRuntime) []types.Row {
	entries := runtime.ProfileSettings.ProfileRegistries
	specs, err := gepprofiles.ParseRegistrySourceSpecs(entries)
	if err != nil {
		return []types.Row{types.NewRow(
			types.MRP("stage", "source-error"),
			types.MRP("source_error", err.Error()),
		)}
	}
	rows := make([]types.Row, 0, len(specs))
	for _, spec := range specs {
		row := types.NewRow(
			types.MRP("stage", "source"),
			types.MRP("source_kind", string(spec.Kind)),
		)
		if spec.Kind == gepprofiles.RegistrySourceKindSQLiteDSN {
			row.Set("source", "sqlite-dsn:***REDACTED***")
		} else {
			row.Set("source", spec.Raw)
		}
		if spec.Path != "" {
			row.Set("source_path", spec.Path)
		}
		rows = append(rows, row)
	}
	if runtime.Documents != nil {
		for _, file := range runtime.Documents.Files {
			rows = append(rows, types.NewRow(
				types.MRP("stage", "source"),
				types.MRP("source_kind", "config-document"),
				types.MRP("source", file.Path),
				types.MRP("source_path", file.Path),
				types.MRP("source_layer", string(file.Layer)),
				types.MRP("source_name", file.SourceName),
			))
		}
	}
	return rows
}

func debugProfileLayerRow(lineage gepprofiles.ResolvedProfileStackEntry, settings any, includeSettings bool) types.Row {
	row := debugSettingsRow("profile-layer", lineage.RegistrySlug.String(), lineage.EngineProfileSlug.String(), settings, includeSettings)
	if lineage.Source != "" {
		row.Set("source", lineage.Source)
	}
	if lineage.Version != 0 {
		row.Set("version", lineage.Version)
	}
	return row
}

func debugSettingsRow(stage, registry, profile string, settings any, includeSettings bool) types.Row {
	row := types.NewRow(types.MRP("stage", stage))
	if registry != "" {
		row.Set("registry", registry)
	}
	if profile != "" {
		row.Set("profile", profile)
	}

	summary, ok := settingsSummaryForDebug(settings)
	if !ok {
		row.Set("api_key_status", []string{"none"})
		return row
	}
	row.Set("api_key_status", apiKeyStatus(summary.APIKeys))
	row.Set("chat_engine", summary.ChatEngine)
	row.Set("chat_api_type", summary.ChatAPIType)
	row.Set("embedding_type", summary.EmbeddingType)
	row.Set("embedding_engine", summary.EmbeddingEngine)
	row.Set("embedding_dimensions", summary.EmbeddingDimensions)
	if includeSettings {
		row.Set("settings_json", summary.SettingsJSON)
	}
	return row
}

type debugSettingsSummary struct {
	APIKeys             map[string]string
	ChatEngine          string
	ChatAPIType         string
	EmbeddingType       string
	EmbeddingEngine     string
	EmbeddingDimensions int
	SettingsJSON        string
}

func settingsSummaryForDebug(raw any) (debugSettingsSummary, bool) {
	settings, ok := raw.(*aistepssettings.InferenceSettings)
	if !ok || settings == nil {
		return debugSettingsSummary{}, false
	}

	summary := debugSettingsSummary{APIKeys: map[string]string{}}
	if settings.API != nil {
		for name, value := range settings.API.APIKeys {
			summary.APIKeys[name] = value
		}
	}
	if settings.Chat != nil {
		if settings.Chat.Engine != nil {
			summary.ChatEngine = *settings.Chat.Engine
		}
		if settings.Chat.ApiType != nil {
			summary.ChatAPIType = string(*settings.Chat.ApiType)
		}
	}
	if settings.Embeddings != nil {
		summary.EmbeddingType = settings.Embeddings.Type
		summary.EmbeddingEngine = settings.Embeddings.Engine
		summary.EmbeddingDimensions = settings.Embeddings.Dimensions
	}
	summary.SettingsJSON = summarizeSettings(settings).SettingsJSON
	return summary, true
}

func apiKeyStatus(keys map[string]string) []string {
	if len(keys) == 0 {
		return []string{"none"}
	}
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	statuses := make([]string, 0, len(names))
	for _, name := range names {
		status := "empty"
		if strings.TrimSpace(keys[name]) != "" {
			status = "present"
		}
		statuses = append(statuses, name+"="+status)
	}
	return statuses
}
