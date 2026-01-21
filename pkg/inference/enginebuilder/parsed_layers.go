package enginebuilder

import (
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
)

// ParsedLayersEngineBuilder is a minimal EngineBuilder implementation for pinocchio
// programs that already have provider selection embedded in ParsedLayers.
//
// It intentionally keeps overrides “opaque” (only used for config signatures) and
// leaves additional middleware composition to the caller.
type ParsedLayersEngineBuilder struct {
	parsed *layers.ParsedLayers
	sink   events.EventSink
}

func NewParsedLayersEngineBuilder(parsed *layers.ParsedLayers, sink events.EventSink) *ParsedLayersEngineBuilder {
	return &ParsedLayersEngineBuilder{
		parsed: parsed,
		sink:   sink,
	}
}

func (b *ParsedLayersEngineBuilder) Build(convID, profileSlug string, overrides map[string]any) (engine.Engine, events.EventSink, error) {
	eng, err := factory.NewEngineFromParsedLayers(b.parsed)
	if err != nil {
		return nil, nil, err
	}
	return eng, b.sink, nil
}
