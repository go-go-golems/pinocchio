package helpers

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

type ResolvedCLIEngineSettings = profilebootstrap.ResolvedCLIEngineSettings

func ResolveBaseInferenceSettings(parsed *values.Values) (*aisettings.InferenceSettings, []string, error) {
	return profilebootstrap.ResolveBaseInferenceSettings(parsed)
}

func ResolveCLIEngineSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedCLIEngineSettings, error) {
	return profilebootstrap.ResolveCLIEngineSettings(ctx, parsed)
}

func ResolveCLIEngineSettingsFromBase(
	ctx context.Context,
	base *aisettings.InferenceSettings,
	parsed *values.Values,
	baseConfigFiles []string,
) (*ResolvedCLIEngineSettings, error) {
	return profilebootstrap.ResolveCLIEngineSettingsFromBase(ctx, base, parsed, baseConfigFiles)
}

func NewEngineFromResolvedCLIEngineSettings(
	resolved *ResolvedCLIEngineSettings,
) (engine.Engine, error) {
	return profilebootstrap.NewEngineFromResolvedCLIEngineSettings(resolved)
}

func NewEngineFromResolvedCLIEngineSettingsWithFactory(
	engineFactory factory.EngineFactory,
	resolved *ResolvedCLIEngineSettings,
) (engine.Engine, error) {
	return profilebootstrap.NewEngineFromResolvedCLIEngineSettingsWithFactory(engineFactory, resolved)
}
