package webchat

import "context"

// TurnSnapshot captures a serialized turn for inspection.
type TurnSnapshot struct {
	ConvID      string
	RunID       string
	TurnID      string
	Phase       string
	CreatedAtMs int64
	Payload     string
}

// TurnQuery describes filters for loading stored turns.
type TurnQuery struct {
	ConvID  string
	RunID   string
	Phase   string
	SinceMs int64
	Limit   int
}

// TurnStore persists serialized turns for inspection/debugging.
type TurnStore interface {
	Save(ctx context.Context, convID, runID, turnID, phase string, createdAtMs int64, payload string) error
	List(ctx context.Context, q TurnQuery) ([]TurnSnapshot, error)
	Close() error
}
