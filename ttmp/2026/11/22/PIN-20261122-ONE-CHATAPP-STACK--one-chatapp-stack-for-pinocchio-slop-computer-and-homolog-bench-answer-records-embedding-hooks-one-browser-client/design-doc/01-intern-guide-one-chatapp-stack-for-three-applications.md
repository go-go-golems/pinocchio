---
Title: 'Intern guide: one chatapp stack for three applications'
Ticket: PIN-20261122-ONE-CHATAPP-STACK
Status: active
Topics:
    - chatapp
    - chat
    - backend
    - frontend
    - refactor
    - design
DocType: design-doc
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: pkg/chatapp/projections.go
      Note: How backend events become timeline entities (ChatRun, ChatMessage); the client must agree with it
    - Path: pkg/chatapp/runtime_sink.go
      Note: Where a run's geppetto events become chatapp events (usage, outcomes)
    - Path: pkg/chatapp/httpapi/server.go
      Note: The HTTP API's options and route table
    - Path: pkg/chatapp/httpapi/sse.go
      Note: The event stream (snapshot, then live UI events, no gap)
    - Path: pkg/chatapp/agentserver/agentserver.go
      Note: The preset-based application on httpapi (agent serve; homolog-bench after migration)
    - Path: proto/pinocchio/chatapp/v1/chat.proto
      Note: Events and entities on the wire
ExternalSources: []
Summary: "How pinocchio's chatapp, slop-computer and homolog-bench fit together today, why the same work is done four times, and the plan to give all three one server stack and one browser client: answers stored as records, embedding hooks, and a framework-free client served by the server it belongs to."
LastUpdated: 2026-11-22T20:30:00-05:00
WhatFor: "Onboarding an engineer who will implement or review the unification across pinocchio, slop-computer and homolog-bench."
WhenToUse: "Before changing chatapp's events or entities, its HTTP API, the browser client, or an application's agent integration."
---

# Intern guide: one chatapp stack for three applications

## 0. What this document is

This guide is for an engineer who knows Go and JavaScript but has not worked on pinocchio, slop-computer or
homolog-bench. After reading it you should be able to:

- explain what each of the three applications does with a language model, and which code does it;
- follow one message from a browser to the model and back, naming every event and stored record on the way;
- explain why the same logic exists four times today, and which bugs that has caused;
- implement the plan in section 8, or review someone else's implementation of it.

The code was read at these revisions:

| Repository | Path on the development machine | Revision |
|---|---|---|
| pinocchio | `~/code/wesen/go-go-golems/pinocchio` | `b271952f` (main, release v0.11.19) |
| slop-computer | `~/code/wesen/go-go-golems/slop-computer` | `707cf6f` (origin/main) |
| homolog-bench | `~/code/wesen/2026-11-19--homolog-workbench` | `5dcc441` (main) |
| react-chat | `~/code/wesen/go-go-golems/react-chat` | `09597c5` (chat-provider v0.6.0) |
| pinocchio-agentd | `~/code/wesen/go-go-golems/pinocchio-agentd` | `2bf4f3e6` (the agentd that homolog-bench still embeds) |

Paths without a repository prefix are in pinocchio. The companion document for chatapp's internals is pinocchio's
`ttmp/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP--…/design-doc/01-intern-guide-merging-agentd-into-chatapp-…md`;
this guide repeats only what it needs from it.

## 1. The three applications

All three applications use the same kind of agent. A person writes a message. A server sends it to a language model
together with the conversation so far, lets the model call tools, runs those tools, gives the results back to the
model, and repeats until the model answers in plain text. The browser shows the answer while it streams: the model's
thinking, its text, and each tool call with its result.

- **pinocchio** is a command-line program and Go library for running language models. Its chat engine, *chatapp*
  (`pkg/chatapp`), is the subject of this guide. pinocchio also ships `pinocchio agent serve`, a small HTTP server
  built on chatapp, and an example web application, `cmd/web-chat`, whose browser part comes from the react-chat
  repository.
- **slop-computer** is a platform that runs small web applications ("apps") for signed-in people. An app's page can
  open an agent session through slop's *agent capability*: the page registers tools that run in the page, sends
  messages and receives answers. slop's server meters every model call against the person's LLM budget.
- **homolog-bench** is a scientific workbench for deciding whether genes are absent in species. Its agent works in a
  *room*: a JavaScript environment on the server that holds the workbench API and the person's workspaces. The agent's
  only tool, `eval`, runs JavaScript in the room. The person watches the conversation in the workbench's Agent panel
  and may have to approve tool work that calls real bioinformatics tools.

One situation recurs through this guide. In homolog-bench a person asks "how is the workspace?". The model thinks
briefly, calls `eval` with `ws = await bench.workspace('iqgap-stage0'); ws.status()`, reads the result, thinks again,
and answers "There are 13 indeterminate calls." The Agent panel shows a thinking block, a tool card with the code and
its result rendered as YAML, the final text, and a footer: "4.2 s · 2.7k in / 140 out · 2 model calls".

## 2. Concepts you need first

### 2.1 Sessions, runs and rounds

- A **session** is one conversation. It has an id, labels such as `preset: homolog-bench` and `room: main`, and a
  stored history. chatapp keeps the session list in `pkg/chatapp/sessions` (`Info{ID, Owner, Labels, Title, Created,
  Updated, Usage, Data}`, `sessions/sessions.go`).
- A **run** is everything the server does for one message: from accepting it until the model's final answer, a
  failure or a stop. chatapp identifies a run by a *message id* such as `chat-msg-93fa4b6b-…`; the person's message
  gets the id `<run id>-user`. A session has at most one run at a time. A second message while one runs is refused
  with `409 run_active`.
- A **round** is one model call inside a run. A run with one tool call has two rounds: the call that asks for the
  tool, and the call that reads its result and answers.

The footer of 1 ("4.2 s · 2.7k in / 140 out · 2 model calls") is a statement about one run. Section 5 shows that no
application can show it reliably today.

### 2.2 Tools that run in the server, and tools that run in the browser

A tool is a function the model may call by name with JSON input. chatapp runs tools in two places:

- **In-process tools** run in the server, inside the run, through a geppetto `ToolRegistry`. homolog-bench's `eval` is
  one: it needs the room, and the room lives in the server.
- **Browser tools** (`pkg/chatapp/frontendtools`) run in a page. The page posts a *manifest* (tool names, JSON
  schemas, mode `frontend` or `human`) and becomes the session's *executor*. When the model calls a browser tool, the
  server publishes `ChatFrontendToolCallRequested` naming the executor, the page runs the tool, and it posts the result.
  slop's apps use only browser tools.

Every tool call has a time limit. A tool that waits for a person, for example an approval, must not be stopped by the
ordinary limit of two minutes. For browser tools, mode `human` gets a longer limit (`personTimeout`). For in-process
tools chatapp has no such distinction yet (section 5.4).

### 2.3 sessionstream: events, ordinals, entities and snapshots

chatapp is built on *sessionstream* (`github.com/go-go-golems/sessionstream`), a library for per-session event
streams. Four of its ideas carry this guide:

- A **backend event** is a fact that happened: a name, a protobuf payload, and an **ordinal**, a number that grows by
  one with each event of a session. The events of one session are totally ordered by ordinal.
- A **timeline entity** is a piece of current state: one chat message, one run, one tool call. It has a kind
  (`ChatMessage`), an id, the ordinal of the event that created it (`CreatedOrdinal`), the ordinal of the last event
  that changed it (`LastEventOrdinal`), and a payload (`sessionstream/projection.go:16`).
- A **projection** maps an event to changes of entities. chatapp's `baseTimelineProjection`
  (`pkg/chatapp/projections.go`) turns a `ChatTextPatch` into "append this text to the `ChatMessage` entity with this
  id". Plugins add their own projections, for thinking, tool calls and browser tools.
- A **snapshot** is all entities of a session as of one ordinal (`Snapshot{SessionId, SnapshotOrdinal, Entities}`). A
  newly connected client starts from it.

Projections run in the server. Each event also produces **UI events** that connected clients receive live. A client
therefore has two inputs: one snapshot, then a stream of UI events after it. To show the conversation, it applies the
UI events to its copy of the snapshot, the same way the server's projections apply backend events to the entities.
Keep this pair in mind, the server's projection and the client's copy: most of section 5 is about it.

### 2.4 Presets and runtimes

For each message the server decides which model, system prompt, tools and limits the run uses. chatapp calls this the
message's **runtime** (`ComposedRuntime`, `pkg/inference/runtime/composer.go`) and asks the application for it through
a **runtime resolver** (`httpapi.RuntimeResolver`). A **preset** (`pkg/chatapp/toolkit/presets`) is a YAML file with a
name, system prompt, model, round limit, tool timeout and result cut. `agentserver` is the resolver that builds a
runtime from a preset. homolog-bench's three agents (`homolog-bench`, `persona`, `judge`) are presets in that
repository's `configs/agent/*.yaml`.

## 3. The chatapp stack today

### 3.1 Layers

```text
 ┌──────────────────────────────── applications ────────────────────────────────┐
 │ agent serve (cmd/pinocchio)     slop's agent capability       (homolog: agentd)│
 └──────────────┬──────────────────────────────┬─────────────────────────────────┘
                │ agentserver.New(presets)     │ httpapi.New(own options)
                ▼                              ▼
 ┌─ pkg/chatapp/agentserver ──┐   ┌─ pkg/chatapp/httpapi ──────────────────────────┐
 │ preset → runtime resolver  │──►│ routes, session store, SSE, auth hooks,        │
 │ create hook, context block │   │ browser-tool routes, exports                   │
 └────────────────────────────┘   └──────────────────┬─────────────────────────────┘
                                                     ▼
                 ┌─ pkg/chatapp (Engine, Service, plugins, projections) ─┐
                 │ runs, busy policy, outcome codes, round guard, tools  │
                 └───────────────┬────────────────────────┬──────────────┘
                                 ▼                        ▼
                     sessionstream hub and stores   geppetto tool loop and engines
```

- `pkg/chatapp` owns runs. It starts and stops them, applies the busy policy, classifies failures into codes
  (`max_rounds`, `rate_limited`, `upstream_error`, …), runs geppetto's tool loop and publishes events. It never writes
  text into the model's context on its own; that is the application's job (section 5 of the companion guide explains
  why).
- `pkg/chatapp/httpapi` is an importable HTTP server around chatapp (`httpapi.New(opts ...Option)`). It keeps the
  session list, serves the event stream and calls the application's hooks: authentication, authorization, session
  creation, the runtime resolver and extra blocks per message.
- `pkg/chatapp/agentserver` is one application of httpapi: presets, model selection, page instructions, a message's
  `context` field, tool timeouts and result cuts. `pinocchio agent serve` is agentserver behind a command line.

### 3.2 The HTTP API

All routes sit under a prefix the application chooses (`/v1` for agent serve, `/_rt/agent` for slop). From
`httpapi/server.go:322-334`:

| Route | What it does |
|---|---|
| `POST /sessions {labels, data, id?}` | creates a session; `201 {sessionId, session}` |
| `GET /sessions?label=k:v` | lists the sessions that have those labels |
| `GET /sessions/{id}` | `{sessionId, session, running, snapshot}` |
| `DELETE /sessions/{id}` | deletes the session |
| `POST /sessions/{id}/messages {prompt, …}` | starts a run; `202 {messageId}` (the run's id) or `409 run_active` |
| `POST /sessions/{id}/stop` | stops the running run |
| `GET /sessions/{id}/events` | server-sent events: one snapshot, then UI events |
| `POST /sessions/{id}/tools/manifest` | a page registers its browser tools |
| `POST /sessions/{id}/tools/results` | a page answers a browser tool call |
| `GET /sessions/{id}/timeline`, `/turns`, `/export` | exports |
| `GET /ws` | the same stream over a websocket (react-chat uses it) |

Errors are `{"error": "<text>", "code": "<code>"}`.

### 3.3 The event stream, captured

This is the stream of one real run, captured from `pinocchio agent serve --engine fake` at `b271952f`. The message
`/tool eval {"code":"6*7"}` makes the scripted engine call a browser tool named `eval`; the test answered `42`. The
whole capture is in this ticket's `sources/01-agent-serve-sse-capture-b271952f.txt`; ids are shortened here.

```text
event: snapshot
data: {"sessionId":"2896…","snapshotOrdinal":"1","status":"idle","entities":[]}

id: 2   ChatUserMessageAccepted        {"messageId":"chat-msg-93fa…-user","role":"user","text":"/tool eval …"}
id: 3   ChatRunStarted                 {"messageId":"chat-msg-93fa…","prompt":"/tool eval …"}
id: 4   ChatProviderCallFinished       {"correlation":{},"hasToolCalls":true,"usage":{"inputTokens":15,"outputTokens":5}}
id: 5   ChatFrontendToolCallRequested  {"messageId":"chat-msg-93fa…","toolCallId":"call_d726c1e8","toolName":"eval",
                                        "input":{"code":"6*7"},"mode":"TOOL_EXECUTION_MODE_FRONTEND_AUTO","executor":{…}}
id: 6   ChatFrontendToolResultReceived {"toolCallId":"call_d726c1e8","result":42,"status":"success"}
id: 7   ChatTextSegmentStarted         {"messageId":"chat-msg-93fa…:text:db21…","role":"assistant","streaming":true}
id: 8   ChatTextPatch                  {"messageId":"…:text:db21…","mode":"CHAT_STREAM_PATCH_MODE_APPEND","text":"eval "}
id: 9   ChatTextPatch                  {… "offset":"5","text":"returned: "}
id: 10  ChatTextPatch                  {… "offset":"15","text":"42"}
id: 11  ChatTextSegmentFinished        {"messageId":"…:text:db21…","content":"eval returned: 42","final":true}
id: 12  ChatProviderCallFinished       {"correlation":{},"usage":{"inputTokens":19,"outputTokens":3}}
id: 13  ChatRunFinished                {"messageId":"chat-msg-93fa…","status":"finished"}
```

On the wire each live event is `id: <ordinal>`, `event: ui` and `data: {"sessionId", "ordinal", "name",
"payload"}`; the table above shows the ordinal, the name and the payload. Notice four things:

- Every entity's id names its run: the person's message is `<run>-user`, a text segment `<run>:text:<segment>`, a tool
  call carries `messageId: <run>`. A client can group a conversation into question and answer by id, without relying
  on positions.
- The stream begins with a snapshot. While the snapshot is read, the server buffers events and then sends only those
  after the snapshot's ordinal (`httpapi/sse.go:74-79`), so a client that connects in the middle of a run receives
  everything exactly once. A client that falls too far behind receives `event: error` with
  `{"code":"stream_overflow"}` and reconnects for a fresh snapshot.
- `ChatProviderCallFinished` has an empty `correlation` and no `messageId`. A client cannot tell which run the tokens
  belong to (section 5.2).
- `ChatRunStarted` and `ChatRunFinished` carry no time, no duration and no model.

### 3.4 Timeline entities

What a snapshot can contain (`chat.proto`, `frontendtools/v1/frontend_tool.proto`):

| Kind | Id | Payload fields that matter | Made by |
|---|---|---|---|
| `ChatMessage`, role `user` | `<run>-user` | `text` | `baseTimelineProjection` |
| `ChatMessage`, role `assistant` | `<run>:text:<segment>` | `text`, `streaming`, `parentMessageId` = run | `baseTimelineProjection` |
| `ChatMessage`, role `thinking` | `<run>:thinking:<segment>` | `text`, `parentMessageId` = run | `plugins/reasoning.go` |
| `ChatMessage`, role `error` | `<run>` | `text` (the failure) | `baseTimelineProjection` on `ChatRunFailed` |
| `ChatRun` | `<run>` | `status` (running, finished, failed, stopped), `code`, `error`, `truncated`, `reason` | `projections.go:50-80` |
| `ChatToolCall`, `ChatToolResult` | tool call id | `toolName`, `input`, `result`, `status`, `messageId` = run | `plugins/toolcall.go` |
| `ChatFrontendToolCall` | tool call id | `toolName`, `input`, `result`, `status`, `executor`, `parentMessageId` | `frontendtools/plugin.go` |

A browser tool call can appear twice: as `ChatToolCall` (what the model asked for, when the provider streams tool
calls) and as `ChatFrontendToolCall` (where it ran and what it returned). A client shows one card for both, keyed by
the tool call id.

## 4. How each application uses it today

### 4.1 pinocchio web-chat with react-chat

`cmd/web-chat` serves a React application whose client is react-chat's `@go-go-golems/chat-provider`
(`react-chat/packages/chat-provider`). It has a websocket transport (`ws/sessionStreamTransport.ts`), a Redux store of
timeline entities (`store/timelineSlice.ts`) fed by `ws/timelineEvents.ts` (UI event → entity change) and
`ws/timelineSnapshot.ts`, and a separate run-statistics slice computed from events (`ws/runStatsEvents.ts`,
`store/runStatsSlice.ts`). It is React-only: `react` and `react-dom` are peer dependencies.

### 4.2 slop-computer

slop's server (`slop-computer/pkg/doors/pages/agent/agent.go`) builds an `httpapi.Server` with its own parts:

- a resolver that takes the engine from slop's model backend and meters it with slop's ledger;
- authentication (the signed-in person and the app) and authorization (only one's own sessions, in the same app);
- a create hook that validates the page's settings;
- and, like agentserver: a context-block function, the browser-tool manager, the three standard plugins, and SQLite
  stores for sessions, turns and the timeline.

The page side is `slop-computer/web/runtime/src/agent.ts` (492 lines), bundled into slop's page runtime. Its `Session`
class keeps a local copy of the timeline (`#entities`):

- a snapshot replaces the copy (`#snapshot`);
- each UI event updates it (`#apply`);
- after each change, `#derive` computes everything callers wait for: whether a run is running, the pending send's
  text and outcome, and the browser tool calls this page must run.

Tool results wait in an outbox until the server accepts or refuses them for good. This design came out of slop's
review rounds (SLOP3-025). The commit that introduced it (`6a62fcb`) explains that a client with two paths, one for
the snapshot and one for live events, had a reconnect bug that a single path makes impossible.

`#apply` handles only what slop's API needs: `ChatRun*`, assistant text, and `ChatFrontendToolCall*`. It ignores
thinking and the model's tool-call entities, because slop's `agent()` returns text, not a rendered conversation.

### 4.3 homolog-bench

homolog-bench does not use chatapp yet. Its server, `hb-server`, embeds the old agentd (`pinocchio/pkg/agentd` at
pinocchio v0.11.18; its source is the `pinocchio-agentd` checkout). From `cmd/hb-server/main.go:107-135`:

```go
opts := agentd.Options{Agents: agents, Store: store, Tools: []agentd.Tool{server.EvalTool(mgr)},
    Instructions: server.Instructions(mgr), Context: server.Context(mgr)}
agentSrv, err := agentd.NewServer(ctx, opts)
```

- `EvalTool` (`internal/server/server.go:65`) runs code in the session's room (label `room`) as actor `agent`. When the
  room asks a person to approve tool work, it calls `OnAwaiting("awaiting-person")`, and later `OnAwaiting("running")`.
  agentd's `Session.SetCallState` (`pinocchio-agentd/pkg/agentd/session.go:388`) resets the call's timer to the person
  limit, or back to the full tool limit, and publishes a `tool.progress` event that the page shows on the tool card.
- `Instructions` returns the room's API reference once, when a session is created; agentd appends it to the system
  prompt.
- `Context` returns a `[context]` block with the room's current state for every message.

The browser side is `web/js/agent.js` (661 lines), a plain script without a build step. It speaks agentd's protocol:
`POST /turns`, `GET /events?after=<seq>`, event types such as `text.delta` and `run.finished`, and a hydration view
`GET /sessions/{id}` that lists `messages`. A second client, written in Go, drives the agent for persona evaluations:
`internal/evalrun/client.go` creates a session, posts `/turns` and follows events until `run.finished`.

## 5. What is wrong

### 5.1 The same conversation is derived four times

The server's projections define the conversation; three clients re-derive it, each in its own way:

| Where | What it derives | Language |
|---|---|---|
| `pkg/chatapp/projections.go` and the plugins | backend events → entities (the definition) | Go |
| react-chat `ws/timelineEvents.ts`, `ws/runStatsEvents.ts` | UI events → entities; run statistics | TS + Redux |
| slop `web/runtime/src/agent.ts`, `#apply` | UI events → entities (four kinds) | TS |
| homolog `web/js/agent.js`, `onEvent`, `toDisplay`, `rehydrate` | agentd events → messages | JS |

Each copy must agree with the server's projection, and nothing checks that it does. Disagreements have already been
bugs:

- homolog-bench, Codex review of PR #3: opening a conversation in the middle of an answer showed only the text that
  arrived after the page connected, because the page appended deltas to an empty message. A workaround, `catchUp` in
  `web/js/agent.js`, fixed it.
- homolog-bench, the same review: when a second answer finished, the first lost its footer. The page rebuilt the
  conversation from the server and copied its own details back for the last answer only. The workaround is
  `turnsOf`/`carry`.
- slop-computer, SLOP3-025: a reconnect could leave a send waiting forever, because the snapshot path did not know
  about runs. The fix was the single-path design of 4.2.

### 5.2 Per-answer facts exist only in the browser

The footer "4.2 s · 2.7k in / 140 out · 2 model calls", a thinking block's "Thought for 3 s" and a tool card's "1.2 s"
are facts about one run or one part of it. The server stores none of them:

- `ChatRunEntity` has status, code, error, truncated and reason, but no model, start, finish or usage;
- `ChatProviderCallFinished` carries usage without the run's id (3.3), so even a client that sees every event can only
  attribute tokens to "the run that is probably running";
- entities have ordinals, not times.

Every client therefore measures these facts itself with `Date.now()` while it watches. After a reload, in a second tab,
or for a finished conversation, they are gone. react-chat's `runStatsSlice` and homolog's `carry` exist only to keep
such page-made facts alive across a rebuild.

### 5.3 The server wiring is copied

slop's `agent.go` and agentserver contain the same code:

- opening `sessions.db`, `turns.db` and `timeline.db` (`agent.go:157-179`, `agentserver.go:250-268`);
- the same three plugins and the browser-tool manager;
- turning a message's `context` field into a block (`agent.go:300-319`, `agentserver.go:474-495`);
- the same `codedError` type.

### 5.4 homolog-bench needs three things chatapp lacks

homolog-bench cannot use agentserver as it is:

1. **Server-side instructions.** agentserver accepts instructions from the page (`data.instructions`), checked against
   a size limit. homolog's instructions come from the room, in the server.
2. **Server-side context per message.** agentserver reads `context` from the message body. homolog's context is the
   room's state, computed in the server when the message arrives.
3. **An in-process tool that waits for a person.** `toolexec.DeadlineBy` gives a call one limit, decided when the call
   starts (`toolexec/toolexec.go:79-111`). The person limit exists only for browser tools in mode `human`
   (`agentserver.go:459-472`). homolog's `eval` waits for a person only sometimes (agent mode "ask" with real tool
   work), and only the room knows when. agentd also told the page that the call waits; chatapp has no event for that.

One behavior change must also be absorbed. chatapp's conversation is append-only: every context block stays in the
history, because editing earlier blocks would invalidate the provider's prompt cache. agentd deleted the previous
message's context block before each message. homolog's context must therefore say what changed, not repeat the whole
state with every message.

## 6. Design

### 6.1 Principles

1. **The server stores every fact a page shows.** Whatever a page displays is in a timeline entity: a run's model,
   times and usage; a part's times; a tool call's waiting state. A rebuild from the snapshot then loses nothing, and no
   page-made state needs to be carried across.
2. **One client, shipped with the server it talks to.** The browser client lives in pinocchio next to the projections
   it mirrors, and the Go module embeds its built file. An application that serves it from its own chatapp server
   cannot run a client of another version.
3. **Applications provide content; chatapp provides mechanism.** An application decides who may act, which model and
   prompt to use, what context to add, and which tools exist. Opening stores, parsing the context field, deadlines and
   the stream belong to chatapp.
4. **No compatibility layers.** None of the three applications has outside users of agentd's protocol. homolog-bench
   moves to chatapp's protocol in one step; agentd's files stay on disk and are not imported.

### 6.2 Target architecture

```text
  pinocchio web-chat (React)        slop app page                     homolog-bench Agent panel
  chat-provider (later)             slop agent() ──┐                  agent.js renders answers()
          ┆                                        ▼                              │
          ┆                 ┌──── pkg/chatapp/webclient (TypeScript) ────┐         │
          └┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄►│ Timeline: pure reducer, answers()          │◄────────┘
                            │ Session: SSE, send, stop, tools, outbox    │  served at {prefix}/client.js
                            └─────────────────────┬──────────────────────┘
                                                  │ HTTP + SSE (3.2, 3.3)
  ┌─────────────────── application servers ───────┴─────────────────────────────────────────────┐
  │ agent serve        │ slop: httpapi + own auth and resolver │ hb-server: agentserver + room     │
  └────────────────────┴───────────────────┬──────────────────┴──────────────┬───────────────────┘
                                           ▼                                 ▼
               httpapi: WithStateDir, ContextField          agentserver: WithSessionInstructions,
                                           │                WithMessageContext, Session(ctx)
                                           ▼                toolexec: AwaitPerson
               chatapp: ChatRun with model, times, usage; part times; tool waiting state
```

### 6.3 Answers as stored records

The run entity grows to hold what a footer shows, and each part entity gets times. The times travel in the events,
not in the projection: a projection must give the same result when it is replayed later, so it may not read a clock.

Proposed protobuf changes (field numbers continue each message):

```proto
message ChatRunStarted           { …; int64 started_at_ms = 4; string model = 5; }
message ChatRunFinished          { …; int64 finished_at_ms = 8; }
message ChatRunStopped           { …; int64 finished_at_ms = 5; }
message ChatRunFailed            { …; int64 finished_at_ms = 6; }
message ChatProviderCallFinished { …; string message_id = 7; }      // the run the tokens belong to

message ChatRunEntity {
  …                                  // message_id, status, prompt, code, error, truncated, reason
  string model = 8;
  int64 started_at_ms = 9;
  int64 finished_at_ms = 10;         // 0 while running
  UsageInfo usage = 11;              // summed over the run's provider calls
  int32 provider_calls = 12;
}

// parts
message ChatMessageEntity      { …; int64 started_at_ms = 15; int64 finished_at_ms = 16; }
message ChatTextSegmentStarted / Finished, ChatReasoningSegmentStarted / Finished, ChatUserMessageAccepted
                               { …; int64 at_ms = <next>; }
message ToolCallEntity         { …; int64 started_at_ms = 8; int64 finished_at_ms = 9; string state = 10; }
message ChatToolExecutionStarted, ChatToolCallFinished { …; int64 at_ms = <next>; }
message ChatToolCallProgress   { string message_id = 1; string tool_call_id = 2; string state = 3; int64 limit_ms = 4; }
message FrontendToolCallEntity { …; int64 requested_at_ms = 10; int64 finished_at_ms = 11; }
```

The run sink (`runtime_sink.go`) publishes these events and stamps them with the server's clock (`time.Now()`); it
already knows the run's message id (`runtime_sink.go:28`) and sets it on `ChatProviderCallFinished`. The model comes
from the runtime: `ComposedRuntime` gets a `Model string` field that resolvers set. agentserver sets the resolved model
reference, and slop sets its engine's model.

The run projection then adds usage (Go):

```go
case *chatappv1.ChatProviderCallFinished:
    return runEntity(view, payload.GetMessageId(), func(r *chatappv1.ChatRunEntity) {
        r.Usage = addUsage(r.GetUsage(), payload.GetUsage())
        r.ProviderCalls++
    }), nil
case *chatappv1.ChatRunStarted:
    return runEntity(view, payload.GetMessageId(), func(r *chatappv1.ChatRunEntity) {
        r.Status, r.Prompt = "running", payload.GetPrompt()
        r.Model, r.StartedAtMs = payload.GetModel(), payload.GetStartedAtMs()
    }), nil
```

For the run captured in 3.3, the snapshot afterwards would hold:

```json
{"kind": "ChatRun", "id": "chat-msg-93fa…", "payload": {
  "messageId": "chat-msg-93fa…", "status": "finished", "model": "fake:scripted",
  "startedAtMs": "1795905000120", "finishedAtMs": "1795905001370",
  "usage": {"inputTokens": 34, "outputTokens": 8}, "providerCalls": 2}}
```

The footer becomes `finishedAtMs − startedAtMs`, `usage` and `providerCalls`, the same for a page that watched the
run and a page opened a day later.

### 6.4 Embedding hooks

Go APIs added to pinocchio (exact signatures in section 10):

- **`httpapi.WithStateDir(dir)`** opens the three SQLite stores in `dir` (sessions, turns, timeline) and closes them
  in `Server.Close`. agentserver and slop use it instead of their copies.
- **`httpapi.ContextField(maxBytes)`** is a `BlocksFunc`. It turns a message body's `context` string into a user block
  after the person's message, and refuses more than `maxBytes` with `prompt_too_large`. agentserver and slop use it.
- **`agentserver.WithSessionInstructions(fn)`** runs `fn(ctx, info)` once, when a session is created. Its text is
  stored in the session's data next to the page's instructions and appended to the system prompt of every run, so the
  prompt prefix stays the same for the life of the session.
- **`agentserver.WithMessageContext(fn)`** runs `fn(ctx, info, msg)` for each message. A non-empty text becomes a user
  block after the message, after the page's own `context` if there is one.
- **`(*agentserver.Server).Session(ctx)`** returns the session of the run that a tool is called in. An in-process tool
  uses it to read labels, such as homolog's `room`.
- **`toolexec.AwaitPerson(ctx)`** tells the deadline middleware that the current call now waits for a person; the
  returned `release` function says it no longer does. Without a deadline middleware in the context it does nothing,
  so a tool may call it unconditionally. The deadline middleware also reports the change through an optional callback,
  which agentserver turns into a `ChatToolCallProgress` event. To publish it, agentserver keeps each run's publisher
  in the tool's context through chatapp's existing run-tools hook (`chatapp.WithRunTools`, which receives
  `RunScope{SessionID, MessageID, Publisher}`). The tool call entity's `state` becomes `awaiting-person`, and a page
  shows it on the tool card.

The timers behave as agentd's did:

```text
 call starts ────────► timer set to the ordinary limit (toolTimeout, e.g. 120 s)
 AwaitPerson(ctx) ───► timer reset to the person limit (personTimeout, e.g. 10 min); state "awaiting-person"
 release() ──────────► timer reset to the ordinary limit again; state "running"
 timer fires ────────► the call's context is cancelled; the result is the deadline message
```

### 6.5 The shared browser client

A new Go package, `pkg/chatapp/webclient`, holds the client's TypeScript source (`src/`), its tests (`test/`) and the
built single-file ES module `chatapp-client.js`. The built file is committed because Go modules do not run build
steps for their users; a `make webclient` target rebuilds it with esbuild, and CI fails when the committed file differs
from a fresh build. The package embeds the file, and httpapi serves it at `GET {prefix}/client.js`.

The client has two layers:

- **`Timeline`** is the local copy, with no I/O. `reset(snapshot)` replaces it, `apply(uiEvent)` changes it the way
  the server's projections change entities, and `answers()` derives the conversation. Because it is pure, its tests
  replay captured streams and compare the result with the server's snapshot after the same events: on every capture,
  client and server must agree.
- **`Session`** does the I/O. It reads the event stream through `fetch` and parses the SSE framing itself, as slop
  does, so that headers and abort signals work. It reconnects with backoff; a new stream always starts with a snapshot.
  It sends messages, stops runs, registers browser tools, runs the calls assigned to this page, and keeps their results
  in an outbox until the server has them.

The derived conversation (TypeScript):

```ts
type Turn = { question: Question | null; answer: Answer | null };
type Question = { id: string; text: string; at: number };            // ChatMessage role user, id "<run>-user"
type Answer = {
  id: string;                                                         // the run's message id
  status: "running" | "finished" | "failed" | "stopped";
  code: string; error: string; truncated: boolean; reason: string;
  model: string; startedAt: number; finishedAt: number;               // ms; finishedAt is 0 while running
  usage: { input: number; output: number; cachedInput: number }; providerCalls: number;
  parts: Part[];                                                      // in the order they were created
};
type Part =
  | { kind: "thinking" | "text"; id: string; text: string; streaming: boolean; startedAt: number; finishedAt: number }
  | { kind: "tool"; id: string; name: string; input: unknown; result: unknown; error: string; status: string;
      state: string; where: "server" | "browser"; startedAt: number; finishedAt: number };
```

Grouping rules:

- a text or thinking part belongs to the answer named by its `parentMessageId`; a `ChatToolCall` or `ChatToolResult`
  to the answer named by its `messageId`; a `ChatFrontendToolCall` to the answer named by its `parentMessageId`;
- a question belongs to the answer whose id it carries before the `-user` suffix;
- parts are ordered by the ordinal that created them: `createdOrdinal` in a snapshot, the event's `ordinal` for a
  part first seen live;
- a browser tool's `ChatToolCall` and `ChatFrontendToolCall` merge into one tool part by tool call id, and so do an
  in-process tool's `ChatToolCall` and `ChatToolResult`.

From homolog-bench's page (a plain script, no build step):

```js
const { Session } = await import('/api/v1/agent/client.js');
const s = await Session.open('/api/v1/agent', id);          // or Session.create(base, { labels, data })
s.on(() => render(s.timeline.answers()));                    // one render path: snapshot, events, reconnects
await s.send(text);                                          // 202 → the run's id; the stream does the rest
```

slop's `agent()` keeps its public API. Its `send()` becomes a thin wrapper: post the message, report text through
`onText` while the answer's parts grow, and resolve or reject when `Session.wait(id)` sees a terminal status.

### 6.6 Decisions

- **D1. The client lives in pinocchio, as a Go package with TypeScript source.** The alternatives were a React-free
  subpath of react-chat's chat-provider, or a new repository. pinocchio was chosen because the client mirrors
  `projections.go`: it belongs next to it and changes in the same commit. The Go module also carries the built file to
  applications that have no JavaScript toolchain (homolog-bench). TypeScript consumers, such as slop's esbuild bundle,
  import the source from the module directory (`go list -m -f '{{.Dir}}' github.com/go-go-golems/pinocchio`), so
  `go.mod` is the only version to keep in step. Status: accepted.
- **D2. Server-sent events, not websockets, for the shared client.** httpapi serves both. SSE is plain HTTP through
  existing proxies (hb-server's, slop's sandbox origin), and slop already uses it. react-chat keeps its websocket
  transport until it adopts the shared core. Status: accepted.
- **D3. The client keeps a reducer that mirrors the server's projections.** The alternative is for the server to
  stream changed entities instead of events, so that the client only replaces entities by id and cannot disagree. That
  removes the mirror entirely, but it changes sessionstream's UI fanout and needs append patches for long texts, or
  every delta would resend the whole text. Deferred: the capture tests of 6.5 check agreement now, and the client's API
  (`Timeline.answers()`) stays the same if the server later sends entities. Status: deferred.
- **D4. Times travel in the events.** The alternative is for sessionstream to stamp every event and entity with times.
  That is more general, but it changes sessionstream's stores in another repository; per-event fields stay inside
  pinocchio. Status: accepted.
- **D5. homolog-bench does not import slop-computer.** slop's agent service is built on slop's identity, budgets,
  consent and page wiring. Only chatapp and the client are generic, and both live in pinocchio. Status: accepted.

## 7. Migration of each application

### 7.1 pinocchio

- agentserver uses `WithStateDir` and `ContextField`; `agent serve` behaves as before.
- The client is served at `/v1/client.js`.
- The ticket's smoke script checks that the run entity has model, times and usage.

### 7.2 slop-computer

- `pkg/doors/pages/agent/agent.go`: `openStores` → `httpapi.WithStateDir(cfg.Dir)`; `contextBlocks` →
  `httpapi.ContextField(MaxContextBytes)`; the resolver sets `ComposedRuntime.Model`.
- `web/runtime/src/agent.ts`: the copy, apply, derive, connect and outbox move to the shared client. slop keeps its
  consent (`Grant`), its `open()` options and its result shape. `web/runtime/build.mjs` resolves the shared client to
  the module directory's `pkg/chatapp/webclient/src/index.ts`.
- Tests: `web/runtime/test/agent.test.ts` and `pkg/doors/pages/agent/agent_test.go` keep what they assert.

### 7.3 homolog-bench

- `cmd/hb-server/main.go`: `agentd.NewServer` → `agentserver.New(presets, …)` with `WithToolRegistry` (the `eval`
  tool), `WithSessionInstructions` (the room's API reference), `WithMessageContext` (the room's context when it
  changed), `WithHTTPOptions(httpapi.WithStateDir(<state>/agent))`, and the scripted engine
  (`toolkit/scripted`) for `--engine fake`.
- `internal/server/server.go`: `EvalTool` becomes a geppetto tool function. It finds the room through
  `srv.Session(ctx)` and calls `toolexec.AwaitPerson(ctx)` from the room's `OnAwaiting` callback.
- `configs/agent/*.yaml`: `hostTools: [eval]` → `tools: [eval]`; `personTimeout` stays.
- `web/js/agent.js`: loads the client from `/api/v1/agent/client.js` and renders `answers()`. It deletes `toDisplay`,
  `rehydrate`, `catchUp`, `turnsOf`, `carry`, the placeholder reordering and its own timing; the footer reads the
  answer record.
- `internal/evalrun/client.go`: the new routes (`/messages`, `/stop`); a run's end is its `ChatRun` entity's terminal
  status.
- Old conversations in `data/hb-server/agentd` are not imported; they stay on disk.

## 8. Implementation plan

Each phase ends with its tests passing and a commit. Phases 1–3 form one pinocchio pull request. A first command to
orient yourself: `cd ~/code/wesen/go-go-golems/pinocchio && go test ./pkg/chatapp/...`, then run the smoke script of
3.3 (`ttmp/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP--…/scripts/02-agent-serve-smoke.sh`) and read its event log.

1. **Answer records (pinocchio).** The proto fields of 6.3, regenerated Go and TypeScript code, the run sink setting
   `message_id` and the times, projections filling the run entity and the part times, `ComposedRuntime.Model` set by
   agentserver. Tests: projection tests for usage per run and for times; the smoke script.
2. **Embedding hooks (pinocchio).** `httpapi.WithStateDir`, `httpapi.ContextField` (agentserver uses both),
   `WithSessionInstructions`, `WithMessageContext`, `Session(ctx)`, `toolexec.AwaitPerson` with the progress event.
   A test for each.
3. **Shared client (pinocchio).** `pkg/chatapp/webclient`: `Timeline`, `answers()`, `Session`, browser tools with an
   outbox; an esbuild build to `chatapp-client.js`; `go:embed`; the `{prefix}/client.js` route. Tests: Node tests of
   the reducer against captured streams and their final snapshots (captured with agent serve and the scripted engine:
   text, thinking, a browser tool, a failure, a stop, a reconnect in the middle of a run); a Go test that the served
   file is the build of the source.
4. **slop-computer** on the new pinocchio: the server helpers and `agent.ts` on the client; slop's tests.
5. **homolog-bench** on the new pinocchio: hb-server, the eval tool, presets, agent.js, the evalrun client. Its Go tests,
   `node tests/core/*.test.mjs`, the UI tests (`agent.test.mjs` and `agent-render.test.mjs`, rewritten for chatapp's
   events) and a scripted demo on a test instance.
6. **react-chat** (later, its own ticket): chat-provider's Redux store fed by the shared `Timeline`.

## 9. Testing strategy

- **Server and client agree.** For each captured stream, replay the UI events into `Timeline` and compare its entities
  with the server's snapshot after the same ordinal. A projection change that the client does not mirror fails here.
  This is the test that makes the duplication of 5.1 safe.
- **A snapshot alone rebuilds everything.** For the same captures, `answers()` from the final snapshot alone equals
  `answers()` from the first snapshot plus all events, including model, times, usage and part order.
- **Attach in the middle.** Connect after the first text patch of a run; the client shows the whole text exactly once.
- **Person waits.** A tool that waits past its ordinary limit inside `AwaitPerson` still succeeds; one that exceeds
  the person limit gets the deadline message; the tool call entity's state is `awaiting-person` meanwhile.
- **Applications.** Each application's existing tests, plus homolog-bench's acceptance checks R1–R6 in its HB-006
  design-doc/02: the running answer is in the snapshot, answer records persist, ids are stable, failures persist,
  gaps are signalled, and text and thinking deltas are symmetric.

## 10. API reference (proposed)

### Go

```go
// pkg/inference/runtime
type ComposedRuntime struct { …; Model string }    // the model reference shown to clients

// pkg/chatapp/httpapi
func WithStateDir(dir string) Option                // sessions.db, turns.db, timeline.db in dir
func ContextField(maxBytes int) BlocksFunc          // body "context" → a user block after the message
// GET {prefix}/client.js serves the shared browser client

// pkg/chatapp/agentserver
func WithSessionInstructions(fn func(ctx context.Context, info sessions.Info) (string, error)) Option
func WithMessageContext(fn func(ctx context.Context, info sessions.Info, msg httpapi.MessageRequest) (string, error)) Option
func (s *Server) Session(ctx context.Context) (sessions.Info, bool)

// pkg/chatapp/toolkit/toolexec
type DeadlineOption func(*deadlineOptions)
func DeadlineBy(limit func(ctx context.Context, call geptools.ToolCall) time.Duration, message DeadlineMessage, opts ...DeadlineOption) Middleware
func WithPersonLimit(limit func(ctx context.Context, call geptools.ToolCall) time.Duration) DeadlineOption
func OnState(fn func(ctx context.Context, call geptools.ToolCall, state string, limit time.Duration)) DeadlineOption
func AwaitPerson(ctx context.Context) (release func())
```

### TypeScript (`pkg/chatapp/webclient/src/index.ts`)

```ts
export class Timeline {
  reset(snapshot: Snapshot): void;
  apply(ev: UIEvent): boolean;                       // true when the copy changed
  entity(kind: string, id: string): Entity | undefined;
  entities(kind?: string): Entity[];                 // in creation order
  answers(): Turn[];
  answer(id: string): Answer | null;
  get running(): boolean;
}
export class Session {
  static create(base: string, req: { labels?: Record<string, string>; data?: unknown; id?: string }, o?: Options): Promise<Session>;
  static open(base: string, id: string, o?: Options): Promise<Session>;
  static attach(base: string, info: SessionInfo, o?: Options): Promise<Session>; // a session the caller already listed or created
  static list(base: string, labels: Record<string, string>, o?: Options): Promise<SessionSummary[]>;
  readonly id: string; readonly timeline: Timeline; info: SessionInfo;
  on(fn: (ev: UIEvent | null) => void): () => void; // after every change (null: a snapshot)
  send(prompt: string, extra?: Record<string, unknown>): Promise<string>;   // the run's message id
  wait(messageId: string, signal?: AbortSignal): Promise<Answer>;            // resolves at a terminal status
  stop(): Promise<void>; delete(): Promise<void>; close(): void;
  tools(list: BrowserTool[]): Promise<void>;         // posts the manifest; runs the calls assigned to this page
}
type Options = { fetch?: typeof fetch; headers?: Record<string, string> };
type BrowserTool = { name: string; description?: string; inputSchema?: object; mode?: "frontend" | "human";
                     execute(input: any, ctx: { signal: AbortSignal; callId: string }): unknown };
```

## 11. Risks and open questions

- **react-chat and new fields.** New optional protobuf fields do not break chat-provider, which ignores fields it does
  not know.
- **Clocks.** All times come from the server's clock, and durations are differences of two server times, so the page's
  clock does not matter.
- **homolog's context rule.** "What changed since the last message" needs the room to remember what it last sent to
  each session. The rule is homolog-bench's to define; a first version sends the room's context block only when it
  differs from the last one sent to that session.
- **Open: does `Session.wait` belong in the shared client?** It is small and generic, so it is proposed there; slop
  would otherwise keep its own.

## 12. File reference

| File | Why you read it |
|---|---|
| `proto/pinocchio/chatapp/v1/chat.proto` | events and entities |
| `pkg/chatapp/projections.go` | how entities are made; the client mirrors it |
| `pkg/chatapp/runtime_sink.go`, `runtime_inference.go` | the run lifecycle and where events are published |
| `pkg/chatapp/plugins/reasoning.go`, `plugins/toolcall.go`, `frontendtools/plugin.go` | thinking and tool entities |
| `pkg/chatapp/httpapi/server.go`, `sse.go`, `sessions.go`, `tools.go` | the HTTP API and the stream |
| `pkg/chatapp/agentserver/agentserver.go` | the preset application, where the new hooks go |
| `pkg/chatapp/toolkit/toolexec/toolexec.go` | the deadline middleware |
| `slop-computer/pkg/doors/pages/agent/agent.go` | slop's server wiring |
| `slop-computer/web/runtime/src/agent.ts` | the client the shared one starts from |
| `react-chat/packages/chat-provider/src/ws/timelineEvents.ts` | react-chat's reducer |
| `homolog-bench/cmd/hb-server/main.go`, `internal/server/server.go` | agentd embedding, the eval tool, the room hooks |
| `homolog-bench/web/js/agent.js` | the Agent panel and its workarounds |
| `homolog-bench/internal/evalrun/client.go` | the Go client for persona runs |
| `homolog-bench/ttmp/2026/11/21/HB-006--…/design-doc/02-agent-session-contract-…md` | acceptance checks R1–R6 |
