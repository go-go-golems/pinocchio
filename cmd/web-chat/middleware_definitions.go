package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gepmiddleware "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
)

const (
	dependencyAgentModeServiceKey = "agentmode.service"
)

type middlewareDefinition struct {
	name        string
	version     uint16
	displayName string
	description string
	schema      map[string]any
	build       func(context.Context, middlewarecfg.BuildDeps, any) (gepmiddleware.Middleware, error)
}

func (d middlewareDefinition) Name() string {
	return d.name
}

func (d middlewareDefinition) MiddlewareVersion() uint16 {
	if d.version == 0 {
		return 1
	}
	return d.version
}

func (d middlewareDefinition) MiddlewareDisplayName() string {
	return strings.TrimSpace(d.displayName)
}

func (d middlewareDefinition) MiddlewareDescription() string {
	return strings.TrimSpace(d.description)
}

func (d middlewareDefinition) ConfigJSONSchema() map[string]any {
	return cloneStringAnyMap(d.schema)
}

func (d middlewareDefinition) Build(
	ctx context.Context,
	deps middlewarecfg.BuildDeps,
	cfg any,
) (gepmiddleware.Middleware, error) {
	if d.build == nil {
		return nil, fmt.Errorf("middleware %q has no build function", strings.TrimSpace(d.name))
	}
	return d.build(ctx, deps, cfg)
}

func newWebChatMiddlewareDefinitionRegistry() (*middlewarecfg.InMemoryDefinitionRegistry, error) {
	registry := middlewarecfg.NewInMemoryDefinitionRegistry()
	definitions := []middlewarecfg.Definition{
		newAgentModeMiddlewareDefinition(),
	}
	for _, def := range definitions {
		if err := registry.RegisterDefinition(def); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func newAgentModeMiddlewareDefinition() middlewarecfg.Definition {
	schema := map[string]any{
		"title":       "Agent Mode Middleware",
		"description": "Parses and applies agent-mode switches from model output.",
		"type":        "object",
		"properties": map[string]any{
			"default_mode": map[string]any{
				"type":    "string",
				"default": agentmode.DefaultConfig().DefaultMode,
			},
			"sanitize_yaml": map[string]any{
				"type":    "boolean",
				"default": agentmode.DefaultConfig().ParseOptions.SanitizeEnabled(),
			},
		},
		"additionalProperties": false,
	}

	type configInput struct {
		DefaultMode  string `json:"default_mode,omitempty"`
		SanitizeYAML *bool  `json:"sanitize_yaml,omitempty"`
	}

	return middlewareDefinition{
		name:        "agentmode",
		version:     1,
		displayName: "Agent Mode",
		description: "Parses and applies agent-mode switches from model output.",
		schema:      schema,
		build: func(_ context.Context, deps middlewarecfg.BuildDeps, cfg any) (gepmiddleware.Middleware, error) {
			svcRaw, ok := deps.Get(dependencyAgentModeServiceKey)
			if !ok || svcRaw == nil {
				return nil, fmt.Errorf("missing dependency %q", dependencyAgentModeServiceKey)
			}
			svc, ok := svcRaw.(agentmode.Service)
			if !ok {
				return nil, fmt.Errorf("dependency %q has unexpected type %T", dependencyAgentModeServiceKey, svcRaw)
			}

			input := configInput{DefaultMode: agentmode.DefaultConfig().DefaultMode}
			if err := decodeResolvedMiddlewareConfig(cfg, &input); err != nil {
				return nil, err
			}

			config := agentmode.DefaultConfig()
			if strings.TrimSpace(input.DefaultMode) != "" {
				config.DefaultMode = strings.TrimSpace(input.DefaultMode)
			}
			if input.SanitizeYAML != nil {
				config.ParseOptions = config.ParseOptions.WithSanitizeYAML(*input.SanitizeYAML)
			}
			return agentmode.NewMiddleware(svc, config), nil
		},
	}
}

func decodeResolvedMiddlewareConfig(cfg any, out any) error {
	if cfg == nil || out == nil {
		return nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize resolved middleware config: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode resolved middleware config: %w", err)
	}
	return nil
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneStringAnyMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}
