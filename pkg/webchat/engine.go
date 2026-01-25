package webchat

import (
	"context"

	"github.com/pkg/errors"

	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/pkg/middlewares/planning"
)

// composeEngineFromSettings builds an engine from step settings then applies middlewares.
func composeEngineFromSettings(stepSettings *settings.StepSettings, sysPrompt string, uses []MiddlewareUse, mwFactories map[string]MiddlewareFactory) (engine.Engine, error) {
	eng, err := factory.NewEngineFromStepSettings(stepSettings)
	if err != nil {
		return nil, errors.Wrap(err, "engine init failed")
	}

	mws := make([]middleware.Middleware, 0, 2+len(uses))
	planningEnabled := false
	planningCfg := planning.DefaultConfig()

	// Always append tool result reorder for UX (outermost).
	mws = append(mws, middleware.NewToolResultReorderMiddleware())

	// Apply requested middlewares (first listed becomes outermost among requested).
	for _, u := range uses {
		if u.Name == "planning" {
			planningEnabled = true
			cfg, err := planningConfigFromAny(u.Config)
			if err != nil {
				return nil, err
			}
			planningCfg = cfg
			continue
		}
		f, ok := mwFactories[u.Name]
		if !ok {
			return nil, errors.Errorf("unknown middleware: %s", u.Name)
		}
		mws = append(mws, f(u.Config))
	}

	// System prompt is near-innermost; planning directive (if enabled) should run closest
	// to provider inference so it can append its directive after the base system prompt.
	if sysPrompt != "" {
		mws = append(mws, middleware.NewSystemPromptMiddleware(sysPrompt))
	}
	if planningEnabled {
		mws = append(mws, planning.NewDirectiveMiddleware())
	}

	builder := &enginebuilder.Builder{Base: eng, Middlewares: mws}
	runner, err := builder.Build(context.Background(), "")
	if err != nil {
		return nil, err
	}

	if planningEnabled {
		provider, model := providerAndModelLabels(stepSettings)
		return planning.NewLifecycleEngine(runner, planningCfg, provider, model), nil
	}
	return runner, nil
}

func planningConfigFromAny(raw any) (planning.Config, error) {
	cfg := planning.DefaultConfig()
	switch v := raw.(type) {
	case nil:
		return cfg, nil
	case bool:
		cfg.Enabled = v
		return cfg.Sanitized(), nil
	case planning.Config:
		return v.Sanitized(), nil
	case map[string]any:
		if b, ok := v["enabled"].(bool); ok {
			cfg.Enabled = b
		}
		if i, ok := v["max_iterations"].(int); ok {
			cfg.MaxIterations = i
		} else if f, ok := v["max_iterations"].(float64); ok {
			cfg.MaxIterations = int(f)
		}
		if s, ok := v["prompt"].(string); ok {
			cfg.Prompt = s
		}
		return cfg.Sanitized(), nil
	default:
		return planning.Config{}, errors.Errorf("planning config must be bool, object, or planning.Config (got %T)", raw)
	}
}

func providerAndModelLabels(stepSettings *settings.StepSettings) (string, string) {
	if stepSettings == nil || stepSettings.Chat == nil {
		return "", ""
	}
	provider := ""
	if stepSettings.Chat.ApiType != nil {
		provider = string(*stepSettings.Chat.ApiType)
	}
	model := ""
	if stepSettings.Chat.Engine != nil {
		model = *stepSettings.Chat.Engine
	}
	return provider, model
}
