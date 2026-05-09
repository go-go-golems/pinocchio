---
Title: OpenAI Chat Completions stream reducer refactor
Ticket: PINO-PROTOCOL-CONFORMANCE
Status: active
Topics:
    - geppetto
    - openai
    - chat
    - architecture
    - testing
DocType: design-doc
Intent: implementation
Owners: []
RelatedFiles:
    - Path: ../../../../../../../geppetto/pkg/events/canonical_events.go
      Note: Defines provider, text, and reasoning lifecycle events emitted by the reducer.
    - Path: ../../../../../../../geppetto/pkg/events/canonical_tool_events.go
      Note: Defines tool lifecycle events emitted by the reducer.
    - Path: ../../../../../../../geppetto/pkg/events/correlation_builders.go
      Note: |-
        Builds Chat Completions correlation keys used by the reducer.
        Chat Completions correlation builder used by reducer state helper
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/chat_stream.go
      Note: |-
        Defines normalized Chat Completions stream chunks consumed by the engine.
        Normalized stream chunk type consumed by reducer tests
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/chat_stream_reducer.go
      Note: Reducer state
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go
      Note: Table-driven reducer tests for canonical lifecycle behavior
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/engine_openai.go
      Note: |-
        Current stream loop that interleaves I/O, protocol state, correlation, observability, and canonical event emission.
        Current OpenAI Chat Completions stream loop to refactor into reducer state and effects
    - Path: ../../../../../../../geppetto/pkg/steps/ai/openai/observability.go
      Note: Records provider-native and normalized stream observations.
ExternalSources: []
Summary: Refactor OpenAI-compatible Chat Completions streaming into a small reducer that turns normalized stream inputs plus explicit state into canonical event/effect outputs, with table-driven lifecycle tests.
LastUpdated: 2026-05-08T20:45:00-04:00
WhatFor: Use this before changing `geppetto/pkg/steps/ai/openai/engine_openai.go` so stream lifecycle rules are explicit, testable, and easy to review.
WhenToUse: Use when implementing or reviewing OpenAI Chat Completions stream handling, cancellation/error semantics, tool-call argument accumulation, or provider-normalization conformance tests.
---



# OpenAI Chat Completions stream reducer refactor

## Executive summary

`geppetto/pkg/steps/ai/openai/engine_openai.go` currently does too much in one streaming loop. It reads from the provider stream, updates local protocol state, builds correlation, emits text/reasoning/tool lifecycle events, records observability rows, accumulates usage, and finalizes the conversation turn.

The proposed refactor is small: keep stream I/O in the engine, but move protocol transitions into a reducer.

```text
state + input chunk/terminal -> next state + effects
```

The point is not cleverness. The point is to make the protocol boring. A stream chunk is just data. A terminal input is just data. The reducer owns the lifecycle rules, and table-driven tests feed it short programs such as “text delta, cancel” or “tool args, EOF”.

We are not doing static analysis or model checking for this ticket. We are doing ordinary, deterministic, readable tests around ordinary Go code.

## Goals

1. Make the OpenAI Chat Completions stream lifecycle explicit.
2. Separate provider stream I/O from protocol state transitions.
3. Close active text/reasoning segments on EOF, cancel, and error.
4. Never manufacture text/reasoning segments on terminal events when no segment started.
5. Never emit executable `ToolCallRequested` events on cancel/error.
6. Preserve streamed tool argument accumulation: `Delta` is the fragment, `Arguments` is accumulated state.
7. Keep correlation construction in one obvious place.
8. Add table-driven tests that describe the protocol in examples.

## Non-goals

- No static-analysis implementation.
- No model checker.
- No provider SDK mock framework.
- No new public event vocabulary.
- No frontend or Pinocchio runtime changes in this refactor.

## Current complexity to remove from the engine loop

The current engine loop owns these mutable facts directly:

```go
message
thinkingBuf
toolCallMerger
usageInputTokens
usageOutputTokens
cachedTokens
reasoningTokens
stopReason
currentResponseID
currentChoiceIndex
textSegmentStarted
reasoningSegmentStarted
startedToolStreams
toolArgumentBuffers
toolArgumentSequences
toolCallIDTracker
```

These should become fields of one state object. Then the loop no longer has to remember what it means for a text segment to be open, whether a tool stream has started, or how to finish active work when cancellation arrives.

## Proposed files

```text
geppetto/pkg/steps/ai/openai/chat_stream_reducer.go
geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go
```

The existing `engine_openai.go` remains responsible for:

- request construction;
- opening and closing the provider stream;
- consuming stream chunks through a small named helper;
- completing EOF/cancel/error through one shared terminal helper;
- applying reducer effects;
- final metadata duration and stop reason;
- appending transcript turn blocks;
- persisting the final inference result.

The reducer owns:

- text segment start/delta/finish;
- reasoning segment start/delta/finish;
- tool-call start/arguments/requested lifecycle;
- usage and finish-reason accumulation;
- terminal semantics for EOF/cancel/error;
- Chat Completions correlation construction.

## Types

The reducer should use plain data types.

```go
type openAIChatStreamInputKind string

const (
    openAIChatStreamInputChunk    openAIChatStreamInputKind = "chunk"
    openAIChatStreamInputTerminal openAIChatStreamInputKind = "terminal"
)

type openAIChatTerminalKind string

const (
    openAIChatTerminalEOF       openAIChatTerminalKind = "eof"
    openAIChatTerminalCancelled openAIChatTerminalKind = "cancelled"
    openAIChatTerminalError     openAIChatTerminalKind = "error"
)

type openAIChatStreamInput struct {
    Kind     openAIChatStreamInputKind
    Chunk    chatStreamEvent
    Terminal openAIChatTerminal
}

type openAIChatTerminal struct {
    Kind openAIChatTerminalKind
    Err  error
}
```

The state should contain protocol facts, not I/O objects.

```go
type openAIChatStreamState struct {
    Metadata events.EventMetadata

    Provider         string
    Model            string
    TurnID           string
    ProviderCallCorr events.Correlation

    CurrentResponseID  string
    CurrentChoiceIndex *int

    Message   strings.Builder
    Reasoning strings.Builder

    TextSegmentStarted  bool
    TextSegmentFinished bool

    ReasoningSegmentStarted  bool
    ReasoningSegmentFinished bool

    ToolCallMerger *ToolCallMerger

    StartedToolStreams map[string]bool
    ToolArgBuffers     map[string]string
    ToolArgSequences   map[string]int64
    ToolCallIDTracker  chatToolCallIDTracker

    UsageInputTokens  int
    UsageOutputTokens int
    CachedTokens      int
    ReasoningTokens   int

    StopReason *string

    ChunkCount int
    Done       bool
    Failed     bool
}
```

Effects are what the engine must perform after the pure transition.

```go
type openAIChatStreamEffect struct {
    Event events.Event

    ObserveProviderEvent    *chatStreamEvent
    ObserveNormalizedReason *openAIReasoningNormalizeObservation
}
```

## Core signatures

```go
func newOpenAIChatStreamState(
    metadata events.EventMetadata,
    provider string,
    model string,
    providerCallCorr events.Correlation,
) openAIChatStreamState

func reduceOpenAIChatStream(
    state openAIChatStreamState,
    input openAIChatStreamInput,
) (openAIChatStreamState, []openAIChatStreamEffect)
```

The reducer delegates to small helpers:

```go
func reduceOpenAIChatChunk(state openAIChatStreamState, chunk chatStreamEvent) (openAIChatStreamState, []openAIChatStreamEffect)
func reduceOpenAIChatTerminal(state openAIChatStreamState, terminal openAIChatTerminal) (openAIChatStreamState, []openAIChatStreamEffect)
func reduceOpenAIChatTextDelta(state openAIChatStreamState, chunk chatStreamEvent, effects []openAIChatStreamEffect) (openAIChatStreamState, []openAIChatStreamEffect)
func reduceOpenAIChatReasoningDelta(state openAIChatStreamState, chunk chatStreamEvent, effects []openAIChatStreamEffect) (openAIChatStreamState, []openAIChatStreamEffect)
func reduceOpenAIChatToolDeltas(state openAIChatStreamState, chunk chatStreamEvent, effects []openAIChatStreamEffect) (openAIChatStreamState, []openAIChatStreamEffect)
func reduceOpenAIChatUsageAndFinish(state openAIChatStreamState, chunk chatStreamEvent, effects []openAIChatStreamEffect) (openAIChatStreamState, []openAIChatStreamEffect)
```

## Correlation helper

The current inline closure should become a state method.

```go
func (s openAIChatStreamState) chatCorrelation(
    choiceIndex *int,
    streamKind string,
    toolCallID string,
    toolCallIndex *int,
) events.Correlation
```

This method is the only place where Chat Completions stream coordinates become canonical correlation fields.

## Terminal rules

EOF, cancellation, and errors all close active segments and run through the same completion helper. They differ in classification and in whether tool requests are materialized.

```text
EOF:
  finish open reasoning/text segments;
  emit ToolCallRequested for complete merged tool calls;
  append reasoning/text/tool-call turn blocks;
  finish provider call as completed or tool_calls_pending.

cancel:
  finish open reasoning/text segments with reason cancelled;
  emit interrupt;
  append partial reasoning/text turn blocks;
  do not emit ToolCallRequested;
  do not append executable tool-call blocks;
  finish provider call as cancelled;
  return the partial turn with ctx.Err().

error:
  finish open reasoning/text segments with reason error;
  emit error;
  append partial reasoning/text turn blocks;
  do not emit ToolCallRequested;
  do not append executable tool-call blocks;
  finish provider call as failed;
  return the partial turn with the stream error.
```

The shared invariant is:

```text
A terminal input may close an existing segment, but it must not create one.
```

## Table-driven tests

The reducer tests should read like examples. Each test is a tiny program.

```go
func TestReduceOpenAIChatStream(t *testing.T) {
    tests := []struct {
        name string
        inputs []openAIChatStreamInput

        wantEventTypes []events.EventType
        wantText string
        wantReasoning string
        wantTextClosed bool
        wantReasoningClosed bool
        wantToolCount int
    }{
        {
            name: "text delta then eof closes text",
            inputs: []openAIChatStreamInput{
                chunk(textDelta("hello")),
                terminal(openAIChatTerminalEOF, nil),
            },
            wantEventTypes: []events.EventType{
                events.EventTypeTextSegmentStarted,
                events.EventTypeTextDelta,
                events.EventTypeTextSegmentFinished,
                events.EventTypeProviderCallFinished,
            },
            wantText: "hello",
            wantTextClosed: true,
        },
        {
            name: "cancel closes active text but does not request tools",
            inputs: []openAIChatStreamInput{
                chunk(textDelta("partial")),
                terminal(openAIChatTerminalCancelled, context.Canceled),
            },
            wantEventTypes: []events.EventType{
                events.EventTypeTextSegmentStarted,
                events.EventTypeTextDelta,
                events.EventTypeTextSegmentFinished,
                events.EventTypeInterrupt,
                events.EventTypeProviderCallFinished,
            },
            wantText: "partial",
            wantTextClosed: true,
        },
        {
            name: "tool args accumulate and eof requests tool",
            inputs: []openAIChatStreamInput{
                chunk(toolDelta("call_1", 0, "search", `{"q"`)),
                chunk(toolDelta("call_1", 0, "", `:"x"}`)),
                terminal(openAIChatTerminalEOF, nil),
            },
            wantEventTypes: []events.EventType{
                events.EventTypeToolCallStarted,
                events.EventTypeToolCallArgumentsDelta,
                events.EventTypeToolCallArgumentsDelta,
                events.EventTypeToolCallRequested,
                events.EventTypeProviderCallFinished,
            },
            wantToolCount: 1,
        },
        {
            name: "error closes reasoning and emits error",
            inputs: []openAIChatStreamInput{
                chunk(reasoningDelta("thinking")),
                terminal(openAIChatTerminalError, errors.New("boom")),
            },
            wantEventTypes: []events.EventType{
                events.EventTypeReasoningSegmentStarted,
                events.EventTypeReasoningDelta,
                events.EventTypeReasoningSegmentFinished,
                events.EventTypeError,
                events.EventTypeProviderCallFinished,
            },
            wantReasoning: "thinking",
            wantReasoningClosed: true,
        },
        {
            name: "eof with no content does not manufacture segment",
            inputs: []openAIChatStreamInput{
                terminal(openAIChatTerminalEOF, nil),
            },
            wantEventTypes: []events.EventType{
                events.EventTypeProviderCallFinished,
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            state := newTestOpenAIChatStreamState()
            var got []events.Event

            for _, input := range tt.inputs {
                var effects []openAIChatStreamEffect
                state, effects = reduceOpenAIChatStream(state, input)
                got = append(got, eventsFromEffects(effects)...)
            }

            assertEventTypes(t, got, tt.wantEventTypes)
            assertState(t, state, tt)
        })
    }
}
```

These tests are deliberately small. They are executable documentation.

## Migration plan

1. Add reducer state, input, effect, and terminal types.
2. Move correlation construction into `state.chatCorrelation`.
3. Move chunk handling into `reduceOpenAIChatChunk`.
4. Add table-driven tests for text, reasoning, tools, EOF, cancel, error, and empty EOF.
5. Wire the existing stream loop to call the reducer and apply effects.
6. Move final turn-block construction to read from reducer state.
7. Run package tests and targeted conformance tests.
8. Update the ticket diary and changelog.

## Review checklist

- The stream loop is mostly I/O and effect application.
- The reducer has no network calls.
- Terminal handling is shared for EOF/cancel/error.
- Cancel/error close active segments but do not request tools.
- EOF with no content emits no text/reasoning segment lifecycle.
- Tool argument deltas preserve both fragment and accumulated arguments.
- Correlation keys are stable and include provider-call parent correlation.
- Tests are table-driven and easy to extend.
