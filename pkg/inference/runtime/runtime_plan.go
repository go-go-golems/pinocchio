package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

type ToolMergeMode string

const (
	ToolMergeModeUnion   ToolMergeMode = "union"
	ToolMergeModeReplace ToolMergeMode = "replace"
)

type MergeProfileRuntimeOptions struct {
	ToolMergeMode ToolMergeMode
}

func DefaultMergeProfileRuntimeOptions() MergeProfileRuntimeOptions {
	return MergeProfileRuntimeOptions{
		ToolMergeMode: ToolMergeModeUnion,
	}
}

type ResolveRuntimePlanOptions struct {
	BaseInferenceSettings *aisettings.InferenceSettings
	BaseRuntime           *ProfileRuntime
	RuntimeMergeOptions   MergeProfileRuntimeOptions
}

type ResolvedRuntimePlan struct {
	ResolvedProfile   *gepprofiles.ResolvedEngineProfile
	ProfileVersion    uint64
	InferenceSettings *aisettings.InferenceSettings
	Runtime           *ProfileRuntime
	ProfileMetadata   map[string]any
}

type RuntimeFingerprintInput struct {
	ProfileVersion uint64          `json:"profile_version,omitempty"`
	RuntimeKey     string          `json:"runtime_key"`
	SystemPrompt   string          `json:"system_prompt"`
	Middlewares    []MiddlewareUse `json:"middlewares"`
	Tools          []string        `json:"tools"`
	StepMetadata   map[string]any  `json:"step_metadata,omitempty"`
}

func ResolveRuntimePlan(
	ctx context.Context,
	registry gepprofiles.Registry,
	resolved *gepprofiles.ResolvedEngineProfile,
	opts ResolveRuntimePlanOptions,
) (*ResolvedRuntimePlan, error) {
	plan := &ResolvedRuntimePlan{
		ResolvedProfile:   resolved,
		ProfileVersion:    ProfileVersionFromResolvedMetadata(metadataFromResolvedProfile(resolved)),
		InferenceSettings: cloneInferenceSettings(opts.BaseInferenceSettings),
		Runtime:           cloneProfileRuntime(opts.BaseRuntime),
		ProfileMetadata:   copyMetadataMap(metadataFromResolvedProfile(resolved)),
	}

	if resolved != nil && resolved.InferenceSettings != nil {
		merged, err := gepprofiles.MergeInferenceSettings(opts.BaseInferenceSettings, resolved.InferenceSettings)
		if err != nil {
			return nil, err
		}
		plan.InferenceSettings = merged
	}

	if resolved == nil {
		return plan, nil
	}
	if registry == nil {
		return nil, fmt.Errorf("profile registry is required to resolve runtime overlays")
	}

	lineage := resolved.StackLineage
	if len(lineage) == 0 {
		lineage = []gepprofiles.ResolvedProfileStackEntry{{
			RegistrySlug:      resolved.RegistrySlug,
			EngineProfileSlug: resolved.EngineProfileSlug,
		}}
	}

	mergeOpts := opts.RuntimeMergeOptions
	if mergeOpts.ToolMergeMode == "" {
		mergeOpts = DefaultMergeProfileRuntimeOptions()
	}

	for _, entry := range lineage {
		profile, err := registry.GetEngineProfile(ctx, entry.RegistrySlug, entry.EngineProfileSlug)
		if err != nil {
			return nil, err
		}
		profileRuntime, _, err := ProfileRuntimeFromEngineProfile(profile)
		if err != nil {
			return nil, err
		}
		plan.Runtime = MergeProfileRuntime(plan.Runtime, profileRuntime, mergeOpts)
	}

	return plan, nil
}

func MergeProfileRuntime(base *ProfileRuntime, overlay *ProfileRuntime, opts MergeProfileRuntimeOptions) *ProfileRuntime {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return cloneProfileRuntime(overlay)
	}
	if overlay == nil {
		return cloneProfileRuntime(base)
	}

	if opts.ToolMergeMode == "" {
		opts = DefaultMergeProfileRuntimeOptions()
	}

	merged := base.Clone()
	if prompt := strings.TrimSpace(overlay.SystemPrompt); prompt != "" {
		merged.SystemPrompt = prompt
	}
	merged.Middlewares = mergeRuntimeMiddlewares(merged.Middlewares, overlay.Middlewares)
	merged.Tools = mergeRuntimeTools(merged.Tools, overlay.Tools, opts.ToolMergeMode)
	return merged
}

func BuildRuntimeFingerprint(input RuntimeFingerprintInput) string {
	b, err := json.Marshal(input)
	if err != nil {
		return strings.TrimSpace(input.RuntimeKey)
	}
	return string(b)
}

func BuildRuntimeFingerprintFromSettings(
	runtimeKey string,
	profileVersion uint64,
	runtime *ProfileRuntime,
	inferenceSettings *aisettings.InferenceSettings,
) string {
	var (
		systemPrompt string
		middlewares  []MiddlewareUse
		tools        []string
		stepMetadata map[string]any
	)
	if runtime != nil {
		systemPrompt = strings.TrimSpace(runtime.SystemPrompt)
		middlewares = append([]MiddlewareUse(nil), runtime.Middlewares...)
		tools = append([]string(nil), runtime.Tools...)
	}
	if inferenceSettings != nil {
		stepMetadata = inferenceSettings.GetMetadata()
	}
	return BuildRuntimeFingerprint(RuntimeFingerprintInput{
		ProfileVersion: profileVersion,
		RuntimeKey:     runtimeKey,
		SystemPrompt:   systemPrompt,
		Middlewares:    middlewares,
		Tools:          tools,
		StepMetadata:   stepMetadata,
	})
}

func ProfileVersionFromResolvedMetadata(metadata map[string]any) uint64 {
	raw := metadata["profile.version"]
	switch v := raw.(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case uint:
		return uint64(v)
	case int64:
		if v >= 0 {
			return uint64(v)
		}
	case int:
		if v >= 0 {
			return uint64(v)
		}
	case float64:
		if v >= 0 {
			return uint64(v)
		}
	}
	return 0
}

func metadataFromResolvedProfile(resolved *gepprofiles.ResolvedEngineProfile) map[string]any {
	if resolved == nil {
		return nil
	}
	return resolved.Metadata
}

func cloneInferenceSettings(in *aisettings.InferenceSettings) *aisettings.InferenceSettings {
	if in == nil {
		return nil
	}
	return in.Clone()
}

func cloneProfileRuntime(in *ProfileRuntime) *ProfileRuntime {
	if in == nil {
		return nil
	}
	return in.Clone()
}

func copyMetadataMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeRuntimeMiddlewares(base []MiddlewareUse, overlay []MiddlewareUse) []MiddlewareUse {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}

	out := make([]MiddlewareUse, 0, len(base)+len(overlay))
	positions := map[string]int{}

	appendUse := func(use MiddlewareUse) {
		key := runtimeMiddlewareKey(use)
		clone := MiddlewareUse{
			Name:    strings.TrimSpace(use.Name),
			ID:      strings.TrimSpace(use.ID),
			Enabled: cloneBoolPtr(use.Enabled),
			Config:  copyStringAnyMap(use.Config),
		}
		if idx, ok := positions[key]; ok {
			out[idx] = clone
			return
		}
		positions[key] = len(out)
		out = append(out, clone)
	}

	for _, use := range base {
		appendUse(use)
	}
	for _, use := range overlay {
		appendUse(use)
	}

	return out
}

func runtimeMiddlewareKey(use MiddlewareUse) string {
	name := strings.TrimSpace(use.Name)
	id := strings.TrimSpace(use.ID)
	if id == "" {
		return name
	}
	return name + "\x00" + id
}

func mergeRuntimeTools(base []string, overlay []string, mode ToolMergeMode) []string {
	switch mode {
	case ToolMergeModeReplace:
		return normalizeRuntimeTools(overlay)
	case ToolMergeModeUnion:
		if len(base) == 0 && len(overlay) == 0 {
			return nil
		}
		out := make([]string, 0, len(base)+len(overlay))
		seen := map[string]struct{}{}
		appendTool := func(tool string) {
			name := strings.TrimSpace(tool)
			if name == "" {
				return
			}
			if _, ok := seen[name]; ok {
				return
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
		for _, tool := range base {
			appendTool(tool)
		}
		for _, tool := range overlay {
			appendTool(tool)
		}
		return out
	}
	return nil
}

func normalizeRuntimeTools(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, tool := range in {
		name := strings.TrimSpace(tool)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
