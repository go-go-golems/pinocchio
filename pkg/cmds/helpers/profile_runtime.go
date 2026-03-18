package helpers

import (
	"context"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	appconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/pkg/errors"
)

const ProfileSettingsSectionSlug = "profile-settings"

type ProfileSettings struct {
	Profile           string `glazed:"profile"`
	ProfileRegistries string `glazed:"profile-registries"`
}

func NewProfileSettingsSection() (schema.Section, error) {
	return schema.NewSection(
		ProfileSettingsSectionSlug,
		"Profile settings",
		schema.WithFields(
			fields.New(
				"profile",
				fields.TypeString,
				fields.WithHelp("Load the profile"),
			),
			fields.New(
				"profile-registries",
				fields.TypeString,
				fields.WithHelp("Comma-separated profile registry sources (yaml/sqlite/sqlite-dsn)"),
			),
		),
	)
}

func ResolveBaseStepSettings(parsed *values.Values) (*aisettings.StepSettings, []string, error) {
	sections_, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, nil, errors.Wrap(err, "create hidden geppetto sections")
	}
	schema_ := schema.NewSchema(schema.WithSections(sections_...))
	parsedValues := values.New()
	configFiles, err := resolveConfigFiles(parsed)
	if err != nil {
		return nil, nil, err
	}
	if err := sources.Execute(
		schema_,
		parsedValues,
		sources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		sources.FromFiles(
			configFiles,
			sources.WithConfigFileMapper(configFileMapper),
			sources.WithParseOptions(fields.WithSource("config")),
		),
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	); err != nil {
		return nil, configFiles, errors.Wrap(err, "resolve hidden pinocchio base step settings")
	}
	stepSettings, err := aisettings.NewStepSettingsFromParsedValues(parsedValues)
	if err != nil {
		return nil, configFiles, errors.Wrap(err, "build step settings from hidden parsed values")
	}
	return stepSettings, configFiles, nil
}

func ResolveProfileSettings(parsed *values.Values) ProfileSettings {
	ret := ProfileSettings{}
	if parsed != nil {
		_ = parsed.DecodeSectionInto(ProfileSettingsSectionSlug, &ret)
	}
	ret.Profile = strings.TrimSpace(ret.Profile)
	ret.ProfileRegistries = strings.TrimSpace(ret.ProfileRegistries)
	if ret.ProfileRegistries == "" {
		ret.ProfileRegistries = defaultPinocchioProfileRegistriesIfPresent()
	}
	return ret
}

func ResolveEffectiveProfileSettings(parsed *values.Values) (ProfileSettings, []string, error) {
	profileSection, err := NewProfileSettingsSection()
	if err != nil {
		return ProfileSettings{}, nil, errors.Wrap(err, "create profile settings section")
	}
	schema_ := schema.NewSchema(schema.WithSections(profileSection))
	resolvedValues := values.New()
	configFiles, err := resolveConfigFiles(parsed)
	if err != nil {
		return ProfileSettings{}, nil, err
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
		return ProfileSettings{}, configFiles, errors.Wrap(err, "resolve profile settings from config/env/defaults")
	}
	if parsed != nil {
		if err := resolvedValues.Merge(parsed); err != nil {
			return ProfileSettings{}, configFiles, errors.Wrap(err, "merge explicit profile settings")
		}
	}
	return ResolveProfileSettings(resolvedValues), configFiles, nil
}

func ResolveStepSettings(
	ctx context.Context,
	parsed *values.Values,
) (*aisettings.StepSettings, *gepprofiles.ResolvedProfile, func(), error) {
	base, _, err := ResolveBaseStepSettings(parsed)
	if err != nil {
		return nil, nil, nil, err
	}
	profileSettings, _, err := ResolveEffectiveProfileSettings(parsed)
	if err != nil {
		return nil, nil, nil, err
	}
	if profileSettings.ProfileRegistries == "" {
		return base, nil, nil, nil
	}

	specEntries, err := gepprofiles.ParseProfileRegistrySourceEntries(profileSettings.ProfileRegistries)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "parse profile registry sources")
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(specEntries)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "parse profile registry source specs")
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "initialize profile registry")
	}

	in := gepprofiles.ResolveInput{}
	if profileSettings.Profile != "" {
		profileSlug, err := gepprofiles.ParseProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, nil, nil, err
		}
		in.ProfileSlug = profileSlug
	}
	resolved, err := chain.ResolveEffectiveProfile(ctx, in)
	if err != nil {
		_ = chain.Close()
		return nil, nil, nil, err
	}
	return base.Clone(), resolved, func() {
		_ = chain.Close()
	}, nil
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
