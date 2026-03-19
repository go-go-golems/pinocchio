package helpers

import (
	"context"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

type ResolvedCLIEngineSettings struct {
	BaseInferenceSettings  *aisettings.InferenceSettings
	FinalInferenceSettings *aisettings.InferenceSettings
	ProfileSelection       *ResolvedCLIProfileSelection
	ResolvedEngineProfile  *gepprofiles.ResolvedEngineProfile
	ConfigFiles            []string
	Close                  func()
}

func ResolveCLIEngineSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedCLIEngineSettings, error) {
	base, baseConfigFiles, err := ResolveBaseInferenceSettings(parsed)
	if err != nil {
		return nil, err
	}
	selection, err := ResolveCLIProfileSelection(parsed)
	if err != nil {
		return nil, err
	}

	configFiles := append([]string(nil), baseConfigFiles...)
	if len(selection.ConfigFiles) > 0 {
		configFiles = append([]string(nil), selection.ConfigFiles...)
	}

	// Precedence is:
	// command defaults -> config files -> environment -> explicit parsed values -> profile overlay.
	if len(selection.ProfileRegistries) == 0 {
		return &ResolvedCLIEngineSettings{
			BaseInferenceSettings:  base,
			FinalInferenceSettings: base,
			ProfileSelection:       selection,
			ConfigFiles:            configFiles,
		}, nil
	}

	specs, err := gepprofiles.ParseRegistrySourceSpecs(selection.ProfileRegistries)
	if err != nil {
		return nil, errors.Wrap(err, "parse profile registry source specs")
	}
	chain, err := gepprofiles.NewChainedRegistryFromSourceSpecs(ctx, specs)
	if err != nil {
		return nil, errors.Wrap(err, "initialize profile registry")
	}

	in := gepprofiles.ResolveInput{}
	if selection.Profile != "" {
		profileSlug, err := gepprofiles.ParseEngineProfileSlug(selection.Profile)
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

	return &ResolvedCLIEngineSettings{
		BaseInferenceSettings:  base,
		FinalInferenceSettings: finalSettings,
		ProfileSelection:       selection,
		ResolvedEngineProfile:  resolved,
		ConfigFiles:            configFiles,
		Close: func() {
			_ = chain.Close()
		},
	}, nil
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
