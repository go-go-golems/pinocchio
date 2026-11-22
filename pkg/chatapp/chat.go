package chatapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

const (
	CommandStartInference = "ChatStartInference"
	CommandStopInference  = "ChatStopInference"

	EventUserMessageAccepted = "ChatUserMessageAccepted"

	EventChatRunStarted                  = "ChatRunStarted"
	EventChatRunFinished                 = "ChatRunFinished"
	EventChatRunStopped                  = "ChatRunStopped"
	EventChatRunFailed                   = "ChatRunFailed"
	EventChatProviderCallStarted         = "ChatProviderCallStarted"
	EventChatProviderCallMetadataUpdated = "ChatProviderCallMetadataUpdated"
	EventChatProviderCallFinished        = "ChatProviderCallFinished"
	EventChatTextSegmentStarted          = "ChatTextSegmentStarted"
	EventChatTextPatch                   = "ChatTextPatch"
	UIEventChatTextDelta                 = "ChatTextDelta"
	UIEventChatReasoningDelta            = "ChatReasoningDelta"
	UIEventChatToolArgumentsDelta        = "ChatToolArgumentsDelta"
	EventChatTextSegmentFinished         = "ChatTextSegmentFinished"
	EventChatReasoningSegmentStarted     = "ChatReasoningSegmentStarted"
	EventChatReasoningPatch              = "ChatReasoningPatch"
	EventChatReasoningSegmentFinished    = "ChatReasoningSegmentFinished"
	EventChatToolCallStarted             = "ChatToolCallStarted"
	EventChatToolArgumentsPatch          = "ChatToolArgumentsPatch"
	EventChatToolCallRequested           = "ChatToolCallRequested"
	EventChatToolExecutionStarted        = "ChatToolExecutionStarted"
	EventChatToolResultReady             = "ChatToolResultReady"
	EventChatToolCallFinished            = "ChatToolCallFinished"

	TimelineEntityChatMessage = "ChatMessage"
)

type Hooks struct {
	OnBackendEvent func(sessionID, eventName string, payload map[string]any)
}

type Option func(*Engine)

// MessageIDGenerator creates opaque root message IDs. Implementations must be
// safe for concurrent use when an Engine can handle commands concurrently.
// Generated roots must not occupy namespaces reserved for derived user, text,
// reasoning, or warning message entities; Engine validates this before publish.
type MessageIDGenerator func() (string, error)

type Engine struct {
	mu                   sync.Mutex
	messageIDGenerator   MessageIDGenerator
	active               map[sessionstream.SessionId]*activeRun
	pending              map[string]PromptRequest
	hooks                Hooks
	features             []ChatPlugin
	turnStore            chatstore.TurnStore
	streamPatchBatch     StreamPatchBatchConfig
	uiEventTransformers  []UIEventTransformer
	busyPolicy           BusyPolicy
	classifyError        func(error) string
	errorText            func(code string, err error) string
	unansweredToolResult string
	usageObserver        UsageObserver
	runTools             []RunToolsFunc
}

// RunScope identifies the run a RunToolsFunc prepares.
type RunScope struct {
	SessionID sessionstream.SessionId
	MessageID string
	// Publisher publishes into the session's event stream for this run.
	Publisher sessionstream.EventPublisher
}

// RunTools is the tool setup of one run: the registry the model sees and the
// executor that runs its calls. A nil Registry means a single model call
// without tools.
type RunTools struct {
	Registry geptools.ToolRegistry
	Executor geptools.ToolExecutor
}

// RunToolsFunc adjusts the tools of one run before it starts. It receives the
// runtime's registry and executor (or what earlier functions returned) and
// returns the ones to use, plus the run's context (e.g. with values the
// executor needs). It must not modify the registry it receives: the runtime's
// registry is shared between runs; clone it to add tools. An error fails the
// run with code "tools_error".
type RunToolsFunc func(ctx context.Context, run RunScope, tools RunTools) (context.Context, RunTools, error)

// WithRunTools adds functions that adjust every run's tools, applied in
// order. frontendtools.Manager.RunTools gives runs the page's browser tools.
func WithRunTools(fns ...RunToolsFunc) Option {
	return func(e *Engine) {
		for _, fn := range fns {
			if fn != nil {
				e.runTools = append(e.runTools, fn)
			}
		}
	}
}

// UsageObserver receives the token usage of every finished provider call of a
// run, before chatapp publishes ChatProviderCallFinished. geppetto calls event
// sinks inline, so an observer's effect (e.g. a session's usage total) is
// visible to the run's RoundGuard before the next model call.
type UsageObserver func(ctx context.Context, sid sessionstream.SessionId, usage *chatappv1.UsageInfo)

// WithUsageObserver sets the usage observer (see sessions.RecordUsage).
func WithUsageObserver(fn UsageObserver) Option {
	return func(e *Engine) {
		e.usageObserver = fn
	}
}

// BusyPolicy decides what happens when a prompt arrives for a session whose
// previous run has not ended.
type BusyPolicy int

const (
	// BusyReject refuses the new prompt with ErrRunActive. Nothing is published.
	BusyReject BusyPolicy = iota
	// BusyReplace cancels the running turn, waits for it to end, and starts the
	// new one.
	BusyReplace
)

// ErrRunActive is returned (through Service.SubmitPromptRequest) when a prompt
// arrives for a busy session under BusyReject.
var ErrRunActive = errors.New("chatapp: a run is already active in this session")

// ErrNoRuntime is returned when a prompt arrives without a runtime engine.
var ErrNoRuntime = errors.New("chatapp: the prompt request has no runtime engine")

// DefaultUnansweredToolResult is the error recorded for a tool call that had
// no result when its run ended (see WithUnansweredToolResult).
const DefaultUnansweredToolResult = "not run: the turn ended before this tool call finished"

// WithBusyPolicy sets the policy for prompts that arrive while a run is
// active. The default is BusyReject.
func WithBusyPolicy(p BusyPolicy) Option {
	return func(e *Engine) {
		e.busyPolicy = p
	}
}

// WithErrorClassifier replaces the function that maps a failed run's error to
// the code published in ChatRunFailed.code. Errors with a Code() string method
// and round-limit errors are classified before this function is called.
func WithErrorClassifier(fn func(error) string) Option {
	return func(e *Engine) {
		e.classifyError = fn
	}
}

// WithErrorText replaces the error text published in ChatRunFailed.error. The
// default publishes err.Error(); applications that expose events to untrusted
// clients should replace provider text here.
func WithErrorText(fn func(code string, err error) string) Option {
	return func(e *Engine) {
		e.errorText = fn
	}
}

// WithUnansweredToolResult sets the error text recorded in the history for a
// tool call that had no result when its run ended. Closing such calls keeps the
// next provider request valid. The default is DefaultUnansweredToolResult.
func WithUnansweredToolResult(text string) Option {
	return func(e *Engine) {
		e.unansweredToolResult = text
	}
}

type activeRun struct {
	messageID string
	cancel    context.CancelFunc
	done      chan struct{}
}

// WithMessageIDGenerator overrides root message ID generation. It is primarily
// useful for deterministic tests and deployments with an external identity
// policy. Invalid or nil generators fail when a start command is handled.
func WithMessageIDGenerator(generate MessageIDGenerator) Option {
	return func(e *Engine) {
		e.messageIDGenerator = generate
	}
}

// StreamPatchBatchConfig controls bounded batching of append-only text,
// reasoning, and tool-argument deltas before they become canonical patch events.
// A non-positive interval disables batching. The first patch in each logical
// stream is always published immediately.
type StreamPatchBatchConfig struct {
	Interval time.Duration
}

// WithStreamPatchBatching enables fixed-window stream patch batching.
func WithStreamPatchBatching(interval time.Duration) Option {
	return func(e *Engine) {
		e.streamPatchBatch.Interval = interval
	}
}

// WithTextPatchBatching is retained as an alias for callers that configured the
// original text-only batcher. It now batches all supported stream patch classes.
func WithTextPatchBatching(interval time.Duration) Option {
	return WithStreamPatchBatching(interval)
}

// WithUIEventTransformers configures transformations applied to projected UI
// events before they are published to a fanout. Transformers never alter the
// canonical backend event or timeline projection.
func WithUIEventTransformers(transformers ...UIEventTransformer) Option {
	return func(e *Engine) {
		e.uiEventTransformers = append([]UIEventTransformer(nil), transformers...)
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
		active:               map[sessionstream.SessionId]*activeRun{},
		pending:              map[string]PromptRequest{},
		messageIDGenerator:   defaultMessageIDGenerator,
		busyPolicy:           BusyReject,
		unansweredToolResult: DefaultUnansweredToolResult,
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
		reg.RegisterEvent(EventUserMessageAccepted, &chatappv1.ChatUserMessageAccepted{}),
		reg.RegisterEvent(EventChatRunStarted, &chatappv1.ChatRunStarted{}),
		reg.RegisterEvent(EventChatRunFinished, &chatappv1.ChatRunFinished{}),
		reg.RegisterEvent(EventChatRunStopped, &chatappv1.ChatRunStopped{}),
		reg.RegisterEvent(EventChatRunFailed, &chatappv1.ChatRunFailed{}),
		reg.RegisterEvent(EventChatProviderCallStarted, &chatappv1.ChatProviderCallStarted{}),
		reg.RegisterEvent(EventChatProviderCallMetadataUpdated, &chatappv1.ChatProviderCallMetadataUpdated{}),
		reg.RegisterEvent(EventChatProviderCallFinished, &chatappv1.ChatProviderCallFinished{}),
		reg.RegisterEvent(EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{}),
		reg.RegisterEvent(EventChatTextPatch, &chatappv1.ChatTextPatch{}),
		reg.RegisterEvent(EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{}),
		reg.RegisterUIEvent(EventUserMessageAccepted, &chatappv1.ChatUserMessageAccepted{}),
		reg.RegisterUIEvent(EventChatRunStarted, &chatappv1.ChatRunStarted{}),
		reg.RegisterUIEvent(EventChatRunFinished, &chatappv1.ChatRunFinished{}),
		reg.RegisterUIEvent(EventChatRunStopped, &chatappv1.ChatRunStopped{}),
		reg.RegisterUIEvent(EventChatRunFailed, &chatappv1.ChatRunFailed{}),
		reg.RegisterUIEvent(EventChatProviderCallStarted, &chatappv1.ChatProviderCallStarted{}),
		reg.RegisterUIEvent(EventChatProviderCallMetadataUpdated, &chatappv1.ChatProviderCallMetadataUpdated{}),
		reg.RegisterUIEvent(EventChatProviderCallFinished, &chatappv1.ChatProviderCallFinished{}),
		reg.RegisterUIEvent(EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{}),
		reg.RegisterUIEvent(EventChatTextPatch, &chatappv1.ChatTextPatch{}),
		reg.RegisterUIEvent(UIEventChatTextDelta, &chatappv1.ChatTextDelta{}),
		reg.RegisterUIEvent(UIEventChatReasoningDelta, &chatappv1.ChatReasoningDelta{}),
		reg.RegisterUIEvent(UIEventChatToolArgumentsDelta, &chatappv1.ChatToolArgumentsDelta{}),
		reg.RegisterUIEvent(EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{}),
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

// Running reports whether the session has an active run.
func (e *Engine) Running(sid sessionstream.SessionId) bool {
	return e.currentRun(sid) != nil
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

// reserveRun makes run the session's active run. Under BusyReject it refuses
// when another run is active; under BusyReplace it returns the run it replaced,
// which the caller cancels and waits for.
func (e *Engine) reserveRun(sid sessionstream.SessionId, run *activeRun) (*activeRun, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	prev := e.active[sid]
	if prev != nil && e.busyPolicy == BusyReject {
		return nil, ErrRunActive
	}
	e.active[sid] = run
	return prev, nil
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
