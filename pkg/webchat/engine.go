package webchat

import (
	"context"
	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

// composeEngineFromSettings builds an engine from step settings then applies middlewares.
func composeEngineFromSettings(stepSettings *settings.StepSettings, sysPrompt string, uses []MiddlewareUse, mwFactories map[string]MiddlewareFactory) (engine.Engine, error) {
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

	// System prompt is innermost so it runs closest to provider inference.
	if sysPrompt != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(sysPrompt))
	}

	builder := &session.ToolLoopEngineBuilder{Base: eng, Middlewares: mws}
	runner, err := builder.Build(context.Background(), "")
	if err != nil {
		return nil, err
	}
	return runner, nil
}
