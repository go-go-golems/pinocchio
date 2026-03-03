---
Title: Pinocchio TUI Integration Guide (Tool-Loop / Agent Mode)
Slug: pinocchio-tui-integration-guide
Short: Step-by-step guide to embed Pinocchio’s Bubble Tea + Bobatea terminal chat UI with the extracted tool-loop backend and agent forwarder.
Topics:
- pinocchio
- tui
- bubbletea
- bobatea
- geppetto
- events
- watermill
- tutorial
Commands:
- simple-chat-agent
- pinocchio
Flags: []
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

This guide explains how to integrate Pinocchio’s terminal TUI stack into a Go application in a way that is understandable to a brand-new intern. It covers the “moving parts” (Bubble Tea, Bobatea timeline entities, Geppetto events, Watermill routing, Pinocchio backends/forwarders), then walks through a minimal integration recipe you can adapt.

The specific reusable pieces extracted in PI-02 are:

- Tool-loop backend: `pinocchio/pkg/ui/backends/toolloop/backend.go`
- Agent forwarder (event → timeline mapping): `pinocchio/pkg/ui/forwarders/agent/forwarder.go`

## What you are building (high level)

This section explains the end-to-end dataflow so you can debug the system without guessing.

In the integrated architecture:

- The **UI** is a Bubble Tea program (`*tea.Program`) rendering a Bobatea chat model.
- The **backend** runs inference (simple or tool-loop) and publishes Geppetto events to Watermill.
- The **forwarder** reads Watermill messages, decodes Geppetto events, and injects **timeline entity** messages into the Bubble Tea program.

### The “wire protocol” between backend and UI: timeline entities

Bobatea’s chat model is **timeline-centric**: it expects backends/forwarders to send messages like:

- `timeline.UIEntityCreated`
- `timeline.UIEntityUpdated`
- `timeline.UIEntityCompleted`
- `timeline.UIEntityDeleted` (less common, but supported)

These messages create and update renderable entities (assistant text, tool calls, logs, web search results, etc.) in the UI timeline. The chat model then renders them using registered renderer factories.

**API reference / anchor files**

- Backend contract: `bobatea/pkg/chat/backend.go`
- Timeline entity messages: `bobatea/pkg/timeline/types.go`

### Event transport: Watermill topic(s)

Geppetto engines emit structured events (partial tokens, final text, tool calls/results, logs, etc.). In this architecture, engines publish those events as JSON payloads to a Watermill topic via:

- `middleware.NewWatermillSink(publisher, topic)` → `events.EventSink`

**API reference / anchor files**

- Event router abstraction: `geppetto/pkg/events/event-router.go`
- Watermill sink: `geppetto/pkg/inference/middleware/sink_watermill.go`
- Event decoding: `geppetto/pkg/events/events.go` (look for `NewEventFromJson`)

### Diagram: end-to-end dataflow

```
User types a prompt
  ↓
Bobatea chat model calls backend.Start(ctx, prompt)
  ↓
ToolLoopBackend appends a Turn and starts inference (tool loop)
  ↓
Geppetto emits events → WatermillSink.PublishEvent(...) → topic "chat" (JSON)
  ↓
EventRouter handler receives Watermill message
  ↓
agent.MakeUIForwarder(program) decodes events + sends timeline.UIEntity* messages
  ↓
Bubble Tea program receives messages → Bobatea timeline shell updates → UI redraw
  ↓
When tool-loop finishes, backend returns BackendFinishedMsg via tea.Cmd to re-enable input
```

## Mental model glossary (intern-friendly)

This section defines the terms you’ll see in code, and why they exist.

- **Bubble Tea program (`*tea.Program`)**: The runtime loop that receives messages and re-renders the screen.
  - You create it with `tea.NewProgram(model, ...)` and start it with `p.Run()`.
  - It has an important method: `p.Send(msg)` which injects messages into the program from another goroutine.

- **Bubble Tea model (`tea.Model`)**: An object with `Init()`, `Update(msg)`, and `View()`.
  - Bobatea provides a chat model that already knows how to render a timeline + input.

- **Bobatea chat model**: The UI component you embed/use as your main Bubble Tea model.
  - You typically create it with `boba_chat.InitialModel(backend, ...)`.

- **Geppetto event**: A structured event emitted by inference as it progresses.
  - Example event families:
    - “LLM text”: partial completion start/partial/final
    - “tools”: tool call / tool execute / tool result
    - “meta”: logs, agent mode switches, web search progress

- **Watermill message**: A transport envelope around a payload (`[]byte` JSON here).
  - Important: handlers should `Ack()` messages to prevent stalls.

- **Forwarder**: A Watermill handler function that:
  1) decodes JSON → Geppetto event,
  2) maps it to `timeline.UIEntity*` messages,
  3) calls `program.Send(...)`.

## Integration recipe: “agent/tool-loop chat” (recommended for rich TUIs)

This section shows an end-to-end integration skeleton. It is not a “copy/paste works in every repo” snippet (you still need to decide engine/provider config), but it is structured so you can implement it without needing hidden context.

### 1) Decide your topic name

Pick one Watermill topic for UI events and use it consistently.

- In `simple-chat-agent`, the topic is `"chat"` (see `pinocchio/cmd/agents/simple-chat-agent/main.go`).
- In the main Pinocchio CLI chat mode, the topic is `"ui"` (see `pinocchio/pkg/ui/runtime/builder.go`).

For a new integration, pick **one** (usually `"chat"` for agent/tool-loop) and wire:

- `WatermillSink(topic)`
- `EventRouter.AddHandler(topic, ...)`

### 2) Build an EventRouter + sink

Pseudocode:

```go
router, err := events.NewEventRouter() // defaults to in-memory pub/sub
if err != nil { return err }

// All inference events published here:
sink := middleware.NewWatermillSink(router.Publisher, "chat")
```

If you want Redis Streams (durable fan-out), use Pinocchio’s Redis helpers:

- `pinocchio/pkg/redisstream` (see `pinocchio/cmd/agents/simple-chat-agent/main.go`)

### 3) Build the engine and middleware

This depends on your environment (profiles, provider keys, etc.).

At minimum you need:

- a Geppetto `engine.Engine`
- a slice of `middleware.Middleware`

The `simple-chat-agent` example builds these from Glazed sections:

- Engine: `factory.NewEngineFromParsedValues(...)`
- Middleware: a slice including system prompt, agent mode switching, tool result reorder, etc.

Anchor file:

- `pinocchio/cmd/agents/simple-chat-agent/main.go` (search for `mws := []middleware.Middleware{`)

### 4) Create the tool registry (optional but typical)

Tool-loop mode requires a tool registry. If you set `reg=nil`, you’re effectively “simple chat”.

Pseudocode:

```go
registry := tools.NewInMemoryToolRegistry()
// registry.Register(...)  // add tools you want
```

### 5) Create the backend (tool loop runner)

Use the extracted backend:

```go
backend := toolloopbackend.NewToolLoopBackend(eng, mws, registry, sink, hook)
```

Anchor files:

- `pinocchio/pkg/ui/backends/toolloop/backend.go` (backend implementation)
- `pinocchio/cmd/agents/simple-chat-agent/main.go` (how it is used)

### 6) Create the Bobatea chat model and register renderers

You need at least an LLM text renderer, and you probably want tool/log renderers too.

Pseudocode (pattern):

```go
chatModel := boba_chat.InitialModel(backend,
  boba_chat.WithTitle("My Agent Chat"),
  boba_chat.WithTimelineRegister(func(r *timeline.Registry) {
    r.RegisterModelFactory(renderers.NewLLMTextFactory())
    r.RegisterModelFactory(renderers.NewToolCallFactory())
    r.RegisterModelFactory(renderers.ToolCallResultFactory{})
    r.RegisterModelFactory(renderers.LogEventFactory{})
    r.RegisterModelFactory(renderers.WebSearchFactory{})
    r.RegisterModelFactory(agentmode.AgentModeFactory{})
  }),
)
```

Anchors:

- Renderer registration example: `pinocchio/cmd/agents/simple-chat-agent/main.go`
- Bobatea registry hook: `bobatea/pkg/chat/model.go` (look for `WithTimelineRegister`)

### 7) Create the Bubble Tea program

```go
p := tea.NewProgram(chatModel, tea.WithAltScreen())
```

If you have your own layout (sidebar, overlays, etc.), wrap the chat model with a host model (see `pinocchio/cmd/agents/simple-chat-agent/pkg/ui`).

### 8) Register the forwarder handler on the router

This is the “bridge” between inference events and UI updates.

```go
router.AddHandler("ui-forward", "chat", agentforwarder.MakeUIForwarder(p))
```

Important semantic detail:

- The agent forwarder intentionally does **not** send `boba_chat.BackendFinishedMsg{}` on provider final/error/interrupt events.
- The tool-loop backend returns `BackendFinishedMsg` only after the overall loop completes (via the `tea.Cmd` returned from `Start`).

### 9) Run router + UI concurrently and shut down cleanly

Pseudocode:

```go
ctx2, cancel := context.WithCancel(ctx)
defer cancel()
eg, groupCtx := errgroup.WithContext(ctx2)

eg.Go(func() error { return router.Run(groupCtx) })
eg.Go(func() error {
  _, err := p.Run()
  cancel() // stop router when UI exits
  return err
})
return eg.Wait()
```

This pattern avoids:

- router goroutine leaks after UI exits,
- UI continuing to run after router dies,
- hard-to-debug shutdown ordering.

## When to use ChatBuilder instead

If you do **not** need tool-loop/agent entities, Pinocchio already has a “simple chat” reusable builder:

- Builder: `pinocchio/pkg/ui/runtime/builder.go`
- Backend: `pinocchio/pkg/ui/backend.go` (`EngineBackend`)
- Forwarder: `pinocchio/pkg/ui/backend.go` (`StepChatForwardFunc`)

See: `glaze help pinocchio-chatbuilder-guide`

## Troubleshooting

| Problem | Likely cause | What to check / do |
|---|---|---|
| UI never updates (blank timeline) | Forwarder not registered or topic mismatch | Confirm the sink topic matches `AddHandler` topic (e.g. both `"chat"`). |
| UI updates, but input stays “blurred” | No `BackendFinishedMsg` ever sent | Confirm backend’s `Start` returns a `tea.Cmd` that emits `boba_chat.BackendFinishedMsg{}` when finished. |
| Tool calls never show up | Missing renderers or missing tool registry | Ensure tool renderer factories are registered; ensure `registry != nil` and tools are registered. |
| Router handler seems stuck | Watermill messages not ack’d | Forwarder must call `msg.Ack()`; see `pinocchio/pkg/ui/forwarders/agent/forwarder.go`. |
| Lots of “unknown event type” logs | Event decoding mismatch | Check `events.NewEventFromJson` and the event types your engine emits. |

## See Also

- `glaze help pinocchio-chatbuilder-guide` (simple chat integration)
- `glaze help webchat-debugging-and-ops` (debugging patterns that translate well to TUI event flows)
- Agent forwarder implementation: `pinocchio/pkg/ui/forwarders/agent/forwarder.go`
