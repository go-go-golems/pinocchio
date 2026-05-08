package chatapp

import (
	"context"
	"fmt"
	"strconv"
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
	case *gepevents.EventPartialCompletion:
		textMessageID, segment := s.ensureTextSegmentID()
		s.mu.Lock()
		s.lastText = ev.Completion
		s.mu.Unlock()
		payload := newChatMessageDelta(textMessageID, ev.Delta, ev.Completion, s.prompt, "streaming", true, "")
		payload.ParentMessageId = s.messageID
		payload.Segment = segment
		payload.SegmentType = "text"
		applyChatMessageProviderInfo(payload, ev.Metadata())
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventTokensDelta, payload)
	case *gepevents.EventFinal:
		textMessageID, segment := s.ensureTextSegmentID()
		s.mu.Lock()
		s.lastText = ev.Text
		s.terminal = true
		s.textActive = false
		s.mu.Unlock()
		payload := newChatMessageUpdate(textMessageID, "assistant", ev.Text, ev.Text, s.prompt, "finished", false, "")
		payload.ParentMessageId = s.messageID
		payload.Segment = segment
		payload.SegmentType = "text"
		payload.Final = true
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventInferenceFinished, payload)
	case *gepevents.EventError:
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventInferenceStopped, s.stoppedMessageUpdate(s.messageID, ev.ErrorString))
	case *gepevents.EventInterrupt:
		textMessageID, segment := s.ensureTextSegmentID()
		s.mu.Lock()
		s.lastText = ev.Text
		s.terminal = true
		s.textActive = false
		s.mu.Unlock()
		payload := newChatMessageUpdate(textMessageID, "assistant", ev.Text, ev.Text, s.prompt, "stopped", false, "")
		payload.ParentMessageId = s.messageID
		payload.Segment = segment
		payload.SegmentType = "text"
		payload.Final = true
		return s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventInferenceStopped, payload)
	default:
		if isTranscriptBoundaryEvent(event) {
			if textMessageID, segment, text, ok := s.finishTextSegment(); ok {
				payload := newChatMessageUpdate(textMessageID, "assistant", text, text, s.prompt, "finished", false, "")
				payload.ParentMessageId = s.messageID
				payload.Segment = segment
				payload.SegmentType = "text"
				if err := s.engine.publish(s.publishContext(), s.sessionID, s.pub, EventInferenceFinished, payload); err != nil {
					return err
				}
			}
		}
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

func (s *runtimeEventSink) stoppedMessageUpdate(defaultMessageID, errText string) *chatappv1.ChatMessageUpdate {
	if s == nil {
		return newChatMessageUpdate(defaultMessageID, "assistant", "", "", "", "stopped", false, errText)
	}
	s.mu.Lock()
	text := s.lastText
	segment := s.textSegment
	active := s.textActive && segment > 0
	if active {
		s.textActive = false
	}
	s.terminal = true
	s.mu.Unlock()

	if !active {
		// If a prior text segment was already closed by a tool/reasoning boundary,
		// do not duplicate that segment's content into the parent run-level stopped row.
		if segment > 0 {
			text = ""
		}
		return newChatMessageUpdate(defaultMessageID, "assistant", text, text, s.prompt, "stopped", false, errText)
	}
	payload := newChatMessageUpdate(textSegmentMessageID(s.messageID, segment), "assistant", text, text, s.prompt, "stopped", false, errText)
	payload.ParentMessageId = s.messageID
	payload.Segment = segment
	payload.SegmentType = "text"
	payload.Final = true
	return payload
}

func (s *runtimeEventSink) finishTextSegment() (string, int32, string, bool) {
	if s == nil {
		return "", 0, "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.textActive || s.textSegment <= 0 || strings.TrimSpace(s.lastText) == "" {
		s.textActive = false
		return "", 0, "", false
	}
	s.textActive = false
	return textSegmentMessageID(s.messageID, s.textSegment), s.textSegment, s.lastText, true
}

func textSegmentMessageID(messageID string, segment int32) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" || segment <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:text:%d", messageID, segment)
}

func applyChatMessageProviderInfo(update *chatappv1.ChatMessageUpdate, metadata gepevents.EventMetadata) {
	if update == nil || metadata.Extra == nil {
		return
	}
	if v, ok := metadata.Extra["provider"].(string); ok {
		update.Provider = strings.TrimSpace(v)
	}
	if v, ok := metadata.Extra["response_id"].(string); ok {
		update.ResponseId = strings.TrimSpace(v)
	}
	if v, ok := int32FromMetadata(metadata.Extra["choice_index"]); ok {
		update.ChoiceIndex = &v
	}
	if v, ok := metadata.Extra["stream_kind"].(string); ok {
		update.StreamKind = strings.TrimSpace(v)
	}
	if v, ok := metadata.Extra["correlation_key"].(string); ok {
		update.CorrelationKey = strings.TrimSpace(v)
	}
}

func int32FromMetadata(v any) (int32, bool) {
	switch tv := v.(type) {
	case int:
		parsed, err := strconv.ParseInt(strconv.Itoa(tv), 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(parsed), true
	case int32:
		return tv, true
	case int64:
		if tv < int64(-1<<31) || tv > int64(1<<31-1) {
			return 0, false
		}
		return int32(tv), true
	case float64:
		if tv < float64(-1<<31) || tv > float64(1<<31-1) || tv != float64(int32(tv)) {
			return 0, false
		}
		return int32(tv), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(tv), 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(parsed), true
	default:
		return 0, false
	}
}

func isTranscriptBoundaryEvent(event gepevents.Event) bool {
	switch event.(type) {
	case *gepevents.EventToolCall, *gepevents.EventToolCallExecute, *gepevents.EventToolResult, *gepevents.EventToolCallExecutionResult:
		return true
	default:
		return false
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
