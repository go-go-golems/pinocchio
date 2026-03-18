package profileswitch

import (
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

type ProfileListItem struct {
	RegistrySlug gepprofiles.RegistrySlug
	ProfileSlug  gepprofiles.EngineProfileSlug
	DisplayName  string
	Description  string
	IsDefault    bool
	Version      uint64
}

// Resolved captures the resolved effective runtime and attribution payload used
// by the UI and persistence layers.
type Resolved struct {
	RegistrySlug gepprofiles.RegistrySlug
	ProfileSlug  gepprofiles.EngineProfileSlug
	RuntimeKey   gepprofiles.RuntimeKey

	RuntimeFingerprint string

	SystemPrompt      string
	InferenceSettings *settings.InferenceSettings

	ProfileVersion uint64
	Metadata       map[string]any
}
