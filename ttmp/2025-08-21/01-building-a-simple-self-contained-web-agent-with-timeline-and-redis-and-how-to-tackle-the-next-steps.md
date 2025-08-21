---
Title: Building a Simple Self-Contained Web Agent with Timeline and Redis — Architecture, Current State, Issues, and Next Steps
Slug: web-agent-timeline-redis-architecture-and-plan
Short: End-to-end notes on the current web agent (Go + Redis Streams + WebSocket + JS timeline), how data flows, what files are involved, observed issues, and a concrete action plan.
Topics:
- pinocchio
- geppetto
- watermill
- redis
- websocket
- timeline
- preact
- tools
Date: 2025-08-21
---

## Overview

This document explains how our new web-based chat agent works end-to-end. It details the code layout (which files do what), the runtime data flow from Geppetto’s inference engine through Redis Streams and Watermill to the browser via WebSockets, and how the JS timeline renders the streaming UI. It also documents the issues we are currently seeing (e.g., “updated” events arriving before “created” in the browser) and proposes a detailed, actionable plan to improve robustness, add components (Preact), and converge on a production-ready structure (compatible with future React/RTK Query adoption).

This is meant as a bootstrapping guide for a new developer or intern who hasn’t seen this codebase before.

## High-Level Architecture

- Geppetto Engines emit typed events during streaming inference (start/partial/final, tool-call/-execute/-result, log/info, agent-mode, etc.).
- We publish those events to a Watermill Publisher; transport can be in-memory or Redis Streams.
- A Watermill Router attaches handlers (consumers) to forward events to sinks, loggers, storage, and our WebSocket server.
- The WebSocket server sends JSON messages to the browser.
- The browser-side app receives messages, updates a timeline store, and renders entities (assistant text, tool calls/results, log/info, agent-mode) with a small timeline UI layer.

## Code Map (Files and Responsibilities)

### Go — Web Agent Server and Routing

- `pinocchio/cmd/web-chat/main.go`
  - Cobra/Viper command and server bootstrapping (HTTP + WebSocket).
  - Parses the `redis` layer flags (enabled, addr, group, consumer) via `pkg/redisstream`.
  - Builds a `geppetto/pkg/events.EventRouter` with either in-memory or Redis Streams pub/sub.
  - Registers logging handlers and sets up per-conversation subscribers (consumer groups) when Redis is enabled.
  - HTTP endpoints:
    - `/` serves `static/index.html` (and assets under `/static/`).
    - `/ws?conv_id=...` upgrades to a WebSocket and joins a conversation.
    - `/chat` POST starts a background inference run for a conversation.
  - Orchestrates a Conversation map (conv_id → conversation, run_id, per-WS connection set, subscriber channel, running flag, cancel func).
  - Runs the tool calling loop with Geppetto’s `toolhelpers.RunToolCallingLoop` and sends Watermill sink events.
  - Graceful shutdown on Ctrl+C: cancels server context, shuts down HTTP, closes the event router.

- `pinocchio/pkg/redisstream/router.go`
  - Helpers to build a Redis-backed `EventRouter` and per-group Subscribers.
  - Symbols: `BuildRouter(Settings, verbose)`, `BuildGroupSubscriber(addr, group, consumer)`.

- `pinocchio/cmd/web-chat/pkg/backend/forwarder.go`
  - Converts Geppetto typed events (`geppetto/pkg/events`) into timeline lifecycle JSON messages for the web UI.
  - Entry point: `TimelineEventsFromEvent(e events.Event) [][]byte`.
  - Handles: llm_text start/partial/final; tool_call/exec/result; log events; agent mode switch; interrupt.
  - Includes debug logging for visibility.

### Go — Engines, Tools, Middlewares

- Geppetto Events and Tool Loop:
  - `geppetto/pkg/inference/toolhelpers/helpers.go`
    - `RunToolCallingLoop(ctx, eng, initialTurn, registry, config) (*turns.Turn, error)` orchestrates inference + tools.
    - Note: we added a nil-robustness fix to ensure `Turn.Data` is initialized (`if t.Data == nil { t.Data = map[string]any{} }`).
  - `geppetto/pkg/events` (typed event model and router wrapper).
  - OpenAI streaming engine emits start/partial/final events; log lines show when start is published.

- Agent Mode and SQL tool middlewares (optional):
  - `pinocchio/pkg/middlewares/agentmode` — adds an agent mode control plane and inserts a system/user message.
  - `pinocchio/pkg/middlewares/sqlitetool` — a tool middleware backed by SQLite (when DB is available).

- Simple tools we register in the web server for now:
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/tools/tools.go` — Calculator tool (`RegisterCalculatorTool`).

### Browser — Web UI Timeline

- `pinocchio/cmd/web-chat/static/index.html`
  - Loads styles (`/static/css/timeline.css`) and our ES modules (`/static/js/...`).
  - The DOM contains a timeline container and a simple input form.

- `pinocchio/cmd/web-chat/static/js/app.js`
  - ESM script using Zustand (ESM) for state.
  - Manages conversation ID, WebSocket connection, and chat POST.
  - Receives WS messages, dispatches them to `handleEvent`, which updates the timeline store.
  - Includes ample console logging for debugging.

- Timeline Modules (plain JS for now; easy to evolve to Preact components):
  - `static/js/timeline/types.js`: Lifecycle enums and event classes.
  - `static/js/timeline/store.js`: Core store managing entities, ordering, and lifecycle applications.
    - Important: if an `updated` event arrives before a `created`, the store now auto-creates a placeholder (e.g., `llm_text`), because on the web the WS may connect after the first `created` message or events may be reordered on initial join.
  - `static/js/timeline/registry.js`: Renderer registry abstraction.
  - `static/js/timeline/controller.js`: Applies store changes to the DOM and keeps it in scroll-sync.
  - Renderers:
    - `renderers/llm-text.js`
    - `renderers/tool-call.js`
    - `renderers/agent-mode.js`
    - `renderers/log-event.js`
  - `static/css/timeline.css`: Styles for all entity types.

## End-to-End Data Flow

1) UI triggers `/chat` with prompt and a `conv_id` (created client-side on first load).
2) Server (`main.go`) looks up or creates the conversation, ensures only one run at a time, and starts a background goroutine:
   - Builds `WatermillSink(router.Publisher, "chat")`.
   - Builds an engine from parsed layers, wraps with middlewares (system prompt, agent mode, tool result reorder, optional SQL middleware).
   - Ensures `Turn.RunID`, seeds a turn with the user prompt.
   - Calls `RunToolCallingLoop` with the sink attached to context so all events hit the `"chat"` topic.
3) Router (`EventRouter`) receives events from the sink. In Redis mode, events go through `watermill-redisstream` and consumer groups.
4) The web server’s per-conversation subscriber (group `ui-<conv_id>`) reads from topic `"chat"` and filters by `RunID`.
5) Each Watermill message is converted into web timeline lifecycle messages by `pkg/backend/forwarder.go` and broadcast to all WS clients attached to that conversation.
6) Browser receives the JSON `{ tl: true, event: { type, entityId, ... } }`, updates the timeline store, and re-renders the DOM via the controller and renderers.

## Current Behavior and Observations

- We see `EventPartialCompletionStart` (“start”) logged on the engine side (OpenAI engine logs), and the forwarder maps it to a timeline `created` for an `llm_text` entity.
- However, it is possible for the client to miss this `created` because:
  - The WS connection may be established slightly after the server started emitting messages.
  - Redis consumer groups and reconnects can cause the first message we observe to be `updated`.
  - Multiple consumers per group can load-balance events if misconfigured (we avoid this by per-conversation group subscribers for UI).
- To remain robust, the JS timeline’s store detects an `updated` without a prior `created` and synthesizes a placeholder entity, so the UI still renders partials and eventually final.

## Known Issues

- **Created vs. Updated ordering on first connect**: If the WS attaches late, the first message the client sees may be an `updated` rather than `created`. We already mitigated this client-side; long-term we could implement server-side replay of recent messages per RunID.

- **Zero `message_id` in some cases**: We occasionally observe an all-zero UUID in logs for `message_id`. This disables stable identity for entities. The engine side has a debug log for this; the forwarder logs the event id and type. We should ensure the provider sets a stable message ID across the stream.

- **Per-conversation consumer groups**: The server currently creates a per-conversation Redis consumer group (e.g., `ui-<conv_id>`). This avoids competition with other handlers, but we need to document lifecycle of these groups and cleanup strategy if conv_ids are many.

- **Client rendering is DOM-imperative**: The current renderers manually construct DOM; maintainable for now but we want to move to Preact components (React-compatible) to pave the way for RTK Query / React.

- **No server-side backfill/replay**: A newly connected WS client won’t see entities created before it attached. Client-side placeholder is a stopgap; better is a limited replay buffer per RunID.

- **Tools and web forms**: We’ve integrated the calculator and agent mode; “generative-ui” tool requires a browser form (modal/dialog) with a request/response loop; not wired yet.

## Logging/Debugging Additions

- `pkg/backend/forwarder.go` now logs every received event with type, event_id, run_id, turn_id, and whether it’s mapped to a `created/updated/completed` timeline message (and entity kind). Use `--log-level debug --with-caller` to see the path.

## Detailed Plan — Next Steps

### A) Stabilize Event Delivery for UI

- Add a small per-RunID replay buffer on the server (in-memory cache keyed by run_id, N recent timeline messages). On WS connect (or `/ws?conv_id=<..>&replay=true`), replay the last M messages. This guarantees `created` arrives for late clients.
- Keep the current client-side placeholder creation as safety.

### B) Componentize the Frontend with Preact + HTM

- Introduce Preact via ESM and HTM (JSX-less) in `static/index.html`:
  - `https://esm.sh/preact@10`, `https://esm.sh/htm@3`.
- Build Preact components for:
  - `LLMText`, `ToolCall`, `ToolResult`, `AgentMode`, `LogEvent`.
  - `Timeline` component that maps the store’s ordered entities to components.
- Replace the DOM-imperative `controller.js` with a thin adapter that triggers a Preact render on store updates.
- This keeps us aligned with a future React+RTK Query migration.

### C) Improve Consumer Group Strategy and Cleanup

- Document the lifecycle of per-conversation groups (`ui-<conv_id>`). Add a background janitor that drops idle groups after N minutes and no pending entries.
- Alternative: a single `ui` group with unique consumer name per WS, and server-level filtering by run_id. This trades off replay simplicity vs. isolation.

### D) Tools and Web Forms Integration

- Add a minimal unstyled modal that can render a “generative-ui” form:
  - Server receives tool UI request → broadcasts a `ui-form` message to WS clients for that conversation.
  - Client displays form, collects values, POSTs result to server under `/tool-ui-result` (or via WS ack). Server continues tool loop.

### E) Persistence and History (Optional)

- Persist messages/events per RunID to SQLite (similar to the TUI agent’s event SQL logger) for reload/replay.
- Provide `/history?conv_id=...` to reload timelines client-side.

### F) Robustness and Observability

- Add metrics around event lag (engine → WS), message counts per type, and consumer group health.
- Add a `/healthz` endpoint and a page that tails recent WS/timeline traffic.

## FAQ

- **Why do I occasionally see only `updated` without `created`?**
  - Timing on initial attach or transport buffering. Use server replay or the current store’s auto-create logic.

- **Why Redis Streams?**
  - Ordered append-only log with consumer groups, easy to scale and to isolate concerns (UI/logs/persist) by group.

- **Why Preact + HTM?**
  - Zero-build dev with a React-compatible mental model, which lets us migrate to React + RTK Query later with minimal rewrite.

## Appendix: Key Symbols and Calls

- Server setup:
  - `rediscfg.BuildRouter(rs, verbose)` → `EventRouter`
  - `events.WithPublisher/WithSubscriber` (via builder)
  - `/ws` → per-conversation subscriber → forwarder → broadcast
  - `/chat` → build engine + middlewares → `RunToolCallingLoop`

- Forwarder:
  - `TimelineEventsFromEvent(e events.Event) [][]byte` → JSON messages like `{ tl: true, event: { type, entityId, ... } }`.

- Timeline store:
  - `applyEvent(event)` routes to `onCreate`/`onUpdate`/`onComplete`/`onDelete`.
  - On `updated` without `created`, it now creates a placeholder entity (default `llm_text`).

---

This document should be enough to get a new contributor productive on the web agent: where events come from, how we forward them, where they land in the browser, what to look for when debugging, and what to build next.


