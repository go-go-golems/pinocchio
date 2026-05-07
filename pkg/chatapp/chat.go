package chatapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

const (
	CommandStartInference = "ChatStartInference"
	CommandStopInference  = "ChatStopInference"

	EventUserMessageAccepted = "ChatUserMessageAccepted"
	EventInferenceStarted    = "ChatInferenceStarted"
	EventTokensDelta         = "ChatTokensDelta"
	EventInferenceFinished   = "ChatInferenceFinished"
	EventInferenceStopped    = "ChatInferenceStopped"

	UIMessageAccepted = "ChatMessageAccepted"
	UIMessageStarted  = "ChatMessageStarted"
	UIMessageAppended = "ChatMessageAppended"
	UIMessageFinished = "ChatMessageFinished"
	UIMessageStopped  = "ChatMessageStopped"

	TimelineEntityChatMessage = "ChatMessage"
)

type Hooks struct {
	OnBackendEvent func(sessionID, eventName string, payload map[string]any)
}

type Option func(*Engine)

type Engine struct {
	mu         sync.Mutex
	nextID     int
	active     map[sessionstream.SessionId]*activeRun
	pending    map[string]PromptRequest
	chunkDelay time.Duration
	hooks      Hooks
	features   []ChatPlugin
	turnStore  chatstore.TurnStore
}

type activeRun struct {
	messageID string
	cancel    context.CancelFunc
	done      chan struct{}
}

func WithChunkDelay(delay time.Duration) Option {
	return func(e *Engine) {
		e.chunkDelay = delay
	}
}

func WithHooks(h Hooks) Option {
	return func(e *Engine) {
		e.hooks = h
	}
}

func WithTurnStore(ts chatstore.TurnStore) Option {
	return func(e *Engine) {
		e.turnStore = ts
	}
}

func NewEngine(opts ...Option) *Engine {
	engine := &Engine{
		active:     map[sessionstream.SessionId]*activeRun{},
		pending:    map[string]PromptRequest{},
		chunkDelay: 20 * time.Millisecond,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}
	return engine
}

func RegisterSchemas(reg *sessionstream.SchemaRegistry, features ...ChatPlugin) error {
	for _, err := range []error{
		reg.RegisterCommand(CommandStartInference, &chatappv1.StartInferenceCommand{}),
		reg.RegisterCommand(CommandStopInference, &chatappv1.StopInferenceCommand{}),
		reg.RegisterEvent(EventUserMessageAccepted, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterEvent(EventInferenceStarted, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterEvent(EventTokensDelta, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterEvent(EventInferenceFinished, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterEvent(EventInferenceStopped, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterUIEvent(UIMessageAccepted, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterUIEvent(UIMessageStarted, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterUIEvent(UIMessageAppended, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterUIEvent(UIMessageFinished, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterUIEvent(UIMessageStopped, &chatappv1.ChatMessageUpdate{}),
		reg.RegisterTimelineEntity(TimelineEntityChatMessage, &chatappv1.ChatMessageEntity{}),
	} {
		if err != nil {
			return err
		}
	}
	for _, feature := range features {
		if feature == nil {
			continue
		}
		if err := feature.RegisterSchemas(reg); err != nil {
			return err
		}
	}
	return nil
}

func Install(hub *sessionstream.Hub, engine *Engine) error {
	if hub == nil {
		return fmt.Errorf("hub is nil")
	}
	if engine == nil {
		engine = NewEngine()
	}
	if err := hub.RegisterCommand(CommandStartInference, engine.handleStartInference); err != nil {
		return err
	}
	if err := hub.RegisterCommand(CommandStopInference, engine.handleStopInference); err != nil {
		return err
	}
	if err := hub.RegisterUIProjection(sessionstream.UIProjectionFunc(engine.uiProjection)); err != nil {
		return err
	}
	if err := hub.RegisterTimelineProjection(sessionstream.TimelineProjectionFunc(engine.timelineProjection)); err != nil {
		return err
	}
	return nil
}

func (e *Engine) WaitIdle(ctx context.Context, sid sessionstream.SessionId) error {
	for {
		run := e.currentRun(sid)
		if run == nil {
			return nil
		}
		select {
		case <-run.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (e *Engine) nextMessageID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextID++
	return fmt.Sprintf("chat-msg-%d", e.nextID)
}

func (e *Engine) swapRun(sid sessionstream.SessionId, run *activeRun) *activeRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	prev := e.active[sid]
	e.active[sid] = run
	return prev
}

func (e *Engine) currentRun(sid sessionstream.SessionId) *activeRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active[sid]
}

func (e *Engine) clearRun(sid sessionstream.SessionId, messageID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.active[sid]
	if current != nil && current.messageID == messageID {
		delete(e.active, sid)
	}
}

func (e *Engine) setPendingRequest(requestID string, req PromptRequest) {
	if e == nil || requestID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pending[requestID] = req
}

func (e *Engine) takePendingRequest(requestID string) PromptRequest {
	if e == nil || requestID == "" {
		return PromptRequest{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	req := e.pending[requestID]
	delete(e.pending, requestID)
	return req
}

func (e *Engine) clearPendingRequest(requestID string) {
	if e == nil || requestID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pending, requestID)
}
