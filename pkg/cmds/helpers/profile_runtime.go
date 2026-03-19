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

type ResolvedInferenceSettings struct {
	InferenceSettings     *aisettings.InferenceSettings
	ResolvedEngineProfile *gepprofiles.ResolvedEngineProfile
	ConfigFiles           []string
	Close                 func()
}

func ResolveBaseInferenceSettings(parsed *values.Values) (*aisettings.InferenceSettings, []string, error) {
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
		return nil, configFiles, errors.Wrap(err, "resolve hidden pinocchio base inference settings")
	}
	stepSettings, err := aisettings.NewInferenceSettingsFromParsedValues(parsedValues)
	if err != nil {
		return nil, configFiles, errors.Wrap(err, "build inference settings from hidden parsed values")
	}
	return stepSettings, configFiles, nil
}

func ResolveFinalInferenceSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedInferenceSettings, error) {
	base, configFiles, err := ResolveBaseInferenceSettings(parsed)
	if err != nil {
		return nil, err
	}
	profileSettings, _, err := ResolveEngineProfileSettings(parsed)
	if err != nil {
		return nil, err
	}
	if len(profileSettings.ProfileRegistries) == 0 {
		return &ResolvedInferenceSettings{
			InferenceSettings: base,
			ConfigFiles:       append([]string(nil), configFiles...),
		}, nil
	}

	specs, err := gepprofiles.ParseRegistrySourceSpecs(profileSettings.ProfileRegistries)
	if err != nil {
		return nil, errors.Wrap(err, "parse profile registry source specs")
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, errors.Wrap(err, "initialize profile registry")
	}

	in := gepprofiles.ResolveInput{}
	if profileSettings.Profile != "" {
		profileSlug, err := gepprofiles.ParseEngineProfileSlug(profileSettings.Profile)
		if err != nil {
			_ = chain.Close()
			return nil, err
		}
		in.EngineProfileSlug = profileSlug
	}
	resolved, err := chain.ResolveEngineProfile(ctx, in)
	if err != nil {
		_ = chain.Close()
		return nil, err
	}
	finalSettings, err := gepprofiles.MergeInferenceSettings(base, resolved.InferenceSettings)
	if err != nil {
		_ = chain.Close()
		return nil, errors.Wrap(err, "merge base inference settings with engine profile")
	}
	return &ResolvedInferenceSettings{
		InferenceSettings:     finalSettings,
		ResolvedEngineProfile: resolved,
		ConfigFiles:           append([]string(nil), configFiles...),
		Close: func() {
			_ = chain.Close()
		},
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
