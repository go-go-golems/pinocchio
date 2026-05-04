package chatapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	gepsession "github.com/go-go-golems/geppetto/pkg/inference/session"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop/enginebuilder"
	"github.com/go-go-golems/geppetto/pkg/turns"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
}

type activeRun struct {
	messageID string
	cancel    context.CancelFunc
	done      chan struct{}
}

type runtimeEventSink struct {
	mu        sync.Mutex
	sessionID sessionstream.SessionId
	messageID string
	prompt    string
	pub       sessionstream.EventPublisher
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

func (e *Engine) handleStartInference(ctx context.Context, cmd sessionstream.Command, _ *sessionstream.Session, pub sessionstream.EventPublisher) error {
	payload, ok := cmd.Payload.(*chatappv1.StartInferenceCommand)
	if !ok || payload == nil {
		return fmt.Errorf("start inference payload must be %T, got %T", &chatappv1.StartInferenceCommand{}, cmd.Payload)
	}
	pending := e.takePendingRequest(strings.TrimSpace(payload.GetRequestId()))
	prompt := strings.TrimSpace(pending.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(payload.GetPrompt())
	}
	if prompt == "" {
		prompt = "Explain evtstream"
	}
	messageID := e.nextMessageID()
	userMessageID := messageID + "-user"
	if err := e.publish(ctx, cmd.SessionId, pub, EventUserMessageAccepted, newChatMessageUpdate(userMessageID, "user", prompt, prompt, "", "", false, "")); err != nil {
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

func (e *Engine) handleStopInference(_ context.Context, cmd sessionstream.Command, _ *sessionstream.Session, _ sessionstream.EventPublisher) error {
	if current := e.currentRun(cmd.SessionId); current != nil {
		current.cancel()
	}
	return nil
}

func (e *Engine) runPrompt(ctx context.Context, sid sessionstream.SessionId, messageID string, pending PromptRequest, prompt string, pub sessionstream.EventPublisher, done chan struct{}) {
	defer close(done)
	defer e.clearRun(sid, messageID)
	if pending.Runtime != nil && pending.Runtime.Engine != nil {
		e.runRuntimeInference(ctx, sid, messageID, prompt, pending.Runtime, pub)
		return
	}
	e.runDemoInference(ctx, sid, messageID, prompt, pub)
}

func (e *Engine) runDemoInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, pub sessionstream.EventPublisher) {
	started := newChatMessageUpdate(messageID, "assistant", "", "", prompt, "streaming", true, "")
	if err := e.publish(ctx, sid, pub, EventInferenceStarted, started); err != nil {
		return
	}

	answer := renderAnswer(prompt)
	chunks := chunkText(answer, 10)
	accumulated := ""
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", accumulated, accumulated, prompt, "stopped", false, ""))
			return
		case <-time.After(e.chunkDelay):
		}
		accumulated += chunk
		if err := e.publish(context.Background(), sid, pub, EventTokensDelta, newChatMessageDelta(messageID, chunk, accumulated, prompt, "streaming", true, "")); err != nil {
			return
		}
	}
	_ = e.publish(context.Background(), sid, pub, EventInferenceFinished, newChatMessageUpdate(messageID, "assistant", accumulated, accumulated, prompt, "finished", false, ""))
}

func (e *Engine) runRuntimeInference(ctx context.Context, sid sessionstream.SessionId, messageID, prompt string, runtime *infruntime.ComposedRuntime, pub sessionstream.EventPublisher) {
	if runtime == nil || runtime.Engine == nil {
		e.runDemoInference(ctx, sid, messageID, prompt, pub)
		return
	}
	started := newChatMessageUpdate(messageID, "assistant", "", "", prompt, "streaming", true, "")
	if err := e.publish(ctx, sid, pub, EventInferenceStarted, started); err != nil {
		return
	}

	baseSink := gepevents.EventSink(&runtimeEventSink{sessionID: sid, messageID: messageID, prompt: prompt, pub: pub, engine: e})
	eventSink := baseSink
	if runtime.WrapSink != nil {
		wrapped, err := runtime.WrapSink(baseSink)
		if err != nil {
			_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", "", "", prompt, "stopped", false, err.Error()))
			return
		}
		eventSink = wrapped
	}
	sink, ok := baseSink.(*runtimeEventSink)
	if !ok {
		_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", "", "", prompt, "stopped", false, "internal runtime sink type assertion failed"))
		return
	}
	sess := gepsession.NewSession()
	sess.Builder = &enginebuilder.Builder{
		Base:       runtime.Engine,
		EventSinks: []gepevents.EventSink{eventSink},
	}
	_, err := sess.AppendNewTurnFromUserPrompt(prompt)
	if err != nil {
		_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", sink.LastText(), sink.LastText(), prompt, "stopped", false, err.Error()))
		return
	}
	handle, err := sess.StartInference(ctx)
	if err != nil {
		_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", sink.LastText(), sink.LastText(), prompt, "stopped", false, err.Error()))
		return
	}
	output, err := handle.Wait()
	if err != nil {
		if !sink.IsTerminal() {
			if isMaxIterationsError(err) {
				_ = e.publish(context.Background(), sid, pub, EventInferenceFinished, newChatMessageUpdate(runtimeWarningMessageID(messageID), "warning", maxIterationsWarningText(err), maxIterationsWarningText(err), prompt, "finished", false, ""))
			}
			_ = e.publish(context.Background(), sid, pub, EventInferenceStopped, newChatMessageUpdate(messageID, "assistant", sink.LastText(), sink.LastText(), prompt, "stopped", false, err.Error()))
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
	_ = e.publish(context.Background(), sid, pub, EventInferenceFinished, newChatMessageUpdate(messageID, "assistant", finalText, finalText, prompt, "finished", false, ""))
}

func (e *Engine) publish(ctx context.Context, sid sessionstream.SessionId, pub sessionstream.EventPublisher, name string, payload proto.Message) error {
	if payload == nil {
		return fmt.Errorf("event %s payload is nil", name)
	}
	if e.hooks.OnBackendEvent != nil {
		e.hooks.OnBackendEvent(string(sid), name, protoMessageAsMap(payload))
	}
	return pub.Publish(ctx, sessionstream.Event{Name: name, SessionId: sid, Payload: payload})
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

func (s *runtimeEventSink) PublishEvent(event gepevents.Event) error {
	if s == nil || s.pub == nil || s.engine == nil {
		return nil
	}
	switch ev := event.(type) {
	case *gepevents.EventPartialCompletion:
		s.mu.Lock()
		s.lastText = ev.Completion
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventTokensDelta, newChatMessageDelta(s.messageID, ev.Delta, ev.Completion, s.prompt, "streaming", true, ""))
	case *gepevents.EventFinal:
		s.mu.Lock()
		s.lastText = ev.Text
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventInferenceFinished, newChatMessageUpdate(s.messageID, "assistant", ev.Text, ev.Text, s.prompt, "finished", false, ""))
	case *gepevents.EventError:
		text := s.LastText()
		s.mu.Lock()
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventInferenceStopped, newChatMessageUpdate(s.messageID, "assistant", text, text, s.prompt, "stopped", false, ev.ErrorString))
	case *gepevents.EventInterrupt:
		s.mu.Lock()
		s.lastText = ev.Text
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(context.Background(), s.sessionID, s.pub, EventInferenceStopped, newChatMessageUpdate(s.messageID, "assistant", ev.Text, ev.Text, s.prompt, "stopped", false, ""))
	default:
		return s.engine.handleFeatureRuntimeEvent(context.Background(), s.sessionID, s.messageID, s.pub, event)
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

func baseUIProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, _ sessionstream.TimelineView) ([]sessionstream.UIEvent, error) {
	payload, ok := ev.Payload.(*chatappv1.ChatMessageUpdate)
	if !ok || payload == nil {
		return nil, nil
	}
	cloned := proto.Clone(payload)
	switch ev.Name {
	case EventUserMessageAccepted:
		return []sessionstream.UIEvent{{Name: UIMessageAccepted, Payload: cloned}}, nil
	case EventInferenceStarted:
		return []sessionstream.UIEvent{{Name: UIMessageStarted, Payload: cloned}}, nil
	case EventTokensDelta:
		return []sessionstream.UIEvent{{Name: UIMessageAppended, Payload: cloned}}, nil
	case EventInferenceFinished:
		return []sessionstream.UIEvent{{Name: UIMessageFinished, Payload: cloned}}, nil
	case EventInferenceStopped:
		return []sessionstream.UIEvent{{Name: UIMessageStopped, Payload: cloned}}, nil
	default:
		return nil, nil
	}
}

func baseTimelineProjection(_ context.Context, ev sessionstream.Event, _ *sessionstream.Session, view sessionstream.TimelineView) ([]sessionstream.TimelineEntity, error) {
	payload, ok := ev.Payload.(*chatappv1.ChatMessageUpdate)
	if !ok || payload == nil {
		return nil, nil
	}
	messageID := strings.TrimSpace(payload.GetMessageId())
	if messageID == "" {
		return nil, nil
	}
	entity, hadEntity := currentChatMessageEntity(view, messageID)
	switch ev.Name {
	case EventUserMessageAccepted:
		entity.MessageId = messageID
		entity.Role = "user"
		entity.Content = firstNonEmpty(payload.GetContent(), payload.GetText())
		entity.Text = entity.Content
		entity.Streaming = false
	case EventInferenceStarted:
		content := firstNonEmpty(payload.GetContent(), payload.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Status = "streaming"
		entity.Streaming = true
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
		if content != "" {
			entity.Content = content
			entity.Text = content
		}
	case EventTokensDelta:
		content := firstNonEmpty(payload.GetContent(), payload.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = "streaming"
		entity.Streaming = true
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
	case EventInferenceFinished:
		content := firstNonEmpty(payload.GetContent(), payload.GetText(), entity.GetContent(), entity.GetText())
		if content == "" && !hadEntity {
			return nil, nil
		}
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = "finished"
		entity.Streaming = false
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
	case EventInferenceStopped:
		content := firstNonEmpty(payload.GetContent(), payload.GetText(), entity.GetContent(), entity.GetText())
		entity.MessageId = messageID
		entity.Role = firstNonEmpty(payload.GetRole(), "assistant")
		entity.Content = content
		entity.Text = content
		entity.Status = "stopped"
		entity.Streaming = false
		if prompt := payload.GetPrompt(); prompt != "" {
			entity.Prompt = prompt
		}
		if errText := payload.GetError(); errText != "" {
			entity.Error = errText
		}
	default:
		return nil, nil
	}
	return []sessionstream.TimelineEntity{{Kind: TimelineEntityChatMessage, Id: messageID, Payload: entity}}, nil
}

func currentChatMessageEntity(view sessionstream.TimelineView, id string) (*chatappv1.ChatMessageEntity, bool) {
	entity, ok := view.Get(TimelineEntityChatMessage, id)
	if !ok || entity.Payload == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	pb, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
	if !ok || pb == nil {
		return &chatappv1.ChatMessageEntity{}, false
	}
	return proto.Clone(pb).(*chatappv1.ChatMessageEntity), true
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

func newChatMessageUpdate(messageID, role, content, text, prompt, status string, streaming bool, errText string) *chatappv1.ChatMessageUpdate {
	return &chatappv1.ChatMessageUpdate{
		MessageId: messageID,
		Role:      role,
		Prompt:    prompt,
		Text:      text,
		Content:   content,
		Status:    status,
		Streaming: streaming,
		Error:     errText,
		Chunk:     "",
	}
}

func newChatMessageDelta(messageID, chunk, content, prompt, status string, streaming bool, errText string) *chatappv1.ChatMessageUpdate {
	return &chatappv1.ChatMessageUpdate{
		MessageId: messageID,
		Role:      "assistant",
		Prompt:    prompt,
		Chunk:     chunk,
		Text:      content,
		Content:   content,
		Status:    status,
		Streaming: streaming,
		Error:     errText,
	}
}

func runtimeWarningMessageID(messageID string) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return "chat-warning"
	}
	return messageID + ":warning"
}

func isMaxIterationsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "max iterations")
}

func maxIterationsWarningText(err error) string {
	message := "tool loop reached the maximum iteration limit"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = strings.TrimSpace(err.Error())
	}
	return "Warning: inference stopped because " + message + ". The answer may be incomplete; try narrowing the request or increasing the max-iterations setting."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func protoMessageAsMap(msg proto.Message) map[string]any {
	if msg == nil {
		return map[string]any{}
	}
	body, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(msg)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return out
}
