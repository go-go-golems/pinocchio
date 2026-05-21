package ui

import (
	"context"
	"fmt"
	"sync"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// UIFanoutProxy lets callers create chatapp/sessionstream infrastructure before
// the final UI target exists, then install the real fanout once the Bubble Tea
// program has been constructed.
type UIFanoutProxy struct {
	mu     sync.RWMutex
	target sessionstream.UIFanout
}

var _ sessionstream.UIFanout = (*UIFanoutProxy)(nil)

func NewUIFanoutProxy() *UIFanoutProxy { return &UIFanoutProxy{} }

func (p *UIFanoutProxy) SetTarget(target sessionstream.UIFanout) error {
	if target == nil {
		return fmt.Errorf("ui fanout proxy target is nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.target = target
	return nil
}

func (p *UIFanoutProxy) PublishUI(ctx context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	p.mu.RLock()
	target := p.target
	p.mu.RUnlock()
	if target == nil {
		return fmt.Errorf("ui fanout proxy target is not set")
	}
	return target.PublishUI(ctx, sid, ord, events)
}
