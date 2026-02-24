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

// MiddlewareBuilder creates a middleware instance from an arbitrary config object.
type MiddlewareBuilder func(cfg any) middleware.Middleware

// ToolRegistrar registers a tool into a registry.
type ToolRegistrar func(reg geptools.ToolRegistry) error

// MiddlewareSpec declares a middleware to attach and its config.
type MiddlewareSpec struct {
	Name   string
	Config any
}

// BuildEngineFromSettings builds an engine from step settings then applies middlewares.
func BuildEngineFromSettings(
	ctx context.Context,
	stepSettings *settings.StepSettings,
	sysPrompt string,
	uses []MiddlewareSpec,
	mwFactories map[string]MiddlewareBuilder,
) (engine.Engine, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	resolvedMiddlewares := make([]middleware.Middleware, 0, len(uses))
	for _, u := range uses {
		f, ok := mwFactories[u.Name]
		if !ok {
			return nil, errors.Errorf("unknown middleware: %s", u.Name)
		}
		resolvedMiddlewares = append(resolvedMiddlewares, f(u.Config))
	}
	return BuildEngineFromSettingsWithMiddlewares(ctx, stepSettings, sysPrompt, resolvedMiddlewares)
}

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
