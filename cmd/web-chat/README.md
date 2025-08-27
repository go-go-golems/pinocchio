---
Title: Pinocchio Web-Chat Example
Slug: pinocchio-web-chat
Short: Modular WebSocket chat UI that streams Geppetto events to the browser.
Topics:
- web-chat
- streaming
- events
- websocket
- middleware
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Pinocchio Web-Chat (Streaming Example)

## Overview

This example serves a minimal, production-style web chat UI that streams model and tool events over WebSockets. It demonstrates how to:

- Build a per-conversation Geppetto engine with middlewares
- Attach an `events.Sink` to capture in-flight engine events
- Convert those events into frontend-friendly semantic envelopes
- Broadcast frames over WebSockets to all clients of a conversation

The code is intentionally modular: the CLI entrypoint (`main.go`) wires flags and layers, while the implementation lives in `pkg/backend/`.

## Directory Structure

- `main.go` — Cobra command that initializes layers and launches the backend server
- `pkg/backend/`
  - `server.go` — HTTP wiring (static assets, `/ws`, `/chat`), event router startup, engine creation
  - `conversation.go` — Per-conversation lifecycle, reader subscription, WebSocket fan-out
  - `forwarder.go` — Maps Geppetto events to semantic envelopes (llm.*, tool.*)
  - `util.go` — Small helper utilities (e.g., log level parsing)
  - `redis_layer_adapter.go` — Exposes the Redis parameter layer to the command
- `static/` — Embedded HTML/JS/CSS assets; Vite build may populate `static/dist/`

## Concepts and Context

Pinocchio’s web-chat mirrors the event-driven approach used in agent UIs: instead of waiting for a final string, the engine emits a sequence of events describing what happens (start, deltas, tools, final, logs). The backend forwards these to the browser as semantic frames. This enables:

- Immediate rendering (`llm.start`)
- Incremental streaming (`llm.delta` chunks)
- Tool lifecycle visualization (`tool.start/delta/result/done`)
- Logs and mode switches

## Control Flow (End-to-End)

1) The CLI boots the server
   - `main.go` builds a Cobra command from Glazed layers and calls `backend.NewServer(...).Run(...)`.
2) A browser connects to `/ws?conv_id={id}`
   - The backend upgrades the connection, attaches it to the conversation, and ensures a per-conversation reader is running.
3) A POST to `/chat` starts a run for that conversation
   - The backend appends the user block to a long-lived `turns.Turn` and runs a tool-calling loop.
   - The run context includes an `events.Sink` which publishes engine events to a per-conversation stream/topic.
4) The reader subscribes to that topic
   - For each event, the backend converts it to one or more semantic frames and broadcasts them to all WebSocket clients of that conversation.
5) The UI updates incrementally
   - `llm.start` appears immediately, `llm.delta` chunks accumulate, and `llm.final` completes the message. Tool events are interleaved.

## Key Files and Symbols

### Command Entrypoint

- File: `main.go`
  - `NewCommand()` — Defines the Cobra command with parameters and layers (Geppetto + Redis)
  - `(*Command).RunIntoWriter(ctx, parsed, _)` — Constructs and runs the backend server:

```go
srv, err := webbackend.NewServer(ctx, parsed, staticFS)
return srv.Run(ctx)
```

### Backend Server

- File: `pkg/backend/server.go`
  - `type WebServerSettings` — Flags for address, agent mode, idle timeout
  - `type Server` — Holds HTTP server, event router, tool registry, DB, agent mode config
  - `NewServer(ctx, parsed, staticFS)` —
    - Parses settings (default layer + `redis` layer)
    - Builds an `events.EventRouter` (in-memory or Redis Streams)
    - Initializes tool registry (calculator) and optional SQLite-backed SQL middleware
    - Configures agent mode middlewares
    - Registers HTTP routes `/`, `/static/*`, optional `/assets/*`, `/ws`, `/chat`
  - `Run(ctx)` — Runs the Watermill router, starts HTTP server, handles graceful shutdown
  - `buildEngine()` — Creates a per-conversation engine and wraps middlewares:
    - `SystemPromptMiddleware`
    - Optional `AgentModeMiddleware`
    - `ToolResultReorderMiddleware`
    - Optional SQLite tool middleware

### Conversations and Streaming

- File: `pkg/backend/conversation.go`
  - `type Conversation` — Per-conversation state:
    - `RunID`, `Turn`, `Eng`, `Sink`
    - active WebSocket connections set and subscriber
    - reader lifecycle (start/stop on idle)
  - `getOrCreateConv(convID)` — Creates a conversation, builds engine and sink, subscribes reader
  - `startReader(conv)` — Subscribes to the per-conversation topic (`chat:{convID}`) and loops messages:

```go
e, _ := events.NewEventFromJson(msg.Payload)
// optional inline debug/log handling
s.convertAndBroadcast(conv, e) // -> SEM frames to sockets
```

  - `convertAndBroadcast(conv, e)` — Uses the forwarder to produce frames and writes to all sockets
  - `addConn/removeConn` — Manages WebSocket lifetimes and idle-stop of the reader

### Semantic Conversion

- File: `pkg/backend/forwarder.go`
  - `SemanticEventsFromEvent(e events.Event) [][]byte` — Maps engine events to semantic frames wrapped as `{ sem: true, event: {...} }`:
    - `EventPartialCompletionStart` → `llm.start`
    - `EventPartialCompletion` → `llm.delta` (with `delta` and `cumulative`)
    - `EventFinal`/`EventInterrupt` → `llm.final`
    - `EventToolCall/Execute/Result/ExecutionResult` → `tool.*`
    - `EventLog` → `log`

## HTTP and WebSocket API

- `GET /` — Serves `static/dist/index.html` if present, otherwise `static/index.html`
- `GET /ws?conv_id={string}` — Upgrades to WebSocket, attaches the socket to the conversation
- `POST /chat` — Starts a run
  - Body: `{ "prompt": string, "conv_id": string (optional) }`
  - Response: `{ "run_id": string, "conv_id": string }`

## Redis Streams (Optional Transport)

When `redis` is enabled via flags, the backend uses a Redis Streams publisher/subscriber under the hood. Each conversation gets a topic `chat:{convID}` with a consumer group `ui`. This allows horizontal scaling of readers and decouples event production from UI delivery.

Parameters (via layer `redis`):

- `redis-enabled` (bool) — Enable Redis Streams transport
- `redis-addr` (string) — Redis `host:port`
- `redis-group` (string) — Consumer group (default `chat-ui`)
- `redis-consumer` (string) — Consumer name (default `ui-1`)

## Minimal End-to-End Run (Pseudocode)

```go
// POST /chat handler
conv, _ := s.getOrCreateConv(convID)
turns.AppendBlock(conv.Turn, turns.NewUserTextBlock(prompt))
runCtx := events.WithEventSinks(s.baseCtx, conv.Sink)
_, _ = toolhelpers.RunToolCallingLoop(
    runCtx, conv.Eng, conv.Turn, s.registry,
    toolhelpers.NewToolConfig().WithMaxIterations(5),
)

// Reader goroutine
for msg := range ch { // subscribed to chat:{convID}
    e, _ := events.NewEventFromJson(msg.Payload)
    for _, b := range SemanticEventsFromEvent(e) {
        socket.Write(b)
    }
    msg.Ack()
}
```

## Building and Running

1) Build Vite assets (optional): ensure `static/dist` contains a built UI, otherwise `static/index.html` is used.
2) Run the command:

```bash
go run ./cmd/web-chat --addr :8080 --redis-enabled=false
```

Open `http://localhost:8080/` and connect.

## Notes and Best Practices

- Keep examples minimal and focused: semantic conversion and event routing are the core ideas.
- Use consumer groups at the tail (`$`) to avoid replaying full history to the UI.
- Use per-conversation topics to isolate runs and simplify filtering.
- Log at debug level for event traffic to troubleshoot mapping issues.


