package runtime

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/toolloop"
	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
)

// ConversationRuntimeRequest contains app-owned runtime policy inputs.
type ConversationRuntimeRequest struct {
	ConvID                     string
	ProfileKey                 string
	ProfileVersion             uint64
	ResolvedInferenceSettings  *aisettings.InferenceSettings
	ResolvedProfileRuntime     *ProfileRuntime
	ResolvedProfileFingerprint string
}

// EventSinkWrapper decorates a base event sink with runtime-owned behavior.
// Use this when runtime composition needs to wrap an application-provided sink,
// for example to inject structured-output filtering that belongs to a middleware.
type EventSinkWrapper func(events.EventSink) (events.EventSink, error)

// ComposedRuntime are the composed runtime pieces consumed by conversation lifecycle code.
type ComposedRuntime struct {
	Engine engine.Engine
	// Registry enables Geppetto tool-loop execution for this runtime. When nil,
	// chatapp runs a single-pass inference.
	Registry geptools.ToolRegistry
	// ToolExecutor optionally overrides Geppetto's default tool executor. This is
	// the extension point used by browser/frontend tool bridges.
	ToolExecutor geptools.ToolExecutor
	// WrapSink decorates an application-provided base sink with runtime-owned behavior.
	// Both canonical evtstream chat and legacy webchat now use this to keep sink decoration
	// owned by runtime composition while letting the application provide the transport sink.
	WrapSink           EventSinkWrapper
	RuntimeFingerprint string
	RuntimeKey         string

	// Loop configures the tool loop: the round limit and an optional final
	// answer at the limit. nil uses geppetto's default (20 rounds, no final
	// answer). The final answer's instruction block is the caller's content.
	Loop *toolloop.LoopConfig
	// ToolConfig configures tool execution (timeouts, parallelism, retries).
	// nil uses geppetto's default.
	ToolConfig *geptools.ToolConfig
	// RoundGuard runs before every model call of a run; an error stops the run
	// before that call. Budgets and quotas are implemented here. An error with
	// a Code() string method fails the run with that code.
	RoundGuard toolloop.RoundGuard
}

// RuntimeBuilder composes an engine/sink runtime for a conversation request.
type RuntimeBuilder interface {
	Compose(ctx context.Context, req ConversationRuntimeRequest) (ComposedRuntime, error)
}

// RuntimeBuilderFunc adapts a function to RuntimeBuilder.
type RuntimeBuilderFunc func(ctx context.Context, req ConversationRuntimeRequest) (ComposedRuntime, error)

func (f RuntimeBuilderFunc) Compose(ctx context.Context, req ConversationRuntimeRequest) (ComposedRuntime, error) {
	return f(ctx, req)
}
