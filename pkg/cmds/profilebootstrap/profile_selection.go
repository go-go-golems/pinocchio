package profilebootstrap

import (
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
type ResolvedCLIConfigFiles = bootstrap.ResolvedCLIConfigFiles
type CLISelectionInput = bootstrap.CLISelectionInput

func pinocchioBootstrapConfig() bootstrap.AppBootstrapConfig {
	return bootstrap.AppBootstrapConfig{
		AppName:          "pinocchio",
		EnvPrefix:        "PINOCCHIO",
		ConfigFileMapper: configFileMapper,
		NewProfileSection: func() (schema.Section, error) {
			return geppettosections.NewProfileSettingsSection()
		},
		BuildBaseSections: func() ([]schema.Section, error) {
			return geppettosections.CreateGeppettoSections()
		},
		ConfigPlanBuilder: pinocchioConfigPlanBuilder,
	}
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

func NewCLISelectionValues(input CLISelectionInput) (*values.Values, error) {
	return bootstrap.NewCLISelectionValues(pinocchioBootstrapConfig(), input)
}

func ResolveCLIConfigFiles(parsed *values.Values) ([]string, error) {
	return bootstrap.ResolveCLIConfigFiles(pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIConfigFilesResolved(parsed *values.Values) (*ResolvedCLIConfigFiles, error) {
	return bootstrap.ResolveCLIConfigFilesResolved(pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIConfigFilesForExplicit(explicit string) ([]string, error) {
	return bootstrap.ResolveCLIConfigFilesForExplicit(pinocchioBootstrapConfig(), explicit)
}

func MapPinocchioConfigFile(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	return configFileMapper(rawConfig)
}

func pinocchioConfigPlanBuilder(parsed *values.Values) (*glazedconfig.Plan, error) {
	explicit := ""
	if parsed != nil {
		commandSettings := &cli.CommandSettings{}
		if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err == nil {
			explicit = strings.TrimSpace(commandSettings.ConfigFile)
		}
	}

	return glazedconfig.NewPlan(
		glazedconfig.WithLayerOrder(
			glazedconfig.LayerSystem,
			glazedconfig.LayerUser,
			glazedconfig.LayerRepo,
			glazedconfig.LayerCWD,
			glazedconfig.LayerExplicit,
		),
		glazedconfig.WithDedupePaths(),
	).Add(
		glazedconfig.SystemAppConfig("pinocchio").Named("system-app-config").Kind("app-config"),
		glazedconfig.HomeAppConfig("pinocchio").Named("home-app-config").Kind("app-config"),
		glazedconfig.XDGAppConfig("pinocchio").Named("xdg-app-config").Kind("app-config"),
		glazedconfig.GitRootFile(".pinocchio-profile.yml").Named("git-root-local-profile").Kind("profile-overlay"),
		glazedconfig.WorkingDirFile(".pinocchio-profile.yml").Named("cwd-local-profile").Kind("profile-overlay"),
		glazedconfig.ExplicitFile(explicit).Named("explicit-config-file").Kind("explicit-file"),
	), nil
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
