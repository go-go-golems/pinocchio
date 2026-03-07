---
Title: Diary
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
    - Path: pinocchio/pkg/webchat/conversation_service.go
      Note: Main extraction seam for moving the current LLM startup path behind a Runner
    - Path: pinocchio/pkg/webchat/conversation.go
      Note: Conversation remains the transport identity that runner startup must reuse
    - Path: pinocchio/pkg/webchat/http/api.go
      Note: HTTP helper layer will need to stay compatible while the runner seam is introduced
    - Path: pinocchio/pkg/inference/runtime/composer.go
      Note: Existing runtime composition contract informs the first Runner API shape
ExternalSources: []
Summary: Implementation diary for GP-030: introducing a Runner abstraction into Pinocchio webchat while preserving Conversation as the transport identity.
LastUpdated: 2026-03-07T16:40:00-05:00
WhatFor: Use this diary to capture the implementation steps, commits, validation, and design decisions for GP-030.
WhenToUse: Use when reviewing or continuing the runner extraction work in Pinocchio webchat.
---

# Diary

## Goal

Capture the GP-030 implementation step by step so the runner extraction can be reviewed as a sequence of small, validated changes rather than as one large refactor.

## Context

GP-030 starts from an architecture discussion and design guide. The job now is to convert that into code without breaking the existing generic transport layer or the current `cmd/web-chat` behavior.

## Step 1: Convert GP-030 into an implementation ticket

I started by turning GP-030 from a design-only ticket into an execution ticket. That means adding a diary, fixing the ticket index so it points at ongoing implementation notes, and collapsing the earlier open questions into explicit standing decisions before code changes begin.

This matters because the code work depends on a few architectural choices already made in the discussion: keep `Conversation`, keep runner instantiation app-owned, and use the current LLM startup path as the first extraction target. Without locking those in, the implementation would drift between alternative designs.

### Prompt Context

**User prompt (verbatim):** "ok, we merged and rebased and pinoicchio is all ready to tackle GP-030, close GP-029, then commit GP-030, and then work on it task by task, comitting at appropriate intervals, and keeping a detailed ddiary."

**Assistant interpretation:** Close out the finished GP-029 ticket, turn GP-030 into an active implementation ticket with a diary and executable tasks, commit that ticket state, and then begin implementing GP-030 incrementally with diary updates and commits.

**Inferred user intent:** Move from design to execution in a disciplined way, with ticket state, diary history, and code changes staying synchronized throughout the refactor.

**Commit (code):** pending

### What I did

- Marked GP-029 as closed in its ticket index.
- Updated the GP-030 index to point at this diary.
- Expanded the GP-030 task list into a real execution backlog with phase ordering.
- Converted previous open questions into standing implementation decisions.
- Created this diary document.

### Why

- The user explicitly asked to close GP-029 before starting GP-030.
- GP-030 previously described phases and alternatives but did not yet read like an implementation ticket.
- The first code slice needs the task order and design decisions captured before the refactor starts.

### What worked

- The design guide already contained enough detail to translate directly into code-oriented tasks.
- The earlier discussion provided clear answers for the most important architectural choices.

### What didn't work

- GP-030 did not yet have a diary or a single clear “phase 1 starts here” task sequence, so I had to create that structure first.

### What I learned

- The most important implementation boundary is still `ConversationService.startInferenceForPrompt(...)`.
- The most useful first increment is to introduce runner types and extract the current LLM path without changing transport ownership.

### What was tricky to build

- The subtle part is keeping runner instantiation app-owned while still exposing enough service-level help that embeddings do not have to reach through private `ConvManager` internals.

### What warrants a second pair of eyes

- Whether the first public helper after extraction should be a minimal `StartRequest` builder or a higher-level `StartWithRunner(...)` method.

### What should be done in the future

- Implement the core `Runner` API and extract the LLM startup path behind it in the next step.

### Code review instructions

- Start with the ticket docs under `pinocchio/ttmp/.../GP-030--webchat-process-oriented-core-brainstorm/`.
- Confirm the task ordering matches the design guide before reviewing the first code refactor.

### Technical details

- GP-029 closure is ticket-only state.
- GP-030 now has a dedicated diary and explicit phase tasks for runner extraction, service wiring, and HTTP adaptation.

## Step 2: Extract the current LLM startup path behind `LLMLoopRunner`

The first code increment was to add the shared runner types and move the body of the current LLM startup path out of `ConversationService.startInferenceForPrompt(...)` and into a dedicated `LLMLoopRunner`. I kept the external chat behavior intact so the refactor could be reviewed as boundary extraction rather than product behavior change.

This slice matters because it creates the first real seam between generic conversation transport ownership and the LLM loop startup logic. Without that seam, there is no credible way to let embedding applications choose their own runner while still reusing the existing `Conversation`, websocket, and timeline machinery.

### What I did

- Added `Runner`, `StartRequest`, `StartResult`, `TimelineEmitter`, and `PrepareRunnerStartInput` in `pkg/webchat/runner.go`.
- Added `pkg/webchat/llm_loop_runner.go` with `LLMLoopRunner` and `LLMLoopStartPayload`.
- Moved the current LLM start sequence out of `ConversationService.startInferenceForPrompt(...)` into `LLMLoopRunner.Start(...)`.
- Added `ConversationService.PrepareRunnerStart(...)` so app-owned code can ensure a conversation and receive `Sink` plus `TimelineEmitter` without reaching through internal globals.
- Added a regression test covering the ensured-conversation surfaces passed into the runner path.

### Why

- The user asked to work GP-030 task by task rather than jumping to a final architecture in one change.
- The design already called for runner instantiation to stay app-owned.
- The existing chat submission path still needs to preserve queue and idempotency semantics while the startup seam is extracted.

### What worked

- The extracted runner could reuse the existing `Conversation`, `Session`, and `StreamHub` ownership model directly.
- `PrepareRunnerStart(...)` was enough to expose the conversation-bound IO surfaces cleanly without opening up the whole `ConvManager`.
- Existing tests around prompt submission and queue behavior continued to work after the refactor.

### What didn't work

- The HTTP layer is still chat-shaped. `NewChatHandler(...)` still talks only in terms of `SubmitPrompt(...)`, so the embedding story is not yet explicit at the route/helper layer.

### What I learned

- The constructor-vs-request-vs-context split works in practice:
- runner constructor holds long-lived shared services
- `StartRequest` carries the per-conversation sink and timeline surfaces
- `context.Context` only needs to carry cancellation and request scope

### What was tricky to build

- Preserving the existing idempotency behavior without pushing queue semantics into the generic runner boundary. That logic still belongs in `SubmitPrompt(...)`, not in `Runner.Start(...)`.

### What warrants a second pair of eyes

- Whether `PrepareRunnerStart(...)` is the right minimal helper name and shape for embedders, or whether phase 3 should add a higher-level convenience around it.

### What should be done in the future

- Adapt the HTTP and embedding surface so there is a documented runner-first startup path while keeping `NewChatHandler(...)` as a compatibility convenience layer.

### Code review instructions

- Review `pkg/webchat/runner.go` first to confirm the core boundary.
- Then review `pkg/webchat/llm_loop_runner.go` to verify the moved startup logic is complete.
- Finally review `pkg/webchat/conversation_service.go` to confirm queue/idempotency behavior still stays outside the generic runner.

### Validation

- `go test ./pkg/webchat -count=1`
- `go test ./pkg/webchat/... ./cmd/web-chat -count=1`

### Commit

- `eed8f09` `refactor: extract webchat llm loop runner`

## Step 3: Add the runner-first embedding path and close the transport gap in `TimelineEmitter`

The next increment was to make the runner architecture actually usable from an app-owned handler. The code already had the internal seam, but an embedding app still lacked one small piece: a way to instantiate the standard `LLMLoopRunner` from the existing webchat service wiring without reconstructing internal dependencies by hand.

While wiring the runner-first integration tests, I also found a real contract mismatch: `TimelineEmitter` was described as the durable timeline surface for runners, but the first adapter only re-emitted `timeline.upsert` SEM and did not persist entities into the timeline store. That meant a fake runner could emit events but `/api/timeline` hydration would not reflect them. I fixed that before moving on.

### What I did

- Added `ChatService.PrepareRunnerStart(...)` as a public pass-through for embedding apps.
- Added `ConversationService.NewLLMLoopRunner(...)` and `ChatService.NewLLMLoopRunner(...)` so app-owned handlers can choose the built-in LLM runner without rebuilding internal dependencies.
- Added `ResolvedConversationRequest.RuntimeRequest()` in `pkg/webchat/http` so handler code can move cleanly from request resolution into `PrepareRunnerStart(...)`.
- Updated `NewChatHandler(...)` documentation to state that it is the convenience adapter, not the only startup model.
- Fixed the `TimelineEmitter` adapter in `ConversationService.startRequestForConversation(...)` so it persists to the configured timeline store before fanout.
- Added direct integration coverage for:
- app-owned `PrepareRunnerStart(...) + LLMLoopRunner.Start(...)`
- websocket frames from the direct runner path
- timeline hydration from the direct runner path
- a fake runner that writes a timeline entity through the generic runner surfaces
- Added an allowed-tools regression test for `LLMLoopRunner`.
- Updated webchat framework docs and the `cmd/web-chat` README to document the runner-first route composition.

### Why

- GP-030 is not useful if the runner boundary exists only as an internal refactor.
- The app-owned handler story needs one clear, supported path.
- `TimelineEmitter` had to be made truthful before fake or non-LLM runners could rely on it.

### What worked

- `PrepareRunnerStart(...)` plus a service-owned `NewLLMLoopRunner(...)` is enough for an app-owned route to choose and start the standard LLM runner.
- The direct runner path reuses the same websocket and `/api/timeline` contract without any route changes on the frontend side.
- The fake runner test now proves that a runner can write timeline state without using the full LLM loop.

### What didn't work

- The first fake-runner attempt showed that `TimelineEmitter` only fanned out SEM and did not persist the upsert. The timeline hydration test failed until that adapter was fixed.

### What I learned

- The minimal public embedding surface is:
- request resolver output
- `PrepareRunnerStart(...)`
- one service-owned constructor for the standard runner
- the existing generic `/ws` and `/api/timeline`

### What was tricky to build

- The subtle issue was not route wiring but contract truthfulness. Calling something a durable timeline emitter while only publishing fanout events would have created a misleading API for future runners.

### What warrants a second pair of eyes

- Whether future custom `timelineUpsert` hooks should remain fanout-only or whether the code should eventually formalize a separate persistence-plus-fanout hook type instead of composing the store write inside the adapter.

### What should be done in the future

- Decide whether the next phase should keep extending `ConversationService` helpers or introduce a narrower embedding-facing starter interface once there are multiple concrete runner types.

### Code review instructions

- Review `pkg/webchat/conversation_service.go` for the new `NewLLMLoopRunner(...)` helper and the fixed timeline persistence behavior.
- Review `pkg/webchat/http/api.go` for the resolved-request conversion helper and the clarified handler role.
- Review `cmd/web-chat/app_owned_chat_integration_test.go` for the direct runner and fake runner composition examples.

### Validation

- `go test ./pkg/webchat/... -count=1`
- `go test ./cmd/web-chat -count=1`

### Commit

- pending

## Step 4: Validate and close the first GP-030 implementation slice

After the runner-first work landed, I ran the focused validation pass and closed the ticket slice. At this point the ticket has code, docs, examples, tests, and a passing ticket doctor run.

### Validation

- `go test ./pkg/webchat/... -count=1`
- `go test ./cmd/web-chat -count=1`
- `go test ./pkg/doc -count=1`
- `docmgr doctor --root /home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/ttmp --ticket GP-030 --stale-after 30`

### Outcome

- The first GP-030 implementation slice is complete and the ticket is now closed.
