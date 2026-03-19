package profilebootstrap

import (
	"strings"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	appconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/pkg/errors"
)

const ProfileSettingsSectionSlug = geppettosections.ProfileSettingsSectionSlug

type ProfileSettings struct {
	Profile           string   `glazed:"profile"`
	ProfileRegistries []string `glazed:"profile-registries"`
}

type ResolvedCLIProfileSelection struct {
	ProfileSettings
	ConfigFiles []string
}

func NewProfileSettingsSection() (schema.Section, error) {
	return geppettosections.NewProfileSettingsSection()
}

func ResolveProfileSettings(parsed *values.Values) ProfileSettings {
	ret := ProfileSettings{}
	if parsed != nil {
		_ = parsed.DecodeSectionInto(ProfileSettingsSectionSlug, &ret)
	}
	ret.Profile = strings.TrimSpace(ret.Profile)
	ret.ProfileRegistries = normalizeProfileRegistries(ret.ProfileRegistries)
	return ret
}

func ResolveCLIProfileSelection(parsed *values.Values) (*ResolvedCLIProfileSelection, error) {
	profileSection, err := NewProfileSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create profile settings section")
	}

	schema_ := schema.NewSchema(schema.WithSections(profileSection))
	resolvedValues := values.New()
	configFiles, err := resolveConfigFiles(parsed)
	if err != nil {
		return nil, err
	}
	if err := sources.Execute(
		schema_,
		resolvedValues,
		sources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		sources.FromFiles(
			configFiles,
			sources.WithConfigFileMapper(configFileMapper),
			sources.WithParseOptions(fields.WithSource("config")),
		),
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	); err != nil {
		return nil, errors.Wrap(err, "resolve profile settings from config/env/defaults")
	}
	if parsed != nil {
		if err := resolvedValues.Merge(parsed); err != nil {
			return nil, errors.Wrap(err, "merge explicit profile settings")
		}
	}

	profileSettings := ResolveProfileSettings(resolvedValues)
	return &ResolvedCLIProfileSelection{
		ProfileSettings: profileSettings,
		ConfigFiles:     append([]string(nil), configFiles...),
	}, nil
}

func ResolveEngineProfileSettings(parsed *values.Values) (ProfileSettings, []string, error) {
	resolved, err := ResolveCLIProfileSelection(parsed)
	if err != nil {
		return ProfileSettings{}, nil, err
	}
	return resolved.ProfileSettings, resolved.ConfigFiles, nil
}

func ResolveCLIConfigFiles(parsed *values.Values) ([]string, error) {
	return resolveConfigFiles(parsed)
}

func ResolveCLIConfigFilesForExplicit(explicit string) ([]string, error) {
	return resolveConfigFilesForExplicit(explicit)
}

func MapPinocchioConfigFile(rawConfig interface{}) (map[string]map[string]interface{}, error) {
	return configFileMapper(rawConfig)
}

func normalizeProfileRegistries(entries []string) []string {
	ret := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			ret = append(ret, trimmed)
		}
	}
	return ret
}

func resolveConfigFiles(parsed *values.Values) ([]string, error) {
	files := make([]string, 0, 2)
	defaultFile, err := appconfig.ResolveAppConfigPath("pinocchio", "")
	if err != nil {
		return nil, errors.Wrap(err, "resolve pinocchio default config path")
	}
	if defaultFile != "" {
		files = append(files, defaultFile)
	}
	if parsed != nil {
		commandSettings := &cli.CommandSettings{}
		if err := parsed.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err == nil {
			explicit := strings.TrimSpace(commandSettings.ConfigFile)
			if explicit != "" {
				explicitPath, err := appconfig.ResolveAppConfigPath("pinocchio", explicit)
				if err != nil {
					return nil, err
				}
				if explicitPath != "" && (len(files) == 0 || files[len(files)-1] != explicitPath) {
					duplicate := false
					for _, f := range files {
						if f == explicitPath {
							duplicate = true
							break
						}
					}
					if !duplicate {
						files = append(files, explicitPath)
					}
				}
			}
		}
	}
	return files, nil
}

func resolveConfigFilesForExplicit(explicit string) ([]string, error) {
	files, err := resolveConfigFiles(nil)
	if err != nil {
		return nil, err
	}
	explicitPath, err := appconfig.ResolveAppConfigPath("pinocchio", explicit)
	if err != nil {
		return nil, err
	}
	if explicitPath == "" {
		return files, nil
	}
	for _, f := range files {
		if f == explicitPath {
			return files, nil
		}
	}
	return append(files, explicitPath), nil
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
