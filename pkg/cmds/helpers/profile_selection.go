package helpers

import (
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

const ProfileSettingsSectionSlug = profilebootstrap.ProfileSettingsSectionSlug

type ProfileSettings = profilebootstrap.ProfileSettings
type ResolvedCLIProfileSelection = profilebootstrap.ResolvedCLIProfileSelection

func NewProfileSettingsSection() (schema.Section, error) {
	return profilebootstrap.NewProfileSettingsSection()
}

func ResolveProfileSettings(parsed *values.Values) ProfileSettings {
	return profilebootstrap.ResolveProfileSettings(parsed)
}

func ResolveCLIProfileSelection(parsed *values.Values) (*ResolvedCLIProfileSelection, error) {
	return profilebootstrap.ResolveCLIProfileSelection(parsed)
}

func ResolveEngineProfileSettings(parsed *values.Values) (ProfileSettings, []string, error) {
	return profilebootstrap.ResolveEngineProfileSettings(parsed)
}
