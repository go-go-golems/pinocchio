package runtime

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

// ConversationRuntimeRequest contains app-owned runtime policy inputs.
type ConversationRuntimeRequest struct {
	ConvID                     string
	ProfileKey                 string
	ProfileVersion             uint64
	ResolvedProfileRuntime     *gepprofiles.RuntimeSpec
	ResolvedProfileFingerprint string
}

// ComposedRuntime are the composed runtime pieces consumed by conversation lifecycle code.
type ComposedRuntime struct {
	Engine             engine.Engine
	Sink               events.EventSink
	RuntimeFingerprint string
	RuntimeKey         string

	// SeedSystemPrompt is used to initialize the first seed turn for a new conversation.
	SeedSystemPrompt string
	// AllowedTools controls runtime tool exposure in the inference loop; empty means "all registered tools".
	AllowedTools []string
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
