---
Title: 'Migration playbook: homolog-bench from agentd to chatapp'
Ticket: PIN-20261121-AGENTD-INTO-CHATAPP
Status: active
Topics:
    - chatapp
    - refactor
    - design
    - migration
    - sessionstream
    - backend
DocType: playbook
Intent: long-term
Owners:
    - manuel
RelatedFiles:
    - Path: pkg/chatapp/agentserver/agentserver.go
      Note: what agent serve does with presets, labels, data and context
    - Path: pkg/chatapp/httpapi/server.go
      Note: route table and options of the HTTP API
    - Path: pkg/chatapp/httpapi/sse.go
      Note: the server-sent events format
    - Path: pkg/doc/topics/pinocchio-agent-serve.md
      Note: user-facing API reference (pinocchio help pinocchio-agent-serve)
    - Path: ttmp/2026/11/21/PIN-20261121-AGENTD-INTO-CHATAPP--merge-agentd-into-chatapp-one-agent-runtime-mechanism-in-the-core-content-left-to-the-caller/scripts/02-agent-serve-smoke.sh
      Note: the whole protocol in curl, runnable without a model
ExternalSources: []
Summary: Step-by-step port of homolog-bench's agent page (web/js/agent.js), its preset and its UI test from agentd to `pinocchio agent serve` on chatapp, with every route, field and event mapped.
LastUpdated: 2026-11-22T07:30:00-05:00
WhatFor: Handing the homolog-bench side of the agentd→chatapp merge to the homolog-bench developer.
WhenToUse: When moving homolog-bench (main at b058c23 or later) to a pinocchio built from task/merge-agentd-into-chatapp or main after it merges.
---

# Migration playbook: homolog-bench from agentd to chatapp

## Purpose

pinocchio no longer contains agentd. `pinocchio agent serve` still exists, but it is now a thin application on top
of chatapp, pinocchio's chat runtime, and its HTTP API changed. This playbook moves homolog-bench to it. It is
written for the developer who owns homolog-bench and has not followed the pinocchio work.

What changes for homolog-bench, in one paragraph: the service is started the same way (with `--presets` instead of
`--agents`); sessions are created with labels and data instead of `agent`/`model`/`instructions` fields; a message is
`POST /messages {prompt, context}` instead of `POST /turns {text, context}`; the event stream sends one snapshot and
then live UI events (no `?after=` resume, no hydration "view"); tool results go to one route with the call id in the
body; the per-call "awaiting a person" progress route is gone (approval tools get a longer limit instead); errors are
`{error, code}`. What stays: the browser runs `eval`, the service owns the model loop, credentials and the
conversation, prompt prefixes stay stable for caching, and the fake engine still works for tests.

What does not change for the page's behaviour: one conversation per session, one active answer per session (a second
message answers 409 `run_active`), tools run in the tab that holds the executor assignment, approval is the page's
own Run/Decline UI.

## Environment assumptions

- homolog-bench at main `b058c23` ("Merge #1: HB-002 workbench agent on the agent service") or later. The files to
  change are `configs/agent/homolog-bench.yaml`, `web/js/agent.js`, `tests/ui/agent.test.mjs`, and whatever starts
  the service (the hbd proxy stays: `/api/v1/agent/*` → the service's `/v1/*`).
- A pinocchio binary built from `task/merge-agentd-into-chatapp` (or main after it merges):
  `go build -o ~/bin/pinocchio ./cmd/pinocchio` in the pinocchio checkout.
- No model credentials are needed for the first steps: `--engine fake` uses the scripted engine.
- Read once: `pinocchio help pinocchio-agent-serve` (the API reference this playbook refers to).

## Step 1 — run the new service against the existing preset

```bash
cd ~/code/wesen/homolog-bench
pinocchio agent serve --presets configs/agent --engine fake --address 127.0.0.1:8797 --state-dir /tmp/hb-agent
```

It fails at start with:

```text
configs/agent/homolog-bench.yaml: yaml: unmarshal errors:
  line 14: cannot unmarshal !!map into []string
```

Edit the preset:

| Old key | Action | Why |
|---|---|---|
| `tools: {eval: auto}` | delete | A browser tool's approval mode is declared by the page in its manifest (`mode: "frontend"` or `"human"`). `tools:` is now a list of *in-process* tools, which agent serve does not have. |
| `pageInstructions: true`, `maxPageInstructionsBytes` | keep | Same meaning: the page may append instructions at session creation. |
| `personTimeout` (if you add it) | optional | Limit of a `human`-mode call; default 10m. |
| `systemPrompt`, `model`, `thinking`, `maxTokens`, `maxRounds`, `toolTimeout`, `resultCut`, `budget.tokensPerSession` | keep | Same meaning; zero uses agent serve's default (40 rounds, 120s, 9000 characters, no budget). |
| — | optional new | `maxContextBytes` (default 64 KiB) limits a message's `context`. |

Then start it again and check it answers:

```bash
curl -s 127.0.0.1:8797/v1/health    # {"ok":true,"presets":1}
curl -s 127.0.0.1:8797/v1/presets
curl -s 127.0.0.1:8797/v1/models
```

The whole protocol can be tried in curl before touching the page: pinocchio's
`ttmp/.../scripts/02-agent-serve-smoke.sh [workdir] [preset-dir]` builds the binary, starts it with the fake engine,
creates a session with instructions, registers `eval`, sends `/tool eval {"code":"6*7"}`, answers `42`, and prints the
conversation and the SSE event names. Pass homolog-bench's `configs/agent` as the preset dir.

Update the service command wherever it is written down (README, `agent.test.mjs` line 2, the 502 "fix" text in the
test at line 95, `AVAIL_TEXT`): `--agents` → `--presets`.

## Step 2 — route and field mapping

All paths below are under the service's prefix `/v1`, i.e. under `BASE = /api/v1/agent` in the page.

| agentd (today in agent.js) | agent serve | Notes |
|---|---|---|
| `GET /agents` → `[{name, model, ...}]` | `GET /presets` → `{presets: [{name, description, model, thinking, maxRounds, pageInstructions, default}]}` | `agentInfo` = the preset named `homolog-bench`; `maxRounds` for the truncation note comes from here |
| `GET /models` → `{models}` | same | unchanged |
| `GET /sessions?agent=NAME` → `[SessionView]` | `GET /sessions?label=preset:NAME` → `{sessions: [{id, title, labels, created, updated, usage, running}]}` | note the wrapper object; `running` is a bool |
| `POST /sessions {agent, model, instructions}` → view (`view.id`, `view.seq`) | `POST /sessions {labels: {preset: NAME, model?: REF}, data: {instructions}}` → `201 {sessionId, session}` | no `seq`: the event stream starts with a snapshot |
| `GET /sessions/{id}` → view (`messages`, `seq`, `running`, `usage`, `limits`) | `GET /sessions/{id}` → `{sessionId, session, running, snapshot}` | see step 4 for the snapshot |
| `DELETE /sessions/{id}` | same → `204` | |
| `POST /sessions/{id}/tools/manifest {revision, clientInstanceId, connectionId, tools}` | same → `{accepted, revision, executor}` | unchanged; `executor` is what `mine` holds |
| `POST /sessions/{id}/turns {text, context}` | `POST /sessions/{id}/messages {prompt, context}` → `202` | `409 run_active` while an answer runs, as before |
| `POST /sessions/{id}/tools/{callId}/result {toolName, status, value, error, executor}` | `POST /sessions/{id}/tools/results {toolCallId, toolName, status, result, error, executor}` | `result` is any JSON value: send the string itself, no `{value: x}` wrapping |
| `POST /sessions/{id}/tools/{callId}/progress {state}` | removed | see step 6 |
| `POST /sessions/{id}/cancel` | `POST /sessions/{id}/stop` → `202` | |
| `GET /sessions/{id}/events?after=SEQ` | `GET /sessions/{id}/events` | see step 3 |
| — | `GET /sessions/{id}/turns?format=yaml|json`, `/timeline`, `/export` | server-side exports, if useful for "Export" |

Result statuses stay `success`, `failed`, `denied` (also `cancelled`, `timeout`). A result for a call the service
already gave up on answers 410 with code `late_result`, as before.

### Errors

agentd answered `{"error": {"code", "message", "fix"}}`; agent serve answers `{"error": "message", "code": "code"}`.
Change `api()`:

```js
if (!r.ok) {
  const err = new Error((data && data.error) || `HTTP ${r.status}`);
  err.code = (data && data.code) || (r.status === 404 && !data ? 'no-proxy' : 'http-' + r.status);
  err.status = r.status;
  throw err;
}
```

There is no `fix` field any more; keep the page's own fix texts (`AVAIL_TEXT`). Codes the page may see on requests:
`run_active`, `session_not_found`, `unknown_preset`, `unknown_model`, `instructions_not_allowed`, `prompt_too_large`
(instructions or context too long), `invalid_request`, `late_result`, `unknown_result`, `executor_mismatch`.

## Step 3 — the event stream

The stream sends one `snapshot` event, then one `ui` event per UI event. Every `ui` event has the same envelope:

```text
event: snapshot
data: {"sessionId":"…","snapshotOrdinal":"12","status":"…","entities":[…]}

id: 13
event: ui
data: {"sessionId":"…","ordinal":"13","name":"ChatTextPatch","payload":{…}}
```

So `connect()` registers two listeners instead of eleven:

```js
es = new EventSource(`${BASE}/sessions/${id}/events`);
es.addEventListener('snapshot', (e) => hydrate(JSON.parse(e.data)));   // replaces open()'s GET and rehydrate()
es.addEventListener('ui', (e) => onEvent(JSON.parse(e.data)));
es.addEventListener('error', (e) => { /* a server "error" event (e.data) means: reconnect; the browser's own error has no data */ });
```

Reconnecting is always correct: a new stream starts with a new snapshot, and the service guarantees nothing is lost
or repeated between the snapshot and the live events. `?after=` and `Last-Event-ID` do nothing. If the page falls far
behind, the service sends `event: error` with `{"code":"stream_overflow"}` and closes; reconnect.

### Event mapping

A run is identified by the run's message id (`payload.messageId` of `ChatRunStarted`); events of the run carry it
(text segments as `<runId>:text:<segment>`, so match the prefix, or use `parentMessageId` where present).

| agentd `ev.type` | chatapp `name` | Payload fields to read |
|---|---|---|
| `user.message` (`d.text`) | `ChatUserMessageAccepted` | `text`, `messageId` |
| `run.started` (`d.model`) | `ChatRunStarted` | `messageId` (the run id). The model is not in the event; use the session's `labels.model` or the preset's |
| `text.delta` (`d.text`, full text) | `ChatTextSegmentStarted`, `ChatTextPatch`, `ChatTextSegmentFinished` | `ChatTextPatch.text` is a **delta** when `mode` is `CHAT_STREAM_PATCH_MODE_APPEND` (`offset` = position), the full text for `…_SNAPSHOT`/`…_REPLACE`. `ChatTextSegmentFinished.content` is the final text |
| `reasoning.delta` (`d.delta`) | `ChatReasoningSegmentStarted`, `ChatReasoningPatch`, `ChatReasoningSegmentFinished` | same patch rules |
| `usage` (`d.total`) | `ChatProviderCallFinished` | `usage` of that one model call (`inputTokens`, `outputTokens`, cache fields). For the session total read `session.usage` from `GET /sessions/{id}` after the run |
| `tools.manifest` (`d.executor`) | — | not broadcast. The manifest answer gives this tab's `executor`; each `ChatFrontendToolCallRequested` names the executor the call is assigned to, so compare `payload.executor.assignmentId` with `mine.assignmentId` instead of tracking a session-wide executor |
| `tool.requested` (`d.callId, d.name, d.input, d.mode`) | `ChatFrontendToolCallRequested` | `toolCallId`, `toolName`, `input`, `mode` (`TOOL_EXECUTION_MODE_FRONTEND_AUTO` or `TOOL_EXECUTION_MODE_FRONTEND_HUMAN`), `executor`, `messageId` |
| `tool.finished` (`d.callId, d.status, d.error`) | `ChatFrontendToolResultReceived` | `toolCallId`, `status`, `result`, `error` |
| `run.finished` (`d.text, d.truncated`) | `ChatRunFinished` | `truncated: true` and `reason: "max_rounds"` when the round limit forced the final answer (show the "ask it to continue" note) |
| `run.failed` (`d.code, d.message`) | `ChatRunFailed` | `code`, `error`. Codes: `max_rounds`, `quota_exceeded`, `authentication_error`, `rate_limited`, `prompt_too_large`, `upstream_error`, `tools_error`, `history_error`. `ERR_TEXT` keys stay valid |
| `run.cancelled` | `ChatRunStopped` | |

`ev.seq` bookkeeping (`cur.view.seq = ev.seq`) goes away.

## Step 4 — hydration (opening a conversation, reload)

agentd's view had `messages: [{kind: user|assistant|tool_call|tool_result, ...}]`; `toDisplay()` turned them into
the page's messages. The snapshot has timeline entities instead, in order:

| Entity `kind` | `payload` | Maps to |
|---|---|---|
| `ChatMessage`, `role: "user"` | `text` | a user message |
| `ChatMessage`, `role: "assistant"` | `text`, `status` (`streaming`/`finished`/`stopped`), `parentMessageId` (the run) | assistant text; one entity per text segment, so a run with tool calls has several |
| `ChatMessage`, `role: "thinking"` | `text` | reasoning |
| `ChatMessage`, `role: "error"` | `text` (the failure text), `status: "failed"` | a failed run |
| `ChatFrontendToolCall` | `toolCallId`, `toolName`, `input`, `result`, `status`, `error`, `executor`, `parentMessageId` | an eval record (`code` = `input.code`) |
| `ChatToolCall` (tool-call plugin) | `toolCallId`, `toolName`, `input` (JSON text), `status` | the model's side of the same call, when the provider streams it; key records by `toolCallId` and merge, or ignore this kind |

Rewrite `toDisplay()` over `snapshot.entities`, grouping by run (`parentMessageId`, or the user message that precedes
the entities). `open(id)` no longer needs `GET /sessions/{id}` for the conversation: connect the stream and build the
view from its `snapshot` event. `GET /sessions/{id}` is still useful for `session` (title, labels, usage) and
`running`.

`running` is exact; the snapshot's `status` is a guess from entities. A run stopped before any output leaves no
assistant entity (known gap); the page's own "Stopped." note covers it.

`rehydrate()` after a run end: a fresh `GET /sessions/{id}` gives the same snapshot the stream would; or simply keep
the live view, since nothing is lost between snapshot and live events.

## Step 5 — creating sessions and sending messages

```js
const created = await api('POST', '/sessions', {
  labels: { preset: AGENT, ...(model ? { model } : {}) },
  data: { instructions: instructions() },
});
cur = { id: created.sessionId, session: created.session, msgs: [] };
connect(created.sessionId);
...
await api('POST', `/sessions/${cur.id}/messages`, { prompt: text, context: snapshot() });
```

The conversation is now **append-only**, and this changes how `context` should be used. agentd deleted the previous
turn's context block before each turn; chatapp keeps every block, because removing or editing earlier blocks breaks
the provider's prompt cache (every turn would pay for the whole conversation again). Each `context` you send stays in
the conversation, after the message it came with. So:

- send what changed since the last message, or a short summary, not the whole state each time;
- or send no context and let the model read the state through `eval` (`ide.get()`, `ws.status()`), which it already
  can;
- keep it under `maxContextBytes` (64 KiB default; 400 `prompt_too_large` above).

The system prompt (the preset's prompt plus `data.instructions`) is fixed when the session is created. To change the
API reference the page sends, start a new session.

## Step 6 — tools, approval and timing

- Results: post `{toolCallId, toolName, status, result, error, executor: mine}` to `/tools/results`. Send `result` as
  the value itself (`out`, a string); delete `resultText()`'s `{value: x}` unwrapping. The model receives the JSON
  value (`"→ 42"` arrives as a JSON string).
- The service cuts a result at `resultCut` characters and appends its own note; the page's own `RESULT_MAX` cut can
  stay or go.
- Approval: register `eval` with `mode: "human"` in the manifest if a person must approve every call, or keep
  `"frontend"` and keep the page's per-call Run/Decline as today (`d.mode === 'human'`). A decline posts
  `status: "denied"` with an error text.
- Time limits: a `frontend` call is stopped after `toolTimeout` (default 120 s) with a tool error telling the model
  the step was stopped; a `human` call waits up to `personTimeout` (default 10 min). The `progress`/
  `awaiting-person` route no longer exists: delete the calls to it. If the workbench's room timer needs to pause while
  a person decides, that is now the page's own logic.
- Stop: `POST /stop` cancels the run; a browser call in progress is answered `cancelled` by the service, and a late
  result answers 410 `late_result` (the existing note "The service had already given up on this call" fits).

## Step 7 — the UI test

`tests/ui/agent.test.mjs`:

- line 2: `pinocchio agent serve --presets configs/agent --engine fake --address 127.0.0.1:8797 --state-dir /tmp/x &`
- line 95: the mocked 502 body becomes `{"error": "not reachable", "code": "agent-unavailable"}`, and the expected
  fix text says `--presets`.
- The scripted engine understands the same commands as before: `/tool NAME {json}`, `/tools N NAME {json}`,
  `/sleep MS text`, otherwise `echo: text`; after a tool result it answers `NAME returned: <result>`. Note the result
  is the JSON value, so a string result prints with quotes: `eval returned: "→ 42"`. Adjust expected strings.
- The script is read from the person's message, not from `context`, so sending `context` does not change it.

## Exit criteria

- `pinocchio agent serve --presets configs/agent` starts with homolog-bench's preset, and `/v1/presets` lists it.
- `node tests/ui/agent.test.mjs` passes against `--engine fake`.
- In the browser, with a real model: a new conversation is created with the API reference, `eval` calls run in the
  tab and their results reach the model, Stop works, a reload while an answer runs shows the conversation so far and
  continues streaming, a second tab does not run tools assigned to the first, the round-limit note appears after
  `maxRounds`, Delete removes the conversation from the list.
- No request in the page goes to `/turns`, `/cancel`, `/tools/{callId}/result` or `/tools/{callId}/progress`.

## Notes

- Sessions and conversations made by agentd's JSON store (`~/.local/state/pinocchio/agentd`) are not read by the new
  service (state now in `~/.local/state/pinocchio/agent`, three SQLite files). There is no import; old conversations
  stay in the old files.
- The service still has no authentication and listens on 127.0.0.1; hbd's proxy stays the only way in.
- Session ids are server-generated uuids unless the page passes `id` at creation (1–128 characters of
  `[A-Za-z0-9._:-]`).
- If homolog-bench later wants the service in-process instead of a second binary, `pkg/chatapp/agentserver.New`
  (presets + `WithModels` or `WithEngine`) gives the same handler; `pkg/chatapp/httpapi` alone is the lower level
  (own resolver, `WithBlocks`, `WithAuthenticator`).
- Questions about the design go to the ticket's guide, `design-doc/01-intern-guide-…`, sections 7.3 (history and
  caching) and 7.5 (tools).
