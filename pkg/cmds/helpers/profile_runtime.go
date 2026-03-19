package helpers

import (
	"context"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cli"
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

func ResolveFinalInferenceSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedInferenceSettings, error) {
	resolved, err := ResolveCLIEngineSettings(ctx, parsed)
	if err != nil {
		return nil, err
	}
	return &ResolvedInferenceSettings{
		InferenceSettings:     resolved.FinalInferenceSettings,
		ResolvedEngineProfile: resolved.ResolvedEngineProfile,
		ConfigFiles:           append([]string(nil), resolved.ConfigFiles...),
		Close:                 resolved.Close,
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
