package webchat

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/stretchr/testify/require"
)

func TestNewChatServiceFromConversation_NilSafe(t *testing.T) {
	require.Nil(t, NewChatServiceFromConversation(nil))
}

func TestChatService_ResolveAndSubmitDelegateToConversationService(t *testing.T) {
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(context.Context, infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		return infruntime.ComposedRuntime{
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

	handle, err := chat.ResolveAndEnsureConversation(context.Background(), ConversationRuntimeRequest{})
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

type testRunHandle struct {
	done <-chan error
}

func (h testRunHandle) Wait() error {
	if h.done == nil {
		return nil
	}
	return <-h.done
}

type blockingRunner struct {
	mu     sync.Mutex
	starts []StartRequest
	done   chan error
}

func (r *blockingRunner) Start(_ context.Context, req StartRequest) (StartResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts = append(r.starts, req)
	return StartResult{
		Response: map[string]any{
			"status":          "started",
			"idempotency_key": req.Payload.(LLMLoopStartPayload).IdempotencyKey,
			"conv_id":         req.ConvID,
			"session_id":      req.SessionID,
		},
		Handle: testRunHandle{done: r.done},
		RunID:  "run-" + req.ConvID,
	}, nil
}

func TestChatService_StartPromptWithRunner_PreservesQueueing(t *testing.T) {
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(context.Context, infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		return infruntime.ComposedRuntime{
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
	runner := &blockingRunner{done: make(chan error, 2)}

	first, err := chat.StartPromptWithRunner(context.Background(), runner, StartPromptWithRunnerInput{
		Runtime: ConversationRuntimeRequest{ConvID: "conv-queued", RuntimeKey: "default"},
		Payload: LLMLoopStartPayload{Prompt: "one"},
	})
	require.NoError(t, err)
	require.Equal(t, 200, first.HTTPStatus)
	require.Equal(t, "started", first.Response["status"])

	second, err := chat.StartPromptWithRunner(context.Background(), runner, StartPromptWithRunnerInput{
		Runtime:        ConversationRuntimeRequest{ConvID: "conv-queued", RuntimeKey: "default"},
		IdempotencyKey: "k-2",
		Payload:        LLMLoopStartPayload{Prompt: "two"},
	})
	require.NoError(t, err)
	require.Equal(t, 202, second.HTTPStatus)
	require.Equal(t, "queued", second.Response["status"])

	runner.done <- nil
	require.Eventually(t, func() bool {
		runner.mu.Lock()
		defer runner.mu.Unlock()
		return len(runner.starts) == 2
	}, 2*time.Second, 20*time.Millisecond)
}
