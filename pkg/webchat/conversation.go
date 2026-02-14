package webchat

import (
	"context"
	"errors"
	"strings"
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
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/turns"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// Conversation holds per-conversation state and streaming attachments.
type Conversation struct {
	ID        string
	SessionID string
	Sess      *session.Session
	Eng       engine.Engine
	Sink      events.EventSink
	mu        sync.Mutex
	sub       message.Subscriber
	subClose  bool
	pool      *ConnectionPool
	stream    *StreamCoordinator
	baseCtx   context.Context

	ProfileSlug  string
	EngConfigSig string

	// Server-side send serialization / queue semantics.
	// All fields below are guarded by mu.
	activeRequestKey string
	queue            []queuedChat
	requests         map[string]*chatRequestRecord

	lastActivity time.Time

	semBuf *semFrameBuffer

	// Durable "actual hydration" projection (optional; enabled when Router has a TimelineStore).
	timelineProj *TimelineProjector
}

// ConvManager stores all live conversations and centralizes lifecycle wiring.
type ConvManager struct {
	mu    sync.Mutex
	conns map[string]*Conversation

	baseCtx        context.Context
	idleTimeoutSec int
	stepCtrl       *toolloop.StepController

	timelineStore      chatstore.TimelineStore
	timelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)

	buildConfig     func(profileSlug string, overrides map[string]any) (EngineConfig, error)
	buildFromConfig func(convID string, cfg EngineConfig) (engine.Engine, events.EventSink, error)
	buildSubscriber func(convID string) (message.Subscriber, bool, error)

	evictIdle     time.Duration
	evictInterval time.Duration
	evictRunning  bool
}

// ConvManagerOptions configures the conversation manager dependencies.
type ConvManagerOptions struct {
	BaseCtx        context.Context
	IdleTimeoutSec int
	StepController *toolloop.StepController
	EvictIdle      time.Duration
	EvictInterval  time.Duration

	TimelineStore      chatstore.TimelineStore
	TimelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV1, version uint64)

	BuildConfig     func(profileSlug string, overrides map[string]any) (EngineConfig, error)
	BuildFromConfig func(convID string, cfg EngineConfig) (engine.Engine, events.EventSink, error)
	BuildSubscriber func(convID string) (message.Subscriber, bool, error)
}

func NewConvManager(opts ConvManagerOptions) *ConvManager {
	if opts.BaseCtx == nil {
		panic("webchat: NewConvManager requires non-nil BaseCtx")
	}
	return &ConvManager{
		conns:              map[string]*Conversation{},
		baseCtx:            opts.BaseCtx,
		idleTimeoutSec:     opts.IdleTimeoutSec,
		stepCtrl:           opts.StepController,
		timelineStore:      opts.TimelineStore,
		timelineUpsertHook: opts.TimelineUpsertHook,
		buildConfig:        opts.BuildConfig,
		buildFromConfig:    opts.BuildFromConfig,
		buildSubscriber:    opts.BuildSubscriber,
		evictIdle:          opts.EvictIdle,
		evictInterval:      opts.EvictInterval,
	}
}

func (cm *ConvManager) SetTimelineStore(store chatstore.TimelineStore) {
	if cm == nil {
		return
	}
	cm.mu.Lock()
	cm.timelineStore = store
	cm.mu.Unlock()
}

func (cm *ConvManager) SetIdleTimeoutSeconds(sec int) {
	if cm == nil {
		return
	}
	cm.mu.Lock()
	cm.idleTimeoutSec = sec
	cm.mu.Unlock()
}

// GetConversation retrieves a conversation by ID (thread-safe).
func (cm *ConvManager) GetConversation(convID string) (*Conversation, bool) {
	if cm == nil || convID == "" {
		return nil, false
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	conv, ok := cm.conns[convID]
	return conv, ok
}

func (c *Conversation) touchLocked(now time.Time) {
	if c == nil {
		return
	}
	c.lastActivity = now
}

// topicForConv computes the event topic for a conversation.
func topicForConv(convID string) string { return "chat:" + convID }

// GetOrCreate creates or reuses a conversation based on engine config signature changes.
// It centralizes engine/sink/subscriber composition through injected builder hooks.
func (cm *ConvManager) GetOrCreate(convID, profileSlug string, overrides map[string]any) (*Conversation, error) {
	if cm == nil {
		return nil, errors.New("conversation manager is nil")
	}
	if cm.buildConfig == nil || cm.buildFromConfig == nil || cm.buildSubscriber == nil {
		return nil, errors.New("conversation manager missing dependencies")
	}
	cfg, err := cm.buildConfig(profileSlug, overrides)
	if err != nil {
		return nil, err
	}
	newSig := cfg.Signature()
	now := time.Now()

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if c, ok := cm.conns[convID]; ok {
		c.mu.Lock()
		c.ensureQueueInitLocked()
		c.touchLocked(now)
		if c.semBuf == nil {
			c.semBuf = newSemFrameBuffer(1000)
		}
		if c.timelineProj == nil && cm.timelineStore != nil {
			hook := cm.timelineUpsertHook
			if hook != nil {
				c.timelineProj = NewTimelineProjector(c.ID, cm.timelineStore, hook(c))
			} else {
				c.timelineProj = NewTimelineProjector(c.ID, cm.timelineStore, nil)
			}
		}
		c.mu.Unlock()
		if c.ProfileSlug != profileSlug || c.EngConfigSig != newSig {
			log.Info().
				Str("component", "webchat").
				Str("conv_id", convID).
				Str("old_profile", c.ProfileSlug).
				Str("new_profile", profileSlug).
				Msg("profile or engine config changed, rebuilding engine")

			eng, sink, err := cm.buildFromConfig(convID, cfg)
			if err != nil {
				return nil, err
			}
			sub, subClose, err := cm.buildSubscriber(convID)
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
					eventSessionID := e.Metadata().SessionID
					if eventSessionID != "" && eventSessionID != c.SessionID {
						return
					}
					if c.pool != nil {
						c.pool.Broadcast(frame)
					}
					if c.semBuf != nil {
						c.semBuf.Add(frame)
					}
					if c.timelineProj != nil {
						_ = c.timelineProj.ApplySemFrame(c.baseCtx, frame)
					}
				},
			)

			if c.stream != nil {
				if err := c.stream.Start(c.baseCtx); err != nil {
					return nil, err
				}
			}
		}
		return c, nil
	}
	sessionID := uuid.NewString()
	conv := &Conversation{
		ID:           convID,
		SessionID:    sessionID,
		baseCtx:      cm.baseCtx,
		ProfileSlug:  profileSlug,
		EngConfigSig: newSig,
		requests:     map[string]*chatRequestRecord{},
		semBuf:       newSemFrameBuffer(1000),
		lastActivity: now,
	}
	if cm.timelineStore != nil {
		hook := cm.timelineUpsertHook
		if hook != nil {
			conv.timelineProj = NewTimelineProjector(conv.ID, cm.timelineStore, hook(conv))
		} else {
			conv.timelineProj = NewTimelineProjector(conv.ID, cm.timelineStore, nil)
		}
	}
	eng, sink, err := cm.buildFromConfig(convID, cfg)
	if err != nil {
		return nil, err
	}
	sub, subClose, err := cm.buildSubscriber(convID)
	if err != nil {
		return nil, err
	}
	conv.Eng = eng
	conv.Sink = sink
	conv.sub = sub
	conv.subClose = subClose

	idleTimeout := time.Duration(cm.idleTimeoutSec) * time.Second
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
			eventSessionID := e.Metadata().SessionID
			if eventSessionID != "" && eventSessionID != conv.SessionID {
				return
			}
			if conv.pool != nil {
				conv.pool.Broadcast(frame)
			}
			if conv.semBuf != nil {
				conv.semBuf.Add(frame)
			}
			if conv.timelineProj != nil {
				_ = conv.timelineProj.ApplySemFrame(conv.baseCtx, frame)
			}
		},
	)

	conv.Sess = &session.Session{
		SessionID: sessionID,
		Builder: &enginebuilder.Builder{
			Base:             eng,
			EventSinks:       []events.EventSink{sink},
			StepController:   cm.stepCtrl,
			StepPauseTimeout: 30 * time.Second,
		},
		Turns: func() []*turns.Turn {
			return []*turns.Turn{buildSeedTurn(sessionID, cfg.SystemPrompt)}
		}(),
	}

	// Start streaming immediately; ConnectionPool idle logic will stop it when no clients are connected.
	if conv.stream != nil {
		if err := conv.stream.Start(conv.baseCtx); err != nil {
			return nil, err
		}
	}

	cm.conns[convID] = conv
	return conv, nil
}

func buildSeedTurn(sessionID string, systemPrompt string) *turns.Turn {
	seed := &turns.Turn{}
	if strings.TrimSpace(systemPrompt) != "" {
		turns.AppendBlock(seed, turns.NewSystemTextBlock(systemPrompt))
	}
	_ = turns.KeyTurnMetaSessionID.Set(&seed.Metadata, sessionID)
	return seed
}

func (cm *ConvManager) AddConn(conv *Conversation, c *websocket.Conn) {
	if conv == nil || c == nil {
		return
	}
	conv.mu.Lock()
	conv.touchLocked(time.Now())
	conv.mu.Unlock()
	if conv.pool != nil {
		conv.pool.Add(c)
	}
	conv.mu.Lock()
	baseCtx := conv.baseCtx
	stream := conv.stream
	conv.mu.Unlock()
	if stream != nil && !stream.IsRunning() {
		_ = stream.Start(baseCtx)
	}
}

func (cm *ConvManager) RemoveConn(conv *Conversation, c *websocket.Conn) {
	if conv == nil || c == nil {
		return
	}
	conv.mu.Lock()
	conv.touchLocked(time.Now())
	conv.mu.Unlock()
	if conv.pool != nil {
		conv.pool.Remove(c)
		return
	}
	_ = c.Close()
}
