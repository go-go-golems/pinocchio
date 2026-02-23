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
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	"github.com/go-go-golems/geppetto/pkg/turns"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
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

	RuntimeKey         string
	RuntimeFingerprint string
	SeedSystemPrompt   string
	AllowedTools       []string

	// Server-side send serialization / queue semantics.
	// All fields below are guarded by mu.
	activeRequestKey string
	queue            []queuedChat
	requests         map[string]*chatRequestRecord

	lastActivity time.Time
	createdAt    time.Time

	semBuf *semFrameBuffer

	// Durable "actual hydration" projection (optional; enabled when Router has a TimelineStore).
	timelineProj *TimelineProjector
	// Highest projected timeline version observed for this conversation.
	lastSeenVersion uint64
}

// ConvManager stores all live conversations and centralizes lifecycle wiring.
type ConvManager struct {
	mu    sync.Mutex
	conns map[string]*Conversation

	baseCtx        context.Context
	idleTimeoutSec int
	stepCtrl       *toolloop.StepController

	timelineStore      chatstore.TimelineStore
	timelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64)

	runtimeComposer infruntime.RuntimeComposer
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
	TimelineUpsertHook func(*Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64)

	RuntimeComposer infruntime.RuntimeComposer
	BuildSubscriber func(convID string) (message.Subscriber, bool, error)
}

func NewConvManager(opts ConvManagerOptions) *ConvManager {
	if opts.BaseCtx == nil {
		panic("webchat: NewConvManager requires non-nil BaseCtx")
	}
	RegisterDefaultTimelineHandlers()
	return &ConvManager{
		conns:              map[string]*Conversation{},
		baseCtx:            opts.BaseCtx,
		idleTimeoutSec:     opts.IdleTimeoutSec,
		stepCtrl:           opts.StepController,
		timelineStore:      opts.TimelineStore,
		timelineUpsertHook: opts.TimelineUpsertHook,
		runtimeComposer:    opts.RuntimeComposer,
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

func (cm *ConvManager) SetRuntimeComposer(composer infruntime.RuntimeComposer) {
	if cm == nil || composer == nil {
		return
	}
	cm.mu.Lock()
	cm.runtimeComposer = composer
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

func (cm *ConvManager) persistConversationIndex(conv *Conversation, status string, lastError string) {
	if cm == nil {
		return
	}
	cm.mu.Lock()
	store := cm.timelineStore
	cm.mu.Unlock()
	cm.persistConversationIndexToStore(store, conv, status, lastError)
}

func (cm *ConvManager) persistConversationIndexToStore(store chatstore.TimelineStore, conv *Conversation, status string, lastError string) {
	if store == nil || conv == nil {
		return
	}
	record := buildConversationRecord(conv, status, lastError)
	if record.ConvID == "" {
		return
	}
	persistCtx := conv.baseCtx
	if persistCtx == nil {
		persistCtx = context.Background()
	}
	persistCtx = context.WithoutCancel(persistCtx)
	if err := store.UpsertConversation(persistCtx, record); err != nil {
		log.Warn().Err(err).Str("component", "webchat").Str("conv_id", conv.ID).Msg("persist conversation index failed")
	}
}

func buildConversationRecord(conv *Conversation, status string, lastError string) chatstore.ConversationRecord {
	now := time.Now()
	conv.mu.Lock()
	defer conv.mu.Unlock()

	createdAtMs := int64(0)
	if !conv.createdAt.IsZero() {
		createdAtMs = conv.createdAt.UnixMilli()
	}
	lastActivityMs := int64(0)
	if !conv.lastActivity.IsZero() {
		lastActivityMs = conv.lastActivity.UnixMilli()
	}
	if lastActivityMs == 0 {
		lastActivityMs = now.UnixMilli()
	}
	if createdAtMs == 0 {
		createdAtMs = lastActivityMs
	}
	if strings.TrimSpace(status) == "" {
		status = "active"
	}
	return chatstore.ConversationRecord{
		ConvID:          conv.ID,
		SessionID:       conv.SessionID,
		RuntimeKey:      conv.RuntimeKey,
		CreatedAtMs:     createdAtMs,
		LastActivityMs:  lastActivityMs,
		LastSeenVersion: conv.lastSeenVersion,
		HasTimeline:     conv.timelineProj != nil,
		Status:          strings.TrimSpace(status),
		LastError:       strings.TrimSpace(lastError),
	}
}

func (cm *ConvManager) timelineProjectorUpsertHook(conv *Conversation) func(entity *timelinepb.TimelineEntityV2, version uint64) {
	var downstream func(entity *timelinepb.TimelineEntityV2, version uint64)
	if cm != nil && cm.timelineUpsertHook != nil {
		downstream = cm.timelineUpsertHook(conv)
	}
	return func(entity *timelinepb.TimelineEntityV2, version uint64) {
		if conv != nil {
			conv.mu.Lock()
			if version > conv.lastSeenVersion {
				conv.lastSeenVersion = version
			}
			conv.mu.Unlock()
		}
		if downstream != nil {
			downstream(entity, version)
		}
	}
}

// topicForConv computes the event topic for a conversation.
func topicForConv(convID string) string { return "chat:" + convID }

// GetOrCreate creates or reuses a conversation based on runtime fingerprint changes.
func (cm *ConvManager) GetOrCreate(
	convID, runtimeKey string,
	overrides map[string]any,
	resolvedRuntime *gepprofiles.RuntimeSpec,
	profileVersion uint64,
) (*Conversation, error) {
	if cm == nil {
		return nil, errors.New("conversation manager is nil")
	}
	if cm.runtimeComposer == nil || cm.buildSubscriber == nil {
		return nil, errors.New("conversation manager missing dependencies")
	}
	req := infruntime.RuntimeComposeRequest{
		ConvID:          convID,
		RuntimeKey:      runtimeKey,
		ProfileVersion:  profileVersion,
		ResolvedRuntime: resolvedRuntime,
		Overrides:       overrides,
	}
	runtime, err := cm.runtimeComposer.Compose(cm.baseCtx, req)
	if err != nil {
		return nil, err
	}
	if runtime.Engine == nil {
		return nil, errors.New("runtime composer returned nil engine")
	}
	if runtime.Sink == nil {
		return nil, errors.New("runtime composer returned nil sink")
	}
	if strings.TrimSpace(runtime.RuntimeKey) == "" {
		runtime.RuntimeKey = runtimeKey
	}
	if strings.TrimSpace(runtime.RuntimeFingerprint) == "" {
		runtime.RuntimeFingerprint = runtime.RuntimeKey
	}
	now := time.Now()

	cm.mu.Lock()
	defer cm.mu.Unlock()
	timelineStore := cm.timelineStore
	if c, ok := cm.conns[convID]; ok {
		c.mu.Lock()
		c.ensureQueueInitLocked()
		c.touchLocked(now)
		if c.semBuf == nil {
			c.semBuf = newSemFrameBuffer(1000)
		}
		if c.timelineProj == nil && cm.timelineStore != nil {
			c.timelineProj = NewTimelineProjector(c.ID, cm.timelineStore, cm.timelineProjectorUpsertHook(c))
		}
		c.mu.Unlock()
		if c.RuntimeFingerprint != runtime.RuntimeFingerprint {
			log.Info().
				Str("component", "webchat").
				Str("conv_id", convID).
				Str("old_runtime_key", c.RuntimeKey).
				Str("new_runtime_key", runtime.RuntimeKey).
				Msg("runtime config changed, rebuilding engine")

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

			c.Eng = runtime.Engine
			c.Sink = runtime.Sink
			c.sub = sub
			c.subClose = subClose
			c.RuntimeKey = runtime.RuntimeKey
			c.RuntimeFingerprint = runtime.RuntimeFingerprint
			c.SeedSystemPrompt = runtime.SeedSystemPrompt
			c.AllowedTools = append([]string(nil), runtime.AllowedTools...)

			c.stream = NewStreamCoordinator(
				c.ID,
				sub,
				nil,
				func(e events.Event, _ StreamCursor, frame []byte) {
					if e != nil {
						eventSessionID := e.Metadata().SessionID
						if eventSessionID != "" && eventSessionID != c.SessionID {
							return
						}
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
		cm.persistConversationIndexToStore(timelineStore, c, "active", "")
		return c, nil
	}
	sessionID := uuid.NewString()
	conv := &Conversation{
		ID:                 convID,
		SessionID:          sessionID,
		baseCtx:            cm.baseCtx,
		RuntimeKey:         runtime.RuntimeKey,
		RuntimeFingerprint: runtime.RuntimeFingerprint,
		SeedSystemPrompt:   runtime.SeedSystemPrompt,
		AllowedTools:       append([]string(nil), runtime.AllowedTools...),
		requests:           map[string]*chatRequestRecord{},
		semBuf:             newSemFrameBuffer(1000),
		lastActivity:       now,
		createdAt:          now,
	}
	if timelineStore != nil {
		conv.timelineProj = NewTimelineProjector(conv.ID, timelineStore, cm.timelineProjectorUpsertHook(conv))
	}
	sub, subClose, err := cm.buildSubscriber(convID)
	if err != nil {
		return nil, err
	}
	conv.Eng = runtime.Engine
	conv.Sink = runtime.Sink
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
			if e != nil {
				eventSessionID := e.Metadata().SessionID
				if eventSessionID != "" && eventSessionID != conv.SessionID {
					return
				}
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
			Base:             runtime.Engine,
			EventSinks:       []events.EventSink{runtime.Sink},
			StepController:   cm.stepCtrl,
			StepPauseTimeout: 30 * time.Second,
		},
		Turns: func() []*turns.Turn {
			return []*turns.Turn{buildSeedTurn(sessionID, runtime.SeedSystemPrompt)}
		}(),
	}

	// Start streaming immediately; ConnectionPool idle logic will stop it when no clients are connected.
	if conv.stream != nil {
		if err := conv.stream.Start(conv.baseCtx); err != nil {
			return nil, err
		}
	}

	cm.conns[convID] = conv
	cm.persistConversationIndexToStore(timelineStore, conv, "active", "")
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
	cm.persistConversationIndex(conv, "active", "")
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
		cm.persistConversationIndex(conv, "active", "")
		return
	}
	_ = c.Close()
	cm.persistConversationIndex(conv, "active", "")
}
