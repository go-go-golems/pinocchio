package helpers

import (
	"strings"

	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

const ProfileSettingsSectionSlug = geppettosections.ProfileSettingsSectionSlug

type ProfileSettings struct {
	Profile           string   `glazed:"profile"`
	ProfileRegistries []string `glazed:"profile-registries"`
}

type ResolvedCLIProfileSelection struct {
	ProfileSettings
	ConfigFiles []string
}

func NewProfileSettingsSection() (schema.Section, error) {
	return geppettosections.NewProfileSettingsSection()
}

func ResolveProfileSettings(parsed *values.Values) ProfileSettings {
	ret := ProfileSettings{}
	if parsed != nil {
		_ = parsed.DecodeSectionInto(ProfileSettingsSectionSlug, &ret)
	}
	ret.Profile = strings.TrimSpace(ret.Profile)
	ret.ProfileRegistries = normalizeProfileRegistries(ret.ProfileRegistries)
	if len(ret.ProfileRegistries) == 0 {
		if defaultPath := defaultPinocchioProfileRegistriesIfPresent(); defaultPath != "" {
			ret.ProfileRegistries = []string{defaultPath}
		}
	}
	return ret
}

func ResolveCLIProfileSelection(parsed *values.Values) (*ResolvedCLIProfileSelection, error) {
	profileSection, err := NewProfileSettingsSection()
	if err != nil {
		return nil, errors.Wrap(err, "create profile settings section")
	}

	schema_ := schema.NewSchema(schema.WithSections(profileSection))
	resolvedValues := values.New()
	configFiles, err := resolveConfigFiles(parsed)
	if err != nil {
		return nil, err
	}
	if err := sources.Execute(
		schema_,
		resolvedValues,
		sources.FromEnv("PINOCCHIO", fields.WithSource("env")),
		sources.FromFiles(
			configFiles,
			sources.WithConfigFileMapper(configFileMapper),
			sources.WithParseOptions(fields.WithSource("config")),
		),
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	); err != nil {
		return nil, errors.Wrap(err, "resolve profile settings from config/env/defaults")
	}
	if parsed != nil {
		if err := resolvedValues.Merge(parsed); err != nil {
			return nil, errors.Wrap(err, "merge explicit profile settings")
		}
	}

	profileSettings := ResolveProfileSettings(resolvedValues)
	return &ResolvedCLIProfileSelection{
		ProfileSettings: profileSettings,
		ConfigFiles:     append([]string(nil), configFiles...),
	}, nil
}

func ResolveEngineProfileSettings(parsed *values.Values) (ProfileSettings, []string, error) {
	resolved, err := ResolveCLIProfileSelection(parsed)
	if err != nil {
		return ProfileSettings{}, nil, err
	}
	return resolved.ProfileSettings, resolved.ConfigFiles, nil
}

func normalizeProfileRegistries(entries []string) []string {
	ret := make([]string, 0, len(entries))
	for _, entry := range entries {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			ret = append(ret, trimmed)
		}
	}
	return ret
}
