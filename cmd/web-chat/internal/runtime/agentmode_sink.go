package runtime

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/events"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
)

type agentModeSinkConfigInput struct {
	SanitizeYAML *bool `json:"sanitize_yaml,omitempty"`
}

// runtimeSinkWrapperFromProfile builds a runtime-owned event sink wrapper that injects
// the agentmode structured-output parser when the profile runtime has agentmode configured.
// It is cmd/web-chat specific and stays out of the shared sessionstream core.
func runtimeSinkWrapperFromProfile(runtime *infruntime.ProfileRuntime) infruntime.EventSinkWrapper {
	if runtime == nil {
		return nil
	}
	found := false
	cfg := agentmode.DefaultStructuredSinkConfig()
	for _, mw := range runtime.Middlewares {
		if !strings.EqualFold(strings.TrimSpace(mw.Name), "agentmode") {
			continue
		}
		if mw.Enabled != nil && !*mw.Enabled {
			continue
		}
		found = true
		input := agentModeSinkConfigInput{}
		if err := decodeAgentModeSinkConfig(mw.Config, &input); err == nil && input.SanitizeYAML != nil {
			cfg.ParseOptions = cfg.ParseOptions.WithSanitizeYAML(*input.SanitizeYAML)
		}
	}
	if !found {
		return nil
	}
	return func(next events.EventSink) (events.EventSink, error) {
		return agentmode.WrapStructuredSink(next, cfg), nil
	}
}

func decodeAgentModeSinkConfig(cfg any, out any) error {
	if cfg == nil || out == nil {
		return nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize agent mode sink config: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode agent mode sink config: %w", err)
	}
	return nil
}
