package configdoc

import (
	"context"
	"errors"
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

func ComposeRegistry(imported gepprofiles.Registry, inline *gepprofiles.StoreRegistry) gepprofiles.Registry {
	switch {
	case inline == nil:
		return imported
	case imported == nil:
		return inline
	default:
		return &composedRegistry{
			imported: imported,
			inline:   inline,
		}
	}
}

type composedRegistry struct {
	imported gepprofiles.Registry
	inline   *gepprofiles.StoreRegistry
}

var _ gepprofiles.Registry = (*composedRegistry)(nil)

func (c *composedRegistry) ListRegistries(ctx context.Context) ([]gepprofiles.RegistrySummary, error) {
	results := []gepprofiles.RegistrySummary{}
	seen := map[gepprofiles.RegistrySlug]struct{}{}
	if c.inline != nil {
		items, err := c.inline.ListRegistries(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			seen[item.Slug] = struct{}{}
			results = append(results, item)
		}
	}
	if c.imported != nil {
		items, err := c.imported.ListRegistries(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if _, ok := seen[item.Slug]; ok {
				continue
			}
			results = append(results, item)
		}
	}
	return results, nil
}

func (c *composedRegistry) GetRegistry(ctx context.Context, registrySlug gepprofiles.RegistrySlug) (*gepprofiles.EngineProfileRegistry, error) {
	if c.inline != nil {
		reg, err := c.inline.GetRegistry(ctx, registrySlug)
		if err == nil {
			return reg, nil
		}
		if !errors.Is(err, gepprofiles.ErrRegistryNotFound) {
			return nil, err
		}
	}
	if c.imported == nil {
		return nil, gepprofiles.ErrRegistryNotFound
	}
	return c.imported.GetRegistry(ctx, registrySlug)
}

func (c *composedRegistry) ListEngineProfiles(ctx context.Context, registrySlug gepprofiles.RegistrySlug) ([]*gepprofiles.EngineProfile, error) {
	if c.inline != nil {
		profiles, err := c.inline.ListEngineProfiles(ctx, registrySlug)
		if err == nil {
			return profiles, nil
		}
		if !errors.Is(err, gepprofiles.ErrRegistryNotFound) {
			return nil, err
		}
	}
	if c.imported == nil {
		return nil, gepprofiles.ErrRegistryNotFound
	}
	return c.imported.ListEngineProfiles(ctx, registrySlug)
}

func (c *composedRegistry) GetEngineProfile(ctx context.Context, registrySlug gepprofiles.RegistrySlug, profileSlug gepprofiles.EngineProfileSlug) (*gepprofiles.EngineProfile, error) {
	if c.inline != nil {
		profile, err := c.inline.GetEngineProfile(ctx, registrySlug, profileSlug)
		if err == nil {
			return profile, nil
		}
		if !errors.Is(err, gepprofiles.ErrRegistryNotFound) && !errors.Is(err, gepprofiles.ErrProfileNotFound) {
			return nil, err
		}
	}
	if c.imported == nil {
		return nil, gepprofiles.ErrProfileNotFound
	}
	return c.imported.GetEngineProfile(ctx, registrySlug, profileSlug)
}

func (c *composedRegistry) ResolveEngineProfile(ctx context.Context, in gepprofiles.ResolveInput) (*gepprofiles.ResolvedEngineProfile, error) {
	if c.inline != nil {
		inlineRegistries, err := c.inline.ListRegistries(ctx)
		if err != nil {
			return nil, err
		}
		if len(inlineRegistries) > 0 {
			inlineRegistrySlug := inlineRegistries[0].Slug
			switch {
			case !in.RegistrySlug.IsZero() && in.RegistrySlug == inlineRegistrySlug:
				return c.inline.ResolveEngineProfile(ctx, in)
			case in.RegistrySlug.IsZero() && in.EngineProfileSlug.IsZero():
				return c.inline.ResolveEngineProfile(ctx, in)
			case in.RegistrySlug.IsZero() && !in.EngineProfileSlug.IsZero():
				resolved, err := c.inline.ResolveEngineProfile(ctx, gepprofiles.ResolveInput{
					RegistrySlug:      inlineRegistrySlug,
					EngineProfileSlug: in.EngineProfileSlug,
				})
				if err == nil {
					return resolved, nil
				}
				if !errors.Is(err, gepprofiles.ErrProfileNotFound) && !errors.Is(err, gepprofiles.ErrRegistryNotFound) {
					return nil, err
				}
			}
		}
	}
	if c.imported == nil {
		return nil, gepprofiles.ErrProfileNotFound
	}
	return c.imported.ResolveEngineProfile(ctx, in)
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
