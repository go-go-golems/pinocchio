package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/evtstream"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"google.golang.org/protobuf/types/known/structpb"
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
	active     map[evtstream.SessionId]*activeRun
	pending    map[evtstream.SessionId]PromptRequest
	chunkDelay time.Duration
	hooks      Hooks
}

type activeRun struct {
	messageID string
	cancel    context.CancelFunc
	done      chan struct{}
}

type runtimeEventSink struct {
	mu        sync.Mutex
	sessionID evtstream.SessionId
	messageID string
	pub       evtstream.EventPublisher
	engine    *Engine
	lastText  string
	terminal  bool
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

func NewEngine(opts ...Option) *Engine {
	engine := &Engine{
		active:     map[evtstream.SessionId]*activeRun{},
		pending:    map[evtstream.SessionId]PromptRequest{},
		chunkDelay: 20 * time.Millisecond,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}
	return engine
}

func RegisterSchemas(reg *evtstream.SchemaRegistry) error {
	for _, err := range []error{
		reg.RegisterCommand(CommandStartInference, &structpb.Struct{}),
		reg.RegisterCommand(CommandStopInference, &structpb.Struct{}),
		reg.RegisterEvent(EventUserMessageAccepted, &structpb.Struct{}),
		reg.RegisterEvent(EventInferenceStarted, &structpb.Struct{}),
		reg.RegisterEvent(EventTokensDelta, &structpb.Struct{}),
		reg.RegisterEvent(EventInferenceFinished, &structpb.Struct{}),
		reg.RegisterEvent(EventInferenceStopped, &structpb.Struct{}),
		reg.RegisterUIEvent(UIMessageAccepted, &structpb.Struct{}),
		reg.RegisterUIEvent(UIMessageStarted, &structpb.Struct{}),
		reg.RegisterUIEvent(UIMessageAppended, &structpb.Struct{}),
		reg.RegisterUIEvent(UIMessageFinished, &structpb.Struct{}),
		reg.RegisterUIEvent(UIMessageStopped, &structpb.Struct{}),
		reg.RegisterTimelineEntity(TimelineEntityChatMessage, &structpb.Struct{}),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

func Install(hub *evtstream.Hub, engine *Engine) error {
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
	if err := hub.RegisterUIProjection(evtstream.UIProjectionFunc(uiProjection)); err != nil {
		return err
	}
	if err := hub.RegisterTimelineProjection(evtstream.TimelineProjectionFunc(timelineProjection)); err != nil {
		return err
	}
	return nil
}

func (e *Engine) handleStartInference(ctx context.Context, cmd evtstream.Command, _ *evtstream.Session, pub evtstream.EventPublisher) error {
	payload := toMap(cmd.Payload)
	pending := e.takePendingRequest(cmd.SessionId)
	prompt := strings.TrimSpace(pending.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(asString(payload["prompt"]))
	}
	if prompt == "" {
		prompt = "Explain evtstream"
	}
	messageID := e.nextMessageID()
	userMessageID := messageID + "-user"
	if err := e.publish(ctx, cmd.SessionId, pub, EventUserMessageAccepted, map[string]any{
		"messageId": userMessageID,
		"role":      "user",
		"content":   prompt,
		"streaming": false,
	}); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	run := &activeRun{messageID: messageID, cancel: cancel, done: make(chan struct{})}
	if previous := e.swapRun(cmd.SessionId, run); previous != nil {
		previous.cancel()
		<-previous.done
	}
	go e.runPrompt(runCtx, cmd.SessionId, messageID, pending, prompt, pub, run.done)
	return nil
}

func (e *Engine) handleStopInference(_ context.Context, cmd evtstream.Command, _ *evtstream.Session, _ evtstream.EventPublisher) error {
	if current := e.currentRun(cmd.SessionId); current != nil {
		current.cancel()
	}
	return nil
}

func (e *Engine) runPrompt(ctx context.Context, sid evtstream.SessionId, messageID string, pending PromptRequest, prompt string, pub evtstream.EventPublisher, done chan struct{}) {
	defer close(done)
	defer e.clearRun(sid, messageID)
	if pending.Runtime != nil && pending.Runtime.Engine != nil {
		e.runRuntimeInference(ctx, sid, messageID, prompt, pending.Runtime, pub)
		return
	}
	e.runDemoInference(ctx, sid, messageID, prompt, pub)
}

func (e *Engine) runDemoInference(ctx context.Context, sid evtstream.SessionId, messageID, prompt string, pub evtstream.EventPublisher) {
	started := map[string]any{"messageId": messageID, "prompt": prompt, "role": "assistant", "content": "", "status": "streaming", "streaming": true}
	if err := e.publish(ctx, sid, pub, EventInferenceStarted, started); err != nil {
		return
	}

	answer := renderAnswer(prompt)
	chunks := chunkText(answer, 10)
	accumulated := ""
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, map[string]any{"messageId": messageID, "role": "assistant", "text": accumulated, "content": accumulated, "status": "stopped", "streaming": false})
			return
		case <-time.After(e.chunkDelay):
		}
		accumulated += chunk
		if err := e.publish(context.Background(), sid, pub, EventTokensDelta, map[string]any{"messageId": messageID, "role": "assistant", "chunk": chunk, "text": accumulated, "content": accumulated, "status": "streaming", "streaming": true}); err != nil {
			return
		}
	}
	_ = e.publish(context.Background(), sid, pub, EventInferenceFinished, map[string]any{"messageId": messageID, "role": "assistant", "text": accumulated, "content": accumulated, "status": "finished", "streaming": false})
}

func (e *Engine) runRuntimeInference(ctx context.Context, sid evtstream.SessionId, messageID, prompt string, runtime *infruntime.ComposedRuntime, pub evtstream.EventPublisher) {
	if runtime == nil || runtime.Engine == nil {
		e.runDemoInference(ctx, sid, messageID, prompt, pub)
		return
	}
	started := map[string]any{"messageId": messageID, "prompt": prompt, "role": "assistant", "content": "", "status": "streaming", "streaming": true}
	if err := e.publish(ctx, sid, pub, EventInferenceStarted, started); err != nil {
		return
	}

	baseSink := gepevents.EventSink(&runtimeEventSink{sessionID: sid, messageID: messageID, pub: pub, engine: e})
	eventSink := baseSink
	if runtime.WrapSink != nil {
		wrapped, err := runtime.WrapSink(baseSink)
		if err != nil {
			_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, map[string]any{"messageId": messageID, "role": "assistant", "text": "", "content": "", "status": "stopped", "streaming": false, "error": err.Error()})
			return
		}
		eventSink = wrapped
	}
	sink, ok := baseSink.(*runtimeEventSink)
	if !ok {
		_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, map[string]any{"messageId": messageID, "role": "assistant", "text": "", "content": "", "status": "stopped", "streaming": false, "error": "internal runtime sink type assertion failed"})
		return
	}
	sess := gepsession.NewSession()
	sess.Builder = &enginebuilder.Builder{
		Base:       runtime.Engine,
		EventSinks: []gepevents.EventSink{eventSink},
	}
	_, err := sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, map[string]any{"messageId": messageID, "role": "assistant", "text": sink.LastText(), "content": sink.LastText(), "status": "stopped", "streaming": false, "error": err.Error()})
		return
	}
	handle, err := sess.StartInference(ctx)
	if err != nil {
		_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, map[string]any{"messageId": messageID, "role": "assistant", "text": sink.LastText(), "content": sink.LastText(), "status": "stopped", "streaming": false, "error": err.Error()})
		return
	}
	output, err := handle.Wait()
	if err != nil {
		if !sink.IsTerminal() {
			_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, map[string]any{"messageId": messageID, "role": "assistant", "text": sink.LastText(), "content": sink.LastText(), "status": "stopped", "streaming": false, "error": err.Error()})
		}
		return
	}
	if sink.IsTerminal() {
		return
	}
	finalText := sink.LastText()
	if finalText == "" {
		finalText = assistantTextFromTurn(output)
	}
	_ = e.publish(context.Background(), sid, pub, EventInferenceFinished, map[string]any{"messageId": messageID, "role": "assistant", "text": finalText, "content": finalText, "status": "finished", "streaming": false})
}

func (e *Engine) publish(ctx context.Context, sid evtstream.SessionId, pub evtstream.EventPublisher, name string, payload map[string]any) error {
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return err
	}
	if e.hooks.OnBackendEvent != nil {
		e.hooks.OnBackendEvent(string(sid), name, cloneMap(payload))
	}
	return pub.Publish(ctx, evtstream.Event{Name: name, SessionId: sid, Payload: pb})
}

func (e *Engine) WaitIdle(ctx context.Context, sid evtstream.SessionId) error {
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

func (e *Engine) swapRun(sid evtstream.SessionId, run *activeRun) *activeRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	prev := e.active[sid]
	e.active[sid] = run
	return prev
}

func (e *Engine) currentRun(sid evtstream.SessionId) *activeRun {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active[sid]
}

func (e *Engine) clearRun(sid evtstream.SessionId, messageID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.active[sid]
	if current != nil && current.messageID == messageID {
		delete(e.active, sid)
	}
}

func (e *Engine) setPendingRequest(sid evtstream.SessionId, req PromptRequest) {
	if e == nil || sid == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pending[sid] = req
}

func (e *Engine) takePendingRequest(sid evtstream.SessionId) PromptRequest {
	if e == nil || sid == "" {
		return PromptRequest{}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	req := e.pending[sid]
	delete(e.pending, sid)
	return req
}

func (s *runtimeEventSink) PublishEvent(event gepevents.Event) error {
	if s == nil || s.pub == nil || s.engine == nil {
		return nil
	}
	switch ev := event.(type) {
	case *gepevents.EventPartialCompletion:
		s.mu.Lock()
		s.lastText = ev.Completion
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventTokensDelta, map[string]any{
			"messageId": s.messageID,
			"role":      "assistant",
			"chunk":     ev.Delta,
			"text":      ev.Completion,
			"content":   ev.Completion,
			"status":    "streaming",
			"streaming": true,
		})
	case *gepevents.EventFinal:
		s.mu.Lock()
		s.lastText = ev.Text
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventInferenceFinished, map[string]any{
			"messageId": s.messageID,
			"role":      "assistant",
			"text":      ev.Text,
			"content":   ev.Text,
			"status":    "finished",
			"streaming": false,
		})
	case *gepevents.EventError:
		text := s.LastText()
		s.mu.Lock()
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventInferenceStopped, map[string]any{
			"messageId": s.messageID,
			"role":      "assistant",
			"text":      text,
			"content":   text,
			"status":    "stopped",
			"streaming": false,
			"error":     ev.ErrorString,
		})
	case *gepevents.EventInterrupt:
		s.mu.Lock()
		s.lastText = ev.Text
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventInferenceStopped, map[string]any{
			"messageId": s.messageID,
			"role":      "assistant",
			"text":      ev.Text,
			"content":   ev.Text,
			"status":    "stopped",
			"streaming": false,
		})
	default:
		return nil
	}
}

func (s *runtimeEventSink) LastText() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastText
}

func (s *runtimeEventSink) IsTerminal() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminal
}

func uiProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, _ evtstream.TimelineView) ([]evtstream.UIEvent, error) {
	payload := toMap(ev.Payload)
	payload["ordinal"] = fmt.Sprintf("%d", ev.Ordinal)
	pb, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, err
	}
	name := ""
	switch ev.Name {
	case EventUserMessageAccepted:
		name = UIMessageAccepted
	case EventInferenceStarted:
		name = UIMessageStarted
	case EventTokensDelta:
		name = UIMessageAppended
	case EventInferenceFinished:
		name = UIMessageFinished
	case EventInferenceStopped:
		name = UIMessageStopped
	default:
		return nil, nil
	}
	return []evtstream.UIEvent{{Name: name, Payload: pb}}, nil
}

func timelineProjection(_ context.Context, ev evtstream.Event, _ *evtstream.Session, view evtstream.TimelineView) ([]evtstream.TimelineEntity, error) {
	payload := toMap(ev.Payload)
	messageID := asString(payload["messageId"])
	if messageID == "" {
		return nil, nil
	}
	entity := currentEntity(view, messageID)
	entity["messageId"] = messageID
	switch ev.Name {
	case EventUserMessageAccepted:
		entity["role"] = "user"
		entity["content"] = asString(payload["content"])
		entity["text"] = asString(payload["content"])
		entity["streaming"] = false
	case EventInferenceStarted:
		entity["prompt"] = asString(payload["prompt"])
		entity["role"] = "assistant"
		entity["content"] = ""
		entity["text"] = ""
		entity["status"] = "streaming"
		entity["streaming"] = true
	case EventTokensDelta:
		entity["role"] = "assistant"
		entity["content"] = asString(payload["content"])
		entity["text"] = asString(payload["text"])
		entity["status"] = "streaming"
		entity["streaming"] = true
	case EventInferenceFinished:
		entity["role"] = "assistant"
		entity["content"] = asString(payload["content"])
		entity["text"] = asString(payload["text"])
		entity["status"] = "finished"
		entity["streaming"] = false
	case EventInferenceStopped:
		entity["role"] = "assistant"
		entity["content"] = asString(payload["content"])
		entity["text"] = asString(payload["text"])
		entity["status"] = "stopped"
		entity["streaming"] = false
		if errText := asString(payload["error"]); errText != "" {
			entity["error"] = errText
		}
	default:
		return nil, nil
	}
	pb, err := structpb.NewStruct(entity)
	if err != nil {
		return nil, err
	}
	return []evtstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: pb}}, nil
}

func currentEntity(view evtstream.TimelineView, id string) map[string]any {
	entity, ok := view.Get(TimelineEntityChatMessage, id)
	if !ok || entity.Payload == nil {
		return map[string]any{}
	}
	if pb, ok := entity.Payload.(*structpb.Struct); ok {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}

func renderAnswer(prompt string) string {
	return "Answer: " + prompt
}

func assistantTextFromTurn(turn *turns.Turn) string {
	if turn == nil {
		return ""
	}
	parts := make([]string, 0, len(turn.Blocks))
	for _, block := range turn.Blocks {
		if block.Role != turns.RoleAssistant {
			continue
		}
		text, _ := block.Payload[turns.PayloadKeyText].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "")
}

func chunkText(text string, size int) []string {
	if size <= 0 || len(text) <= size {
		return []string{text}
	}
	out := make([]string, 0, (len(text)+size-1)/size)
	for len(text) > 0 {
		if len(text) <= size {
			out = append(out, text)
			break
		}
		out = append(out, text[:size])
		text = text[size:]
	}
	return out
}

func toMap(msg any) map[string]any {
	if pb, ok := msg.(*structpb.Struct); ok && pb != nil {
		return cloneMap(pb.AsMap())
	}
	return map[string]any{}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
