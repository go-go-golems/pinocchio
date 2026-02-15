package webchat

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/require"
)

func TestNewChatServiceFromConversation_NilSafe(t *testing.T) {
	require.Nil(t, NewChatServiceFromConversation(nil))
}

func TestChatService_ResolveAndSubmitDelegateToConversationService(t *testing.T) {
	runtimeComposer := RuntimeComposerFunc(func(context.Context, RuntimeComposeRequest) (RuntimeArtifacts, error) {
		return RuntimeArtifacts{
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

	svc, err := NewConversationService(ConversationServiceConfig{
		BaseCtx:     context.Background(),
		ConvManager: cm,
	})
	require.NoError(t, err)

	chat := NewChatServiceFromConversation(svc)
	require.NotNil(t, chat)

	handle, err := chat.ResolveAndEnsureConversation(context.Background(), AppConversationRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, handle.ConvID)

	resp, err := chat.SubmitPrompt(context.Background(), SubmitPromptInput{
		ConvID:         handle.ConvID,
		RuntimeKey:     "default",
		Prompt:         "   ",
		IdempotencyKey: "k-chat-1",
	})
	require.NoError(t, err)
	require.Equal(t, 400, resp.HTTPStatus)
	require.Equal(t, "error", resp.Response["status"])
}
