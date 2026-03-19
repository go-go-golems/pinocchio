package runtime

import (
	"context"

	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

// ToolRegistrar registers a tool into a registry.
type ToolRegistrar func(reg geptools.ToolRegistry) error

// BuildEngineFromSettingsWithMiddlewares builds an engine from step settings and applies middleware chain.
func BuildEngineFromSettingsWithMiddlewares(
	ctx context.Context,
	stepSettings *settings.StepSettings,
	sysPrompt string,
	resolvedMiddlewares []middleware.Middleware,
) (engine.Engine, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	eng, err := factory.NewEngineFromStepSettings(stepSettings)
	if err != nil {
		return nil, errors.Wrap(err, "engine init failed")
	}

	mws := make([]middleware.Middleware, 0, 2+len(resolvedMiddlewares))
	mws = append(mws, middleware.NewToolResultReorderMiddleware())
	mws = append(mws, resolvedMiddlewares...)
	if sysPrompt != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(sysPrompt))
	}
	builder := &enginebuilder.Builder{Base: eng, Middlewares: mws}
	runner, err := builder.Build(ctx, "")
	if err != nil {
		return nil, err
	}

	return runner, nil
}
