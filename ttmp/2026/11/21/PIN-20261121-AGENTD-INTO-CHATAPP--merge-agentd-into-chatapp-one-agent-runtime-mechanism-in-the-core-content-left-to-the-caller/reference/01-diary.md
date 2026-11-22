---
Title: Diary
Ticket: PIN-20261121-AGENTD-INTO-CHATAPP
Status: active
Topics:
    - chatapp
    - refactor
    - design
    - migration
    - sessionstream
    - backend
DocType: reference
Intent: long-term
Owners:
    - manuel
RelatedFiles: []
ExternalSources: []
Summary: "Chronological implementation diary for merging pinocchio's agentd into chatapp."
LastUpdated: 2026-11-21T23:25:22.922662164-05:00
WhatFor: "Record what was done, why, what failed, and how to review it."
WhenToUse: "Read the latest step to resume; read earlier steps for the reasoning behind a decision."
---

# Diary

## Goal

Merge pinocchio's `pkg/agentd` (the agent service homolog-bench uses) into `pkg/chatapp` (pinocchio's main chat
engine), so pinocchio has one agent runtime. The core keeps mechanism only (sessions, runs, the tool loop, tools,
events, transport); everything that goes into the model's context window is left to the caller, with optional helpers.
No backwards compatibility: homolog-bench adapts, guided by the migration playbook in this ticket.

## Context

- Workspace: `/home/manuel/workspaces/2026-10-28/slop-computer` (WSM workspace `slop-computer`).
- pinocchio worktree: `pinocchio/`, branch `task/merge-agentd-into-chatapp`, started from `origin/main` `22a0e56`
  (v0.11.18, "Merge pull request #220 from go-go-golems/task/agentd-hb003").
- geppetto worktree: `geppetto/` at `b35d0275` (v0.14.3).
- sessionstream: module `github.com/go-go-golems/sessionstream v0.1.2` (read from the Go module cache).
- Design guide: `design-doc/01-intern-guide-merging-agentd-into-chatapp-one-agent-runtime-mechanism-in-the-core.md`.
- Migration playbook: `playbook/01-migration-playbook-homolog-bench-from-agentd-to-chatapp.md`.

## Step 1: Ticket, workspace and the research behind the design

This step set up the work and gathered the evidence the design rests on. The conversation before the ticket had
already established the direction (chatapp is the main engine; agentd merges into it; the core holds mechanism and the
caller holds content). This step turned that direction into a code map: what chatapp, sessionstream, frontendtools,
web-chat and agentd each do today, and where their responsibilities overlap.

The main outcome is a list of facts the guide depends on. chatapp runs on a sessionstream hub whose commands are
dispatched synchronously, so a run that refuses to start can return an error to the caller. chatapp does not pass loop
limits to geppetto, cancels a running turn when a new prompt arrives, and loses a stopped turn from the model's
memory. Nothing in geppetto closes a tool call that never got a result, which matters once stopped turns are kept.

### Prompt Context

**User prompt (verbatim):** "ok, let's go. Create a new docmgr ticket and Create  a detailed analysis / design /
implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is,
with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should
be very clear and technical. Store in the ticket and the nupload to remarkable.

merge agentd into chatapp. No backwards compatibility, we'll have homolog-bench adapt properly.

Then, implement, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from
the skill)"

**Follow-up prompt (verbatim, sent while this step ran):** "keepa detailed diary and also write a migration playbook
so that we can hand it off to our homolog-bench colleague"

**Assistant interpretation:** Create a pinocchio docmgr ticket; write an intern-level design and implementation guide
for merging agentd into chatapp; upload it to reMarkable; implement the merge in phases with commits; keep an
investigation-format diary; write a migration playbook for the homolog-bench developer instead of porting
homolog-bench in this work.

**Inferred user intent:** One agent runtime in pinocchio that other projects (homolog-bench now, slop-computer later)
can build on, with a hand-off document so the homolog-bench side can move independently.

### What I did

- Added pinocchio to the workspace: `wsm add slop-computer pinocchio`. It created the worktree on branch
  `task/slop-computer`; I fast-forwarded it to `origin/main` (`22a0e56`).
- Created branch `task/merge-agentd-into-chatapp` and the ticket:
  `docmgr ticket create-ticket --ticket PIN-20261121-AGENTD-INTO-CHATAPP --title "Merge agentd into chatapp: …" --topics chatapp,refactor,design,migration,sessionstream,backend`.
- Added three documents: the design guide (`design-doc`), this diary (`reference`) and the playbook (`playbook`).
- Read, in this order:
  - `pkg/chatapp/chat.go`, `service.go`, `runner.go`, `runtime_inference.go`, `features.go`, the event names in
    `chat.go:18-41`, and the plugin event mapping in `runtime_sink.go` and `plugins/*.go`.
  - sessionstream `hub.go` (`Submit` → `dispatch` → handler, `localEventPublisher.Publish` → `projectAndApply`),
    `hydration.go` (`HydrationStore`, `EventStore`, `Snapshot`), `projection.go` (`UIEvent`, `TimelineEntity`) and
    the websocket server's hydration buffer (`transport/ws/server.go:664-780`).
  - `pkg/chatapp/frontendtools/manager.go` (`Request`, error codes, `ManagerConfig`), `bridge.go`, `plugin.go`.
  - `cmd/web-chat/internal/appserver/*` (routes, `RuntimeResolver`) and `cmd/web-chat/internal/runtime/composer.go`.
  - `pkg/chatapp/serverkit/contracts.go`, `stores.go`, `http.go`.
  - `proto/pinocchio/chatapp/v1/chat.proto` (`ChatRun*`, `ChatProviderCallFinished`, `UsageInfo`,
    `StartInferenceCommand`).
  - `pkg/agentd/*` (all files) and `cmd/pinocchio/cmds/agent.go`; the agentd test list.
  - geppetto `pkg/inference/toolloop/config.go` and `limits.go`; searched geppetto for any handling of tool calls
    without results.
- Delegated two read-only surveys (geppetto; pinocchio + sessionstream) and spot-checked their key claims against the
  code before using them.

### Why

- The guide has to describe the current system accurately before it can propose changes, and an intern has to be
  able to follow one prompt from HTTP request to model call to UI event.
- The surveys were delegated because they spanned two large repositories; their claims were verified where the
  design depends on them (chatapp's run replacement, `EventStore`, `TurnStore`, the websocket's non-replay, geppetto's
  per-call `runner` session, the unused `run-*` events, the persister ignoring errors).

### What worked

- `wsm add slop-computer pinocchio` created the worktree; `git merge --ff-only origin/main` brought it to `22a0e56`.
- The ticket and three documents were created without errors.

### What didn't work

- `wsm add` rewrote the workspace's `go.work`: it dropped `./geppetto` (added by hand earlier, not in `wsm.json`) and
  changed `go 1.26.6` to `go 1.26`. Builds then failed with:
  `go: module ../pinocchio listed in go.work file requires go >= 1.26.11, but go.work lists go 1.26; to update it: go work use`.
  I restored `./geppetto` and set `go 1.26.11`; with `GOTOOLCHAIN=auto` Go fetched the 1.26.11 toolchain and
  `go build ./...` in slop-computer succeeded.

### What I learned

- `Hub.Submit` calls the command handler inline when no event bus is configured (`hub.go:318-328`). A start handler
  that refuses (busy session) can therefore return an error straight to `Service.SubmitPromptRequest`.
- chatapp builds its `enginebuilder.Builder` with only `Base`, `Registry`, `ToolExecutor` and `EventSinks`
  (`runtime_inference.go:108-113`): no loop limits, no final answer, no round guard.
- A new prompt cancels the running one (`runtime_inference.go:49-52`).
- History is loaded from the latest `"final"` turn in the `TurnStore` (`runtime_inference.go:122`), but chatapp never
  saves it: web-chat's composer wraps the engine with a persister (`composer.go:121`), the CLI has its own persister
  (`pkg/cmds/chat_persistence.go`), and the TUI keeps an accumulator via `OnFinalTurn`. On cancel nothing is saved
  (`runtime_inference.go:161-170`).
- geppetto has no code that closes a tool call left without a result. If a stopped turn is kept in the history, the
  next provider request would contain a tool call with no matching result, which providers reject.
- chatapp writes content of its own: the prompt `"Explain evtstream"` when a start command arrives empty
  (`runtime_inference.go:37`), a demo inference when no runtime is given (`:67-71`), and a max-rounds warning text
  (`:179-181`).
- The websocket transport already solves "snapshot, then live, with no gap": it buffers UI batches while a client
  hydrates and flushes only those after the snapshot ordinal (`transport/ws/server.go:664-780`). An SSE transport can
  copy that algorithm.

### What was tricky to build

- N/A (research only).

### What warrants a second pair of eyes

- The workspace `go.work` now lists `go 1.26.11`; workspace builds of slop-computer pick up pinocchio's newer shared
  dependencies (golang.org/x/sys, x/crypto, x/text) through the combined build list.

### What should be done in the future

- Tell the wsm maintainers that `wsm add` drops hand-added `go.work` entries.

### Code review instructions

- Start with the guide's sections 2 and 3, then check the cited lines in `pkg/chatapp/runtime_inference.go`.

### Technical details

- Revisions: pinocchio `22a0e56`, geppetto `b35d0275`, sessionstream `v0.1.2`, Go `1.26.11`.

## Step 2: The design guide, and its upload

I wrote the intern guide (`design-doc/01-…`), reviewed it once end to end, and uploaded it to reMarkable. The guide
teaches the system before proposing changes: geppetto turns, the tool loop and tools; sessionstream's hub, ordinals,
projections and snapshots; chatapp's run path with line references; frontendtools; web-chat's server; agentd; then the
rule (mechanism in the core, content with the caller), the target design, an API sketch, a feature-by-feature mapping,
phases, tests and decision records.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Deliver the guide in the ticket and on the reMarkable before implementing.
**Inferred user intent:** A document someone new can learn the system from, and a plan to review.
**Commit (code):** 94066a3 — "PIN-20261121-AGENTD-INTO-CHATAPP: ticket, intern design guide, diary step 1"

### What I did
- First draft, then a review pass against the code. Corrections made in the review:
  - agentd's size: about 2,100 lines without tests (it includes the scripted engine), not 1,900.
  - The section 6 sketch used types from section 7 without saying so, and set `req.Blocks` on a request type that
    has no such field; it now says where the types are defined and uses httpapi's `Blocks` hook.
  - "geppetto's executor publishes tool events" was checked: `pkg/inference/tools/base_executor.go:112` publishes
    the execution events, which chatapp's tool-call plugin projects.
  - "nothing outside agentd wires the browser-tool executor" was checked with a repository-wide grep for
    `BridgeExecutor|RuntimeContext|WithBridgeContext` outside `pkg/agentd` and `pkg/chatapp/{frontendtools,service.go,runtime_inference.go}`: no hits.
- `docmgr doctor --ticket PIN-20261121-AGENTD-INTO-CHATAPP --stale-after 30`: all checks passed.
- `remarquee upload bundle <guide> --name "PIN-20261121 Merging agentd into chatapp - intern guide" --remote-dir "/ai/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP" --toc-depth 2 --non-interactive`
  → `OK: uploaded PIN-20261121 Merging agentd into chatapp - intern guide.pdf -> /ai/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP`.

### Why
- ASCII diagrams instead of mermaid: the reMarkable pipeline renders Markdown through pandoc, which does not draw
  mermaid.

### What worked
- Upload on the first attempt.

### What didn't work
- I checked the "uploaded" task before uploading; unchecked it (`docmgr task uncheck --id rzci`) and checked it after
  the upload succeeded.

### What I learned
- N/A beyond Step 1.

### What was tricky to build
- Keeping current, proposed and decided statements apart; the guide labels each.

### What warrants a second pair of eyes
- Decisions D3–D8 in section 12 are proposals I made while writing; D1, D2 and D9 come from the user.

### What should be done in the future
- Update the guide's section 10 with what was built once the phases land.

### Code review instructions
- Read the guide sections 4.3 (feature sorting) and 7.2–7.3 (runs and history) first.

### Technical details
- Upload destination: `/ai/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP`.

## Step 3: Phase 1 — chatapp runs are mechanism only

This step changed chatapp's run so that it provides mechanism and nothing else. A caller now configures the round
limit, the final answer and a round guard through `ComposedRuntime`; a busy session is refused or replaced by policy;
every run ends with an outcome decided in one place, published with a code; the caller can append its own blocks to
a turn; and chatapp keeps every run's turn as the conversation, closing tool calls that never got a result. The
content chatapp used to write itself is gone.

The outcome is commit 0ac7b5e. All pinocchio tests pass (`go test ./...`), and the pre-commit hook's `make lintmax`,
`make test` and `make web-check` passed on the second attempt.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 1 of the guide's plan (section 10).
**Inferred user intent:** chatapp becomes the runtime agentd's features can live in.
**Commit (code):** 0ac7b5e — "chatapp: runs are mechanism only (limits, busy policy, outcome codes, kept history)"

### What I did
- `pkg/inference/runtime/composer.go`: `ComposedRuntime.Loop *toolloop.LoopConfig`, `ToolConfig *geptools.ToolConfig`,
  `RoundGuard toolloop.RoundGuard`, passed into the run's `enginebuilder.Builder`.
- `pkg/chatapp/chat.go`: `BusyPolicy` (`BusyReject` default, `BusyReplace`), `ErrRunActive`, `ErrNoRuntime`,
  `WithBusyPolicy`, `WithErrorClassifier`, `WithErrorText`, `WithUnansweredToolResult` /
  `DefaultUnansweredToolResult`; `swapRun` became `reserveRun`, which refuses under `BusyReject`. Removed
  `WithChunkDelay` and the engine's `chunkDelay`.
- `pkg/chatapp/run_outcome.go` (new): `decideOutcome` (finished / stopped / failed; truncated when the final answer
  was given), `CodedError`, the code constants, `ClassifyError` (agentd's text matching, moved), `failureText`,
  `closeUnansweredToolCalls` (geppetto's `toolblocks.ExtractPendingToolCalls` + `AppendToolResultsBlocks`).
- `pkg/chatapp/runtime_inference.go`: the start handler validates first (no empty prompt; `ErrNoRuntime`), reserves
  the run before publishing anything, cancels and awaits a replaced run, then publishes the user message;
  `runRuntimeInference` appends `PromptRequest.Blocks` after the user block, decides the outcome, and `endRun` closes
  unanswered calls, saves the turn as `"final"` and calls `OnFinalTurn` for every outcome. Failed saves are now
  logged.
- `pkg/chatapp/runtime_sink.go`: the failure the sink publishes on geppetto's `EventError` carries a code and the
  configured text.
- `proto/pinocchio/chatapp/v1/chat.proto`: `ChatRunFinished.truncated` (6), `.reason` (7); `ChatRunFailed.code` (5).
  Regenerated Go and TypeScript (see What didn't work).
- `pkg/chatapp/service.go`: `PromptRequest.Blocks []turns.Block`.
- Deleted `pkg/chatapp/demo.go`; removed the max-rounds warning helpers from `messages.go`; `RunnerOptions.ChunkDelay`
  became `RunnerOptions.EngineOptions []Option`.
- Removed the persisters that chatapp now replaces: `pkg/ui` `TurnPersister`/`WithTurnPersister`, the CLI's
  `cliTurnStorePersister` (`pkg/cmds/chat_persistence.go`), web-chat's composer `Persister` (its `SnapshotHook`
  stays) and its `turnStorePersister`.
- Tests: `answer_engine_test.go` (a streaming engine that answers "Answer: <prompt>", replacing the demo);
  `run_outcome_test.go` (busy reject publishes nothing; busy replace stops the first run; no runtime is rejected;
  round limit with and without a final answer; a guard's coded error; classifier and text options; the default
  classifier; a stopped turn keeps the person's message and its unanswered call is closed; caller blocks follow the
  message and the second turn's prompt starts with the first turn's prompt). Existing tests were moved off the demo,
  and the two "persists failed turn" tests now expect phase `"final"`. web-chat's appserver tests got a default
  resolver that answers like the demo did.

### Why
- Rejecting by default: a silent replacement loses the person's running turn; agentd's 409 was the safer contract.
  The CLI and TUI never send while running (the TUI backend refuses itself), so the default does not change them.
- Saving in chatapp: chatapp is the one that loads `"final"`, and there were three savers with different behavior.
- Using geppetto's `toolblocks` for closing calls: the same code geppetto's loop uses to find pending calls.

### What worked
- `go test ./...` in pinocchio: all packages ok.
- `go test ./pkg/chatapp -run Busy -count=20`: ok (the busy tests are not flaky).
- Pre-commit on the second attempt: `✔️ lint (33.51 seconds)`, test and web-check passed; commit created.

### What didn't work
- `buf generate --template buf.chatapp.gen.yaml --path proto/pinocchio` failed with
  `Failure: your Buf API token for buf.build is invalid. Run "buf registry login" using a valid Buf API token.`
  The templates use buf.build remote plugins. I installed the plugin versions the committed files were generated with
  (`protoc-gen-go v1.36.12`; `protoc-gen-es v2.14.0`, from the generated headers) into the scratchpad and ran buf with
  local templates; only the chat files changed. The procedure is `scripts/01-proto-gen-local.sh`.
- The editor's language server reported `go: go.work requires go >= 1.26.11 (running go 1.25.5)` on every edit; it runs
  an older Go than the shell (1.26.11). Ignored.
- First test run after the core change: `TestBusyReplaceCancelsTheRunningTurn` failed with
  `"[]" should have 1 item(s), but has 0` for `ChatRunStopped`. Cause: the replaced run had not started yet; it
  published `ChatRunStarted` with its already-cancelled context, the publish failed, and the run returned without any
  terminal event. Fix: the start event and the history load use `publishContext(ctx)` (non-cancellable); a
  `StartInference` error goes through `decideOutcome` like any other, so a cancelled run ends `ChatRunStopped` and its
  turn is kept.
- `go vet` on the test: `conversion from BlockKind (int) to string yields a string of one rune` — used
  `b.Kind.String()`.
- First commit attempt: the hook's lint failed with three issues — `exhaustive` on the outcome switch and on a
  `BlockKind` switch in a test, and `gofmt` on the new test file. Added the missing cases and ran gofmt;
  `golangci-lint run` on the touched packages then reported `0 issues`, and both vet tools (`/tmp/geppetto-lint`,
  `/tmp/glazed-lint`) exited 0.
- web-chat's appserver tests submitted messages without a runtime resolver and failed with HTTP 500 once the demo was
  gone; they now use a default resolver.

### What I learned
- geppetto sets `MaxIterationsError.Answered` only when the final answer succeeded; a failed final answer wraps its
  cause next to `Answered: false` (`toolloop/limits.go:18-22`, `loop.go:248`).
- The appserver tests and several CLI paths relied on the demo without saying so.

### What was tricky to build
- Ordering in the start handler. The busy check must happen before anything is published, so the refused prompt leaves
  no trace; the replaced run must be cancelled and awaited before the new run loads history, or the new run would read
  a history the old one is still writing. The handler now: validate → reserve (refuse here) → cancel and await the
  replaced run → publish the user message (on failure: clear the reservation, cancel, close `done`) → start the run.

### What warrants a second pair of eyes
- `OnFinalTurn` now fires for every outcome, not only success. The TUI's accumulator therefore keeps stopped and failed
  turns, matching the saved history.
- A save failure is logged, not returned; the TUI used to show a persistence error.
- `ClassifyError` matches text, as agentd did; it should become `errors.As` on typed geppetto errors.

### What should be done in the future
- Typed provider errors in geppetto's OpenAI and Responses engines.

### Code review instructions
- `pkg/chatapp/runtime_inference.go` (`handleStartInference`, `runRuntimeInference`, `endRun`), `pkg/chatapp/run_outcome.go`,
  `pkg/chatapp/run_outcome_test.go`.
- `go test ./pkg/chatapp/... ./pkg/ui/... ./pkg/cmds/... ./cmd/web-chat/...`

### Technical details
- Outcome table as implemented: no error → finished; cancelled → stopped; `MaxIterationsError{Answered: true}` →
  finished, `truncated`, `reason: "max_rounds"`; `CodedError` → failed with its code; `MaxIterationsError` otherwise →
  failed `max_rounds`; anything else → failed with the classifier's code. History load failures → failed
  `history_error`.

## Step 4: Phase 2 — session metadata and usage

This step added `pkg/chatapp/sessions`: who owns a session, its labels and title, and the tokens it used. It holds
only metadata; the conversation stays in the turn store and the timeline in the hydration store. chatapp gained one
hook, `WithUsageObserver`, so the package can count tokens without chatapp knowing about it.

Commit 057330b. `go test ./pkg/chatapp/sessions` passes; the pre-commit hook passed on the second attempt.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 2 of the guide's plan.
**Inferred user intent:** agentd's session list, labels and usage survive the merge as chatapp mechanism.
**Commit (code):** 057330b — "chatapp/sessions: session metadata (owner, labels, title, usage) with memory and SQLite stores"

### What I did
- `sessions.go`: `Info{ID, Owner, Labels, Title, Created, Updated, Usage}`, `Usage{Input, Output, Cached,
  CacheCreation, CacheRead, Calls}` with `Add` and `Tokens`, `Filter{Owner, Labels, Limit}`, the `Store` interface
  (`Create`, `Get`, `List` newest first, atomic `Update`, `Delete`), `ErrNotFound`, `ErrExists`, `TitleFrom`.
- `memory.go` (`MemoryStore`) and `sqlite.go` (`SQLiteStore`, table `chatapp_sessions`, labels and usage as JSON
  columns, mattn/go-sqlite3, which pinocchio already uses).
- `pkg/chatapp/chat.go`: `type UsageObserver func(ctx, sid, *chatappv1.UsageInfo)` and `WithUsageObserver`;
  `runtime_sink.go` calls it on `EventProviderCallFinished` before publishing `ChatProviderCallFinished`.
- `usage.go`: `RecordUsage(store) chatapp.UsageObserver` and `UsageFromProto`.
- Tests: one contract test run against both stores; `TitleFrom`; a chatapp run whose engine reports usage twice adds
  up on the session.

### Why
- An observer, not a plugin as the guide proposed (`NewUsagePlugin`). A plugin sees usage only after the event is
  stored and projected. The observer runs inside geppetto's event sink, which geppetto calls inline, so a round
  guard reading the session's total (phase 4's budget) sees the new value before the next model call.
- A SQLite store next to the memory one: agentd kept sessions in JSON files across restarts, and `pinocchio agent
  serve` needs the same.

### What worked
- Both stores passed the same contract test unchanged.

### What didn't work
- The test first built the event wrongly: `unknown field Usage in struct literal of type events.EventMetadata` and
  `cannot use 0 (untyped int constant) as *int64 value`. Usage is an argument of `NewProviderCallFinishedEvent`,
  and the duration is `*int64`.
- Lint on the first commit attempt: `pkg/chatapp/sessions/sessions.go:108:29: param max has same name as predeclared
  identifier (predeclared)`. My shell command chained the commit after the lint run, so the hook ran and refused the
  commit. Renamed the parameter to `maxRunes`.

### What I learned
- geppetto's `EventProviderCallFinished` carries usage as a field set by the constructor, not through metadata.

### What was tricky to build
- Nothing beyond the hook placement described in Why.

### What warrants a second pair of eyes
- `SQLiteStore.Update` serializes in-process with a mutex; two server processes on one file could interleave a
  read-modify-write. Fine for one `agent serve` process; a shared deployment needs a transaction.
- Deleting a session leaves its turns and timeline rows; `TurnStore` and `HydrationStore` have no delete.

### What should be done in the future
- Update the guide's 7.4 and 8.1: `RecordUsage` and `WithUsageObserver` replace `NewUsagePlugin`.

### Code review instructions
- `pkg/chatapp/sessions/*.go`, `pkg/chatapp/runtime_sink.go` (the `EventProviderCallFinished` case).
- `go test ./pkg/chatapp/sessions`

### Technical details
- SQLite schema: `chatapp_sessions(id PRIMARY KEY, owner, labels JSON, title, created_ms, updated_ms, usage JSON)`,
  index on `(owner, created_ms)`.

## Step 5: Phase 3 — the HTTP API

This step moved web-chat's `internal/appserver` to a public package, `pkg/chatapp/httpapi`, and gave it what
agentd's server had: session routes backed by the sessions store, 409 for a busy session, stop, server-sent events,
and authentication hooks. web-chat now runs on it. The SSE transport hydrates like the websocket does: snapshot
first, then only the events after the snapshot, with no gap and no duplicate.

Commit 9a08f24. `go test ./...` passes in pinocchio (httpapi and sessions also under `-race`); `golangci-lint run
./pkg/... ./cmd/...` reports `0 issues`; the pre-commit hook passed on the first attempt.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 3, then use the resulting API names in the playbook.
**Inferred user intent:** One server package that web-chat, `pinocchio agent serve` and homolog-bench can all use.
**Commit (code):** 9a08f24 — "chatapp/httpapi: the HTTP API (sessions, messages, stop, SSE, auth) replaces web-chat's appserver"

### What I did
- `git mv` of appserver's `server.go`, `snapshot.go`, `routes_exports.go` → `exports.go`, `routes_frontend_tools.go` →
  `tools.go`, `server_test.go` → `httpapi_test.go`; the rest of appserver deleted.
- `server.go`: functional options (`WithRuntimeResolver` (required), `WithSessionStore`, `WithAutoCreateSessions`,
  `WithHydrationStoreSpec/Factory`, `WithTurnStore`, `WithTurnsDBPath`, `WithChatPlugins`, `WithFrontendTools`,
  `WithEngineOptions`, `WithAuthenticator`, `WithAuthorizer`, `WithBlocks`, `WithSSEConfig`, `WithMaxBodyBytes`,
  `WithPrefix`), `New`, `Register(mux)` with Go 1.22 method patterns, `Handler()`, `Service()`, `Sessions()`, `Hub()`.
  `RuntimeResolver.Resolve(ctx, r, sessions.Info, MessageRequest)`. A `transports` fanout over the websocket server
  and the SSE hub.
- `auth.go`: `Principal`, `Action` (`create`, `list`, `read`, `send`, `stop`, `delete`, `tools`), `Authenticator`,
  `Authorizer`, `OwnerOnly` (default), `PrincipalFromContext`.
- `sessions.go`: the `session(action, handler)` wrapper (authenticate → validate id → look up or auto-create →
  authorize), create/list/get/delete/stop/message handlers.
- `sse.go`: `sseHub` (a `UIFanout`), per-stream hydration buffer and bounded live queue, `handleEvents`,
  `SSEConfig`.
- `types.go`: `ErrorResponse{error, code}` and the code constants, `CreateSessionRequest`, `SessionResponse`,
  `SessionListResponse`, `MessageRequest` (with `Raw`), `AcceptedResponse`.
- chatapp: `Engine.Running` and `Service.Running`.
- web-chat: `webapp/routes.go` calls `Register`; `webchatcmd/run.go` builds `httpapi.New(...)` with
  `WithAutoCreateSessions(true)`; `runtime/canonical_resolver.go` takes the new signature. Its tests moved to the new
  API; the mock-parity HTTP test moved to `cmd/web-chat/main_runtime_test.go`, because `pkg/` cannot import web-chat's
  internal mock runtime.
- serverkit: removed the request/response contracts, `DecodeJSON`, `WriteError`, `ParseSessionPath*` and their test;
  only `SnapshotEntity` and `SessionSnapshotResponse` remain.
- Docs: `cmd/web-chat/README.md` route list; `pkg/doc/topics/chatapp-protobuf-plugins.md` names `httpapi.MessageRequest`.

### Why
- Functional options instead of the guide's `Options` struct: pinocchio's `AGENTS.md` says "Use functional options
  pattern for configurable components", and appserver already used them.
- Auto-creation as an option: chat-provider (`node_modules/@go-go-golems/chat-provider/core/createChatClient.js`)
  stores the session id in the URL and session storage and reuses it without creating it again. With an in-memory
  session store, a restarted server would answer 404 for every message.
- `sessionId` in the create answer: chat-provider reads `data.sessionId`.
- The fanout logs delivery errors instead of returning them: sessionstream's hub treats a fanout error as a failed
  publish (`hub.go:510`), but the event is already stored, and one closed browser tab must not fail the run.
- `running` on `GET /sessions/{id}`: the snapshot's `status` is inferred from entities and cannot see a run that was
  stopped before any output (below).

### What worked
- The SSE no-gap test: a stream opened before the message sees every UI event; a second stream opened mid-run gets a
  snapshot ordinal > 0 and then exactly the first stream's events with higher ordinals, in order.
- `go test -race ./pkg/chatapp/httpapi ./pkg/chatapp/sessions`: ok.

### What didn't work
- `use of internal package github.com/go-go-golems/pinocchio/cmd/web-chat/internal/mockruntime not allowed` once the
  test file sat in `pkg/`. Moved that one test to web-chat.
- Two mistakes in my first draft, caught when building: a placeholder type `[]turnsBlock` and a drain loop whose
  `break` only left the `select`, which would have looped forever after an overflow.
- `TestSecondMessageWhileRunningIsRefusedWith409` timed out: `run did not stop; status "streaming"`. The snapshot
  held only the user entity (`"status": "accepted"`). The stop reached the run before its first text segment, and
  chatapp projects nothing for `ChatRunStopped` (only `ChatRunFailed` gets an entity, role `error`). Segment entities
  use ids derived from the run's message id (`<messageID><delimiter><segment>`), so the projection cannot tell
  whether a stopped run produced output. I did not change the projection; I added `Service.Running` and a `running`
  field, and the test waits on that.

### What I learned
- chat-provider already calls `POST /sessions/{id}/stop`; appserver never had that route, so web-chat's stop button
  only cancelled frontend tools.
- sessionstream's websocket server runs its subscribe authorizer with a context derived from the upgrade request, so a
  principal put into the request context reaches it.

### What was tricky to build
- The SSE hand-over from snapshot to live. The stream must register before reading the snapshot (otherwise an event
  published in between is lost), keep what arrives meanwhile, drop what the snapshot already contains (ordinal ≤
  snapshot ordinal; the hub stores before it fans out, so an event in the snapshot may still be delivered after
  registration), then switch to live under the same lock. Batches queued after the switch come after the buffered
  ones because one goroutine writes both.
- The hub calls the fanout inside its publish, so the SSE fanout must never block: a full queue closes the stream with
  an `error` event (`stream_overflow`), and the client reconnects.

### What warrants a second pair of eyes
- A run stopped before any output leaves no assistant entity; a client that hydrates later sees the person's message
  as "accepted". `running` covers the "is it still going" question, but the timeline itself shows nothing.
- Auto-created sessions get the requesting principal as owner. With authentication, whoever first uses an unknown id
  owns it; ids are uuids by default.
- Each SSE write sets its own deadline (`SSEConfig.WriteTimeout`, 10 s) through `http.ResponseController`, which
  replaces web-chat's 60 s server `WriteTimeout` for the stream.

### What should be done in the future
- Project `ChatRunStopped` into the timeline (needs to know whether the run had output, e.g. a field on the event).
- The SSE transport could move into sessionstream next to the websocket transport.

### Code review instructions
- Start with `pkg/chatapp/httpapi/server.go` (route table), then `sessions.go` (`session`, `lookup`,
  `handleMessage`), then `sse.go` (`offer`, `goLive`, `handleEvents`).
- `go test -race ./pkg/chatapp/httpapi`; `go test ./cmd/web-chat/...`

### Technical details
- SSE wire format: `event: snapshot` + `data: SessionSnapshotResponse`; per UI event `id: <ordinal>`, `event: ui`,
  `data: {"sessionId","ordinal","name","payload"}` (payload is protojson); `event: error` + `data: {"code","error"}`
  before the server ends a stream (`hydration_buffer_overflow`, `stream_overflow`, `session_deleted`,
  `server_closed`); `: ping` every 15 s.
- Error answers: `{"error": "...", "code": "..."}` with codes `invalid_request`, `request_too_large`,
  `invalid_session_id`, `session_not_found`, `session_exists`, `unauthenticated`, `forbidden`, `run_active`,
  `runtime_error` (or the resolver's `CodedError` code), `no_runtime`, `frontend_tools_disabled`, `internal_error`.

## Step 6: Phase 4 — browser tools in every run, results of any JSON type, the toolkit

This step made chatapp's runs use the page's browser tools, which until now only agentd wired, and let a browser
tool return any JSON value. It also added three optional helper packages: per-call deadlines and result cuts, a
session token budget, and agentd's preset files as plain data.

Commits 11da2fa (tools) and 7cbef6f (toolkit). `go test ./...` passes; the hook passed both times on the first
attempt, including `web-check` for the regenerated TypeScript.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement phase 4: "chatapp wires browser tools; frontendtools results as any JSON value"
and "toolkit: toolexec WithDeadline/CutResults, budget.SessionTokens, presets loader".
**Inferred user intent:** homolog-bench gets the same browser-tool behavior from chatapp that agentd gave it, plus the
limits it relied on, without any of them being chatapp's content.
**Commit (code):** 11da2fa — "chatapp: runs get the page's browser tools; tool results are any JSON value";
7cbef6f — "chatapp/toolkit: tool deadlines and result cuts, session token budgets, presets"

### What I did
- `pkg/chatapp/chat.go`: `RunScope{SessionID, MessageID, Publisher}`, `RunTools{Registry, Executor, Config}` with
  `ExecutorOrDefault()`, `RunToolsFunc`, `WithRunTools`. `runtime_inference.go` applies the functions after
  `PromptRequest.RuntimeContext` and before building the run; an error fails the run with `CodeToolsError`
  (`"tools_error"`).
- `pkg/chatapp/frontendtools/bridge.go`: `(*Manager).RunTools()`; sessions without available tools run unchanged.
  `httpapi.WithFrontendTools` installs it.
- `proto/pinocchio/chatapp/frontendtools/v1/frontend_tool.proto`: the three `result` fields became
  `google.protobuf.Value`; regenerated with `scripts/01-proto-gen-local.sh` (only the frontendtools Go and TS files
  changed). `BridgeExecutor` gives the model `AsInterface()`; httpapi's result route accepts `result` of any JSON type;
  agentd's result route passes its value through (agentd is deleted in phase 5).
- `pkg/chatapp/toolkit/toolexec`: `Middleware`, `Chain`, `Deadline`, `CutResults`, `RunTools`,
  `DefaultDeadlineMessage`, `DefaultCutNote`.
- `pkg/chatapp/toolkit/budget`: `SessionTokens(store, limit)`, `QuotaExceededError` (`Code() == "quota_exceeded"`),
  `All`.
- `pkg/chatapp/toolkit/presets`: `Preset`, `LoadFile`, `Load`, `Names`, `Duration`.
- Tests: `TestBrowserToolRoundTripThroughHTTP` (manifest → message → `ChatFrontendToolCallRequested` over SSE →
  string result posted → answer `the page says: "Hello from the page"`); chatapp hook tests; toolexec, budget (a real
  chatapp tool loop stopped by the guard after two model calls) and presets tests.

### Why
- A hook in chatapp rather than chatapp calling frontendtools: `pkg/chatapp/frontendtools/plugin.go` imports
  `chatapp` (`chatapp.ChatPlugin`), so the other direction would be an import cycle. The hook is also what any other
  tool source needs.
- `RunTools.Config` and `ExecutorOrDefault`: geppetto's loop builds its default executor from the run's
  `ToolConfig` only when the executor is nil (`toolloop/loop.go:361-363`). A wrapper around a nil executor has to
  build that default itself, and `NewBridgeExecutor(m, nil)` used `DefaultToolConfig()`, dropping the runtime's
  timeouts and parallelism.
- `Value` instead of the guide's fallback (wrapping inside frontendtools): chat-provider posts objects
  (`normalizeRecord` in `tools/toolRuntime.js`) and reads `payload.result` from JSON, not through generated types, and
  web-chat's own source does not import the generated frontendtools types. An object encodes the same as a `Struct`
  or a `Value`, so nothing on the wire changed for them.
- The toolkit's messages are parameters: agentd's `TimeoutMessage` and cut note told the model how to behave ("Do not
  wait in loops…"), which is content.

### What worked
- The browser-tool round trip passed on its first run.
- The budget test confirmed that the loop's context carries the geppetto session id (`WithSessionMeta` in
  `session.go:282`), so a single guard serves every session.

### What didn't work
- `go build` after the proto change: `result.GetResult().AsMap undefined (type *structpb.Value has no field or method
  AsMap)` in `bridge.go:115`, then `cannot use st (variable of type *structpb.Struct) as *structpb.Value` in
  `pkg/agentd/server.go:413`, then the frontendtools tests (`resultCommand` helper and `AsMap` reads). Fixed each.
- `pkg/chatapp/runtime_inference.go:121:88: undefined: geptools` after adding `RunTools.Config`: goimports did not
  add the import (two candidates with the same name); added it by hand.
- `TestCutResults` failed with `expected: "{\"a\":1|++"`, `actual: "{\"a\":|++"`: my expectation was wrong
  (`{"a":1}` is 7 characters; 5 kept, 2 cut).

### What I learned
- geppetto's loop JSON-encodes every tool result into the `tool_use` block (`toolloop/loop.go:177-182`), so a string
  result reaches the model with quotes: exactly the JSON value.
- web-chat accepted browser-tool manifests, but its runs never offered those tools to the model: nothing outside
  agentd set the bridge context.

### What was tricky to build
- Order of executor wrappers. frontendtools routes browser calls before the runtime's executor, so a deadline set on
  `ComposedRuntime.ToolExecutor` would not cover them. `toolexec.RunTools` wraps the run's final executor and has to
  be installed after `Manager.RunTools`; with httpapi that holds because httpapi's own engine options come before
  `WithEngineOptions`.

### What warrants a second pair of eyes
- `Deadline` runs a batch one call at a time (as `BridgeExecutor` already does); with in-process tools that are
  normally run in parallel, adding a deadline makes them sequential.
- `SessionTokens` fails closed on store errors other than not-found.

### What should be done in the future
- `frontendtools` tab liveness (fail a browser call whose executor connection is gone), noted in the guide's 7.5.

### Code review instructions
- `pkg/chatapp/chat.go` (`RunTools` types), `pkg/chatapp/runtime_inference.go` (the loop over `e.runTools`),
  `pkg/chatapp/frontendtools/bridge.go` (`RunTools`), `pkg/chatapp/toolkit/*`.
- `go test ./pkg/chatapp/... -run 'RunTools|BrowserTool|Budget|Cut|Deadline|Load'`

### Technical details
- Installing everything with httpapi:
  `httpapi.New(httpapi.WithFrontendTools(m), httpapi.WithEngineOptions(chatapp.WithRunTools(toolexec.RunTools(toolexec.Deadline(2*time.Minute, msg), toolexec.CutResults(9000, note)))), ...)`
  and per run `ComposedRuntime{RoundGuard: budget.SessionTokens(srv.Sessions(), limit)}`.

## Step 7: Phase 5 — `pinocchio agent serve` on chatapp, agentd deleted

This step rebuilt `pinocchio agent serve` as an application of chatapp and deleted `pkg/agentd`. A new package,
`pkg/chatapp/agentserver`, is an httpapi server whose sessions each run one preset; agentd's scripted test engine
moved to `pkg/chatapp/toolkit/scripted`. Getting there needed four small additions to the mechanism: opaque session
data, a create hook, a runtime-level executor wrapper, and a default in-memory turn store.

Commits db83f65 (the additions) and e67e8e1 (agent serve, agentd deleted). `go test ./...` passes; `golangci-lint
run ./pkg/... ./cmd/...` reports `0 issues`; `scripts/02-agent-serve-smoke.sh` runs the real binary end to end.

### Prompt Context
**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Phase 5: "`pinocchio agent serve` on httpapi + presets + modelsource; delete pkg/agentd; docs".
**Inferred user intent:** One agent runtime in pinocchio; homolog-bench moves to it with a playbook.
**Commit (code):** db83f65 — "chatapp: session data, a create hook, running in listings, runtime executor wrapper";
e67e8e1 — "agent serve runs on chatapp; pkg/agentd is deleted"

### What I did
- Checked how homolog-bench uses agentd (`~/code/wesen/homolog-bench`, main at b058c23): it runs `pinocchio agent serve
  --agents configs/agent` as a separate process behind its own proxy (`/api/v1/agent`), its UI test starts it with
  `--engine fake`, and its preset uses `pageInstructions: true` to append the page's API reference to the system
  prompt at session creation.
- `sessions.Info.Data` (SQLite column `data`), `CreateSessionRequest.Data`, `httpapi.WithCreateHook` (POST and
  auto-creation; the hook cannot change the id), `SessionSummary` with `running` in `GET /sessions`,
  `ComposedRuntime.WrapToolExecutor` applied after the run-tools functions, `toolexec.Compose`.
- httpapi defaults to `serverkit.NewMemoryTurnStore()`; new test `TestASessionRemembersItsConversation`.
- `pkg/chatapp/agentserver`: `New(presets, opts...)` with `WithModels`, `WithEngine`, `WithStateDir`,
  `WithDefaultPreset`, `WithPrefix`, `WithTexts`, `WithToolRegistry`, `WithHTTPOptions`; the create hook
  (`completeSession`) and the resolver (`resolve`); `GET /health`, `/presets`, `/models`; error codes
  `unknown_preset`, `unknown_model`, `instructions_not_allowed`, `prompt_too_large`.
- `pkg/chatapp/toolkit/scripted`: agentd's `FakeEngine` as `scripted.Engine`, plus a `TextSegmentFinished` event.
- `cmd/pinocchio/cmds/agent.go`: flags `--presets`, `--default-preset`, `--prefix`; state dir
  `~/.local/state/pinocchio/agent`. `pkg/agentd/agentcli` → `pkg/cmds/agentcli`; `pkg/agentd` deleted.
- `pkg/doc/topics/pinocchio-agent-serve.md`: the help topic the command's long help points to.
- `scripts/02-agent-serve-smoke.sh`: build, start with `--engine fake`, create with instructions, register `eval`,
  message `/tool eval {"code":"6*7"}`, answer 42, read the conversation and the SSE event names.

### Why
- Page instructions stay, but in agentserver: they are content, and agentserver is an application. They go into the
  session's system prompt once, at creation (session data), so the prefix is the same every turn.
- `WrapToolExecutor` on the runtime: a preset's tool timeout is per session, but `WithRunTools` is engine-wide, and a
  wrapper set as `ComposedRuntime.ToolExecutor` would sit behind the browser-tool bridge and not cover browser calls.
- The default turn store: without one, chatapp has no history (it loads history only from a store), so every message
  of an httpapi session started a new conversation; web-chat always passed one, which hid this.
- The scripted engine now finds "the person's message" as the first user block of the turn's last run of user blocks,
  because caller blocks after the message would otherwise be read as the script.

### What worked
- The smoke test against the binary: `eval returned: 42`; SSE carried `ChatFrontendToolCallRequested`,
  `ChatFrontendToolResultReceived`, `ChatRunFinished`, and the session's usage was recorded (1772 input tokens, the
  system prompt included).
- The restart test: a second server on the same state dir continues the conversation.

### What didn't work
- homolog-bench's preset as-is: `homolog-bench.yaml: yaml: unmarshal errors: line 14: cannot unmarshal !!map into
  []string` — its `tools: {eval: auto}` approval map. The page declares a tool's mode in its manifest now, so the
  playbook says to delete it. I left the YAML error as it is.
- First agentserver test run, two failures, both in my tests: the system-prompt assertion looked for an escaped `\n`
  in YAML that has real line breaks; and a 1-token budget stopped the first message between rounds
  (`error: the session used 9 of its 1 tokens`), which is the guard working, so the budget got its own preset and test.
- `TestFullExportOmitsTurnsWhenStoreUnavailable` failed after the turn-store default (the export now has turns);
  renamed and inverted.

### What I learned
- agentd's package comment points to a `pkg/doc/topics` page about agentd that never existed; there was no help page
  to migrate.

### What was tricky to build
- Keeping agentd's behavior without its mechanisms: the final-round instruction was a context block agentd removed on
  the next turn; now it is an ordinary block that stays in the conversation (append-only), which is also what the
  turn export shows.

### What warrants a second pair of eyes
- agentserver defaults: tool retries 0 (a retried browser call runs page code twice), one tool at a time,
  geppetto's own execution timeout = preset timeout + 5 s so the deadline wrapper acts first.
- `pageInstructions` and `maxPageInstructionsBytes` are read from `Preset.Extra`, not typed fields.

### What should be done in the future
- homolog-bench's migration (the playbook).

### Code review instructions
- `pkg/chatapp/agentserver/agentserver.go` (`completeSession`, `resolve`), `cmd/pinocchio/cmds/agent.go`.
- `go test ./pkg/chatapp/agentserver ./pkg/chatapp/toolkit/scripted`; `ttmp/.../scripts/02-agent-serve-smoke.sh`

### Technical details
- Removed flags: `--agents` → `--presets`. Removed routes: `/v1/agents` (→ `/v1/presets`), `/v1/sessions/{id}/turns`
  (→ `/messages`), `/tools/{callId}/result` (→ `/tools/results`), `/tools/{callId}/progress` (gone), `/cancel`
  (→ `/stop`).

## Step 8: Mapping homolog-bench's client, the playbook, the guide as built, final validation

This step wrote the migration playbook for the homolog-bench developer. Writing it meant reading homolog-bench's
agent page (`web/js/agent.js`, 531 lines) call by call against the new API. That reading found three things agent
serve lacked, which I fixed first; then the playbook, the guide's "as built" section, and the final checks.

Commit e84a616 for the code; this step's ticket commit for the documents. Final validation: `go test ./...` (no
failures), `make lint` (`0 issues`, both vet tools), `docmgr doctor --ticket PIN-20261121-AGENTD-INTO-CHATAPP`
(`All checks passed`).

### Prompt Context
**User prompt (verbatim):** "keepa detailed diary and also write a migration playbook so that we can hand it off to our homolog-bench colleague"
(mid-turn, before Step 1's work started); later "ok, how far are we?", "what are we spending time on?" and "continue".
**Assistant interpretation:** Hand homolog-bench's side over as a document precise enough to follow without the
pinocchio history, after the API it describes exists.
**Inferred user intent:** The homolog-bench developer ports the page alone.
**Commit (code):** e84a616 — "agentserver: message context blocks, person timeout for approved tools, reasoning and tool-call plugins"

### What I did
- Read agent.js: its `api()` error parsing (`data.error.{code,message,fix}`), the 11 SSE event types it listens to,
  hydration from `view.messages`/`view.seq`/`view.running`, `POST /turns {text, context: snapshot()}`,
  `/tools/{callId}/result` with `{value}` unwrapping, `/cancel`, `?agent=` listing, executor tracking through
  `tools.manifest` events, and the human-mode Run/Decline path.
- Fixed in agentserver (e84a616): message `context` → a block after the message (`httpapi.WithBlocks`,
  `maxContextBytes`); `personTimeout` for `human`-mode browser tools via the new `toolexec.DeadlineBy`; the reasoning
  and tool-call plugins; httpapi answers a BlocksFunc's `CodedError` code. Tests for the first two.
- Captured the real SSE stream of the smoke script to document the payload fields (`messageId` of the run, text
  segment ids `<run>:text:<segment>`, `CHAT_STREAM_PATCH_MODE_APPEND` deltas with `offset`, tool modes as
  `TOOL_EXECUTION_MODE_FRONTEND_*`).
- `playbook/01-…`: seven steps (preset, route/field map, errors, event stream and event map, hydration from entities,
  creating and sending with the append-only consequence for `context`, tools/approval/timing, the UI test), exit
  criteria, notes.
- Guide: status paragraph in section 0, notes in 7.4/7.6, commit list in 10, decision statuses in 12, new section 14
  (differences table with reasons, API as built, known gaps), RelatedFiles moved off deleted files.

### Why
- The three fixes before the playbook: without them the playbook would have had to say "this no longer works"
  for the page's reasoning display, its per-turn page state, and approvals longer than 120 s.

### What worked
- The playbook's claims were checked against the code: status codes (`late_result` → 410), result statuses
  (`success|failed|denied|cancelled|timeout`), error code strings, entity fields from the protos.

### What didn't work
- One claim was wrong in my first draft: I wrote the tool-call plugin's entity kind as `ToolCall`; it is
  `ChatToolCall` (`pkg/chatapp/plugins/toolcall.go:29`) with `toolName`/`input`. Corrected.
- `gepsession` and `toolv1` imports were missing twice after scripted edits (`undefined: gepsession`); added by hand.

### What I learned
- homolog-bench's page sends its whole page state with every turn. Under agentd that was cheap for the conversation
  (deleted next turn) but broke caching; under chatapp it is kept, so the playbook tells the page to send changes or
  let the model read state through `eval`.

### What was tricky to build
- Deciding what of agentd's "content" features to keep. They stay out of chatapp and httpapi, but agentserver is an
  application, so page instructions, context and the person timeout live there as preset options.

### What warrants a second pair of eyes
- The playbook's event table is the contract the page will be rewritten against; a review by the homolog-bench
  developer before they start would catch anything the page uses that I missed.
- A run stopped before output is invisible in snapshots (guide 14, known gaps).

### What should be done in the future
- homolog-bench's port (phase 6). Then decide whether `ChatRunStopped` gets a timeline entity.

### Code review instructions
- Read the playbook with agent.js side by side; run `scripts/02-agent-serve-smoke.sh` with homolog-bench's
  `configs/agent` after removing its `tools:` map.

### Technical details
- Branch state: `task/merge-agentd-into-chatapp` in the pinocchio worktree, code commits 0ac7b5e → e84a616.
