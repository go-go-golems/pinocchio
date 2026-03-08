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

**Commit (code):** `812b964` — "refactor: rebuild webchat runner boundary"

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

## Step 2: Rewind the first runner attempt and rebuild the seam

I rewound the first GP-030 code slice after reviewing the boundary problems. The rollback was targeted: revert the three runner-path commits, keep GP-029 intact, then rebuild from the pre-runner baseline with a stricter split between generic conversation transport, chat-owned queueing, and lazy LLM execution state.

The key implementation decision in this step was not to abandon the `Runner` idea, but to stop letting the generic runner API expose raw `*Conversation` state or Geppetto-specific execution handles. Instead, the rebuild uses a transport-safe `StartRequest`, a generic `RunHandle`, and a chat-specific `StartPromptWithRunner(...)` path that preserves queue/idempotency behavior.

### Prompt Context

**User prompt (verbatim):** "alright, rewind with git then and address things."

**Assistant interpretation:** Revert the problematic GP-030 runner refactor commits, then rebuild the runner architecture with the cleaner boundaries we discussed instead of patching the bad version in place.

**Inferred user intent:** Recover from the first incorrect implementation quickly, keep the repo history explicit, and continue toward a cleaner GP-030 implementation without carrying forward the architectural mistakes.

**Commit (code):** `812b964` — "refactor: rebuild webchat runner boundary"

### What I did
- Reverted `f0505a9`, `be3fca8`, and `eed8f09` with `git revert --no-edit ...`
- Updated the GP-030 task list to reflect rollback + rebuild work instead of pretending the first slice was done
- Reintroduced `Runner` with a transport-safe `StartRequest` and a generic `RunHandle`
- Added lazy LLM execution state in `pkg/webchat/llm_state.go` so generic conversation ensure no longer creates a `session.Session`
- Moved runner-preparation back behind `ConversationService.PrepareRunnerStart(...)`
- Moved prompt queue/idempotency orchestration into `ChatService.StartPromptWithRunner(...)`
- Reintroduced `LLMLoopRunner`, but made it resolve its execution state by `conv_id` instead of receiving a raw `*Conversation`
- Added tests for lazy LLM state creation, prompt queue preservation on the new path, and app-owned integration routes

### Why
- The first runner attempt leaked LLM-specific state through generic APIs and bypassed queue/idempotency on the new app-owned path
- Reverting was cheaper than untangling the bad boundary in place
- The corrected design keeps generic transport reusable while preserving current prompt-submission behavior

### What worked
- `git revert` cleanly removed the first runner-path slice without disturbing GP-029
- Rebuilding from the old baseline made the new seam much easier to reason about
- The new queue-preserving path fits naturally in `ChatService`
- The revised runner API compiled and passed targeted package tests once the old queue helpers and tests were updated

### What didn't work
- The first compile pass after the refactor failed because old tests and helper code still referenced removed fields/methods:
  `pkg/webchat/chat_service.go:12:2: toolloop redeclared in this block`
  `pkg/webchat/conv_manager_eviction.go:112:15: conv.isBusyLocked undefined`
  `pkg/webchat/conversation.go:284:5: c.ensureQueueInitLocked undefined`
- After fixing those, the next pass failed because tests still expected eager LLM state on `Conversation`:
  `pkg/webchat/conversation_service_test.go:73:3: unknown field Sess in struct literal of type Conversation`
  `pkg/webchat/conversation_service_test.go:135:34: handle.SeedSystemPrompt undefined`
  `pkg/webchat/send_queue_test.go:26:20: conv.PrepareSessionInference undefined`

### What I learned
- The real issue was not “how to keep idempotency” but “which layer owns prompt submission policy”
- A generic `RunHandle` is enough for queue-drain orchestration; exposing Geppetto handles was unnecessary
- Lazy LLM state creation is the key move that keeps websocket-first conversation attachment generic

### What was tricky to build
- The sharp edge was queue draining: the new app-owned path still needed a way to wait for completion and start the next queued prompt without reintroducing Geppetto types into the generic runner API. The fix was to add a generic `RunHandle` with `Wait() error` and let `ChatService` own the drain loop.
- The second sharp edge was deciding how far to move prompt state out of `Conversation`. I kept the storage private on `Conversation` for now to avoid collateral breakage in eviction/debug code, but moved the orchestration functions into chat-owned code so the public boundary is cleaner.

### What warrants a second pair of eyes
- Whether `Conversation` should keep the private prompt queue fields long-term or whether those should move into a dedicated chat submission store
- Whether `ResolvedConversationRequest` in `pkg/webchat/http/api.go` should be split next so websocket/runtime resolution stops carrying prompt fields
- Whether current coverage is enough for allowed-tool filtering on the rebuilt runner path

### What should be done in the future
- Finish the remaining documentation/migration work for the corrected runner path
- Decide whether to fully split runtime-resolution request types in `pkg/webchat/http`

### Code review instructions
- Start with `pkg/webchat/runner.go`, `pkg/webchat/chat_service.go`, `pkg/webchat/llm_state.go`, and `pkg/webchat/llm_loop_runner.go`
- Then review the state changes in `pkg/webchat/conversation.go` and the generic helper changes in `pkg/webchat/conversation_service.go`
- Validate with:
  `go test ./pkg/webchat/... ./cmd/web-chat -count=1`

### Technical details
- Revert commits:
  - `e367174` `Revert "chore: deprecate legacy webchat chat startup path"`
  - `51d7c29` `Revert "refactor: add runner-first webchat embedding path"`
  - `71ff299` `Revert "refactor: extract webchat llm loop runner"`
- New/updated code paths:
  - `pkg/webchat/runner.go`
  - `pkg/webchat/llm_state.go`
  - `pkg/webchat/llm_loop_runner.go`
  - `pkg/webchat/chat_service.go`
  - `pkg/webchat/conversation_service.go`
  - `cmd/web-chat/app_owned_chat_integration_test.go`

## Step 3: Close the coverage gaps on the rebuilt runner path

Once the rebuilt boundary was stable, I added the remaining coverage that mattered for trusting it. The important missing assertions were allowed-tool filtering on the rebuilt `LLMLoopRunner`, turn persistence still firing on the runner path, and websocket delivery for an app-owned runner endpoint instead of only the legacy `/chat` helper.

This step matters because the first rebuild commit proved the new boundary compiled and kept the package tests green, but it still left open whether the LLM-specific behavior had actually survived the refactor. These tests close that gap.

### Prompt Context

**User prompt (verbatim):** (same as Step 2)

**Assistant interpretation:** Continue the GP-030 rebuild work and finish the remaining verification instead of stopping at the first passing compile/test state.

**Inferred user intent:** Ensure the refactor is defended by behavior-level tests, not just by cleaner types.

**Commit (code):** `22bbd31` — "test: extend webchat runner coverage"

### What I did
- Added `pkg/webchat/llm_loop_runner_test.go` to assert:
  - allowed-tool filtering still works
  - turn-store persistence still fires on the runner path
- Extended `cmd/web-chat/app_owned_chat_integration_test.go` with:
  - app-owned `POST /chat-runner`
  - fake runner timeline hydration
  - websocket `chat.message` delivery on the runner-owned route
- Re-ran:
  - `go test ./pkg/webchat/... ./cmd/web-chat -count=1`
  - `go test ./... -count=1`
  - `make lintmax`

### Why
- The migration is only credible if the rebuilt runner path preserves the old LLM behavior where it is supposed to
- App-owned routes needed their own coverage instead of relying on the legacy helper path

### What worked
- The runner-path websocket test passed without more production changes
- The LLM runner still filtered tools correctly after the lazy-state refactor
- Turn-store persistence still triggered on the rebuilt path

### What didn't work
- N/A

### What I learned
- The new boundary is strong enough that most of the remaining confidence work could be done with tests rather than more production refactors
- The fake-runner plus timeline test is a good proof that the generic transport story is now real

### What was tricky to build
- The subtle part was asserting turn persistence without reintroducing Geppetto-specific handles into the public runner API. The solution was to use a recording turn store and validate the side effect after `RunHandle.Wait()` rather than trying to inspect internal execution handles.

### What warrants a second pair of eyes
- Whether the current turn-persistence assertion is the right long-term check, or whether a higher-level persisted-turn readback test should eventually replace it

### What should be done in the future
- Finish the migration and postmortem docs for embedders and reviewers

### Code review instructions
- Start with `pkg/webchat/llm_loop_runner_test.go`
- Then review the new runner-route tests in `cmd/web-chat/app_owned_chat_integration_test.go`
- Validate with:
  - `go test ./pkg/webchat/... ./cmd/web-chat -count=1`
  - `go test ./... -count=1`
  - `make lintmax`

### Technical details
- New code review entry points:
  - `pkg/webchat/llm_loop_runner_test.go`
  - `cmd/web-chat/app_owned_chat_integration_test.go`
