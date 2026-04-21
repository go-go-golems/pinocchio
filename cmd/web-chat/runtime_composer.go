package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gepmiddleware "github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type ProfileRuntimeComposer struct {
	definitions middlewarecfg.DefinitionRegistry
	buildDeps   middlewarecfg.BuildDeps
	base        *settings.InferenceSettings
}

func newProfileRuntimeComposer(
	definitions middlewarecfg.DefinitionRegistry,
	buildDeps middlewarecfg.BuildDeps,
	base *settings.InferenceSettings,
) *ProfileRuntimeComposer {
	return &ProfileRuntimeComposer{
		definitions: definitions,
		buildDeps:   buildDeps,
		base:        base,
	}
}

func (c *ProfileRuntimeComposer) Compose(ctx context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
	if c == nil {
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

	var effectiveInferenceSettings *settings.InferenceSettings
	if req.ResolvedInferenceSettings != nil {
		effectiveInferenceSettings = req.ResolvedInferenceSettings.Clone()
	} else if c.base != nil {
		effectiveInferenceSettings = c.base.Clone()
	} else {
		effectiveInferenceSettings, err = settings.NewInferenceSettings()
		if err != nil {
			return infruntime.ComposedRuntime{}, err
		}
	}

	eng, err := infruntime.BuildEngineFromSettingsWithMiddlewares(
		ctx,
		effectiveInferenceSettings,
		systemPrompt,
		resolvedMiddlewares,
	)
	if err != nil {
		return infruntime.ComposedRuntime{}, err
	}
	runtimeFingerprint := strings.TrimSpace(req.ResolvedProfileFingerprint)
	if runtimeFingerprint == "" {
		runtimeFingerprint = infruntime.BuildRuntimeFingerprintFromSettings(runtimeKey, req.ProfileVersion, &infruntime.ProfileRuntime{
			SystemPrompt: systemPrompt,
			Middlewares:  resolvedUses,
			Tools:        tools,
		}, effectiveInferenceSettings)
	}

	return infruntime.ComposedRuntime{
		Engine:             eng,
		WrapSink:           runtimeSinkWrapperFromProfile(req.ResolvedProfileRuntime),
		RuntimeKey:         runtimeKey,
		RuntimeFingerprint: runtimeFingerprint,
		SeedSystemPrompt:   systemPrompt,
	}, nil
}

type middlewareResolveInput struct {
	Use           infruntime.MiddlewareUse
	ProfileConfig map[string]any
}

func (c *ProfileRuntimeComposer) resolveMiddlewares(
	ctx context.Context,
	inputs []middlewareResolveInput,
) ([]gepmiddleware.Middleware, []infruntime.MiddlewareUse, error) {
	if len(inputs) == 0 {
		return nil, nil, nil
	}
	if c == nil || c.definitions == nil {
		return nil, nil, fmt.Errorf("middleware definitions are not configured")
	}

	resolved := make([]middlewarecfg.ResolvedInstance, 0, len(inputs))
	resolvedUses := make([]infruntime.MiddlewareUse, 0, len(inputs))
	for i, input := range inputs {
		use := middlewarecfg.Use{
			Name:    input.Use.Name,
			ID:      input.Use.ID,
			Enabled: cloneBoolPtr(input.Use.Enabled),
		}
		instanceKey := middlewarecfg.MiddlewareInstanceKey(use, i)
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
		resolvedCfg, err := resolver.Resolve(def, middlewarecfg.Use{
			Name:    input.Use.Name,
			ID:      input.Use.ID,
			Enabled: cloneBoolPtr(input.Use.Enabled),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("resolve middleware %s: %w", instanceKey, err)
		}

		resolved = append(resolved, middlewarecfg.ResolvedInstance{
			Key: instanceKey,
			Use: middlewarecfg.Use{
				Name:    input.Use.Name,
				ID:      input.Use.ID,
				Enabled: cloneBoolPtr(input.Use.Enabled),
			},
			Resolved: resolvedCfg,
			Def:      def,
		})

		useForFingerprint := infruntime.MiddlewareUse{
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

func (s fixedPayloadSource) Payload(middlewarecfg.Definition, middlewarecfg.Use) (map[string]any, bool, error) {
	if len(s.payload) == 0 {
		return nil, false, nil
	}
	return copyStringAnyMap(s.payload), true, nil
}

func runtimeMiddlewaresFromProfile(spec *infruntime.ProfileRuntime) ([]infruntime.MiddlewareUse, error) {
	if spec == nil || len(spec.Middlewares) == 0 {
		return nil, nil
	}

	middlewares := make([]infruntime.MiddlewareUse, 0, len(spec.Middlewares))
	for i, mw := range spec.Middlewares {
		name := strings.TrimSpace(mw.Name)
		if name == "" {
			continue
		}
		config, err := normalizeConfigObject(mw.Config, fmt.Sprintf("profile middleware %s config", middlewarecfg.MiddlewareInstanceKey(middlewarecfg.Use{Name: mw.Name, ID: mw.ID, Enabled: cloneBoolPtr(mw.Enabled)}, i)))
		if err != nil {
			return nil, err
		}
		middlewares = append(middlewares, infruntime.MiddlewareUse{
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
func runtimeMiddlewareInputsFromProfile(profileMiddlewares []infruntime.MiddlewareUse) ([]middlewareResolveInput, error) {
	inputs := make([]middlewareResolveInput, 0, len(profileMiddlewares))

	for i, use := range profileMiddlewares {
		name := strings.TrimSpace(use.Name)
		if name == "" {
			continue
		}
		profileConfig, err := normalizeConfigObject(use.Config, fmt.Sprintf("profile middleware %s config", middlewarecfg.MiddlewareInstanceKey(middlewarecfg.Use{Name: use.Name, ID: use.ID, Enabled: cloneBoolPtr(use.Enabled)}, i)))
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, middlewareResolveInput{
			Use: infruntime.MiddlewareUse{
				Name:    name,
				ID:      strings.TrimSpace(use.ID),
				Enabled: cloneBoolPtr(use.Enabled),
			},
			ProfileConfig: profileConfig,
		})
	}
	return inputs, nil
}

func runtimeToolsFromProfile(spec *infruntime.ProfileRuntime) []string {
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
