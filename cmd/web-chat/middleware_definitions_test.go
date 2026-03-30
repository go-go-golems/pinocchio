package main

import (
	"context"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/inference/middlewarecfg"
	"github.com/go-go-golems/geppetto/pkg/turns"
	agentmode "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
	"github.com/stretchr/testify/require"
)

func TestAgentModeMiddlewareDefinition_DefaultModeMatchesWebChatCatalog(t *testing.T) {
	def := newAgentModeMiddlewareDefinition()
	svc := agentmode.NewStaticService([]*agentmode.AgentMode{
		{Name: defaultWebChatAgentMode, Prompt: "You are a financial analyst."},
	})

	mw, err := def.Build(context.Background(), middlewarecfg.BuildDeps{
		Values: map[string]any{
			dependencyAgentModeServiceKey: svc,
		},
	}, nil)
	require.NoError(t, err)

	handler := mw(func(ctx context.Context, turn *turns.Turn) (*turns.Turn, error) {
		return turn, nil
	})

	turn := &turns.Turn{ID: "turn-1"}
	res, err := handler(context.Background(), turn)
	require.NoError(t, err)
	require.NotNil(t, res)

	modeName, ok, err := turns.KeyAgentMode.Get(res.Data)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, defaultWebChatAgentMode, modeName)

	require.Len(t, res.Blocks, 1)
	require.Equal(t, turns.RoleUser, res.Blocks[0].Role)
	text, _ := res.Blocks[0].Payload[turns.PayloadKeyText].(string)
	require.Contains(t, text, "<currentMode>")
	require.Contains(t, text, "financial analyst")
}
