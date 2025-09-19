# Writing a Custom Agent Backend from Scratch (with Geppetto/Pinocchio)

> Audience: New contributors who want to build their own agent applications.
> Goal: Scaffold a minimal but production-ready backend that composes a provider-agnostic engine, custom middlewares, and a set of domain-specific tools (actions). Use `pinocchio/cmd/agents/simple-chat-agent/main.go` as a practical reference.

## 1) Core Concepts (10,000‑ft view)

- **Turn**: Immutable-ish unit of work passed to the engine, holding ordered blocks (`llm_text`, `tool_call`, `tool_use`) and metadata. See `geppetto/pkg/turns`.
- **Engine**: Provider adapter that implements `RunInference(ctx, *turns.Turn) (*turns.Turn, error)`. It maps model IO to Turn blocks and emits streaming events when a sink is configured. See `geppetto/pkg/inference/engine` and `.../factory`.
- **Middleware**: Composable wrappers adding cross‑cutting behavior around `RunInference`. Typical uses: logging, agent mode selection, tool execution orchestration, data injection. See `geppetto/pkg/inference/middleware` and the tutorial in `geppetto/pkg/doc/topics/09-middlewares.md`.
- **Tools (Actions)**: Functions the model can call with structured inputs. Tools are attached per‑Turn via `Turn.Data`, advertised to the provider, and executed by middleware or helpers. See `geppetto/pkg/doc/topics/07-tools.md`.
- **Events / Sink**: Engines and helpers publish `events.Event` to a channel (e.g. Watermill). These drive UIs, logs, and persistence.

A minimal backend wires: router + sink → engine → middlewares → tool registry → run loop.

## 2) The Reference Blueprint

The reference `simple-chat-agent` shows a complete wiring from CLI to TUI:

- Creates a Watermill router and sink, optionally backed by Redis Streams
- Instantiates an engine from Glazed layers (`factory.NewEngineFromParsedLayers`)
- Composes middlewares: system prompt, agent mode, tool-result reordering, SQLite bridge, stable IDs, snapshot logging
- Registers tools: calculator, a simple “generative UI” tool (demonstrates tool → UI handshakes)
- Runs a Bubble Tea chat UI that renders timeline events

Keep it open while you implement your custom backend.

## 3) Scaffolding the Command

Create a Cobra/Glazed command. This gives consistent flags, logging, and config layering.

```go
// main.go
root := &cobra.Command{Use: "my-agent", PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
    return logging.InitLoggerFromViper()
}}
helpSystem := help.NewHelpSystem()
help_cmd.SetupCobraRootCommand(helpSystem, root)
_ = clay.InitViper("pinocchio", root)

cmdDesc := cmds.NewCommandDescription(
    "my-agent",
    cmds.WithShort("Custom agent with middlewares and tools"),
    cmds.WithLayersList(geppettolayers.CreateGeppettoLayersMust()),
)

// Implement RunIntoWriter(ctx, parsed, w) on your command and add it to root.
```

Tip: Use `geppettolayers.CreateGeppettoLayers()` to let users select providers/models via flags/env.

## 4) Event Router and Sink

Real‑time streaming is core to the developer experience. The reference uses Watermill and supports Redis Streams.

```go
rs := rediscfg.Settings{}
_ = parsed.InitializeStruct("redis", &rs)
router, err := rediscfg.BuildRouter(rs, false)
if err != nil { return errors.Wrap(err, "router") }

sink := middleware.NewWatermillSink(router.Publisher, "chat")
// Add handlers for logging and persistence as needed
```

You can add a simple log handler that translates `events.Event` into structured logs.

## 5) Create the Engine (Provider‑agnostic)

Use the factory to instantiate the engine. This keeps your backend provider‑agnostic.

```go
e, err := factory.NewEngineFromParsedLayers(parsed)
if err != nil { return errors.Wrap(err, "engine") }
```

Attach the sink by wrapping or passing options at creation time (see `06-inference-engines.md`).

## 6) Compose Middlewares

Middlewares add behavior without coupling to a specific provider. Pick a few that matter for your domain.

### 6.1 System Prompt Middleware

Ensure a consistent system instruction at the start of each run.

```go
e = middleware.NewEngineWithMiddleware(e,
    middleware.NewSystemPromptMiddleware("You are a <role>. Focus on <outcomes>."),
)
```

### 6.2 Agent Mode Middleware (example)

Switches the agent’s persona, instructions, and allowed tools via a lightweight service.

```go
svc := agentmode.NewStaticService([]*agentmode.AgentMode{
    {Name: "data_cleaner", Prompt: "You clean and normalize tabular data. Propose edits; never write."},
    {Name: "sql_reviewer", Prompt: "You review SQL. Suggest changes; avoid DML."},
})
cfg := agentmode.DefaultConfig()
cfg.DefaultMode = "sql_reviewer"

e = middleware.NewEngineWithMiddleware(e, agentmode.NewMiddleware(svc, cfg))
```

### 6.3 Tool Result Reorder Middleware

Keeps `tool_use` blocks adjacent to their originating `tool_call` for cleaner timelines.

```go
e = middleware.NewEngineWithMiddleware(e, middleware.NewToolResultReorderMiddleware())
```

### 6.4 SQLite Tool Bridge (optional)

Expose a read‑only SQL tool backed by SQLite, with `REGEXP` support.

```go
db, _ := sqlite_regexp.OpenWithRegexp("my-data.db")
e = middleware.NewEngineWithMiddleware(e, sqlitetool.NewMiddleware(sqlitetool.Config{DB: db, MaxRows: 500}))
```

### 6.5 Stable IDs + Snapshot Persistence (custom)

Attach deterministic Run/Turn IDs and persist pre/post snapshots for debugging or audit.

```go
sessionRunID := uuid.NewString()
e = middleware.NewEngineWithMiddleware(e,
    func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
            if t == nil { t = &turns.Turn{} }
            if t.RunID == "" { t.RunID = sessionRunID }
            if t.ID == "" { t.ID = uuid.NewString() }
            return next(ctx, t)
        }
    },
)

// Wrap with a snapshot logger (persist to DB or files)
store := mustOpenSnapshotStore("my-agent.db")
e = middleware.NewEngineWithMiddleware(e,
    func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
            _ = store.SaveTurnSnapshot(ctx, t, "pre_middleware")
            res, err := next(ctx, t)
            if res != nil { _ = store.SaveTurnSnapshot(ctx, res, "post_middleware") }
            return res, err
        }
    },
)
```

### 6.6 Redaction Middleware (custom)

Guardrails that redact secrets in user/assistant blocks before they reach the provider.

```go
redact := func(next middleware.HandlerFunc) middleware.HandlerFunc {
    return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
        for i := range t.Blocks {
            b := &t.Blocks[i]
            if b.Type == turns.BlockTypeLLMText || b.Type == turns.BlockTypeUserText {
                txt := turns.GetTextPayload(b)
                txt = redactSecrets(txt)
                turns.SetTextPayload(b, txt)
            }
        }
        return next(ctx, t)
    }
}

e = middleware.NewEngineWithMiddleware(e, redact)
```

See `09-middlewares.md` for more patterns and guidance.

## 7) Define Tools (Actions)

Register domain actions in a per‑Turn registry, using JSON Schema auto‑inferred from Go functions.

```go
// 7.1 Calculator (toy, but great for validation)
type AddReq struct { A, B float64 `json:"a" jsonschema:"required"` }
type AddRes struct { Sum float64 `json:"sum"` }
func addTool(req AddReq) AddRes { return AddRes{Sum: req.A + req.B} }

reg := tools.NewInMemoryToolRegistry()
addDef, _ := tools.NewToolFromFunc("add", "Add two numbers", addTool)
_ = reg.RegisterTool("add", *addDef)

// 7.2 HTTP GET JSON (useful in many agents)
type FetchReq struct { URL string `json:"url" jsonschema:"required,format=uri"` }
type FetchRes struct { Status int `json:"status"`; Body string `json:"body"` }
func fetchJSON(ctx context.Context, req FetchReq) (FetchRes, error) {
    r, err := http.Get(req.URL)
    if err != nil { return FetchRes{}, err }
    defer r.Body.Close()
    b, _ := io.ReadAll(r.Body)
    return FetchRes{Status: r.StatusCode, Body: string(b)}, nil
}
fetchDef, _ := tools.NewToolFromFunc("http_get_json", "Fetch JSON from a URL", fetchJSON)
_ = reg.RegisterTool("http_get_json", *fetchDef)

// 7.3 Domain tool example: classify a transaction
type ClassifyReq struct { Description string `json:"description" jsonschema:"required"` }
type ClassifyRes struct { Category string `json:"category"` }
func classifyTransaction(req ClassifyReq) ClassifyRes { return ClassifyRes{Category: naiveCategory(req.Description)} }
clsDef, _ := tools.NewToolFromFunc("classify_transaction", "Classify a transaction description", classifyTransaction)
_ = reg.RegisterTool("classify_transaction", *clsDef)
```

Attach the registry to the Turn before calling the engine. See `07-tools.md` for per‑Turn attachment and helper loops.

## 8) Run the Tool Loop

You can either:

- Use a backend helper that runs a turn‑native tool loop and emits timeline events, or
- Use `toolhelpers.RunToolCallingLoop` for a conversation‑first flow.

A common pattern is to wrap the engine with your middlewares and hand it to a small backend that:

- Attaches the `reg` registry and tool config to each Turn
- Calls the engine until no pending tool calls remain (respecting max iterations/timeouts)
- Publishes tool/log/info events to `sink` so UIs and loggers stay in sync

If you’re building a TUI, the reference `simple-chat-agent` composes a Bubble Tea model that subscribes to events and renders a timeline.

## 9) Putting It Together (Skeleton)

```go
func (c *MyAgentCmd) RunIntoWriter(ctx context.Context, parsed *layers.ParsedLayers, _ io.Writer) error {
    rs := rediscfg.Settings{}
    _ = parsed.InitializeStruct("redis", &rs)
    router, err := rediscfg.BuildRouter(rs, false)
    if err != nil { return errors.Wrap(err, "router") }

    sink := middleware.NewWatermillSink(router.Publisher, "chat")

    e, err := factory.NewEngineFromParsedLayers(parsed)
    if err != nil { return errors.Wrap(err, "engine") }

    e = middleware.NewEngineWithMiddleware(e,
        middleware.NewSystemPromptMiddleware("You are a helpful <role>."),
        agentModeMw(),
        middleware.NewToolResultReorderMiddleware(),
        redactMw(),
    )

    reg := buildToolRegistry()

    // Optional: snapshot store, UI forwarders, additional handlers
    // Start router and your UI or HTTP server here

    // Kick off your run loop using the sink, engine, and reg
    return runLoop(ctx, router, sink, e, reg)
}
```

## 10) Choosing Your Middlewares and Actions (Examples)

- **Data team agent**
  - Middlewares: system prompt describing analysis scope; agent mode toggles for “explorer” vs “reviewer”; SQLite tool bridge (read‑only); result reordering; redaction
  - Tools: `sql_preview` (COUNT + sample), `http_get_json` (metadata), `classify_transaction`

- **Integration agent**
  - Middlewares: system prompt with safety; rate limiting; redaction
  - Tools: `http_get_json`, `post_webhook`, `transform_payload`

- **Docs assistant**
  - Middlewares: system prompt injecting doc context; mode for “authoring” vs “review only”
  - Tools: `search_docs`, `summarize`, `open_issue`

## 11) Debugging and Testing

- Add a log handler on the router that parses `events.Event` and logs the type/payload.
- Persist pre/post snapshots of `*turns.Turn` to SQLite and inspect them when behavior seems odd.
- Unit test: middleware behavior (inputs → outputs), tool functions (pure), prompt builders.
- Integration test: start the router with a mock engine that appends canned blocks and verify emitted events.

## 12) Production Notes

- Keep middlewares stateless when possible; prefer writing to `*turns.Turn` over hidden globals.
- Cap iterations/timeouts to avoid infinite tool loops.
- Avoid leaking sensitive data in events; redact early.
- Prefer per‑Turn tool registries to keep engines provider‑focused and testable.

## 13) References and Further Reading

- `pinocchio/cmd/agents/simple-chat-agent/main.go` — end‑to‑end wiring blueprint
- `geppetto/pkg/doc/topics/06-inference-engines.md` — engines, factories, streaming
- `geppetto/pkg/doc/topics/09-middlewares.md` — middleware patterns and best practices
- `geppetto/pkg/doc/topics/07-tools.md` — defining and executing tools per Turn
- `pinocchio/cmd/web-chat/*` — reference for HTTP/WebSocket backends and semantic event forwarding

---

With these building blocks, you can spin up a focused agent for your domain by composing the right middlewares and registering a small set of well‑scoped tools. Start from the skeleton above, then iterate: add one middleware or tool at a time, verify via events and snapshots, and keep engines free of orchestration logic.
