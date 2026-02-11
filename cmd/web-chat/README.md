---
Title: Pinocchio Web-Chat Example
Slug: pinocchio-web-chat
Short: Modular WebSocket chat UI that streams Geppetto events to the browser.
Topics:
- webchat
- streaming
- events
- websocket
- middleware
Commands:
- web-chat
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

The code is intentionally modular: the CLI entrypoint (`main.go`) wires flags and layers, while the implementation lives under `pinocchio/pkg/webchat/`.

## Directory Structure

- `main.go` — Cobra command that initializes layers and launches the backend server
- `gen_frontend.go` — `go generate` hook to build the frontend into `static/dist`
- `static/` — Embedded HTML/JS assets (built output under `static/dist/`)
- `web/` — Vite + React + TypeScript frontend source
- `pinocchio/pkg/webchat/`
  - `router.go` — HTTP wiring (static assets, `/ws`, `/chat`, `/timeline`), engine creation
  - `conversation.go` — Per-conversation lifecycle, queue state, WebSocket fan-out
  - `stream_coordinator.go` — Single-writer + fan-out coordination for streaming SEM frames
  - `sem_translator.go` — Geppetto events → SEM envelope frames (registry-only; protobuf payloads)
  - `sem_buffer.go` — In-memory SEM frame buffer for hydration gating
  - `send_queue.go` — Backend-owned send serialization + idempotency
  - `engine_from_req.go` — Build engine/profile selection from request + headers/body

## Concepts and Context

Pinocchio’s web-chat mirrors the event-driven approach used in agent UIs: instead of waiting for a final string, the engine emits a sequence of events describing what happens (start, deltas, tools, final, logs). The backend forwards these to the browser as semantic frames. This enables:

- Immediate rendering (`llm.start`)
- Incremental streaming (`llm.delta` chunks)
- Tool lifecycle visualization (`tool.start/delta/result/done`)
- Logs and mode switches

## Control Flow (End-to-End)

1) The CLI boots the server
   - `main.go` builds a Cobra command from Glazed layers and calls `webchat.NewRouter(...).BuildHTTPServer(...)`.
2) A browser connects to `/ws?conv_id={id}`
   - The backend upgrades the connection, attaches it to the conversation, and ensures a per-conversation reader is running.
3) A POST to `/chat` starts a run for that conversation
   - The backend queues user prompts per conversation (backend-owned send serialization and idempotency).
   - The run context includes an `events.Sink` which publishes engine events to the stream coordinator.
4) The reader subscribes to that topic
   - For each event, the backend converts it to one or more semantic frames and broadcasts them to all WebSocket clients of that conversation.
5) The UI updates incrementally
   - `llm.start` appears immediately, `llm.delta` chunks accumulate, and `llm.final` completes the message. Tool events are interleaved.

## Key Files and Symbols

### Command Entrypoint

- File: `main.go`
  - `NewCommand()` — Defines the Cobra command with parameters and layers (Geppetto + Redis)
  - `(*Command).RunIntoWriter(ctx, parsed, _)` — Constructs and runs the webchat router/server:

```go
r, err := webchat.NewRouter(ctx, parsed, staticFS)
srv, err := r.BuildHTTPServer()
return srv.ListenAndServe()
```

### Backend Server

- File: `pinocchio/pkg/webchat/router.go`
  - `NewRouter(ctx, parsed, staticFS)` — builds a composable router:
    - Parses settings (default layer + `redis` layer)
    - Builds an event router (in-memory or Redis Streams)
    - Registers HTTP routes `/`, `/static/*`, optional `/assets/*`, `/ws`, `/chat`, `/timeline`
  - `APIHandler()` / `UIHandler()` — optional split handlers to serve API/WS separately from the UI
  - `RunEventRouter(ctx)` — runs the underlying Watermill router loop
  - `BuildHTTPServer()` — constructs the `http.Server` using parsed layers

### Conversations and Streaming

- File: `pinocchio/pkg/webchat/conversation.go`
  - `type Conversation` — Per-conversation state:
    - `SessionID`, queue state/idempotency records, stream coordinator
    - active WebSocket connections set
  - `ConvManager.GetOrCreate` — Creates or reuses a conversation, builds engine and sink, subscribes reader
  - `ConvManager.AddConn/RemoveConn` — Manages WebSocket lifetimes and idle-stop of the reader
- `StreamCoordinator` derives `seq` from Redis stream IDs when available and includes `stream_id` in SEM frames; if missing it falls back to a time-based monotonic `seq` so timeline ordering remains stable

### Semantic Conversion

- File: `pinocchio/pkg/webchat/sem_translator.go`
  - Registry-only mapping of Geppetto events to SEM frames, each wrapped as `{ sem: true, event: {...} }`:
    - `EventPartialCompletionStart` → `llm.start`
    - `EventPartialCompletion` → `llm.delta` (with `delta` and `cumulative`)
    - `EventFinal`/`EventInterrupt` → `llm.final`
    - `EventToolCall/Execute/Result/ExecutionResult` → `tool.*`
    - `EventLog` → `log`

## HTTP and WebSocket API

- `GET /` — Serves `static/dist/index.html` if present, otherwise `static/index.html`
- `GET /ws?conv_id={string}` — Upgrades to WebSocket, attaches the socket to the conversation
- `POST /chat` — Starts an inference for the current session
  - Body: `{ "prompt": string, "conv_id": string (optional) }`
  - Response: `{ "status": "started"|"queued", "conv_id": string, "session_id": string, "turn_id": string, "inference_id": string, "queue_position"?: number, "idempotency_key": string }`
- `GET /timeline?conv_id={string}&since_version={uint64?}&limit={int?}` — Returns timeline snapshot entities (backed by SQLite when configured, otherwise in-memory)
- `GET /turns?conv_id={string}&session_id={string?}&phase={string?}&since_ms={int64?}&limit={int?}` — Returns stored turn snapshots (only when turn store is configured)

## Redis Streams (Optional Transport)

## Durable Timeline Snapshots (PI-004 “actual hydration”)

The `/timeline` endpoint is backed by a SQLite projection store when configured, otherwise it falls back to an in-memory store for a single unified hydration path.

Enable durable persistence by passing one of:
- `--timeline-dsn "<sqlite dsn>"`
- `--timeline-db "<path/to/timeline.db>"`

Example:

```bash
go run ./cmd/web-chat --addr :8080 --timeline-db /tmp/pinocchio-timeline.db
```

## Durable Turn Snapshots (Turn Inspection)

Turn snapshots persist the **exact** LLM input blocks (including middleware-injected system prompts). This is intended for debugging and inspection.

Enable turn persistence by passing one of:
- `--turns-dsn "<sqlite dsn>"`
- `--turns-db "<path/to/turns.db>"`

Example:

```bash
go run ./cmd/web-chat --addr :8080 --turns-db /tmp/pinocchio-turns.db
```

You can then query `/turns` to see YAML payloads for each stored phase.

When `redis` is enabled via flags, the backend uses a Redis Streams publisher/subscriber under the hood. Each conversation gets a topic `chat:{convID}` with a consumer group `ui`. This allows horizontal scaling of readers and decouples event production from UI delivery.

Parameters (via layer `redis`):

- `redis-enabled` (bool) — Enable Redis Streams transport
- `redis-addr` (string) — Redis `host:port`
- `redis-group` (string) — Consumer group (default `chat-ui`)
- `redis-consumer` (string) — Consumer name (default `ui-1`)

Eviction controls (flags):

- `idle-timeout-seconds` — stop per-conversation reader after N seconds with no sockets
- `evict-idle-seconds` — evict idle conversations after N seconds (0 disables)
- `evict-interval-seconds` — sweep idle conversations every N seconds (0 disables)

## Minimal End-to-End Inference (Pseudocode)

```go
// POST /chat handler (enqueue; at most one active inference per conversation/session)
conv, _ := r.cm.GetOrCreate(convID, profileSlug, overrides)
prep, _ := conv.PrepareSessionInference(idempotencyKey, profileSlug, overrides, prompt)
if !prep.Start {
    return prep.HTTPStatus
}
go r.startInferenceForPrompt(conv, profileSlug, overrides, prompt, idempotencyKey) // emits SEM frames via stream coordinator
```

## Building and Running

1) Build Vite assets:

```bash
go generate ./cmd/web-chat
```

2) Run the command:

```bash
go run ./cmd/web-chat --addr :8080 --redis-enabled=false
```

Open `http://localhost:8080/` and connect.

## Frontend Checks (Webchat)

Run these from `cmd/web-chat/web`:

```bash
npm run typecheck   # TypeScript checks
npm run lint        # Biome lint
npm run check       # typecheck + lint
```

## Notes and Best Practices

- Keep examples minimal and focused: semantic conversion and event routing are the core ideas.
- Use consumer groups at the tail (`$`) to avoid replaying full history to the UI.
- Use per-conversation topics to isolate sessions and simplify filtering.
- Log at debug level for event traffic to troubleshoot mapping issues.

## Related Documentation

For deeper understanding of the webchat framework:

- [Webchat Framework Guide](../../pkg/doc/topics/webchat-framework-guide.md) — End-to-end usage guide
- [Backend Reference](../../pkg/doc/topics/webchat-backend-reference.md) — StreamCoordinator and ConnectionPool API
- [Backend Internals](../../pkg/doc/topics/webchat-backend-internals.md) — Implementation details
- [Debugging and Ops](../../pkg/doc/topics/webchat-debugging-and-ops.md) — Troubleshooting
- [Frontend Integration](../../pkg/doc/topics/webchat-frontend-integration.md) — WebSocket and HTTP patterns
- [SEM and UI](../../pkg/doc/topics/webchat-sem-and-ui.md) — Event routing and entities
