package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/rs/zerolog/log"
)

type webChatRuntimeComposer struct {
	parsed      *values.Values
	mwFactories map[string]infruntime.MiddlewareFactory
}

func newWebChatRuntimeComposer(parsed *values.Values, mwFactories map[string]infruntime.MiddlewareFactory) *webChatRuntimeComposer {
	return &webChatRuntimeComposer{
		parsed:      parsed,
		mwFactories: mwFactories,
	}
}

func (c *webChatRuntimeComposer) Compose(ctx context.Context, req infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
	if c == nil || c.parsed == nil {
		return infruntime.RuntimeArtifacts{}, fmt.Errorf("runtime composer is not configured")
	}
	if err := validateOverrides(req.Overrides); err != nil {
		return infruntime.RuntimeArtifacts{}, err
	}
	if ctx == nil {
		return infruntime.RuntimeArtifacts{}, fmt.Errorf("compose context is nil")
	}

	runtimeKey := strings.TrimSpace(req.RuntimeKey)
	if runtimeKey == "" {
		runtimeKey = "default"
	}

	systemPrompt := ""
	if req.ResolvedRuntime != nil {
		systemPrompt = strings.TrimSpace(req.ResolvedRuntime.SystemPrompt)
	}
	middlewares := runtimeMiddlewaresFromProfile(req.ResolvedRuntime)
	tools := runtimeToolsFromProfile(req.ResolvedRuntime)
	if req.Overrides != nil {
		if v, ok := req.Overrides["system_prompt"].(string); ok && strings.TrimSpace(v) != "" {
			systemPrompt = v
		}
		if arr, ok := req.Overrides["middlewares"].([]any); ok {
			parsed, err := parseMiddlewareOverrides(arr)
			if err != nil {
				return infruntime.RuntimeArtifacts{}, err
			}
			middlewares = parsed
		}
		if arr, ok := req.Overrides["tools"].([]any); ok {
			parsed, err := parseToolOverrides(arr)
			if err != nil {
				return infruntime.RuntimeArtifacts{}, err
			}
			tools = parsed
		}
	}
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "You are an assistant"
	}

	stepSettings, err := settings.NewStepSettingsFromParsedValues(c.parsed)
	if err != nil {
		return infruntime.RuntimeArtifacts{}, err
	}
	eng, err := infruntime.ComposeEngineFromSettings(
		ctx,
		stepSettings.Clone(),
		systemPrompt,
		middlewares,
		c.mwFactories,
	)
	if err != nil {
		return infruntime.RuntimeArtifacts{}, err
	}

	return infruntime.RuntimeArtifacts{
		Engine:             eng,
		RuntimeKey:         runtimeKey,
		RuntimeFingerprint: runtimeFingerprint(runtimeKey, req.ProfileVersion, systemPrompt, middlewares, tools, stepSettings),
		SeedSystemPrompt:   systemPrompt,
		AllowedTools:       tools,
	}, nil
}

func runtimeMiddlewaresFromProfile(spec *gepprofiles.RuntimeSpec) []infruntime.MiddlewareUse {
	if spec == nil || len(spec.Middlewares) == 0 {
		return nil
	}
	middlewares := make([]infruntime.MiddlewareUse, 0, len(spec.Middlewares))
	for _, mw := range spec.Middlewares {
		name := strings.TrimSpace(mw.Name)
		if name == "" {
			continue
		}
		middlewares = append(middlewares, infruntime.MiddlewareUse{
			Name:   name,
			Config: mw.Config,
		})
	}
	if len(middlewares) == 0 {
		return nil
	}
	return middlewares
}

func runtimeToolsFromProfile(spec *gepprofiles.RuntimeSpec) []string {
	if spec == nil || len(spec.Tools) == 0 {
		return nil
	}
	tools := make([]string, 0, len(spec.Tools))
	for _, tool := range spec.Tools {
		name := strings.TrimSpace(tool)
		if name == "" {
			continue
		}
		tools = append(tools, name)
	}
	if len(tools) == 0 {
		return nil
	}
	return tools
}

type runtimeFingerprintPayload struct {
	ProfileVersion uint64                     `json:"profile_version,omitempty"`
	RuntimeKey     string                     `json:"runtime_key"`
	SystemPrompt   string                     `json:"system_prompt"`
	Middlewares    []infruntime.MiddlewareUse `json:"middlewares"`
	Tools          []string                   `json:"tools"`
	StepMetadata   map[string]any             `json:"step_metadata,omitempty"`
}

func runtimeFingerprint(
	runtimeKey string,
	profileVersion uint64,
	systemPrompt string,
	middlewares []infruntime.MiddlewareUse,
	tools []string,
	stepSettings *settings.StepSettings,
) string {
	var metadata map[string]any
	if stepSettings != nil {
		metadata = stepSettings.GetMetadata()
	}
	payload := runtimeFingerprintPayload{
		ProfileVersion: profileVersion,
		RuntimeKey:     runtimeKey,
		SystemPrompt:   systemPrompt,
		Middlewares:    middlewares,
		Tools:          tools,
		StepMetadata:   metadata,
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

func parseMiddlewareOverrides(arr []any) ([]infruntime.MiddlewareUse, error) {
	mws := make([]infruntime.MiddlewareUse, 0, len(arr))
	for _, raw := range arr {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("middleware override entries must be objects")
		}
		name, _ := m["name"].(string)
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("middleware override missing name")
		}
		mws = append(mws, infruntime.MiddlewareUse{Name: strings.TrimSpace(name), Config: m["config"]})
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
