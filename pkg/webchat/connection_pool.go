package webchat

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// ConnectionPool manages websocket connections for a conversation.
// It centralizes broadcasting, error handling, and idle detection so higher-level
// router logic stays small.
type ConnectionPool struct {
	convID      string
	mu          sync.Mutex
	conns       map[*websocket.Conn]struct{}
	idleTimer   *time.Timer
	idleTimeout time.Duration
	onIdle      func()
}

func NewConnectionPool(convID string, idleTimeout time.Duration, onIdle func()) *ConnectionPool {
	return &ConnectionPool{
		convID:      convID,
		conns:       map[*websocket.Conn]struct{}{},
		idleTimeout: idleTimeout,
		onIdle:      onIdle,
	}
}

func (cp *ConnectionPool) Add(conn *websocket.Conn) {
	if cp == nil || conn == nil {
		return
	}
	cp.mu.Lock()
	cp.conns[conn] = struct{}{}
	cp.stopIdleTimerLocked()
	cp.mu.Unlock()
}

func (cp *ConnectionPool) Remove(conn *websocket.Conn) {
	if cp == nil || conn == nil {
		_ = closeConn(conn)
		return
	}
	cp.mu.Lock()
	delete(cp.conns, conn)
	cp.scheduleIdleTimerLocked()
	cp.mu.Unlock()
	_ = closeConn(conn)
}

func (cp *ConnectionPool) Broadcast(data []byte) {
	if cp == nil || len(data) == 0 {
		return
	}
	cp.mu.Lock()
	for conn := range cp.conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Warn().Err(err).Str("component", "webchat").Str("conv_id", cp.convID).Msg("ws broadcast failed, dropping connection")
			delete(cp.conns, conn)
			_ = closeConn(conn)
		}
	}
	cp.scheduleIdleTimerLocked()
	cp.mu.Unlock()
}

func (cp *ConnectionPool) SendToOne(conn *websocket.Conn, data []byte) {
	if cp == nil || conn == nil || len(data) == 0 {
		return
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if _, ok := cp.conns[conn]; !ok {
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Warn().Err(err).Str("component", "webchat").Str("conv_id", cp.convID).Msg("ws send failed, dropping connection")
		delete(cp.conns, conn)
		_ = closeConn(conn)
	}
}

func (cp *ConnectionPool) Count() int {
	if cp == nil {
		return 0
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return len(cp.conns)
}

func (cp *ConnectionPool) IsEmpty() bool {
	return cp.Count() == 0
}

func (cp *ConnectionPool) CloseAll() {
	if cp == nil {
		return
	}
	cp.mu.Lock()
	for conn := range cp.conns {
		_ = closeConn(conn)
		delete(cp.conns, conn)
	}
	cp.stopIdleTimerLocked()
	cp.mu.Unlock()
}

func (cp *ConnectionPool) CancelIdleTimer() {
	if cp == nil {
		return
	}
	cp.mu.Lock()
	cp.stopIdleTimerLocked()
	cp.mu.Unlock()
}

func (cp *ConnectionPool) stopIdleTimerLocked() {
	if cp.idleTimer != nil {
		cp.idleTimer.Stop()
		cp.idleTimer = nil
	}
}

func (cp *ConnectionPool) scheduleIdleTimerLocked() {
	if len(cp.conns) != 0 || cp.idleTimeout <= 0 || cp.onIdle == nil {
		cp.stopIdleTimerLocked()
		return
	}
	cp.stopIdleTimerLocked()
	cp.idleTimer = time.AfterFunc(cp.idleTimeout, cp.triggerIdle)
}

func (cp *ConnectionPool) triggerIdle() {
	if cp == nil {
		return
	}
	var callback func()
	cp.mu.Lock()
	if len(cp.conns) == 0 {
		callback = cp.onIdle
	}
	cp.idleTimer = nil
	cp.mu.Unlock()
	if callback != nil {
		callback()
	}
}

func closeConn(conn *websocket.Conn) error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}
