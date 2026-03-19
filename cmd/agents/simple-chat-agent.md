---
Title: Simple Chat Agent with Streaming, Tools, and a Tiny REPL
Slug: pinocchio-simple-chat-agent
Short: A minimal Cobra command that streams model output, pretty-prints tool calls/results with Lipgloss, and offers a tiny REPL.
Topics:
- pinocchio
- geppetto
- tutorial
- inference
- streaming
- tools
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

### Overview

Build a minimal yet production-ready chat agent that streams model output, supports tool calling, and renders pretty, readable events in the terminal. The agent includes a tiny REPL for iterative prompts.

### Audience & Outcome

- You already use Geppetto/Pinocchio and want a clean reference agent.
- You’ll get a working `simple-chat-agent` command with streaming, tool-calling, and pretty event output.

### Key Features

- Engine-first architecture (provider-agnostic)
- Streaming via Watermill-backed event router
- Tool calling with a simple calculator tool (`calc`)
- Pretty event output using Charmbracelet Lipgloss
- Tiny REPL (`:q` to quit)

### Prerequisites

- Go 1.24+
- Pinocchio profiles configured (providers/models) — Geppetto layers will pick these up automatically

### Quick Start

From the `pinocchio` module root:

```sh
go run ./cmd/agents/simple-chat-agent
```

Type your prompts at `>`. Use `:q` to quit.

### How It Works

- Creates a Watermill-backed `EventRouter`, adds a pretty-print handler
- Creates an engine from Geppetto layers (no engine sink to avoid duplicate events)
- Registers a `calc` tool and (optionally) configures tools on the engine
- Uses `session.Session` + `toolloop/enginebuilder` to orchestrate inference + tool execution
- Attaches the same sink via `enginebuilder.WithEventSinks(...)` so engine/tools publish events
- REPL appends prompts as new Turns; results stream to stdout

### Detailed Explanation

Root command and configuration
- A `cobra.Command` root initializes logging with `logging.InitLoggerFromViper()` in `PersistentPreRunE` so logs are consistent across subcommands.
- `clay.InitViper("pinocchio", root)` wires Viper to load Pinocchio/Geppetto profiles and settings. This means provider/model are picked up without extra flags.
- The command is constructed via `cli.BuildCobraCommand(..., geppetto layers middlewares)` so Geppetto’s configuration layers are applied automatically.

Event router and sinks
- A Watermill-backed `events.EventRouter` is started in the background. Handlers are added with `router.AddHandler(name, topic, handler)`.
- A `middleware.NewWatermillSink(router.Publisher, "chat")` sink is created and attached to the context using `events.WithEventSinks(ctx, sink)` so helpers/tools can publish.
- To prevent duplicate events, the engine is created without an engine-level sink; only context-carried sinks are used.

Engine initialization
- The engine is created with `factory.NewEngineFromParsedLayers(parsedLayers)`. This is provider-agnostic and reads settings from Geppetto layers.
- If the selected engine supports `ConfigureTools`, the registered tools are provided as schema definitions (name/description/parameters) using `engine.ToolDefinition`.

Tool registry and configuration
- Tools live in a `tools.ToolRegistry` (here an in-memory one). The sample registers a `calc` tool via `tools.NewToolFromFunc("calc", ..., calculatorFn)`.
- Tool orchestration is not handled by the engine. Instead, the tool loop (`toolloop.Loop`) extracts tool calls from Turn blocks, executes tools locally, appends `tool_use` blocks, and iterates until completion.

Conversation manager
- A conversation is prepared through `builder.NewManagerBuilder()` (e.g., setting a system prompt). The manager gives access to the current `conversation.Conversation` and appends new messages.

REPL loop and orchestration
- The REPL reads user input from stdin; `:q` exits.
- Each user prompt is appended as a new Turn (see `session.AppendNewTurnFromUserPrompt(...)`).
- `session.StartInference(ctx)` runs the complete loop: inference → tool_call → tool execution → tool_use → repeat (bounded by `toolloop.LoopConfig`).

Event rendering
- The pretty-print handler uses `events.NewEventFromJson` to parse `*message.Message` payloads into events.
- It renders: `EventPartialCompletionStart` (started), `EventPartialCompletion` (delta), `EventFinal` (finished), provider-side `EventToolCall`/`EventToolResult` (if any), and helper-side `EventToolCallExecute`/`EventToolCallExecutionResult`.

#### Run

– Ensure your Pinocchio profiles are configured for your provider/model
– Build and run from the `pinocchio` module root (layers load provider/model)
– Type messages at the REPL; `:q` to quit

To try the calculator tool, ask the model something like:

- "What is 7 mul 6? You may use the calc tool."
- Or explicitly: "Use the calc tool with a=7, b=6, op=mul and tell me the result."

Tool calls and results will be printed with Lipgloss-styled blocks.

#### Notes

- Prefer a single publishing path to avoid duplicate events. This example publishes only via context-carried sink; the engine is created without `engine.WithSink(...)`.
- The tool loop orchestrates tool calling; engines focus on provider I/O.
- Event types rendered: start, partial, final, provider `tool-call`, provider `tool-result` (if any), helper `tool-call-execute`, helper `tool-call-execution-result`.
- Event type definitions: see `geppetto/pkg/events/chat-events.go`.

### APIs Used

- CLI and configuration
  - `cobra.Command`, `PersistentPreRunE` with `logging.InitLoggerFromViper()`
  - `clay.InitViper("pinocchio", root)` for profile/config loading
  - `cli.BuildCobraCommand(..., geppetto layers middlewares)`
- Events and streaming
  - `events.NewEventRouter()`, `AddHandler(name, topic, handler)`
  - `middleware.NewWatermillSink(router.Publisher, "chat")`
  - `events.WithEventSinks(ctx, sink)`, `events.PublishEventToContext(...)`
- Engine and tools
  - `factory.NewEngineFromParsedLayers(parsedLayers)`
  - `tools.NewInMemoryToolRegistry()`, `tools.NewToolFromFunc(...)`, `RegisterTool(...)`
  - Optional: `engine.ConfigureTools(defs, engine.ToolConfig{ Enabled: true })`
- Conversation and orchestration
  - `session.NewSession()`, `session.AppendNewTurnFromUserPrompt(...)`
  - `enginebuilder.New(...)` (toolloop/enginebuilder)
  - `session.StartInference(ctx)` / `handle.Wait()`

### Pseudocode (sketch)

```text
main:
  root := cobra.Command{ Use: "simple-chat-agent", PersistentPreRunE: logging.InitLoggerFromViper }
  clay.InitViper("pinocchio", root)
  cmd := NewSimpleAgentCmd() // description with Geppetto layers
  root.AddCommand(cli.BuildCobraCommand(cmd, GeppettoMiddlewares))
  root.Execute()

NewSimpleAgentCmd.RunIntoWriter(ctx, parsedLayers, w):
  router := events.NewEventRouter()
  sink := middleware.NewWatermillSink(router.Publisher, "chat")
  router.AddHandler("pretty", "chat", prettyPrinter(w))

  engine := factory.NewEngineFromParsedLayers(parsedLayers) // no engine sink (avoid duplicates)

  registry := tools.NewInMemoryToolRegistry(); registry.RegisterTool("calc", tools.NewToolFromFunc(...))
  if engine supports ConfigureTools: engine.ConfigureTools(defs, {Enabled:true})

  manager := builder.NewManagerBuilder().WithSystemPrompt("You are a helpful assistant. You can use tools.").Build()

  run router in background
  loop:
    read line from stdin (":q" to quit)
    sess.AppendNewTurnFromUserPrompt(line)
    handle := sess.StartInference(ctx)
    handle.Wait()

prettyPrinter(w):
  switch e := events.NewEventFromJson(msg.Payload).(type):
    EventPartialCompletionStart → print "started"
    EventPartialCompletion → print e.Delta
    EventFinal → print "finished"
    EventToolCall → print Name + ID + Input (pretty JSON)
    EventToolCallExecute → print Name + ID + Input (pretty JSON)
    EventToolResult → print ID + Result (if provider emits)
    EventToolCallExecutionResult → print ID + Result (pretty JSON)
```

### Troubleshooting

- Duplicate events: ensure the engine is created without `engine.WithSink(...)` if you attach sinks via context.
- No tool result shown: confirm your provider supports tool-calling metadata and that the sink is wired (via `enginebuilder.WithEventSinks(...)` or `events.WithEventSinks(ctx, sink)`).
- Tools not called: verify `ConfigureTools` is applied when the engine supports it, and that the registry contains your tools.

### References

- Topics: `geppetto-inference-engines`, `geppetto-events-streaming-watermill`
- Tutorial: `geppetto-streaming-inference-tools`
