package configdoc

import (
	"path/filepath"
	"strings"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/pkg/errors"
)

const (
	LocalOverrideFileName       = ".pinocchio.yml"
	LegacyLocalOverrideFileName = ".pinocchio-profile.yml"
)

type Document struct {
	App      AppBlock                  `yaml:"app"`
	Profile  ProfileBlock              `yaml:"profile"`
	Profiles map[string]*InlineProfile `yaml:"profiles"`
}

type AppBlock struct {
	Repositories []string `yaml:"repositories,omitempty"`

	hasRepositories bool `yaml:"-"`
}

type ProfileBlock struct {
	Active     string   `yaml:"active,omitempty"`
	Registries []string `yaml:"registries,omitempty"`

	hasActive     bool `yaml:"-"`
	hasRegistries bool `yaml:"-"`
}

type InlineProfile struct {
	DisplayName       string                         `yaml:"display_name,omitempty"`
	Description       string                         `yaml:"description,omitempty"`
	Stack             []gepprofiles.EngineProfileRef `yaml:"stack,omitempty"`
	InferenceSettings *aisettings.InferenceSettings  `yaml:"inference_settings,omitempty"`
	Extensions        map[string]any                 `yaml:"extensions,omitempty"`

	hasDisplayName       bool `yaml:"-"`
	hasDescription       bool `yaml:"-"`
	hasStack             bool `yaml:"-"`
	hasInferenceSettings bool `yaml:"-"`
	hasExtensions        bool `yaml:"-"`
}

func ValidateLocalOverrideFileName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	base := filepath.Base(trimmed)
	if base == LegacyLocalOverrideFileName {
		return errors.Errorf("legacy local config filename %q is no longer supported; rename it to %q", LegacyLocalOverrideFileName, LocalOverrideFileName)
	}
	return nil
}

func (d *Document) NormalizeAndValidate() error {
	if d == nil {
		return errors.New("config document cannot be nil")
	}

	normalizedRepositories := make([]string, 0, len(d.App.Repositories))
	for i, repo := range d.App.Repositories {
		trimmed := strings.TrimSpace(repo)
		if trimmed == "" {
			return errors.Errorf("app.repositories[%d] cannot be empty", i)
		}
		normalizedRepositories = append(normalizedRepositories, trimmed)
	}
	d.App.Repositories = normalizedRepositories
	if len(d.App.Repositories) == 0 {
		d.App.Repositories = nil
	}

	if strings.TrimSpace(d.Profile.Active) != "" {
		slug, err := gepprofiles.ParseEngineProfileSlug(d.Profile.Active)
		if err != nil {
			return errors.Wrap(err, "profile.active")
		}
		d.Profile.Active = slug.String()
	}

	normalizedRegistries := make([]string, 0, len(d.Profile.Registries))
	for i, entry := range d.Profile.Registries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			return errors.Errorf("profile.registries[%d] cannot be empty", i)
		}
		normalizedRegistries = append(normalizedRegistries, trimmed)
	}
	d.Profile.Registries = normalizedRegistries
	if len(d.Profile.Registries) == 0 {
		d.Profile.Registries = nil
	}

	if len(d.Profiles) == 0 {
		d.Profiles = nil
		return nil
	}

	normalizedProfiles := make(map[string]*InlineProfile, len(d.Profiles))
	for rawSlug, profile := range d.Profiles {
		if profile == nil {
			return errors.Errorf("profiles.%s cannot be null", rawSlug)
		}
		slug, err := gepprofiles.ParseEngineProfileSlug(rawSlug)
		if err != nil {
			return errors.Wrapf(err, "profiles.%s", rawSlug)
		}
		normalizedSlug := slug.String()
		if _, ok := normalizedProfiles[normalizedSlug]; ok {
			return errors.Errorf("duplicate profile slug %q after normalization", normalizedSlug)
		}
		normalizedProfiles[normalizedSlug] = profile
	}
	d.Profiles = normalizedProfiles
	return nil
}

func cloneDocument(in *Document) *Document {
	if in == nil {
		return nil
	}
	ret := &Document{
		App: AppBlock{
			Repositories:    append([]string(nil), in.App.Repositories...),
			hasRepositories: in.App.hasRepositories,
		},
		Profile: ProfileBlock{
			Active:        in.Profile.Active,
			Registries:    append([]string(nil), in.Profile.Registries...),
			hasActive:     in.Profile.hasActive,
			hasRegistries: in.Profile.hasRegistries,
		},
	}
	if len(in.Profiles) > 0 {
		ret.Profiles = make(map[string]*InlineProfile, len(in.Profiles))
		for slug, profile := range in.Profiles {
			ret.Profiles[slug] = cloneInlineProfile(profile)
		}
	}
	return ret
}

func cloneInlineProfile(in *InlineProfile) *InlineProfile {
	if in == nil {
		return nil
	}
	var clonedInferenceSettings *aisettings.InferenceSettings
	if in.InferenceSettings != nil {
		clonedInferenceSettings = in.InferenceSettings.Clone()
	}
	return &InlineProfile{
		DisplayName:          in.DisplayName,
		Description:          in.Description,
		Stack:                append([]gepprofiles.EngineProfileRef(nil), in.Stack...),
		InferenceSettings:    clonedInferenceSettings,
		Extensions:           deepCopyStringAnyMap(in.Extensions),
		hasDisplayName:       in.hasDisplayName,
		hasDescription:       in.hasDescription,
		hasStack:             in.hasStack,
		hasInferenceSettings: in.hasInferenceSettings,
		hasExtensions:        in.hasExtensions,
	}
}

func deepCopyStringAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	ret := make(map[string]any, len(in))
	for k, v := range in {
		ret[k] = deepCopyAny(v)
	}
	return ret
}

func deepCopyAny(in any) any {
	switch v := in.(type) {
	case map[string]any:
		return deepCopyStringAnyMap(v)
	case []any:
		ret := make([]any, 0, len(v))
		for _, item := range v {
			ret = append(ret, deepCopyAny(item))
		}
		return ret
	default:
		return in
	}
}
