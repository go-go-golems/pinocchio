package webchat

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	defaultSendBuffer   = 64
	defaultWriteTimeout = 5 * time.Second
)

type wsConn interface {
	WriteMessage(messageType int, data []byte) error
	Close() error
	SetWriteDeadline(t time.Time) error
}

type poolClient struct {
	conn      wsConn
	send      chan []byte
	closeOnce sync.Once
}

func (c *poolClient) TrySend(data []byte) bool {
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

func (c *poolClient) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.send)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

// ConnectionPool manages websocket connections for a conversation.
// It centralizes broadcasting, error handling, and idle detection so higher-level
// router logic stays small.
type ConnectionPool struct {
	convID       string
	mu           sync.Mutex
	conns        map[wsConn]*poolClient
	idleTimer    *time.Timer
	idleTimeout  time.Duration
	onIdle       func()
	sendBuffer   int
	writeTimeout time.Duration
}

func NewConnectionPool(convID string, idleTimeout time.Duration, onIdle func()) *ConnectionPool {
	return &ConnectionPool{
		convID:       convID,
		conns:        map[wsConn]*poolClient{},
		idleTimeout:  idleTimeout,
		onIdle:       onIdle,
		sendBuffer:   defaultSendBuffer,
		writeTimeout: defaultWriteTimeout,
	}
}

func (cp *ConnectionPool) Add(conn wsConn) {
	if cp == nil || conn == nil {
		return
	}
	cp.mu.Lock()
	if _, ok := cp.conns[conn]; ok {
		cp.mu.Unlock()
		return
	}
	client := &poolClient{conn: conn, send: make(chan []byte, cp.sendBuffer)}
	cp.conns[conn] = client
	cp.stopIdleTimerLocked()
	cp.mu.Unlock()

	go cp.writer(client)
}

func (cp *ConnectionPool) Remove(conn wsConn) {
	if cp == nil || conn == nil {
		return
	}
	cp.mu.Lock()
	client := cp.conns[conn]
	delete(cp.conns, conn)
	cp.scheduleIdleTimerLocked()
	cp.mu.Unlock()

	if client != nil {
		client.Close()
	}
}

func (cp *ConnectionPool) Broadcast(data []byte) {
	if cp == nil || len(data) == 0 {
		return
	}
	cp.mu.Lock()
	clients := make([]*poolClient, 0, len(cp.conns))
	for _, client := range cp.conns {
		clients = append(clients, client)
	}
	cp.mu.Unlock()

	for _, client := range clients {
		if client == nil {
			continue
		}
		if !client.TrySend(data) {
			log.Warn().Str("component", "webchat").Str("conv_id", cp.convID).Msg("ws send buffer full, dropping connection")
			cp.dropClient(client)
		}
	}
}

func (cp *ConnectionPool) SendToOne(conn wsConn, data []byte) {
	if cp == nil || conn == nil || len(data) == 0 {
		return
	}
	cp.mu.Lock()
	client := cp.conns[conn]
	cp.mu.Unlock()
	if client == nil {
		return
	}
	if !client.TrySend(data) {
		log.Warn().Str("component", "webchat").Str("conv_id", cp.convID).Msg("ws send buffer full, dropping connection")
		cp.dropClient(client)
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
	clients := make([]*poolClient, 0, len(cp.conns))
	for _, client := range cp.conns {
		clients = append(clients, client)
	}
	cp.conns = map[wsConn]*poolClient{}
	cp.stopIdleTimerLocked()
	cp.mu.Unlock()

	for _, client := range clients {
		if client != nil {
			client.Close()
		}
	}
}

func (cp *ConnectionPool) CancelIdleTimer() {
	if cp == nil {
		return
	}
	cp.mu.Lock()
	cp.stopIdleTimerLocked()
	cp.mu.Unlock()
}

func (cp *ConnectionPool) writer(client *poolClient) {
	if cp == nil || client == nil {
		return
	}
	for msg := range client.send {
		if client.conn == nil {
			continue
		}
		if cp.writeTimeout > 0 {
			_ = client.conn.SetWriteDeadline(time.Now().Add(cp.writeTimeout))
		}
		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Warn().Err(err).Str("component", "webchat").Str("conv_id", cp.convID).Msg("ws write failed, dropping connection")
			cp.dropClient(client)
			return
		}
	}
}

func (cp *ConnectionPool) dropClient(client *poolClient) {
	if cp == nil || client == nil {
		return
	}
	cp.mu.Lock()
	if _, ok := cp.conns[client.conn]; ok {
		delete(cp.conns, client.conn)
		cp.scheduleIdleTimerLocked()
	}
	cp.mu.Unlock()
	client.Close()
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
