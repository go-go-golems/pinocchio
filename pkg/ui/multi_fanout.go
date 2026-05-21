package ui

import (
	"context"
	"fmt"

	sessionstream "github.com/go-go-golems/sessionstream/pkg/sessionstream"
)

// MultiUIFanout publishes each UI event batch to multiple fanouts in order.
// It is used when the command TUI needs to receive live chatapp/sessionstream
// events while the same projected events are also recorded to a debug JSONL file.
type MultiUIFanout struct {
	targets []sessionstream.UIFanout
}

var _ sessionstream.UIFanout = (*MultiUIFanout)(nil)

func NewMultiUIFanout(targets ...sessionstream.UIFanout) (*MultiUIFanout, error) {
	filtered := make([]sessionstream.UIFanout, 0, len(targets))
	for _, target := range targets {
		if target != nil {
			filtered = append(filtered, target)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("multi ui fanout requires at least one target")
	}
	return &MultiUIFanout{targets: filtered}, nil
}

func (f *MultiUIFanout) PublishUI(ctx context.Context, sid sessionstream.SessionId, ord uint64, events []sessionstream.UIEvent) error {
	if f == nil || len(f.targets) == 0 {
		return fmt.Errorf("multi ui fanout is not initialized")
	}
	for i, target := range f.targets {
		if err := target.PublishUI(ctx, sid, ord, events); err != nil {
			return fmt.Errorf("ui fanout target %d: %w", i, err)
		}
	}
	return nil
}
