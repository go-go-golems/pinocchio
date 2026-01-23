package webchat

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

// Conversation holds per-conversation state and streaming attachments.
type Conversation struct {
	ID       string
	RunID    string
	Sess     *session.Session
	Eng      engine.Engine
	Sink     events.EventSink
	mu       sync.Mutex
	sub      message.Subscriber
	subClose bool
	pool     *ConnectionPool
	stream   *StreamCoordinator
	baseCtx  context.Context

	ProfileSlug  string
	EngConfigSig string
}

// ConvManager stores all live conversations.
type ConvManager struct {
	mu    sync.Mutex
	conns map[string]*Conversation
}

// topicForConv computes the event topic for a conversation.
func topicForConv(convID string) string { return "chat:" + convID }

// getOrCreateConv creates or reuses a conversation based on engine config signature changes.
// It centralizes engine/sink/subscriber composition by delegating to the Router EngineBuilder methods.
func (r *Router) getOrCreateConv(convID, profileSlug string, overrides map[string]any) (*Conversation, error) {
	if r == nil {
		return nil, errors.New("router is nil")
	}
	cfg, err := r.BuildConfig(profileSlug, overrides)
	if err != nil {
		return nil, err
	}
	newSig := cfg.Signature()

	r.cm.mu.Lock()
	defer r.cm.mu.Unlock()
	if c, ok := r.cm.conns[convID]; ok {
		if c.ProfileSlug != profileSlug || c.EngConfigSig != newSig {
			log.Info().
				Str("component", "webchat").
				Str("conv_id", convID).
				Str("old_profile", c.ProfileSlug).
				Str("new_profile", profileSlug).
				Msg("profile or engine config changed, rebuilding engine")

			eng, sink, err := r.BuildFromConfig(convID, cfg)
			if err != nil {
				return nil, err
			}
			sub, subClose, err := r.buildSubscriber(convID)
			if err != nil {
				return nil, err
			}

			// Replace stream/subscriber (avoid closing shared in-memory subscriber).
			if c.stream != nil {
				if c.subClose {
					c.stream.Close()
				} else {
					c.stream.Stop()
				}
			}

			c.Eng = eng
			c.Sink = sink
			c.sub = sub
			c.subClose = subClose
			c.ProfileSlug = profileSlug
			c.EngConfigSig = newSig

			c.stream = NewStreamCoordinator(
				c.ID,
				sub,
				nil,
				func(e events.Event, _ StreamCursor, frame []byte) {
					run := e.Metadata().SessionID
					if run != "" && run != c.RunID {
						return
					}
					if c.pool != nil {
						c.pool.Broadcast(frame)
					}
				},
			)

			if c.stream != nil {
				ctx := c.baseCtx
				if ctx == nil {
					ctx = context.Background()
				}
				if err := c.stream.Start(ctx); err != nil {
					return nil, err
				}
			}
		}
		return c, nil
	}
	runID := uuid.NewString()
	conv := &Conversation{
		ID:           convID,
		RunID:        runID,
		baseCtx:      r.baseCtx,
		ProfileSlug:  profileSlug,
		EngConfigSig: newSig,
	}
	eng, sink, err := r.BuildFromConfig(convID, cfg)
	if err != nil {
		return nil, err
	}
	sub, subClose, err := r.buildSubscriber(convID)
	if err != nil {
		return nil, err
	}
	conv.Eng = eng
	conv.Sink = sink
	conv.sub = sub
	conv.subClose = subClose

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
		Builder: &toolloop.EngineBuilder{
			Base:             eng,
			EventSinks:       []events.EventSink{sink},
			StepController:   r.stepCtrl,
			StepPauseTimeout: 30 * time.Second,
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
