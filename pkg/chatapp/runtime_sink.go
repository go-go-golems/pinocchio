package chatapp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
	"google.golang.org/protobuf/proto"
)

type pendingStreamPatch struct {
	name    string
	key     string
	payload proto.Message
}

type runtimeEventSink struct {
	mu                  sync.Mutex
	publishMu           sync.Mutex
	publishCtx          context.Context
	sessionID           sessionstream.SessionId
	messageID           string
	prompt              string
	pub                 sessionstream.EventPublisher
	engine              *Engine
	lastText            string
	lastTextMessageID   string
	lastTextCorrelation gepevents.Correlation
	terminal            bool
	textSegment         int32
	textActive          bool
	batchInterval       time.Duration
	batchFirstSent      map[string]bool
	batchPending        *pendingStreamPatch
	batchTimer          *time.Timer
	batchGeneration     uint64
	batchErr            error
}

func (s *runtimeEventSink) PublishEvent(event gepevents.Event) error {
	if s == nil || s.pub == nil || s.engine == nil {
		return nil
	}
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if err := s.batchingError(); err != nil {
		return err
	}
	if !isBatchableRuntimeDelta(event) {
		if err := s.flushStreamPatch(); err != nil {
			return err
		}
	}
	switch ev := event.(type) {
	case *gepevents.EventProviderCallStarted:
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatProviderCallStarted, &chatappv1.ChatProviderCallStarted{Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventProviderCallMetadataUpdated:
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatProviderCallMetadataUpdated, &chatappv1.ChatProviderCallMetadataUpdated{StopReason: ev.StopReason, Usage: usageInfoFromGeppetto(ev.Usage), Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventProviderCallFinished:
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatProviderCallFinished, &chatappv1.ChatProviderCallFinished{StopReason: ev.StopReason, FinishClass: ev.FinishClass, Usage: usageInfoFromGeppetto(ev.Usage), DurationMs: ev.DurationMs, HasToolCalls: ev.HasToolCalls, Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventTextSegmentStarted:
		corr := ev.Correlation()
		textMessageID, _ := s.textSegmentIDForCorrelation(corr)
		s.mu.Lock()
		s.lastTextMessageID = textMessageID
		s.lastTextCorrelation = corr
		s.textActive = true
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{MessageId: textMessageID, Role: firstNonEmpty(ev.Role, "assistant"), Prompt: s.prompt, Status: "streaming", Streaming: true, Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventTextDelta:
		return s.publishOrBatchTextDelta(ev)
	case *gepevents.EventTextSegmentFinished:
		corr := ev.Correlation()
		textMessageID, _ := s.textSegmentIDForCorrelation(corr)
		s.mu.Lock()
		s.lastText = ev.Text
		s.lastTextMessageID = textMessageID
		s.lastTextCorrelation = corr
		s.textActive = false
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: s.prompt, Text: ev.Text, Content: ev.Text, Status: "finished", Streaming: false, Final: true, FinishReason: ev.FinishReason, Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventError:
		s.mu.Lock()
		s.terminal = true
		s.mu.Unlock()
		if err := s.finishActiveTextSegment("failed", "error", ""); err != nil {
			return err
		}
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatRunFailed, &chatappv1.ChatRunFailed{MessageId: s.messageID, Status: "failed", Error: ev.ErrorString})
	case *gepevents.EventInterrupt:
		s.mu.Lock()
		s.terminal = true
		s.mu.Unlock()
		if err := s.finishActiveTextSegment("stopped", "stopped", ev.Text); err != nil {
			return err
		}
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: s.messageID, Status: "stopped", Error: ev.Text})
	default:
		return s.engine.handleFeatureRuntimeEvent(s.publishContext(), s.sessionID, s.messageID, s.pub, s.publishOrBatchFeaturePatch, event)
	}
}

func isBatchableRuntimeDelta(event gepevents.Event) bool {
	switch event.(type) {
	case *gepevents.EventTextDelta, *gepevents.EventReasoningDelta, *gepevents.EventToolCallArgumentsDelta:
		return true
	default:
		return false
	}
}

func (s *runtimeEventSink) publishOrBatchTextDelta(ev *gepevents.EventTextDelta) error {
	corr := ev.Correlation()
	textMessageID, _ := s.textSegmentIDForCorrelation(corr)
	patch := &chatappv1.ChatTextPatch{MessageId: textMessageID, Role: "assistant", Prompt: s.prompt, StreamId: textMessageID, Sequence: Uint64FromInt64(ev.Sequence), Offset: PatchOffset(ev.Text, ev.Delta), Text: ev.Delta, Mode: chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND, Status: "streaming", Correlation: correlationInfoFromEvent(ev)}

	s.mu.Lock()
	s.lastText = ev.Text
	s.lastTextMessageID = textMessageID
	s.lastTextCorrelation = corr
	s.textActive = true
	s.mu.Unlock()
	return s.publishOrBatchStreamPatch(EventChatTextPatch, "text:"+textMessageID, patch)
}

func (s *runtimeEventSink) publishOrBatchFeaturePatch(ctx context.Context, eventName string, payload proto.Message) error {
	key := ""
	switch patch := payload.(type) {
	case *chatappv1.ChatReasoningPatch:
		key = "reasoning:" + patch.GetMessageId()
	case *chatappv1.ChatToolArgumentsPatch:
		key = "tool-arguments:" + patch.GetToolCallId()
	default:
		if err := s.flushStreamPatch(); err != nil {
			return err
		}
		return s.engine.publish(ctx, s.sessionID, s.pub, eventName, payload)
	}
	return s.publishOrBatchStreamPatch(eventName, key, payload)
}

func (s *runtimeEventSink) publishOrBatchStreamPatch(eventName, key string, payload proto.Message) error {
	if s.batchInterval <= 0 || strings.TrimSpace(key) == "" || !isAppendStreamPatch(payload) {
		if err := s.flushStreamPatch(); err != nil {
			return err
		}
		return s.publishStreamPatch(&pendingStreamPatch{name: eventName, key: key, payload: payload})
	}

	s.mu.Lock()
	if s.batchFirstSent == nil {
		s.batchFirstSent = map[string]bool{}
	}
	if s.batchPending != nil && s.batchPending.key != key {
		pending := s.detachPendingStreamPatchLocked()
		s.mu.Unlock()
		if err := s.publishStreamPatch(pending); err != nil {
			return err
		}
		s.mu.Lock()
	}
	if !s.batchFirstSent[key] {
		s.batchFirstSent[key] = true
		s.mu.Unlock()
		return s.publishStreamPatch(&pendingStreamPatch{name: eventName, key: key, payload: payload})
	}
	if s.batchPending == nil {
		s.batchPending = &pendingStreamPatch{name: eventName, key: key, payload: payload}
		s.armBatchTimerLocked()
	} else if err := mergeStreamPatch(s.batchPending.payload, payload); err != nil {
		pending := s.detachPendingStreamPatchLocked()
		s.mu.Unlock()
		if publishErr := s.publishStreamPatch(pending); publishErr != nil {
			return publishErr
		}
		return s.publishStreamPatch(&pendingStreamPatch{name: eventName, key: key, payload: payload})
	}
	s.mu.Unlock()
	return nil
}

func isAppendStreamPatch(payload proto.Message) bool {
	switch patch := payload.(type) {
	case *chatappv1.ChatTextPatch:
		return patch.GetMode() == chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND
	case *chatappv1.ChatReasoningPatch:
		return patch.GetMode() == chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND
	case *chatappv1.ChatToolArgumentsPatch:
		return patch.GetMode() == chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND
	default:
		return false
	}
}

func mergeStreamPatch(pending, next proto.Message) error {
	switch current := pending.(type) {
	case *chatappv1.ChatTextPatch:
		update, ok := next.(*chatappv1.ChatTextPatch)
		if !ok || current.GetMode() != chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND || update.GetMode() != chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND {
			return fmt.Errorf("incompatible text stream patch")
		}
		current.Text += update.GetText()
		current.Sequence = update.GetSequence()
		current.Correlation = update.GetCorrelation()
	case *chatappv1.ChatReasoningPatch:
		update, ok := next.(*chatappv1.ChatReasoningPatch)
		if !ok || current.GetMode() != chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND || update.GetMode() != chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND {
			return fmt.Errorf("incompatible reasoning stream patch")
		}
		current.Text += update.GetText()
		current.Sequence = update.GetSequence()
		current.Correlation = update.GetCorrelation()
	case *chatappv1.ChatToolArgumentsPatch:
		update, ok := next.(*chatappv1.ChatToolArgumentsPatch)
		if !ok || current.GetMode() != chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND || update.GetMode() != chatappv1.ChatStreamPatchMode_CHAT_STREAM_PATCH_MODE_APPEND {
			return fmt.Errorf("incompatible tool-argument stream patch")
		}
		current.Arguments += update.GetArguments()
		current.Sequence = update.GetSequence()
		current.Correlation = update.GetCorrelation()
	default:
		return fmt.Errorf("unsupported stream patch %T", pending)
	}
	return nil
}

func (s *runtimeEventSink) armBatchTimerLocked() {
	s.batchGeneration++
	generation := s.batchGeneration
	if s.batchTimer != nil {
		s.batchTimer.Stop()
	}
	s.batchTimer = time.AfterFunc(s.batchInterval, func() {
		s.publishMu.Lock()
		defer s.publishMu.Unlock()
		s.mu.Lock()
		if generation != s.batchGeneration {
			s.mu.Unlock()
			return
		}
		pending := s.detachPendingStreamPatchLocked()
		s.mu.Unlock()
		if err := s.publishStreamPatch(pending); err != nil {
			s.mu.Lock()
			if s.batchErr == nil {
				s.batchErr = err
			}
			s.mu.Unlock()
		}
	})
}

func (s *runtimeEventSink) detachPendingStreamPatchLocked() *pendingStreamPatch {
	pending := s.batchPending
	s.batchPending = nil
	s.batchGeneration++
	if s.batchTimer != nil {
		s.batchTimer.Stop()
		s.batchTimer = nil
	}
	return pending
}

func (s *runtimeEventSink) flushStreamPatch() error {
	s.mu.Lock()
	pending := s.detachPendingStreamPatchLocked()
	s.mu.Unlock()
	return s.publishStreamPatch(pending)
}

func (s *runtimeEventSink) publishStreamPatch(pending *pendingStreamPatch) error {
	if pending == nil || pending.payload == nil {
		return nil
	}
	return s.engine.publish(s.publishContext(), s.sessionID, s.pub, pending.name, pending.payload)
}

func (s *runtimeEventSink) batchingError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.batchErr
}

func (s *runtimeEventSink) finishActiveTextSegment(status, finishReason, text string) error {
	if s == nil || s.engine == nil || s.pub == nil {
		return nil
	}
	s.mu.Lock()
	content := firstNonEmpty(text, s.lastText)
	textMessageID := s.lastTextMessageID
	corr := s.lastTextCorrelation
	hadActiveText := s.textActive
	s.textActive = false
	s.mu.Unlock()
	if !hadActiveText || strings.TrimSpace(textMessageID) == "" {
		return nil
	}
	return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: s.prompt, Text: content, Content: content, Status: status, Streaming: false, Final: true, FinishReason: finishReason, Correlation: CorrelationInfoFromGeppetto(corr)})
}

func (s *runtimeEventSink) publishContext() context.Context {
	if s == nil {
		return context.Background()
	}
	return publishContext(s.publishCtx)
}

func (s *runtimeEventSink) ensureTextSegmentID() (string, int32) {
	if s == nil {
		return "", 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.textActive {
		s.textSegment++
		s.textActive = true
	}
	return textSegmentMessageID(s.messageID, s.textSegment), s.textSegment
}

func (s *runtimeEventSink) textSegmentIDForCorrelation(corr gepevents.Correlation) (string, int32) {
	if s == nil {
		return "", 0
	}
	if suffix := sanitizeCorrelationID(corr.SegmentID); suffix != "" {
		return fmt.Sprintf("%s:text:%s", strings.TrimSpace(s.messageID), suffix), 0
	}
	return s.ensureTextSegmentID()
}

func textSegmentMessageID(messageID string, segment int32) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || segment <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:text:%d", messageID, segment)
}

func sanitizeCorrelationID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ':' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-:_ .")
}

func (s *runtimeEventSink) HasTextSegment() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(s.lastTextMessageID) != ""
}

func (s *runtimeEventSink) HasActiveTextSegment() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.textActive && strings.TrimSpace(s.lastTextMessageID) != ""
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

func Uint64FromInt64(value int64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func PatchOffset(snapshot, delta string) uint64 {
	if delta == "" || len(snapshot) < len(delta) {
		return 0
	}
	return Uint64FromInt(len(snapshot) - len(delta))
}

func Uint64FromInt(value int) uint64 {
	if value <= 0 {
		return 0
	}
	// #nosec G115 -- value is non-negative and Go int fits in uint64 on supported architectures.
	return uint64(value)
}
