package configdoc

import (
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

func MergeDocuments(low, high *Document) (*Document, error) {
	if low == nil {
		return cloneDocument(high), nil
	}
	if high == nil {
		return cloneDocument(low), nil
	}

	ret := cloneDocument(low)
	if ret == nil {
		ret = &Document{}
	}

	if high.App.hasRepositories {
		ret.App.Repositories = mergeRepositories(ret.App.Repositories, high.App.Repositories)
		ret.App.hasRepositories = ret.App.hasRepositories || high.App.hasRepositories
	}

	if high.Profile.hasActive {
		ret.Profile.Active = high.Profile.Active
		ret.Profile.hasActive = true
	}
	if high.Profile.hasRegistries {
		ret.Profile.Registries = append([]string(nil), high.Profile.Registries...)
		ret.Profile.hasRegistries = true
	}

	if len(high.Profiles) > 0 {
		if ret.Profiles == nil {
			ret.Profiles = map[string]*InlineProfile{}
		}
		for slug, highProfile := range high.Profiles {
			lowProfile, ok := ret.Profiles[slug]
			if !ok {
				ret.Profiles[slug] = cloneInlineProfile(highProfile)
				continue
			}
			merged, err := mergeInlineProfiles(lowProfile, highProfile)
			if err != nil {
				return nil, err
			}
			ret.Profiles[slug] = merged
		}
	}

	return ret, nil
}

func mergeRepositories(low, high []string) []string {
	ret := make([]string, 0, len(low)+len(high))
	seen := map[string]struct{}{}
	for _, repo := range low {
		if _, ok := seen[repo]; ok {
			continue
		}
		seen[repo] = struct{}{}
		ret = append(ret, repo)
	}
	for _, repo := range high {
		if _, ok := seen[repo]; ok {
			continue
		}
		seen[repo] = struct{}{}
		ret = append(ret, repo)
	}
	if len(ret) == 0 {
		return nil
	}
	return ret
}

func mergeInlineProfiles(low, high *InlineProfile) (*InlineProfile, error) {
	if low == nil {
		return cloneInlineProfile(high), nil
	}
	if high == nil {
		return cloneInlineProfile(low), nil
	}

	ret := cloneInlineProfile(low)
	if high.hasDisplayName {
		ret.DisplayName = high.DisplayName
		ret.hasDisplayName = true
	}
	if high.hasDescription {
		ret.Description = high.Description
		ret.hasDescription = true
	}
	if high.hasStack {
		ret.Stack = append([]gepprofiles.EngineProfileRef(nil), high.Stack...)
		ret.hasStack = true
	}
	if high.hasInferenceSettings {
		merged, err := gepprofiles.MergeInferenceSettings(ret.InferenceSettings, high.InferenceSettings)
		if err != nil {
			return nil, err
		}
		ret.InferenceSettings = merged
		ret.hasInferenceSettings = true
	}
	if high.hasExtensions {
		ret.Extensions = mergeStringAnyMaps(ret.Extensions, high.Extensions)
		ret.hasExtensions = true
	}
	return ret, nil
}

func mergeStringAnyMaps(low, high map[string]any) map[string]any {
	if len(low) == 0 {
		return deepCopyStringAnyMap(high)
	}
	if len(high) == 0 {
		return deepCopyStringAnyMap(low)
	}
	ret := deepCopyStringAnyMap(low)
	for k, highValue := range high {
		if lowValue, ok := ret[k]; ok {
			ret[k] = mergeAny(lowValue, highValue)
			continue
		}
		ret[k] = deepCopyAny(highValue)
	}
	return ret
}

func mergeAny(low, high any) any {
	lowMap, lowOK := low.(map[string]any)
	highMap, highOK := high.(map[string]any)
	if lowOK && highOK {
		return mergeStringAnyMaps(lowMap, highMap)
	}
	return deepCopyAny(high)
}
