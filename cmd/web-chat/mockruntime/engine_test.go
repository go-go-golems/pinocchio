package mockruntime

import (
	"context"
	"testing"

	gepevents "github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/turns"
	"github.com/stretchr/testify/require"
)

type captureSink struct{ events []gepevents.Event }

func (s *captureSink) PublishEvent(event gepevents.Event) error {
	s.events = append(s.events, event)
	return nil
}

func TestEngineEmitsDeterministicParityEvents(t *testing.T) {
	sink := &captureSink{}
	ctx := gepevents.WithEventSinks(context.Background(), sink)
	out, err := NewEngine(Options{}).RunInference(ctx, &turns.Turn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotEmpty(t, out.Blocks)

	names := make([]string, 0, len(sink.events))
	for _, ev := range sink.events {
		names = append(names, string(ev.Type()))
	}
	require.Contains(t, names, string(gepevents.EventTypeReasoningSegmentStarted))
	require.Contains(t, names, string(gepevents.EventTypeReasoningDelta))
	require.Contains(t, names, string(gepevents.EventTypeReasoningSegmentFinished))
	require.Contains(t, names, string(gepevents.EventTypeToolCallStarted))
	require.Contains(t, names, string(gepevents.EventTypeToolCallRequested))
	require.Contains(t, names, string(gepevents.EventTypeToolResultReady))
	require.Contains(t, names, string(gepevents.EventTypeTextSegmentStarted))
	require.Contains(t, names, string(gepevents.EventTypeTextDelta))
	require.Contains(t, names, string(gepevents.EventTypeTextSegmentFinished))

	var toolID string
	for _, ev := range sink.events {
		if tool, ok := ev.(*gepevents.EventToolCallStarted); ok {
			toolID = tool.ToolCallID
		}
	}
	require.Equal(t, "mock-backend-tool-1", toolID)
}
