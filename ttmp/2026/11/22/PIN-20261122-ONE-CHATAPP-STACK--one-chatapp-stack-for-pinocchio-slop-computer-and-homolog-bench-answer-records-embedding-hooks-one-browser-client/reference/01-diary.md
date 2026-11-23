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

## Step 2: Answers as records, and the embedding hooks (phases 1 and 2)

Phase 1 made every fact a page shows about an answer part of the timeline: the run entity now holds model, start,
finish, usage and the number of model calls, and every part entity has start and end times. Phase 2 moved what each
embedding program re-implemented into pinocchio, and added the three hooks homolog-bench needs (server-side
instructions, server-side context per message, an in-process tool that waits for a person).

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phases 1 and 2 of the guide, with tests and commits.
**Commit (code):** db9cc10a — "chatapp: an answer is a record (run model, times, usage; part times)"; a42c4a26 —
"chatapp: embedding hooks (state dir, context field, server instructions and context, person waits)"

### What I did
- Protos (`chat.proto`, `frontend_tool.proto`): times on lifecycle events (`at_ms`, `started_at_ms`,
  `finished_at_ms`), `model` on `ChatRunStarted`, `message_id` on `ChatProviderCallFinished`, record fields on
  `ChatRunEntity`, times on `ChatMessageEntity`/`ToolCallEntity`/`FrontendToolCallEntity`, `state` on
  `ToolCallEntity`, new `ChatToolCallProgress`. Regenerated with the earlier ticket's
  `scripts/01-proto-gen-local.sh` (protoc-gen-go v1.36.12, protoc-gen-es v2.14.0) and buf v1.50.0 installed into the
  scratchpad; generator headers unchanged.
- `pkg/chatapp/stamp.go`: `Stamped(pub)` and `StampTimes(msg, now)` set unset time fields through protoreflect.
  `handleStartInference` stamps the run's publisher (tools receive the same publisher), frontendtools'
  `HandleManifest`, `HandleResult` and `Request` stamp theirs.
- Projections: `ChatRun` gets model/times/usage (`AddUsage`, `providerCalls++` per provider call of the run); text
  (`projections.go`), thinking (`plugins/reasoning.go`), tool calls (`plugins/toolcall.go`, plus
  `ChatToolCallProgress` → `state`) and browser tools (`frontendtools/plugin.go`) copy times; `StartAt` sets a start
  once.
- `ComposedRuntime.Model`; agentserver sets the resolved model reference.
- scripted engine: `/think THOUGHT -- REST`.
- httpapi: `WithStateDir`, `ContextField`, `Coded`; agentserver: `WithSessionInstructions`, `WithMessageContext`,
  `Session(ctx)` (the run kept through `chatapp.WithRunTools(keepRun)`), person limit and `OnState` →
  `ChatToolCallProgress`; its `openStores` and `codedError` removed.
- toolexec: the deadline uses a resettable timer (`time.AfterFunc` + `context.WithCancelCause`) and puts a
  `callControl` into the call's context; `AwaitPerson` resets it to the person limit, its release back to the full
  ordinary limit (agentd's `SetCallState` semantics).

### Why
- Times are stamped where events are published, not in projections, so replaying the event log gives the same
  entities (guide D4). Stamping in one publisher wrapper instead of at each of ~15 publish sites means a new event
  type with an `at_ms` field cannot be forgotten.

### What worked
- `TestAnAnswerIsARecordInTheSnapshot` (`/think plan the call -- /tool noop {}`): the snapshot alone has model
  `fake:scripted`, start ≤ finish within the test window, `providerCalls` 2, usage, and times on the user message,
  thinking, text and tool call.
- `TestAToolThatWaitsForAPersonOutlastsTheToolLimitAndSaysSo`: a tool waits 150 ms (limit 50 ms) inside
  `AwaitPerson` and succeeds; the stream carries `awaiting-person`, then `running`.
- `go test -race ./pkg/chatapp/...` and `make lint` (0 issues) pass.

### What didn't work
- A Python edit script failed before writing anything: `SyntaxError: invalid syntax. Perhaps you forgot a comma?`
  (a `\'` inside a single-quoted literal). Rewrote it with double quotes.
- `which buf protoc-gen-go protoc-gen-es` found nothing; installed buf v1.50.0 with `GOBIN=<scratch>/bin go install`.
- A text segment's entity is created by its first `ChatTextPatch`, not by `ChatTextSegmentStarted` (the projection
  returns nil for an empty segment), so start times were missing until patches got `at_ms` too.
- `TestOnlySessionsReachesTheStoreUnchecked` failed: `state.go:35:5: openStateDir uses .storeOpt; handlers reach
  sessions through s.guard.For(p)`. httpapi has an AST test that allows the raw store only in `New` and
  `WithSessionStore`. Instead of widening it, `openStateDir` returns the session store and `New` assigns it.
- Two assertion mistakes in my tests: geppetto JSON-encodes a string tool result (`ask returned: "approved in
  lab"`), and protojson omits an empty `state` (nil, not `""`).
- `make lint`: `gofmt` on agentserver.go and `nonamedreturns` on `AwaitPerson(ctx) (release func())`; fixed.
- A negative check (stash projections.go, expect the test to fail) broke the build instead, since the plugins use
  helpers from that file; the test's assertions (`providerCalls` 2, `model`) cannot pass without the change anyway.

### What I learned
- `ChatProviderCallFinished` reached clients with `"correlation":{}`; the sink knew the run all along.
- httpapi protects its session store with a structural test; respect such tests rather than editing their allowlist.

### What was tricky to build
- The person-wait timer. `context.WithTimeoutCause` cannot be extended, so the middleware now owns a `time.Timer`
  that cancels the call's context with `errDeadline`; a deadline is recognised by `context.Cause`, a run stop by the
  parent context's error (unchanged behaviour, `TestDeadlineKeepsTheResultWhenTheRunIsStopped`). Nested waits count;
  only the outermost resets the timer.
- Server instructions must not be page-settable. The create hook always rewrites `serverInstructions` in the
  session's data, including deleting a page-sent value when the program sets none.

### What warrants a second pair of eyes
- `StampTimes` relies on field names (`at_ms`, `started_at_ms`, `finished_at_ms`). Entities have fields with the same
  names; entities are never published, but a future event that embeds an entity would be stamped at the top level
  only.
- `ChatToolCallProgress` is published from the tool's goroutine through the run's publisher, concurrently with the
  run sink's events; sessionstream orders them by ordinal, and `-race` is clean.

### What should be done in the future
- N/A for these phases.

### Code review instructions
- `git show db9cc10a a42c4a26`; start with `pkg/chatapp/stamp.go`, `projections.go`, `agentserver/run.go`,
  `toolkit/toolexec/toolexec.go` (`DeadlineBy`, `AwaitPerson`).
- `go test -race ./pkg/chatapp/...`; `make lint`.

## Step 3: The shared browser client (phase 3)

`pkg/chatapp/webclient` is the one browser client: a pure `Timeline` that mirrors the server's projections and
derives `answers()`, and a `Session` that does the I/O. Its built file is embedded in the Go module and served at
`{prefix}/client.js`. The test that matters is the agreement test: real conversations through agentserver, captured,
and replayed into the client, which must reproduce the server's snapshot exactly.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 3 of the guide.
**Commit (code):** 75780996 — "chatapp: one browser client, served by the server it talks to (pkg/chatapp/webclient)"

### What I did
- `src/timeline.ts`: reducer per event name, written against `projections.go`, `plugins/reasoning.go`
  (`reasoningEntityFromFields`), `plugins/toolcall.go` (`toolCallFieldsFromCanonical`, `mergeToolCallFields`),
  `frontendtools/plugin.go`; `ApplyStreamPatch` semantics (append for APPEND/UNSPECIFIED, replace otherwise).
  `answers()` groups by id (`<run>-user`, `parentMessageId`, `messageId`), orders parts by creation ordinal, merges a
  call's `ChatToolCall`/`ChatToolResult`/`ChatFrontendToolCall` into one part.
- `src/session.ts`: from slop's `agent.ts` (single path, outbox, executor assignment), generalized: `create/open/list`,
  `on`, `send` → message id, `wait(id)`, `stop`, `delete`, `close`, `tools(list)`; SSE parsed from `fetch` bodies.
- `webclient.go` (`//go:embed chatapp-client.js`, `Handler` with ETag); httpapi route `GET {prefix}/client.js`.
- `capture_test.go` (package `webclient_test`): four scenarios through agentserver + scripted engine (`Delay` 15 ms so
  a second stream can attach mid-answer); `node --test test/` with `CHATAPP_CAPTURES`; skipped without node.
- `package.json` (esbuild 0.25.12, typescript 5.9.3, the versions slop pins), `tsconfig.json`, `make webclient`,
  `make webclient-check`, `.github/workflows/webclient-check.yml` (Node 22).

### Why
- Testing the client against captures from the current server, on every `go test`, turns "the client mirrors the
  projections" from a promise into a checked property (guide D3).
- Tests import the built bundle, not the TypeScript, so they run on any Node with `node:test` (no type stripping), and
  they check the exact file that is served.

### What worked
- First run against the server: 17/17 Node checks, including the stream that attached mid-answer (its snapshot is
  partial; replaying the rest gives the final snapshot).
- Mutation check: changing `providerCalls + 1` to `+ 2` in the bundle fails 9 of 17 checks; a rebuild reproduces the
  committed file byte for byte (`cmp`).
- `go test ./...`, `make lint` (0 issues), `make logcopter-check` pass.

### What didn't work
- `tsc`: `Property 'messageId' does not exist on type '{ toolCallId: string; }'` — a spread object needed the
  `Payload` type.
- `TestTheServerServesTheEmbeddedClient` asserted `class Session`; esbuild emits `var Session = class _Session`.
  Now asserts an ES module (`export {`).
- `make logcopter-check`: `generated file is not current: …/pkg/chatapp/webclient/logcopter.go` — every package
  carries a generated logger; generated it with `go tool logcopter-gen … ./pkg/chatapp/webclient`.
- `gofmt` on `httpapi/server.go` after adding the import.

### What I learned
- Go's `./...` does not skip `node_modules`; esbuild's npm package contains no `.go` files, checked with `find`.
- `.gitignore` has `dist/`, so the bundle lives at the package root as `chatapp-client.js`.

### What was tricky to build
- Agreement under protojson: the server omits empty values, the client keeps some explicitly (`streaming: false`,
  `state: ""`). The comparison normalizes both sides the same way (drop `""`, `false`, `0`, `"0"`, `null`, empty
  arrays and objects); int64 values stay strings on both sides because the client copies them from events.
- Creation order: the client uses the event's `ordinal` for a new entity, the server's snapshot `createdOrdinal`; both
  are the ordinal of the creating event, which the test compares.

### What warrants a second pair of eyes
- Compact UI transformers (`chatapp.WithUIEventTransformers`) are opt-in and httpapi does not use them; a server that
  enables them would send events this client does not reduce.
- `Session.#derive` aborts a running browser call when its entity leaves `requested` (the server gave up or got a
  result from elsewhere).

### What should be done in the future
- react-chat's chat-provider could feed its Redux store from `Timeline` (phase 6, own ticket).

### Code review instructions
- `pkg/chatapp/webclient/src/timeline.ts` next to `pkg/chatapp/projections.go`; `test/agree.test.mjs`;
  `capture_test.go`.
- `make webclient-check`; `go test ./pkg/chatapp/webclient -keep-captures <dir>` to look at the captures.

## Step 4: slop-computer on the shared client and the server helpers (phase 4)

slop's agent capability now runs on what pinocchio provides. On the server, `httpapi.WithStateDir`, `ContextField`
and `Coded` replace slop's copies, and `ComposedRuntime.Model` names the model. In the page, the stream, the
timeline, browser tools and the outbox are the shared `Session`; `agent.ts` keeps slop's consent, open-by-key and a
send's lifecycle. All 11 of slop's agent tests pass unchanged except for one fixture fix.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 4 of the guide.
**Commits:** pinocchio `ea7ebbe8` ("webclient: Session.attach(base, info) …", branch `task/webclient-attach`,
PR #222); slop-computer `261f9835` (branch `task/one-chatapp-client`, PR go-go-golems/slop-computer#57).

### What I did
- Pushing went wrong first (see below); then a worktree from `origin/main` (`../slop-computer-one-chatapp`), so the
  diverged local `main` stays untouched.
- pinocchio: `Session.attach(base, info)`. slop lists or creates its session itself (open by key), so it needs a
  session object without another GET.
- slop `go get github.com/go-go-golems/pinocchio@ea7ebbe8162d`; `agent.go`/`meter.go` onto `httpapi.WithStateDir`,
  `httpapi.ContextField(MaxContextBytes)`, `httpapi.Coded`; `Model: model.Model`.
- `web/runtime/sync-chatapp.mjs` copies `pkg/chatapp/webclient/src/*.ts` from `go list -m -f {{.Dir}}` into
  `src/chatapp/`; `test/chatapp-copy.test.ts` fails while the copy differs from the module's source.
- `agent.ts`: its `Session` wraps `ChatSession.attach(BASE, info, { fetch: fetchVia })` (slop's authenticated
  `request` has fetch's shape); a send settles from `timeline.answer(runId)`; errors map from `ChatError` to slop's
  `fail(code, message)`.

### Why
- Copying the TypeScript (not the built bundle) lets slop's esbuild bundle it into its IIFE runtime with its
  transport, and the copy test keeps it the same as the Go module slop runs.

### What worked
- slop `make test` (Go): pass. Runtime tests: 32/32, including all 11 agent tests.

### What didn't work
- **The push.** `git push -q -u origin task/one-chatapp-stack` (phases 1–3) updated pinocchio **main**: the branch
  was created from `origin/main`, so it tracked it, and the global `push.default=tracking` pushes to the upstream.
  `gh pr create` then said `No commits between main and task/one-chatapp-stack`. The six commits
  (`94c6f440..78a009e4`) are on main without review; PR #222 says so and asks for their review. Since then:
  `--no-track` branches, `git push origin HEAD:refs/heads/<branch>`, and `git ls-remote` before a PR.
- `pnpm test` hung: a failing test without a per-test timeout. With `--test-timeout=8000`, two tests failed:
  `a run whose events all arrive before its 202 settles when the 202 names it` (8 s timeout). The scripted server's
  events after a snapshot reused ordinals below the snapshot's (`snapshotOrdinal: "30"`, then `ordinal` 6, 7, …),
  and the shared client ignores an event whose ordinal is not above the snapshot's, as it must. Fix in the fixture:
  `ordinal = 30` (and `41`) after those snapshots.
- `tsc` reports `Cannot find module 'node:test'` for every test file; the same 19 errors on untouched
  `origin/main` (checked with `git stash -u`), so not from this change; `src/` has no errors.

### What I learned
- slop's old agent.ts accepted out-of-order ordinals because it never compared them; the shared client's rule
  ("an event the copy already reflects is ignored") made the fixture's mistake visible.

### What warrants a second pair of eyes
- `sync-chatapp.mjs` needs `go` at test time; without it the copy test skips.

## Step 5: homolog-bench on agentserver and the shared client (phase 5)

hb-server's agent is now `agentserver` with the room's eval, instructions and context; the page renders
`timeline.answers()` from the shared client served by the agent service; hb-eval reads replies from the answer's
record. The agent session contract (homolog-bench HB-006 design-doc/02, R1–R6) is checked against the server.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 5 of the guide.
**Commits (homolog-bench, branch `task/hb-007-chatapp`, PR wesen/homolog-bench#5):** `516d621` (server, eval,
hb-eval), `6a6b3c7` (page, UI tests), `87d32ae` (R5 check), `211366e` (HB-007 ticket).

### What I did
- `server.Agent(mgr, presets, opts...)`: `agentserver.New` with `WithPrefix("/api/v1/agent")`,
  `WithToolRegistry(eval)`, `WithSessionInstructions(room API reference)`, `WithMessageContext(room context)`.
  The tool needs the server (`srv.Session`) and the server needs the tool: a closure over the variable.
- eval: `geptools.NewToolFromFunc` with ctx; `CurrentToolCallFromContext` for the call id the room logs; a
  `personWait` turns the room's `OnAwaiting("awaiting-person" | "running")` into `toolexec.AwaitPerson` and its
  release (mutex: the room calls it from its goroutine).
- `GET /api/v1/rooms/{id}/agent-context`: the judge (hb-eval) was shown the session's first context message;
  chatapp keeps context blocks in the turn, not in the timeline, so the room renders it on request.
- evalrun client: `NewSession(preset, model, labels)`, `Turn` (stream opened before the POST; ends on the run's
  `ChatRun*` event, or a 2 s poll when the run ended before its id was known; reply from the snapshot), `Model`,
  `SessionUsage`, cost from `GET /models` prices.
- agent.js: `import(BASE + '/client.js')`; `displayOf(timeline, cache)` maps answers to the renderer's messages,
  keeping display objects per entity id (`kept`) and bumping a part's version only when a field changed (`upd`);
  a thinking part ends when its segment finished or another part follows it.
- Tests: Go (recording engine shows the model's turn; person wait; hb-eval against hb-server); UI (R1, R2, R4 on a
  real server with the scripted engine; R5 and a reload in the scripted render test).

### Why
- Mapping answers to the existing display objects kept the renderer (incremental, per-part versions, open state)
  unchanged; only its data source changed.
- The render test makes snapshots with the page's own `Timeline` (client.js imported into Node as a data URL), so
  its fixtures cannot drift from what the reducer accepts.

### What worked
- `go test ./...`; UI: agent (22 checks), agent-render (33), dock, replica, jobs, restart.
- R1 on a real server: a second page opened while eval waits 2.5 s shows `reasoning,tool:running`, the same as
  the page that watched, and the same finished answer and footer.

### What didn't work
- The first person-wait test called `host.ask` from agent code:
  `ReferenceError: host is internal to the room; use the workbench API (bench.help())`. Only real-engine tool work
  asks a person, so the end-to-end test was replaced by one that drives `personWait` under `toolexec.DeadlineBy`
  (20 ms limit, 100 ms wait, states `awaiting-person`, `running`).
- `TestTheModelGetsTheRoomsInstructionsAndContext` asserted `bench.help()` in the system prompt; the reference
  starts with "API reference".
- The R5 render check read the footers before the next animation frame (`["190 ms · 1.1k in / 20 out · 1 model
  call"]`, one footer); a `frame()` before reading.
- My first cut of `internal/evalrun/client.go` also removed the hbd job helpers between the agent functions
  (`undefined: Job`); restored from `git show HEAD`.

### What I learned
- The contract's R3 (ids) and R5 (gaps) need no code in the page: entity ids carry the pairing, and a stream with no
  resume cursor cannot skip events.

### What was tricky to build
- `Turn` must not miss a person wait: the stream is opened (and its first snapshot received) before the message is
  posted; the run's id is known only from the 202, so run-end events before it are kept and a poll covers them.

### What warrants a second pair of eyes
- Conversations in `data/hb-server/agentd` are not migrated (state moves to `data/hb-server/agent`).
- Cost is estimated on the client from `/models` prices; the record has tokens, not cost.

### Code review instructions
- homolog-bench: `internal/server/server.go` (`Agent`, `EvalTool`, `personWait`), `web/js/agent.js`
  (`displayOf`, `answerOf`, `partOf`, `derive`, `send`), `internal/evalrun/client.go` (`Turn`, `reply`).
- Run: `go test ./...`; `hb-server serve --address 127.0.0.1:8806 --engine fake --state-dir <tmp> --web-dir web`,
  then `cd tests/ui && node agent.test.mjs http://127.0.0.1:8806/` and `node agent-render.test.mjs …`.
