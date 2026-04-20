package evtstream

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ThreeDotsLabs/watermill/message"
)

// OrdinalAssigner owns consumption-time ordinal assignment.
type OrdinalAssigner struct {
	mu      sync.Mutex
	current map[SessionId]uint64
	cursor  func(ctx context.Context, sid SessionId) (uint64, error)
}

func NewOrdinalAssigner(cursor func(ctx context.Context, sid SessionId) (uint64, error)) *OrdinalAssigner {
	return &OrdinalAssigner{
		current: map[SessionId]uint64{},
		cursor:  cursor,
	}
}

func (o *OrdinalAssigner) Next(ctx context.Context, sid SessionId, metadata message.Metadata) (uint64, error) {
	if sid == "" {
		return 0, fmt.Errorf("session id is empty")
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	current, ok := o.current[sid]
	if !ok {
		var err error
		if o.cursor != nil {
			current, err = o.cursor(ctx, sid)
			if err != nil {
				return 0, err
			}
		}
		o.current[sid] = current
	}

	next := current + 1
	if streamID := ExtractStreamID(metadata); streamID != "" {
		if derived, ok := DeriveOrdinalFromStreamID(streamID); ok && derived > current {
			next = derived
		}
	}
	if next <= current {
		next = current + 1
	}
	o.current[sid] = next
	return next, nil
}

// ExtractStreamID finds a stream id from watermill message metadata.
func ExtractStreamID(metadata message.Metadata) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range []string{MetadataKeyStreamID, "xid", "redis_xid"} {
		if v := metadata.Get(key); v != "" {
			return v
		}
	}
	return ""
}

// DeriveOrdinalFromStreamID derives a stable uint64 ordinal from Redis-style stream ids.
func DeriveOrdinalFromStreamID(streamID string) (uint64, bool) {
	parts := strings.Split(streamID, "-")
	if len(parts) != 2 {
		return 0, false
	}
	ms, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	seq, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return ms*1_000_000 + seq, true
}
