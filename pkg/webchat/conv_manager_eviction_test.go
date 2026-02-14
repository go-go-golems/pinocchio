package webchat

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/require"
)

func TestConvManagerEvictIdleOnce(t *testing.T) {
	cm := NewConvManager(ConvManagerOptions{BaseCtx: context.Background()})
	cm.SetEvictionConfig(10*time.Second, time.Second)

	conv := &Conversation{
		ID:           "c1",
		lastActivity: time.Now().Add(-time.Hour),
		pool:         NewConnectionPool("c1", 0, nil),
	}

	cm.mu.Lock()
	cm.conns["c1"] = conv
	cm.mu.Unlock()

	evicted := cm.evictIdleOnce(time.Now())
	require.Equal(t, 1, evicted)

	_, ok := cm.GetConversation("c1")
	require.False(t, ok)
}

func TestConvManagerEvictIdleOnce_SkipsBusy(t *testing.T) {
	cm := NewConvManager(ConvManagerOptions{BaseCtx: context.Background()})
	cm.SetEvictionConfig(10*time.Second, time.Second)

	conv := &Conversation{
		ID:               "c1",
		activeRequestKey: "busy",
		lastActivity:     time.Now().Add(-time.Hour),
		pool:             NewConnectionPool("c1", 0, nil),
	}

	cm.mu.Lock()
	cm.conns["c1"] = conv
	cm.mu.Unlock()

	evicted := cm.evictIdleOnce(time.Now())
	require.Equal(t, 0, evicted)

	_, ok := cm.GetConversation("c1")
	require.True(t, ok)
}

type evictionStubSubscriber struct {
	mu         sync.Mutex
	ch         chan *message.Message
	closeCalls int
}

func (s *evictionStubSubscriber) Subscribe(_ context.Context, _ string) (<-chan *message.Message, error) {
	return s.ch, nil
}

func (s *evictionStubSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCalls++
	if s.ch != nil {
		close(s.ch)
		s.ch = nil
	}
	return nil
}

func (s *evictionStubSubscriber) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCalls
}

func TestCleanupConversation_ClosesSubscriberOnlyOnceWhenStreamOwnsClose(t *testing.T) {
	cm := NewConvManager(ConvManagerOptions{BaseCtx: context.Background()})
	sub := &evictionStubSubscriber{ch: make(chan *message.Message)}
	conv := &Conversation{
		ID:       "c1",
		sub:      sub,
		subClose: true,
		stream:   NewStreamCoordinator("c1", sub, nil, nil),
	}

	cm.cleanupConversation(conv)

	require.Equal(t, 1, sub.calls())
}
