---
Title: Implementation Diary
Ticket: PIN-20260521-RPC-STDIN-MULTITURN
Status: active
Topics:
    - pinocchio
    - chatapp
    - rpc
    - sessionstream
    - cli
    - persistence
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/pinocchio/doc/general/06-rpc-jsonl-output.md
      Note: User-facing stdin RPC documentation
    - Path: pkg/chatapp/rpc/jsonl/fanout.go
      Note: Request-id-aware JSONL fanout
    - Path: pkg/chatapp/rpc/jsonl/writer.go
      Note: Request-id line helper
    - Path: pkg/chatapp/runtime_inference.go
      Note: InitialTurn and OnFinalTurn semantics that shape the design
    - Path: pkg/chatapp/service.go
      Note: PromptRequest API used by proposed multi-turn adapter
    - Path: pkg/cmds/cmd.go
      Note: |-
        Current one-shot RPC implementation and future stdin RPC entry point
        Current RPC implementation inspected for Step 1
        Stdin RPC server and run mode dispatch
    - Path: pkg/cmds/cmd_rpc_stdin_test.go
      Note: Multi-turn stdin RPC tests
    - Path: pkg/cmds/cmdlayers/helpers.go
      Note: stdin-rpc helper flag
    - Path: pkg/cmds/run/context.go
      Note: Reader and RunModeRPCStdin
    - Path: proto/pinocchio/chatapp/rpc/v1/rpc.proto
      Note: |-
        Existing RpcLine contract and future request proto location
        RPC contract inspected for Step 1
        Stdin RpcRequestLine contract
    - Path: ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/01-multi-turn-stdin-stdout-rpc-mode.md
      Note: Design produced in Step 1
ExternalSources: []
Summary: Diary for the multi-turn stdin/stdout RPC design ticket.
LastUpdated: 2026-05-21T17:30:00-04:00
WhatFor: Track design decisions, implementation guidance, validation, and future notes for stdin-driven multi-turn Pinocchio RPC.
WhenToUse: Read before implementing or reviewing PIN-20260521-RPC-STDIN-MULTITURN.
---



# Diary

## Goal

This diary captures the creation of the `PIN-20260521-RPC-STDIN-MULTITURN` ticket and the first design pass for multi-turn stdin/stdout RPC mode. It records what was inspected, why the design is scoped the way it is, and how future implementation should be reviewed.

## Step 1: Create the ticket and design the stdin RPC protocol

The goal of this step was to turn the existing future-work note for bidirectional RPC into a focused docmgr ticket with a detailed implementation guide. Current `--rpc` and `--output jsonl` modes are intentionally one-shot stdout streams. This ticket defines a separate explicit stdin-RPC mode so subprocess clients can keep one Pinocchio process alive and send multiple prompts to the same session.

The design keeps the same core architectural boundary used by the TUI work: `turns.Turn` is model context, while `sessionstream` is visible event and hydration state. The proposed RPC server should maintain a server-held final-turn accumulator per `session_id`, submit full input turns through `chatapp.PromptRequest.InitialTurn`, and update the accumulator from `PromptRequest.OnFinalTurn`.

### Prompt Context

**User prompt (verbatim):** "Create the ticket, and Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a dedicated docmgr ticket for multi-turn RPC support, write a detailed intern-ready design/implementation guide with system context, pseudocode, diagrams, API/file references, store it in the ticket, and upload the bundle to reMarkable.

**Inferred user intent:** The user wants RPC multi-turn work prepared as a durable, reviewable implementation package before coding begins, similar to the previous TUI persistence ticket workflow.

**Commit (code):** N/A — this step creates design documentation only.

### What I did

- Created ticket `PIN-20260521-RPC-STDIN-MULTITURN` titled `Add multi-turn stdin RPC mode`.
- Added design doc `design-doc/01-multi-turn-stdin-stdout-rpc-mode.md`.
- Wrote this implementation diary.
- Inspected current code and docs:
  - `proto/pinocchio/chatapp/rpc/v1/rpc.proto`
  - `pkg/chatapp/service.go`
  - `pkg/chatapp/runtime_inference.go`
  - `pkg/cmds/cmd.go`, especially `runRPCJSONL`
  - existing ticket docs for `PIN-20260520-CLI-RPC-JSONL`
- Designed a new explicit stdin mode instead of changing existing `--rpc` behavior.
- Recommended adding `RpcRequestLine` request messages while keeping stdout `RpcLine` as the response/event family.
- Recommended server-held in-memory `map[session_id]currentTurn` for the first implementation.

### Why

- Existing RPC JSONL is already useful as a one-shot stdout protocol and should remain stable.
- Multi-turn RPC needs request lines, request IDs, per-request done frames, and a long-lived process loop.
- Reusing `chatapp` and `sessionstream` avoids duplicate event mapping.
- Reusing final `turns.Turn` values avoids reconstructing inference context from UI state.

### What worked

- The current stdout `RpcLine` already has `session_id` and `request_id`, so it can represent request-scoped output for stdin RPC.
- `chatapp.PromptRequest` already has the required seams:
  - `InitialTurn` for caller-owned model context;
  - `OnFinalTurn` for updating the accumulator after success.
- The TUI backend is a proven local reference for final-turn accumulation.
- Existing debug/TUI/RPC fanout code gives a clear place to add request-id-aware writing.

### What didn't work

- No code implementation was attempted in this step.
- There is no dedicated stdin request proto yet; the design proposes adding it.
- Current `run.RunContext` has a writer but no reader, so stdin RPC implementation will likely need a `Reader io.Reader` field for tests and process wiring.

### What I learned

- Current `runRPCJSONL` submits exactly one `PromptRequest` with `InitialTurn` and returns the seed; multi-turn RPC needs a separate run mode rather than modifying that function in place.
- The existing `TurnStore` path inside `chatapp.Engine` is useful for web-chat-style persistence, but the first stdin RPC implementation can be simpler and process-local.
- Request ID propagation is the main adapter-level gap: stdout frames need to be correlated to active stdin requests.

### What was tricky to build

The tricky design point was separating three similar but distinct concepts: one-shot RPC, multi-turn stdin RPC, and durable resume. One-shot RPC should remain stable. Multi-turn stdin RPC needs a live process and an in-memory accumulator. Durable resume can be layered later with `TurnStore`, but should not block the protocol.

Another subtle point is request ID propagation. `RpcLine` already has a `request_id` field, but `sessionstream.UIFanout` is session-oriented. The implementation must add a request-aware writer/fanout wrapper or a request ID provider so streamed `uiEvent` frames can be grouped by client request.

### What warrants a second pair of eyes

- Whether the request proto should live in `rpc.proto` or a sibling file.
- Whether `--stdin-rpc` should imply `--rpc` or require it explicitly.
- Whether cancellation belongs in the first implementation or should be a follow-up.
- Whether final turns should be persisted to `--turns-db` in the first implementation, or kept strictly in memory.
- Whether request IDs should be mandatory or generated when omitted.

### What should be done in the future

- Implement Phase 1: protobuf request contract.
- Implement Phase 2: request-id-aware JSONL writer/fanout.
- Implement Phase 3: CLI flag and run mode.
- Implement Phase 4: stdin RPC server with submit/snapshot/shutdown.
- Add unit and subprocess tests for multi-turn, malformed input, session isolation, and shutdown.

### Code review instructions

- Start with the design doc:
  - `ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode/design-doc/01-multi-turn-stdin-stdout-rpc-mode.md`
- Then review current one-shot RPC:
  - `pkg/cmds/cmd.go`, `runRPCJSONL`
- Then review current protobuf and JSONL writer/fanout:
  - `proto/pinocchio/chatapp/rpc/v1/rpc.proto`
  - `pkg/chatapp/rpc/jsonl/writer.go`
  - `pkg/chatapp/rpc/jsonl/fanout.go`
- Then review final-turn semantics:
  - `pkg/chatapp/service.go`
  - `pkg/chatapp/runtime_inference.go`
  - `pkg/ui/chatapp_backend.go`

### Technical details

The intended first-pass protocol is:

```text
server stdout: RpcLine hello(capabilities include stdin-rpc,multi-turn)
client stdin:  RpcRequestLine submit(session_id, request_id, prompt)
server stdout: RpcLine uiEvent*(same session_id/request_id)
server stdout: RpcLine done(same session_id/request_id,status=ok)
client stdin:  RpcRequestLine submit(same session_id, new request_id, next prompt)
```

The intended context rule is:

```text
session currentTurn before request
  + new user prompt
  -> PromptRequest.InitialTurn
  -> inference
  -> PromptRequest.OnFinalTurn(finalTurn)
  -> session currentTurn = finalTurn.Clone()
```

The first implementation should preserve this invariant:

```text
final turns.Turn -> next model context
sessionstream events/snapshots -> stdout UI/RPC/debug state
```

## Step 2: Reconfirm and publish the intern-ready guide

This step responded to a follow-up request to create a ticket and upload an intern-ready multi-turn RPC design package. The ticket already existed from the earlier design pass, so I did not create a duplicate ticket. Instead, I verified the existing ticket, refreshed the guide with an explicit "How To Use This Guide As A New Intern" orientation section, and re-published the bundle to reMarkable.

The design remains intentionally scoped: it documents a future explicit stdin/stdout multi-turn RPC mode, not a change to the existing one-shot `--rpc` / `--output jsonl` behavior.

### Prompt Context

**User prompt (verbatim):** "Ok, let's creeate a new ticket for multi-turn RPC mode (if we haven't got one already).

Then Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Check whether the multi-turn RPC ticket exists; if so, reuse it. Ensure the guide is intern-ready, store it in the ticket, and upload the bundle to reMarkable.

**Inferred user intent:** The user wants a durable onboarding-quality design package for a future implementer and wants it available on reMarkable.

**Commit (code):** N/A — documentation refresh and upload only.

### What I did

- Verified existing ticket `PIN-20260521-RPC-STDIN-MULTITURN`.
- Verified the guide exists at `design-doc/01-multi-turn-stdin-stdout-rpc-mode.md`.
- Added a new "How To Use This Guide As A New Intern" section near the top of the guide.
- Updated this diary with the follow-up prompt context.
- Prepared the ticket bundle for reMarkable upload.

### Why

- Avoiding duplicate tickets keeps future implementation work in one canonical workspace.
- The existing guide already contains system background, protocol design, pseudocode, diagrams, API references, file references, risks, and implementation phases.
- The new orientation section makes it clearer how an intern should approach the guide.

### What worked

- `docmgr ticket list --ticket PIN-20260521-RPC-STDIN-MULTITURN` confirmed the ticket exists.
- The existing guide was already detailed and only needed a reader-orientation refresh.

### What didn't work

- N/A.

### What I learned

- This ticket had already been created and uploaded once, so the right action was to refresh and re-upload rather than create a duplicate.

### What was tricky to build

The main risk was duplicating documentation and splitting implementation context across two tickets. I kept the existing ticket as the source of truth.

### What warrants a second pair of eyes

- Whether the first implementation should include durable `TurnStore` persistence or stay process-local as the guide recommends.
- Whether `--stdin-rpc` is the final flag name or should be revised before implementation.

### What should be done in the future

- Implement Phase 1 of the guide: protobuf request contract for stdin request lines.

### Code review instructions

- Start with `design-doc/01-multi-turn-stdin-stdout-rpc-mode.md`.
- Validate that the guide still preserves the key invariant: final `turns.Turn` is model context; `sessionstream` is visible state.

### Technical details

Canonical ticket path:

```text
ttmp/2026/05/21/PIN-20260521-RPC-STDIN-MULTITURN--add-multi-turn-stdin-rpc-mode
```

## Step 3: Implement first-pass stdin multi-turn RPC

This step implemented the first process-local version of the multi-turn stdin/stdout RPC mode. The existing one-shot `--rpc` / `--output jsonl` behavior remains unchanged; the new long-lived mode is explicit via `--stdin-rpc`.

The implementation reads protobuf JSON `RpcRequestLine` values from stdin, stamps stdout `RpcLine` frames with the active request id, and keeps an in-memory `map[session_id]*turns.Turn` accumulator. Each successful submit stores the final Geppetto turn returned through `PromptRequest.OnFinalTurn`, and the next submit for the same session appends the new user prompt to that final turn.

### Prompt Context

**User prompt (verbatim):** "not yet. 

Lets add tasks to the ticket, and implemlent the feature, commit at appropriate intervals. Keep a diary"

**Assistant interpretation:** Add/adjust implementation tasks for the multi-turn RPC ticket, implement the stdin RPC feature, validate it, and keep the implementation diary current.

**Inferred user intent:** The user wants the designed multi-turn stdin/stdout RPC mode implemented now, while preserving ticket bookkeeping and reviewable commits.

**Commit (code):** d6f307a — "rpc: add stdin multiturn mode"

### What I did

- Extended `proto/pinocchio/chatapp/rpc/v1/rpc.proto` with `RpcRequestLine` and request messages:
  - `SubmitPromptRequest`
  - `CancelRequest`
  - `SnapshotRequest`
  - `ShutdownRequest`
- Ran `make proto-gen` to regenerate Go and TypeScript protobuf bindings.
- Added request-id stamping support to `pkg/chatapp/rpc/jsonl`:
  - `WithRequestID` helper;
  - `UIFanout.SetRequestID`;
  - request id propagation for UI, snapshot, hello, error, done, and backend frames.
- Added `--stdin-rpc` helper flag and `run.RunModeRPCStdin`.
- Added `Reader io.Reader` to `run.RunContext` for testable stdin injection.
- Implemented `PinocchioCommand.runStdinRPC`:
  - emits initial hello;
  - scans stdin JSONL;
  - handles submit/snapshot/cancel/shutdown;
  - maintains server-held final-turn accumulators by session id;
  - emits request-scoped done/error/snapshot frames.
- Added `pkg/cmds/cmd_rpc_stdin_test.go` for multi-turn accumulation and malformed JSON recovery.
- Updated user docs in `cmd/pinocchio/doc/general/06-rpc-jsonl-output.md`.
- Updated ticket tasks.

### Why

- Multi-turn subprocess clients need one process that can accept multiple prompt submissions.
- Existing stdout `RpcLine` already has `request_id`; the missing half was stdin request input and request-aware fanout stamping.
- Keeping state process-local avoids taking on durable session semantics in the first implementation.

### What worked

- Targeted Go tests passed:

```bash
go test ./pkg/cmds ./pkg/chatapp/rpc/jsonl ./pkg/chatapp -count=1
make schema-vet
go test ./pkg/cmds ./pkg/chatapp/rpc/jsonl ./pkg/chatapp -count=1
```

- Frontend typecheck passed after protobuf generation:

```bash
cd cmd/web-chat/web && npm run typecheck
```

- Biome import ordering needed a generated-file fix, then lint passed:

```bash
cd cmd/web-chat/web && npx --yes @biomejs/biome@2.3.8 check --write \
  src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts \
  src/chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb.ts
cd cmd/web-chat/web && npm run lint
```

### What didn't work

- Initial frontend lint failed after `make proto-gen` because generated TypeScript imports were not sorted:

```text
src/chatapp/pb/proto/pinocchio/chatapp/rpc/v1/rpc_pb.ts:5:1 FIXABLE
The imports and exports are not sorted.

src/chatapp/pb/proto/pinocchio/chatapp/v1/chat_pb.ts:5:1 FIXABLE
The imports and exports are not sorted.
```

I fixed it with Biome `check --write` on the generated protobuf TypeScript files.

### What I learned

- The existing one-shot RPC path and TUI persistence work provided almost all required seams: `PromptRequest.InitialTurn`, `PromptRequest.OnFinalTurn`, and `sessionstream.UIFanout` were sufficient for first-pass multi-turn behavior.
- Request id stamping is easiest as mutable state on the JSONL fanout because the first implementation processes one request at a time.

### What was tricky to build

The tricky part was keeping protocol concerns separate from model-context accumulation. `RpcRequestLine` carries client intent, `RpcLine` carries projected state, and only final `turns.Turn` values update model context. The implementation does not reconstruct context from snapshots.

Another subtle point is run status. A single runner/fanout is reused across requests, so `runStatusFanout` needed a `Reset` method to avoid leaking terminal status between submissions.

### What warrants a second pair of eyes

- `CancelRequest` currently submits `Service.Stop` and emits done, but there is not yet a concurrent test that cancels an in-flight provider call.
- The first implementation is sequential and process-local. It intentionally does not support overlapping submits in the same session.
- Request id stamping is fanout-local mutable state; this is correct for sequential handling but would need redesign for concurrent request processing.

### What should be done in the future

- Add session isolation tests with interleaved session ids.
- Add cancel-while-running tests with a blocking fake engine.
- Run a real subprocess smoke test with a cheap profile.
- Consider durable accumulator persistence only after process-local semantics are stable.

### Code review instructions

- Start with `proto/pinocchio/chatapp/rpc/v1/rpc.proto` to review the request contract.
- Then review `pkg/cmds/cmd.go`, especially `runStdinRPC` and `turnWithUserPrompt`.
- Review `pkg/chatapp/rpc/jsonl/fanout.go` for request-id stamping.
- Review `pkg/cmds/cmd_rpc_stdin_test.go` for expected multi-turn behavior.

### Technical details

Example stdin:

```jsonl
{"version":1,"sessionId":"s1","requestId":"r1","submit":{"prompt":"first"}}
{"version":1,"sessionId":"s1","requestId":"r2","submit":{"prompt":"second"}}
{"version":1,"sessionId":"s1","requestId":"r3","shutdown":{}}
```

Expected stdout properties:

- frames for request `r1` carry `requestId: r1`;
- frames for request `r2` carry `requestId: r2`;
- second submit sees first submit's final assistant turn as context;
- shutdown emits `done.status = "shutdown"` and exits.

## Step 4: Add session-isolation coverage and run a real stdin RPC smoke test

This step tightened validation after the first implementation commit. I added a regression test that alternates requests across two session ids and verifies that each session keeps its own final-turn accumulator.

I also ran the new mode as an actual subprocess using `PINOCCHIO_PROFILE=gpt-5-nano-low`. The smoke test sent two submit requests and one shutdown request through stdin, then verified that stdout contained request-scoped protobuf JSONL frames for both turns and a final shutdown done frame.

### Prompt Context

**User prompt (verbatim):** "go ahead"

**Assistant interpretation:** Continue with the remaining validation and follow-up tasks for the stdin RPC implementation.

**Inferred user intent:** The user wants the feature carried forward beyond the first implementation: stronger tests, real smoke validation, ticket updates, and commits.

**Commit (code):** da3b864 — "test: cover stdin RPC session isolation"

### What I did

- Added `TestRunWithOptionsStdinRPCIsolatesSessionAccumulators` in `pkg/cmds/cmd_rpc_stdin_test.go`.
- The test sends requests for `s1`, `s2`, then `s1` again, and asserts:
  - `s1-r1` completes for `s1`;
  - `s2-r1` completes for `s2`;
  - `s1-r2` completes for `s1`;
  - the final returned turn for `s1-r2` has `users=3`, proving `s2` did not contaminate `s1`'s accumulator.
- Ran focused stdin RPC tests:

```bash
go test ./pkg/cmds -run 'TestRunWithOptionsStdinRPC' -count=1
```

- Ran a real subprocess smoke test:

```bash
printf '%s\n' \
  '{"version":1,"sessionId":"smoke-stdin-rpc","requestId":"r1","submit":{"prompt":"Reply with exactly: one"}}' \
  '{"version":1,"sessionId":"smoke-stdin-rpc","requestId":"r2","submit":{"prompt":"Reply with exactly: two"}}' \
  '{"version":1,"sessionId":"smoke-stdin-rpc","requestId":"r3","shutdown":{}}' \
| PINOCCHIO_PROFILE=gpt-5-nano-low timeout 180 \
  go run ./cmd/pinocchio --log-level error generate-prompt \
    --goal 'stdin rpc smoke' --rpc --stdin-rpc --non-interactive
```

### Why

- Session isolation is a core correctness property for any long-lived subprocess server.
- The fake-engine tests validate accumulator mechanics quickly, but a real subprocess smoke catches CLI wiring, profile selection, stdout cleanliness, and actual protobuf JSONL behavior.

### What worked

- Focused stdin RPC tests passed:

```text
ok  github.com/go-go-golems/pinocchio/pkg/cmds  0.039s
```

- The real subprocess smoke exited `0`.
- It produced 26 stdout JSONL frames.
- The tail included:
  - request `r2` `ChatRunFinished`;
  - request `r2` snapshot containing assistant text `two`;
  - request `r2` `done.status = "ok"`;
  - request `r3` `done.status = "shutdown"`.
- stderr was empty with `--log-level error`.

### What didn't work

- N/A for this step.

### What I learned

- The actual profile-backed subprocess path preserves stdout cleanliness for JSONL when logging is kept off stdout.
- The process-local accumulator was visible in the second real request snapshot: both turns remained in the sessionstream snapshot for the same session.

### What was tricky to build

The main tricky point was testing isolation through the same public command path rather than reaching into internals. The fake engine reports the number of user blocks in the final turn, so a session contamination bug would be visible as an unexpected count in the final assistant text.

The real smoke also needs `--non-interactive` and `--log-level error` so the JSONL protocol is not mixed with continuation prompts or verbose logs.

### What warrants a second pair of eyes

- Cancel semantics are still only lightly covered. The current implementation accepts cancel requests, but a dedicated cancel-while-running test is still warranted.
- The smoke test used sequential submit requests. It did not test overlapping submit rejection or in-flight cancellation.

### What should be done in the future

- Add a blocking fake engine and a cancel-while-running test.
- Decide whether stdin RPC should process requests strictly sequentially or eventually support concurrent sessions with per-session request-id stamping.

### Code review instructions

- Review `pkg/cmds/cmd_rpc_stdin_test.go`, especially `TestRunWithOptionsStdinRPCIsolatesSessionAccumulators`.
- Re-run focused tests with:

```bash
go test ./pkg/cmds -run 'TestRunWithOptionsStdinRPC' -count=1
```

- Re-run the subprocess smoke with `PINOCCHIO_PROFILE=gpt-5-nano-low` if credentials and profile configuration are available.

### Technical details

Smoke output proof points:

```text
exit=0
26 stdout JSONL lines
requestId="r2" done.status="ok"
requestId="r3" done.status="shutdown"
stderr empty
```
