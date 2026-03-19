package profilebootstrap

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/cli/bootstrap"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

type ResolvedCLIEngineSettings = bootstrap.ResolvedCLIEngineSettings

func ResolveBaseInferenceSettings(parsed *values.Values) (*aisettings.InferenceSettings, []string, error) {
	return bootstrap.ResolveBaseInferenceSettings(pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIEngineSettings(
	ctx context.Context,
	parsed *values.Values,
) (*ResolvedCLIEngineSettings, error) {
	return bootstrap.ResolveCLIEngineSettings(ctx, pinocchioBootstrapConfig(), parsed)
}

func ResolveCLIEngineSettingsFromBase(
	ctx context.Context,
	base *aisettings.InferenceSettings,
	parsed *values.Values,
	baseConfigFiles []string,
) (*ResolvedCLIEngineSettings, error) {
	return bootstrap.ResolveCLIEngineSettingsFromBase(ctx, pinocchioBootstrapConfig(), base, parsed, baseConfigFiles)
}

func NewEngineFromResolvedCLIEngineSettings(
	resolved *ResolvedCLIEngineSettings,
) (engine.Engine, error) {
	return bootstrap.NewEngineFromResolvedCLIEngineSettings(resolved)
}

func NewEngineFromResolvedCLIEngineSettingsWithFactory(
	engineFactory factory.EngineFactory,
	resolved *ResolvedCLIEngineSettings,
) (engine.Engine, error) {
	return bootstrap.NewEngineFromResolvedCLIEngineSettingsWithFactory(engineFactory, resolved)
}
