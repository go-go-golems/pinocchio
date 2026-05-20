package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/bobatea/pkg/timeline"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// BubbleTeaSender is the small subset of tea.Program used by the chatapp UI
// fanout. *tea.Program satisfies this interface, and tests can provide a fake.
type BubbleTeaSender interface {
	Send(tea.Msg)
}

// ChatAppUIFanout adapts projected chatapp/sessionstream UI events into the
// Bubble Tea timeline messages consumed by the existing bobatea chat widgets.
type ChatAppUIFanout struct {
	sender BubbleTeaSender
	mu     sync.Mutex
	seen   map[string]bool
	starts map[string]time.Time
}

var _ sessionstream.UIFanout = (*ChatAppUIFanout)(nil)

func NewChatAppUIFanout(sender BubbleTeaSender) (*ChatAppUIFanout, error) {
	if sender == nil {
		return nil, fmt.Errorf("bubble tea sender is nil")
	}
	return &ChatAppUIFanout{sender: sender, seen: map[string]bool{}, starts: map[string]time.Time{}}, nil
}

func NewChatAppUIFanoutForProgram(program *tea.Program) (*ChatAppUIFanout, error) {
	if program == nil {
		return nil, fmt.Errorf("bubble tea program is nil")
	}
	return NewChatAppUIFanout(program)
}

func (f *ChatAppUIFanout) PublishUI(_ context.Context, _ sessionstream.SessionId, _ uint64, events []sessionstream.UIEvent) error {
	if f == nil || f.sender == nil {
		return fmt.Errorf("chatapp UI fanout is not initialized")
	}
	for _, ev := range events {
		if err := f.publishOne(ev); err != nil {
			return err
		}
	}
	return nil
}

// HydrateSnapshot sends Bubble Tea timeline messages for already-hydrated
// chatapp timeline entities. Callers can use this when entering a TUI session
// before new UI events start streaming.
func (f *ChatAppUIFanout) HydrateSnapshot(snap sessionstream.Snapshot) error {
	if f == nil || f.sender == nil {
		return fmt.Errorf("chatapp UI fanout is not initialized")
	}
	for _, entity := range snap.Entities {
		msg, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
		if !ok || msg == nil {
			continue
		}
		id := firstNonEmpty(msg.GetMessageId(), entity.Id)
		text := firstNonEmpty(msg.GetContent(), msg.GetText())
		role := firstNonEmpty(msg.GetRole(), "assistant")
		streaming := msg.GetStreaming() || strings.EqualFold(msg.GetStatus(), "streaming")
		f.sender.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Renderer: timeline.RendererDescriptor{Kind: "llm_text"}, Props: map[string]any{"role": role, "text": text, "streaming": streaming}, StartedAt: time.Now()})
		if !streaming {
			f.sender.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Result: map[string]any{"text": text}})
		}
	}
	return nil
}

func (f *ChatAppUIFanout) publishOne(ev sessionstream.UIEvent) error {
	switch p := ev.Payload.(type) {
	case *chatappv1.ChatUserMessageAccepted:
		id := firstNonEmpty(p.GetMessageId(), "user-message")
		text := firstNonEmpty(p.GetContent(), p.GetText())
		f.sender.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Renderer: timeline.RendererDescriptor{Kind: "llm_text"}, Props: map[string]any{"role": "user", "text": text, "streaming": false}, StartedAt: time.Now()})
		f.sender.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Result: map[string]any{"text": text}})
	case *chatappv1.ChatTextSegmentStarted:
		f.markStart(p.GetMessageId())
	case *chatappv1.ChatTextPatch:
		id := firstNonEmpty(p.GetMessageId(), p.GetStreamId())
		text := p.GetText()
		if strings.TrimSpace(text) != "" {
			f.ensureAssistant(id, p.GetRole(), text)
		}
		if f.has(id) {
			f.sender.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Patch: map[string]any{"text": text, "streaming": !p.GetFinal()}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		}
	case *chatappv1.ChatTextSegmentFinished:
		id := p.GetMessageId()
		text := firstNonEmpty(p.GetContent(), p.GetText())
		if strings.TrimSpace(text) != "" {
			f.ensureAssistant(id, p.GetRole(), text)
		}
		if f.has(id) {
			f.sender.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Result: map[string]any{"text": text}})
			f.sender.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
			f.clear(id)
		}
		f.sender.Send(boba_chat.BackendFinishedMsg{})
	case *chatappv1.ChatRunFailed:
		id := firstNonEmpty(p.GetMessageId(), "chat-run-failed")
		text := "**Error**\n\n" + p.GetError()
		f.ensureAssistant(id, "assistant", text)
		f.sender.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Result: map[string]any{"text": text}})
		f.sender.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Patch: map[string]any{"streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		f.clear(id)
		f.sender.Send(boba_chat.BackendFinishedMsg{})
	case *chatappv1.ChatRunStopped:
		f.sender.Send(boba_chat.BackendFinishedMsg{})
	case *chatappv1.ChatReasoningSegmentStarted:
		id := firstNonEmpty(p.GetMessageId(), p.GetParentMessageId()+":thinking")
		f.sender.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Renderer: timeline.RendererDescriptor{Kind: "llm_text"}, Props: map[string]any{"role": "thinking", "text": "", "streaming": true}, StartedAt: time.Now()})
	case *chatappv1.ChatReasoningPatch:
		id := firstNonEmpty(p.GetMessageId(), p.GetParentMessageId()+":thinking")
		f.sender.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Patch: map[string]any{"text": p.GetText(), "streaming": !p.GetFinal()}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
	case *chatappv1.ChatReasoningSegmentFinished:
		id := firstNonEmpty(p.GetMessageId(), p.GetParentMessageId()+":thinking")
		text := firstNonEmpty(p.GetContent(), p.GetText())
		f.sender.Send(timeline.UIEntityUpdated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Patch: map[string]any{"text": text, "streaming": false}, Version: time.Now().UnixNano(), UpdatedAt: time.Now()})
		f.sender.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}})
	}
	return nil
}

func (f *ChatAppUIFanout) markStart(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.starts[id].IsZero() {
		f.starts[id] = time.Now()
	}
}

func (f *ChatAppUIFanout) ensureAssistant(id, role, initialText string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	f.mu.Lock()
	if f.seen[id] {
		f.mu.Unlock()
		return
	}
	started := f.starts[id]
	if started.IsZero() {
		started = time.Now()
	}
	f.seen[id] = true
	f.mu.Unlock()
	f.sender.Send(timeline.UIEntityCreated{ID: timeline.EntityID{LocalID: id, Kind: "llm_text"}, Renderer: timeline.RendererDescriptor{Kind: "llm_text"}, Props: map[string]any{"role": firstNonEmpty(role, "assistant"), "text": initialText, "streaming": true}, StartedAt: started})
}

func (f *ChatAppUIFanout) has(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen[id]
}

func (f *ChatAppUIFanout) clear(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.seen, id)
	delete(f.starts, id)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
