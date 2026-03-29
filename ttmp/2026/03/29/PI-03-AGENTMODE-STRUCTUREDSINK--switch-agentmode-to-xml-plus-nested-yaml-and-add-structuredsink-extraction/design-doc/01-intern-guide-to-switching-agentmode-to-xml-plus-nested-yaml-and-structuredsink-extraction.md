---
Title: Intern guide to switching agentmode to XML plus nested YAML and structuredsink extraction
Ticket: PI-03-AGENTMODE-STRUCTUREDSINK
Status: active
Topics:
    - pinocchio
    - structured-sinks
    - webchat
    - yaml
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: geppetto/pkg/events/structuredsink/filtering_sink.go
      Note: Structured streaming extractor lifecycle and sink semantics
    - Path: pinocchio/cmd/web-chat/web/src/sem/registry.ts
      Note: Frontend reducer for agent.mode frames
    - Path: pinocchio/pkg/middlewares/agentmode/middleware.go
      Note: Current middleware responsibilities
    - Path: pinocchio/pkg/middlewares/agentmode/service.go
      Note: Mode resolution and persistence boundary
    - Path: pinocchio/pkg/webchat/router.go
      Note: Web-chat sink composition seam
    - Path: pinocchio/pkg/webchat/sem_translator.go
      Note: Existing SEM translation for agent mode events
    - Path: sanitize/pkg/yaml/sanitize.go
      Note: Reusable YAML sanitization entry point
ExternalSources: []
Summary: Detailed intern guide for migrating Pinocchio agentmode from free-form fenced YAML detection to an XML-like tagged block with nested YAML, adding a structuredsink extractor, integrating sanitize-based YAML repair, and confirming the existing web-chat SEM path.
LastUpdated: 2026-03-29T16:28:55.590850551-04:00
WhatFor: Understand the current agentmode pipeline, design the XML plus nested YAML protocol, add a streaming structured extractor, and wire the result cleanly into Pinocchio web-chat.
WhenToUse: Use when implementing or reviewing the agentmode protocol migration, structuredsink adoption, or web-chat event integration.
---


# Intern guide to switching agentmode to XML plus nested YAML and structuredsink extraction

## Executive Summary

This ticket proposes a focused refactor of Pinocchio's `agentmode` control plane. Today, the middleware injects mode-switch instructions into the prompt, then parses assistant output after inference by scanning newly added LLM text blocks for fenced YAML and unmarshaling it directly with `yaml.Unmarshal`. That behavior lives in [`pinocchio/pkg/middlewares/agentmode/middleware.go`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go). The current approach works, but it has three important drawbacks: it depends on free-form YAML anywhere in assistant text, it duplicates parsing logic that should be reusable, and it cannot participate in Geppetto's structured streaming pipeline.

The recommended design is to move the protocol to an XML-like tagged block with nested YAML. Concretely, the model should emit an outer structured tag compatible with Geppetto filtering sinks, for example `<pinocchio:agent_mode_switch:v1> ... </pinocchio:agent_mode_switch:v1>`, and put the current YAML payload inside that block. The middleware should continue to own policy, mode persistence, and final application of the switch, but it should stop owning ad hoc parsing. Instead, both the middleware and the new structured extractor should rely on a shared parser that strips optional fences, sanitizes the YAML with the reusable `sanitize/pkg/yaml` library, and only then decodes into a typed `ModeSwitchPayload`.

The proposed structured extractor should be added in addition to the middleware, not instead of it. That distinction matters. Geppetto's filtering sink is explicitly a progressive extraction and UX tool, not the authoritative place to commit durable state. The extractor is the right layer for progressive detection, filtering the tagged block out of visible assistant output, and optionally emitting preview or proposal events. The middleware remains the authoritative control point for updating `turns.KeyAgentMode`, calling `Service.RecordModeChange`, and appending the final system notice. In Pinocchio web-chat, the event-sink wrapping seam already exists via `webchat.WithEventSinkWrapper(...)`, and the SEM bridge for `agent.mode` already exists. The ticket therefore needs to document an implementation that plugs the new extractor into that seam and reuses or extends the existing SEM path rather than inventing a parallel one.

## Problem Statement

The user request is specific:

1. Update `agentmode` to use an XML approach with nested YAML.
2. Provide a `structuredsink` in addition to the middleware.
3. Update the YAML parsing to use the `sanitize/` path.
4. Integrate the resulting structured stream into Pinocchio web-chat SEM if it is not already integrated.
5. Produce a detailed implementation guide for a new intern and document the source material used to reach the design.

The current codebase partially supports the desired behavior but stops short of the requested architecture.

Observed current behavior:

1. `agentmode.NewMiddleware(...)` resolves the current mode, injects a user block with the current mode prompt and YAML instructions, then after inference scans newly added LLM blocks for YAML fences and parses them with `yaml.Unmarshal` in `DetectYamlModeSwitchInBlocks(...)` and `DetectYamlModeSwitch(...)`.
2. The injected prompt format is not a Geppetto structured block. It is ordinary user text that contains a fenced YAML example from `BuildYamlModeSwitchInstructions(...)`.
3. Pinocchio web-chat already knows how to wrap the default event sink with `WithEventSinkWrapper(...)`, which is the exact seam needed for `structuredsink.NewFilteringSink(...)`.
4. Pinocchio web-chat already translates `EventAgentModeSwitch` into SEM `agent.mode` frames and the React reducer already turns `agent.mode` into an `agent_mode` entity.
5. Geppetto already provides a general structured-sink abstraction designed for tagged streaming extraction, plus documentation and examples showing how to wrap a Watermill sink with `NewFilteringSink(...)`.
6. The reusable YAML sanitizer exists in the sibling `sanitize` repository and exposes a documented Go API in `sanitize/pkg/yaml`.

The design problem is therefore not "invent a brand-new subsystem." The real problem is to remove protocol ambiguity and duplicated parsing by putting `agentmode` onto the same structured streaming rails the rest of the Geppetto stack already provides.

## Scope

In scope:

- Change the `agentmode` prompt and parse contract from free-form fenced YAML to an XML-like structured block with nested YAML.
- Add a shared parser and sanitizer-backed decode path for `agentmode`.
- Add a structured extractor for agent mode switch blocks.
- Integrate that extractor into at least Pinocchio web-chat and document where to integrate it in the simple chat agent.
- Reuse or minimally extend the SEM path so structured mode events appear in web-chat cleanly.
- Update documentation and tests.

Out of scope:

- Redesigning the entire agent mode service model.
- Building a general-purpose XML parser for all middleware protocols.
- Changing unrelated middleware composition behavior.
- Changing the fundamental meaning of `agent_mode` in turn data.

## Terminology and Concepts

This section exists because the requested change crosses several abstractions, and a new intern will get lost if those abstractions are not named clearly.

### Middleware

In this codebase, a middleware is a function around `RunInference(ctx, *turns.Turn)` that can inspect and modify the turn before inference, and inspect and modify the returned turn after inference. `agentmode` is one such middleware. It owns policy decisions like "what mode am I in?" and "should I apply a mode change now?".

### Structured Sink

`structuredsink.FilteringSink` is an `events.EventSink` wrapper that watches streaming LLM text events, recognizes special tagged blocks, removes those blocks from user-visible text, and dispatches their payload bytes to extractor sessions. It lives in [`geppetto/pkg/events/structuredsink/filtering_sink.go`](../../../../../../geppetto/pkg/events/structuredsink/filtering_sink.go). The extractor lifecycle is `OnStart`, `OnRaw`, and `OnCompleted`.

### SEM

SEM is Pinocchio's semantic event layer for web-chat. Backend event types are translated into normalized stream frames, then the frontend reducer converts those frames into timeline entities. For agent mode, the backend currently maps `EventAgentModeSwitch` to the `agent.mode` frame type, and the frontend reducer turns that into an `agent_mode` entity.

### XML-Like Structured Tag

Geppetto's filtering sink is built around tags that look like XML but are really a small structured envelope format:

```text
<package:type:version>
payload bytes here
</package:type:version>
```

This is the "XML approach" that fits the existing structured streaming stack. It is not full arbitrary XML. It is a recognized tag protocol that is already compatible with `FilteringSink`.

### Nested YAML

Inside the structured tag, we still want YAML because the current middleware contract and agentmode payload shape are already YAML-friendly. The key change is that the YAML becomes the payload of a structured block rather than a free-form fence floating somewhere in assistant prose.

## Documentation and Reference Material Reviewed

This ticket should explicitly record the documentation used to derive the plan. These are the most important sources.

### Geppetto documentation

1. [`geppetto/pkg/doc/topics/11-structured-sinks.md`](../../../../../../geppetto/pkg/doc/topics/11-structured-sinks.md)
   - Main structured-sink API reference.
   - Documents `Extractor`, `ExtractorSession`, `OnStart`, `OnRaw`, and `OnCompleted`.
   - Includes a mode-switch-flavored example extractor later in the document.
2. [`geppetto/pkg/doc/tutorials/04-structured-data-extraction.md`](../../../../../../geppetto/pkg/doc/tutorials/04-structured-data-extraction.md)
   - Walks through sink creation, extractor registration, and runtime wiring.
3. [`geppetto/pkg/doc/playbooks/03-progressive-structured-data.md`](../../../../../../geppetto/pkg/doc/playbooks/03-progressive-structured-data.md)
   - Shows the exact `NewFilteringSink(...)` wrapping pattern around `middleware.NewWatermillSink(...)`.
4. [`geppetto/pkg/doc/topics/04-events.md`](../../../../../../geppetto/pkg/doc/topics/04-events.md)
   - Background for event sinks and event routing.
5. [`geppetto/pkg/doc/topics/09-middlewares.md`](../../../../../../geppetto/pkg/doc/topics/09-middlewares.md)
   - Middleware composition background.

### Pinocchio documentation

1. [`pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md`](../../../../../../pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md)
   - Uses `agentmode` as the motivating example for middleware + event renderer integration.
2. [`pinocchio/pkg/doc/topics/webchat-sem-and-ui.md`](../../../../../../pinocchio/pkg/doc/topics/webchat-sem-and-ui.md)
   - Documents the `agent.mode` SEM frame already used by web-chat.
3. [`pinocchio/pkg/doc/topics/webchat-profile-registry.md`](../../../../../../pinocchio/pkg/doc/topics/webchat-profile-registry.md)
   - Documents that `agentmode` is an app-owned runtime middleware used via profile runtime configuration.

### Sanitize documentation

1. [`sanitize/README.md`](../../../../../../sanitize/README.md)
   - Documents the public Go library usage for `sanitize/pkg/yaml`.
2. `sanitize/pkg/yaml/*`
   - Code-level API reference for `Sanitize(...)`, `SanitizeWithOptions(...)`, and available options.

### Existing ticket and design material discovered during investigation

These are not the canonical product docs, but they capture prior thinking in this repo and are useful context:

1. [`geppetto/ttmp/2026/03/28/GP-59-YAML-SANITIZATION--add-yaml-sanitization-to-streaming-structured-event-extractions/design-doc/01-intern-guide-to-adding-optional-by-default-yaml-sanitization-to-streaming-structured-event-extractions.md`](../../../../../../geppetto/ttmp/2026/03/28/GP-59-YAML-SANITIZATION--add-yaml-sanitization-to-streaming-structured-event-extractions/design-doc/01-intern-guide-to-adding-optional-by-default-yaml-sanitization-to-streaming-structured-event-extractions.md)
2. [`pinocchio/ttmp/2025-08-21/01-building-a-simple-self-contained-web-agent-with-timeline-and-redis-and-how-to-tackle-the-next-steps.md`](../../../../../../pinocchio/ttmp/2025-08-21/01-building-a-simple-self-contained-web-agent-with-timeline-and-redis-and-how-to-tackle-the-next-steps.md)
3. [`pinocchio/ttmp/2025-08-22/02-backend-semantic-event-mapping.md`](../../../../../../pinocchio/ttmp/2025-08-22/02-backend-semantic-event-mapping.md)

These prior docs are informative, but the concrete code paths listed later in this document are the primary evidence.

## Current-State Architecture

### 1. Current agentmode middleware behavior

The current implementation is in [`pinocchio/pkg/middlewares/agentmode/middleware.go`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go).

Observed responsibilities:

1. Determine the current mode from `turns.KeyAgentMode`, service storage, or `Config.DefaultMode`.
2. Resolve the mode definition via `Service.GetMode(...)`.
3. Remove previously inserted agentmode-related prompt blocks.
4. Build a user block containing the current mode prompt and YAML switch instructions.
5. Insert that block into the turn.
6. Save `AllowedTools` into `turns.KeyAgentModeAllowedTools`.
7. Run the next handler.
8. Scan newly added assistant LLM blocks for fenced YAML.
9. If a new mode is found, update `turns.KeyAgentMode`, call `Service.RecordModeChange(...)`, append a system block, and emit `EventAgentModeSwitch`.

Important evidence:

- Current mode resolution and defaulting: [`middleware.go:83`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go#L83)
- Prompt injection: [`middleware.go:130`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go#L130)
- Allowed-tools handoff: [`middleware.go:176`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go#L176)
- Post-inference YAML scan and state mutation: [`middleware.go:195`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go#L195)
- YAML instruction builder: [`middleware.go:223`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go#L223)
- Raw fenced-YAML detectors: [`middleware.go:252`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go#L252)

The middleware currently uses `parse.ExtractYAMLBlocks(...)` from [`geppetto/pkg/steps/parse/yaml_blocks.go`](../../../../../../geppetto/pkg/steps/parse/yaml_blocks.go) and then directly calls `yaml.Unmarshal` on the extracted fence body. This means:

- it only sees YAML after `next()` returns,
- it does not sanitize malformed YAML,
- it cannot progressively stream parsed mode information,
- it depends on free-form fences anywhere in assistant text.

### 2. Current agentmode service model

The service contract is in [`pinocchio/pkg/middlewares/agentmode/service.go`](../../../../../../pinocchio/pkg/middlewares/agentmode/service.go).

Important observations:

1. `Service` owns mode lookup and mode change persistence.
2. `StaticService` stores current mode per session in memory.
3. `SQLiteService` delegates persistence to `SQLiteStore`.

This matters because it clarifies the right ownership boundary. Structured extraction should not replace the service. The service remains the durable source of mode changes.

### 3. Existing structuredsink behavior in Geppetto

The structured sink abstraction is defined in [`geppetto/pkg/events/structuredsink/filtering_sink.go`](../../../../../../geppetto/pkg/events/structuredsink/filtering_sink.go).

Key properties:

1. It is an `events.EventSink` wrapper, not a middleware.
2. It watches `EventPartialCompletion` and `EventFinal`.
3. It recognizes structured tagged blocks and starts extractor sessions for them.
4. It can filter recognized blocks out of visible assistant output.
5. It is designed for progressive extraction and telemetry rather than authoritative persistence.

The extractor contract is documented and implemented as:

```go
type Extractor interface {
    TagPackage() string
    TagType() string
    TagVersion() string
    NewSession(ctx context.Context, meta events.EventMetadata, itemID string) ExtractorSession
}

type ExtractorSession interface {
    OnStart(ctx context.Context) []events.Event
    OnRaw(ctx context.Context, chunk []byte) []events.Event
    OnCompleted(ctx context.Context, raw []byte, success bool, err error) []events.Event
}
```

See [`filtering_sink.go:48`](../../../../../../geppetto/pkg/events/structuredsink/filtering_sink.go#L48) and [`11-structured-sinks.md:96`](../../../../../../geppetto/pkg/doc/topics/11-structured-sinks.md#L96).

The intended lifecycle is explicitly documented in [`11-structured-sinks.md:107`](../../../../../../geppetto/pkg/doc/topics/11-structured-sinks.md#L107).

### 4. Existing web-chat sink integration seam

Pinocchio web-chat already has the exact seam needed for this work.

Evidence:

1. `WithEventSinkWrapper(...)` exists in [`pinocchio/pkg/webchat/router_options.go:119`](../../../../../../pinocchio/pkg/webchat/router_options.go#L119).
2. The router composes a default Watermill sink when none is provided, then applies the wrapper before returning runtime artifacts in [`pinocchio/pkg/webchat/router.go:302`](../../../../../../pinocchio/pkg/webchat/router.go#L302).

That means web-chat does not need a new architectural escape hatch. It already supports wrapping the default chat event sink with a filtering sink.

### 5. Existing SEM integration for agent mode

This is important because the user asked for SEM integration "if it isn't already."

The relevant answer is: the base SEM path for agent mode already exists.

Evidence:

1. `EventAgentModeSwitch` is defined in [`geppetto/pkg/events/chat-events.go:868`](../../../../../../geppetto/pkg/events/chat-events.go#L868).
2. Pinocchio web-chat maps `EventAgentModeSwitch` to an `agent.mode` SEM frame in [`pinocchio/pkg/webchat/sem_translator.go:489`](../../../../../../pinocchio/pkg/webchat/sem_translator.go#L489).
3. The frontend reducer already maps `agent.mode` into an `agent_mode` entity in [`pinocchio/cmd/web-chat/web/src/sem/registry.ts:230`](../../../../../../pinocchio/cmd/web-chat/web/src/sem/registry.ts#L230).
4. The docs already list `agent.mode` as a supported UI/system event in [`pinocchio/pkg/doc/topics/webchat-sem-and-ui.md:68`](../../../../../../pinocchio/pkg/doc/topics/webchat-sem-and-ui.md#L68).

So the correct requirement refinement is:

- If the new structured extractor emits `EventAgentModeSwitch`, the current SEM bridge already covers it.
- If the new structured extractor emits a new event type, the SEM translator must be extended, but the frontend can probably still reuse the existing `agent.mode` frame and `agent_mode` entity shape.

### 6. Existing sanitize library

The YAML sanitization API already exists in the sibling `sanitize` repository:

- Library usage documented at [`sanitize/README.md:65`](../../../../../../sanitize/README.md#L65)
- `Sanitize(...)` and `SanitizeWithOptions(...)` implemented in [`sanitize/pkg/yaml/sanitize.go`](../../../../../../sanitize/pkg/yaml/sanitize.go)
- Option surface implemented in [`sanitize/pkg/yaml/options.go`](../../../../../../sanitize/pkg/yaml/options.go)

The important takeaway is that this ticket should not invent another YAML repair layer in Pinocchio or Geppetto. It should reuse this package.

## Gap Analysis

The current state and the requested state do not line up in several concrete ways.

### Gap 1: Current protocol is free-form fenced YAML, not a structured XML-like block

Today, the model is asked to emit:

```yaml
mode_switch:
  analysis: |
    ...
  new_mode: MODE_NAME
```

inside ordinary prompt text. That format is human-readable but structurally weak. It can appear anywhere in the assistant answer, which means:

- the middleware has to scan arbitrary markdown,
- the block shows up in user-visible output unless something else strips it,
- there is no extractor lifecycle.

### Gap 2: Current YAML parse path is duplicated and unsanitized

Both detector functions in `agentmode` go directly from extracted fence body to `yaml.Unmarshal`. That is the exact behavior the user asked to improve.

### Gap 3: There is no agentmode-specific structured extractor today

Geppetto has generic infrastructure and docs, but Pinocchio does not currently ship a concrete `agentmode` extractor registered on a filtering sink.

### Gap 4: Web-chat integration seam exists, but it is not currently used for structured agentmode extraction

The router can wrap sinks, but the default `cmd/web-chat` command does not appear to install an `agentmode` filtering sink today.

### Gap 5: Existing SEM integration covers final switch events, not necessarily streaming proposals

The current `EventAgentModeSwitch` path is enough for a final switch card, but it does not automatically cover a distinct preview/proposal event if we choose to add one.

## Design Goals

The design should satisfy these goals simultaneously:

1. Make the protocol machine-readable and compatible with `FilteringSink`.
2. Keep the middleware as the authoritative owner of mode application and persistence.
3. Avoid duplicate parsing logic by introducing a shared parser.
4. Reuse `sanitize/pkg/yaml` rather than adding local YAML cleanup helpers.
5. Preserve the existing web-chat `agent.mode` widget path where possible.
6. Be straightforward enough that a new intern can implement it one layer at a time without guessing which abstraction owns what.

## Proposed Solution

### High-level design

Use an XML-like structured tag for the outer envelope and keep YAML as the inner payload.

Recommended on-wire shape:

```text
<pinocchio:agent_mode_switch:v1>
```yaml
mode_switch:
  analysis: |
    The user is asking for regex review rather than regex creation.
  new_mode: category_regexp_reviewer
```
</pinocchio:agent_mode_switch:v1>
```

This shape gives us four things immediately:

1. It is compatible with Geppetto `FilteringSink`.
2. It keeps the current YAML payload shape mostly intact.
3. It allows progressive extraction during stream time.
4. It lets the sink hide the control payload from the visible assistant message.

### Shared parser module

Create a shared agentmode parser package or file set under `pinocchio/pkg/middlewares/agentmode`, for example:

```text
pinocchio/pkg/middlewares/agentmode/
  protocol.go
  parser.go
  structured_extractor.go
```

Recommended responsibilities:

1. Constants for package/type/version and metadata keys.
2. Prompt-building helper that emits XML-like structured instructions.
3. Payload type definitions.
4. A single YAML decode function that:
   - strips optional markdown fences,
   - sanitizes YAML with `sanitize/pkg/yaml`,
   - unmarshals the sanitized YAML into a typed struct,
   - validates required fields,
   - returns a typed proposal.

Suggested API sketch:

```go
const (
    TagPackage = "pinocchio"
    TagType    = "agent_mode_switch"
    TagVersion = "v1"
)

type ModeSwitchPayload struct {
    ModeSwitch struct {
        Analysis string `yaml:"analysis"`
        NewMode  string `yaml:"new_mode,omitempty"`
    } `yaml:"mode_switch"`
}

type ParsedModeSwitch struct {
    Analysis      string
    NewMode       string
    RawYAML       string
    SanitizedYAML string
    ParseClean    bool
}

func BuildStructuredModeSwitchInstructions(current string, available []string) string
func ParseModeSwitchPayload(raw []byte) (*ParsedModeSwitch, error)
func ParseModeSwitchFromMarkdownText(markdown string) (*ParsedModeSwitch, error)
```

### Sanitization rule

The intern should follow this exact sequence:

1. Extract the raw payload bytes inside the structured block.
2. Strip outer markdown fences if the model included them.
3. Pass the body string to `yamlsanitize.Sanitize(...)`.
4. Unmarshal the sanitized text with `yaml.Unmarshal`.
5. Validate semantic requirements:
   - `analysis` may be present without `new_mode`,
   - `new_mode` should be optional,
   - blank `analysis` should be treated as no-op,
   - `new_mode` should be trimmed.

Important note: `yamlsanitize.Sanitize(...)` returns a `Result`, not an error-only API. If `SanitizeWithOptions(...)` is preferred, that is also valid, especially if the implementation wants configuration validation failures to be explicit.

### Middleware responsibilities after the refactor

After the refactor, the middleware should still own:

1. resolving current mode,
2. injecting prompt instructions,
3. applying the mode change to the returned turn,
4. persisting the change through the service,
5. emitting the final authoritative switch event.

The middleware should stop owning:

1. ad hoc YAML fence discovery from arbitrary assistant prose,
2. direct `yaml.Unmarshal`,
3. protocol-specific parsing details that the extractor will also need.

Recommended middleware behavior:

1. Replace `BuildYamlModeSwitchInstructions(...)` with a structured-tag-aware instruction builder.
2. Replace `DetectYamlModeSwitchInBlocks(...)` with a shared parser that looks for the new structured block, not arbitrary fenced YAML.
3. Parse through the shared sanitize-backed decode path.
4. Keep post-inference application as the authoritative step.

### Structured extractor responsibilities

Add a new `structuredsink.Extractor` owned by `agentmode`, for example `ModeSwitchExtractor`.

Recommended behavior:

1. Register for `pinocchio:agent_mode_switch:v1`.
2. On `OnRaw`, optionally debounce or accumulate bytes but avoid heavy per-character work.
3. On `OnCompleted`, parse the full payload with the same shared sanitize-backed parser.
4. Emit either:
   - a new preview/proposal event type, or
   - an existing `EventAgentModeSwitch` only if the application is explicitly choosing sink-driven final events.

Recommended design decision:

- Keep the middleware's `EventAgentModeSwitch` as the authoritative final event.
- Let the extractor emit a distinct proposal or preview event type if progressive UI is desired.

Why this is recommended:

1. It avoids duplicate "mode switched" cards if both the sink and middleware are active.
2. It preserves the semantic difference between "the model proposed a switch" and "the application applied a switch."
3. It matches the structured-sink guidance that streaming extraction is not the durable source of truth.

Suggested event shape for previews:

```go
type EventAgentModeProposal struct {
    events.EventImpl
    Message string                 `json:"message"`
    Data    map[string]interface{} `json:"data,omitempty"`
}
```

Suggested `Data` payload:

```json
{
  "from": "financial_analyst",
  "to": "category_regexp_reviewer",
  "analysis": "...",
  "phase": "proposal",
  "source": "structuredsink",
  "parse_clean": true
}
```

If the team prefers not to add a new event type right now, the extractor can emit nothing user-facing and serve only as a filtering plus future-extensibility hook. That is still a valid ticket outcome, but the richer option is to introduce a distinct proposal event.

### SEM integration strategy

There are two viable SEM strategies.

#### Strategy A: Reuse existing `EventAgentModeSwitch`

If the sink emits the existing `EventAgentModeSwitch`, no new SEM translator work is needed because:

- [`pinocchio/pkg/webchat/sem_translator.go:489`](../../../../../../pinocchio/pkg/webchat/sem_translator.go#L489) already maps that event to `agent.mode`.
- [`pinocchio/cmd/web-chat/web/src/sem/registry.ts:230`](../../../../../../pinocchio/cmd/web-chat/web/src/sem/registry.ts#L230) already reduces `agent.mode`.

Downside:

- sink events and middleware events can duplicate,
- "proposal" and "applied switch" become semantically blurry.

#### Strategy B: Add `EventAgentModeProposal` but still reuse `agent.mode`

This is the recommended path.

Implementation:

1. Add a new backend event type for proposal/preview.
2. Extend `sem_translator.go` to map both `EventAgentModeProposal` and `EventAgentModeSwitch` to the same `agent.mode` SEM frame.
3. Put semantic flags like `phase`, `source`, or `applied` into `data`.
4. Keep the frontend reducer unchanged if it already forwards the generic `data` object.

This gives the UI enough information to differentiate a proposal card from a committed switch card without requiring a brand-new frontend entity kind.

## Proposed Architecture and Flow

### Flow 1: Prompt injection

```text
user turn
  -> agentmode middleware resolves current mode
  -> middleware injects current-mode prompt + structured protocol instructions
  -> provider sees clear request to emit <pinocchio:agent_mode_switch:v1>...</...>
```

### Flow 2: Streaming extraction

```text
provider partial text
  -> Watermill sink
  -> FilteringSink
  -> ModeSwitchExtractor session
       - accumulates payload bytes
       - optional preview parse
       - final sanitize + decode on completion
  -> downstream sink receives filtered visible text
```

### Flow 3: Authoritative mode application

```text
next() returns final turn
  -> agentmode middleware scans newly added blocks for the structured tag
  -> shared parser sanitize + decode
  -> if valid new_mode != current mode:
       - turns.KeyAgentMode.Set(...)
       - Service.RecordModeChange(...)
       - append system block
       - emit EventAgentModeSwitch
```

### Diagram

```text
                          +---------------------------+
                          |  agentmode middleware     |
                          |  - current mode lookup    |
user turn --------------> |  - prompt injection       | ----+
                          |  - final apply/persist    |     |
                          +---------------------------+     |
                                                            v
                                                    +---------------+
                                                    |  LLM provider  |
                                                    +---------------+
                                                            |
                                                            v
                                              partial/final text events
                                                            |
                                                            v
                     +------------------------------------------------------+
                     | web-chat / app sink chain                            |
                     |  WatermillSink <- wrapped by FilteringSink           |
                     +------------------------------------------------------+
                                       |                    |
                                       | filtered text      | structured payload bytes
                                       v                    v
                               visible assistant UI   agentmode extractor
                                                           |
                                                           v
                                              proposal/preview event(s)
                                                           |
                                                           v
                                                      SEM translator
                                                           |
                                                           v
                                                      web-chat UI
```

## API and File-Level Design

### Files that should change

#### Pinocchio middleware package

1. [`pinocchio/pkg/middlewares/agentmode/middleware.go`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go)
   - swap instruction builder,
   - replace free-form YAML detection,
   - call shared parser,
   - optionally stop exporting raw YAML detection helpers.
2. New file: `pinocchio/pkg/middlewares/agentmode/protocol.go`
   - protocol constants,
   - prompt builder,
   - payload types.
3. New file: `pinocchio/pkg/middlewares/agentmode/parser.go`
   - strip fences,
   - sanitize,
   - unmarshal,
   - validate.
4. New file: `pinocchio/pkg/middlewares/agentmode/structured_extractor.go`
   - extractor and extractor session.
5. Potential new file: `pinocchio/pkg/middlewares/agentmode/events.go`
   - if a proposal event type is added in Pinocchio or if Geppetto owns the event type, adjust accordingly.

#### Pinocchio web-chat

1. [`pinocchio/pkg/webchat/router_options.go`](../../../../../../pinocchio/pkg/webchat/router_options.go)
   - no API change required if current wrapper API is sufficient; document usage.
2. [`pinocchio/pkg/webchat/router.go`](../../../../../../pinocchio/pkg/webchat/router.go)
   - no API change required if the caller injects the wrapper; potentially no code change.
3. [`pinocchio/cmd/web-chat/main.go`](../../../../../../pinocchio/cmd/web-chat/main.go)
   - install an `eventSinkWrapper` that wraps the default sink with `structuredsink.NewFilteringSink(...)`.
4. [`pinocchio/pkg/webchat/sem_translator.go`](../../../../../../pinocchio/pkg/webchat/sem_translator.go)
   - update only if a new event type is introduced.
5. [`pinocchio/cmd/web-chat/web/src/sem/registry.ts`](../../../../../../pinocchio/cmd/web-chat/web/src/sem/registry.ts)
   - likely no change if the frame remains `agent.mode`.

#### Geppetto and sanitize dependencies

1. [`geppetto/pkg/events/structuredsink/filtering_sink.go`](../../../../../../geppetto/pkg/events/structuredsink/filtering_sink.go)
   - likely no code change needed.
2. [`sanitize/pkg/yaml/sanitize.go`](../../../../../../sanitize/pkg/yaml/sanitize.go)
   - referenced, not changed by this ticket unless integration exposes missing API needs.

### Recommended protocol constants

```go
const (
    AgentModeTagPackage = "pinocchio"
    AgentModeTagType    = "agent_mode_switch"
    AgentModeTagVersion = "v1"
)
```

Why use these names:

1. `pinocchio` reflects app ownership.
2. `agent_mode_switch` is explicit and stable.
3. `v1` leaves room for format evolution.

### Recommended prompt example

```text
<modeSwitchGuidelines>
Analyze the conversation and decide whether switching modes would improve the response.
If so, emit exactly one structured block using this format:
</modeSwitchGuidelines>

<pinocchio:agent_mode_switch:v1>
```yaml
mode_switch:
  analysis: |
    Explain why the current mode is or is not a fit.
  new_mode: MODE_NAME  # omit this field if no switch is recommended
```
</pinocchio:agent_mode_switch:v1>

Current mode: financial_analyst
Available modes: financial_analyst, category_regexp_designer, category_regexp_reviewer
```

Note the key requirement: the structured tag is the protocol anchor. The YAML fence inside it is optional from a parser perspective if the parser strips fences, but the prompt should show it explicitly because models often produce cleaner YAML when the example includes the fence.

## Pseudocode

### Shared parser

```go
func ParseModeSwitchPayload(raw []byte) (*ParsedModeSwitch, error) {
    _, body := parsehelpers.StripCodeFenceBytes(raw)
    src := strings.TrimSpace(string(body))
    if src == "" {
        return nil, ErrEmptyPayload
    }

    sanitized := yamlsanitize.Sanitize(src)
    candidate := sanitized.Sanitized
    if strings.TrimSpace(candidate) == "" {
        candidate = src
    }

    var payload ModeSwitchPayload
    if err := yaml.Unmarshal([]byte(candidate), &payload); err != nil {
        return nil, err
    }

    analysis := strings.TrimSpace(payload.ModeSwitch.Analysis)
    newMode := strings.TrimSpace(payload.ModeSwitch.NewMode)
    if analysis == "" && newMode == "" {
        return nil, ErrNoMeaningfulProposal
    }

    return &ParsedModeSwitch{
        Analysis:      analysis,
        NewMode:       newMode,
        RawYAML:       src,
        SanitizedYAML: candidate,
        ParseClean:    sanitized.ParseClean && sanitized.LintClean,
    }, nil
}
```

### Extractor session

```go
type ModeSwitchExtractor struct{}

func (e *ModeSwitchExtractor) TagPackage() string { return AgentModeTagPackage }
func (e *ModeSwitchExtractor) TagType() string    { return AgentModeTagType }
func (e *ModeSwitchExtractor) TagVersion() string { return AgentModeTagVersion }

func (e *ModeSwitchExtractor) NewSession(ctx context.Context, meta events.EventMetadata, itemID string) structuredsink.ExtractorSession {
    return &modeSwitchSession{meta: meta, itemID: itemID}
}

type modeSwitchSession struct {
    meta  events.EventMetadata
    itemID string
    buf   bytes.Buffer
}

func (s *modeSwitchSession) OnStart(ctx context.Context) []events.Event {
    return nil
}

func (s *modeSwitchSession) OnRaw(ctx context.Context, chunk []byte) []events.Event {
    s.buf.Write(chunk)
    return nil
}

func (s *modeSwitchSession) OnCompleted(ctx context.Context, raw []byte, success bool, err error) []events.Event {
    if !success {
        return nil
    }
    parsed, parseErr := ParseModeSwitchPayload(raw)
    if parseErr != nil {
        return nil
    }
    return []events.Event{
        NewAgentModeProposalEvent(s.meta, parsed),
    }
}
```

### Web-chat sink integration

```go
srv, err := webchat.NewServer(
    ctx,
    parsed,
    staticFS,
    webchat.WithRuntimeComposer(runtimeComposer),
    webchat.WithEventSinkWrapper(func(convID string, req infruntime.ConversationRuntimeRequest, downstream events.EventSink) (events.EventSink, error) {
        return structuredsink.NewFilteringSink(
            downstream,
            structuredsink.Options{
                Malformed: structuredsink.MalformedErrorEvents,
            },
            &agentmode.ModeSwitchExtractor{},
        ), nil
    }),
)
```

### Middleware final application

```go
addedBlocks := rootmw.NewBlocksNotIn(res, baselineIDs)
proposal, err := ParseModeSwitchFromNewBlocks(addedBlocks)
if err != nil || proposal == nil {
    return res, nil
}

if proposal.NewMode != "" && proposal.NewMode != currentMode {
    turns.KeyAgentMode.Set(&res.Data, proposal.NewMode)
    svc.RecordModeChange(ctx, NewChange(sessionID, res.ID, currentMode, proposal.NewMode, proposal.Analysis))
    turns.AppendBlock(res, turns.NewSystemTextBlock(fmt.Sprintf("[agent-mode] switched to %s", proposal.NewMode)))
    publishAgentModeSwitchEvent(...)
}
```

## Design Decisions

### Decision 1: Keep middleware as the authoritative state mutator

Rationale:

1. The middleware already owns mode persistence and turn mutation.
2. `FilteringSink` is explicitly documented as a progressive extraction and UX boundary, not the persistence boundary.
3. This preserves clear responsibility: stream extractor for observation, middleware for commitment.

### Decision 2: Use the Geppetto structured-tag format, not arbitrary XML

Rationale:

1. It already integrates with `FilteringSink`.
2. It avoids adding a second tag grammar.
3. It satisfies the user's "XML approach" request in the way this codebase already understands.

### Decision 3: Reuse YAML payload semantics rather than replacing them with JSON

Rationale:

1. The existing `agentmode` protocol is already YAML.
2. The user's request explicitly says "nested YAML."
3. The sanitize library already has a reusable YAML repair path.

### Decision 4: Introduce a shared parser used by both middleware and extractor

Rationale:

1. It removes duplicate parse logic.
2. It guarantees the middleware and the extractor interpret the payload identically.
3. It ensures the `sanitize` integration is not forgotten in one layer.

### Decision 5: Prefer a distinct proposal event for extractor-driven UI

Rationale:

1. It separates "proposed" from "applied."
2. It avoids duplicate final switch cards.
3. It still allows SEM reuse by mapping both events to `agent.mode`.

## Alternatives Considered

### Alternative A: Leave middleware as-is and only add sanitize around current YAML parsing

Pros:

- smallest code change,
- lowest immediate risk.

Cons:

- does not deliver the XML plus nested YAML protocol,
- does not add structured streaming extraction,
- leaves protocol ambiguity in assistant-visible text,
- duplicates parsing logic if a sink is added later.

Decision: rejected as incomplete.

### Alternative B: Move all mode-switch logic into the extractor and remove middleware parsing entirely

Pros:

- cleaner streaming-centric protocol,
- less duplicate post-inference scanning.

Cons:

- extractor layer is not the right durable state boundary,
- would force state mutation to depend on sink wiring,
- makes middleware less self-sufficient.

Decision: rejected because the middleware should remain authoritative.

### Alternative C: Use JSON inside the structured tag

Pros:

- machine-friendly.

Cons:

- contradicts the user request for nested YAML,
- current ecosystem and examples already assume YAML,
- not materially simpler given the existing sanitize/YAML work.

Decision: rejected.

### Alternative D: Emit the existing `EventAgentModeSwitch` from both extractor and middleware

Pros:

- no new SEM event type needed.

Cons:

- duplicate timeline cards,
- proposal and final switch become indistinguishable.

Decision: not recommended unless the team explicitly chooses simplicity over semantic clarity.

## Implementation Plan

This section is organized as phases because that is how an intern should implement and review the work. The most important implementation rule is that there must be exactly one YAML decode path for agentmode. The middleware and the structured extractor may have different responsibilities, but they must both call the same sanitize-backed parser so behavior does not drift between the final state transition path and the streaming structured path.

### Phase 1: Introduce the shared protocol and parser

Files:

- `pinocchio/pkg/middlewares/agentmode/protocol.go`
- `pinocchio/pkg/middlewares/agentmode/parser.go`
- `pinocchio/pkg/middlewares/agentmode/middleware.go`

Steps:

1. Define structured-tag constants.
2. Define typed payload structs.
3. Add a new instruction builder that uses the structured tag example.
4. Add parse options with `sanitize_yaml` optional and defaulting to `true`.
5. Add sanitize-backed payload parsing in a dedicated parser helper.
6. Replace existing direct `yaml.Unmarshal` calls in middleware with the shared parser.
7. Remove or deprecate the old raw fence-scanning helpers if they become redundant.

Recommended API shape:

```go
type ParseOptions struct {
    SanitizeYAML *bool `json:"sanitize_yaml,omitempty" yaml:"sanitize_yaml,omitempty"`
}

func DefaultParseOptions() ParseOptions
func (o ParseOptions) SanitizeEnabled() bool
func (o ParseOptions) WithSanitizeYAML(v bool) ParseOptions

func ParseModeSwitchPayload(raw []byte, opts ParseOptions) (*ParsedModeSwitch, error)
func DetectModeSwitchInBlocks(blocks []turns.Block, opts ParseOptions) (*ParsedModeSwitch, bool)
```

Validation:

- unit tests for valid payload,
- unit tests for fenced inner YAML,
- unit tests for malformed-but-repairable YAML,
- unit tests proving sanitize is enabled by default,
- unit tests proving sanitize can be disabled and that malformed YAML then fails,
- unit tests for blank/no-op payloads.

### Phase 2: Add the structured extractor

Files:

- `pinocchio/pkg/middlewares/agentmode/structured_extractor.go`
- possibly `pinocchio/pkg/middlewares/agentmode/events.go`

Steps:

1. Implement `ModeSwitchExtractor`.
2. Pass the same `ParseOptions` type into the extractor config.
3. Add session buffering and final parse.
4. Reuse `ParseModeSwitchPayload(...)` from Phase 1 rather than parsing locally.
5. Decide whether to emit preview/proposal events.
6. If new event types are added, define them with clear semantics.

Validation:

- extractor unit tests against `FilteringSink`,
- malformed tag tests,
- success and failure cases,
- ensure filtered assistant text no longer displays the control block.

### Phase 3: Wire the extractor into web-chat

Files:

- `pinocchio/cmd/web-chat/main.go`
- possibly small helper file for sink wrapper construction

Steps:

1. Add `webchat.WithEventSinkWrapper(...)` to server construction.
2. Add a small helper that inspects the resolved runtime middlewares and only installs the sink when `agentmode` is enabled.
3. Plumb `sanitize_yaml` from the resolved middleware config into the shared `ParseOptions`.
4. Inside the wrapper, wrap the downstream Watermill sink with `structuredsink.NewFilteringSink(...)`.
5. Register the agentmode extractor.
6. Ensure malformed policy is explicit.

Validation:

- integration test or harness verifying the structured block is filtered,
- verify downstream chat UI still receives assistant text without the control payload,
- verify structured events are published.

### Phase 4: Extend SEM only if needed

Files:

- `pinocchio/pkg/webchat/sem_translator.go`
- possibly `pinocchio/cmd/web-chat/web/src/sem/registry.ts`

Steps:

1. If the extractor emits only existing `EventAgentModeSwitch`, confirm no code change is needed.
2. If the extractor emits a new proposal event, add translator mapping to `agent.mode`.
3. Reuse the existing `AgentModeV1` payload by putting extra semantic flags into `data`.

Validation:

- backend translator test,
- frontend reducer test or story fixture,
- manual browser check that the widget still renders.

### Phase 5: Update docs and examples

Files:

- `pinocchio/pkg/doc/topics/webchat-sem-and-ui.md`
- `pinocchio/pkg/doc/topics/webchat-profile-registry.md`
- `pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md`
- `geppetto/pkg/doc/topics/11-structured-sinks.md` if the team wants a first-class agentmode example

Steps:

1. Update the protocol example to show the structured outer tag.
2. Document whether proposal events exist and how they differ from final switch events.
3. Document the sink-wrapper seam for web-chat.

## Testing and Validation Strategy

### Unit tests

Add or update tests for:

1. protocol builder output,
2. sanitize-backed parser behavior,
3. middleware final-mode application,
4. extractor success and malformed cases,
5. SEM mapping if new event types are introduced.

Specific candidates:

- new tests under `pinocchio/pkg/middlewares/agentmode/*_test.go`
- translator tests near `pinocchio/pkg/webchat/sem_translator.go`
- runtime composition or web-chat integration tests in `pinocchio/cmd/web-chat/*_test.go`

### Recommended commit boundaries

To keep this ticket reviewable, the implementation should be committed in at least two code commits and one documentation follow-up:

1. Shared protocol/parser plus middleware migration and tests.
2. Structured extractor plus web-chat sink wrapper, config plumbing, and tests.
3. Ticket diary/changelog/task updates capturing commands, failures, verification, and commit hashes.

### Integration tests

At least one end-to-end test should feed a completion containing:

1. normal assistant prose,
2. one structured `agent_mode_switch` block,
3. malformed but repairable YAML inside the block.

Expected assertions:

1. visible assistant text excludes the control block,
2. extractor event appears,
3. middleware applies the new mode only at final boundary,
4. SEM output includes `agent.mode`,
5. frontend reducer accepts the frame shape.

### Manual validation checklist

1. Start web-chat with the new sink wrapper enabled.
2. Submit a prompt that should trigger a mode switch.
3. Verify the browser does not show raw control YAML.
4. Verify the timeline shows the expected agent mode card.
5. Verify the conversation's next turn uses the new mode.

## Risks and Sharp Edges

### Risk 1: Duplicate agent mode cards

If both extractor and middleware emit the same final event, the UI may show duplicate cards. This is the strongest argument for a distinct proposal event.

### Risk 2: Over-sanitizing semantics

`sanitize` can repair syntax, but a repaired YAML document can still represent the wrong semantic intent. The code should validate semantic meaning after sanitization rather than assuming parse success implies correctness.

### Risk 3: Protocol drift between instructions and parser

If prompt examples, tag constants, and parser expectations are spread across files, the protocol will drift. That is why the design recommends a shared protocol module.

### Risk 4: Partial-event noise

If `OnRaw` emits proposal events too aggressively, the UI may churn. Prefer conservative final-only events first, then add debounced partials if there is clear product value.

### Risk 5: Backward compatibility

If active profiles or prompts still rely on the old free-form fenced YAML example, behavior may change. This ticket intentionally recommends the new structured format as the canonical one rather than carrying a permanent compatibility shim. If a temporary fallback is required, document it explicitly and remove it in a follow-up.

## Open Questions

1. Should the extractor emit a new proposal event type now, or should the first version only filter and parse silently?
2. Should the middleware retain a temporary fallback for legacy free-form fenced YAML during rollout?
3. Does the team want the structured tag to be visible in debug tools and raw turn snapshots, or should some views get a cleaned projection?
4. Should the same extractor be installed in the simple chat agent as part of this ticket, or only documented as a second integration step after web-chat?

## Recommended First Implementation Slice

If an intern is overwhelmed, tell them to do the work in this exact order:

1. Add shared protocol constants and sanitize-backed parser.
2. Change middleware prompt builder and final parser to use the new protocol.
3. Add extractor unit tests with `FilteringSink`.
4. Install the sink in web-chat via `WithEventSinkWrapper(...)`.
5. Only then decide whether a proposal event is worth exposing in SEM.

That slice delivers the highest-value architectural cleanup first and keeps the streaming-preview enhancement incremental.

## References

### Core code references

1. [`pinocchio/pkg/middlewares/agentmode/middleware.go`](../../../../../../pinocchio/pkg/middlewares/agentmode/middleware.go)
2. [`pinocchio/pkg/middlewares/agentmode/service.go`](../../../../../../pinocchio/pkg/middlewares/agentmode/service.go)
3. [`geppetto/pkg/steps/parse/yaml_blocks.go`](../../../../../../geppetto/pkg/steps/parse/yaml_blocks.go)
4. [`geppetto/pkg/events/structuredsink/filtering_sink.go`](../../../../../../geppetto/pkg/events/structuredsink/filtering_sink.go)
5. [`geppetto/pkg/events/chat-events.go`](../../../../../../geppetto/pkg/events/chat-events.go)
6. [`pinocchio/pkg/webchat/router_options.go`](../../../../../../pinocchio/pkg/webchat/router_options.go)
7. [`pinocchio/pkg/webchat/router.go`](../../../../../../pinocchio/pkg/webchat/router.go)
8. [`pinocchio/pkg/webchat/sem_translator.go`](../../../../../../pinocchio/pkg/webchat/sem_translator.go)
9. [`pinocchio/cmd/web-chat/web/src/sem/registry.ts`](../../../../../../pinocchio/cmd/web-chat/web/src/sem/registry.ts)
10. [`sanitize/pkg/yaml/sanitize.go`](../../../../../../sanitize/pkg/yaml/sanitize.go)
11. [`sanitize/pkg/yaml/options.go`](../../../../../../sanitize/pkg/yaml/options.go)

### Documentation references

1. [`geppetto/pkg/doc/topics/11-structured-sinks.md`](../../../../../../geppetto/pkg/doc/topics/11-structured-sinks.md)
2. [`geppetto/pkg/doc/tutorials/04-structured-data-extraction.md`](../../../../../../geppetto/pkg/doc/tutorials/04-structured-data-extraction.md)
3. [`geppetto/pkg/doc/playbooks/03-progressive-structured-data.md`](../../../../../../geppetto/pkg/doc/playbooks/03-progressive-structured-data.md)
4. [`pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md`](../../../../../../pinocchio/pkg/doc/tutorials/01-building-a-middleware-with-renderer.md)
5. [`pinocchio/pkg/doc/topics/webchat-sem-and-ui.md`](../../../../../../pinocchio/pkg/doc/topics/webchat-sem-and-ui.md)
6. [`pinocchio/pkg/doc/topics/webchat-profile-registry.md`](../../../../../../pinocchio/pkg/doc/topics/webchat-profile-registry.md)
7. [`sanitize/README.md`](../../../../../../sanitize/README.md)

## Proposed Solution

<!-- Describe the proposed solution in detail -->

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
