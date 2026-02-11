package webchat

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubConn struct {
	mu       sync.Mutex
	writes   int
	blockCh  chan struct{}
	closedCh chan struct{}
}

func newStubConn(blockWrites bool) *stubConn {
	blockCh := make(chan struct{})
	if !blockWrites {
		close(blockCh)
	}
	return &stubConn{blockCh: blockCh, closedCh: make(chan struct{})}
}

func (s *stubConn) WriteMessage(_ int, _ []byte) error {
	select {
	case <-s.closedCh:
		return errors.New("closed")
	case <-s.blockCh:
	}
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	return nil
}

func (s *stubConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.closedCh:
		return nil
	default:
		close(s.closedCh)
		return nil
	}
}

func (s *stubConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

func TestConnectionPoolDropsOnFullBuffer(t *testing.T) {
	pool := NewConnectionPool("c1", 0, nil)
	pool.sendBuffer = 1
	pool.writeTimeout = 0

	conn := newStubConn(true)
	pool.Add(conn)

	pool.Broadcast([]byte("one"))
	pool.Broadcast([]byte("two"))
	pool.Broadcast([]byte("three"))

	require.Eventually(t, func() bool {
		return pool.Count() == 0
	}, time.Second, 10*time.Millisecond)
}
