package cmds

import (
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	profilebootstrap "github.com/go-go-golems/pinocchio/pkg/cmds/profilebootstrap"
)

// baseSettingsFromParsedValues computes a InferenceSettings that does not include any parsed field values
// that originated from the profile registry middleware (source == "profiles").
//
// This allows interactive chat to switch profiles by re-applying a new profile patch onto the same
// underlying config/env/flag baseline.
func baseSettingsFromParsedValues(parsed *values.Values) (*settings.InferenceSettings, error) {
	return profilebootstrap.ResolveParsedBaseInferenceSettings(parsed)
}

func baseSettingsFromParsedValuesWithBase(parsed *values.Values, initial *settings.InferenceSettings) (*settings.InferenceSettings, error) {
	return profilebootstrap.ResolveParsedBaseInferenceSettingsWithBase(parsed, initial)
}
