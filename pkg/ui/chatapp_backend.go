package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/go-go-golems/pinocchio/pkg/chatapp"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// TurnPersister stores successful final turns produced by a chat backend run.
type TurnPersister interface {
	PersistTurn(ctx context.Context, t *turns.Turn) error
}

// ChatAppBackend is the Bubble Tea chat backend backed by chatapp/sessionstream.
type ChatAppBackend struct {
	service *chatapp.Service
	sid     sessionstream.SessionId
	runtime *infruntime.ComposedRuntime

	turnPersister TurnPersister

	mu          sync.Mutex
	currentTurn *turns.Turn
	running     bool
	killed      atomic.Bool
}

var _ boba_chat.Backend = (*ChatAppBackend)(nil)

type ChatAppBackendOption func(*ChatAppBackend)

func WithTurnPersister(p TurnPersister) ChatAppBackendOption {
	return func(b *ChatAppBackend) {
		b.turnPersister = p
	}
}

func NewChatAppBackend(service *chatapp.Service, sid sessionstream.SessionId, runtime *infruntime.ComposedRuntime, seed *turns.Turn, opts ...ChatAppBackendOption) (*ChatAppBackend, error) {
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
	backend := &ChatAppBackend{service: service, sid: sid, runtime: runtime, currentTurn: seedClone}
	for _, opt := range opts {
		if opt != nil {
			opt(backend)
		}
	}
	return backend, nil
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

	var finalTurnMu sync.Mutex
	var finalTurn *turns.Turn
	req := chatapp.PromptRequest{
		Prompt:      prompt,
		InitialTurn: initialTurn,
		Runtime:     b.runtime,
		OnFinalTurn: func(t *turns.Turn) {
			finalTurnMu.Lock()
			defer finalTurnMu.Unlock()
			if t != nil {
				finalTurn = t.Clone()
			}
		},
	}
	if err := b.service.SubmitPromptRequest(ctx, b.sid, req); err != nil {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		return nil, err
	}

	return func() tea.Msg {
		err := b.service.WaitIdle(ctx, b.sid)
		if err != nil {
			b.mu.Lock()
			b.running = false
			b.mu.Unlock()
			return boba_chat.ErrorMsg(err)
		}

		finalTurnMu.Lock()
		updatedTurn := finalTurn
		finalTurnMu.Unlock()
		if updatedTurn != nil && b.turnPersister != nil {
			if err := b.turnPersister.PersistTurn(ctx, updatedTurn.Clone()); err != nil {
				b.mu.Lock()
				b.running = false
				b.mu.Unlock()
				return boba_chat.ErrorMsg(err)
			}
		}

		b.mu.Lock()
		if updatedTurn != nil {
			b.currentTurn = updatedTurn.Clone()
		} else {
			b.currentTurn = initialTurn.Clone()
		}
		b.running = false
		b.mu.Unlock()
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
