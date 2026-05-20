package ui

import (
	"context"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"github.com/stretchr/testify/require"
)

func TestChatAppBackendCarriesSnapshotHistoryAcrossTurns(t *testing.T) {
	runner, err := chatapp.NewRunner(chatapp.RunnerOptions{})
	require.NoError(t, err)
	defer func() { _ = runner.Close() }()

	seed := &turns.Turn{}
	turns.AppendBlock(seed, turns.NewSystemTextBlock("system seed"))
	backend, err := NewChatAppBackend(runner.Service, sessionstream.SessionId("chatapp-backend-history"), &infruntime.ComposedRuntime{Engine: recordingTurnEngine{}}, seed)
	require.NoError(t, err)

	cmd, err := backend.Start(context.Background(), "first")
	require.NoError(t, err)
	require.NotNil(t, cmd)
	_ = cmd()

	cmd, err = backend.Start(context.Background(), "second")
	require.NoError(t, err)
	_ = cmd()

	backend.mu.Lock()
	current := backend.currentTurn.Clone()
	backend.mu.Unlock()

	require.Len(t, current.Blocks, 5)
	require.Equal(t, turns.RoleSystem, current.Blocks[0].Role)
	require.Equal(t, "system seed", current.Blocks[0].Payload[turns.PayloadKeyText])
	require.Equal(t, turns.RoleUser, current.Blocks[1].Role)
	require.Equal(t, "first", current.Blocks[1].Payload[turns.PayloadKeyText])
	require.Equal(t, turns.RoleAssistant, current.Blocks[2].Role)
	require.Equal(t, "seen: first", current.Blocks[2].Payload[turns.PayloadKeyText])
	require.Equal(t, turns.RoleUser, current.Blocks[3].Role)
	require.Equal(t, "second", current.Blocks[3].Payload[turns.PayloadKeyText])
	require.Equal(t, turns.RoleAssistant, current.Blocks[4].Role)
	require.Equal(t, "seen: second", current.Blocks[4].Payload[turns.PayloadKeyText])
}

type recordingTurnEngine struct{}

func (recordingTurnEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	lastUser := ""
	for i := len(t.Blocks) - 1; i >= 0; i-- {
		if t.Blocks[i].Role == turns.RoleUser {
			lastUser, _ = t.Blocks[i].Payload[turns.PayloadKeyText].(string)
			break
		}
	}
	turns.AppendBlock(t, turns.NewAssistantTextBlock("seen: "+lastUser))
	return t, nil
}
