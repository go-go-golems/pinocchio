package webchat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWSPublisher_PublishJSON_ConversationNotFound(t *testing.T) {
	cm := &ConvManager{conns: map[string]*Conversation{}}
	publisher := NewWSPublisher(cm)

	err := publisher.PublishJSON(context.Background(), "missing-conv", map[string]any{"type": "x"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrConversationNotFound))
}

func TestWSPublisher_PublishJSON_NoConnectionPool(t *testing.T) {
	cm := &ConvManager{
		conns: map[string]*Conversation{
			"conv-1": {ID: "conv-1"},
		},
	}
	publisher := NewWSPublisher(cm)

	err := publisher.PublishJSON(context.Background(), "conv-1", map[string]any{"type": "x"})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrConnectionPoolAbsent))
}

func TestWSPublisher_PublishJSON_SuccessfulFanout(t *testing.T) {
	pool := NewConnectionPool("conv-1", 0, nil)

	conn := newStubConn(false)
	pool.Add(conn)

	cm := &ConvManager{
		conns: map[string]*Conversation{
			"conv-1": {
				ID:   "conv-1",
				pool: pool,
			},
		},
	}
	publisher := NewWSPublisher(cm)

	err := publisher.PublishJSON(context.Background(), "conv-1", map[string]any{"type": "x", "payload": map[string]any{"ok": true}})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.writes > 0
	}, time.Second, 10*time.Millisecond)
}
