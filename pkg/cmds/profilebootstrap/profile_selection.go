package profilebootstrap

import (
	"context"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	glazedconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/pkg/errors"
)

const ProfileSettingsSectionSlug = bootstrap.ProfileSettingsSectionSlug

type ProfileSettings = bootstrap.ProfileSettings
type ResolvedCLIProfileSelection = bootstrap.ResolvedCLIProfileSelection
type ResolvedCLIProfileRuntime = bootstrap.ResolvedCLIProfileRuntime
type CLISelectionInput = bootstrap.CLISelectionInput

func pinocchioBootstrapConfig() bootstrap.AppBootstrapConfig {
	cfg := bootstrap.AppBootstrapConfig{
		AppName:          "pinocchio",
		EnvPrefix:        "PINOCCHIO",
		ConfigFileMapper: configFileMapper,
		NewProfileSection: func() (schema.Section, error) {
			return geppettosections.NewProfileSettingsSection()
		},
		BuildBaseSections: func() ([]schema.Section, error) {
			return geppettosections.CreateGeppettoSections()
		},
	}
	cfg.ConfigPlanBuilder = func(parsed *values.Values) (*glazedconfig.Plan, error) {
		explicit := ""
		if parsed != nil {
			commandSettings := &cli.CommandSettings{}
			if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err == nil {
				explicit = strings.TrimSpace(commandSettings.ConfigFile)
			}
		}
		return glazedconfig.NewPlan(
			glazedconfig.WithLayerOrder(glazedconfig.LayerSystem, glazedconfig.LayerUser, glazedconfig.LayerExplicit),
			glazedconfig.WithDedupePaths(),
		).Add(
			glazedconfig.SystemAppConfig(cfg.AppName).Named("system-app-config").Kind("app-config"),
			glazedconfig.XDGAppConfig(cfg.AppName).Named("xdg-app-config").Kind("app-config"),
			glazedconfig.HomeAppConfig(cfg.AppName).Named("home-app-config").Kind("app-config"),
			glazedconfig.ExplicitFile(explicit).Named("explicit-config").Kind("explicit-file"),
		), nil
	}
	return cfg
}

func BootstrapConfig() bootstrap.AppBootstrapConfig {
	return pinocchioBootstrapConfig()
}

func NewProfileSettingsSection() (schema.Section, error) {
	return bootstrap.NewProfileSettingsSection(pinocchioBootstrapConfig())
}

func ResolveProfileSettings(parsed *values.Values) ProfileSettings {
	return bootstrap.ResolveProfileSettings(parsed)
}

func ResolveCLIProfileSelection(parsed *values.Values) (*ResolvedCLIProfileSelection, error) {
	return bootstrap.ResolveCLIProfileSelection(pinocchioBootstrapConfig(), parsed)
}

func ResolveEngineProfileSettings(parsed *values.Values) (ProfileSettings, []string, error) {
	return bootstrap.ResolveEngineProfileSettings(pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIProfileRuntime(ctx context.Context, parsed *values.Values) (*ResolvedCLIProfileRuntime, error) {
	return bootstrap.ResolveCLIProfileRuntime(ctx, pinocchioBootstrapConfig(), parsed)
}

func NewCLISelectionValues(input CLISelectionInput) (*values.Values, error) {
	return bootstrap.NewCLISelectionValues(pinocchioBootstrapConfig(), input)
}

func ResolveCLIConfigFiles(parsed *values.Values) ([]string, error) {
	return bootstrap.ResolveCLIConfigFiles(pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIConfigFilesForExplicit(explicit string) ([]string, error) {
	return bootstrap.ResolveCLIConfigFilesForExplicit(pinocchioBootstrapConfig(), explicit)
}

func MapPinocchioConfigFile(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	return configFileMapper(rawConfig)
}

func configFileMapper(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	configMap, ok := rawConfig.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("expected map[string]interface{}, got %T", rawConfig)
	}

	result := make(map[string]map[string]interface{})
	excludedKeys := map[string]bool{
		"repositories": true,
	}
	for key, value := range configMap {
		if excludedKeys[key] {
			continue
		}
		layerParams, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		result[key] = layerParams
	}
	return result, nil
}

var _ sources.ConfigFileMapper = configFileMapper
