package configdoc

import (
	"context"
	"sort"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
)

const DefaultInlineRegistrySlug = "config-inline"

func InlineProfilesToRegistry(doc *Document, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.EngineProfileRegistry, error) {
	if registrySlug.IsZero() {
		registrySlug = gepprofiles.MustRegistrySlug(DefaultInlineRegistrySlug)
	}

	ret := &gepprofiles.EngineProfileRegistry{
		Slug:     registrySlug,
		Profiles: map[gepprofiles.EngineProfileSlug]*gepprofiles.EngineProfile{},
	}
	if doc == nil || len(doc.Profiles) == 0 {
		return ret, nil
	}

	slugs := make([]string, 0, len(doc.Profiles))
	for slug := range doc.Profiles {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, rawSlug := range slugs {
		profile := doc.Profiles[rawSlug]
		parsedSlug := gepprofiles.MustEngineProfileSlug(rawSlug)
		var clonedInferenceSettings = profile.InferenceSettings
		if profile.InferenceSettings != nil {
			clonedInferenceSettings = profile.InferenceSettings.Clone()
		}
		ret.Profiles[parsedSlug] = &gepprofiles.EngineProfile{
			Slug:              parsedSlug,
			DisplayName:       profile.DisplayName,
			Description:       profile.Description,
			Stack:             append([]gepprofiles.EngineProfileRef(nil), profile.Stack...),
			InferenceSettings: clonedInferenceSettings,
			Extensions:        deepCopyStringAnyMap(profile.Extensions),
		}
	}

	ret.DefaultEngineProfileSlug = resolveDefaultInlineProfileSlug(ret)
	return ret, nil
}

func NewInlineStoreRegistry(doc *Document, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.StoreRegistry, error) {
	reg, err := InlineProfilesToRegistry(doc, registrySlug)
	if err != nil {
		return nil, err
	}
	store := gepprofiles.NewInMemoryEngineProfileStore()
	if err := store.UpsertRegistry(context.Background(), reg, gepprofiles.SaveOptions{}); err != nil {
		return nil, err
	}
	return gepprofiles.NewStoreRegistry(store, reg.Slug)
}

func resolveDefaultInlineProfileSlug(reg *gepprofiles.EngineProfileRegistry) gepprofiles.EngineProfileSlug {
	if reg == nil || len(reg.Profiles) == 0 {
		return ""
	}
	if _, ok := reg.Profiles[gepprofiles.MustEngineProfileSlug("default")]; ok {
		return gepprofiles.MustEngineProfileSlug("default")
	}
	slugs := make([]string, 0, len(reg.Profiles))
	for slug := range reg.Profiles {
		slugs = append(slugs, slug.String())
	}
	sort.Strings(slugs)
	return gepprofiles.MustEngineProfileSlug(slugs[0])
}
