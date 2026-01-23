package webchat

import (
	"encoding/json"

	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/rs/zerolog/log"
)

// EngineConfig captures all inputs that influence engine composition.
// It is JSON-serializable so we can derive a deterministic signature for caching and rebuild decisions.
type EngineConfig struct {
	ProfileSlug  string                 `json:"profile_slug"`
	SystemPrompt string                 `json:"system_prompt"`
	Middlewares  []MiddlewareUse        `json:"middlewares"`
	StepSettings *settings.StepSettings `json:"step_settings"`
}

type engineConfigSignature struct {
	ProfileSlug  string          `json:"profile_slug"`
	SystemPrompt string          `json:"system_prompt"`
	Middlewares  []MiddlewareUse `json:"middlewares"`
	// StepMetadata is a sanitized subset of StepSettings that excludes secrets (notably API keys).
	// It is used only for rebuild decisions and should remain safe to log/debug.
	StepMetadata map[string]any `json:"step_metadata,omitempty"`
}

// Signature returns a deterministic representation of the configuration.
// We intentionally return the JSON string itself (instead of hashing) to keep it debuggable.
func (ec EngineConfig) Signature() string {
	var meta map[string]any
	if ec.StepSettings != nil {
		meta = ec.StepSettings.GetMetadata()
	}
	sig := engineConfigSignature{
		ProfileSlug:  ec.ProfileSlug,
		SystemPrompt: ec.SystemPrompt,
		Middlewares:  ec.Middlewares,
		StepMetadata: meta,
	}
	b, err := json.Marshal(sig)
	if err != nil {
		log.Warn().Err(err).Str("component", "webchat").Msg("engine config signature fallback")
		return ec.ProfileSlug
	}
	return string(b)
}
