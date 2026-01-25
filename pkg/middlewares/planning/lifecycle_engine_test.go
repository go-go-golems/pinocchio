package planning

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/inference/engine"
	"github.com/go-go-golems/geppetto/pkg/inference/middleware"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureSink struct {
	out []events.Event
}

func (s *captureSink) PublishEvent(ev events.Event) error {
	s.out = append(s.out, ev)
	return nil
}

type fakeEngine struct{}

func (e *fakeEngine) RunInference(_ context.Context, t *turns.Turn) (*turns.Turn, error) {
	if t == nil {
		t = &turns.Turn{}
	}
	// Detect planner prompt marker in system blocks.
	isPlanner := false
	for _, b := range t.Blocks {
		if b.Kind != turns.BlockKindSystem {
			continue
		}
		txt, _ := b.Payload[turns.PayloadKeyText].(string)
		if txt != "" && strings.Contains(txt, "PINOCCHIO_PLANNER_JSON_V1") {
			isPlanner = true
			break
		}
	}
	if isPlanner {
		turns.AppendBlock(t, turns.NewAssistantTextBlock(`{"iterations":[{"iteration_index":1,"action":"respond","reasoning":"ok","strategy":"direct","progress":"ready","tool_name":"","reflection_text":""}],"final_decision":"execute","status_reason":"ok","final_directive":"Answer concisely."}`))
		return t, nil
	}
	turns.AppendBlock(t, turns.NewAssistantTextBlock("hello"))
	return t, nil
}

func TestLifecycleEngine_EmitsPlanningAndExecutionEvents(t *testing.T) {
	base := &fakeEngine{}
	inner := &engineWithMiddleware{base: base, mws: []middleware.Middleware{NewDirectiveMiddleware()}}
	eng := NewLifecycleEngine(inner, DefaultConfig(), "test", "model-x")

	sink := &captureSink{}
	ctx := events.WithEventSinks(context.Background(), sink)

	turn := &turns.Turn{ID: "turn-1"}
	turns.AppendBlock(turn, turns.NewUserTextBlock("hi"))
	_ = turns.KeyTurnMetaSessionID.Set(&turn.Metadata, "sess-1")
	_ = turns.KeyTurnMetaInferenceID.Set(&turn.Metadata, "run-1")
	_ = engine.KeyToolConfig.Set(&turn.Data, engine.ToolConfig{Enabled: false})

	_, err := eng.RunInference(ctx, turn)
	require.NoError(t, err)

	types := make([]string, 0, len(sink.out))
	for _, ev := range sink.out {
		types = append(types, string(ev.Type()))
	}
	assert.Equal(t, []string{
		"planning.start",
		"planning.iteration",
		"planning.complete",
		"execution.start",
		"execution.complete",
	}, types)

	directive, ok, err := KeyDirective.Get(turn.Data)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Answer concisely.", directive)

	// Directive middleware should have injected markers into the first system block.
	hasMarkers := false
	for _, b := range turn.Blocks {
		if b.Kind != turns.BlockKindSystem {
			continue
		}
		txt, _ := b.Payload[turns.PayloadKeyText].(string)
		if strings.Contains(txt, "pinocchio:planning-directive:start") {
			hasMarkers = true
		}
	}
	assert.True(t, hasMarkers)
}

type engineWithMiddleware struct {
	base engine.Engine
	mws  []middleware.Middleware
}

func (e *engineWithMiddleware) RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
	h := func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
		return e.base.RunInference(ctx, t)
	}
	return middleware.Chain(h, e.mws...)(ctx, t)
}
