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
}

type ProfileBlock struct {
	Active     string   `yaml:"active,omitempty"`
	Registries []string `yaml:"registries,omitempty"`
}

type InlineProfile struct {
	DisplayName       string                         `yaml:"display_name,omitempty"`
	Description       string                         `yaml:"description,omitempty"`
	Stack             []gepprofiles.EngineProfileRef `yaml:"stack,omitempty"`
	InferenceSettings *aisettings.InferenceSettings  `yaml:"inference_settings,omitempty"`
	Extensions        map[string]any                 `yaml:"extensions,omitempty"`
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
