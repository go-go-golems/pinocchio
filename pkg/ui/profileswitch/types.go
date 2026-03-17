package profileswitch

import (
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

type ProfileListItem struct {
	RegistrySlug gepprofiles.RegistrySlug
	ProfileSlug  gepprofiles.ProfileSlug
	DisplayName  string
	Description  string
	IsDefault    bool
	Version      uint64
}

// Resolved captures the resolved effective runtime and attribution payload used
// by the UI and persistence layers.
type Resolved struct {
	RegistrySlug gepprofiles.RegistrySlug
	ProfileSlug  gepprofiles.ProfileSlug
	RuntimeKey   gepprofiles.RuntimeKey

	RuntimeFingerprint string

	SystemPrompt string
	StepSettings *settings.StepSettings

	ProfileVersion uint64
	Metadata       map[string]any
}
