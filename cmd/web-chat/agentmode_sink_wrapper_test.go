package main

import (
	"testing"

	"github.com/go-go-golems/geppetto/pkg/events"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type collectingSink struct {
	list []events.Event
}

func (c *collectingSink) PublishEvent(ev events.Event) error {
	c.list = append(c.list, ev)
	return nil
}

func collectFinalText(list []events.Event) string {
	for i := len(list) - 1; i >= 0; i-- {
		if final, ok := list[i].(*events.EventFinal); ok {
			return final.Text
		}
	}
	return ""
}

func TestAgentModeStructuredSinkConfigFromRuntime_Defaults(t *testing.T) {
	cfg, ok := agentModeStructuredSinkConfigFromRuntime(&infruntime.ProfileRuntime{
		Middlewares: []infruntime.MiddlewareUse{{Name: "agentmode"}},
	})

	require.True(t, ok)
	require.True(t, cfg.ParseOptions.SanitizeEnabled())
}

func TestAgentModeStructuredSinkConfigFromRuntime_RespectsDisable(t *testing.T) {
	disabled := false
	cfg, ok := agentModeStructuredSinkConfigFromRuntime(&infruntime.ProfileRuntime{
		Middlewares: []infruntime.MiddlewareUse{
			{Name: "agentmode", Enabled: &disabled},
		},
	})

	require.False(t, ok)
	require.True(t, cfg.ParseOptions.SanitizeEnabled())
}

func TestAgentModeStructuredSinkConfigFromRuntime_UsesSanitizeOverride(t *testing.T) {
	cfg, ok := agentModeStructuredSinkConfigFromRuntime(&infruntime.ProfileRuntime{
		Middlewares: []infruntime.MiddlewareUse{
			{Name: "agentmode", Config: map[string]any{"sanitize_yaml": false}},
		},
	})

	require.True(t, ok)
	require.False(t, cfg.ParseOptions.SanitizeEnabled())
}

func TestNewAgentModeStructuredSinkWrapper_FiltersStructuredPayload(t *testing.T) {
	wrapper := newAgentModeStructuredSinkWrapper()
	downstream := &collectingSink{}
	req := infruntime.ConversationRuntimeRequest{
		ResolvedProfileRuntime: &infruntime.ProfileRuntime{
			Middlewares: []infruntime.MiddlewareUse{{Name: "agentmode"}},
		},
	}

	sink, err := wrapper("conv-1", req, downstream)
	require.NoError(t, err)
	require.NotSame(t, downstream, sink)

	meta := events.EventMetadata{
		ID:          uuid.New(),
		SessionID:   "sess-1",
		InferenceID: "inf-1",
		TurnID:      "turn-1",
	}
	text := "hello <" + agentmode.ModeSwitchTagPackage + ":" + agentmode.ModeSwitchTagType + ":" + agentmode.ModeSwitchTagVersion + ">\n```yaml\nmode_switch:\n  analysis: ok\n```\n</" + agentmode.ModeSwitchTagPackage + ":" + agentmode.ModeSwitchTagType + ":" + agentmode.ModeSwitchTagVersion + ">"
	require.NoError(t, sink.PublishEvent(events.NewPartialCompletionEvent(meta, text, text)))
	require.NoError(t, sink.PublishEvent(events.NewFinalEvent(meta, text)))

	final := collectFinalText(downstream.list)
	require.Equal(t, "hello ", final)
	require.NotContains(t, final, "agent_mode_switch")
}
