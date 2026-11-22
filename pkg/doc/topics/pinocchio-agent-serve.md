---
Title: Agent service and the chat HTTP API
Slug: pinocchio-agent-serve
Short: How `pinocchio agent serve` and pkg/chatapp/httpapi run agent sessions over HTTP, with browser-executed tools, presets, limits and server-sent events.
Topics:
- chatapp
- agent
- http
- sessionstream
- frontendtools
Commands:
- agent
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## What it is

`pinocchio agent serve` runs an HTTP service where a web page talks to a model. The page creates a session,
registers the tools it implements (for example an `eval` that runs code in the page), sends messages and reads
events. When the model calls one of the page's tools, the service asks the page to run it and waits for the result.
The model loop, the credentials and the conversation stay in the service.

The service is built from three pinocchio packages, which other programs can use directly:

- `pkg/chatapp` runs a session's turns: the tool loop, its limits, the busy policy, the outcome of every run and the
  history. It writes no text of its own into the conversation.
- `pkg/chatapp/httpapi` serves chatapp sessions over HTTP: session routes, messages, stop, browser-tool routes,
  exports, and two event transports (websocket and server-sent events).
- `pkg/chatapp/agentserver` is the agent application on top: it turns a preset (model, system prompt, limits) into
  each message's runtime. `pinocchio agent serve` is a thin command around it.

## Running it

```bash
pinocchio agent serve --presets configs/agent                 # models from profiles and Pi
pinocchio agent serve --presets configs/agent --engine fake   # scripted engine, no credentials
pinocchio agent models                                        # the models it can use
```

Flags: `--address` (default `127.0.0.1:8790`), `--presets` (directory of YAML presets), `--default-preset`,
`--state-dir` (SQLite files `sessions.db`, `turns.db`, `timeline.db`; default `~/.local/state/pinocchio/agent`),
`--prefix` (default `/v1`), `--engine models|fake`, and the model-source flags (`--profile-registries`, `--no-pi`,
`--pi-coding-agent-dir`, ...).

The service has no authentication of its own and listens on 127.0.0.1. Put the application's server in front of it.
A program that embeds `httpapi` adds authentication with `httpapi.WithAuthenticator` and `httpapi.WithAuthorizer`.

## Presets

```yaml
name: bench
description: Workbench agent
model: pi:anthropic/claude-sonnet-4-5   # a modelsource reference
thinking: low
maxTokens: 8000
systemPrompt: bench.system.md          # relative to this file
pageInstructions: true                 # the page may append instructions at session creation
maxPageInstructionsBytes: 65536
maxRounds: 40                          # default 40
toolTimeout: 120s                      # per tool call, browser tools included; default 120s
resultCut: 9000                        # characters of one tool result; default 9000
budget:
  tokensPerSession: 2000000            # input + output over the session; 0 = none
```

A preset's `tools` list names in-process tools, which only a program embedding `agentserver` with
`WithToolRegistry` can offer. Browser tools are declared by the page, including whether a person approves each call
(`mode: "human"`).

## The API

All routes are under the prefix (`/v1` for agent serve, `/api/chat` by default for `httpapi`). Errors answer
`{"error": "...", "code": "..."}`.

| Route | Body → answer |
|---|---|
| `GET /health`, `/presets`, `/models` | agent serve only |
| `POST /sessions` | `{id?, title?, labels?: {preset?, model?, thinking?}, data?: {instructions?}}` → `201 {sessionId, session}` |
| `GET /sessions?label=k:v&limit=n` | → `{sessions: [{id, owner, labels, title, created, updated, usage, running}]}` |
| `GET /sessions/{id}` | → `{sessionId, session, running, snapshot}` |
| `DELETE /sessions/{id}` | stops the run, deletes the metadata → `204` |
| `POST /sessions/{id}/tools/manifest` | `{clientInstanceId, connectionId, revision, tools: [{name, description, mode, inputSchema, available}]}` → `{accepted, revision, executor}` |
| `POST /sessions/{id}/messages` | `{prompt, attachments?, idempotencyKey?}` → `202`; `409 run_active` while a run is active |
| `POST /sessions/{id}/stop` | → `202` |
| `GET /sessions/{id}/events` | server-sent events (below) |
| `POST /sessions/{id}/tools/results` | `{toolCallId, toolName, status, result, error?, executor}`; `result` is any JSON value |
| `GET /sessions/{id}/timeline`, `/turns`, `/export` | exports (`?format=json|yaml|minitrace`) |
| `GET /ws` | websocket (sessionstream protocol) |

`executor` is the object the manifest answer returned; a result must come from the tab that holds the assignment.

### Events

`GET /sessions/{id}/events` first sends one `snapshot` event (the session's timeline entities and the ordinal they
are current at), then one `ui` event per UI event, with the ordinal as the SSE `id`:

```text
event: snapshot
data: {"sessionId":"…","snapshotOrdinal":"12","entities":[…]}

id: 13
event: ui
data: {"sessionId":"…","ordinal":"13","name":"ChatTextPatch","payload":{…}}
```

The stream never skips or repeats an event between the snapshot and the live events. A reconnecting client reads a
new snapshot; `Last-Event-ID` is ignored. A client that falls too far behind gets `event: error` with
`{"code":"stream_overflow"}` and should reconnect.

The UI event names a client needs:

| Event | Meaning |
|---|---|
| `ChatUserMessageAccepted` | the person's message is in the session |
| `ChatRunStarted` | a run began |
| `ChatTextSegmentStarted`, `ChatTextPatch`, `ChatTextSegmentFinished` | the answer text |
| `ChatReasoning…`, `ChatToolCall…` | with the reasoning and tool-call plugins |
| `ChatFrontendToolCallRequested` | the page should run a browser tool (`toolCallId`, `toolName`, `input`, `executor`) |
| `ChatFrontendToolResultReceived` | the result was accepted |
| `ChatProviderCallFinished` | one model call ended; `usage` |
| `ChatRunFinished{truncated, reason}` | the run ended; `truncated` with `reason: "max_rounds"` when the round limit forced the final answer |
| `ChatRunStopped` | the run was stopped |
| `ChatRunFailed{code, error}` | the run failed: `max_rounds`, `quota_exceeded`, `authentication_error`, `rate_limited`, `prompt_too_large`, `upstream_error`, `tools_error`, `history_error` |

## How a run is built

For each message agent serve builds a runtime from the session's preset:

1. The engine for the session's model (`labels.model`, else the preset's) and thinking level.
2. The system prompt middleware with the preset's prompt plus the session's `data.instructions`. It is the same text
   for every turn of the session, so the prompt prefix stays the same (prompt caching).
3. The tool loop with the preset's round limit and a final answer at the limit.
4. A per-call deadline and a result cut around the run's final executor, so browser tools are covered.
5. A round guard over the session's token usage.

The conversation is append-only: chatapp never edits or removes earlier blocks. A program that wants to give the
model per-message state (for example the page's current state) adds blocks after the person's message with
`httpapi.WithBlocks`; they stay in the conversation.

## Embedding the API

```go
srv, err := httpapi.New(
    httpapi.WithRuntimeResolver(resolver),            // required: the runtime of each message
    httpapi.WithSessionStore(store),                  // default: memory
    httpapi.WithTurnStore(turns),                     // default: memory
    httpapi.WithFrontendTools(frontendtools.NewManager()),
    httpapi.WithChatPlugins(frontendtools.NewPlugin()),
    httpapi.WithAuthenticator(auth),
    httpapi.WithBlocks(stateBlocks),
    httpapi.WithEngineOptions(chatapp.WithRunTools(toolexec.RunTools(toolexec.Deadline(2*time.Minute, nil)))),
)
srv.Register(mux)
```

See also: `pinocchio help chatapp-protobuf-plugins`, `pinocchio help geppetto-model-sources`.
