package runtime

import (
	"context"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
)

// RuntimeComposeRequest contains app-owned runtime policy inputs.
type RuntimeComposeRequest struct {
	ConvID          string
	RuntimeKey      string
	ResolvedRuntime *gepprofiles.RuntimeSpec
	Overrides       map[string]any
}

// RuntimeArtifacts are the composed runtime pieces consumed by conversation lifecycle code.
type RuntimeArtifacts struct {
	Engine             engine.Engine
	Sink               events.EventSink
	RuntimeFingerprint string
	RuntimeKey         string

	// SeedSystemPrompt is used to initialize the first seed turn for a new conversation.
	SeedSystemPrompt string
	// AllowedTools controls runtime tool exposure in the inference loop; empty means "all registered tools".
	AllowedTools []string
}

// RuntimeComposer composes an engine/sink runtime for a conversation request.
type RuntimeComposer interface {
	Compose(ctx context.Context, req RuntimeComposeRequest) (RuntimeArtifacts, error)
}

// RuntimeComposerFunc adapts a function to RuntimeComposer.
type RuntimeComposerFunc func(ctx context.Context, req RuntimeComposeRequest) (RuntimeArtifacts, error)

func (f RuntimeComposerFunc) Compose(ctx context.Context, req RuntimeComposeRequest) (RuntimeArtifacts, error) {
	return f(ctx, req)
}
