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

// Resolved captures the selected engine profile and the final merged
// InferenceSettings used by the UI and persistence layers.
type Resolved struct {
	RegistrySlug      gepprofiles.RegistrySlug
	ProfileSlug       gepprofiles.EngineProfileSlug
	InferenceSettings *settings.InferenceSettings

	ProfileVersion uint64
	Metadata       map[string]any
}
