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
	mu          sync.Mutex
	publishCtx  context.Context
	sessionID   sessionstream.SessionId
	messageID   string
	prompt      string
	pub         sessionstream.EventPublisher
	engine      *Engine
	lastText    string
	terminal    bool
	textSegment int32
	textActive  bool
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
		textMessageID, _ := s.textSegmentIDForCorrelation(ev.Correlation())
		s.mu.Lock()
		s.textActive = true
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextSegmentStarted, &chatappv1.ChatTextSegmentStarted{MessageId: textMessageID, Role: firstNonEmpty(ev.Role, "assistant"), Prompt: s.prompt, Status: "streaming", Streaming: true, Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventTextDelta:
		textMessageID, _ := s.textSegmentIDForCorrelation(ev.Correlation())
		s.mu.Lock()
		s.lastText = ev.Text
		s.textActive = true
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextDelta, &chatappv1.ChatTextDelta{MessageId: textMessageID, Role: "assistant", Prompt: s.prompt, Chunk: ev.Delta, Text: ev.Text, Content: ev.Text, Status: "streaming", Streaming: true, Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventTextSegmentFinished:
		textMessageID, _ := s.textSegmentIDForCorrelation(ev.Correlation())
		s.mu.Lock()
		s.lastText = ev.Text
		s.textActive = false
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatTextSegmentFinished, &chatappv1.ChatTextSegmentFinished{MessageId: textMessageID, Role: "assistant", Prompt: s.prompt, Text: ev.Text, Content: ev.Text, Status: "finished", Streaming: false, Final: true, FinishReason: ev.FinishReason, Correlation: correlationInfoFromEvent(ev)})
	case *gepevents.EventError:
		s.mu.Lock()
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatRunFailed, &chatappv1.ChatRunFailed{MessageId: s.messageID, Status: "failed", Error: ev.ErrorString})
	case *gepevents.EventInterrupt:
		s.mu.Lock()
		s.terminal = true
		s.mu.Unlock()
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventChatRunStopped, &chatappv1.ChatRunStopped{MessageId: s.messageID, Status: "stopped", Error: ev.Text})
	default:
		return s.engine.handleFeatureRuntimeEvent(s.publishContext(), s.sessionID, s.messageID, s.pub, event)
	}
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
	if corr.SegmentIndex > 0 {
		s.mu.Lock()
		if corr.SegmentIndex > s.textSegment {
			s.textSegment = corr.SegmentIndex
		}
		s.mu.Unlock()
		return textSegmentMessageID(s.messageID, corr.SegmentIndex), corr.SegmentIndex
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
