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
    - Path: pkg/chatapp/runtime_inference.go
      Note: InitialTurn and OnFinalTurn semantics that shape the design
    - Path: pkg/chatapp/service.go
      Note: PromptRequest API used by proposed multi-turn adapter
    - Path: pkg/cmds/cmd.go
      Note: |-
        Current one-shot RPC implementation and future stdin RPC entry point
        Current RPC implementation inspected for Step 1
    - Path: proto/pinocchio/chatapp/rpc/v1/rpc.proto
      Note: |-
        Existing RpcLine contract and future request proto location
        RPC contract inspected for Step 1
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
