package webchat

import (
	"context"

	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	gcompat "github.com/go-go-golems/pinocchio/pkg/geppettocompat"
)

// composeEngineFromSettings builds an engine from step settings then applies middlewares.
func composeEngineFromSettings(stepSettings *settings.StepSettings, sysPrompt string, uses []MiddlewareUse, mwFactories map[string]MiddlewareFactory) (engine.Engine, error) {
	eng, err := factory.NewEngineFromStepSettings(stepSettings)
	if err != nil {
		return nil, errors.Wrap(err, "engine init failed")
	}

<<<<<<< HEAD
	mws := make([]middleware.Middleware, 0, 2+len(uses))
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
	// System prompt first
	if sysPrompt != "" {
		eng = middleware.NewEngineWithMiddleware(eng, middleware.NewSystemPromptMiddleware(sysPrompt))
	}
=======
	// System prompt first
	if sysPrompt != "" {
		eng = gcompat.WrapEngineWithMiddlewares(eng, middleware.NewSystemPromptMiddleware(sysPrompt))
	}
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)

	// Always append tool result reorder for UX (outermost).
	mws = append(mws, middleware.NewToolResultReorderMiddleware())

	// Apply requested middlewares (first listed becomes outermost among requested).
	for _, u := range uses {
		f, ok := mwFactories[u.Name]
		if !ok {
			return nil, errors.Errorf("unknown middleware: %s", u.Name)
		}
<<<<<<< HEAD
		mws = append(mws, f(u.Config))
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
		eng = middleware.NewEngineWithMiddleware(eng, f(u.Config))
=======
		eng = gcompat.WrapEngineWithMiddlewares(eng, f(u.Config))
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
	}

<<<<<<< HEAD
	// System prompt is near-innermost so it stays close to provider inference.
	if sysPrompt != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(sysPrompt))
	}
||||||| parent of 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)
	// Always append tool result reorder for UX
	eng = middleware.NewEngineWithMiddleware(eng, middleware.NewToolResultReorderMiddleware())
=======
	// Always append tool result reorder for UX
	eng = gcompat.WrapEngineWithMiddlewares(eng, middleware.NewToolResultReorderMiddleware())
>>>>>>> 9909af2 (refactor(pinocchio): port runtime to toolloop/tools and metadata-based IDs)

	builder := &enginebuilder.Builder{Base: eng, Middlewares: mws}
	runner, err := builder.Build(context.Background(), "")
	if err != nil {
		return nil, err
	}

	return runner, nil
}
