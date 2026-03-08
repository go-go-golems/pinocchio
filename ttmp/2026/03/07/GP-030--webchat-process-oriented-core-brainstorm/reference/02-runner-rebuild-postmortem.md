---
Title: GP-030 Runner Rebuild Postmortem
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
LastUpdated: 2026-03-07T19:10:00-05:00
WhatFor: Use this postmortem to understand the failed first GP-030 attempt, the corrected implementation, and the reasoning behind the final code boundaries.
WhenToUse: Use when reviewing the runner refactor, onboarding a new contributor, or planning the next cleanup steps in Pinocchio webchat.
---

# GP-030 Runner Rebuild Postmortem

## Executive Summary

The first GP-030 implementation introduced a `Runner` API, but it did not actually separate generic transport from LLM-specific execution. The public runner path still carried raw `*Conversation` state, generic conversation ensure still created LLM session state eagerly, and the new app-owned start path bypassed the queue/idempotency behavior that the old `SubmitPrompt(...)` path preserved.

That implementation was rolled back with explicit `git revert` commits and rebuilt from the pre-runner baseline. The corrected version keeps the useful part of the original idea, which is the `Runner` seam, but fixes the boundary:

- `Conversation` remains the transport identity
- `StartRequest` is transport-safe
- `StartResult` uses a generic completion handle
- prompt queue/idempotency stays in `ChatService`
- LLM execution state is created lazily in `llm_state.go`

This document explains what went wrong, what was rebuilt, why the new version is better, and how to review it.

## The Original Problem

The product need was valid. We wanted:

- app-owned feature start endpoints
- generic websocket attach and timeline hydration
- a way to start an LLM loop or another SEM-emitting process without hard-wiring everything to `SubmitPrompt(...)`

The first implementation moved quickly toward a `Runner` abstraction, but it accidentally preserved the wrong dependencies:

- generic runner start still exposed `*Conversation`
- runner results still exposed Geppetto execution handles
- queue/idempotency stayed only on the old legacy path
- generic conversation ensure still built chat-oriented session state

So the first version had a `Runner` type, but not a genuinely generic runner boundary.

## What Was Wrong With The First Attempt

### 1. The public runner API was not actually generic

The first version of `StartRequest` carried a raw `*Conversation`. That meant any runner could reach into conversation internals:

- session state
- engine state
- stream lifecycle
- tool exposure

The first version of `StartResult` also exposed Geppetto execution handles directly. That made the "generic" API implicitly LLM-loop-shaped.

### 2. Generic conversation ensure still created LLM state eagerly

The old `ConvManager.GetOrCreate(...)` path did too much:

- composed runtime
- created engine
- created `session.Session`
- seeded the first system turn

That meant websocket-first attachment could create full LLM runtime/session state before anything had actually started.

### 3. The new runner path lost queue/idempotency behavior

The old `SubmitPrompt(...)` path did this:

1. ensure conversation
2. apply queue/idempotency
3. start inference

The first runner path did this:

1. ensure conversation
2. return a runnable request
3. start immediately

That sounds cleaner, but it broke an important behavior contract. Two fast prompt submissions for the same `conv_id` no longer naturally shared the old `202 queued` / replay behavior.

### 4. Generic transport response types still carried LLM details

The old `ConversationHandle` exposed LLM-specific fields like seed prompt and allowed tools. Those are useful to the LLM runner, but not to generic transport attach or timeline hydration.

## Why Rollback Was Better Than Patching

Once the review findings were clear, patching the first version would have meant:

- special-casing `LLMLoopStartPayload` inside a supposedly generic helper
- preserving a bad public API because tests were already written against it
- adding more compatibility glue on top of the wrong seam

At that point, the cheapest correct move was:

1. revert the first runner slice
2. keep GP-029 intact
3. rebuild the runner seam from the old baseline

That is exactly what happened.

## What The Corrected Design Looks Like

## 1. Conversation still exists

`Conversation` still means:

- `conv_id`
- `session_id`
- websocket attach state
- stream buffering
- timeline projection
- runtime metadata

That part was never the problem.

## 2. StartRequest is now transport-safe

The new public runner input is in [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go).

It contains:

- `ConvID`
- `SessionID`
- `RuntimeKey`
- `RuntimeFingerprint`
- `Sink`
- `Timeline`
- `Payload`
- `Metadata`

It does not expose:

- raw `*Conversation`
- raw Geppetto session state
- websocket objects

That is the correct public boundary.

## 3. StartResult uses a generic completion handle

The new runner result returns a generic `RunHandle`:

```go
type RunHandle interface {
    Wait() error
}
```

This was the key move for preserving queue draining without leaking Geppetto-specific types. `ChatService` only needs to know when the run is done. It does not need a raw execution handle.

## 4. ChatService owns prompt queue/idempotency again

The corrected `ChatService` now owns:

- `SubmitPrompt(...)`
- `StartPromptWithRunner(...)`
- queue preparation
- queue drain
- prompt completion bookkeeping

That fixes the regression from the first runner attempt. Prompt-submission policy is chat-owned again, but runner selection is now app-owned.

## 5. LLM state is lazy

The new [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go) creates LLM execution state only when the LLM runner needs it:

- runtime compose
- engine creation
- `session.Session`
- seed turn

Generic conversation ensure no longer does that work eagerly.

## 6. The LLM runner is now a proper implementation detail

The new [llm_loop_runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_loop_runner.go) is where the LLM-specific logic belongs:

- payload decoding
- tool filtering
- prompt append
- user `chat.message` SEM event
- builder setup
- inference start
- generic run-handle wrapping

That logic is no longer pretending to be generic transport code.

## File-By-File Walkthrough

### [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go)

This is the public generic runner seam.

Read this file first when reviewing the rebuild.

It defines:

- `TimelineEmitter`
- `RunHandle`
- `StartRequest`
- `StartResult`
- `Runner`
- `PrepareRunnerStartInput`

### [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)

This file is generic again.

It now focuses on:

- ensure conversation
- attach websocket
- build a transport-safe `StartRequest`
- emit timeline upserts

It no longer owns the core prompt-start logic.

### [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go)

This file now clearly owns prompt-driven behavior:

- prompt queue/idempotency
- legacy `SubmitPrompt(...)`
- new `StartPromptWithRunner(...)`
- prompt completion bookkeeping
- queue drain

This is the correct home for prompt-specific semantics.

### [conversation.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation.go)

This file no longer creates `session.Session` eagerly in `GetOrCreate(...)`.

It still stores private prompt queue fields to avoid collateral breakage in debug and eviction code. That is a compromise, not the final ideal, but the orchestration logic is no longer exposed as generic `Conversation` methods.

### [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go)

This file is the new internal bridge between generic conversation transport and LLM-specific execution state.

Its job is:

- look at conversation/runtime metadata
- compose runtime lazily
- construct the internal LLM state only when needed

### [llm_loop_runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_loop_runner.go)

This is the first concrete runner implementation.

The main review points are:

- payload decoding
- allowed-tool filtering
- prompt append and user-message emission
- builder setup
- wrapping the underlying execution with `RunHandle`

### [app_owned_chat_integration_test.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/cmd/web-chat/app_owned_chat_integration_test.go)

This is the easiest place to see the intended external usage.

It now covers:

- legacy `POST /chat`
- app-owned `POST /chat-runner`
- app-owned fake runner path
- websocket attach
- timeline hydration

## The New Control Flow

### Prompt-driven app-owned handler

```text
HTTP handler
  -> resolver.Resolve(...)
  -> choose runner
  -> ChatService.StartPromptWithRunner(...)
       -> ensure conversation transport
       -> apply queue/idempotency
       -> build StartRequest
       -> runner.Start(...)
       -> wait for completion
       -> drain queued prompt if needed
```

### Non-chat runner path

```text
HTTP handler
  -> resolver.Resolve(...)
  -> choose runner
  -> ConversationService.PrepareRunnerStart(...)
  -> runner.Start(...)
```

### LLM runner internals

```text
LLMLoopRunner.Start(...)
  -> resolve conversation by conv_id
  -> ensure lazy LLM state
  -> filter allowed tools
  -> append user turn
  -> emit chat.message SEM
  -> start inference
  -> return StartResult + RunHandle
```

## What Was Intentionally Not Solved Yet

This rebuild fixed the most important architectural mistakes, but it deliberately did not try to finish every cleanup in one step.

Still-open cleanup areas:

- `ResolvedConversationRequest` in `pkg/webchat/http/api.go` still carries prompt/idempotency fields shared with websocket resolution
- private prompt queue storage is still on `Conversation`, even though the orchestration moved into `ChatService`
- some docs still describe the old convenience path as if it were the only path

These are follow-up cleanups, not reasons to reject the rebuilt boundary.

## How To Review The Result

Start here:

1. [runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/runner.go)
2. [conversation_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/conversation_service.go)
3. [chat_service.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/chat_service.go)
4. [llm_state.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_state.go)
5. [llm_loop_runner.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/llm_loop_runner.go)

Then validate with:

```bash
go test ./pkg/webchat/... ./cmd/web-chat -count=1
go test ./... -count=1
make lintmax
docmgr doctor --root /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp --ticket GP-030 --stale-after 30
```

## Final Assessment

The rebuilt GP-030 slice is materially better than the first attempt because it fixed the actual boundary instead of layering compatibility code over the wrong abstraction.

The new result is:

- generic enough for non-LLM runners
- specific enough to preserve chat queue semantics
- safer for websocket-first transport attachment
- easier to extend because the public seam now matches the real ownership boundaries

That is why the rollback was the right call.
