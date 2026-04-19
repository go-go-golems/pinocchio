package profilebootstrap

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

type ResolvedCLIEngineSettings struct {
	BaseInferenceSettings  *aisettings.InferenceSettings
	FinalInferenceSettings *aisettings.InferenceSettings
	ProfileRuntime         *ResolvedCLIProfileRuntime
	ResolvedEngineProfile  *gepprofiles.ResolvedEngineProfile
	ConfigFiles            []string
	Close                  func()
}

func ResolveBaseInferenceSettings(parsed *values.Values) (*aisettings.InferenceSettings, []string, error) {
	cfg := pinocchioBootstrapConfig()
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	resolved, err := resolveConfigRuntime(parsed)
	if err != nil {
		return nil, nil, err
	}

	hiddenBase, err := resolveHiddenBaseInferenceSettings(cfg)
	if err != nil {
		return nil, nil, err
	}

	configFiles := []string{}
	if resolved.ConfigFiles != nil {
		configFiles = append(configFiles, resolved.ConfigFiles.Paths...)
	}
	return hiddenBase, configFiles, nil
}

func ResolveCLIEngineSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedCLIEngineSettings, error) {
	hiddenBase, baseConfigFiles, err := ResolveBaseInferenceSettings(parsed)
	if err != nil {
		return nil, err
	}
	base := hiddenBase
	if parsed != nil {
		base, err = ResolveParsedBaseInferenceSettingsWithBase(parsed, hiddenBase)
		if err != nil {
			return nil, err
		}
	}
	return ResolveCLIEngineSettingsFromBase(ctx, base, parsed, baseConfigFiles)
}

func ResolveCLIEngineSettingsFromBase(
	ctx context.Context,
	base *aisettings.InferenceSettings,
	parsed *values.Values,
	baseConfigFiles []string,
) (*ResolvedCLIEngineSettings, error) {
	if base == nil {
		return nil, errors.New("base inference settings cannot be nil")
	}

	profileRuntime, err := ResolveCLIProfileRuntime(ctx, parsed)
	if err != nil {
		return nil, err
	}

	configFiles := append([]string(nil), baseConfigFiles...)
	if profileRuntime != nil && profileRuntime.ConfigFiles != nil && len(profileRuntime.ConfigFiles.Paths) > 0 {
		configFiles = append([]string(nil), profileRuntime.ConfigFiles.Paths...)
	}

	registryChain := profileRuntime.ProfileRegistryChain
	if registryChain == nil || registryChain.Registry == nil {
		return &ResolvedCLIEngineSettings{
			BaseInferenceSettings:  base,
			FinalInferenceSettings: base,
			ProfileRuntime:         profileRuntime,
			ConfigFiles:            configFiles,
			Close:                  profileRuntime.Close,
		}, nil
	}

	resolved, err := registryChain.Registry.ResolveEngineProfile(ctx, registryChain.DefaultProfileResolve)
	if err != nil {
		if profileRuntime.Close != nil {
			profileRuntime.Close()
		}
		return nil, err
	}
	finalSettings, err := gepprofiles.MergeInferenceSettings(base, resolved.InferenceSettings)
	if err != nil {
		if profileRuntime.Close != nil {
			profileRuntime.Close()
		}
		return nil, errors.Wrap(err, "merge base inference settings with engine profile")
	}

	return &ResolvedCLIEngineSettings{
		BaseInferenceSettings:  base,
		FinalInferenceSettings: finalSettings,
		ProfileRuntime:         profileRuntime,
		ResolvedEngineProfile:  resolved,
		ConfigFiles:            configFiles,
		Close:                  profileRuntime.Close,
	}, nil
}

func resolveHiddenBaseInferenceSettings(cfg bootstrap.AppBootstrapConfig) (*aisettings.InferenceSettings, error) {
	sections_, err := cfg.BuildBaseSections()
	if err != nil {
		return nil, errors.Wrap(err, "create hidden base sections")
	}
	schema_ := schema.NewSchema(schema.WithSections(sections_...))
	parsedValues := values.New()
	if err := sources.Execute(
		schema_,
		parsedValues,
		sources.FromEnv(cfg.EnvPrefix, fields.WithSource("env")),
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	); err != nil {
		return nil, errors.Wrap(err, "resolve hidden base inference settings")
	}
	stepSettings, err := aisettings.NewInferenceSettingsFromParsedValues(parsedValues)
	if err != nil {
		return nil, errors.Wrap(err, "build inference settings from hidden parsed values")
	}
	return stepSettings, nil
}

func NewEngineFromResolvedCLIEngineSettings(
	resolved *ResolvedCLIEngineSettings,
) (engine.Engine, error) {
	return NewEngineFromResolvedCLIEngineSettingsWithFactory(nil, resolved)
}

func NewEngineFromResolvedCLIEngineSettingsWithFactory(
	engineFactory factory.EngineFactory,
	resolved *ResolvedCLIEngineSettings,
) (engine.Engine, error) {
	if resolved == nil {
		return nil, errors.New("resolved engine settings cannot be nil")
	}
	if resolved.FinalInferenceSettings == nil {
		return nil, errors.New("resolved final inference settings cannot be nil")
	}
	if engineFactory == nil {
		engineFactory = factory.NewStandardEngineFactory()
	}
	return engineFactory.CreateEngine(resolved.FinalInferenceSettings)
}
