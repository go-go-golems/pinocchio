package chatstore

import "context"

// TurnSnapshot captures a serialized turn for inspection.
type TurnSnapshot struct {
	ConvID      string `json:"conv_id"`
	SessionID   string `json:"session_id"`
	TurnID      string `json:"turn_id"`
	Phase       string `json:"phase"`
	RuntimeKey  string `json:"runtime_key,omitempty"`
	InferenceID string `json:"inference_id,omitempty"`
	CreatedAtMs int64  `json:"created_at_ms"`
	Payload     string `json:"payload"`
}

// TurnSaveOptions controls first-class per-turn query columns.
//
// Payload metadata remains canonical, but these fields are projected into
// indexed columns for query/debug surfaces.
type TurnSaveOptions struct {
	RuntimeKey  string
	InferenceID string
}

// TurnQuery describes filters for loading stored turns.
type TurnQuery struct {
	ConvID    string
	SessionID string
	Phase     string
	SinceMs   int64
	Limit     int
}

// TurnStore persists serialized turns for inspection/debugging.
type TurnStore interface {
	Save(ctx context.Context, convID, sessionID, turnID, phase string, createdAtMs int64, payload string, opts TurnSaveOptions) error
	List(ctx context.Context, q TurnQuery) ([]TurnSnapshot, error)
	Close() error
}
