---
Title: Provider event table-driven testing guide
Ticket: PINO-PROTOCOL-CONFORMANCE
Status: active
Topics:
    - geppetto
    - provider-normalization
    - testing
    - chat
    - architecture
DocType: design-doc
Intent: implementation
Owners: []
RelatedFiles:
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/openai/chat_stream_reducer.go
      Note: OpenAI-compatible Chat Completions reducer shape and effects model.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/openai/chat_stream_reducer_test.go
      Note: Existing table-driven reducer tests to use as the first concrete model.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/openai_responses/streaming.go
      Note: OpenAI Responses stream orchestration after setup/consume/complete refactor.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/openai_responses/stream_events.go
      Note: OpenAI Responses provider-native event handler to cover with provider-specific tables.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/openai_responses/stream_state.go
      Note: OpenAI Responses explicit stream state used by completion helpers.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/claude/content-block-merger.go
      Note: Claude reducer-like merger for Anthropic content block stream events.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/claude/content-block-merger_test.go
      Note: Existing Claude table-oriented tests to extend into conformance coverage.
    - Path: ../../../../../../../../geppetto/pkg/steps/ai/gemini/engine_gemini.go
      Note: Gemini stream logic is still inline and should be extracted before deep table testing.
ExternalSources: []
Summary: Reference guide for deriving provider-specific table-driven tests from shared canonical lifecycle scenarios.
LastUpdated: 2026-05-08T23:05:00-04:00
WhatFor: Use this document when writing provider-normalization tests for OpenAI Chat Completions, OpenAI Responses, Claude, and Gemini.
WhenToUse: Use before adding or reviewing table-driven tests that translate provider-native stream events into canonical Geppetto provider/text/reasoning/tool lifecycles.
---

# Provider event table-driven testing guide

## Purpose

The provider adapters do not receive the same native stream data. OpenAI-compatible Chat Completions, OpenAI Responses, Claude, and Gemini all expose different stream grammars. Therefore, the tests should not try to force all providers into one shared input format.

Instead, the tests should use a shared **conformance vocabulary**:

```text
same lifecycle questions
provider-specific native inputs
provider-specific reducer/adapter entry points
shared-ish canonical trace assertions
```

The goal is that each provider has its own table-driven tests, but the rows in those tables are recognizably the same scenarios. This gives us one reference document for planning tests while preserving provider-native realism.

## The pattern

Each provider package should own its native input tables:

```text
OpenAI Chat Completions:
  input: reducer inputs / chat completion stream chunks
  unit under test: chat stream reducer and completion helpers

OpenAI Responses:
  input: Responses SSE event names + provider JSON maps
  unit under test: Responses stream state / event handler / completion helpers

Claude:
  input: []api.StreamingEvent
  unit under test: ContentBlockMerger.Add and final completion helpers if extracted

Gemini:
  input: []*genai.GenerateContentResponse or a small provider-native wrapper
  unit under test: gemini stream reducer once extracted
```

The output should be projected into a compact canonical trace rather than comparing complete event structs. Complete structs include IDs, timestamps, and metadata that are useful in runtime but noisy in conformance tests.

## Canonical trace projection

A test helper can project Geppetto events into a compact shape:

```go
type canonicalTraceEvent struct {
    Type          events.EventType
    SegmentType   string
    StreamKind    string
    Delta         string
    Text          string
    ToolCallID    string
    ToolName      string
    Arguments     string
    StopReason    string
    FinishClass   string
    HasUsage      bool
    Correlated    bool
    CorrelationOK bool
}
```

Not every test needs every field. The important idea is to assert lifecycle behavior without relying on generated IDs.

Recommended helper behavior:

- Preserve event order.
- Record event type.
- For segment events, record segment type and stream kind.
- For text/reasoning deltas, record delta and accumulated text where available.
- For tool events, record tool id/name/arguments when stable.
- For provider-call finished events, record stop reason, finish class, usage presence, and `hasToolCalls`.
- For every canonical event, record whether it implements typed correlation and whether `events.ValidateCanonicalEvent` passes.

Start provider-local if that is faster. Move to `pkg/steps/ai/internal/streamtest` only when duplication becomes annoying.

## Shared invariants

Every provider-specific table should be designed to exercise these canonical invariants.

### Provider-call lifecycle

- A successful provider stream emits exactly one provider-call start and one provider-call finish.
- Provider final/EOF/message-stop events are provider-call lifecycle events, not text lifecycle events.
- Provider-call finish records stop reason and finish class when known.
- Provider-call finish records usage when the provider supplied usage.
- Provider-call finish says whether tool calls are pending.

### Text segment lifecycle

- Text deltas are emitted only when provider-native text exists.
- A text segment starts before the first text delta.
- A text segment finishes only if a text segment was actually active.
- EOF/final/message-stop without text does not manufacture a text segment.
- Accumulated text in deltas is monotonic and matches final segment text.

### Reasoning segment lifecycle

- Reasoning deltas are emitted only when provider-native reasoning exists.
- A reasoning segment starts before the first reasoning delta when the provider has explicit reasoning block/item starts.
- A reasoning segment finishes when the provider closes the reasoning item/block, or when terminal cleanup semantics require closing an active reasoning segment.
- Provider final/EOF alone must not manufacture reasoning.
- Final aggregate summaries should not be double-emitted as normal reasoning deltas unless the provider actually streamed or exposed them as reasoning content.

### Tool lifecycle

- Tool-call started comes before tool-call argument deltas and requested/final executable calls.
- Tool argument deltas accumulate into the current full argument string.
- Tool-call requested is emitted only when the provider has produced a complete executable tool call.
- Cancel/error must not materialize partial tool arguments as executable tool calls or final tool-call turn blocks.
- Tool call IDs used in canonical events come from typed correlation and event fields, not `metadata.Extra`.

### Terminal semantics

- EOF success closes active text/reasoning segments where appropriate.
- Cancel/error closes active text/reasoning safely where implemented.
- Cancel/error returns or records the terminal error while preserving safe partial text/reasoning transcript state.
- Cancel/error does not fabricate provider success or executable tools.

### Correlation

- Every canonical event implements typed correlation where required.
- Required identities are present:
  - all canonical events: `CorrelationKey`;
  - provider-call events: `ProviderCallID`;
  - text/reasoning segment events: `SegmentID` and `SegmentType`;
  - tool lifecycle events: `ToolCallID`.
- Joining and routing identity comes from typed `events.Correlation`, not `metadata.Extra`.
- Provider-native identifiers may appear in correlation as provenance fields, but are not the only downstream identity.

## Scenario matrix

The following scenarios are the shared reference list. Each provider should implement the rows that apply to its native protocol and supported features.

Legend:

- **Required**: should be implemented for this provider.
- **If supported**: implement when the provider/API exposes this behavior in the adapter.
- **N/A**: not meaningful for the current provider path.

| ID | Scenario | OpenAI Chat | OpenAI Responses | Claude | Gemini |
|---|---|---:|---:|---:|---:|
| P01 | Provider-call start and successful finish | Required | Required | Required | Required |
| P02 | Provider metadata update with usage/stop reason | If supported | Required | Required | Required |
| P03 | Provider final/EOF with no content does not create text | Required | Required | Required | Required |
| P04 | Provider stream read error emits error/failure and no success finish unless explicitly failed finish is expected | Required | Required | Required | Required |
| P05 | Context cancellation preserves safe partial state and returns cancellation | Required | Required | Required | Required |
| T01 | Single text delta creates text start/delta/finish | Required | Required | Required | Required |
| T02 | Multiple text deltas accumulate monotonically | Required | Required | Required | Required |
| T03 | Text segment followed by provider final closes exactly once | Required | Required | Required | Required |
| T04 | Empty text provider event is ignored | Required | Required | Required | Required |
| T05 | Terminal done payload repeats streamed text and does not duplicate text | N/A | Required | N/A | N/A |
| T06 | Terminal done payload supplies missing suffix and backfills text | N/A | Required | N/A | N/A |
| R01 | Reasoning text delta creates reasoning lifecycle | If supported | Required | N/A today | If supported later |
| R02 | Reasoning summary delta normalizes and accumulates | N/A | Required | N/A | N/A |
| R03 | Reasoning item/block done appends reasoning turn block | N/A | Required | N/A today | If supported later |
| R04 | Error/cancel with active reasoning closes/preserves partial reasoning safely | If supported | Required | N/A today | If supported later |
| TL01 | Complete tool call emits started/arguments/requested | Required | Required | Required | Required |
| TL02 | Tool argument deltas accumulate full argument string | Required | Required | Required | N/A today: Gemini function calls arrive complete |
| TL03 | Partial tool arguments plus cancel/error do not emit requested/final executable call | Required | Required | Required | If reducer models partial calls later |
| TL04 | Text before tool closes or transitions cleanly | Required | Required | Required | Required |
| TL05 | Multiple tool calls preserve stable order and call identity | Required | Required | Required | Required |
| C01 | Every canonical event has valid typed correlation | Required | Required | Required | Required |
| C02 | Segment correlation identifies text/reasoning/tool separately | Required | Required | Required | Required |
| C03 | Provider-native IDs are preserved as provenance when available | Required | Required | Required | If available |
| F01 | Final turn blocks match canonical stream output | Required | Required | Required | Required |
| F02 | Cancel/error does not append partial tool-call blocks | Required | Required | Required | Required |
| F03 | Inference result persistence matches metadata/finish/tool state | Required | Required | Required | Required |

## Provider-specific table designs

### OpenAI-compatible Chat Completions

Current seam:

```text
pkg/steps/ai/openai/chat_stream_reducer.go
pkg/steps/ai/openai/chat_stream_reducer_test.go
```

This provider is already the model for the rest. It should keep table-driven reducer tests using provider-like reducer inputs and terminal inputs.

Example table shape:

```go
tests := []struct {
    name     string
    inputs   []openAIChatStreamInput
    terminal openAIChatTerminal
    want     []canonicalTraceEvent
}{
    {
        name: "text delta then eof closes text",
        inputs: []openAIChatStreamInput{
            chatTextDelta("hello"),
        },
        terminal: chatEOF(),
        want: []canonicalTraceEvent{
            traceProviderStarted(),
            traceTextStarted(),
            traceTextDelta("hello", "hello"),
            traceTextFinished("hello"),
            traceProviderFinished("", "completed"),
        },
    },
}
```

Priority rows:

- T01/T02/T03 text success.
- P03 EOF without content.
- TL01/TL02 complete tool call.
- TL03 cancel/error with partial tool args.
- R01/R04 for reasoning-capable compatible providers.
- C01/C02 for correlation validation.

### OpenAI Responses

Current seams:

```text
pkg/steps/ai/openai_responses/stream_state.go
pkg/steps/ai/openai_responses/stream_events.go
pkg/steps/ai/openai_responses/streaming.go
```

Responses has the richest native protocol. Tests should use small helper constructors for SSE-like provider events:

```go
type responsesTestEvent struct {
    eventName string
    data      map[string]any
}
```

Example table shape:

```go
tests := []struct {
    name     string
    events   []responsesTestEvent
    terminal responsesStreamTerminal
    want     []canonicalTraceEvent
}{
    {
        name: "completed without output does not create text",
        events: []responsesTestEvent{
            responsesCompleted(),
        },
        terminal: responsesEOF(),
        want: []canonicalTraceEvent{
            traceProviderStarted(),
            traceProviderFinished("", "completed"),
        },
    },
}
```

Priority rows:

- P01/P02/P03 provider lifecycle.
- T01/T05/T06 output text delta/done backfill behavior.
- R01/R02/R03 reasoning text and summary behavior.
- TL01/TL02/TL03 function-call argument lifecycle.
- F01/F02 final turn-block behavior.
- C01/C02/C03 correlation validation.

Implementation note: because `handleResponsesProviderEvent` currently publishes directly through the engine, either capture events with an event sink/observer in tests or introduce a tiny test harness that invokes the handler with a state and records emitted events. Avoid a broad framework until the needed seams are clear.

### Claude

Current seam:

```text
pkg/steps/ai/claude/content-block-merger.go
pkg/steps/ai/claude/content-block-merger_test.go
```

Claude already has a reducer-like object: `ContentBlockMerger`. `Add(event)` accepts one native `api.StreamingEvent`, mutates merger state, and returns canonical Geppetto events.

Example table shape:

```go
tests := []struct {
    name   string
    events []api.StreamingEvent
    want   []canonicalTraceEvent
}{
    {
        name: "text content block",
        events: []api.StreamingEvent{
            claudeMessageStart("msg_1"),
            claudeContentBlockStart(0, api.ContentTypeText),
            claudeTextDelta(0, "hello"),
            claudeContentBlockStop(0),
            claudeMessageStop("end_turn"),
        },
        want: []canonicalTraceEvent{
            traceProviderStarted(),
            traceTextStarted(),
            traceTextDelta("hello", "hello"),
            traceTextFinished("hello"),
            traceProviderFinished("end_turn", "completed"),
        },
    },
}
```

Priority rows:

- P01/P02 provider start, metadata update, stop.
- P03 message start/stop with no content does not create text.
- T01/T02/T03 indexed text content blocks.
- TL01/TL02/TL03 tool_use content block with partial JSON.
- TL04 text block, tool block, later text block ordering.
- C01/C02/C03 correlation validation.

Claude does not need a full rewrite before these tests. Treat `ContentBlockMerger` as the reducer. Later, if desired, rename/wrap it as `claudeStreamState` or extract `consumeClaudeStream`/`completeClaudeStream` helpers around the engine loop.

### Gemini

Current seam is weak:

```text
pkg/steps/ai/gemini/engine_gemini.go
```

Gemini stream state is currently local to `RunInference`. Extract a state/reducer seam before writing deep tests.

Suggested state:

```go
type geminiStreamState struct {
    providerCallCorr events.Correlation
    message string
    chunkCount int
    finalStopReason string
    finalUsage *events.Usage
    textSegmentStarted bool
    textSequence int64
    textCorr events.Correlation
    toolCallIndex int
    pendingCalls []geminiPendingCall
}

type geminiPendingCall struct {
    id string
    name string
    args map[string]any
}
```

Suggested reducer/helper:

```go
func reduceGeminiStreamChunk(
    metadata events.EventMetadata,
    state *geminiStreamState,
    resp *genai.GenerateContentResponse,
) ([]events.Event, error)
```

Completion helper:

```go
func completeGeminiStream(
    t *turns.Turn,
    metadata events.EventMetadata,
    state *geminiStreamState,
    terminal geminiStreamTerminal,
) ([]events.Event, events.EventMetadata, error)
```

Example table shape after extraction:

```go
tests := []struct {
    name     string
    chunks   []*genai.GenerateContentResponse
    terminal geminiStreamTerminal
    want     []canonicalTraceEvent
}{
    {
        name: "single text part",
        chunks: []*genai.GenerateContentResponse{
            geminiTextChunk("hello"),
        },
        terminal: geminiEOF(),
        want: []canonicalTraceEvent{
            traceProviderStarted(),
            traceTextStarted(),
            traceTextDelta("hello", "hello"),
            traceTextFinished("hello"),
            traceProviderFinished("", "completed"),
        },
    },
}
```

Priority rows:

- P01/P02/P03 provider lifecycle, usage, finish reason, empty EOF.
- T01/T02/T03 text parts across one or more chunks.
- TL01 complete function call parts.
- TL05 multiple function call parts preserve order.
- F01 final turn blocks from text and function calls.
- C01/C02 typed correlation validation.

Gemini function calls currently arrive complete, so TL02/TL03 only apply if a future SDK/provider path exposes partial function-call arguments.

## Suggested implementation order

1. **Keep OpenAI Chat Completions as the reference.** It already has reducer tests.
2. **Add OpenAI Responses handler/state tests** for the most important lifecycle rows.
3. **Extend Claude `ContentBlockMerger` tests** using the shared scenario list.
4. **Extract Gemini stream state/reducer.** Keep behavior identical.
5. **Add Gemini reducer tests** using provider-native `genai.GenerateContentResponse` fixtures.
6. **Only then extract a shared test helper** if duplication is obvious.

## What not to do yet

Do not build a large generic provider conformance framework now. It would have to understand too many provider-native grammars and would obscure the purpose of the tests.

Do not force providers to use one artificial input representation. That would make tests less realistic and hide provider-specific edge cases.

Do not compare full event structs unless the test is specifically about exact metadata. Prefer projected canonical traces for lifecycle tests.

## Acceptance checklist for each provider

A provider's Phase 1 test set is acceptable when:

- It uses provider-native fixtures.
- It is table-driven.
- It checks provider-call, text, tool, and available reasoning lifecycle events.
- It checks EOF/final-without-content behavior.
- It checks cancel/error behavior where the reducer seam supports terminal simulation.
- It validates typed correlation on emitted canonical events.
- It asserts final turn blocks and inference metadata when testing completion helpers.
- It is small enough that adding a new provider-native edge case is easy.
