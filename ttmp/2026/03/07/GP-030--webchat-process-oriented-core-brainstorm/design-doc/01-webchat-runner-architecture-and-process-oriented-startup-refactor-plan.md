---
Title: Webchat Runner Architecture and Process-Oriented Startup Refactor Plan
Ticket: GP-030
Status: active
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/inference/runtime/composer.go
      Note: ComposedRuntime already contains runtime-level concepts such as AllowedTools and Sink that inform the proposed Runner API
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: Conversation remains the transport identity in the proposed design
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: ConversationService contains the LLM-loop-specific startup seam to extract behind Runner
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: HTTP helpers already show the app-owned request resolution and generic transport split
    - Path: pinocchio/pkg/webchat/router.go
      Note: Router construction is the current composition root and the first place to separate Values parsing from generic dependencies
    - Path: pinocchio/pkg/webchat/stream_hub.go
      Note: StreamHub already demonstrates the generic websocket attachment layer that should remain unchanged
ExternalSources: []
Summary: Detailed intern-facing design and implementation guide for keeping Conversation as the transport identity while introducing app-owned Runner startup semantics in Pinocchio webchat.
LastUpdated: 2026-03-07T14:40:18-05:00
WhatFor: Use this design guide to plan a Pinocchio refactor that keeps Conversation as the transport identity while introducing an app-owned Runner abstraction for process startup.
WhenToUse: Use when designing or implementing a more generic webchat startup path, especially when an application wants feature-specific start endpoints and non-chat SEM-emitting processes on the same transport substrate.
---


# Webchat Runner Architecture and Process-Oriented Startup Refactor Plan

## Executive Summary

Pinocchio webchat is already split in an important way: the stream, websocket, timeline hydration, and turn persistence machinery is generic, while the request resolver is application-owned. The remaining coupling is in startup. The current startup path still assumes that the thing being started is a prompt-driven LLM loop. That assumption lives mostly in `ConversationService` and in the surrounding `ChatService` surface.

The recommended refactor is not to replace `Conversation` with `Process`. `Conversation` is still a useful transport identity because it already anchors `conv_id`, `session_id`, websocket attachment, timeline hydration, timeline projection, and turn persistence. Instead, add a new `Runner` abstraction next to `Conversation`. A `Runner` represents startup and execution behavior for one conversation-backed process. One implementation can start the existing LLM loop. Future implementations can start extraction-style jobs, agent orchestration, or even non-LLM SEM-emitting workflows.

The application-owned HTTP handler should instantiate or select the `Runner`. Pinocchio core should own the generic conversation lifecycle and transport plumbing. The per-run communication surfaces such as SEM sinks and timeline upsert hooks should be passed in the `StartRequest`, not hidden in `context.Context` and not hard-bound at runner construction time.

## Problem Statement

Today, Pinocchio has a strong generic transport layer but a startup path that is still shaped around a specific use case: "submit a prompt and start an inference loop."

The current code shows the split clearly:

- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go) builds the generic infrastructure: stream backend, timeline store, turn store, conversation manager, conversation service, and HTTP helpers.
- [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go) already lets the application resolve request policy and feed `conv_id`, runtime, overrides, and prompt into generic handlers.
- [conversation.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation.go) models the long-lived conversation identity and its transport state.
- [stream_hub.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/stream_hub.go) attaches websocket clients to a conversation and ensures the conversation exists.
- [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go) resolves the conversation and then immediately drops into prompt-specific inference startup via `startInferenceForPrompt`.
- [composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go) already supports a `ComposedRuntime` with `Engine`, `Sink`, `RuntimeFingerprint`, `RuntimeKey`, `SeedSystemPrompt`, and `AllowedTools`.

This means the transport identity is broader than the startup path. The system can carry any SEM-emitting workflow over the transport layer, but the public startup surface still speaks as if everything were "chat."

The immediate product need behind this refactor is to support application-specific start semantics such as:

- feature-owned `POST /v1/run-chat/chat`;
- feature-owned `POST /v1/extractions`;
- later feature-owned `POST /v1/agents/runs`;
- all converging on the same generic `conv_id`, websocket, timeline, and turn snapshot machinery.

The design problem is therefore:

- how to preserve the generic transport substrate;
- how to let the application own startup semantics and runner selection;
- how to avoid turning `ConversationRuntime` or `context.Context` into an unstructured bag of everything.

## Goals And Non-Goals

### Goals

- Keep `Conversation` as the canonical transport identity.
- Introduce a `Runner` abstraction for startup and execution behavior.
- Let the application-owned handler instantiate or choose the runner.
- Preserve the generic websocket, stream, SEM, timeline hydration, and turn persistence layers.
- Make per-run communication surfaces explicit in typed request objects.
- Allow non-LLM SEM producers to fit behind the same conversation-backed transport.

### Non-Goals

- Renaming the entire system from `Conversation` to `Process`.
- Building a separate transport stack for non-chat features.
- Adding websocket multiplexing in the first refactor.
- Replacing the current runtime composer immediately with a totally new subsystem.

## Current Architecture Walkthrough

The best way to understand the refactor is to map the current system by responsibility.

### 1. Router composition

[router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go) currently does four jobs:

- decodes config from `*values.Values`;
- constructs storage and stream dependencies;
- builds the core conversation services;
- mounts utility HTTP handlers.

Conceptually:

```text
values.Values
  -> stream backend
  -> timeline store
  -> turn store
  -> ConvManager
  -> ConversationService
  -> ChatService
  -> HTTP helper handlers
```

This is already close to the desired architecture, except the startup surface exposed by `ChatService` still encodes "chat prompt" semantics.

### 2. Conversation identity and lifecycle

[conversation.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation.go) defines `Conversation`, which already holds the durable identity and long-lived execution state:

- `ID` and `SessionID`;
- session, engine, and sink references;
- runtime metadata;
- allowed tools;
- queueing and idempotency state;
- timeline projector and observed timeline version;
- connection pool and stream coordinator.

This is why keeping `Conversation` is the right move. These fields are not merely "chat" fields. They are the backbone of the generic transport/session model.

### 3. Websocket and hydration transport

[stream_hub.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/stream_hub.go) is already generic. It does not need to know whether the underlying work is chat, extraction, or something else. It only needs:

- a `conv_id`;
- a way to ensure the backing conversation exists;
- a connection to attach.

[http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go) mirrors that split:

- `NewChatHandler` handles start semantics;
- `NewWSHandler` handles attach semantics;
- `NewTimelineHandler` handles hydration semantics.

The important observation is that the generic part is already modular.

### 4. Runtime composition

[composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go) defines the runtime composition seam:

```go
type ComposedRuntime struct {
    Engine             engine.Engine
    Sink               events.EventSink
    RuntimeFingerprint string
    RuntimeKey         string
    SeedSystemPrompt   string
    AllowedTools       []string
}
```

This is already very close to the right abstraction for conversation-level runtime policy. It means the system already accepts the idea that tool exposure and sink choice are runtime-level concerns.

### 5. Startup is still chat-shaped

[conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go) is where the generic transport layer narrows into an LLM-loop-specific implementation. `SubmitPrompt` does the following:

1. Resolve or create the conversation.
2. Validate prompt/idempotency semantics.
3. Call `PrepareSessionInference`.
4. Start inference through `startInferenceForPrompt`.

That last step is the main refactor seam. The code there is not transport-specific. It is one concrete execution strategy.

## Proposed Architecture

The proposal is to preserve the existing conversation substrate and make the startup path pluggable.

### Core idea

- `Conversation` remains the transport identity.
- `Runner` becomes the startup/execution abstraction.
- the application handler chooses or instantiates the runner.
- the runner receives explicit per-run communication surfaces in `StartRequest`.

### Conceptual model

```text
App HTTP handler
  -> validates feature request
  -> resolves domain objects and runtime policy
  -> chooses or instantiates Runner
  -> builds StartRequest
  -> asks Pinocchio to ensure Conversation
  -> calls Runner.Start(...)

Runner
  -> emits SEM to Sink
  -> upserts timeline entities through TimelineEmitter
  -> persists turns through existing conversation/session flow as appropriate

Pinocchio transport
  -> fans out websocket frames
  -> stores timeline entities
  -> serves /api/timeline hydration
```

### Proposed interfaces

The specific types can still change, but this is the intended shape:

```go
type TimelineEmitter interface {
    Upsert(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error
}

type StartRequest struct {
    ConvID    string
    SessionID string

    Sink     events.EventSink
    Timeline TimelineEmitter

    Payload  any
    Metadata map[string]any
}

type StartResult struct {
    Accepted          bool
    ExternalRunID     string
    RuntimeKey        string
    RuntimeFingerprint string
}

type Runner interface {
    Start(ctx context.Context, req StartRequest) (StartResult, error)
}
```

An alternative is a single IO abstraction:

```go
type RunIO interface {
    EmitSEM(ctx context.Context, event any) error
    UpsertTimeline(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error
}
```

That can work, but the split `Sink + TimelineEmitter` is slightly easier for new contributors to reason about because it keeps event streaming and durable projection distinct.

## Why Keep Conversation

Renaming everything to `Process` looks tempting but is the wrong first move.

`Conversation` already means something durable and technical:

- it owns `conv_id`;
- it owns `session_id`;
- websocket clients attach to it;
- timeline hydration queries target it;
- turn snapshots are associated with it;
- timeline projection state is cached on it;
- runtime metadata is tracked on it.

That makes `Conversation` a very good transport/session identity even if the thing started is not human chat.

The correct generalization is not:

- replace `Conversation` with `Process`.

The correct generalization is:

- keep `Conversation`;
- add `Runner`.

## Why The App Handler Should Own Runner Selection

The application knows what the request means. Pinocchio does not.

For example, an application-owned handler may need to decide:

- whether the user is starting a chat against an extraction run;
- whether a specific tool set is enabled;
- whether the request should run an LLM loop or a non-LLM workflow;
- how the feature-specific payload should be validated.

That makes the application the natural owner of runner instantiation.

The boundary should look like this:

```text
Pinocchio core owns:
  conversation lifecycle
  transport
  SEM fanout
  timeline hydration
  turn snapshots

Application owns:
  route meaning
  request validation
  domain lookup
  runner choice
  payload construction
```

This also keeps Pinocchio useful as a library. It remains a generic conversation transport and runtime host, not a feature policy engine.

## What To Pass To A Runner

This was one of the key design questions in the discussion. The answer is to separate inputs by lifecycle.

### Constructor-time dependencies

Use the runner constructor for long-lived, reusable dependencies:

- engine factories;
- database handles and stores;
- tool registry builders;
- feature services;
- logger;
- static config.

Example:

```go
type LLMLoopRunner struct {
    engineFactory EngineFactory
    logger        zerolog.Logger
    toolFactories map[string]ToolFactory
}
```

### `StartRequest` dependencies

Use `StartRequest` for anything that changes per conversation start:

- `ConvID`;
- `SessionID`;
- SEM event sink;
- timeline emitter or upsert hook;
- feature payload;
- run metadata.

Example:

```go
req := StartRequest{
    ConvID:    conv.ID,
    SessionID: conv.SessionID,
    Sink:      conv.Sink,
    Timeline:  timelineEmitter,
    Payload:   payload,
    Metadata: map[string]any{
        "feature": "run-chat",
        "chat_session_id": "chat-123",
    },
}
```

### `context.Context`

Use `ctx` only for ambient concerns:

- cancellation;
- deadlines;
- tracing;
- correlation IDs;
- actor or request metadata when already represented in existing middleware.

Do not put primary runtime dependencies in `ctx`. It makes the API opaque and makes testing harder.

### Anti-patterns to avoid

- Do not pass raw websocket connections into the runner.
- Do not bind a reusable runner instance to one conversation-specific sink in the constructor.
- Do not bury the event sink or timeline writer inside `context.Context`.

## How This Maps To Existing Code

### Minimal refactor boundary

The lowest-risk path is:

1. keep `StreamHub` and `TimelineService` unchanged;
2. keep `Conversation` and `ConvManager` unchanged or nearly unchanged;
3. introduce `Runner` next to `ConversationService`;
4. move LLM-loop-specific startup from `startInferenceForPrompt` into an `LLMLoopRunner`.

### Suggested responsibility split after refactor

`ConversationService`:

- resolve and ensure conversations;
- expose transport helpers;
- coordinate runner startup entrypoints.

`LLMLoopRunner`:

- take an ensured conversation and start prompt-driven inference;
- honor allowed tools from the runtime/composed conversation;
- emit SEM and timeline updates through typed interfaces.

Application handler:

- parse request;
- choose runner;
- build payload;
- call start path.

### Possible API sketch inside Pinocchio

```go
type ConversationService struct {
    baseCtx context.Context
    cm      *ConvManager
    streams *StreamHub
}

func (s *ConversationService) StartWithRunner(
    ctx context.Context,
    runtimeReq ConversationRuntimeRequest,
    runner Runner,
    startReq StartRequest,
) (StartResult, error) {
    handle, err := s.ResolveAndEnsureConversation(ctx, runtimeReq)
    if err != nil {
        return StartResult{}, err
    }

    conv, ok := s.cm.GetConversation(handle.ConvID)
    if !ok || conv == nil {
        return StartResult{}, errors.New("conversation not found after resolve")
    }

    startReq.ConvID = conv.ID
    startReq.SessionID = conv.SessionID
    if startReq.Sink == nil {
        startReq.Sink = conv.Sink
    }

    return runner.Start(ctx, startReq)
}
```

The exact location of this helper can vary. The main point is that ensuring the conversation and choosing the runner are separate actions.

## Endpoint Shape After Refactor

The transport endpoints remain generic:

```text
GET /ws?conv_id=...
GET /api/timeline?conv_id=...
```

Start endpoints remain feature-owned:

```text
POST /v1/run-chat/chat
POST /v1/extractions
POST /v1/agents/runs
```

That means the application can keep route semantics precise while the frontend still converges on a stable transport contract:

- create or resolve the feature session;
- receive a `conv_id`;
- attach websocket using `conv_id`;
- hydrate timeline using `conv_id`;
- optionally resume from turn snapshots using `session_id`.

## Sequence Diagrams

### Sequence 1: LLM-backed feature start

```text
Client
  -> POST /v1/run-chat/chat
Application handler
  -> validate request
  -> resolve runtime + domain objects
  -> instantiate LLMLoopRunner
  -> ResolveAndEnsureConversation(...)
Pinocchio ConversationService
  -> returns conv_id + session_id
Application handler
  -> Runner.Start(ctx, StartRequest{ConvID, SessionID, Sink, Timeline, Payload})
LLMLoopRunner
  -> emits SEM
  -> updates timeline
Client
  -> GET /ws?conv_id=...
  -> GET /api/timeline?conv_id=...
```

### Sequence 2: Non-LLM SEM producer

```text
Client
  -> POST /v1/extractions
Application handler
  -> instantiate ExtractionRunner
  -> ensure conversation
  -> ExtractionRunner.Start(...)
ExtractionRunner
  -> emits SEM status / progress / results
  -> writes timeline entities
Client
  -> attaches to the same generic websocket and timeline APIs
```

## Alternatives Considered

### Alternative 1: Keep current startup shape and only separate `Values`

Pros:

- smallest change;
- helps embedding and testing immediately.

Cons:

- startup remains chat-specific;
- non-LLM SEM workflows still have no clean first-class abstraction.

### Alternative 2: Make `ConversationRuntime` carry everything

Pros:

- keeps the number of new interfaces low;
- leverages the existing `ComposedRuntime`.

Cons:

- risks turning `ConversationRuntime` into an overloaded bag of feature semantics;
- still does not clearly answer who starts work and how.

### Alternative 3: Rename `Conversation` to `Process`

Pros:

- generic terminology.

Cons:

- high churn;
- loses the valuable transport/session meaning already present in the code;
- does not solve runner ownership by itself.

### Alternative 4: Put sinks and runtime objects into `context.Context`

Pros:

- small surface change.

Cons:

- poor discoverability;
- harder testing;
- encourages hidden dependencies;
- makes startup contracts harder for interns and reviewers to understand.

### Alternative 5: Multiplex many conversations over one websocket

Pros:

- fewer browser sockets.

Cons:

- much more protocol complexity;
- harder auth and reconnect logic;
- one broken connection affects all followed conversations.

Recommendation:

- keep one websocket per active conversation unless a real dashboard use case proves otherwise.

## Detailed Implementation Plan

### Phase 1: Separate constructor concerns from parsed values

Objective:

- keep `NewRouter` convenience behavior;
- add a dependency-injected path underneath.

Tasks:

- add a lower-level constructor such as `NewRouterFromDeps(...)`;
- move `values.Values` decoding into a thin adapter or compatibility wrapper;
- make router settings explicit as a struct passed into the lower-level constructor.

Expected files:

- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- stream backend constructor file
- server/bootstrap integration files

### Phase 2: Introduce `Runner`

Objective:

- make startup behavior pluggable without changing transport identity.

Tasks:

- add `Runner`, `StartRequest`, and `StartResult`;
- add a timeline-emission interface or equivalent;
- extract the current LLM-loop startup code into `LLMLoopRunner`.

Expected files:

- [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)
- new runner file under `pinocchio/pkg/webchat/`
- tests adjacent to conversation service and chat service

### Phase 3: Adapt the application start surface

Objective:

- let feature handlers instantiate runners and feed typed payloads into them.

Tasks:

- add a runner-aware start helper in Pinocchio or in the embedding application;
- update example apps to show app-owned runner selection;
- keep `/ws` and `/api/timeline` unchanged.

Expected files:

- [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
- example app handlers
- webchat example wiring

### Phase 4: Optional cleanup and generalization

Objective:

- reduce chat-centric naming where it blocks extension, but keep `Conversation`.

Tasks:

- review `ChatService` naming;
- decide whether a more neutral facade such as `StartService` is useful;
- document the distinction between transport identity and startup behavior.

### Phase 5: Optional future multiplexing

This is not recommended for the first implementation, but if needed later:

- define subscribe and unsubscribe control messages;
- tag outgoing frames with `conv_id`;
- add tests for fanout isolation and reconnect behavior.

## Testing Plan

### Unit tests

- `Runner.Start` receives the correct `ConvID`, `SessionID`, and IO surfaces.
- `ConversationService` still ensures conversations before startup.
- `LLMLoopRunner` preserves idempotency and queue behavior now covered by existing chat tests.
- tool filtering still honors `AllowedTools`.

### Integration tests

- start an LLM-backed conversation and verify:
  - websocket receives SEM;
  - `/api/timeline` hydrates the same conversation;
  - turn snapshots still persist.
- start a fake non-LLM runner and verify the same transport path works.

### Regression focus

- runtime rebuild on profile fingerprint/version changes;
- idle eviction and reconnection behavior;
- timeline projection consistency;
- backward compatibility for existing webchat examples.

## Guidance For A New Intern

If you are implementing this for the first time, read the code in this order:

1. [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
2. [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
3. [stream_hub.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/stream_hub.go)
4. [conversation.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation.go)
5. [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)
6. [composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go)

Questions to keep asking yourself:

- is this code about transport identity or startup behavior?
- should this dependency live on the runner constructor, the `StartRequest`, or `ctx`?
- is this app policy, or generic Pinocchio machinery?

If you are unsure, prefer explicit typed parameters over hidden context values.

## Open Questions

- Should Pinocchio provide a helper like `ConversationService.StartWithRunner(...)`, or should embeddings call `ResolveAndEnsureConversation(...)` and `Runner.Start(...)` directly?
- Should `StartRequest` carry `Sink` and `Timeline` separately, or should there be a single `RunIO` abstraction?
- How much of today’s `ChatService` should remain after runner generalization?
- Should `AllowedTools` remain on `ComposedRuntime`, or should there be a more explicit runner-facing tool configuration struct?

## References

- [Brainstorm: Webchat Process Core Brainstorm](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/07/GP-030--webchat-process-oriented-core-brainstorm/reference/01-webchat-process-core-brainstorm.md)
- [GP-029 Values Separation Brief](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/design-doc/01-webchat-values-separation-brief.md)
- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- [conversation.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation.go)
- [stream_hub.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/stream_hub.go)
- [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)
- [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
- [composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go)
