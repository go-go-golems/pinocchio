package webchat

import (
	"context"
	"errors"

	"github.com/go-go-golems/geppetto/pkg/events"
	timelinepb "github.com/go-go-golems/pinocchio/pkg/sem/pb/proto/sem/timeline"
)

// TimelineEmitter lets a runner upsert durable timeline entities without depending on transport details.
type TimelineEmitter interface {
	Upsert(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error
}

// TimelineEmitterFunc adapts a function to TimelineEmitter.
type TimelineEmitterFunc func(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error

func (f TimelineEmitterFunc) Upsert(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error {
	if f == nil {
		return errors.New("timeline emitter is nil")
	}
	return f(ctx, entity, version)
}

// RunHandle lets callers wait for a started runner-backed process without exposing
// the underlying LLM/session implementation.
type RunHandle interface {
	Wait() error
}

// StartRequest contains the per-conversation surfaces needed by a Runner.
type StartRequest struct {
	ConvID             string
	SessionID          string
	RuntimeKey         string
	RuntimeFingerprint string
	Sink               events.EventSink
	Timeline           TimelineEmitter
	Payload            any
	Metadata           map[string]any
}

// StartResult captures the immediate result of starting a runner-backed process.
type StartResult struct {
	Response map[string]any
	Handle   RunHandle
	RunID    string
	TurnID   string
}

// Runner starts one conversation-backed process.
type Runner interface {
	Start(ctx context.Context, req StartRequest) (StartResult, error)
}

// PrepareRunnerStartInput tells ConversationService which conversation/runtime to ensure before a Runner starts.
type PrepareRunnerStartInput struct {
	Runtime  ConversationRuntimeRequest
	Payload  any
	Metadata map[string]any
}
