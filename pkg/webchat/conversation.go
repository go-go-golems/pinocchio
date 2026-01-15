package webchat

import (
	"context"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
)

// Conversation holds per-conversation state and streaming attachments.
type Conversation struct {
	ID        string
	RunID     string
	runSeq    int
	State     *conversation.ConversationState
	stateMu   sync.RWMutex
	Eng       engine.Engine
	Sink      *middleware.WatermillSink
	running   bool
	cancel    context.CancelFunc
	mu        sync.Mutex
	conns     map[*websocket.Conn]bool
	connsMu   sync.RWMutex
	sub       message.Subscriber
	stopRead  context.CancelFunc
	reading   bool
	idleTimer *time.Timer
}

// ConvManager stores all live conversations.
type ConvManager struct {
	mu    sync.Mutex
	conns map[string]*Conversation
}

// topicForConv computes the event topic for a conversation.
func topicForConv(convID string) string { return "chat:" + convID }

// startReader subscribes to the per-conversation topic and forwards events to websocket clients.
func (r *Router) startReader(conv *Conversation) error {
	if conv.reading {
		return nil
	}
	log.Info().Str("conv_id", conv.ID).Str("topic", topicForConv(conv.ID)).Msg("starting conversation reader")
	readCtx, readCancel := context.WithCancel(context.Background())
	conv.stopRead = readCancel
	ch, err := conv.sub.Subscribe(readCtx, topicForConv(conv.ID))
	if err != nil {
		readCancel()
		conv.stopRead = nil
		return err
	}
	conv.reading = true
	go func() {
		for msg := range ch {
			e, err := events.NewEventFromJson(msg.Payload)
			if err != nil {
				log.Warn().Err(err).Str("component", "ws_reader").Msg("failed to decode event json")
				msg.Ack()
				continue
			}
			runID := e.Metadata().RunID
			if runID != "" && runID != conv.RunID {
				msg.Ack()
				continue
			}
			r.convertAndBroadcast(conv, e)
			msg.Ack()
		}
		conv.mu.Lock()
		conv.reading = false
		conv.stopRead = nil
		conv.mu.Unlock()
		log.Info().Str("conv_id", conv.ID).Msg("conversation reader stopped")
	}()
	return nil
}

func (r *Router) convertAndBroadcast(conv *Conversation, e events.Event) {
	send := func(b []byte) {
		conv.connsMu.RLock()
		for c := range conv.conns {
			_ = c.WriteMessage(websocket.TextMessage, b)
		}
		conv.connsMu.RUnlock()
	}
	if frames := SemanticEventsFromEvent(e); frames != nil {
		for _, b := range frames {
			send(b)
		}
	}
}

// getOrCreateConv creates a new conversation with engine and sink using the provided engineFactory.
func (r *Router) getOrCreateConv(convID string, buildEng func() (engine.Engine, *middleware.WatermillSink, message.Subscriber, error)) (*Conversation, error) {
	r.cm.mu.Lock()
	defer r.cm.mu.Unlock()
	if c, ok := r.cm.conns[convID]; ok {
		return c, nil
	}
	conv := &Conversation{ID: convID, RunID: uuid.NewString(), conns: map[*websocket.Conn]bool{}}
	eng, sink, sub, err := buildEng()
	if err != nil {
		return nil, err
	}
	conv.Eng = eng
	conv.Sink = sink
	conv.sub = sub
	conv.State = conversation.NewConversationState(conv.RunID)
	if err := r.startReader(conv); err != nil {
		return nil, err
	}
	r.cm.conns[convID] = conv
	return conv, nil
}

func (c *Conversation) snapshotForPrompt(prompt string) (*turns.Turn, error) {
	temp := conversation.NewConversationState(c.RunID)
	c.stateMu.RLock()
	if c.State != nil {
		temp.ID = c.State.ID
		temp.RunID = c.State.RunID
		temp.Blocks = append([]turns.Block(nil), c.State.Blocks...)
		temp.Data = c.State.Data.Clone()
		temp.Metadata = c.State.Metadata.Clone()
		temp.Version = c.State.Version
	}
	c.stateMu.RUnlock()

	if prompt != "" {
		if err := temp.Apply(conversation.MutateAppendUserText(prompt)); err != nil {
			return nil, err
		}
	}
	cfg := conversation.DefaultSnapshotConfig()
	return temp.Snapshot(cfg)
}

func (c *Conversation) updateStateFromTurn(t *turns.Turn) {
	if t == nil {
		return
	}
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if c.State == nil {
		c.State = conversation.NewConversationState(t.RunID)
	}
	c.State.Blocks = filterSystemPromptBlocks(t.Blocks)
	c.State.Data = t.Data.Clone()
	c.State.Metadata = t.Metadata.Clone()
	if t.RunID != "" {
		c.State.RunID = t.RunID
	}
}

func filterSystemPromptBlocks(blocks []turns.Block) []turns.Block {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]turns.Block, 0, len(blocks))
	for _, b := range blocks {
		if isSystemPromptBlock(b) {
			continue
		}
		out = append(out, b)
	}
	return out
}

func isSystemPromptBlock(b turns.Block) bool {
	if b.Kind != turns.BlockKindSystem {
		return false
	}
	val, ok, err := turns.KeyBlockMetaMiddleware.Get(b.Metadata)
	if err != nil || !ok {
		return false
	}
	return val == "systemprompt"
}

func (r *Router) addConn(conv *Conversation, c *websocket.Conn) {
	conv.connsMu.Lock()
	conv.conns[c] = true
	conv.connsMu.Unlock()
	conv.mu.Lock()
	if conv.idleTimer != nil {
		conv.idleTimer.Stop()
		conv.idleTimer = nil
	}
	wasReading := conv.reading
	conv.mu.Unlock()
	if !wasReading && r.usesRedis {
		_ = r.startReader(conv)
	}
}

func (r *Router) removeConn(conv *Conversation, c *websocket.Conn) {
	conv.connsMu.Lock()
	delete(conv.conns, c)
	conv.connsMu.Unlock()
	_ = c.Close()
	if r.idleTimeoutSec <= 0 {
		return
	}
	conv.connsMu.RLock()
	empty := len(conv.conns) == 0
	conv.connsMu.RUnlock()
	if !empty {
		return
	}
	conv.mu.Lock()
	if conv.idleTimer == nil {
		d := time.Duration(r.idleTimeoutSec) * time.Second
		conv.idleTimer = time.AfterFunc(d, func() {
			conv.mu.Lock()
			defer conv.mu.Unlock()
			conv.connsMu.RLock()
			isEmpty := len(conv.conns) == 0
			conv.connsMu.RUnlock()
			if isEmpty && conv.stopRead != nil {
				conv.stopRead()
				conv.stopRead = nil
				conv.reading = false
			}
		})
	}
	conv.mu.Unlock()
}
