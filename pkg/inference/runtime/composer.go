package runtime

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
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
	// WrapSink decorates an application-provided base sink with runtime-owned behavior.
	// Both canonical evtstream chat and legacy webchat now use this to keep sink decoration
	// owned by runtime composition while letting the application provide the transport sink.
	WrapSink           EventSinkWrapper
	RuntimeFingerprint string
	RuntimeKey         string

	// SeedSystemPrompt is used to initialize the first seed turn for a new conversation.
	SeedSystemPrompt string
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
