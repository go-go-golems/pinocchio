package webchat

import (
    "github.com/pkg/errors"

    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

// composeEngineFromSettings builds an engine from step settings then applies middlewares.
func composeEngineFromSettings(stepSettings *settings.StepSettings, sysPrompt string, uses []MiddlewareUse, mwFactories map[string]MiddlewareFactory) (engine.Engine, error) {
    eng, err := factory.NewEngineFromStepSettings(stepSettings)
    if err != nil {
        return nil, errors.Wrap(err, "engine init failed")
    }

    // System prompt first
    if sysPrompt != "" {
        eng = middleware.NewEngineWithMiddleware(eng, middleware.NewSystemPromptMiddleware(sysPrompt))
    }

    // Apply requested middlewares in order
    for _, u := range uses {
        f, ok := mwFactories[u.Name]
        if !ok {
            return nil, errors.Errorf("unknown middleware: %s", u.Name)
        }
        eng = middleware.NewEngineWithMiddleware(eng, f(u.Config))
    }

    // Always append tool result reorder for UX
    eng = middleware.NewEngineWithMiddleware(eng, middleware.NewToolResultReorderMiddleware())

    return eng, nil
}


