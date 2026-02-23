package webchat

import (
	"context"
	"fmt"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestConversationService_PersistenceWiringFromConfig(t *testing.T) {
	timelineStore := &stubTimelineStore{}
	turnStore := &stubTurnStore{}

	svc, err := NewConversationService(ConversationServiceConfig{
		BaseCtx:       context.Background(),
		ConvManager:   &ConvManager{},
		TimelineStore: timelineStore,
		TurnStore:     turnStore,
	})
	require.NoError(t, err)
	require.Same(t, timelineStore, svc.timelineStore)
	require.Same(t, turnStore, svc.turnStore)

	nextTimelineStore := &stubTimelineStore{}
	nextTurnStore := &stubTurnStore{}
	svc.SetTimelineStore(nextTimelineStore)
	svc.SetTurnStore(nextTurnStore)

	require.Same(t, nextTimelineStore, svc.timelineStore)
	require.Same(t, nextTurnStore, svc.turnStore)
}

type noopEngine struct{}

func (noopEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) { return t, nil }

type versionedEngine struct {
	id string
}

func (e *versionedEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	return t, nil
}

type noopSink struct{}

func (noopSink) PublishEvent(events.Event) error { return nil }

func TestConversationService_SubmitPromptQueuesWhenConversationBusy(t *testing.T) {
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
	conv := &Conversation{
		ID:                 "conv-1",
		SessionID:          "session-1",
		Sess:               &session.Session{SessionID: "session-1"},
		baseCtx:            context.Background(),
		RuntimeKey:         "default",
		RuntimeFingerprint: "fp-default",
		activeRequestKey:   "inflight",
		requests:           map[string]*chatRequestRecord{},
	}
	cm.conns["conv-1"] = conv

	svc, err := NewConversationService(ConversationServiceConfig{
		BaseCtx:     context.Background(),
		ConvManager: cm,
	})
	require.NoError(t, err)

	resp, err := svc.SubmitPrompt(context.Background(), SubmitPromptInput{
		ConvID:         "conv-1",
		RuntimeKey:     "default",
		Prompt:         "hello",
		IdempotencyKey: "k-1",
	})
	require.NoError(t, err)
	require.Equal(t, 202, resp.HTTPStatus)
	require.Equal(t, "queued", resp.Response["status"])
	require.Equal(t, "k-1", resp.Response["idempotency_key"])
	require.Len(t, conv.queue, 1)
}

func TestConversationService_ResolveAndEnsureConversation_DefaultsAndLifecycle(t *testing.T) {
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(context.Context, infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		return infruntime.ComposedRuntime{
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
	svc, err := NewConversationService(ConversationServiceConfig{
		BaseCtx:     context.Background(),
		ConvManager: cm,
	})
	require.NoError(t, err)

	handle, err := svc.ResolveAndEnsureConversation(context.Background(), ConversationRuntimeRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, handle.ConvID)
	_, parseErr := uuid.Parse(handle.ConvID)
	require.NoError(t, parseErr)
	require.Equal(t, "default", handle.RuntimeKey)
	require.Equal(t, "fp-default", handle.RuntimeFingerprint)
	require.Equal(t, "seed", handle.SeedSystemPrompt)
}

func TestConversationService_ResolveAndEnsureConversation_RebuildsOnProfileVersionChange(t *testing.T) {
	callCount := 0
	runtimeComposer := infruntime.RuntimeBuilderFunc(func(_ context.Context, req infruntime.ConversationRuntimeRequest) (infruntime.ComposedRuntime, error) {
		callCount++
		engineID := fmt.Sprintf("eng-v%d-call-%d", req.ProfileVersion, callCount)
		return infruntime.ComposedRuntime{
			Engine:             &versionedEngine{id: engineID},
			Sink:               noopSink{},
			RuntimeKey:         "default",
			RuntimeFingerprint: fmt.Sprintf("fp-v%d", req.ProfileVersion),
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

	handleV1, err := svc.ResolveAndEnsureConversation(context.Background(), ConversationRuntimeRequest{
		ConvID:          "conv-versioned",
		RuntimeKey:      "default",
		ProfileVersion:  1,
		ResolvedRuntime: nil,
	})
	require.NoError(t, err)
	require.Equal(t, "fp-v1", handleV1.RuntimeFingerprint)
	conv, ok := cm.GetConversation("conv-versioned")
	require.True(t, ok)
	require.NotNil(t, conv)
	engineV1 := conv.Eng
	require.NotNil(t, engineV1)

	handleV1Repeat, err := svc.ResolveAndEnsureConversation(context.Background(), ConversationRuntimeRequest{
		ConvID:         "conv-versioned",
		RuntimeKey:     "default",
		ProfileVersion: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "fp-v1", handleV1Repeat.RuntimeFingerprint)
	convRepeat, ok := cm.GetConversation("conv-versioned")
	require.True(t, ok)
	require.Same(t, engineV1, convRepeat.Eng, "same profile version should not rebuild engine")

	handleV2, err := svc.ResolveAndEnsureConversation(context.Background(), ConversationRuntimeRequest{
		ConvID:         "conv-versioned",
		RuntimeKey:     "default",
		ProfileVersion: 2,
	})
	require.NoError(t, err)
	require.Equal(t, "fp-v2", handleV2.RuntimeFingerprint)
	convV2, ok := cm.GetConversation("conv-versioned")
	require.True(t, ok)
	require.NotSame(t, engineV1, convV2.Eng, "new profile version should rebuild engine")
}

func TestConversationService_SubmitPromptRejectsMissingPrompt(t *testing.T) {
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

	resp, err := svc.SubmitPrompt(context.Background(), SubmitPromptInput{
		ConvID:     "conv-1",
		RuntimeKey: "default",
		Prompt:     "   ",
	})
	require.NoError(t, err)
	require.Equal(t, 400, resp.HTTPStatus)
	require.Equal(t, "error", resp.Response["status"])
	require.Equal(t, "missing prompt", resp.Response["error"])
}

func TestConversationService_AttachWebSocketValidatesArguments(t *testing.T) {
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

	err = svc.AttachWebSocket(context.Background(), "", nil, WebSocketAttachOptions{})
	require.ErrorContains(t, err, "missing convID")

	err = svc.AttachWebSocket(context.Background(), "conv-1", nil, WebSocketAttachOptions{})
	require.ErrorContains(t, err, "websocket connection is nil")
}
