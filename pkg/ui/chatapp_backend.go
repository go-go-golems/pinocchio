package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	chatappv1 "github.com/go-go-golems/pinocchio/pkg/chatapp/pb/proto/pinocchio/chatapp/v1"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// ChatAppBackend is the Bubble Tea chat backend backed by chatapp/sessionstream.
type ChatAppBackend struct {
	service *chatapp.Service
	sid     sessionstream.SessionId
	runtime *infruntime.ComposedRuntime

	mu          sync.Mutex
	seed        *turns.Turn
	currentTurn *turns.Turn
	running     bool
	killed      atomic.Bool
}

var _ boba_chat.Backend = (*ChatAppBackend)(nil)

func NewChatAppBackend(service *chatapp.Service, sid sessionstream.SessionId, runtime *infruntime.ComposedRuntime, seed *turns.Turn) (*ChatAppBackend, error) {
	if service == nil {
		return nil, fmt.Errorf("chatapp service is nil")
	}
	if sid == "" {
		return nil, fmt.Errorf("session id is empty")
	}
	if runtime == nil || runtime.Engine == nil {
		return nil, fmt.Errorf("chatapp backend runtime engine is nil")
	}
	var seedClone *turns.Turn
	if seed != nil {
		seedClone = seed.Clone()
	}
	return &ChatAppBackend{service: service, sid: sid, runtime: runtime, seed: seedClone, currentTurn: seedClone}, nil
}

func (b *ChatAppBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is empty")
	}
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return nil, fmt.Errorf("chatapp backend is already running")
	}
	initialTurn := turnWithUserPrompt(b.currentTurn, prompt)
	b.running = true
	b.mu.Unlock()

	req := chatapp.PromptRequest{Prompt: prompt, InitialTurn: initialTurn, Runtime: b.runtime}
	if err := b.service.SubmitPromptRequest(ctx, b.sid, req); err != nil {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		return nil, err
	}

	return func() tea.Msg {
		err := b.service.WaitIdle(ctx, b.sid)
		if err == nil {
			var snap sessionstream.Snapshot
			snap, err = b.service.Snapshot(ctx, b.sid)
			if err == nil {
				b.mu.Lock()
				b.currentTurn = turnFromSnapshot(b.seed, snap)
				b.running = false
				b.mu.Unlock()
			}
		}
		if err != nil {
			b.mu.Lock()
			b.running = false
			b.mu.Unlock()
			return boba_chat.ErrorMsg(err)
		}
		return boba_chat.BackendFinishedMsg{}
	}, nil
}

func (b *ChatAppBackend) Interrupt() {
	if b == nil || b.service == nil {
		return
	}
	_ = b.service.Stop(context.Background(), b.sid)
}

func (b *ChatAppBackend) Kill() {
	b.killed.Store(true)
	b.Interrupt()
}

func (b *ChatAppBackend) IsFinished() bool {
	if b == nil || b.killed.Load() {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.running
}

func (b *ChatAppBackend) SessionID() string { return string(b.sid) }

func turnWithUserPrompt(base *turns.Turn, prompt string) *turns.Turn {
	var t *turns.Turn
	if base != nil {
		t = base.Clone()
	} else {
		t = &turns.Turn{}
	}
	turns.AppendBlock(t, turns.NewUserTextBlock(prompt))
	return t
}

func turnFromSnapshot(seed *turns.Turn, snap sessionstream.Snapshot) *turns.Turn {
	out := &turns.Turn{}
	if seed != nil {
		for _, block := range seed.Blocks {
			if block.Role == turns.RoleUser || block.Role == turns.RoleAssistant {
				continue
			}
			turns.AppendBlock(out, block)
		}
	}
	entities := append([]sessionstream.TimelineEntity(nil), snap.Entities...)
	sort.SliceStable(entities, func(i, j int) bool { return entities[i].CreatedOrdinal < entities[j].CreatedOrdinal })
	for _, entity := range entities {
		msg, ok := entity.Payload.(*chatappv1.ChatMessageEntity)
		if !ok || msg == nil {
			continue
		}
		text := strings.TrimSpace(firstNonEmpty(msg.GetContent(), msg.GetText()))
		if text == "" {
			continue
		}
		switch msg.GetRole() {
		case "user":
			turns.AppendBlock(out, turns.NewUserTextBlock(text))
		case "assistant":
			turns.AppendBlock(out, turns.NewAssistantTextBlock(text))
		}
	}
	return out
}
