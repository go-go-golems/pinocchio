# Chatapp stream patch batching

## Overview

`pkg/chatapp` can reduce canonical stream-patch publication frequency with `WithStreamPatchBatching`:

```go
engine := chatapp.NewEngine(
    chatapp.WithPlugins(
        plugins.NewReasoningPlugin(),
        plugins.NewToolCallPlugin(),
    ),
    chatapp.WithStreamPatchBatching(25*time.Millisecond),
)
```

Batching is disabled by default. A non-positive interval publishes every provider delta as a separate canonical patch.

The batcher applies to append-only patches produced from these Geppetto runtime events:

| Runtime event | Canonical Pinocchio event | Appended field | Logical stream key |
|---|---|---|---|
| `EventTextDelta` | `ChatTextPatch` | `text` | text message ID |
| `EventReasoningDelta` | `ChatReasoningPatch` | `text` | reasoning message ID |
| `EventToolCallArgumentsDelta` | `ChatToolArgumentsPatch` | `arguments` | tool-call ID |

Batching occurs before `Engine.publish`. The canonical backend event, timeline projection, and UI projection therefore observe the same coalesced patch boundaries.

## Fixed-window behavior

This is fixed-window batching, not trailing-edge debouncing.

For each logical stream:

1. The first patch is published immediately.
2. The second patch starts a timer for the configured interval.
3. Further compatible patches arriving before the timer expires are merged into the pending patch.
4. Timer expiry publishes the accumulated patch.
5. The next patch starts a new window.

The timer is not reset for every additional delta. Under continuous generation, this bounds the interval between accumulated patch publications while retaining immediate first output.

With a 25 ms interval, the additional delay for a patch held in the pending window is at most approximately 25 ms, excluding scheduler and publication overhead.

## Merge semantics

Only `CHAT_STREAM_PATCH_MODE_APPEND` patches are merged.

A merged patch preserves:

- the event name and logical stream identity;
- the first pending patch's offset;
- concatenated `text` or `arguments` in provider order;
- the latest patch's sequence;
- the latest patch's correlation metadata.

The first offset must remain unchanged because the merged payload starts where the first pending delta started. The latest sequence identifies the last provider delta represented by the accumulated patch.

Snapshot, replace, unspecified, unsupported, or incompatible payloads are not concatenated. The runtime sink flushes any pending append patch and publishes the new patch directly.

## Ordering and flush rules

A runtime sink owns one pending patch and one timer. It does not run independent timers for every stream. This preserves global provider-event order when reasoning, text, or multiple tool calls interleave.

The sink flushes a pending patch before:

- publishing a non-delta runtime event;
- publishing a lifecycle event such as segment finish or tool-call request;
- publishing the first patch for a different logical stream;
- publishing a non-append patch;
- handling terminal error or interrupt paths.

For example, if a pending reasoning patch is followed by the first tool-argument patch, publication order is:

```text
pending ChatReasoningPatch
first ChatToolArgumentsPatch
```

The tool patch remains immediate for its stream, but it cannot overtake earlier provider output.

## Plugins

Reasoning and tool-call patches originate in chatapp plugins. `RuntimeEventContext.Publish` is routed through the runtime sink so supported plugin patches enter the same batcher as assistant text.

A custom plugin should not assume every call to `RuntimeEventContext.Publish` reaches the backend synchronously. Supported stream patches may be retained until timer expiry or a flush boundary. Lifecycle events still flush earlier patch data before they are published.

Custom plugin payloads are not automatically batchable. Adding another patch class requires:

1. a stable logical stream key;
2. explicit append-mode detection;
3. a merge implementation with documented offset and sequence behavior;
4. runtime-event classification so consecutive deltas do not force premature flushes;
5. protocol tests for timer expiry, lifecycle flush, cross-stream order, and terminal paths.

## Compatibility alias

`WithTextPatchBatching(interval)` remains available for existing callers. It delegates to `WithStreamPatchBatching(interval)` and therefore now enables batching for text, reasoning, and tool-call argument patches when the corresponding plugins are installed.

New integrations should use `WithStreamPatchBatching` because its name reflects current behavior.

## Batching versus compact UI events

Batching and compact UI projection are independent optimizations.

`WithStreamPatchBatching` reduces the number of canonical patches. It affects canonical event boundaries, timeline projection frequency, and UI projection frequency.

`CompactChatTextDeltaTransformer()` reduces the fields delivered by UI events after projection:

- `ChatTextPatch` becomes `ChatTextDelta`;
- `ChatReasoningPatch` becomes `ChatReasoningDelta`;
- `ChatToolArgumentsPatch` becomes `ChatToolArgumentsDelta`.

The transformer does not alter canonical event payloads or timeline entity schemas. It also does not perform batching. Applications can enable either optimization independently or use both:

```go
engine := chatapp.NewEngine(
    chatapp.WithPlugins(features...),
    chatapp.WithStreamPatchBatching(25*time.Millisecond),
    chatapp.WithUIEventTransformers(
        chatapp.CompactChatTextDeltaTransformer(),
    ),
)
```

## Choosing an interval

Choose an interval from measurements rather than assuming one value fits every deployment.

A practical starting range for an interactive browser UI is 25–50 ms. Evaluate:

- time to first visible patch;
- p50 and p95 patch-to-display latency;
- canonical events per generated token or byte;
- WebSocket frames per second;
- timeline writes per second;
- rendering work in the client;
- provider behavior for interleaved tool calls.

A smaller interval lowers accumulation delay but produces more events. A larger interval reduces event and rendering frequency but makes streaming visibly coarser.

## Validation

Run the focused runtime sink tests:

```bash
go test ./pkg/chatapp \
  -run 'TestRuntimeEventSink(Batches|Flushes)' \
  -count=1
```

Run the full repository suite before committing:

```bash
go test ./... -count=1
```

The focused tests cover:

- immediate first text patch;
- text accumulation and timer publication;
- immediate first reasoning patch;
- reasoning accumulation;
- tool-argument accumulation;
- flush before tool-call request;
- flush before text segment finish;
- cross-stream ordering.

For application-level acceptance, capture decoded WebSocket frames and compare the final rendered content with hydrated timeline entities. Batching may change patch boundaries, but it must not change accumulated text, reasoning, tool arguments, lifecycle order, or final snapshots.

## Implementation references

- `pkg/chatapp/chat.go` — configuration and options
- `pkg/chatapp/runtime_inference.go` — interval propagation into the runtime sink
- `pkg/chatapp/runtime_sink.go` — classification, merge, timer, and flush logic
- `pkg/chatapp/features.go` — plugin publication routing
- `pkg/chatapp/runtime_sink_protocol_test.go` — protocol tests
- `pkg/chatapp/ui_event_transformer.go` — separate compact UI projection
