package webchat

import (
	"context"
	"sync"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/engineprofiles"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/stretchr/testify/require"

	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
)

type recordingTurnStore struct {
	mu        sync.Mutex
	saveCount int
}

func (s *recordingTurnStore) Save(context.Context, string, string, string, string, int64, string, chatstore.TurnSaveOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCount++
	return nil
}

func (s *recordingTurnStore) List(context.Context, chatstore.TurnQuery) ([]chatstore.TurnSnapshot, error) {
	return nil, nil
}

func (s *recordingTurnStore) Close() error { return nil }

type stubMessagePublisher struct{}

func (stubMessagePublisher) Publish(topic string, messages ...*message.Message) error { return nil }
func (stubMessagePublisher) Close() error                                             { return nil }

func TestLLMLoopRunner_StartFiltersRegisteredToolsAndPersistsTurns(t *testing.T) {
	allowedTool, err := geptools.NewToolFromFunc("allowed_tool", "allowed", func() (string, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	blockedTool, err := geptools.NewToolFromFunc("blocked_tool", "blocked", func() (string, error) {
		return "nope", nil
	})
	require.NoError(t, err)

	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		return infruntime.ComposedRuntime{
			Engine:             noopEngine{},
			Sink:               noopSink{},
			RuntimeKey:         req.ProfileKey,
			RuntimeFingerprint: "fp-tools",
			SeedSystemPrompt:   "seed",
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
	_, startReq, err := svc.PrepareRunnerStart(context.Background(), PrepareRunnerStartInput{
		Runtime: ConversationRuntimeRequest{
			ConvID:     "conv-tools",
			RuntimeKey: "default",
			ResolvedRuntime: &gepprofiles.RuntimeSpec{
				Tools: []string{"allowed_tool"},
			},
		},
		Payload: LLMLoopStartPayload{
			Prompt:         "hello",
			IdempotencyKey: "k-tools",
		},
	})
	require.NoError(t, err)

	turnStore := &recordingTurnStore{}
	runner := NewLLMLoopRunner(LLMLoopRunnerConfig{
		BaseCtx:      context.Background(),
		ConvManager:  cm,
		TurnStore:    turnStore,
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

	result, err := runner.Start(context.Background(), startReq)
	require.NoError(t, err)
	require.NotNil(t, result.Handle)
	require.NoError(t, result.Handle.Wait())

	conv, ok := cm.GetConversation("conv-tools")
	require.True(t, ok)
	state, err := cm.ensureLLMState(conv)
	require.NoError(t, err)
	builder, ok := state.session.Builder.(*enginebuilder.Builder)
	require.True(t, ok)
	tools := builder.Registry.ListTools()
	require.Len(t, tools, 1)
	require.Equal(t, "allowed_tool", tools[0].Name)

	turnStore.mu.Lock()
	saveCount := turnStore.saveCount
	turnStore.mu.Unlock()
	require.Greater(t, saveCount, 0)
}
