package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gepmiddleware "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/rs/zerolog/log"
)

type ProfileRuntimeComposer struct {
	parsed      *values.Values
	definitions middlewarecfg.DefinitionRegistry
	buildDeps   middlewarecfg.BuildDeps
}

func newProfileRuntimeComposer(
	parsed *values.Values,
	definitions middlewarecfg.DefinitionRegistry,
	buildDeps middlewarecfg.BuildDeps,
) *ProfileRuntimeComposer {
	return &ProfileRuntimeComposer{
		parsed:      parsed,
		definitions: definitions,
		buildDeps:   buildDeps,
	}
}

func (c *ProfileRuntimeComposer) Compose(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
	if c == nil || c.parsed == nil {
		return infruntime.ComposedRuntime{}, fmt.Errorf("runtime composer is not configured")
	}
	if ctx == nil {
		return infruntime.ComposedRuntime{}, fmt.Errorf("compose context is nil")
	}

	runtimeKey := strings.TrimSpace(req.ProfileKey)
	if runtimeKey == "" {
		runtimeKey = "default"
	}

	systemPrompt := ""
	if req.ResolvedProfileRuntime != nil {
		systemPrompt = strings.TrimSpace(req.ResolvedProfileRuntime.SystemPrompt)
	}

	profileMiddlewares, err := runtimeMiddlewaresFromProfile(req.ResolvedProfileRuntime)
	if err != nil {
		return infruntime.ComposedRuntime{}, err
	}
	tools := runtimeToolsFromProfile(req.ResolvedProfileRuntime)

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "You are an assistant"
	}

	middlewareInputs, err := runtimeMiddlewareInputsFromProfile(profileMiddlewares)
	if err != nil {
		return infruntime.ComposedRuntime{}, err
	}

	resolvedMiddlewares, resolvedUses, err := c.resolveMiddlewares(ctx, middlewareInputs)
	if err != nil {
		return infruntime.ComposedRuntime{}, err
	}

	stepSettings, err := settings.NewStepSettingsFromParsedValues(c.parsed)
	if err != nil {
		return infruntime.ComposedRuntime{}, err
	}
	effectiveStepSettings := stepSettings.Clone()
	if req.ResolvedProfileRuntime != nil && len(req.ResolvedProfileRuntime.StepSettingsPatch) > 0 {
		effectiveStepSettings, err = gepprofiles.ApplyRuntimeStepSettingsPatch(stepSettings, req.ResolvedProfileRuntime.StepSettingsPatch)
		if err != nil {
			return infruntime.ComposedRuntime{}, fmt.Errorf("apply profile step_settings_patch: %w", err)
		}
	}

	eng, err := infruntime.BuildEngineFromSettingsWithMiddlewares(
		ctx,
		effectiveStepSettings,
		systemPrompt,
		resolvedMiddlewares,
	)
	if err != nil {
		return infruntime.ComposedRuntime{}, err
	}
	runtimeFingerprint := strings.TrimSpace(req.ResolvedProfileFingerprint)
	if runtimeFingerprint == "" {
		runtimeFingerprint = buildRuntimeFingerprint(runtimeKey, req.ProfileVersion, systemPrompt, resolvedUses, tools, effectiveStepSettings)
	}

	return infruntime.ComposedRuntime{
		Engine:             eng,
		RuntimeKey:         runtimeKey,
		RuntimeFingerprint: runtimeFingerprint,
		SeedSystemPrompt:   systemPrompt,
		AllowedTools:       tools,
	}, nil
}

type middlewareResolveInput struct {
	Use           gepprofiles.MiddlewareUse
	ProfileConfig map[string]any
}

func (c *ProfileRuntimeComposer) resolveMiddlewares(
	ctx context.Context,
	inputs []middlewareResolveInput,
) ([]gepmiddleware.Middleware, []gepprofiles.MiddlewareUse, error) {
	if len(inputs) == 0 {
		return nil, nil, nil
	}
	if c == nil || c.definitions == nil {
		return nil, nil, fmt.Errorf("middleware definitions are not configured")
	}

	resolved := make([]middlewarecfg.ResolvedInstance, 0, len(inputs))
	resolvedUses := make([]gepprofiles.MiddlewareUse, 0, len(inputs))
	for i, input := range inputs {
		instanceKey := middlewarecfg.MiddlewareInstanceKey(input.Use, i)
		def, ok := c.definitions.GetDefinition(input.Use.Name)
		if !ok {
			return nil, nil, fmt.Errorf("resolve middleware %s: unknown middleware %q", instanceKey, input.Use.Name)
		}

		sources := make([]middlewarecfg.Source, 0, 1)
		if len(input.ProfileConfig) > 0 {
			sources = append(sources, fixedPayloadSource{
				name:    "profile",
				layer:   middlewarecfg.SourceLayerProfile,
				payload: input.ProfileConfig,
			})
		}

		resolver := middlewarecfg.NewResolver(sources...)
		resolvedCfg, err := resolver.Resolve(def, gepprofiles.MiddlewareUse{
			Name:    input.Use.Name,
			ID:      input.Use.ID,
			Enabled: cloneBoolPtr(input.Use.Enabled),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("resolve middleware %s: %w", instanceKey, err)
		}

		resolved = append(resolved, middlewarecfg.ResolvedInstance{
			Key:      instanceKey,
			Use:      input.Use,
			Resolved: resolvedCfg,
			Def:      def,
		})

		useForFingerprint := gepprofiles.MiddlewareUse{
			Name:    input.Use.Name,
			ID:      input.Use.ID,
			Enabled: cloneBoolPtr(input.Use.Enabled),
			Config:  copyStringAnyMap(resolvedCfg.Config),
		}
		resolvedUses = append(resolvedUses, useForFingerprint)
	}

	chain, err := middlewarecfg.BuildChain(ctx, c.buildDeps, resolved)
	if err != nil {
		return nil, nil, err
	}
	return chain, resolvedUses, nil
}

type fixedPayloadSource struct {
	name    string
	layer   middlewarecfg.SourceLayer
	payload map[string]any
}

func (s fixedPayloadSource) Name() string {
	return s.name
}

func (s fixedPayloadSource) Layer() middlewarecfg.SourceLayer {
	return s.layer
}

func (s fixedPayloadSource) Payload(middlewarecfg.Definition, gepprofiles.MiddlewareUse) (map[string]any, bool, error) {
	if len(s.payload) == 0 {
		return nil, false, nil
	}
	return copyStringAnyMap(s.payload), true, nil
}

func runtimeMiddlewaresFromProfile(spec *gepprofiles.RuntimeSpec) ([]gepprofiles.MiddlewareUse, error) {
	if spec == nil || len(spec.Middlewares) == 0 {
		return nil, nil
	}

	middlewares := make([]gepprofiles.MiddlewareUse, 0, len(spec.Middlewares))
	for i, mw := range spec.Middlewares {
		name := strings.TrimSpace(mw.Name)
		if name == "" {
			continue
		}
		config, err := normalizeConfigObject(mw.Config, fmt.Sprintf("profile middleware %s config", middlewarecfg.MiddlewareInstanceKey(mw, i)))
		if err != nil {
			return nil, err
		}
		middlewares = append(middlewares, gepprofiles.MiddlewareUse{
			Name:    name,
			ID:      strings.TrimSpace(mw.ID),
			Enabled: cloneBoolPtr(mw.Enabled),
			Config:  config,
		})
	}
	if len(middlewares) == 0 {
		return nil, nil
	}
	return middlewares, nil
}
func runtimeMiddlewareInputsFromProfile(profileMiddlewares []gepprofiles.MiddlewareUse) ([]middlewareResolveInput, error) {
	inputs := make([]middlewareResolveInput, 0, len(profileMiddlewares))

	for i, use := range profileMiddlewares {
		name := strings.TrimSpace(use.Name)
		if name == "" {
			continue
		}
		profileConfig, err := normalizeConfigObject(use.Config, fmt.Sprintf("profile middleware %s config", middlewarecfg.MiddlewareInstanceKey(use, i)))
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, middlewareResolveInput{
			Use: gepprofiles.MiddlewareUse{
				Name:    name,
				ID:      strings.TrimSpace(use.ID),
				Enabled: cloneBoolPtr(use.Enabled),
			},
			ProfileConfig: profileConfig,
		})
	}
	return inputs, nil
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

type RuntimeFingerprintInput struct {
	ProfileVersion uint64                      `json:"profile_version,omitempty"`
	RuntimeKey     string                      `json:"runtime_key"`
	SystemPrompt   string                      `json:"system_prompt"`
	Middlewares    []gepprofiles.MiddlewareUse `json:"middlewares"`
	Tools          []string                    `json:"tools"`
	StepMetadata   map[string]any              `json:"step_metadata,omitempty"`
}

func buildRuntimeFingerprint(
	runtimeKey string,
	profileVersion uint64,
	systemPrompt string,
	middlewares []gepprofiles.MiddlewareUse,
	tools []string,
	stepSettings *settings.StepSettings,
) string {
	var metadata map[string]any
	if stepSettings != nil {
		metadata = stepSettings.GetMetadata()
	}
	payload := RuntimeFingerprintInput{
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

func normalizeConfigObject(raw any, context string) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	if object, ok := raw.(map[string]any); ok {
		return copyStringAnyMap(object), nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be JSON-serializable: %w", strings.TrimSpace(context), err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("%s must be an object: %w", strings.TrimSpace(context), err)
	}
	if out == nil {
		return nil, nil
	}
	return out, nil
}

func copyStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
