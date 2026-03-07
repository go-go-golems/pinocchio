package webchat

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/stretchr/testify/require"

	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
)

type stubMessagePublisher struct{}

func (stubMessagePublisher) Publish(topic string, messages ...*message.Message) error { return nil }
func (stubMessagePublisher) Close() error                                             { return nil }

func TestLLMLoopRunner_StartFiltersRegisteredToolsByAllowedTools(t *testing.T) {
	allowedTool, err := geptools.NewToolFromFunc("allowed_tool", "allowed", func() (string, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	blockedTool, err := geptools.NewToolFromFunc("blocked_tool", "blocked", func() (string, error) {
		return "nope", nil
	})
	require.NoError(t, err)

	runner := NewLLMLoopRunner(LLMLoopRunnerConfig{
		BaseCtx:      context.Background(),
		SEMPublisher: stubMessagePublisher{},
		ToolFactories: map[string]infruntime.ToolRegistrar{
			"allowed_tool": func(reg geptools.ToolRegistry) error {
				return reg.RegisterTool("allowed_tool", *allowedTool)
			},
			"blocked_tool": func(reg geptools.ToolRegistry) error {
				return reg.RegisterTool("blocked_tool", *blockedTool)
			},
		},
	})

	conv := &Conversation{
		ID:           "conv-tools",
		SessionID:    "session-tools",
		Sess:         &session.Session{SessionID: "session-tools"},
		Eng:          noopEngine{},
		baseCtx:      context.Background(),
		Sink:         noopSink{},
		AllowedTools: []string{"allowed_tool"},
	}

	_, err = runner.Start(context.Background(), StartRequest{
		Conversation: conv,
		ConvID:       conv.ID,
		SessionID:    conv.SessionID,
		Sink:         conv.Sink,
		Payload: LLMLoopStartPayload{
			Prompt:         "hello",
			IdempotencyKey: "k-tools",
		},
	})
	require.NoError(t, err)

	builder, ok := conv.Sess.Builder.(*enginebuilder.Builder)
	require.True(t, ok)
	require.NotNil(t, builder.Registry)

	tools := builder.Registry.ListTools()
	require.Len(t, tools, 1)
	require.Equal(t, "allowed_tool", tools[0].Name)
}
