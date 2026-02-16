package webchat

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/stretchr/testify/require"
)

func TestNewStreamHub_ValidatesRequiredDependencies(t *testing.T) {
	_, err := NewStreamHub(StreamHubConfig{})
	require.ErrorContains(t, err, "base context is nil")

	_, err = NewStreamHub(StreamHubConfig{BaseCtx: context.Background()})
	require.ErrorContains(t, err, "conv manager is nil")
}

func TestStreamHub_ResolveAndEnsureConversation_Defaults(t *testing.T) {
	runtimeComposer := infruntime.RuntimeComposerFunc(func(context.Context, infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
		return infruntime.RuntimeArtifacts{
			Engine:             noopEngine{},
			Sink:               noopSink{},
			RuntimeKey:         "default",
			RuntimeFingerprint: "fp-default",
			SeedSystemPrompt:   "seed",
		}, nil
	})
	cm := NewConvManager(ConvManagerOptions{
		BaseCtx:         context.Background(),
		RuntimeComposer: runtimeComposer,
		BuildSubscriber: func(string) (message.Subscriber, bool, error) { return nil, false, nil },
	})
	hub, err := NewStreamHub(StreamHubConfig{
		BaseCtx:     context.Background(),
		ConvManager: cm,
	})
	require.NoError(t, err)

	handle, err := hub.ResolveAndEnsureConversation(context.Background(), AppConversationRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, handle.ConvID)
	require.Equal(t, "default", handle.RuntimeKey)
	require.Equal(t, "fp-default", handle.RuntimeFingerprint)
	require.Equal(t, "seed", handle.SeedSystemPrompt)
}

func TestStreamHub_AttachWebSocketValidatesArguments(t *testing.T) {
	runtimeComposer := infruntime.RuntimeComposerFunc(func(context.Context, infruntime.RuntimeComposeRequest) (infruntime.RuntimeArtifacts, error) {
		return infruntime.RuntimeArtifacts{
			Engine:             noopEngine{},
			Sink:               noopSink{},
			RuntimeKey:         "default",
			RuntimeFingerprint: "fp-default",
		}, nil
	})
	cm := NewConvManager(ConvManagerOptions{
		BaseCtx:         context.Background(),
		RuntimeComposer: runtimeComposer,
		BuildSubscriber: func(string) (message.Subscriber, bool, error) { return nil, false, nil },
	})
	hub, err := NewStreamHub(StreamHubConfig{
		BaseCtx:     context.Background(),
		ConvManager: cm,
	})
	require.NoError(t, err)

	err = hub.AttachWebSocket(context.Background(), "", nil, WebSocketAttachOptions{})
	require.ErrorContains(t, err, "missing convID")

	err = hub.AttachWebSocket(context.Background(), "conv-1", nil, WebSocketAttachOptions{})
	require.ErrorContains(t, err, "websocket connection is nil")
}
