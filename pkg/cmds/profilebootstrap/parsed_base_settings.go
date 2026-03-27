package profilebootstrap

import (
	"strings"

	embeddingconfig "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
)

// ResolveParsedBaseInferenceSettings computes InferenceSettings from parsed values
// after removing any field values whose last active source came from the profiles middleware.
func ResolveParsedBaseInferenceSettings(parsed *values.Values) (*aisettings.InferenceSettings, error) {
	return ResolveParsedBaseInferenceSettingsWithBase(parsed, nil)
}

// ResolveParsedBaseInferenceSettingsWithBase overlays parsed non-profile values
// onto an optional initial baseline. This is useful when a command already rebuilt
// a hidden base from config/env/defaults and wants to preserve explicit parsed CLI
// values without letting profile-derived values contaminate that baseline.
func ResolveParsedBaseInferenceSettingsWithBase(
	parsed *values.Values,
	initial *aisettings.InferenceSettings,
) (*aisettings.InferenceSettings, error) {
	if parsed == nil {
		return nil, errors.New("base settings: parsed values is nil")
	}

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
				toDelete = append(toDelete, k)
				return
			}

			step := fv.Log[lastIdx]
			typed, err := fv.Definition.CheckValueValidity(step.Value)
			if err != nil {
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

	var (
		ss  *aisettings.InferenceSettings
		err error
	)
	if initial != nil {
		ss = initial.Clone()
	} else {
		ss, err = aisettings.NewInferenceSettings()
		if err != nil {
			return nil, err
		}
	}
	if err := overlayInferenceSettingsFromParsedValues(ss, base); err != nil {
		return nil, err
	}
	return ss, nil
}

func overlayInferenceSettingsFromParsedValues(ss *aisettings.InferenceSettings, parsed *values.Values) error {
	if ss == nil {
		return errors.New("base settings: inference settings is nil")
	}

	if _, ok := parsed.Get(aisettings.AiClientSlug); ok {
		if err := parsed.DecodeSectionInto(aisettings.AiClientSlug, ss.Client); err != nil {
			return err
		}
	}
	if _, ok := parsed.Get(aisettings.AiChatSlug); ok {
		if err := parsed.DecodeSectionInto(aisettings.AiChatSlug, ss.Chat); err != nil {
			return err
		}
	}
	if _, ok := parsed.Get(openai.OpenAiChatSlug); ok {
		if err := parsed.DecodeSectionInto(openai.OpenAiChatSlug, ss.OpenAI); err != nil {
			return err
		}
		if err := parsed.DecodeSectionInto(openai.OpenAiChatSlug, ss.API); err != nil {
			return err
		}
	}
	if _, ok := parsed.Get(claude.ClaudeChatSlug); ok {
		if err := parsed.DecodeSectionInto(claude.ClaudeChatSlug, ss.Claude); err != nil {
			return err
		}
		if err := parsed.DecodeSectionInto(claude.ClaudeChatSlug, ss.API); err != nil {
			return err
		}
	}
	if _, ok := parsed.Get(gemini.GeminiChatSlug); ok {
		if err := parsed.DecodeSectionInto(gemini.GeminiChatSlug, ss.Gemini); err != nil {
			return err
		}
		if err := parsed.DecodeSectionInto(gemini.GeminiChatSlug, ss.API); err != nil {
			return err
		}
	}
	if _, ok := parsed.Get(embeddingconfig.EmbeddingsSlug); ok {
		if err := parsed.DecodeSectionInto(embeddingconfig.EmbeddingsSlug, ss.Embeddings); err != nil {
			return err
		}
		if err := parsed.DecodeSectionInto(embeddingconfig.EmbeddingsSlug, ss.API); err != nil {
			return err
		}
	}
	if _, ok := parsed.Get(aisettings.AiInferenceSlug); ok {
		if ss.Inference == nil {
			ss.Inference = &engine.InferenceConfig{}
		}
		if err := parsed.DecodeSectionInto(aisettings.AiInferenceSlug, ss.Inference); err != nil {
			return err
		}
	}

	return nil
}
