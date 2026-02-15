package webchat

import (
	"context"
	"testing"

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
