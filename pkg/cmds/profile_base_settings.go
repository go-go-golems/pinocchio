package cmds

import (
	"strings"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

// baseSettingsFromParsedValues computes a StepSettings that does not include any parsed field values
// that originated from the profile registry middleware (source == "profiles").
//
// This allows interactive chat to switch profiles by re-applying a new profile patch onto the same
// underlying config/env/flag baseline.
func baseSettingsFromParsedValues(parsed *values.Values) (*settings.StepSettings, error) {
	if parsed == nil {
		return nil, errors.New("base settings: parsed values is nil")
	}

	// Create a clone so we can mutate values in-place.
	base := parsed.Clone()
	base.ForEach(func(_ string, sv *values.SectionValues) {
		if sv == nil || sv.Fields == nil {
			return
		}

		toDelete := []string{}
		sv.Fields.ForEach(func(k string, fv *fields.FieldValue) {
			if fv == nil || fv.Definition == nil {
				toDelete = append(toDelete, k)
				return
			}

			// Find the last parse step not originating from the profile registry stack.
			lastIdx := -1
			for i := len(fv.Log) - 1; i >= 0; i-- {
				step := fv.Log[i]
				if strings.TrimSpace(step.Source) == "profiles" {
					continue
				}
				lastIdx = i
				break
			}

			if lastIdx < 0 {
				// Only profile-derived value existed -> unset and let StepSettings defaults apply.
				toDelete = append(toDelete, k)
				return
			}

			step := fv.Log[lastIdx]
			typed, err := fv.Definition.CheckValueValidity(step.Value)
			if err != nil {
				// Be conservative: if we can't validate, unset rather than carrying a potentially invalid type.
				toDelete = append(toDelete, k)
				return
			}

			fv.Value = typed
			fv.Log = []fields.ParseStep{step}
		})

		for _, k := range toDelete {
			sv.Fields.Delete(k)
		}
	})

	ss, err := settings.NewStepSettings()
	if err != nil {
		return nil, err
	}
	if err := ss.UpdateFromParsedValues(base); err != nil {
		return nil, err
	}
	return ss, nil
}
