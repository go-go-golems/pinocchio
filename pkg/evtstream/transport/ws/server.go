package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	frameTypeHello        = "hello"
	frameTypeSubscribed   = "subscribed"
	frameTypeUnsubscribed = "unsubscribed"
	frameTypeSnapshot     = "snapshot"
	frameTypeUIEvent      = "ui-event"
	frameTypeError        = "error"
	frameTypePing         = "ping"
	frameTypePong         = "pong"
)

// SnapshotProvider provides snapshot lookup for subscribe flows.
type SnapshotProvider interface {
	Snapshot(ctx context.Context, sid evtstream.SessionId) (evtstream.Snapshot, error)
}

// Hooks observes websocket lifecycle and payload sequencing for debugging and labs.
type Hooks struct {
	OnConnect      func(evtstream.ConnectionId)
	OnDisconnect   func(evtstream.ConnectionId)
	OnSubscribe    func(evtstream.ConnectionId, evtstream.SessionId, uint64)
	OnUnsubscribe  func(evtstream.ConnectionId, evtstream.SessionId)
	OnSnapshotSent func(evtstream.ConnectionId, evtstream.SessionId, evtstream.Snapshot)
	OnUIEventSent  func(evtstream.ConnectionId, evtstream.SessionId, uint64, evtstream.UIEvent)
	OnClientFrame  func(evtstream.ConnectionId, map[string]any)
}

// Option configures a websocket Server.
type Option func(*Server) error

// WithHooks installs optional lifecycle hooks.
func WithHooks(h Hooks) Option {
	return func(s *Server) error {
		s.hooks = h
		return nil
	}
}

// WithUpgrader overrides the default websocket upgrader.
func WithUpgrader(u websocket.Upgrader) Option {
	return func(s *Server) error {
		s.upgrader = u
		return nil
	}
}

// Server is the Phase 3 websocket transport. It is both an HTTP handler and an evtstream.UIFanout.
type Server struct {
	snapshots SnapshotProvider
	upgrader  websocket.Upgrader
	hooks     Hooks

	nextID uint64

	mu        sync.RWMutex
	conns     map[evtstream.ConnectionId]*connection
	bySession map[evtstream.SessionId]map[evtstream.ConnectionId]struct{}
}

type connection struct {
	id    evtstream.ConnectionId
	ws    *websocket.Conn
	send  chan []byte
	close sync.Once

	mu   sync.RWMutex
	subs map[evtstream.SessionId]subscription
}

type subscription struct {
	sinceOrdinal uint64
}

// ConnectionInfo describes the current transport-visible state of one connection.
type ConnectionInfo struct {
	ConnectionId  string   `json:"connectionId"`
	Subscriptions []string `json:"subscriptions"`
}

type clientFrame struct {
	Type         string `json:"type"`
	SessionID    string `json:"sessionId,omitempty"`
	SinceOrdinal string `json:"sinceOrdinal,omitempty"`
}

type envelope struct {
	Type         string `json:"type"`
	ConnectionID string `json:"connectionId,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
	SinceOrdinal string `json:"sinceOrdinal,omitempty"`
	Ordinal      string `json:"ordinal,omitempty"`
	Name         string `json:"name,omitempty"`
	Payload      any    `json:"payload,omitempty"`
	Entities     []any  `json:"entities,omitempty"`
	Error        string `json:"error,omitempty"`
}

var _ http.Handler = (*Server)(nil)
var _ evtstream.UIFanout = (*Server)(nil)

// NewServer builds a websocket transport server.
func NewServer(snapshots SnapshotProvider, opts ...Option) (*Server, error) {
	if snapshots == nil {
		return nil, fmt.Errorf("snapshot provider is nil")
	}
	server := &Server{
		snapshots: snapshots,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		conns:     map[evtstream.ConnectionId]*connection{},
		bySession: map[evtstream.SessionId]map[evtstream.ConnectionId]struct{}{},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(server); err != nil {
			return nil, err
		}
	}
	return server, nil
}

// ServeHTTP upgrades a connection and serves the websocket protocol.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	cid := evtstream.ConnectionId(fmt.Sprintf("conn-%d", atomic.AddUint64(&s.nextID, 1)))
	c := &connection{
		id:   cid,
		ws:   conn,
		send: make(chan []byte, 128),
		subs: map[evtstream.SessionId]subscription{},
	}
	s.mu.Lock()
	s.conns[cid] = c
	s.mu.Unlock()
	if s.hooks.OnConnect != nil {
		s.hooks.OnConnect(cid)
	}

	go s.writeLoop(c)
	_ = s.sendEnvelope(c, envelope{Type: frameTypeHello, ConnectionID: string(cid)})
	s.readLoop(r.Context(), c)
	s.closeConnection(c)
}

// PublishUI fans projected UI events out to subscribed websocket clients.
func (s *Server) PublishUI(_ context.Context, sid evtstream.SessionId, ord uint64, events []evtstream.UIEvent) error {
	if len(events) == 0 {
		return nil
	}
	targets := s.connectionsForSession(sid)
	for _, c := range targets {
		for _, event := range events {
			if err := s.sendEnvelope(c, envelope{
				Type:      frameTypeUIEvent,
				SessionID: string(sid),
				Ordinal:   fmt.Sprintf("%d", ord),
				Name:      event.Name,
				Payload:   encodeProtoJSON(event.Payload),
			}); err != nil {
				s.closeConnection(c)
				continue
			}
			if s.hooks.OnUIEventSent != nil {
				s.hooks.OnUIEventSent(c.id, sid, ord, cloneUIEvent(event))
			}
		}
	}
	return nil
}

// Connections returns a stable snapshot of current connections and subscriptions.
func (s *Server) Connections() []ConnectionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ConnectionInfo, 0, len(s.conns))
	for id, conn := range s.conns {
		conn.mu.RLock()
		subs := make([]string, 0, len(conn.subs))
		for sid := range conn.subs {
			subs = append(subs, string(sid))
		}
		conn.mu.RUnlock()
		sort.Strings(subs)
		out = append(out, ConnectionInfo{ConnectionId: string(id), Subscriptions: subs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ConnectionId < out[j].ConnectionId })
	return out
}

func (s *Server) readLoop(ctx context.Context, c *connection) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var frame clientFrame
		if err := c.ws.ReadJSON(&frame); err != nil {
			return
		}
		if s.hooks.OnClientFrame != nil {
			s.hooks.OnClientFrame(c.id, map[string]any{
				"type":         frame.Type,
				"sessionId":    frame.SessionID,
				"sinceOrdinal": frame.SinceOrdinal,
			})
		}
		if err := s.handleClientFrame(ctx, c, frame); err != nil {
			_ = s.sendEnvelope(c, envelope{Type: frameTypeError, Error: err.Error()})
		}
	}
}

func (s *Server) handleClientFrame(ctx context.Context, c *connection, frame clientFrame) error {
	switch frame.Type {
	case frameTypePing:
		return s.sendEnvelope(c, envelope{Type: frameTypePong})
	case "subscribe":
		sid := evtstream.SessionId(frame.SessionID)
		if sid == "" {
			return fmt.Errorf("subscribe missing session id")
		}
		since, err := parseUint(frame.SinceOrdinal)
		if err != nil {
			return fmt.Errorf("parse since ordinal: %w", err)
		}
		snap, err := s.snapshots.Snapshot(ctx, sid)
		if err != nil {
			return fmt.Errorf("load snapshot for %q: %w", sid, err)
		}
		if err := s.sendEnvelope(c, envelope{
			Type:      frameTypeSnapshot,
			SessionID: string(sid),
			Ordinal:   fmt.Sprintf("%d", snap.Ordinal),
			Entities:  encodeSnapshotEntities(snap.Entities),
		}); err != nil {
			return err
		}
		if s.hooks.OnSnapshotSent != nil {
			s.hooks.OnSnapshotSent(c.id, sid, cloneSnapshot(snap))
		}
		c.mu.Lock()
		c.subs[sid] = subscription{sinceOrdinal: since}
		c.mu.Unlock()
		s.mu.Lock()
		set := s.bySession[sid]
		if set == nil {
			set = map[evtstream.ConnectionId]struct{}{}
			s.bySession[sid] = set
		}
		set[c.id] = struct{}{}
		s.mu.Unlock()
		if s.hooks.OnSubscribe != nil {
			s.hooks.OnSubscribe(c.id, sid, since)
		}
		return s.sendEnvelope(c, envelope{Type: frameTypeSubscribed, SessionID: string(sid), SinceOrdinal: fmt.Sprintf("%d", since)})
	case "unsubscribe":
		sid := evtstream.SessionId(frame.SessionID)
		if sid == "" {
			return fmt.Errorf("unsubscribe missing session id")
		}
		s.removeSubscription(c, sid)
		if s.hooks.OnUnsubscribe != nil {
			s.hooks.OnUnsubscribe(c.id, sid)
		}
		return s.sendEnvelope(c, envelope{Type: frameTypeUnsubscribed, SessionID: string(sid)})
	default:
		return fmt.Errorf("unknown frame type %q", frame.Type)
	}
}

func (s *Server) writeLoop(c *connection) {
	for msg := range c.send {
		if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (s *Server) closeConnection(c *connection) {
	if c == nil {
		return
	}
	c.close.Do(func() {
		c.mu.Lock()
		subs := make([]evtstream.SessionId, 0, len(c.subs))
		for sid := range c.subs {
			subs = append(subs, sid)
		}
		c.subs = map[evtstream.SessionId]subscription{}
		c.mu.Unlock()

		s.mu.Lock()
		delete(s.conns, c.id)
		for _, sid := range subs {
			delete(s.bySession[sid], c.id)
			if len(s.bySession[sid]) == 0 {
				delete(s.bySession, sid)
			}
		}
		s.mu.Unlock()

		close(c.send)
		_ = c.ws.Close()
		if s.hooks.OnDisconnect != nil {
			s.hooks.OnDisconnect(c.id)
		}
	})
}

func (s *Server) connectionsForSession(sid evtstream.SessionId) []*connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set := s.bySession[sid]
	if len(set) == 0 {
		return nil
	}
	out := make([]*connection, 0, len(set))
	for cid := range set {
		if conn := s.conns[cid]; conn != nil {
			out = append(out, conn)
		}
	}
	return out
}

func (s *Server) removeSubscription(c *connection, sid evtstream.SessionId) {
	if c == nil || sid == "" {
		return
	}
	c.mu.Lock()
	delete(c.subs, sid)
	c.mu.Unlock()
	s.mu.Lock()
	delete(s.bySession[sid], c.id)
	if len(s.bySession[sid]) == 0 {
		delete(s.bySession, sid)
	}
	s.mu.Unlock()
}

func (s *Server) sendEnvelope(c *connection, env envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	select {
	case c.send <- body:
		return nil
	default:
		return fmt.Errorf("connection %s send buffer full", c.id)
	}
}

func encodeSnapshotEntities(in []evtstream.TimelineEntity) []any {
	if len(in) == 0 {
		return []any{}
	}
	out := make([]any, 0, len(in))
	for _, entity := range in {
		out = append(out, map[string]any{
			"kind":      entity.Kind,
			"id":        entity.Id,
			"tombstone": entity.Tombstone,
			"payload":   encodeProtoJSON(entity.Payload),
		})
	}
	return out
}

func encodeProtoJSON(msg proto.Message) any {
	if msg == nil {
		return nil
	}
	body, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(msg)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return string(body)
	}
	return out
}

func parseUint(raw string) (uint64, error) {
	if raw == "" {
		return 0, nil
	}
	var out uint64
	if _, err := fmt.Sscanf(raw, "%d", &out); err != nil {
		return 0, err
	}
	return out, nil
}

func cloneSnapshot(snap evtstream.Snapshot) evtstream.Snapshot {
	out := evtstream.Snapshot{SessionId: snap.SessionId, Ordinal: snap.Ordinal}
	out.Entities = make([]evtstream.TimelineEntity, 0, len(snap.Entities))
	for _, entity := range snap.Entities {
		cloned := entity
		if entity.Payload != nil {
			cloned.Payload = proto.Clone(entity.Payload)
		}
		out.Entities = append(out.Entities, cloned)
	}
	return out
}

func cloneUIEvent(ev evtstream.UIEvent) evtstream.UIEvent {
	out := ev
	if ev.Payload != nil {
		out.Payload = proto.Clone(ev.Payload)
	}
	return out
}
