package chatstore

import (
	"context"

	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// ConversationRecord captures persisted conversation-level metadata used for
// historical debug listing and conversation-level joins.
type ConversationRecord struct {
	ConvID          string `json:"conv_id"`
	SessionID       string `json:"session_id"`
	RuntimeKey      string `json:"runtime_key"`
	CreatedAtMs     int64  `json:"created_at_ms"`
	LastActivityMs  int64  `json:"last_activity_ms"`
	LastSeenVersion uint64 `json:"last_seen_version"`
	HasTimeline     bool   `json:"has_timeline"`
	Status          string `json:"status"`
	LastError       string `json:"last_error,omitempty"`
}

// TimelineStore is the durable "actual hydration" projection store.
//
// It stores the canonical timeline entity set for a conversation and supports
// snapshot retrieval by a per-conversation monotonic version.
type TimelineStore interface {
	Upsert(ctx context.Context, convID string, version uint64, entity *timelinepb.TimelineEntityV2) error
	GetSnapshot(ctx context.Context, convID string, sinceVersion uint64, limit int) (*timelinepb.TimelineSnapshotV2, error)
	UpsertConversation(ctx context.Context, record ConversationRecord) error
	GetConversation(ctx context.Context, convID string) (ConversationRecord, bool, error)
	ListConversations(ctx context.Context, limit int, sinceMs int64) ([]ConversationRecord, error)
	Close() error
}
