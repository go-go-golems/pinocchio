### How to do multi-turn conversation management in Pinocchio LLM applications

This document explains how other programs in this repo manage multi-turn conversations with Geppetto (Turns/Blocks), and how to implement an in-memory, SEM-only multi-turn flow in the web-chat backend so a single WS connection carries a full run with state.

---

### 1) What other programs do (patterns observed)

- Engines and middlewares
  - Engines are created via `factory.NewEngineFromParsedLayers(parsedLayers, engineOptions...)`.
  - A streaming sink (Watermill) is attached through engine options or via middleware wrapping.
  - Middlewares compose around `RunInference(ctx, *turns.Turn)` to add: system prompts, agent modes, tool execution, logging, reordering, snapshots.

- Turn and Block lifecycle
  - A Turn is the unit of state across inference calls. It contains ordered Blocks with kinds like `llm_text`, `tool_call`, `tool_use`, `system`, `user`.
  - Multi-turn is achieved by carrying the same Turn (or appending to it) across calls. Each iteration appends Blocks; engines/middlewares read previous Blocks to continue.
  - Helpers and middleware inspect Turn metadata and blocks to orchestrate tools.

- Event routing
  - Engines emit events (start/partial/final, tools) via sinks to a router; handlers convert these to UI frames (timeline or semantic) and broadcast to clients.
  - In terminal apps, a router/tea program is run with forwarding handlers; in our web-chat, WS broadcasting takes this role.

- State persistence options in examples
  - Some examples persist snapshots to sqlite (pre/post middleware hooks) and/or keep Turn state in memory while UI is active.
  - For a web server, we can keep per-conversation state in memory keyed by `conv_id`, with a TTL cleanup.

References:
- `cmd/agents/simple-chat-agent/main.go`: middleware chaining, snapshots, dedicated UI forwarders.
- Geppetto docs: `08-turns.md`, `06-inference-engines.md`, `09-middlewares.md` (Turns/Blocks model, engine responsibilities, middleware composition).

Key symbols and where to find them:
- Geppetto types: `github.com/go-go-golems/geppetto/pkg/turns` (`Turn`, `Block`, `AppendBlock`, `NewUserTextBlock`).
- Event router/sink: `github.com/go-go-golems/geppetto/pkg/events` (`EventRouter`, `WithHandlerSubscriber`, `WithEventSinks`) and `github.com/go-go-golems/geppetto/pkg/inference/middleware.NewWatermillSink`.
- Engine factory: `github.com/go-go-golems/geppetto/pkg/inference/engine/factory.NewEngineFromParsedLayers`.
- Tool registry: `github.com/go-go-golems/geppetto/pkg/inference/tools`.
- Middlewares: `github.com/go-go-golems/geppetto/pkg/inference/middleware`.
- Pinocchio web backend WS: `pinocchio/cmd/web-chat/main.go`.
- SEM forwarder: `pinocchio/cmd/web-chat/pkg/backend/forwarder.go` (`SemanticEventsFromEvent`, `wrapSem`).
- Web client store: `pinocchio/cmd/web-chat/web/src/store.js` (SEM-only `handleIncoming`).

---

### 2) Requirements for web-chat

- SEM-only protocol: backend emits `{ sem: true, event }` frames over WS.
- In-memory multi-turn: keep a long-lived Turn per `conv_id` while a WS connection is active. No DB.
- On user prompt: append a `user` block to the Turn; run inference; publish SEM frames streamed to the same WS clients.
- Tool loop: middlewares handle tool execution; SEM frames include `tool.start/delta/result/done`.
- Isolation: multiple conversations (WS connections) run independently and concurrently.
- Cleanup: GC idle conversations (e.g., after WS closes or after inactivity timeout).

---

### 3) Proposed backend structure (in-memory)

- Conversation registry
```go
type Conversation struct {
  ID       string
  RunID    string
  Turn     *turns.Turn
  Engine   engine.Engine          // middleware-wrapped
  Sink     *middleware.WatermillSink
  Router   *events.EventRouter
  // Bookkeeping
  conns    map[*websocket.Conn]bool
  connsMu  sync.RWMutex
  ctx      context.Context
  cancel   context.CancelFunc
  // Optional last-activity timestamp for GC
}

type ConvManager struct {
  mu    sync.Mutex
  convs map[string]*Conversation
}
```

- Lifecycle
  - On first client message/connection for a `conv_id`:
    - Create `Conversation{ ID: conv_id, RunID: uuid.NewString(), Turn: &turns.Turn{} }`.
    - Create Router + Watermill sink; wrap base engine with needed middlewares (system prompt, agent mode, tool reordering, tool execution, logging).
    - Start the router.
  - Attach WS to `conns` set; on close, remove and possibly GC the conversation.
  - Broadcast helper writes SEM frames to all conns in the conversation.

Source code anchors to study:
- WS handling and broadcaster scaffold in `pinocchio/cmd/web-chat/main.go` (search for `convertAndBroadcast`, `conv.conns`, `conv.connsMu`).
- SEM forwarder in `pinocchio/cmd/web-chat/pkg/backend/forwarder.go` (`SemanticEventsFromEvent`).

---

### 4) Control flow per user prompt (single WS connection)

1) Client sends POST `/chat` with `{ prompt, conv_id }`.
2) Backend ensures a `Conversation` exists for `conv_id`; if not, creates it.
3) Append a user block to the Turn:
```go
turns.AppendBlock(conv.Turn, turns.NewUserTextBlock(prompt))
```
4) Kick off inference loop for this Turn (non-blocking if streaming):
```go
go func(){
  // Attach sink to ctx so tools/middlewares can emit events
  runCtx := events.WithEventSinks(conv.ctx, conv.Sink)
  // RunInference reads/extends conv.Turn in-place
  updated, err := conv.Engine.RunInference(runCtx, conv.Turn)
  if err == nil && updated != nil { conv.Turn = updated }
}()
```
5) As engine and middlewares stream, the forwarder converts Geppetto events into `{ sem: true, event }` and broadcasts to all WS clients joined to this `conv_id`.
6) On additional user inputs, repeat steps 3–5, continuously carrying forward the same Turn (multi-turn).

Notes:
- If your provider requires interleaving tool execution and model calls, the tool middleware performs internal loops while still appending blocks to the same Turn.
- Agent-mode or other middlewares can enrich behavior without changing this flow.

Related files and symbols:
- HTTP endpoint: `POST /chat` handler in `pinocchio/cmd/web-chat/main.go` (look for JSON `{prompt, conv_id}`) and response `{ run_id, conv_id }`.
- Conversation WS join: in `main.go`, see the WS upgrade path and `conv_id` parsing.

---

### 5) Engine and middleware wiring (web-chat)

Build a single engine template at server startup from CLI/layer config, then for each conversation wrap with middlewares that are stateless or only depend on Turn fields:

```go
base, _ := factory.NewEngineFromParsedLayers(parsed)
wrapped := middleware.NewEngineWithMiddleware(base,
  middleware.NewSystemPromptMiddleware("..."),
  agentmode.NewMiddleware(service, cfg),
  middleware.NewToolResultReorderMiddleware(),
  // optional: logging middleware
)
```

Attach Watermill sink bound to the conversation’s router topic. The router handler should call our SEM forwarder (already added in `forwarder.go`) and broadcast via WS.

Pointers:
- Engine factory: `factory.NewEngineFromParsedLayers` (provider config via layers/viper; see `cmd/agents/simple-chat-agent/main.go` for setup patterns).
- Middlewares composition: `middleware.NewEngineWithMiddleware` (examples in docs `09-middlewares.md`).
- Tools per Turn: attach registry/config via `Turn.Data` keys when needed.

---

### 6) WebSocket integration

- Per connection, join it to the conversation’s `conns` map.
- `ws.onmessage` on the client remains SEM-only; server sends frames directly from the router handler:
```go
sendBytes := func(b []byte) {
  conv.connsMu.RLock()
  for c := range conv.conns { _ = c.WriteMessage(websocket.TextMessage, b) }
  conv.connsMu.RUnlock()
}
for _, b := range backend.SemanticEventsFromEvent(e) { sendBytes(b) }
```

Where this lives today:
- `pinocchio/cmd/web-chat/main.go`: the `convertAndBroadcast` closure calls the forwarder and writes to all `conv.conns`.
- After SEM-only migration, ensure only `SemanticEventsFromEvent` is used.

---

### 7) Cleaning up and GC

- On WS close: remove the conn. If there are no more conns and no active inference, schedule cleanup after a short TTL.
- Optionally expose an admin endpoint to drop a conversation.
- Track last activity timestamp to expire inactive conversations.

---

### 8) Implementation checklist (web-chat)

- [ ] Conversation manager: in-memory registry keyed by `conv_id` with Turn, Engine, Router, Sink, WS set
- [ ] Build middleware-wrapped Engine per conversation
- [ ] Route Geppetto events through SEM forwarder only (already in place)
- [ ] POST `/chat`: append user block to the Turn and start inference on the same Turn
- [ ] WS: associate connection with `conv_id`, broadcast SEM frames from router handler
- [ ] GC/TTL: remove conversations when idle/closed
- [ ] Logging/metrics: per-conversation counters, last-activity timestamp

Filenames and functions to touch:
- `pinocchio/cmd/web-chat/main.go`:
  - Conversation struct and map (add if missing).
  - WS upgrade and registration (ensure `conv_id` parsing and `conns` set management).
  - HTTP `POST /chat` handler (append user block; kick inference goroutine).
  - Engine/middleware creation per conversation.
  - Event subscriber that invokes `backend.SemanticEventsFromEvent` and broadcasts bytes.
- `pinocchio/cmd/web-chat/pkg/backend/forwarder.go`:
  - `SemanticEventsFromEvent(e events.Event) [][]byte` (extend if needed).
- `pinocchio/cmd/web-chat/web/src/store.js`:
  - `handleIncoming` SEM-only routes to semantic actions.
- Docs to reference: `geppetto/pkg/doc/topics/08-turns.md`, `06-inference-engines.md`, `09-middlewares.md`.

---

### 9) Notes on state and consistency

- The Turn object is shared within a conversation. Use a mutex if concurrent user prompts can overlap; otherwise serialize prompts per conv.
- Engines should be treated as stateless; conversation state lives in Turn/Blocks, not inside Engine (other than configuration).
- Tool registries can be per-Turn (preferred) or per-conversation. Attach via `Turn.Data` when needed.

---

### 10) Why this works well with Geppetto

- Turns/Blocks let the engine and middlewares understand prior context, tool calls, and results without rebuilding prompts manually.
- Middlewares compose to add new behaviors without changing the store/WS logic.
- The SEM-only bridge separates UI concerns from inference concerns while keeping the action log semantically rich.

---

### 11) New developer quickstart (where to look / how to run)

- Backend entrypoints and core files:
  - `pinocchio/cmd/web-chat/main.go` (HTTP/WS server, conversation registry, event subscriber)
  - `pinocchio/cmd/web-chat/pkg/backend/forwarder.go` (Geppetto event → SEM mapping)
- Frontend (web):
  - `pinocchio/cmd/web-chat/web/index.html` (DOM skeleton)
  - `pinocchio/cmd/web-chat/web/src/app.js` (mount, WS connect, submit handler)
  - `pinocchio/cmd/web-chat/web/src/store.js` (Zustand store, SEM ingestion, semantic actions, timeline state)
  - `pinocchio/cmd/web-chat/web/src/timeline/components.js` (Preact renderers)
- Geppetto documentation to read:
  - `geppetto/pkg/doc/topics/08-turns.md`
  - `geppetto/pkg/doc/topics/06-inference-engines.md`
  - `geppetto/pkg/doc/topics/09-middlewares.md`

Run:
- Backend: `go run ./pinocchio/cmd/web-chat`
- Browser: open the server URL (see `main.go` for port)

Debug:
- Backend logs: look for `component=web_forwarder` (SEM emission) and WS broadcast logs.
- Frontend devtools: check `ws/message`, `sem/recv`, and `sem/*` action entries.


