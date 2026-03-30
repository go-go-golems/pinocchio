package main

import (
	"strings"

	"github.com/go-go-golems/geppetto/pkg/events"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
)

type agentModeSinkConfigInput struct {
	SanitizeYAML *bool `json:"sanitize_yaml,omitempty"`
}

func agentModeStructuredSinkConfigFromRuntime(runtime *infruntime.ProfileRuntime) (agentmode.StructuredSinkConfig, bool) {
	cfg := agentmode.DefaultStructuredSinkConfig()
	if runtime == nil {
		return cfg, false
	}

	found := false
	for _, mw := range runtime.Middlewares {
		if !strings.EqualFold(strings.TrimSpace(mw.Name), "agentmode") {
			continue
		}
		if mw.Enabled != nil && !*mw.Enabled {
			continue
		}

		found = true
		input := agentModeSinkConfigInput{}
		if err := decodeResolvedMiddlewareConfig(mw.Config, &input); err == nil && input.SanitizeYAML != nil {
			cfg.ParseOptions = cfg.ParseOptions.WithSanitizeYAML(*input.SanitizeYAML)
		}
	}
	return cfg, found
}

func newAgentModeStructuredSinkWrapper() webchat.EventSinkWrapper {
	return func(_ string, req infruntime.ConversationRuntimeRequest, sink events.EventSink) (events.EventSink, error) {
		cfg, ok := agentModeStructuredSinkConfigFromRuntime(req.ResolvedProfileRuntime)
		if !ok {
			return sink, nil
		}
		return agentmode.WrapStructuredSink(sink, cfg), nil
	}
}
