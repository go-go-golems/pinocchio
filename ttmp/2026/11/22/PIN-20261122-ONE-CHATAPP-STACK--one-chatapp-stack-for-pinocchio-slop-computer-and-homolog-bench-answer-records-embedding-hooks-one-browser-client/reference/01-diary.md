---
Title: Diary
Ticket: PIN-20261122-ONE-CHATAPP-STACK
Status: active
Topics:
    - chatapp
    - chat
    - backend
    - frontend
    - refactor
    - design
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles: []
ExternalSources: []
Summary: "Chronological record of the unification: evidence, decisions, failures, commits."
LastUpdated: 2026-11-22T20:45:00-05:00
WhatFor: "Following how the one-chatapp-stack work was done and why."
WhenToUse: "When resuming or reviewing this ticket."
---

# Diary

## Step 1: Evidence, ticket and the intern guide

The question that started this ticket came from homolog-bench. Its agent panel had just had two Codex findings fixed
as point fixes, and pinocchio had meanwhile merged agentd into chatapp (#221, v0.11.19), with slop-computer already on
it. The goal is one server stack and one browser client for all three applications. This step gathered the evidence,
created the ticket, and wrote the intern guide (design-doc/01) with a first draft and a second pass.

### Prompt Context

**User prompts (verbatim), in order:**
- "so I think go-go-golems/pinocchio and go-go-golems/slop-computer have gotten a new chatapp from pinocchio as backbone for the LLM functionality, can you check?"
- "how much can you unify your approach with pinocchio/slop-computrs? ideally it would be identical I think?"
- "so how can we unify things? ideally i guess we can have everything in pinocchio? or should we import slop into homolog-workbench?"
- "ok, create a new docmgr ticket, and Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."
- "Then implement, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"
- "And when done,  write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill). Commit and push the bsidian vault when done (go-go-parc vault)."

**Assistant interpretation:** Write the guide, upload it, then implement phases 1–5 across pinocchio, slop-computer and
homolog-bench with commits and this diary, then write an Obsidian deep-dive post and push the vault.

**Inferred user intent:** One chatapp-based agent stack shared by the three applications, so that the bug classes
seen in homolog-bench and slop cannot recur, documented well enough for a newcomer.

### What I did
- Read pinocchio at `b271952f`: `pkg/chatapp/projections.go`, `runtime_sink.go`, `httpapi/server.go` (route table at
  :322-334), `httpapi/sse.go`, `agentserver/agentserver.go`, `toolkit/toolexec/toolexec.go`, `plugins/reasoning.go`,
  `plugins/toolcall.go`, `proto/pinocchio/chatapp/v1/chat.proto`, and the earlier ticket's intern guide and homolog
  migration playbook (`PIN-20261121-AGENTD-INTO-CHATAPP`).
- Read slop-computer at origin/main `707cf6f`: `pkg/doors/pages/agent/agent.go`, `web/runtime/src/agent.ts`.
- Read react-chat `09597c5`: `packages/chat-provider` layout and imports (`ws/`, `store/`).
- Read homolog-bench `5dcc441`: `cmd/hb-server/main.go:95-160`, `internal/server/server.go:60-147`,
  `internal/evalrun/client.go`, `configs/agent/*.yaml`; and pinocchio-agentd `Session.SetCallState`
  (`pkg/agentd/session.go:388`).
- Captured a real SSE stream with the earlier ticket's smoke script
  (`PIN-20261121-…/scripts/02-agent-serve-smoke.sh`, scripted engine, port 8799); stored as
  `sources/01-agent-serve-sse-capture-b271952f.txt`.
- Created the ticket, tasks, the guide and this diary; branched pinocchio `task/one-chatapp-stack` from `origin/main`.

### What worked
- The capture showed exactly the gaps the design needs to close: `ChatProviderCallFinished` arrives with
  `"correlation":{}` and no `messageId`; `ChatRunStarted`/`ChatRunFinished` carry no time or model.
- The smoke script ran unchanged against the release (`{"ok":true,"presets":1}` … `[('user', …), ('assistant', 'eval
  returned: 42')]`).

### What didn't work
- My first picture of homolog-bench was wrong: I described `eval` as a browser tool. `configs/agent/homolog-bench.yaml`
  says `hostTools: [eval]` and `internal/server/server.go:65` runs it in the room on the server. Corrected before the
  guide was written; it is the reason for the three server-side hooks of guide 5.4.
- My first draft described the person wait as pausing the ordinary timer and resuming it with the remaining time.
  agentd resets the timer instead (`d.timer.Reset(limit)` for both states, `session.go:405-406`) and publishes
  `tool.progress`. The guide now specifies agentd's behavior and a `ChatToolCallProgress` event.
- The local slop-computer `main` has diverged from origin (`ad2d739`, three local SLOP3-024 commits, without #56);
  all slop reading was done on `origin/main`. Not touched.

### What I learned
- slop's `agent.ts` already has the single-path design (snapshot replaces, events reduce, derive) that homolog-bench's
  HB-006 store introduced for the room; the shared client should start from it.
- homolog-bench has a fourth client: Go, `internal/evalrun/client.go` (persona runs).
- Entity ids already encode the run (`<run>-user`, `<run>:text:<seg>`, `<run>:thinking:<seg>`, `messageId` on tool
  calls), so grouping by id needs no protocol change.

### What was tricky to build
- Deciding where the times live. Projections must be deterministic over replayed events, so the clock must be read
  where events are published (the run sink), not in the projection. Recorded as D4 in the guide.

### What warrants a second pair of eyes
- D3 (keep a client reducer that mirrors projections, with capture tests) versus streaming entities from the server.
  The guide defers the second; a reviewer may prefer it now.

### What should be done in the future
- Phase 6 (react-chat on the shared Timeline) is a separate ticket.

### Code review instructions
- Read `design-doc/01-intern-guide-one-chatapp-stack-for-three-applications.md` top to bottom; check sections 3.3/3.4
  against `sources/01-agent-serve-sse-capture-b271952f.txt`.

### Technical details
- Second-pass review notes (applied): thinking entity id was unexplained; the grouping rule named `messageId` for
  browser tool calls, which use `parentMessageId`; "four times" needed "one definition, three copies"; who stamps times
  and how a tool's waiting state reaches the stream were missing; the built-file check was vague (now a `make` target
  and CI diff); the plan lacked a first command.
