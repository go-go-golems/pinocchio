package webchat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConvManagerEvictIdleOnce(t *testing.T) {
	cm := NewConvManager(ConvManagerOptions{})
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
	cm := NewConvManager(ConvManagerOptions{})
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
