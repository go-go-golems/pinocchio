---
Title: 'Intern guide: merging agentd into chatapp (one agent runtime, mechanism in the core)'
Ticket: PIN-20261121-AGENTD-INTO-CHATAPP
Status: active
Topics:
    - chatapp
    - refactor
    - design
    - migration
    - sessionstream
    - backend
DocType: design-doc
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: repo://cmd/web-chat/internal/appserver/server.go
      Note: web-chat HTTP stack to be promoted to pkg/chatapp/httpapi
    - Path: repo://pkg/agentd/session.go
      Note: agentd's run, context-block and tool-policy code being merged or removed
    - Path: repo://pkg/chatapp/frontendtools/manager.go
      Note: browser-tool manager chatapp will wire directly
    - Path: repo://pkg/chatapp/runtime_inference.go
      Note: the run lifecycle the merge changes (start handler, history, builder, outcomes)
    - Path: repo://pkg/inference/runtime/composer.go
      Note: ComposedRuntime gains Loop/ToolConfig/RoundGuard
ExternalSources: []
Summary: How pinocchio's two agent runtimes (chatapp and agentd) work today, why agentd is merged into chatapp, the rule that the core holds mechanism while the caller owns everything in the context window, the target design with API sketches, and the phased implementation and test plan.
LastUpdated: 2026-11-21T23:30:00-05:00
WhatFor: Onboard a new engineer to pinocchio's agent runtime and guide the agentd → chatapp merge.
WhenToUse: Before changing pkg/chatapp, pkg/agentd, cmd/web-chat or the homolog-bench integration.
---

# Intern guide: merging agentd into chatapp

## 0. What this document is

This guide explains how pinocchio runs LLM agents, why it currently has two separate runtimes for that, and how we
are merging them into one. It is written for someone who knows Go and has used an LLM API, but has not worked on
pinocchio, geppetto or sessionstream before. Read sections 1–4 to understand the system as it is, sections 5–7 for
the design, and sections 8–11 when you implement or review a phase. Section 13 lists the files. The homolog-bench
side of the change has its own document in this ticket: `playbook/01-migration-playbook-homolog-bench-from-agentd-to-chatapp.md`.

Three kinds of statements appear below and are kept apart:

- **Current behavior**, with file and line references. These were checked against the code at pinocchio `22a0e56`,
  geppetto `b35d0275` (v0.14.3) and sessionstream `v0.1.2`.
- **Proposed behavior**, marked as such. API sketches are sketches: names may change during implementation, and the
  implementation status in section 10 says what has landed.
- **Decisions**, collected as short records in section 12.

## 1. The system in one page

An *agent* here means a program that sends a person's message to a language model, lets the model ask for tools to be
run, runs them, sends the results back, and repeats until the model answers in plain text. One message from the person
and everything the model does until it answers is called a *turn*; one model call inside a turn is called a *round*.

pinocchio has two codebases that do this:

- **chatapp** (`pkg/chatapp`) is pinocchio's main chat engine. The CLI's blocking and JSONL RPC modes, the terminal UI
  (`--chat`) and the example web application `cmd/web-chat` all run on it. It is built on sessionstream, a library for
  per-session event streams with persistence and live fan-out to clients.
- **agentd** (`pkg/agentd`) is a newer agent service, released as `pinocchio agent serve`. homolog-bench's server
  embeds it as a library. It has features chatapp lacks: a session list, loop limits with a forced final answer, tools
  that run inside the server process, tool deadlines, token budgets, error codes, and an HTTP API with server-sent
  events.

Both run the same geppetto tool loop, and both reimplement the same surrounding work: starting and cancelling runs,
mapping geppetto's events to client events, loading and saving history, and describing which model and tools an agent
uses. This ticket deletes agentd and moves the mechanism it adds into chatapp.

```text
            today                                         after this ticket

  CLI / TUI / RPC / web-chat     homolog-bench         CLI / TUI / RPC / web-chat / agent serve / homolog-bench
            │                         │                                   │
            ▼                         ▼                                   ▼
      pkg/chatapp               pkg/agentd                          pkg/chatapp  (+ httpapi, sessions, toolkit)
   (sessionstream hub,      (own event log, own                     (sessionstream hub, one run lifecycle,
    projections, stores)     store, own HTTP/SSE)                     one store set, one HTTP/SSE/ws surface)
            │                         │                                   │
            └──────── geppetto: engines, tool loop, tools, events ────────┘
```

The merge follows one rule that shapes most decisions below: **the core provides mechanism; everything that goes into
the model's context window belongs to the caller.** The core starts runs, enforces limits the caller configured,
executes tools, streams events and stores history. It never writes a system prompt, injects a status block, rewords
an error for the model, or decides how long a tool may take. Section 5 explains why.

## 2. Concepts you need first

### 2.1 geppetto: turns, blocks and engines

geppetto (`github.com/go-go-golems/geppetto`) is the inference library under pinocchio. Its central data structure is
the **turn**: an ordered list of **blocks**, each with a kind and a payload (`pkg/turns`). A conversation is not stored
as a list of turns; the latest turn holds the whole conversation, and each new turn is a clone of the previous one with
blocks appended.

A turn after one user message and one tool round looks like this (simplified YAML of `turns.Turn`):

```yaml
id: t-3
blocks:
  - kind: system      # optional; whoever builds the turn decides
    payload: {text: "You are a careful assistant."}
  - kind: user
    payload: {text: "How many files are in src/?"}
  - kind: tool_call   # the model asked for a tool
    payload: {id: call_1, name: list_files, args: {dir: "src"}}
  - kind: tool_use    # the tool's result, matched to the call by id
    payload: {id: call_1, result: "12 files"}
  - kind: llm_text    # the model's answer
    payload: {text: "There are 12 files in src/."}
```

An **engine** (`pkg/inference/engine`) is one provider connection (OpenAI Responses, Claude, Gemini, …).
`RunInference(ctx, turn)` sends the turn to the model once and returns the turn with the model's blocks appended. An
engine does not run tools.

Notice two things. First, a `tool_call` must be followed by a `tool_use` with the same id before the next model call;
providers reject a request that contains an unanswered tool call. Second, the order of blocks is what the provider sees,
so whatever changes earlier blocks changes the start of the next request (section 4.4 shows why that matters).

### 2.2 The tool loop

The **tool loop** (`pkg/inference/toolloop`) turns an engine into an agent. In pseudocode:

```text
for round := 0; round < MaxIterations; round++:
    RoundGuard(ctx, turn, round)            // optional; an error stops the loop before the call
    turn = engine.RunInference(ctx, turn)   // one round
    calls = tool calls in the new blocks
    if no calls: return turn                // the model answered
    results = executor.Execute(calls)       // run the tools
    append a tool_use block per result
if FinalAnswer configured:
    append FinalAnswer.Prompt(MaxIterations)   // a block the caller supplies
    one more call with tool choice "none"      // the model must answer
    return turn, MaxIterationsError{Answered: true}
return turn, MaxIterationsError{Answered: false}
```

The configuration is `toolloop.LoopConfig{MaxIterations int; FinalAnswer *FinalAnswer}` (`config.go`; default 20
rounds, no final answer) and `toolloop.RoundGuard func(ctx, *turns.Turn, round int) error` (`limits.go:40`). Callers
usually build the loop through `enginebuilder.Builder` (`pkg/inference/toolloop/enginebuilder/builder.go:38`), whose
fields include `Base` (the engine), `Middlewares`, `Registry` (tools; nil means a single model call with no loop),
`LoopConfig`, `ToolConfig`, `ToolExecutor`, `EventSinks`, `SnapshotHook`, `RoundGuard` and `Persister`.

### 2.3 Tools

A tool is a `ToolDefinition` (name, description, JSON schema, and a Go function) held in a `ToolRegistry`
(`pkg/inference/tools/registry.go:9`). A `ToolExecutor` (`executor.go:8`) runs the calls the model makes:

```go
type ToolExecutor interface {
    ExecuteToolCall(ctx context.Context, call ToolCall, reg ToolRegistry) (*ToolResult, error)
    ExecuteToolCalls(ctx context.Context, calls []ToolCall, reg ToolRegistry) ([]*ToolResult, error)
}
```

Executors compose by wrapping: an outer executor can add a deadline or cut a long result, then call the inner one.
`ToolResult{ID, Result, Error}` is what the model receives; a non-empty `Error` reaches the model as a failed call it
can react to. `ToolConfig` holds execution settings such as `ExecutionTimeout`, `MaxParallelTools` and `RetryConfig`.

### 2.4 Events

While it runs, geppetto publishes events to `EventSink`s (`pkg/events/sink.go:15`): text and reasoning deltas, tool
call started/requested/finished, and `EventProviderCallFinished`, which carries the call's token usage (input, output,
cached tokens). Consumers translate these into whatever their clients need.

### 2.5 sessionstream: one ordered event stream per session

sessionstream (`github.com/go-go-golems/sessionstream`, `pkg/sessionstream`) is the event backbone chatapp runs on. It
has a few concepts, and they fit together like this:

```text
 command ──► Hub.Submit ──► command handler ──► publishes backend events
                                                     │  (each gets the next ordinal for its session)
                                                     ▼
                                         EventStore.AppendEvent   (optional: replay log)
                                                     │
                                 ┌───────────────────┴────────────────────┐
                                 ▼                                        ▼
                        UI projection                          timeline projection
                  (backend event → UI events)          (backend event → timeline entities)
                                 │                                        │
                                 ▼                                        ▼
                     UIFanout.PublishUI                      HydrationStore.Apply
                 (to connected clients: ws, …)          (current state of the session)
                                                                          │
                                                           Snapshot(sid) ─┘ (what a new client loads)
```

- A **command** is a request to do something (start inference, stop). `Hub.Submit(ctx, sid, name, payload)` validates
  the payload type and calls the registered handler. Without an event bus the call is synchronous: the handler runs
  inside `Submit` and its error is `Submit`'s error (`hub.go:204-219`, `dispatch` at `:318`).
- A **backend event** is a fact that happened (`Event{Name, Payload, SessionId, Ordinal}`, `types.go:21`). The hub gives
  each event the next **ordinal** of its session, so events of one session are totally ordered.
- **Projections** turn backend events into two outputs. *UI events* are what connected clients receive live.
  *Timeline entities* (`TimelineEntity{Kind, Id, CreatedOrdinal, LastEventOrdinal, Payload, Tombstone}`,
  `projection.go:16`) are the durable current state: one entity per chat message, tool call, and so on.
- The **hydration store** keeps the timeline entities; `Snapshot(sid)` returns them with the ordinal they reflect
  (`Snapshot{SessionId, SnapshotOrdinal, Entities}`, `hydration.go:44`). Backends exist for memory, SQLite and MySQL.
- A **UI fanout** delivers UI events to clients. The websocket transport (`transport/ws`) is one.

A client that connects sends "subscribe", receives a snapshot, then live UI events. The websocket server avoids a gap
between the two: while a client hydrates it buffers incoming UI batches, and after sending the snapshot it flushes only
the batches whose ordinal is greater than the snapshot's (`transport/ws/server.go:664-780`). A reconnecting client
simply hydrates again. The server does not replay missed UI events by ordinal (`server.go:116-124`).

## 3. chatapp today

### 3.1 The pieces

- `Engine` (`pkg/chatapp/chat.go:58`) owns active runs per session, pending prompt requests, plugins and an optional
  `chatstore.TurnStore`. It handles two commands, `ChatStartInference` and `ChatStopInference`, and provides the UI
  and timeline projections (`Install`, `chat.go:195`).
- `Service` (`service.go:45`) is the app-facing surface: `SubmitPromptRequest`, `Stop`, `WaitIdle`, `Snapshot`,
  `SubmitCommand`.
- `PromptRequest` (`service.go:17`) is one prompt: `Prompt`, `Attachments`, `IdempotencyKey`, `Runtime`,
  `InitialTurn` (a fully built turn that replaces history loading), `OnFinalTurn` (a callback with the final turn) and
  `RuntimeContext` (a hook to decorate the run's `context.Context`, intended for the browser-tool bridge).
- `ComposedRuntime` (`pkg/inference/runtime/composer.go:28`) is what the caller supplies per prompt: `Engine`,
  `Registry`, `ToolExecutor`, `WrapSink`, `RuntimeKey`, `RuntimeFingerprint`. The caller decides the model, the
  middlewares (including any system prompt) and the tools.
- **Plugins** (`ChatPlugin`, `features.go:11`) extend chatapp: they register schemas, translate geppetto events, and add
  UI events and timeline entities. Existing plugins: reasoning, tool calls, browser tools (`frontendtools.NewPlugin`),
  widgets.
- `Runner` (`runner.go:39`) wires registry, hydration store, hub, engine and service without HTTP, for the CLI, TUI
  and RPC modes.

### 3.2 One prompt, end to end

```text
caller                    Service                 Hub            Engine.handleStartInference        geppetto
  │ SubmitPromptRequest     │                       │                       │                          │
  ├────────────────────────►│ store pending req     │                       │                          │
  │                         ├── Submit(StartInf) ──►│── handler (inline) ──►│ publish UserMessageAccepted
  │                         │                       │                       │ cancel previous run (!)  │
  │                         │                       │                       │ go runPrompt ───────────►│
  │◄──────── nil ───────────┤◄──────────────────────┤◄──────────────────────┤                          │
  │                                                                         │ load "final" turn        │
  │                                                                         │ append user block        │
  │                                                                         │ toolloop (rounds) ──────►│
  │                                                                         │◄── events via sink ──────┤
  │                                                                         │ publish ChatRun*, text,  │
  │                                                                         │ tool, usage events       │
```

The steps, with references:

1. `Service.SubmitPromptRequest` (`service.go:62`) rejects an empty prompt without attachments, stores the request
   under a fresh request id, and submits `ChatStartInference`.
2. `handleStartInference` (`runtime_inference.go:21`) publishes `ChatUserMessageAccepted`, then **cancels the previous
   run of the session and waits for it** (`:49-52`), then starts `runPrompt` in a goroutine.
3. `runRuntimeInference` (`:74`) publishes `ChatRunStarted`, builds a `gepsession.Session`, loads the latest turn with
   phase `"final"` from the `TurnStore` (`:122`), appends the user block, and runs an `enginebuilder.Builder` with only
   `Base`, `Registry`, `ToolExecutor` and `EventSinks` (`:108-113`). No loop limit, final answer or round guard is
   passed, so geppetto's default of 20 rounds applies.
4. The run sink (`runtime_sink.go`) and plugins translate geppetto events into chatapp backend events:
   `ChatTextSegmentStarted/Patch/Finished`, `ChatReasoning*`, `ChatToolCall*`, `ChatProviderCall*` (with
   `UsageInfo`).
5. The run ends with `ChatRunFinished`, `ChatRunStopped` (cancel) or `ChatRunFailed{error string}`. A max-rounds error
   adds a warning text segment written by chatapp (`:179-181`).

### 3.3 History: one loader, three savers

chatapp loads history (step 3) but does not save it. Saving happens elsewhere:

- web-chat's runtime composer wraps the engine in an `enginebuilder.Builder` with a `Persister` that saves the turn as
  `"final"` after every model call, plus a `SnapshotHook` (`cmd/web-chat/internal/runtime/composer.go:118-123`);
- the CLI has its own turn persister (`pkg/cmds/chat_persistence.go`);
- the TUI keeps an accumulator through `OnFinalTurn` and passes it back as `InitialTurn` (`pkg/ui/chatapp_backend.go:90`).

On cancel, `runRuntimeInference` saves nothing (`runtime_inference.go:161-170`). With web-chat's composer, the last
saved `"final"` turn is the one after the last model call, which can end in a tool call without a result. With the
other paths, the stopped turn, including the person's message, is not in the next turn's history at all.

### 3.4 Browser tools (frontendtools)

Some tools must run in a browser tab, because they act on state that only exists there (a page's DOM, an editor).
`pkg/chatapp/frontendtools` implements this:

- The page sends a **manifest** listing its tools (name, description, JSON schema, mode `auto` or `human`) with an
  executor identity `{clientInstanceId, connectionId}`. `Manager.AcceptManifest` (`manager.go:222`) records it and
  assigns an `assignmentId`. The latest manifest's identity is the session's **executor**.
- `BridgeExecutor` (`bridge.go:64`) is a `ToolExecutor`. For a call to a manifest tool it calls `Manager.Request`
  (`manager.go:402`), which publishes `ChatFrontendToolCallRequested` and blocks until the executor posts a result
  (`ChatFrontendToolResult` command, `HandleResult`, `:298`) or the context ends.
- Results are checked: an unknown call, a result from another executor, a different second result, and so on are
  rejected with stable codes (`InvocationError*`, `manager.go:38-50`). An identical duplicate is accepted.
- `frontendtools.Plugin` projects the calls into `ChatFrontendToolCall` timeline entities, so a reloaded page sees
  pending calls.

The bridge needs the session id and publisher in the context (`WithBridgeContext`, `bridge.go:46`); `PromptRequest.
RuntimeContext` exists for that. In pinocchio today, only agentd actually sets it. web-chat installs the manager and
has manifest and result routes, but no executor is wired, so a model there cannot call browser tools.

Results travel as `google.protobuf.Struct` (`proto/pinocchio/chatapp/frontendtools/v1`), which can only hold a JSON
object. A page that returns a string must wrap it.

### 3.5 web-chat's HTTP server

`cmd/web-chat/internal/appserver` builds the whole stack (schema registry, hydration store, engine, hub with the
websocket fanout, service, export service) and serves:

| Route | Handler |
|---|---|
| `POST /api/chat/sessions` | returns a new UUID; nothing is recorded |
| `GET /api/chat/sessions/{id}` | snapshot |
| `POST /api/chat/sessions/{id}/messages` | resolve a runtime, `SubmitPromptRequest` |
| `GET /api/chat/sessions/{id}/timeline`, `/turns`, `/export` | exports |
| `POST /api/chat/sessions/{id}/tools/manifest`, `/tools/results` | browser tools |
| `/api/chat/ws` | websocket subscribe |

The runtime per message comes from a `RuntimeResolver` (`appserver/runtime.go`):
`Resolve(ctx, req, sessionID, profile, registry) (*ComposedRuntime, error)`. This is where an application decides
model, prompt and tools. Because the package is under `cmd/.../internal`, no other program can import it.

### 3.6 What chatapp lacks

No session list, metadata, owner or delete; no loop limits or final answer; no outcome codes; a busy session is
handled by cancelling the running turn; no tool deadlines; no usage totals or budget; browser tools not wired; no
importable HTTP server; no server-sent events.

## 4. agentd today

### 4.1 What it is and who uses it

agentd (`pkg/agentd`, about 2,100 lines without tests, including its scripted test engine) is an HTTP service: `pinocchio agent serve`
(`cmd/pinocchio/cmds/agent.go`) runs it standalone, and homolog-bench's `hb-server` embeds it through
`agentd.NewServer(opts).Handler()`. It keeps sessions in memory, saves each as a JSON file, and runs one geppetto
session per agentd session.

### 4.2 Its protocol

Endpoints under `/v1`: `health`, `agents`, `models`, `sessions` (list, create, get, delete), `sessions/{id}/turns`,
`events` (server-sent events with resume from a sequence number), `tools/manifest`, `tools/{callId}/result`,
`tools/{callId}/progress`, `cancel`. Events: `user.message`, `run.started`, `text.delta`, `reasoning.delta`,
`tool.requested`, `tool.progress`, `tool.finished`, `usage`, `run.finished`, `run.failed`, `run.cancelled`. Its own
event log (`events.go:37`) gives every event a sequence number and keeps the last 5,000 in memory.

### 4.3 Its features, sorted

Every agentd feature falls into one of three groups. This sorting is the core of the merge.

| Feature | Where in agentd | Group |
|---|---|---|
| session list, labels, title, delete, persistence | `server.go`, `store.go` | mechanism → chatapp |
| one run per session; refuse a second (409) | `session.go:205` | mechanism → chatapp (as a policy option) |
| max rounds; final answer with tools off | `session.go:226-227` | mechanism → chatapp (caller supplies the text) |
| budget checked between rounds | `session.go:368` (`budgetGuard`) | mechanism → `RoundGuard`; toolkit helper |
| outcome decided in one place, error codes | `session.go:339` (`runOutcome`), `:468` (`classify`) | mechanism → chatapp |
| usage and cost per session | `session.go:167`, `adapters.go` | mechanism → sessions + plugin |
| in-process tools (`hostTools`) | `host.go` | mechanism → plain geppetto tools |
| browser tools wired to the loop | `session.go:216, 239, 247` | mechanism → chatapp |
| `{value}` wrap/unwrap of browser results | `server.go:403`, `session.go:555` | transport detail → inside frontendtools |
| HTTP + SSE | `server.go` | mechanism → `chatapp/httpapi` |
| system prompt file; page instructions; `Instructions`/`Context` hooks | `config.go`, `server.go:259-305` | **content → caller** |
| `[context]` block per turn, previous one deleted | `session.go:275-304` | **content → caller** |
| final-round text, refused-tool text, timeout text, error texts | `session.go:61, 383, 500, 471-478` | **content → caller** |
| result cut at 9,000 characters with advice text | `session.go:527-537` | **context policy → toolkit helper** |
| 120 s per tool call; "awaiting a person" pause | `session.go:70-76, 388-410` | **tool's own policy → tool / toolkit** |
| agent YAML presets | `config.go` | app convenience → toolkit helper |

### 4.4 Why content in the core hurts: an example

agentd deletes the previous turn's `[context]` block before appending the next one (`appendTurnLocked`,
`session.go:275`). Consider two turns. Turn 1's last model call sent:

```text
[system][user: add a reset button][context, 10:00][tool_call: read main.ts][tool_use: 9 KB of main.ts]…
```

Turn 2's first call sends:

```text
[system][user: add a reset button][tool_call: read main.ts][tool_use: 9 KB of main.ts]…[user: make it red][context, 10:05]
                                  ↑ first difference
```

Provider prompt caches reuse the longest identical prefix of a previous request. Because a block in the middle
disappeared, everything after `[user: add a reset button]` is processed again at full price, once per turn. In a long
homolog-bench persona run, input tokens were 2.8 M against 84 k output (homolog-bench HB-004 diary, step 11), so the
cache matters.

The library made a content decision (old context is noise) that silently broke a mechanism property (the history is
append-only). Under the rule in this guide, the core never edits blocks it has sent; a caller that wants a fresh
context block appends one, and a caller that wants to shorten history does so explicitly.

## 5. Design principles

**P1. One runtime.** chatapp is the engine. agentd's package and protocol are deleted, not wrapped. No compatibility
layer: homolog-bench adapts (see the playbook in this ticket).

**P2. Mechanism in the core, content with the caller.** *Mechanism* is what makes runs work the same way for every
application: sessions, runs, the loop, executing tools, events, transports, storage, limits the caller configured.
*Content* is anything the model reads: the system prompt, status or context blocks, tool descriptions, the wording of
errors and limits, how results are cut, how history is shortened. Content changes from application to application,
and it decides cost and caching, so the application owns it. The core may ship **helpers** for common content needs,
but never applies them by itself.

Examples:

| Need | Core provides | Caller provides |
|---|---|---|
| a system prompt | middleware passthrough (`ComposedRuntime.Engine` already carries middlewares) | the text, e.g. `NewSystemPromptMiddleware(text)` |
| live state each turn | `PromptRequest.Blocks`: extra blocks appended after the person's message | the block's text, and whether to send one |
| an answer at the round limit | `LoopConfig.FinalAnswer` plumbing | the instruction block |
| a long tool result | nothing by default; helper `toolexec.CutResults` | the limit and the note |
| a slow tool | cancellation on stop; helper `toolexec.WithDeadline` | the duration and the message |

**P3. Append-only history.** The core appends blocks and never edits or deletes blocks already sent. Stopped and
failed turns stay in the history, so the model remembers what it did.

**P4. A toolkit, not a framework.** Optional packages for common needs (deadlines, result cuts, budgets, presets,
later a code-mode kernel and a JS client). An application can use any subset or none.

**P5. Small, testable phases.** Each phase compiles, passes tests and leaves the tree usable.

## 6. Target architecture

```text
pkg/inference/runtime      ComposedRuntime  (+ Loop, ToolConfig, RoundGuard)
        │
pkg/chatapp                Engine, Service, PromptRequest (+ Blocks), plugins, projections
  │                        run lifecycle: busy policy, outcome codes, history persistence, unanswered-call closing
  ├── frontendtools        browser tools: manifest, executor, bridge (wired by chatapp), results of any JSON type
  ├── sessions     (new)   session metadata store: id, owner, labels, title, usage; usage plugin
  ├── httpapi      (new)   HTTP server promoted from cmd/web-chat: sessions, messages, stop, tools, exports,
  │                        websocket and SSE transports, Authenticate/Authorize hooks, RuntimeResolver
  ├── serverkit            stores, JSON helpers (unchanged)
  └── toolkit      (new)   optional helpers
        ├── toolexec       WithDeadline, CutResults
        ├── budget         a RoundGuard that stops a run when the session's tokens are used up
        └── presets        YAML agent presets → model ref, prompt text, loop and tool settings

cmd/web-chat               uses httpapi
cmd/pinocchio agent serve  uses httpapi + presets + modelsource
pkg/agentd                 deleted
```

Who does what for one prompt after the merge:

| Step | Owner |
|---|---|
| authenticate the HTTP request, authorize the session | `httpapi` hooks supplied by the app |
| decide model, system prompt, middlewares, tools, loop limits | the app's `RuntimeResolver` |
| decide extra blocks for this turn (live state) | the app, via `PromptRequest.Blocks` |
| refuse or replace a busy session | chatapp, per the configured policy |
| load history, append the person's message and the caller's blocks | chatapp |
| run the loop, execute tools (in-process or in a browser) | chatapp + geppetto |
| stop between rounds when over budget | the app's `RoundGuard` (toolkit helper available) |
| publish events, update usage, save the final turn | chatapp + sessions plugin |
| deliver events to clients (websocket, SSE) | `httpapi` transports |

What a caller writes, as a sketch of homolog-bench's case. The types (`sessions.Info`, `httpapi.MessageRequest`) are
defined in section 7; helpers such as `withMiddlewares`, `a.room` and `a.evalTool` stand for the application's own code:

```go
// Sketch: an app's runtime resolver. Everything the model reads is decided here.
func (a *App) Resolve(ctx context.Context, r *http.Request, s sessions.Info, req httpapi.MessageRequest) (*infruntime.ComposedRuntime, error) {
    eng, model, err := a.models.Resolve(ctx, a.preset.Model, modelsource.Options{Thinking: a.preset.Thinking})
    if err != nil { return nil, err }
    system := a.preset.SystemPrompt + "\n\n" + a.room(s).Instructions()   // the app's content
    reg := geptools.NewInMemoryToolRegistry()
    _ = reg.RegisterTool("eval", a.evalTool(s))                            // an in-process tool
    return &infruntime.ComposedRuntime{
        Engine:       withMiddlewares(eng, gepmiddleware.NewSystemPromptMiddleware(system)),
        Registry:     reg,
        ToolExecutor: toolexec.CutResults(geptools.NewDefaultToolExecutor(...), 9000, a.cutNote),
        Loop:         &toolloop.LoopConfig{MaxIterations: 40, FinalAnswer: &toolloop.FinalAnswer{Prompt: a.finalBlock}},
        RoundGuard:   budget.SessionTokens(a.sessions, s.ID, 2_000_000),
    }, nil
}

// Per message, the live state as an appended block, through httpapi's Blocks hook (7.6):
func (a *App) Blocks(ctx context.Context, s sessions.Info, req httpapi.MessageRequest) ([]turns.Block, error) {
    return []turns.Block{turns.NewUserTextBlock("[context]\n" + a.room(s).Context())}, nil
}
```

## 7. Detailed design

### 7.1 Runtime composition

**Proposed.** `ComposedRuntime` gains three optional fields, passed straight into the `enginebuilder.Builder` that
chatapp builds for a run:

```go
type ComposedRuntime struct {
    Engine       engine.Engine
    Registry     geptools.ToolRegistry
    ToolExecutor geptools.ToolExecutor
    WrapSink     EventSinkWrapper
    RuntimeFingerprint, RuntimeKey string

    Loop       *toolloop.LoopConfig   // nil: geppetto's default (20 rounds, no final answer)
    ToolConfig *geptools.ToolConfig   // nil: geppetto's default
    RoundGuard toolloop.RoundGuard    // nil: none
}
```

The defaults stay geppetto's. chatapp adds no limit of its own. An app that wants agentd's behavior sets
`MaxIterations: 40` and a `FinalAnswer` with its own text.

### 7.2 Runs: lifecycle, busy sessions and outcomes

A run moves through these states; each arrow publishes the named event.

```text
                SubmitPromptRequest
                        │
          session busy? ├── yes, policy Reject  → error ErrRunActive (nothing published)
                        ├── yes, policy Replace → cancel the old run, wait for it to end
                        ▼
               ChatUserMessageAccepted → ChatRunStarted
                        │
         ┌──────────────┼───────────────────────────┬──────────────────────────────┐
         ▼              ▼                           ▼                              ▼
  ChatRunFinished   ChatRunFinished           ChatRunStopped                 ChatRunFailed{code}
  (answered)        {truncated, reason:       (Stop, or replaced)            (provider error, guard error,
                     "max_rounds"}                                             limit without final answer)
                    (final answer given)
```

**Busy sessions (proposed).** `WithBusyPolicy(BusyReject | BusyReplace)`. With `BusyReject` (the default), the start
handler returns `ErrRunActive` before publishing anything, and because `Hub.Submit` is synchronous the error reaches
`Service.SubmitPromptRequest`'s caller; `httpapi` maps it to `409 run_active`. With `BusyReplace`, today's behavior:
the old run is cancelled and awaited. The CLI/TUI can choose either.

**Outcome codes (proposed).** `ChatRunFailed` gains `code`; `ChatRunFinished` gains `truncated` and `reason`. One
function decides the outcome from the loop's error, as agentd's `runOutcome` does:

| Loop result | Event | Code / reason |
|---|---|---|
| no error | `ChatRunFinished` | — |
| context cancelled (Stop, Replace) | `ChatRunStopped` | — |
| `MaxIterationsError{Answered: true}` | `ChatRunFinished` | `truncated: true`, `reason: "max_rounds"` |
| `MaxIterationsError{Answered: false}` | `ChatRunFailed` | `max_rounds` |
| error with a `Code() string` method (e.g. a budget guard's error) | `ChatRunFailed` | that code |
| anything else | `ChatRunFailed` | from the classifier: `authentication_error`, `rate_limited`, `prompt_too_large`, `upstream_error` |

The classifier is an option (`WithErrorClassifier`). The default matches error text, as agentd's does, because
geppetto's OpenAI and Responses engines return untyped errors; it becomes `errors.As` once geppetto has typed errors.
The `error` text stays the raw error by default; an app exposing events to untrusted clients sets
`WithErrorText(func(code string, err error) string)` to replace it. chatapp's own max-rounds warning segment is removed.

### 7.3 History

**Proposed.** chatapp owns saving the conversation, because it owns loading it. When the engine has a `TurnStore`, at
the end of **every** run (finished, failed or stopped) it saves the session's latest turn with phase `"final"`. Before
saving, it closes unanswered tool calls: for each `tool_call` block with no `tool_use` of the same id, it appends a
`tool_use` block with an error, so the next request is valid.

```text
stopped during a tool call               saved as "final"
[user: run the tests]                    [user: run the tests]
[tool_call: run_tests, id c7]            [tool_call: run_tests, id c7]
                                         [tool_use: id c7, error: "not run: the turn ended first"]
```

The error string is the only text the core writes into the history, and it is an option (`WithUnansweredToolResult`)
so an application can word it. web-chat's composer stops attaching a `Persister` (its `SnapshotHook` for debugging
snapshots stays), and the CLI persister is reviewed in the same phase so a turn is not saved twice.

**Caller blocks (proposed).** `PromptRequest.Blocks []turns.Block` are appended after the person's message, in order,
for this turn only. They are never removed later (P3). This is how an application adds live state; how often and how
big is its decision.

**Content removed from chatapp:** the `"Explain evtstream"` prompt substituted for an empty start command
(`runtime_inference.go:37`) becomes a rejected command; the demo inference used when no runtime is given (`:67-71`)
becomes explicit (a test helper or an option), never a silent fallback.

### 7.4 Sessions

**Proposed.** `pkg/chatapp/sessions` keeps what agentd's session list offered:

```go
type Info struct {
    ID      string
    Owner   string            // set by httpapi from the authenticated principal; "" when anonymous
    Labels  map[string]string // free-form metadata, e.g. {"room": "main"}; filterable
    Title   string            // set by the caller (helper: first line of the first message)
    Created, Updated time.Time
    Usage   Usage             // summed over every provider call
}
type Usage struct{ Input, Output, Cached, CacheCreation, CacheRead, Calls int }

type Filter struct{ Owner string; Labels map[string]string; Limit int }

type Store interface {
    Create(ctx context.Context, info Info) error
    Get(ctx context.Context, id string) (Info, error)          // ErrNotFound
    List(ctx context.Context, f Filter) ([]Info, error)         // newest first
    Update(ctx context.Context, id string, fn func(*Info) error) error
    Delete(ctx context.Context, id string) error
}
func NewMemoryStore() Store
func NewUsagePlugin(store Store) chatapp.ChatPlugin   // adds each ChatProviderCallFinished's usage to Info.Usage
```

The conversation itself stays in the `TurnStore` and the timeline in the hydration store; `sessions` holds only
metadata. Deleting a session removes its metadata; `TurnStore` and `HydrationStore` have no delete today, so their rows
remain until those interfaces gain one (noted in section 12).

### 7.5 Tools

- **In-process tools** are ordinary geppetto tools in `ComposedRuntime.Registry`. geppetto's base executor already
  publishes their execution events (`pkg/inference/tools/base_executor.go:112`), which chatapp's tool-call plugin
  projects. agentd's `hostExecutor` and `Tool` type are not carried over.
- **Browser tools** are wired by chatapp itself when the engine is given a `frontendtools.Manager`: chatapp registers
  the session's manifest tools into the run's registry, wraps the executor in `BridgeExecutor`, and sets the bridge
  context. Applications no longer write the `RuntimeContext` glue that only agentd had.
- **Results of any JSON type (proposed).** The result fields in the frontendtools protos change from
  `google.protobuf.Struct` to `google.protobuf.Value`, so a page can return a string, number or array and the model
  receives exactly that. This replaces agentd's `{value: x}` wrapping. (Fallback if the proto change is too costly:
  wrap and unwrap inside frontendtools, invisible to callers.)
- **Approval** is the existing `human` mode of a browser tool: the page shows Run/Decline and posts `denied`.
- **Timing belongs to the tool.** The core cancels a tool call's context on Stop and does nothing else. agentd's
  "awaiting a person" mechanism existed only because agentd imposed a 120 s limit that homolog-bench's own room timer
  had to pause; without the outer limit it is unnecessary. `toolexec.WithDeadline` is available to apps that want one.
- **Future: tab liveness.** A browser-tool call whose tab closed currently waits until its context ends. The bridge
  should fail such a call when the executor's connection has been gone for N seconds. This needs the transports to
  report disconnects to the manager; it is not part of the first phases.

### 7.6 HTTP and transports

**Proposed.** `cmd/web-chat/internal/appserver` moves to `pkg/chatapp/httpapi` and gains what agentd had. The server
builds the stack (registry, stores, engine, hub, service, transports) from options:

```go
type Options struct {
    Prefix        string                         // e.g. "/api/chat"
    Runtime       RuntimeResolver                // required: model, prompt, tools, limits per message
    Sessions      sessions.Store                 // default: memory
    Hydration     serverkit.StoreSpec            // default: memory
    Turns         chatstore.TurnStore            // optional
    Plugins       []chatapp.ChatPlugin
    BrowserTools  *frontendtools.Manager         // optional
    BusyPolicy    chatapp.BusyPolicy
    Authenticate  func(*http.Request) (Principal, error)                        // optional; nil = anonymous
    Authorize     func(ctx context.Context, p Principal, a Action, s *sessions.Info) error // optional; nil = allow
    Blocks        func(ctx context.Context, s sessions.Info, req MessageRequest) ([]turns.Block, error) // optional
}
type RuntimeResolver interface {
    Resolve(ctx context.Context, r *http.Request, s sessions.Info, req MessageRequest) (*infruntime.ComposedRuntime, error)
}
```

| Method and path (under `Prefix`) | Body → answer |
|---|---|
| `POST /sessions` | `{labels?, title?}` → `201 Info` |
| `GET /sessions?label=k:v` | → `[Info]` (owner-filtered when authenticated) |
| `GET /sessions/{id}` | → `{session: Info, snapshot: SessionSnapshotResponse}` |
| `DELETE /sessions/{id}` | stops a running turn, deletes the metadata → `204` |
| `POST /sessions/{id}/messages` | `{prompt, attachments?, idempotencyKey?, profile?, registry?}` → `202`; `409 run_active` |
| `POST /sessions/{id}/stop` | → `202` |
| `GET /sessions/{id}/events` | SSE: one `snapshot` event, then `ui` events |
| `GET /ws` | websocket (sessionstream protocol) |
| `POST /sessions/{id}/tools/manifest`, `/tools/results` | browser tools (as today) |
| `GET /sessions/{id}/timeline`, `/turns`, `/export` | exports (as today) |

**SSE transport (proposed).** The same algorithm as the websocket server's hydration, over one HTTP response:

```text
on GET /sessions/{id}/events:
    sub = register subscriber for sid in state "hydrating"   // PublishUI buffers batches for it
    snap = hub.Snapshot(sid)
    write  event: snapshot, data: {snapshotOrdinal, entities}
    for batch in sub.buffer where batch.ordinal > snap.SnapshotOrdinal:   write event: ui
    mark sub "live"                                           // PublishUI now writes directly (bounded queue)
    every 15 s write ": ping"
    on queue overflow: close the stream; the client reconnects and hydrates again
```

The fanout given to the hub is a small multiplexer over the websocket server and the SSE subscribers. A reconnecting
client hydrates again; there is no `Last-Event-ID` replay (decision D5).

**Auth.** `Authenticate` turns a request into a `Principal{ID string; Attrs map[string]string}`; `httpapi` stores
`Owner = Principal.ID` on create and calls `Authorize` before every session action (`read`, `send`, `stop`, `delete`,
`tools`). With no hooks, everything is allowed, as today for a local server.

### 7.7 The toolkit

All optional. Each helper is a small function over existing interfaces.

- `toolexec.WithDeadline(next geptools.ToolExecutor, d time.Duration, message func(call geptools.ToolCall, ran time.Duration) string) geptools.ToolExecutor`
  — cancels the call after `d` and returns `message(...)` as the tool error.
- `toolexec.CutResults(next geptools.ToolExecutor, max int, note func(shown, total int) string) geptools.ToolExecutor`
  — cuts a result's text to `max` characters and appends `note`.
- `budget.SessionTokens(store sessions.Store, sessionID string, limit int) toolloop.RoundGuard` — returns an error with
  `Code() == "quota_exceeded"` once `Input+Output >= limit`, so the run fails with that code before the next call.
- `presets.Load(dir) (map[string]*Preset, error)` — agentd's YAML format (name, description, model, thinking,
  maxTokens, systemPrompt file, maxRounds, resultCut, tools), minus the fields that became tool or caller policy. A
  preset is data the caller uses to build its runtime; chatapp never reads presets.

Later, outside this ticket's first phases: a code-mode kernel (one `eval` tool over a JavaScript API, with browser and
goja hosts), a JS client library (sessions, hydrate-then-stream, browser-tool registration, a timeline model), a
prompt-prefix checker for caching, opt-in compaction.

### 7.8 What gets deleted

- `pkg/agentd/` entirely: `server.go`, `session.go`, `events.go`, `store.go`, `host.go`, `adapters.go`, `config.go`,
  `fake.go` (its scripted engine moves to a test helper if still needed), `agentd_test.go`; `pkg/agentd/agentcli`
  moves to wherever `pinocchio agent serve` keeps its flag sections.
- From chatapp: the `"Explain evtstream"` default, the silent demo fallback, the max-rounds warning text.
- `cmd/web-chat/internal/appserver` (replaced by `pkg/chatapp/httpapi`).

## 8. API reference (proposed)

### 8.1 Go

```go
// pkg/chatapp
type BusyPolicy int
const ( BusyReject BusyPolicy = iota; BusyReplace )
var ErrRunActive = errors.New("chatapp: a run is already active in this session")
func WithBusyPolicy(p BusyPolicy) Option
func WithErrorClassifier(fn func(error) string) Option
func WithErrorText(fn func(code string, err error) string) Option
func WithUnansweredToolResult(text string) Option
func WithBrowserTools(m *frontendtools.Manager) Option   // name may change; see 7.5

type PromptRequest struct {
    Prompt         string
    Attachments    []Attachment
    IdempotencyKey string
    Runtime        *infruntime.ComposedRuntime
    Blocks         []turns.Block       // new
    InitialTurn    *turns.Turn
    OnFinalTurn    func(*turns.Turn)
    RuntimeContext func(ctx context.Context, sid sessionstream.SessionId, messageID string, pub sessionstream.EventPublisher) context.Context
}

// pkg/inference/runtime: ComposedRuntime + Loop, ToolConfig, RoundGuard (7.1)
// pkg/chatapp/sessions: Info, Usage, Filter, Store, NewMemoryStore, NewUsagePlugin (7.4)
// pkg/chatapp/httpapi: Options, RuntimeResolver, Principal, Action, New(Options) (*Server, error), (*Server).Handler() (7.6)
// pkg/chatapp/toolkit/{toolexec,budget,presets} (7.7)
```

### 8.2 Events an agent client uses

These are chatapp's backend event names (`chat.go:18-41`) and the browser-tool events (`frontendtools/manager.go:19-26`).
Over the websocket and SSE they arrive as UI events with protobuf-JSON payloads.

| Event | Meaning |
|---|---|
| `ChatUserMessageAccepted` | the person's message is in the session |
| `ChatRunStarted` | a run began (`message_id` identifies it) |
| `ChatTextSegmentStarted`, `ChatTextDelta`, `ChatTextSegmentFinished` | the answer text, streamed |
| `ChatReasoningSegmentStarted`, `ChatReasoningDelta`, `ChatReasoningSegmentFinished` | thinking, streamed |
| `ChatToolCallStarted`, `ChatToolArgumentsDelta`, `ChatToolCallRequested`, `ChatToolExecutionStarted`, `ChatToolResultReady`, `ChatToolCallFinished` | a tool call's life |
| `ChatFrontendToolCallRequested`, `ChatFrontendToolResultReceived` | a browser tool call for the executor tab |
| `ChatProviderCallFinished` | one model call ended; `usage` |
| `ChatRunFinished{truncated, reason}`, `ChatRunStopped`, `ChatRunFailed{code, error}` | the run ended |

## 9. Every agentd feature and its new home

| agentd | After the merge |
|---|---|
| `Server`, `Options.Agents`, `/v1/agents` | app code; `presets` helper |
| `/v1/models` | app route using `modelsource` (`pinocchio agent serve` keeps one) |
| sessions list/create/get/delete, labels, title | `sessions` + `httpapi` |
| `POST /turns` | `POST /sessions/{id}/messages` |
| `409 run_active` | `BusyReject` + `httpapi` |
| `maxRounds`, final answer | `ComposedRuntime.Loop` (app supplies the block) |
| `budgetGuard`, `ErrQuota` | `ComposedRuntime.RoundGuard`; `budget.SessionTokens` |
| `runOutcome`, `classify` | chatapp outcome function, `WithErrorClassifier`, `WithErrorText` |
| `usage` events, cost | `ChatProviderCallFinished`; `sessions.NewUsagePlugin`; cost: open (12, D8) |
| `hostTools`, `Tool`, `hostExecutor` | geppetto tools in `ComposedRuntime.Registry` |
| browser tools via `frontendtools` | chatapp wires them; results as `Value` |
| `boundedExecutor` deadline | `toolexec.WithDeadline` (optional) |
| `resultCut` | `toolexec.CutResults` (optional) |
| awaiting a person, `tool.progress`, `personTimeout` | removed; the tool owns its timing |
| `[context]` blocks, page instructions, `Instructions`/`Context` hooks | removed; `PromptRequest.Blocks`, app's system prompt |
| SSE with `Last-Event-ID` replay, `EventLog` | SSE transport: snapshot then live (D5) |
| JSON-file `Store` | `TurnStore` + hydration store + `sessions.Store` |
| `FakeEngine` (`/tool name {json}`) | test helper |

## 10. Implementation plan

Each phase is one or more commits on `task/merge-agentd-into-chatapp`, ends with `go build ./...` and the affected
tests passing, and is recorded in the diary.

| Phase | Work | Main files | Tests |
|---|---|---|---|
| 1. Core runs | `ComposedRuntime.Loop/ToolConfig/RoundGuard` passthrough; busy policy; outcome codes in proto; error classifier/text options; `PromptRequest.Blocks`; chatapp saves the final turn on every outcome with unanswered calls closed; remove chatapp's content defaults | `pkg/inference/runtime/composer.go`, `pkg/chatapp/{chat,service,runtime_inference}.go`, `proto/.../chat.proto`, web-chat composer | busy reject/replace; final answer; limit without answer; coded guard error; stopped turn kept and closed; blocks appended |
| 2. Sessions | `sessions` package (memory store), usage plugin | `pkg/chatapp/sessions/*` | store CRUD/filter; usage summed |
| 3. HTTP | `httpapi` from web-chat's appserver; session routes; stop; SSE transport; auth hooks; web-chat switched | `pkg/chatapp/httpapi/*`, `cmd/web-chat/*` | handler tests with a fake engine: create → message → events → finished; 409; SSE hydration has no gap; auth denies |
| 4. Tools and toolkit | chatapp wires browser tools; result `Value`; `toolexec`, `budget`, `presets` | `pkg/chatapp/frontendtools/*`, `proto/.../frontendtools/v1/*.proto`, `pkg/chatapp/toolkit/*` | browser round trip through HTTP; deadline; cut; budget stops between rounds |
| 5. agent serve, delete agentd | `pinocchio agent serve` on `httpapi` + presets + modelsource; delete `pkg/agentd`; docs | `cmd/pinocchio/cmds/agent.go`, `pkg/agentd` (deleted), `pkg/doc` | command smoke test with a fake engine |
| 6. homolog-bench | hb-server and the page move to chatapp | homolog-bench (separate repo) | by the homolog-bench developer, following the playbook |

The phases are ordered so that each one is useful alone. Phase 1 already gives the CLI and TUI an answer at the round
limit instead of a failure, and fixes the lost stopped turn.

## 11. Testing strategy

- **Unit tests in chatapp** with a scripted engine (an `engine.Engine` that returns preset blocks per call, including a
  tool call, and observes cancellation): one per row of the outcome table in 7.2, the busy policies, unanswered-call
  closing, caller blocks, history across two turns.
- **Prefix stability:** a test that records the turn sent on every model call across two turns and checks that each
  call's blocks start with the previous call's blocks. This is P3 as an executable check and guards against the
  section 4.4 regression.
- **httpapi tests** with `httptest.Server`: the HTTP flows in phase 3, including a browser tool posted by the test acting
  as the page, SSE snapshot-then-live ordering under concurrent publishing, and `Authorize` denials.
- **Ported agentd scenarios:** echo turn and history; tool round trip through the browser; tool error; cancel during a
  tool; reconnect and resume (now: hydrate again); manifest human mode and rejections; max rounds with an answer;
  budget between rounds; failed final answer. Scenarios that tested removed content (context-block replacement,
  awaiting-person, result cut text) are dropped or move to toolkit tests.
- `make lint` and `go test ./...` at the end of each phase.

## 12. Decisions, risks and open questions

| # | Decision | Context and alternatives | Status |
|---|---|---|---|
| D1 | chatapp is the only runtime; agentd is deleted | keep both (duplication), or move chatapp onto agentd (agentd lacks plugins, stores, CLI/TUI) | decided by the user |
| D2 | Mechanism in the core, content with the caller | agentd's hooks and defaults put homolog-bench's prompt design into a shared library | decided by the user |
| D3 | Append-only history; stopped and failed turns kept, unanswered calls closed | today stopped turns vanish (chatapp) or are kept with a dangling call (web-chat composer) | proposed; the closing text is an option |
| D4 | Busy policy is an option, default reject | today chatapp replaces silently; agentd rejects | proposed |
| D5 | Clients reconnect by hydrating again (snapshot, then live); no sequence replay | agentd replayed from a sequence number; sessionstream's model is snapshot-based and already gap-free | proposed |
| D6 | Agent presets are a toolkit helper, not part of engine profiles | earlier idea: extend `ProfileRuntime`; but prompts are caller content (D2) | proposed |
| D7 | Error codes in the core; error text replaceable | agentd hid provider text; chatapp shows it | proposed |
| D8 | Cost is computed in one place | geppetto `ModelInfo.ComputeCost` and agentd `costUSD` treat cached tokens differently | open; agentd's formula is dropped with agentd, the usage plugin records tokens only until this is settled |
| D9 | homolog-bench adapts to chatapp's protocol; no compatibility shim | a shim would keep agentd's event names alive | decided by the user |

Risks:

- **Proto changes** (`ChatRun*` fields, frontendtools `Value`) require regenerating Go and web-chat's TypeScript code
  (`make proto-gen-core`, which uses buf remote plugins and biome).
- **Changing the busy default** changes behavior for any CLI path that relied on replacement; phase 1 audits the
  callers (`pkg/cmds/cmd.go`, `pkg/ui/chatapp_backend.go`).
- **Session delete** leaves turns and timeline rows behind until the stores gain delete operations.
- **Browser-tool calls of a closed tab** still wait until stopped (tab liveness is future work).

## 13. File reference

Current code (pinocchio `22a0e56` unless noted):

- `pkg/chatapp/chat.go` — engine, options, command and event names, `Install`.
- `pkg/chatapp/runtime_inference.go` — the run: start handler, history loading, builder, outcomes.
- `pkg/chatapp/service.go` — `Service`, `PromptRequest`.
- `pkg/chatapp/runtime_sink.go`, `plugins/{toolcall,reasoning}.go` — geppetto events → chatapp events.
- `pkg/chatapp/features.go` — `ChatPlugin`.
- `pkg/chatapp/runner.go` — non-web wiring.
- `pkg/chatapp/frontendtools/{manager,bridge,plugin}.go` — browser tools.
- `pkg/chatapp/serverkit/{contracts,stores,http,turn_persistence}.go` — HTTP contracts, stores.
- `pkg/inference/runtime/composer.go` — `ComposedRuntime`.
- `cmd/web-chat/internal/appserver/*`, `cmd/web-chat/internal/runtime/composer.go` — web-chat server and runtime.
- `pkg/agentd/*`, `cmd/pinocchio/cmds/agent.go` — agentd and `pinocchio agent serve`.
- `proto/pinocchio/chatapp/v1/chat.proto`, `proto/pinocchio/chatapp/frontendtools/v1/*.proto` — wire types.
- geppetto `pkg/inference/toolloop/{config,limits,loop}.go`, `enginebuilder/builder.go`, `pkg/inference/tools/*`,
  `pkg/events/*` — loop, tools, events.
- sessionstream `pkg/sessionstream/{hub,hydration,projection,types}.go`, `transport/ws/server.go` — hub and transport.

Start here: `go test ./pkg/chatapp/...` in the pinocchio worktree, then read `runtime_inference.go` top to bottom.
