package webchat

import (
	"context"
	"testing"

	chatstore "github.com/go-go-golems/pinocchio/pkg/persistence/chatstore"
	"github.com/stretchr/testify/require"
)

func resetTimelineRuntimeForTest(t *testing.T) {
	t.Helper()
	ClearTimelineHandlers()
	resetTimelineHandlerBootstrapForTests()
	t.Cleanup(func() {
		ClearTimelineHandlers()
		resetTimelineHandlerBootstrapForTests()
		RegisterDefaultTimelineHandlers()
	})
}

func TestJSTimelineRuntime_ReducerCreatesEntityAndConsumesEvent(t *testing.T) {
	resetTimelineRuntimeForTest(t)

	runtime := NewJSTimelineRuntime()
	require.NoError(t, runtime.LoadScriptSource("reducers.js", `
const p = require("pinocchio");
p.timeline.registerSemReducer("gepa.progress", function(ev, ctx) {
  return {
    consume: true,
    upserts: [
      {
        id: ev.id + "-progress",
        kind: "gepa.progress",
        props: {
          label: ev.data && ev.data.label,
          nowMs: ctx.now_ms,
        }
      }
    ]
  };
});
`))
	SetTimelineRuntime(runtime)

	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-js-runtime-consume", store, nil)

	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "gepa.progress", "evt-1", 1, map[string]any{
		"label": "building",
	})))

	snap, err := store.GetSnapshot(context.Background(), "conv-js-runtime-consume", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	require.Equal(t, "evt-1-progress", snap.Entities[0].Id)
	require.Equal(t, "gepa.progress", snap.Entities[0].Kind)
	props := snap.Entities[0].GetProps().AsMap()
	require.Equal(t, "building", props["label"])
	require.NotNil(t, props["nowMs"])
}

func TestJSTimelineRuntime_HandlerErrorIsContained(t *testing.T) {
	resetTimelineRuntimeForTest(t)

	runtime := NewJSTimelineRuntime()
	require.NoError(t, runtime.LoadScriptSource("handlers.js", `
const p = require("pnocchio");
var seen = 0;
p.timeline.onSem("*", function(ev) {
  seen = seen + 1;
  if (ev.type === "bad.throw") {
    throw new Error("boom");
  }
});
p.timeline.registerSemReducer("check.seen", function(ev) {
  return {
    consume: true,
    upserts: [{
      id: ev.id,
      kind: "js.seen",
      props: { seen: seen }
    }]
  };
});
`))
	SetTimelineRuntime(runtime)

	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-js-runtime-errors", store, nil)

	// First event throws inside wildcard handler; projection should still continue.
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "bad.throw", "evt-throw", 1, map[string]any{})))
	// Second event checks that runtime remained alive and saw both events.
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "check.seen", "evt-check", 2, map[string]any{})))

	snap, err := store.GetSnapshot(context.Background(), "conv-js-runtime-errors", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 1)
	props := snap.Entities[0].GetProps().AsMap()
	require.Equal(t, float64(2), props["seen"])
}

func TestJSTimelineRuntime_NonConsumingReducerAllowsBuiltinProjection(t *testing.T) {
	resetTimelineRuntimeForTest(t)

	runtime := NewJSTimelineRuntime()
	require.NoError(t, runtime.LoadScriptSource("nonconsume.js", `
const p = require("pinocchio");
p.timeline.registerSemReducer("llm.delta", function(ev) {
  return {
    consume: false,
    upserts: [{
      id: ev.id + "-side",
      kind: "llm.side",
      props: { cumulative: ev.data && ev.data.cumulative }
    }]
  };
});
`))
	SetTimelineRuntime(runtime)

	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-js-runtime-nonconsume", store, nil)

	msgID := "msg-1"
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.start", msgID, 1, map[string]any{
		"id":   msgID,
		"role": "assistant",
	})))
	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "llm.delta", msgID, 2, map[string]any{
		"id":         msgID,
		"delta":      "h",
		"cumulative": "hello",
	})))

	snap, err := store.GetSnapshot(context.Background(), "conv-js-runtime-nonconsume", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 2)

	byID := map[string]string{}
	for _, e := range snap.Entities {
		byID[e.Id] = e.Kind
	}
	require.Equal(t, "message", byID[msgID])
	require.Equal(t, "llm.side", byID[msgID+"-side"])
}

func TestJSTimelineRuntime_ConsumeOnlyReducerDoesNotCreateSyntheticEntity(t *testing.T) {
	resetTimelineRuntimeForTest(t)

	runtime := NewJSTimelineRuntime()
	require.NoError(t, runtime.LoadScriptSource("consume-only.js", `
const p = require("pinocchio");
p.timeline.registerSemReducer("gepa.control", function() {
  return { consume: true };
});
`))
	SetTimelineRuntime(runtime)

	store := chatstore.NewInMemoryTimelineStore(100)
	p := NewTimelineProjector("conv-js-runtime-consume-only", store, nil)

	require.NoError(t, p.ApplySemFrame(context.Background(), semFrame(t, "gepa.control", "evt-control", 1, map[string]any{})))

	snap, err := store.GetSnapshot(context.Background(), "conv-js-runtime-consume-only", 0, 100)
	require.NoError(t, err)
	require.Len(t, snap.Entities, 0, "consume-only reducers must not synthesize timeline entities")
}
