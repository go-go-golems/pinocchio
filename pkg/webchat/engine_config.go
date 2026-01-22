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

// Signature returns a deterministic representation of the configuration.
// We intentionally return the JSON string itself (instead of hashing) to keep it debuggable.
func (ec EngineConfig) Signature() string {
	b, err := json.Marshal(ec)
	if err != nil {
		log.Warn().Err(err).Str("component", "webchat").Msg("engine config signature fallback")
		return ec.ProfileSlug
	}
	return string(b)
}
