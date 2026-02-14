package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	"github.com/rs/zerolog/log"
)

type webChatRuntimeComposer struct {
	parsed      *values.Values
	mwFactories map[string]webchat.MiddlewareFactory
}

func newWebChatRuntimeComposer(parsed *values.Values, mwFactories map[string]webchat.MiddlewareFactory) *webChatRuntimeComposer {
	return &webChatRuntimeComposer{
		parsed:      parsed,
		mwFactories: mwFactories,
	}
}

func (c *webChatRuntimeComposer) Compose(ctx context.Context, req webchat.RuntimeComposeRequest) (webchat.RuntimeArtifacts, error) {
	if c == nil || c.parsed == nil {
		return webchat.RuntimeArtifacts{}, fmt.Errorf("runtime composer is not configured")
	}
	if err := validateOverrides(req.Overrides); err != nil {
		return webchat.RuntimeArtifacts{}, err
	}
	if ctx == nil {
		return webchat.RuntimeArtifacts{}, fmt.Errorf("compose context is nil")
	}

	runtimeKey := strings.TrimSpace(req.RuntimeKey)
	if runtimeKey == "" {
		runtimeKey = "default"
	}

	systemPrompt := "You are an assistant"
	middlewares := []webchat.MiddlewareUse{}
	tools := []string{}
	if req.Overrides != nil {
		if v, ok := req.Overrides["system_prompt"].(string); ok && strings.TrimSpace(v) != "" {
			systemPrompt = v
		}
		if arr, ok := req.Overrides["middlewares"].([]any); ok {
			parsed, err := parseMiddlewareOverrides(arr)
			if err != nil {
				return webchat.RuntimeArtifacts{}, err
			}
			middlewares = parsed
		}
		if arr, ok := req.Overrides["tools"].([]any); ok {
			parsed, err := parseToolOverrides(arr)
			if err != nil {
				return webchat.RuntimeArtifacts{}, err
			}
			tools = parsed
		}
	}
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "You are an assistant"
	}

	stepSettings, err := settings.NewStepSettingsFromParsedValues(c.parsed)
	if err != nil {
		return webchat.RuntimeArtifacts{}, err
	}
	eng, err := webchat.ComposeEngineFromSettings(
		ctx,
		stepSettings.Clone(),
		systemPrompt,
		middlewares,
		c.mwFactories,
	)
	if err != nil {
		return webchat.RuntimeArtifacts{}, err
	}

	return webchat.RuntimeArtifacts{
		Engine:             eng,
		RuntimeKey:         runtimeKey,
		RuntimeFingerprint: runtimeFingerprint(runtimeKey, systemPrompt, middlewares, tools, stepSettings),
		SeedSystemPrompt:   systemPrompt,
		AllowedTools:       tools,
	}, nil
}

type runtimeFingerprintPayload struct {
	RuntimeKey   string                  `json:"runtime_key"`
	SystemPrompt string                  `json:"system_prompt"`
	Middlewares  []webchat.MiddlewareUse `json:"middlewares"`
	Tools        []string                `json:"tools"`
	StepMetadata map[string]any          `json:"step_metadata,omitempty"`
}

func runtimeFingerprint(
	runtimeKey string,
	systemPrompt string,
	middlewares []webchat.MiddlewareUse,
	tools []string,
	stepSettings *settings.StepSettings,
) string {
	var metadata map[string]any
	if stepSettings != nil {
		metadata = stepSettings.GetMetadata()
	}
	payload := runtimeFingerprintPayload{
		RuntimeKey:   runtimeKey,
		SystemPrompt: systemPrompt,
		Middlewares:  middlewares,
		Tools:        tools,
		StepMetadata: metadata,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Warn().Err(err).Msg("runtime fingerprint fallback")
		return runtimeKey
	}
	return string(b)
}

func validateOverrides(overrides map[string]any) error {
	if overrides == nil {
		return nil
	}

	if v, ok := overrides["system_prompt"]; ok {
		if _, ok2 := v.(string); !ok2 {
			return fmt.Errorf("system_prompt override must be a string")
		}
	}
	if v, ok := overrides["middlewares"]; ok {
		if _, ok2 := v.([]any); !ok2 {
			return fmt.Errorf("middlewares override must be an array")
		}
	}
	if v, ok := overrides["tools"]; ok {
		if _, ok2 := v.([]any); !ok2 {
			return fmt.Errorf("tools override must be an array")
		}
	}
	return nil
}

func parseMiddlewareOverrides(arr []any) ([]webchat.MiddlewareUse, error) {
	mws := make([]webchat.MiddlewareUse, 0, len(arr))
	for _, raw := range arr {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("middleware override entries must be objects")
		}
		name, _ := m["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("middleware override missing name")
		}
		mws = append(mws, webchat.MiddlewareUse{Name: strings.TrimSpace(name), Config: m["config"]})
	}
	return mws, nil
}

func parseToolOverrides(arr []any) ([]string, error) {
	tools := make([]string, 0, len(arr))
	for _, raw := range arr {
		switch v := raw.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("tool override contains empty name")
			}
			tools = append(tools, strings.TrimSpace(v))
		default:
			return nil, fmt.Errorf("tool override entries must be strings")
		}
	}
	return tools, nil
}
