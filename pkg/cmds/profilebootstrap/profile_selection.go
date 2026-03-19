package profilebootstrap

import (
	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

const ProfileSettingsSectionSlug = bootstrap.ProfileSettingsSectionSlug

type ProfileSettings = bootstrap.ProfileSettings
type ResolvedCLIProfileSelection = bootstrap.ResolvedCLIProfileSelection
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
	}
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
