package chatapp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

type runtimeEventSink struct {
	mu                  sync.Mutex
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
}

func (s *runtimeEventSink) PublishEvent(event gepevents.Event) error {
	if s == nil || s.pub == nil || s.engine == nil {
		return nil
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
		corr := ev.Correlation()
		textMessageID, _ := s.textSegmentIDForCorrelation(corr)
		s.mu.Lock()
		s.lastText = ev.Text
		s.lastTextMessageID = textMessageID
		s.lastTextCorrelation = corr
		s.textActive = true
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextDelta, &chatappv1.ChatTextDelta{MessageId: textMessageID, Role: "assistant", Prompt: s.prompt, Chunk: ev.Delta, Text: ev.Text, Content: ev.Text, Status: "streaming", Streaming: true, Correlation: correlationInfoFromEvent(ev)})
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
		return s.engine.handleFeatureRuntimeEvent(s.publishContext(), s.sessionID, s.messageID, s.pub, event)
	}
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
