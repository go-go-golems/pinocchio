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

// MiddlewareFactory creates a middleware instance from an arbitrary config object.
type MiddlewareFactory func(cfg any) middleware.Middleware

// ToolFactory registers a tool into a registry.
type ToolFactory func(reg geptools.ToolRegistry) error

// MiddlewareUse declares a middleware to attach and its config.
type MiddlewareUse struct {
	Name   string
	Config any
}

// ComposeEngineFromSettings builds an engine from step settings then applies middlewares.
func ComposeEngineFromSettings(
	ctx context.Context,
	stepSettings *settings.StepSettings,
	sysPrompt string,
	uses []MiddlewareUse,
	mwFactories map[string]MiddlewareFactory,
) (engine.Engine, error) {
	if ctx == nil {
		return nil, errors.New("ctx is nil")
	}
	eng, err := factory.NewEngineFromStepSettings(stepSettings)
	if err != nil {
		return nil, errors.Wrap(err, "engine init failed")
	}

	mws := make([]middleware.Middleware, 0, 2+len(uses))

	// Always append tool result reorder for UX (outermost).
	mws = append(mws, middleware.NewToolResultReorderMiddleware())

	// Apply requested middlewares (first listed becomes outermost among requested).
	for _, u := range uses {
		f, ok := mwFactories[u.Name]
		if !ok {
			return nil, errors.Errorf("unknown middleware: %s", u.Name)
		}
		mws = append(mws, f(u.Config))
	}

	// System prompt is near-innermost so it stays close to provider inference.
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
