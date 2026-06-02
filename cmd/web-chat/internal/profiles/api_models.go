package profiles

import (
	"encoding/json"
	"sort"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

func profileDocFromModel(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.EngineProfileRegistry, p *gepprofiles.EngineProfile) ProfileDocument {
	doc := ProfileDocument{Registry: registrySlug.String()}
	if p == nil {
		return doc
	}
	doc.Slug = p.Slug.String()
	doc.DisplayName = p.DisplayName
	doc.Description = p.Description
	if runtime, _, err := infruntime.ProfileRuntimeFromEngineProfile(p); err == nil {
		doc.Runtime = runtime
	}
	doc.Metadata = p.Metadata
	doc.Extensions = cloneExtensionMap(p.Extensions)
	if p.InferenceSettings != nil {
		doc.ModelInfo = p.InferenceSettings.ModelInfo.Clone()
	}
	doc.IsDefault = registry != nil && registry.DefaultEngineProfileSlug == p.Slug
	return doc
}

func profileListItemsFromRegistry(registrySlug gepprofiles.RegistrySlug, registry *gepprofiles.EngineProfileRegistry, profiles_ []*gepprofiles.EngineProfile) []ProfileListItem {
	sort.Slice(profiles_, func(i, j int) bool {
		if profiles_[i] == nil {
			return false
		}
		if profiles_[j] == nil {
			return true
		}
		return profiles_[i].Slug < profiles_[j].Slug
	})

	items := make([]ProfileListItem, 0, len(profiles_))
	for _, p := range profiles_ {
		if p == nil {
			continue
		}
		defaultPrompt := ""
		if runtime, _, err := infruntime.ProfileRuntimeFromEngineProfile(p); err == nil && runtime != nil {
			defaultPrompt = runtime.SystemPrompt
		}
		var modelInfo *aisettings.ModelInfo
		if p.InferenceSettings != nil {
			modelInfo = p.InferenceSettings.ModelInfo.Clone()
		}
		items = append(items, ProfileListItem{
			Registry:      registrySlug.String(),
			Slug:          p.Slug.String(),
			DisplayName:   p.DisplayName,
			Description:   p.Description,
			DefaultPrompt: defaultPrompt,
			Extensions:    cloneExtensionMap(p.Extensions),
			ModelInfo:     modelInfo,
			IsDefault:     registry != nil && registry.DefaultEngineProfileSlug == p.Slug,
			Version:       p.Metadata.Version,
		})
	}
	return items
}

func cloneExtensionMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func mockParityProfileListItem(registrySlug gepprofiles.RegistrySlug) ProfileListItem {
	return ProfileListItem{
		Registry:    registrySlug.String(),
		Slug:        MockParityProfile,
		DisplayName: "Mock parity engine",
		Description: "Deterministic web-chat event stream for parity tests; no LLM/API key required.",
		IsDefault:   false,
	}
}

func mockParityProfileDocument(registrySlug gepprofiles.RegistrySlug) ProfileDocument {
	return ProfileDocument{
		Registry:    registrySlug.String(),
		Slug:        MockParityProfile,
		DisplayName: "Mock parity engine",
		Description: "Deterministic web-chat event stream for parity tests; no LLM/API key required.",
		IsDefault:   false,
	}
}
