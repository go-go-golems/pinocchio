package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gepmiddleware "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	sqlitetool "github.com/go-go-golems/pinocchio/pkg/middlewares/sqlitetool"
)

const (
	dependencyAgentModeServiceKey = "agentmode.service"
	dependencySQLiteDBKey         = "sqlite.db"
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
		newSQLiteMiddlewareDefinition(),
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
		},
		"additionalProperties": false,
	}

	type configInput struct {
		DefaultMode string `json:"default_mode,omitempty"`
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
			return agentmode.NewMiddleware(svc, config), nil
		},
	}
}

func newSQLiteMiddlewareDefinition() middlewarecfg.Definition {
	schema := map[string]any{
		"title":       "SQLite Tool Middleware",
		"description": "Executes SQL tool calls against configured SQLite connection settings.",
		"type":        "object",
		"properties": map[string]any{
			"dsn": map[string]any{
				"type": "string",
			},
			"max_rows": map[string]any{
				"type":    "integer",
				"minimum": 1,
			},
			"execution_timeout_ms": map[string]any{
				"type":    "integer",
				"minimum": 1,
			},
			"max_output_lines": map[string]any{
				"type":    "integer",
				"minimum": 1,
			},
			"max_output_bytes": map[string]any{
				"type":    "integer",
				"minimum": 1,
			},
		},
		"additionalProperties": false,
	}

	type configInput struct {
		DSN                *string `json:"dsn,omitempty"`
		MaxRows            *int    `json:"max_rows,omitempty"`
		ExecutionTimeoutMs *int64  `json:"execution_timeout_ms,omitempty"`
		MaxOutputLines     *int    `json:"max_output_lines,omitempty"`
		MaxOutputBytes     *int    `json:"max_output_bytes,omitempty"`
	}

	return middlewareDefinition{
		name:        "sqlite",
		version:     1,
		displayName: "SQLite Tool",
		description: "Executes SQL tool calls against configured SQLite connection settings.",
		schema:      schema,
		build: func(_ context.Context, deps middlewarecfg.BuildDeps, cfg any) (gepmiddleware.Middleware, error) {
			config := sqlitetool.DefaultConfig()
			if dbRaw, ok := deps.Get(dependencySQLiteDBKey); ok && dbRaw != nil {
				db, ok := dbRaw.(sqlitetool.DBLike)
				if !ok {
					return nil, fmt.Errorf("dependency %q has unexpected type %T", dependencySQLiteDBKey, dbRaw)
				}
				config.DB = db
			}

			var input configInput
			if err := decodeResolvedMiddlewareConfig(cfg, &input); err != nil {
				return nil, err
			}
			if input.DSN != nil {
				config.DSN = strings.TrimSpace(*input.DSN)
			}
			if input.MaxRows != nil {
				config.MaxRows = *input.MaxRows
			}
			if input.ExecutionTimeoutMs != nil {
				config.ExecutionTimeout = time.Duration(*input.ExecutionTimeoutMs) * time.Millisecond
			}
			if input.MaxOutputLines != nil {
				config.MaxOutputLines = *input.MaxOutputLines
			}
			if input.MaxOutputBytes != nil {
				config.MaxOutputBytes = *input.MaxOutputBytes
			}

			return sqlitetool.NewMiddleware(config), nil
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
