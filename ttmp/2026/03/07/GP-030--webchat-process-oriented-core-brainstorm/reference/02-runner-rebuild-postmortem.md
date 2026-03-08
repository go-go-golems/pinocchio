---
Title: GP-030 Runner Rebuild Postmortem
Ticket: GP-030
Status: complete
Topics:
    - webchat
    - backend
    - pinocchio
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pinocchio/pkg/webchat/runner.go
      Note: Final generic runner API after the rollback and rebuild
    - Path: pinocchio/pkg/webchat/chat_service.go
      Note: Chat-owned queue/idempotency path and prompt runner startup
    - Path: pinocchio/pkg/webchat/llm_state.go
      Note: Lazy LLM execution-state creation that replaced eager session creation
    - Path: pinocchio/pkg/webchat/llm_loop_runner.go
      Note: Final LLM runner implementation on top of the generic runner seam
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Final generic conversation transport service after chat startup was moved out
ExternalSources: []
Summary: Detailed postmortem of the first GP-030 runner refactor, why it was rolled back, and how the corrected architecture works.
LastUpdated: 2026-03-07T19:45:00-05:00
WhatFor: Use this postmortem to understand the failed first GP-030 attempt, the corrected implementation, and the reasoning behind the final code boundaries.
WhenToUse: Use when reviewing the runner refactor, onboarding a new contributor, or planning the next cleanup steps in Pinocchio webchat.
---

# GP-030 Runner Rebuild Postmortem

## Executive Summary

GP-030 was supposed to make Pinocchio webchat more generic by introducing a `Runner` abstraction while keeping the existing websocket, SEM, timeline, and conversation machinery reusable. The first implementation did introduce `Runner`, but it did not move the actual ownership boundaries. It mostly renamed the problem instead of separating it.

The failure mode was specific:

- generic runner startup still exposed raw `*Conversation`
- generic conversation creation still eagerly created LLM state
- the new runner-first app path skipped the existing prompt queue/idempotency contract
- several public types still leaked LLM-specific data

So the first version compiled, tested, and looked plausible at a glance, but the abstraction line was wrong. The right response was to rewind those commits and rebuild from the pre-runner baseline with stricter rules.

The corrected implementation keeps the good part of the original idea:

- `Conversation` remains the transport identity
- `Runner` is the process-execution seam
- `StartRequest` is transport-safe
- `RunHandle` is generic
- prompt queue/idempotency remains owned by `ChatService`
- LLM session state is created lazily in [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go)

This document explains:

- what the webchat subsystem is
- what GP-030 was trying to accomplish
- what was wrong with the first attempt
- how I approached the rebuild
- why the first attempt likely went wrong from a process and reasoning perspective
- what the final design now looks like

## Audience

This document is written for:

- a new intern trying to understand the subsystem
- a reviewer trying to understand why the first version was rejected
- a future maintainer deciding how to extend webchat beyond LLM chat

## System Primer

Before the postmortem makes sense, it helps to understand the relevant parts of Pinocchio webchat.

### What the subsystem does

Pinocchio webchat provides a generic realtime transport surface around a conversation-like identity:

- a `conv_id`
- websocket attachment
- timeline hydration
- SEM fanout
- turn persistence

In practical terms, it lets a frontend:

1. create or resolve a conversation
2. connect to `GET /ws?conv_id=...`
3. fetch `GET /api/timeline?conv_id=...`
4. watch a backend process emit SEM and timeline entities into that conversation

### Core concepts

#### `Conversation`

`Conversation` is the transport identity. It should answer questions like:

- what is the `conv_id`?
- what session is attached?
- what stream buffer belongs to this conversation?
- what metadata/timeline state is associated with it?

It should not be the public API for “how to start any arbitrary backend process.”

Primary file:
- [conversation.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation.go)

#### `ConversationService`

`ConversationService` is the generic service boundary around conversation transport operations:

- ensure conversation exists
- prepare transport-safe start inputs
- attach websocket
- emit timeline entities

Primary file:
- [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)

#### `ChatService`

`ChatService` owns prompt-driven chat semantics:

- queueing
- idempotency
- prompt submission
- completion bookkeeping
- queue drain

Primary file:
- [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go)

#### `Runner`

`Runner` is the execution seam. It should answer:

- what process am I starting?
- what does it emit?
- when is it done?

It should not own generic websocket transport or conversation identity.

Primary file:
- [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go)

#### `LLMLoopRunner`

`LLMLoopRunner` is one implementation of `Runner`. It wraps the current LLM loop behavior:

- decode payload
- ensure lazy LLM state
- filter tools
- append user prompt
- emit SEM
- start inference

Primary file:
- [llm_loop_runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_loop_runner.go)

### Before GP-030

Before GP-030, the easiest way to start chat was effectively:

```text
HTTP /chat
  -> ConversationService.SubmitPrompt(...)
     -> ensure conversation
     -> queue/idempotency
     -> build runtime/session/tool state
     -> start inference
```

That worked for chat, but it tied too many responsibilities together.

### What GP-030 was trying to achieve

The goal was to support this architecture:

```text
feature-specific POST /...
  -> app resolves domain/runtime policy
  -> app chooses runner
  -> app starts runner for a conversation

generic GET /ws
generic GET /api/timeline
```

That is the right direction. The first implementation failed because it changed the API before changing the ownership model.

## What The First Attempt Changed

The earlier implementation made these moves:

- added `Runner`
- added `StartRequest`
- added `StartResult`
- added a new app-owned startup path
- added `LLMLoopRunner`

Those changes sounded correct, but the internal structure still looked like this:

```text
generic runner API
  -> raw Conversation
  -> raw Geppetto execution handle
  -> eager LLM setup hidden in generic ensure
  -> old queue/idempotency bypassed
```

That is not a true separation. It is a relabeling of an LLM chat startup path as if it were generic.

## What Was Wrong With The Previous Version

### Problem 1: The public abstraction was wider than the actual boundary

The first `StartRequest` exposed raw `*Conversation`. That meant any new runner could reach into:

- session state
- engine state
- stream fields
- internal queue state
- tool metadata

That is a design smell because a “generic” abstraction should not give callers more power than they need.

What the runner actually needed was much smaller:

- `ConvID`
- `SessionID`
- a sink for SEM
- a timeline emitter
- runtime metadata
- payload

That smaller shape is what the corrected `StartRequest` now provides in [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go).

### Problem 2: `StartResult` leaked Geppetto-specific execution details

The first version returned a Geppetto execution handle. That meant the generic API already assumed the process behind the runner was:

- an LLM loop
- backed by a Geppetto session
- using the same execution/control model

That breaks the whole point of the runner seam. A fake runner, extractor, or other SEM producer should not have to pretend to be a Geppetto execution.

The corrected shape is:

```go
type RunHandle interface {
    Wait() error
}
```

That is intentionally small. It gives queue-drain logic what it needs without hard-coding the backend model.

### Problem 3: Generic conversation ensuring still eagerly created LLM state

This was one of the biggest architectural leaks.

The generic conversation-creation path still did LLM-specific work:

- compose runtime
- create engine
- create `session.Session`
- seed the system turn

That created a subtle but serious bug in the architecture:

```text
frontend just attaches websocket
  -> generic get-or-create
  -> hidden LLM session creation
  -> hidden engine setup
```

The transport layer was still starting to behave like execution setup.

That is why lazy LLM state in [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go) was not just an optimization. It was the main structural fix.

### Problem 4: The new app-owned path lost queue/idempotency

This was the behavioral regression that made the design bug obvious.

The old path:

```text
SubmitPrompt
  -> ensure conversation
  -> prepare session inference
  -> maybe queue / maybe replay / maybe run now
  -> start inference
```

The first runner path:

```text
PrepareRunnerStart
  -> ensure conversation
  -> return runnable StartRequest

runner.Start
  -> run now
```

That skipped the existing contract for prompt-driven chat:

- same prompt/request can be replayed idempotently
- concurrent prompt submissions can queue
- the client gets the established `202 queued` / replay semantics

This is why the correct fix was not “teach `PrepareRunnerStart(...)` about `LLMLoopStartPayload`.” That would have moved chat policy into a generic service and made the boundary even worse.

The real fix was:

- keep `ConversationService.PrepareRunnerStart(...)` generic
- move prompt-specific queue/idempotency into `ChatService.StartPromptWithRunner(...)`

### Problem 5: Generic helper types still leaked LLM details

The first runner iteration still had generic handles carrying things like:

- seed prompt
- allowed tools
- prompt/idempotency fields on shared resolver types

Those are not transport facts. They are runtime or prompt-submission facts.

That kind of leakage usually means the system is still organized around the old use case, even if the new types have more generic names.

## Why The First Attempt Likely Went Wrong

This part matters because the mistake was not random. It followed a recognizable engineering pattern.

### Short version

The first implementation optimized for visible forward motion:

- define the new type
- route the current code through it
- keep everything working

That is a reasonable instinct, but it tends to produce abstraction-first refactors where the naming changes faster than the ownership boundaries.

### More detailed process analysis

I think the earlier implementation likely followed this mental model:

1. “We need a generic process starter.”
2. “The existing chat startup logic already works.”
3. “Let’s extract that logic behind `Runner`.”
4. “To keep the diff small, pass through the existing objects.”

That usually leads to a first version like:

```go
type StartRequest struct {
    Conversation *Conversation
    Payload      any
}
```

and:

```go
type StartResult struct {
    Exec *session.ExecutionHandle
}
```

This is tempting because:

- it compiles quickly
- existing logic moves with fewer edits
- tests are easier to preserve initially

But it quietly bakes old assumptions into the new API.

### The likely reasoning error

The key reasoning mistake was probably:

“If the old code path is now called from `Runner.Start(...)`, then the abstraction exists.”

That is not enough.

An abstraction is only real if:

- it reduces the dependency surface
- it removes the old assumptions from the public boundary
- it preserves the behavioral contract that still matters

The first attempt changed the call graph without first reducing the dependency graph.

### Why this happens in real refactors

This kind of mistake is common when:

- the current behavior is complicated and working
- there is pressure to show the new seam quickly
- the first implementation starts in the middle of the stack instead of at the ownership boundary

In this case, the “middle of the stack” was `startInferenceForPrompt(...)`.

That was the wrong first incision point by itself because the real missing design question was:

“Which layer owns prompt submission policy, and which layer owns generic conversation transport?”

Until that question was answered explicitly, extracting a `Runner` was too early.

### The specific trap around tests

The first implementation also had a natural testing trap:

- if the tests still prove an LLM prompt can start
- and the websocket still gets events
- and the new route compiles

it can look “done enough”

But those tests do not prove the boundary is right. They only prove the happy path still runs.

That is why the rebuild added tests for:

- prompt queue/idempotency preservation
- allowed-tools filtering on the rebuilt runner path
- fake non-LLM runners emitting SEM
- generic prepare paths not eagerly creating LLM state

Those tests are much closer to the architecture claim.

## Why Reverting Was The Right Call

After the review findings, there were two choices:

### Option A: Patch the first version

That would have meant:

- adding LLM-specific special cases to generic helpers
- preserving public types that were already wrong
- adding more adapter code around a bad seam

### Option B: Rewind and replay carefully

That meant:

1. revert the first runner commits
2. go back to the last known-good chat behavior
3. redraw the seam from the ownership model outward

Option B was cheaper and safer.

That is what I did with explicit revert commits:

- `e367174`
- `51d7c29`
- `71ff299`

## How I Approached The Rebuild

The rebuild followed a different sequence than the first attempt.

### Step 1: Keep `Conversation`

I did not try to rename the system to `Process`.

Reason:

- `Conversation` is still the right identity for transport
- the actual missing abstraction was execution startup, not conversation storage

### Step 2: Minimize the runner contract

Before re-extracting any LLM logic, I reduced the runner surface to what a runner genuinely needs:

- IDs
- sink
- timeline emitter
- runtime metadata
- payload
- generic completion handle

That produced the current design in [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go).

### Step 3: Put prompt semantics back in `ChatService`

Instead of making `PrepareRunnerStart(...)` smarter, I restored the correct ownership boundary:

- `ConversationService` is generic
- `ChatService` owns prompt queue/idempotency
- app code chooses the runner

That led to [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go) gaining `StartPromptWithRunner(...)`.

### Step 4: Delay LLM state creation

This was the structural fix that makes generic transport believable.

Instead of creating LLM state during generic conversation ensure, the rebuilt code does:

```text
ensure conversation transport
  -> only transport/conversation state

runner.Start
  -> ensure LLM state lazily if this runner is LLM-backed
```

That logic now lives in [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go).

### Step 5: Re-add the LLM runner as an implementation, not as the architecture

Only after the boundaries were fixed did I add `LLMLoopRunner` back as a concrete runner.

This is why the current LLM runner is much cleaner:

- it decodes its payload
- it fetches its LLM state lazily
- it filters tools
- it emits user-message SEM
- it starts inference
- it returns a generic `RunHandle`

## Before And After

### Before the rebuild

```text
HTTP /chat-runner
  -> PrepareRunnerStart
      -> ensure conversation
      -> still coupled to eager LLM state
  -> runner.Start
      -> gets raw Conversation
      -> returns Geppetto execution handle

Problems:
  - generic API not generic
  - queue/idempotency bypassed
  - attach path still creates LLM state
```

### After the rebuild

```text
Prompt-driven path

HTTP /chat-runner
  -> resolver.Resolve
  -> choose LLMLoopRunner
  -> ChatService.StartPromptWithRunner
      -> ensure conversation
      -> apply queue/idempotency
      -> build StartRequest
      -> runner.Start
      -> wait/drain queue

Generic non-chat path

HTTP /feature-run
  -> resolver.Resolve
  -> choose runner
  -> ConversationService.PrepareRunnerStart
  -> runner.Start
```

### Architectural diagram

```text
                    +-------------------------+
                    |  App-Owned HTTP Layer   |
                    |  feature POST /...      |
                    +-----------+-------------+
                                |
                    chooses runner / runtime
                                |
         +----------------------+----------------------+
         |                                             |
         v                                             v
+-------------------------+               +---------------------------+
| ChatService             |               | ConversationService       |
| prompt queue/idempotency|               | generic prepare / attach  |
+------------+------------+               +-------------+-------------+
             |                                            |
             +-------------------+------------------------+
                                 |
                                 v
                        +------------------+
                        | Runner.Start(...)|
                        +---------+--------+
                                  |
                    +-------------+--------------+
                    | concrete implementation    |
                    | LLMLoopRunner / fake runner|
                    +-------------+--------------+
                                  |
                                  v
                   SEM sink / timeline emitter / turn store
                                  |
                                  v
                         websocket + /api/timeline
```

## File-By-File Guide

### [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go)

Start here.

This file defines the stable runner contract:

- `TimelineEmitter`
- `RunHandle`
- `StartRequest`
- `StartResult`
- `Runner`

Read this file to understand the final public execution seam.

### [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)

This file defines the generic conversation transport boundary:

- ensure conversation
- attach websocket
- prepare `StartRequest`
- emit timeline entities

If you are asking “what is generic?” this file is the answer.

### [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go)

This file defines the prompt-specific policy boundary.

Read this file if you are asking:

- where does idempotency live?
- where does prompt queueing live?
- how does prompt-driven runner startup preserve old behavior?

### [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go)

This file is the bridge from generic conversation transport to LLM execution state.

It exists because the transport layer should not eagerly create engines or sessions.

### [llm_loop_runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_loop_runner.go)

This is the first concrete runner.

It is intentionally specific. That is a feature, not a flaw.

### [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)

This file still contains a small amount of remaining cleanup debt. It supports the new split, but some resolver types still carry prompt-oriented fields.

That is documented as a follow-up, not part of the GP-030 acceptance boundary.

### [app_owned_chat_integration_test.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/cmd/web-chat/app_owned_chat_integration_test.go)

This is the easiest end-to-end review entry point.

It shows:

- legacy chat path
- app-owned LLM runner path
- fake non-LLM runner path
- websocket attach
- timeline hydration

## Pseudocode: Final Recommended Patterns

### Prompt-driven app-owned start

```go
func handleChatRunner(w http.ResponseWriter, r *http.Request) {
    resolved := resolver.Resolve(r)
    runner := webchat.NewLLMLoopRunner(...)

    result, err := chatService.StartPromptWithRunner(ctx, runner, webchat.StartPromptWithRunnerInput{
        Runtime:        resolved.RuntimeRequest(),
        Prompt:         resolved.Prompt,
        IdempotencyKey: resolved.IdempotencyKey,
        Metadata:       resolved.Metadata,
    })

    writeJSON(w, result)
}
```

### Non-chat runner start

```go
func handleFeatureRun(w http.ResponseWriter, r *http.Request) {
    resolved := resolver.Resolve(r)
    runner := newFeatureRunner(...)

    handle, startReq, err := conversationService.PrepareRunnerStart(ctx, webchat.PrepareRunnerStartInput{
        Runtime:  resolved.RuntimeRequest(),
        Payload:  featurePayload,
        Metadata: featureMetadata,
    })
    if err != nil { ... }

    _, err = runner.Start(ctx, startReq)
    if err != nil { ... }

    writeJSON(w, map[string]any{
        "conv_id": handle.ConvID,
    })
}
```

### Lazy LLM state

```go
func ensureLLMState(conv *Conversation) (*llmState, error) {
    if conv.llmState != nil {
        return conv.llmState, nil
    }

    runtime := composeRuntime(conv.runtimeRequest)
    engine := buildEngine(runtime)
    sess := session.New(...)
    seedSystemTurn(sess, runtime)

    conv.llmState = &llmState{...}
    return conv.llmState, nil
}
```

## What I Intentionally Did Not “Fix” In GP-030

A common refactor failure mode is trying to solve the entire next six months of cleanup in one ticket. I did not do that here.

I intentionally left these as follow-ups:

- split prompt/idempotency fields out of shared resolver request types in [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go)
- move private prompt queue storage off `Conversation` entirely if that later proves worth the churn
- further demote older convenience helpers in docs once embedders have migrated

Those are real cleanup opportunities. They are not blockers for the corrected runner boundary.

## Validation Strategy

The rebuild was validated with tests that align to the architectural claims rather than only the happy-path behavior:

- allowed-tool filtering still works on the rebuilt LLM runner path
- turn persistence still works on the rebuilt LLM runner path
- app-owned runner routes still drive websocket and timeline behavior
- fake non-LLM runners can emit SEM through the same transport path
- generic prepare paths do not eagerly create LLM state

Primary review/test files:

- [llm_loop_runner_test.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_loop_runner_test.go)
- [app_owned_chat_integration_test.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/cmd/web-chat/app_owned_chat_integration_test.go)

Validation commands:

```bash
go test ./pkg/webchat/... ./cmd/web-chat -count=1
go test ./... -count=1
make lintmax
docmgr doctor --root /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp --ticket GP-030 --stale-after 30
```

## Lessons For A New Intern

If you take one lesson from this postmortem, it should be this:

Do not mistake “I introduced a new interface” for “I created a clean abstraction.”

When refactoring:

- start from ownership boundaries, not naming
- preserve behavior contracts before generalizing APIs
- shrink dependency surfaces before moving logic behind new types
- do not let the old dominant use case leak through a supposedly generic public API

In this ticket, the successful version came from asking:

1. What is transport?
2. What is prompt policy?
3. What is execution?
4. What must remain generic?
5. What can stay specific?

The failed version started instead from:

1. What new interface name do we want?
2. How can we quickly route old code through it?

That difference in order is the entire story.

## Final Assessment

The rebuilt GP-030 implementation is better not because it is more abstract, but because it is more honest about the system:

- `Conversation` is still transport identity
- `ChatService` still owns prompt semantics
- `Runner` owns execution startup
- `LLMLoopRunner` is specific
- generic transport does not eagerly become LLM runtime state

That is why the rollback was justified, and that is why the rebuilt version is the one we should extend from.
