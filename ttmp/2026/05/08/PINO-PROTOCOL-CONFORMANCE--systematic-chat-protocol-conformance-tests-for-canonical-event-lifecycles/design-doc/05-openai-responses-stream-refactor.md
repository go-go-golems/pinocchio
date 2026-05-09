---
Title: OpenAI Responses stream refactor using the Chat Completions pattern
Ticket: PINO-PROTOCOL-CONFORMANCE
Status: active
Topics:
    - geppetto
    - openai-responses
    - chat
    - architecture
    - testing
DocType: design-doc
Intent: implementation
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/events/canonical_events.go
      Note: Provider, text, and reasoning lifecycle events emitted by Responses streaming.
    - Path: ../../../../../../../geppetto/pkg/events/canonical_tool_events.go
      Note: Tool lifecycle events emitted by Responses streaming.
    - Path: ../../../../../../../geppetto/pkg/events/correlation_builders.go
      Note: Builds Responses correlation keys used by the stream state helper.
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/chat_stream_reducer.go
      Note: |-
        Reference reducer state/effect structure for OpenAI-compatible Chat Completions.
        Reference reducer state/effects and terminal handling pattern
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/engine_openai.go
      Note: |-
        Reference implementation of the adopted consume/complete/reducer pattern for Chat Completions.
        Reference consume/complete shape from Chat Completions refactor
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai_responses/streaming.go
      Note: |-
        Current Responses streaming implementation to reshape around explicit state, consume, and complete helpers.
        Responses streaming implementation to reshape around explicit state
ExternalSources: []
Summary: 'Refactor OpenAI Responses streaming to follow the same visible pattern as Chat Completions: initialize state, consume provider stream, reduce/provider-handle events, complete terminal state, and append/persist final turn data.'
LastUpdated: 2026-05-08T21:40:00-04:00
WhatFor: Use this before changing `geppetto/pkg/steps/ai/openai_responses/streaming.go` so Responses streaming converges on the same structure as Chat Completions.
WhenToUse: Use when implementing or reviewing Responses stream handling, cancellation/error semantics, provider item handling, tool-call accumulation, reasoning persistence, and final metadata persistence.
---


# OpenAI Responses stream refactor using the Chat Completions pattern

## Executive summary

We now have a clearer structure for OpenAI-compatible Chat Completions:

```text
setup request
initialize stream state
consume stream chunks
complete terminal state
append transcript/tool blocks
persist inference metadata
return turn + terminal error if any
```

`openai_responses/streaming.go` should adopt the same shape. The Responses API is more complex than Chat Completions because it has provider-native output items, message items, reasoning items, reasoning summaries, web-search items, and function-call argument deltas. The refactor also removes the old non-streaming runtime path so Responses provider normalization has one canonical lifecycle path. The goal is therefore not to force a tiny reducer too early. The goal is to make the same principle visible:

```text
provider input + explicit Responses state -> canonical effects + final turn data
```

For this step we are explicitly **not** adding broad provider-normalization matrix tests. We are refactoring structure first, preserving behavior, and using existing package tests plus small focused table-driven tests for newly extracted helpers where useful.

## Reference pattern from Chat Completions

The desired high-level shape is close to this:

```go
state := newOpenAIChatStreamState(...)
state, terminal, runErr := e.consumeOpenAIChatStream(ctx, stream, state, metadata, req.Model)
state, metadata = e.completeOpenAIChatStream(ctx, t, state, metadata, req.Model, startTime, terminal)
return t, runErr
```

Responses should read similarly:

```go
state := newResponsesStreamState(metadata, reqBody, providerCallCorr, tap)
state, terminal, runErr := e.consumeResponsesStream(ctx, reader, state)
state, metadata = e.completeResponsesStream(ctx, t, state, metadata, startTime, terminal)
return t, runErr
```

The exact names may differ, but the story should be obvious.

## Goals

1. Make `runStreamingInference` show the protocol shape instead of containing all stream logic inline.
2. Move Responses mutable stream facts into an explicit `responsesStreamState`.
3. Move Responses correlation helpers onto the state.
4. Move final metadata and turn-block appending into named completion helpers.
5. Make cancellation/error run the same completion path as EOF where safe:
   - preserve partial text/reasoning;
   - persist final metadata and duration;
   - close provider-call lifecycle;
   - avoid materializing incomplete function calls as executable tool-call blocks.
6. Keep existing behavior for provider item handling, reasoning summaries, citations, web search, and final tool calls.
7. Use existing package tests as behavior protection, plus small table-driven helper tests if new helper behavior is not already covered.
8. Remove the non-streaming Responses path so cancellation/error/EOF semantics only need to be correct once.

## Non-goals

- No static-analysis implementation.
- No model-checking implementation.
- No broad provider-normalization matrix tests in this step.
- No public event vocabulary changes.
- No schema/protobuf/frontend changes.
- No attempt to make Responses and Chat Completions share one generic reducer type.

## Current complexity to isolate

The current `runStreamingInference` keeps many independent mutable facts as local variables:

```go
message
inputTokens/outputTokens/cachedTokens/reasoningTokens
stopReason
responseCompleted
streamErr
thinkBuf/sayBuf/summaryBuf
currentReasoningText/currentReasoningSummary
currentReasoningItemID/lastReasoningItemID
assistantByItem
currentResponseID
callsByItem/finalCalls
currentReasoningEncryptedContent
currentReasoningOutputIndex/lastReasoningOutputIndex
currentReasoningSummaryIndex/lastReasoningSummaryIndex
currentReasoningStatus
latestMessageItemID/latestMessageOutputIndex/latestMessageStatus
providerCallCorr
```

These should become one explicit state object. The state object does not need to be exported. It only needs to make the protocol legible.

## Proposed files

Initial implementation can keep the code close to the existing package:

```text
geppetto/pkg/steps/ai/openai_responses/streaming.go
geppetto/pkg/steps/ai/openai_responses/stream_state.go        optional
geppetto/pkg/steps/ai/openai_responses/stream_state_test.go   optional
```

If a new file improves clarity, prefer it. If moving code would create noisy diffs, keep helper types in `streaming.go` until the shape stabilizes.

## Proposed types

```go
type responsesStreamTerminalKind string

const (
    responsesStreamTerminalEOF       responsesStreamTerminalKind = "eof"
    responsesStreamTerminalCancelled responsesStreamTerminalKind = "cancelled"
    responsesStreamTerminalError     responsesStreamTerminalKind = "error"
)

type responsesStreamTerminal struct {
    Kind responsesStreamTerminalKind
    Err  error
}
```

State sketch:

```go
type responsesStreamState struct {
    Metadata events.EventMetadata
    ReqBody responsesRequest
    Tap engine.DebugTap

    ProviderCallCorr events.Correlation
    CurrentResponseID string

    Message string
    ThinkingText string
    SayingText string
    SummaryText string

    InputTokens int
    OutputTokens int
    CachedTokens int
    ReasoningTokens int
    StopReason *string
    ResponseCompleted bool

    StreamErr error

    CurrentReasoningItemID string
    LastReasoningItemID string
    CurrentReasoningText strings.Builder
    CurrentReasoningSummary strings.Builder
    CurrentReasoningEncryptedContent string
    CurrentReasoningOutputIndex *int
    LastReasoningOutputIndex *int
    CurrentReasoningSummaryIndex *int
    LastReasoningSummaryIndex *int
    CurrentReasoningStatus string

    LatestMessageItemID string
    LatestMessageOutputIndex *int
    LatestMessageStatus string

    AssistantByItem map[string]string
    CallsByItem map[string]*responsesPendingCall
    FinalCalls []responsesPendingCall
}
```

The concrete implementation can keep `strings.Builder` where existing code benefits from it. The important part is that state is named.

## Correlation methods

Move the inline closures to methods:

```go
func (s responsesStreamState) providerCallCorrelation() events.Correlation
func (s responsesStreamState) segmentCorrelation(itemID string, outputIndex, summaryIndex *int, segmentType string) events.Correlation
func (s responsesStreamState) toolCorrelation(itemID, callID string, outputIndex *int) events.Correlation
```

This mirrors Chat Completions' `state.chatCorrelation(...)` and gives reviewers one place to check parent correlation, provider-call IDs, model, turn ID, and segment IDs.

## Consume helper

The consume helper should own SSE reading and provider-object dispatch:

```go
func (e *Engine) consumeResponsesStream(
    ctx context.Context,
    reader *bufio.Reader,
    state responsesStreamState,
) (responsesStreamState, responsesStreamTerminal)
```

It should:

- read `event:` / `data:` SSE frames;
- flush complete provider objects;
- update `state.CurrentResponseID` from envelopes;
- call a provider-object handler;
- return `cancelled`, `error`, or `eof` terminal.

## Provider-object handler

The current large `switch providerEventType` can first become a method without changing its internal logic much:

```go
func (e *Engine) handleResponsesProviderObject(
    ctx context.Context,
    state *responsesStreamState,
    eventName string,
    raw string,
    obj map[string]any,
) error
```

This is not yet a perfect pure reducer. That is acceptable. Responses is complex, and the first win is getting state and terminal completion explicit. Later, individual cases can be extracted if needed:

```go
handleResponsesOutputItemAdded(...)
handleResponsesReasoningDelta(...)
handleResponsesOutputItemDone(...)
handleResponsesFunctionCallArgumentsDelta(...)
```

## Completion helper

Completion should be shared by EOF, cancellation, and stream error:

```go
func (e *Engine) completeResponsesStream(
    ctx context.Context,
    t *turns.Turn,
    state responsesStreamState,
    metadata events.EventMetadata,
    startTime time.Time,
    terminal responsesStreamTerminal,
) (responsesStreamState, events.EventMetadata)
```

EOF:

```text
persist final metadata;
append assistant text;
append completed function-call blocks;
publish provider-call finished as completed/stream_closed/tool_calls_pending.
```

Cancel:

```text
persist final metadata with stop reason cancelled;
append partial assistant/reasoning text that is already safe;
do not append incomplete function-call blocks;
publish provider-call finished as cancelled;
return partial turn with ctx.Err().
```

Error:

```text
persist final metadata with stop reason error;
append partial assistant/reasoning text that is already safe;
do not append incomplete function-call blocks;
publish provider-call finished as failed;
return partial turn with stream/provider error.
```

The exact handling of reasoning blocks is slightly different from Chat Completions because Responses reasoning blocks are normally appended when `response.output_item.done` arrives, carrying provider item metadata and encrypted content. The completion helper should not manufacture rich reasoning blocks without item metadata. It can, however, preserve already appended reasoning blocks and final metadata.

## Tasks

1. Add this design doc and update ticket tasks/diary/changelog. **Done.**
2. Extract Responses terminal types and `responsesStreamState`. **Done.**
3. Move provider-call, segment, and tool correlation closures onto state methods. **Done for correlation construction; remaining event handling still uses local scratch variables in places.**
4. Extract final metadata update into `finalizeResponsesStreamMetadata`. **Done.**
5. Extract final assistant/tool turn-block appending into `appendResponsesFinalTurnBlocks`. **Done.**
6. Extract provider-call finish classification and inference-result persistence helpers. **Done.**
7. Extract SSE reading into `consumeResponsesSSE` while preserving behavior. **Done.**
8. Extract HTTP stream opening into `openResponsesStream`. **Done.**
9. Extract terminal completion into `completeResponsesStream`. **Done.**
10. Add small table-driven helper tests only for new helper behavior that is not already protected. **Started: suffix backfill and chunk conversion helpers are covered.**
11. Continue moving provider-event cases into named handlers only where readability improves.
12. Continue moving mutable scratch state into `responsesStreamState` carefully, without large unsafe rewrites.
13. Run `go test ./pkg/steps/ai/openai_responses -count=1` and then Geppetto pre-commit/full validation at commit points.
14. Update diary/tasks/changelog and commit each stable checkpoint.

## Review checklist

- `runStreamingInference` reads as setup → state → consume → complete → return.
- Correlation construction lives on state methods.
- Completion is shared by EOF/cancel/error.
- Cancel/error do not append executable tool-call blocks.
- Existing reasoning block persistence remains unchanged for normal provider item completion.
- Existing package tests continue to pass.
- No provider-normalization matrix tests are added in this step.
