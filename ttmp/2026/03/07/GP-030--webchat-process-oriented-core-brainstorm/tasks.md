# Tasks

## Completed

- [x] Create Pinocchio ticket workspace under `pinocchio/ttmp`
- [x] Write brainstorm document capturing the discussion state, alternatives, and questions
- [x] Prepare the ticket for follow-up design work
- [x] Update the brainstorm with the latest runner wiring and dependency-passing guidance
- [x] Write a detailed intern-facing design and implementation guide for a `Conversation` + `Runner` architecture
- [x] Refresh the ticket index to point at the new design guide
- [x] Decide the standing direction for implementation: keep `Conversation`, keep runner instantiation app-owned, and extract the current LLM startup path first

## Completed

- [x] Create an implementation diary and turn the design guide into an executable backlog
- [x] Rewind the first `Runner` implementation slice after review found that generic transport code still leaked LLM session and prompt-submission concerns
- [x] Rebuild the core `Runner` API around a transport-safe start envelope and a generic completion handle
- [x] Move prompt queue/idempotency back behind `ChatService` while keeping `ConversationService` transport-oriented
- [x] Rebuild the LLM path with lazy execution-state creation instead of eager session creation during conversation ensure
- [x] Adapt app-owned start surfaces while preserving current chat behavior
- [x] Write a migration guide for embedders moving from legacy chat startup to the runner path
- [x] Write a detailed postmortem explaining the rollback and rebuild
- [x] Validate, document, upload, and close the first GP-030 implementation slice

## Key Decisions

- [x] Keep `Conversation` as the transport identity instead of renaming the subsystem to `Process`
- [x] Keep runner instantiation in the app-owned handler/composition layer
- [x] Use `Sink + TimelineEmitter` rather than a unified `RunIO` interface in phase 1
- [x] Keep chat-oriented HTTP convenience helpers during the extraction, but treat them as adapters over the runner path rather than the core model
- [x] Use typed payload structs where the implementation already knows the runner kind, starting with an `LLMLoopRunner`

## Phase 1: Ticket and diary setup

- [x] Add a diary reference document for step-by-step implementation notes
- [x] Update the ticket index to link the diary and implementation status
- [x] Refine the phase ordering so code tasks can be completed and committed incrementally

## Phase 2: Introduce Runner

### 2.1 Core runner API

- [x] Define `Runner` interface shape and document its contract
- [x] Define `StartRequest` as a transport-safe envelope without raw `*Conversation` access
- [x] Define `StartResult` around a generic completion handle rather than a Geppetto execution handle
- [x] Add `TimelineEmitter` and its first concrete adapter over the existing timeline-upsert hook
- [x] Add an LLM-specific typed payload struct for the first runner extraction

Acceptance criteria:

- there is one documented API shape for `Runner`, `StartRequest`, and `StartResult`
- the API makes the constructor-vs-request-vs-context split explicit
- the design does not require raw websocket objects, generic `*Conversation` access, or hidden `context.Context` dependencies

### 2.2 Extract the current LLM startup seam

- [x] Identify the exact logic in `ConversationService.startInferenceForPrompt(...)` that belongs in an `LLMLoopRunner`
- [x] Separate generic conversation resolution from LLM-loop-specific startup
- [x] Preserve current idempotency and queue semantics around prompt submission
- [x] Preserve current allowed-tool filtering behavior from composed runtime state
- [x] Preserve current runtime fingerprint and runtime-key handling in responses and internal state
- [x] Move LLM session construction behind lazy execution-state creation instead of generic conversation ensure

Acceptance criteria:

- there is a clear code path where conversation ensuring happens before runner execution
- there is no eager LLM session creation during generic conversation ensuring
- LLM-loop startup is implemented behind a runner-shaped boundary
- no transport behavior regresses while extracting the startup logic

### 2.3 Add runner-aware service wiring

- [x] Keep runner startup app-composed in the public recommendation, but add the minimum service helper needed to build a `StartRequest` from an ensured conversation
- [x] Implement the helper that resolves `ConvID`/`SessionID`, looks up the ensured `Conversation`, and provides `Sink` plus `TimelineEmitter`
- [x] Keep prompt queue and idempotency semantics in the chat submission path rather than generic runner startup
- [x] Ensure runner startup can reuse the existing `Conversation`, `ConvManager`, and `StreamHub` lifecycle without duplicating ownership logic

Acceptance criteria:

- there is exactly one recommended startup path for embeddings
- the service boundary makes it obvious who owns conversation resolution, prompt submission policy, and runner execution
- the runner can receive all required per-conversation IO surfaces without reaching into internal globals

### 2.4 Add tests for runner extraction

- [x] Add unit tests covering `Runner` startup with ensured conversation identity
- [x] Add tests that verify `ConvID`, `SessionID`, and runtime metadata are correctly propagated to the runner
- [x] Add tests that preserve idempotency behavior for repeated prompt submissions
- [x] Add tests that preserve queue behavior while a previous request is running
- [x] Add tests that preserve allowed-tools filtering and tool registration behavior
- [x] Add tests that generic resolve/prepare paths do not eagerly create LLM execution state

Acceptance criteria:

- tests cover the old LLM path after extraction behind the runner abstraction
- failures localize clearly to runner startup vs transport attach/hydration

## Phase 3: Adapt App-Owned Start Surface

### 3.1 Define embedding pattern for app-owned handlers

- [x] Write down the canonical composition sequence for an embedding application:
- [x] parse request
- [x] resolve domain/runtime policy
- [x] instantiate or choose runner
- [x] ensure conversation
- [x] build `StartRequest`
- [x] call `Runner.Start(...)`
- [x] return `conv_id` and related metadata to the client
- [x] Clarify which steps are Pinocchio responsibilities versus embedding-app responsibilities
- [x] Document the minimal interface an embedding app needs from Pinocchio to follow this pattern

Acceptance criteria:

- the embedding story is explicit enough that a new app does not need to infer runner ownership from examples
- the design clearly keeps feature semantics in the app-owned handler

### 3.2 Refactor HTTP helper expectations

- [x] Review [http/api.go](/home/manuel/workspaces/2026-03-02/deliver-mento-1/pinocchio/pkg/webchat/http/api.go) and decide which pieces remain generic helper code versus chat-specific convenience
- [x] Decide whether `NewChatHandler(...)` remains as an LLM convenience adapter over the new runner path
- [x] Ensure websocket attach and timeline hydration handlers remain generic and unchanged in contract
- [x] Ensure the request resolver contract still works when the app owns more of the start semantics
- [x] Document the intended route split in tests/examples:
- [x] feature-owned `POST /...`
- [x] generic `GET /ws`
- [x] generic `GET /api/timeline`

Acceptance criteria:

- generic transport endpoints remain stable
- any remaining chat-specific helper is explicitly described as a convenience layer, not as the only startup model

### 3.3 Update examples and embedding guidance

- [x] Update at least one example embedding to demonstrate app-owned runner instantiation
- [x] Show how a feature-specific endpoint can start an LLM-backed runner while still using generic `/ws` and `/api/timeline`
- [x] Add one documented example of a non-LLM or fake runner emitting SEM over the same transport path
- [x] Ensure example docs explain why one websocket per active conversation remains the default recommendation
- [x] Publish a dedicated migration guide in `pkg/doc/topics`

Acceptance criteria:

- there is at least one runnable or near-runnable example that demonstrates the intended architecture
- the examples make it obvious that the generic transport layer is reusable across feature-specific start endpoints

### 3.4 Add end-to-end regression coverage

- [x] Add integration coverage for an LLM-backed feature start through the new runner path
- [x] Verify websocket attachment still receives SEM frames for the started conversation
- [x] Verify `/api/timeline` hydrates the same conversation after startup
- [x] Verify turn snapshots or existing turn persistence behavior still works for the LLM path
- [x] Add a smoke test for a fake runner that emits SEM without relying on the full LLM loop

Acceptance criteria:

- the refactor is proven to preserve current LLM webchat behavior
- the new abstraction is proven capable of supporting a non-LLM SEM producer
