---
Title: Webchat Process Core Brainstorm
Ticket: GP-030
Status: active
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/inference/runtime/composer.go
      Note: Current runtime composition contract discussed as a possible seam
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Current LLM-loop-centric startup path under discussion
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: Current app-owned request resolver and handler boundary
    - Path: pinocchio/pkg/webchat/router.go
      Note: Core transport/router boundary discussed in the brainstorm
    - Path: pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/design-doc/01-webchat-values-separation-brief.md
      Note: Related Values-separation brief that feeds into this broader brainstorm
ExternalSources: []
Summary: Brainstorm capturing the discussion about moving Pinocchio webchat from conversation-centric LLM startup toward a more generic process-oriented core while preserving app-owned start semantics.
LastUpdated: 2026-03-07T14:40:18-05:00
WhatFor: Use this brainstorm to capture the current discussion about making Pinocchio webchat less LLM-chat-centric at startup and more generically centered around SEM-emitting processes.
WhenToUse: Use when evaluating or continuing a refactor around Values separation, app-owned runner instantiation, Conversation vs Process naming, and websocket/timeline ownership boundaries.
---


# Webchat Process Core Brainstorm

## Goal

Capture the architecture discussion so far around a possible Pinocchio refactor where:

- applications explicitly decide what process to start;
- Pinocchio continues to provide generic `conv_id`, websocket, timeline hydration, turn persistence, and SEM projection;
- startup semantics may move from a chat-specific conversation model toward a more generic runner or process abstraction.

## Context

The immediate trigger for this discussion was a temporal-relationships integration, but the underlying questions are Pinocchio-level:

- should `Router` keep depending on `*values.Values`?
- should `/chat` mean one universal thing in core, or should feature-specific start endpoints stay app-owned?
- should `ConversationRuntime` fully define the enabled tools and process startup semantics?
- should Pinocchio move from a conversation-centric startup model toward a generic process or runner abstraction?
- who should instantiate the runner: core or the application-owned handler?
- should multiple active conversations be multiplexed over a single websocket?

Current Pinocchio already has a partial split:

- app-owned request resolution in [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
- generic timeline/websocket services in [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- explicit runtime composition in [composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go)

The pressure point is that startup is still shaped around “chat prompt starts an LLM loop,” even though the stream/timeline transport layer is much more generic.

## Quick Reference

### Discussion State So Far

#### Values separation

Observed:

- `Router` currently depends on `*values.Values`.
- This is mainly used to decode router settings and Redis stream settings.

Current direction:

- move parsed-values decoding out of the core constructor;
- keep a compatibility wrapper;
- prefer a dependency-injected constructor underneath.

Related ticket:

- [GP-029 values separation brief](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/design-doc/01-webchat-values-separation-brief.md)

#### App-owned start endpoints, generic transport

Current recommendation:

- keep process start endpoints application-specific by feature;
- keep websocket/timeline/hydration generic.

Examples discussed:

```text
POST /v1/run-chat/chat
POST /v1/extractions
POST /v1/agents/runs

GET /ws?conv_id=...
GET /api/timeline?conv_id=...
```

Interpretation:

- core should not force one universal meaning for `POST /chat`;
- applications decide what process is being started;
- Pinocchio handles transport and persistence once a `conv_id` exists.

#### ConversationRuntime vs process-oriented startup

User direction:

- `ConversationRuntime` should contain enabled tools;
- it may be the right place to describe everything needed to start an LLM-loop-driven process;
- perhaps the startup path should become more generic so even non-LLM SEM emitters can fit behind the same interface.

Current analysis:

- `AllowedTools` already exists in Pinocchio runtime composition;
- the startup path in `ConversationService` is still strongly LLM-loop-shaped;
- the natural refactor seam is above transport, not inside websocket/timeline code.

#### Runner ownership

User direction:

- the app-owned handler should instantiate the runner so that applications can supply their own runners.

Current leaning:

- keep runner ownership with the app;
- core should not own feature-specific runner selection.

#### What the runner should receive

Question raised:

- what should a `Runner` receive so it knows where to emit events and how to communicate with the rest of the system?

Current recommendation:

- split runner inputs by lifecycle rather than hiding everything in `context.Context`.

Constructor-time dependencies:

- engine factories;
- database handles or stores;
- tool registry builders;
- logging;
- feature configuration.

`StartRequest` dependencies:

- `ConvID`;
- `SessionID`;
- SEM/event sink;
- timeline emitter or timeline upsert hook;
- feature payload;
- per-run metadata.

`context.Context` responsibilities:

- cancellation;
- deadlines;
- tracing and correlation;
- request-scoped actor metadata.

Current leaning:

- do not pass raw websocket handles into the runner;
- do not hide primary communication surfaces in `ctx`;
- do not bind a reusable runner instance to one sink at constructor time.

Preferred sketch:

```go
type StartRequest struct {
    ConvID    string
    SessionID string

    Sink     events.EventSink
    Timeline TimelineEmitter

    Payload  any
    Metadata map[string]any
}

type Runner interface {
    Start(ctx context.Context, req StartRequest) (StartResult, error)
}
```

Optional variation:

```go
type RunIO interface {
    EmitSEM(ctx context.Context, event any) error
    UpsertTimeline(ctx context.Context, entity *timelinepb.TimelineEntityV2, version uint64) error
}
```

#### Conversation vs Process naming

Current tension:

- `Conversation` is a good identity for `conv_id`, websocket attach, timeline hydration, and turn persistence.
- `Conversation` is a weaker name for “any SEM-emitting process”.

Tentative split:

- keep `Conversation` as the stream/timeline identity;
- add a generic `Runner`, `ProcessRunner`, or similar abstraction for startup behavior.

Likely outcome:

- do not rename everything around transport;
- add a new abstraction next to `Conversation`.

#### Websocket multiplexing

Question raised:

- should multiple active things share one websocket to reduce frontend connection count?

Current recommendation:

- not by default;
- one websocket per active conversation is simpler and likely good enough;
- only add multiplexing if there is a real multi-conversation dashboard requirement.

Reasons:

- simpler reconnect logic;
- simpler buffering/hydration;
- clearer per-conversation isolation;
- lower protocol complexity.

### Alternatives Under Discussion

#### Alternative A: Minimal API cleanup only

Do:

- Values separation
- app-owned feature start endpoints
- generic `conv_id` / websocket / timeline hydration

Do not:

- change the LLM-loop-centric startup model in core

Pros:

- smallest refactor

Cons:

- startup path remains chat/LLM specific

#### Alternative B: Richer ConversationRuntime only

Do:

- make runtime composition fully describe enabled tools and startup inputs per conversation

Do not:

- introduce a more generic runner abstraction

Pros:

- improves per-conversation flexibility

Cons:

- still leaves the startup path conceptually chat-shaped

#### Alternative C: Add a generic runner abstraction

Sketch:

```go
type Runner interface {
    Start(ctx context.Context, req StartRequest) (StartResult, error)
}
```

App handlers choose or instantiate the runner.

Pros:

- fits LLM loops and non-LLM SEM processes
- keeps transport generic

Cons:

- medium refactor
- needs careful request/ownership boundaries

#### Alternative D: Replace Conversation with Process everywhere

Pros:

- uniform terminology

Cons:

- likely too much churn
- risks losing a useful transport identity concept

Current leaning:

- keep `Conversation` for transport identity
- add `Runner` or `ProcessRunner` for startup behavior

### Questions Asked In This Discussion

These are the user questions that materially shaped the brainstorm:

1. “can we pull Values out of NewRouter? what would we need to pass in?”
2. “Overall I want the /chat endpoint that starts a backend chat (or really, any SEM emitting process) to be more application specific, but the SEM projection + loop + websocket mechanism generic. Is that how this is already built? what kind of chat stasrting endpoint are you planning to build”
3. “so how mcuh would we need to change in pinocchio for that? if at all?”
4. “I want ConversationRuntime to contain the enabled tools, basically COnversationRuntime is all that is needed to start a "process" based oround an LLM Loop, maybe this can actually be made even more generic and encapsulate everything behind a "Run" method that is responsible for the LLM loop, and make an implementation of that interface? Then I can put things behind the request that are not even llm related?”
5. “Also, what would it take to combine multiple running things to a single websocket, so that I can minimize the number of connections in the frontend (if that makes esnse, maybe there is no reason to keep the amount of connections low)”
6. “whowould call Start and StartRequest?”
7. “I think the app-owner handler should instantiate the runner so that an app can instantiate its own runners i guess?”
8. “let's keep Conversation for now, Runner is good. Now, what would we need to pass the Runner so we know where to emit events / communicate with the rest of the system? At convstructor time? At ctx time? Other ways I can't think about?”

### Current Working Recommendation

The most coherent direction from this discussion appears to be:

1. separate `Values` parsing from core router construction;
2. keep application-owned start endpoints explicit;
3. keep `Conversation` as the generic transport/timeline identity;
4. add a generic runner abstraction for startup behavior;
5. let the app-owned handler instantiate or choose the runner;
6. keep one websocket per active conversation unless a true multiplexing use case appears.

## Usage Examples

### Example 1: Reviewing the likely refactor boundary

Start with these files:

- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)
- [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
- [composer.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/inference/runtime/composer.go)

Then use this brainstorm to decide whether the next step is:

- a small constructor/API cleanup;
- richer conversation runtime composition;
- or a new runner/process abstraction.

### Example 2: Candidate future split

One plausible future shape:

```go
type ConversationIdentity struct {
    ConvID    string
    SessionID string
}

type StartRequest struct {
    Identity ConversationIdentity
    Sink     events.EventSink
    Timeline TimelineEmitter
    Metadata map[string]any
    Payload  any
}

type Runner interface {
    Start(ctx context.Context, req StartRequest) (StartResult, error)
}
```

The application-owned handler would instantiate the runner and the request, while Pinocchio would continue to own the transport/persistence tied to the conversation identity.

## Related

Related work:

- [GP-029 values separation brief](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp/2026/03/07/GP-029--webchat-values-separation-brief/design-doc/01-webchat-values-separation-brief.md)
- [router.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/router.go)
- [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)
