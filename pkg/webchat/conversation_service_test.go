package webchat

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
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

type noopSink struct{}

func (noopSink) PublishEvent(events.Event) error { return nil }

func TestConversationService_SubmitPromptQueuesWhenConversationBusy(t *testing.T) {
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
