package webchat

import (
	"context"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

// Conversation holds per-conversation state and streaming attachments.
type Conversation struct {
	ID      string
	RunID   string
	Sess    *session.Session
	Eng     engine.Engine
	Sink    *middleware.WatermillSink
	mu      sync.Mutex
	sub     message.Subscriber
	pool    *ConnectionPool
	stream  *StreamCoordinator
	baseCtx context.Context
}

// ConvManager stores all live conversations.
type ConvManager struct {
	mu    sync.Mutex
	conns map[string]*Conversation
}

// topicForConv computes the event topic for a conversation.
func topicForConv(convID string) string { return "chat:" + convID }

// getOrCreateConv creates a new conversation with engine and sink using the provided engineFactory.
func (r *Router) getOrCreateConv(convID string, buildEng func() (engine.Engine, *middleware.WatermillSink, message.Subscriber, error)) (*Conversation, error) {
	r.cm.mu.Lock()
	defer r.cm.mu.Unlock()
	if c, ok := r.cm.conns[convID]; ok {
		return c, nil
	}
	runID := uuid.NewString()
	conv := &Conversation{
		ID:      convID,
		RunID:   runID,
		baseCtx: r.baseCtx,
	}
	eng, sink, sub, err := buildEng()
	if err != nil {
		return nil, err
	}
	conv.Eng = eng
	conv.Sink = sink
	conv.sub = sub

	idleTimeout := time.Duration(r.idleTimeoutSec) * time.Second
	onIdle := func() {
		if conv.stream != nil {
			log.Info().Str("component", "webchat").Str("conv_id", conv.ID).Dur("idle_timeout", idleTimeout).Msg("idle timeout reached, stopping stream")
			conv.stream.Stop()
		}
	}
	conv.pool = NewConnectionPool(conv.ID, idleTimeout, onIdle)
	conv.stream = NewStreamCoordinator(
		conv.ID,
		sub,
		nil,
		func(e events.Event, _ StreamCursor, frame []byte) {
			run := e.Metadata().SessionID
			if run != "" && run != conv.RunID {
				return
			}
			if conv.pool != nil {
				conv.pool.Broadcast(frame)
			}
		},
	)

	conv.Sess = &session.Session{
		SessionID: runID,
		Builder: &session.ToolLoopEngineBuilder{
			Base:       eng,
			EventSinks: []events.EventSink{sink},
		},
		Turns: func() []*turns.Turn {
			seed := &turns.Turn{}
			_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, runID)
			return []*turns.Turn{seed}
		}(),
	}

	// Start streaming immediately; ConnectionPool idle logic will stop it when no clients are connected.
	if conv.stream != nil {
		ctx := conv.baseCtx
		if ctx == nil {
			ctx = context.Background()
		}
		if err := conv.stream.Start(ctx); err != nil {
			return nil, err
		}
	}

	r.cm.conns[convID] = conv
	return conv, nil
}

func (r *Router) addConn(conv *Conversation, c *websocket.Conn) {
	if conv == nil || c == nil {
		return
	}
	if conv.pool != nil {
		conv.pool.Add(c)
	}
	conv.mu.Lock()
	baseCtx := conv.baseCtx
	stream := conv.stream
	conv.mu.Unlock()
	if stream != nil && !stream.IsRunning() {
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		_ = stream.Start(baseCtx)
	}
}

func (r *Router) removeConn(conv *Conversation, c *websocket.Conn) {
	if conv == nil || c == nil {
		return
	}
	if conv.pool != nil {
		conv.pool.Remove(c)
		return
	}
	_ = c.Close()
}
