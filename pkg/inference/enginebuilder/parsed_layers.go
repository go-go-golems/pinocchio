package enginebuilder

import (
	"encoding/json"
	"fmt"

	"github.com/go-go-golems/geppetto/pkg/events"
	gebuilder "github.com/go-go-golems/geppetto/pkg/inference/builder"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
)

type simpleEngineConfig struct {
	sig string
}

func (c simpleEngineConfig) Signature() string { return c.sig }

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

func (b *ParsedLayersEngineBuilder) Build(convID, profileSlug string, overrides map[string]any) (engine.Engine, events.EventSink, gebuilder.EngineConfig, error) {
	eng, err := factory.NewEngineFromParsedLayers(b.parsed)
	if err != nil {
		return nil, nil, nil, err
	}
	cfg, err := b.BuildConfig(profileSlug, overrides)
	if err != nil {
		return nil, nil, nil, err
	}
	return eng, b.sink, cfg, nil
}

func (b *ParsedLayersEngineBuilder) BuildConfig(profileSlug string, overrides map[string]any) (gebuilder.EngineConfig, error) {
	var overridesJSON string
	if overrides != nil {
		if b, err := json.Marshal(overrides); err == nil {
			overridesJSON = string(b)
		} else {
			overridesJSON = fmt.Sprintf("<marshal error: %v>", err)
		}
	}
	return simpleEngineConfig{sig: fmt.Sprintf("profile=%s overrides=%s", profileSlug, overridesJSON)}, nil
}

func (b *ParsedLayersEngineBuilder) BuildFromConfig(convID string, config gebuilder.EngineConfig) (engine.Engine, events.EventSink, error) {
	eng, err := factory.NewEngineFromParsedLayers(b.parsed)
	if err != nil {
		return nil, nil, err
	}
	return eng, b.sink, nil
}
