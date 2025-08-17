## Goal

Refactor `pinocchio/cmd/agents/simple-chat-agent/main.go` to use the new TimelineShell + input field UI, and drive it with a backend that executes the inference tool-calling loop. Keep the existing engine/middleware/tooling architecture and event logging, while replacing the current REPL-centric UI with a chat timeline plus text input.

---

## Current state (what we have)

- **Agent entrypoint**: `pinocchio/cmd/agents/simple-chat-agent/main.go` sets up:
  - Event router + `WatermillSink`, and stdout event logging.
  - Engine via `factory.NewEngineFromParsedLayers(...)` and wraps with:
    - `SystemPromptMiddleware`
    - `agentmode` (static modes with default)
    - RW `sqlitetool` with REGEXP enabled DB
  - Tool registry: `calc` and `generative-ui`.
  - A snapshot store capturing turn snapshots at several phases.
  - A REPL evaluator (`pkg/eval/chat.go`) that runs `toolhelpers.RunToolCallingLoop(...)` and returns last assistant text, used by a `repl.Model` inside an `AppModel` (`pkg/ui/app.go`).
  - `AppModel` integrates: a REPL viewport, event-driven status, a sidebar, and a mechanism to render Huh forms for the `generative-ui` tool via a request channel.

- **New UI components available**:
  - `bobatea/pkg/chat/model.go`: a chat UI model that already embeds a `TimelineShell` (`bobatea/pkg/timeline/shell.go`) and a `textarea` input. It consumes timeline lifecycle messages: `UIEntityCreated`, `UIEntityUpdated`, `UIEntityCompleted`, `UIEntityDeleted`. It requires a `chat.Backend` to start/interrupt/kill inference, and manages input focus, streaming state, selection mode, clipboard helpers, save-to-file, etc. It supports an "external input" mode if we need to host our own input elsewhere.
  - `bobatea/pkg/timeline/shell.go`: viewport + controller that renders entities via registered renderers (e.g., `llm_text`, `tool_calls_panel`).
  - Renderers: `llm_text_model.go`, `tool_calls_panel_model.go`, `plain_model.go`.

- **Event-to-UI bridging**:
  - `pinocchio/pkg/ui/backend.go` provides:
    - `EngineBackend` (simple one-shot `RunInference`, no tool loop).
    - `StepChatForwardFunc(p *tea.Program)`: a router handler that forwards `geppetto` events to timeline UI messages on the Bubble Tea program.

- **Tool loop evaluator**:
  - `pkg/eval/chat.go` shows the correct tool loop: it appends the user block to a running `turns.Turn`, then runs `toolhelpers.RunToolCallingLoop(ctx, engine, turn, registry, ...)`, emitting events via the `events.WithEventSinks(ctx, sink)` context when provided.

---

## Target architecture (what we want)

1. Replace REPL-based UI with the chat model:
   - Use `bobatea/pkg/chat.InitialModel(backend, ...)` with timeline renderers registered, and the built-in input widget.
   - Keep the sidebar as an optional right panel (if desired) or port its bits later.

2. Implement a backend that runs the inference tool-calling loop:
   - New `ToolLoopBackend` implementing `bobatea/pkg/chat.Backend` that encapsulates:
     - `engine.Engine`
     - `*tools.InMemoryToolRegistry`
     - `*middleware.WatermillSink`
     - snapshot hook
     - current `*turns.Turn` (carried over across messages)
   - On `Start(ctx, prompt)`: append user block to the in-flight turn, set up `events.WithEventSinks(ctx, sink)` and snapshot hook in context, then `toolhelpers.RunToolCallingLoop(...)` with timeout and iteration limits. When finished, send `BackendFinishedMsg`.
   - Use `StepChatForwardFunc(program)` to translate event stream into timeline UI messages (assistant streaming, etc.).

3. Keep existing engine/middleware setup in `main.go`:
   - `SystemPromptMiddleware` and `agentmode` stay.
   - `sqlitetool` with REGEXP stays.
   - Tool registry: `calc`, `generative-ui` stay.
   - Snapshot storage stays (pre/post middleware + explicit phases via hook during tool loop).

4. Preserve event logging and persistence:
   - Keep stdout logs for tools and info/log events.
   - Keep handler that writes events to SQLite store.

5. Tool UI forms integration:
   - The current `AppModel` renders Huh forms for `generative-ui` via a channel. The chat model does not.
   - Introduce a small overlay-form controller that listens to a `ToolUIRequest` channel and, when present, blurs the chat input (`BlurInputMsg`), runs a Huh form model, then unblurs and replies on completion.
   - This overlay can be composed around the chat model using a thin wrapper `tea.Model` (see plan below), preserving the chat model unchanged.

---

## Proposed refactor steps

### 1) Introduce a backend that runs the tool loop

- New file (suggested): `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`.
- Responsibilities:
  - Hold `engine`, `registry`, `sink`, `snapshotHook`, and a mutable `*turns.Turn`.
  - Implement `Start(ctx, prompt) (tea.Cmd, error)`:
    - Append user text block to the current turn (initializing turn if nil).
    - Build `runCtx := events.WithEventSinks(ctx, sink)`, and if present `toolhelpers.WithTurnSnapshotHook(runCtx, hook)`.
    - Call `toolhelpers.RunToolCallingLoop(runCtx, engine, turn, registry, ToolConfig{...})`.
    - Update internal `turn` with the returned value.
    - Return a command that emits `chat.BackendFinishedMsg{}` to the program.
  - Implement `Interrupt`, `Kill`, `IsFinished` similarly to `EngineBackend` (with a cancellable context).

Pseudo-structure (refer to `pkg/eval/chat.go` for the loop):

```go
type ToolLoopBackend struct {
  eng engine.Engine
  reg *tools.InMemoryToolRegistry
  sink *middleware.WatermillSink
  hook toolhelpers.SnapshotHook
  turn *turns.Turn
  cancel context.CancelFunc
  running atomic.Bool
}

func (b *ToolLoopBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
  if !b.running.CompareAndSwap(false, true) { return nil, errors.New("already running") }
  if b.turn == nil { b.turn = &turns.Turn{Data: map[string]any{}} }
  turns.AppendBlock(b.turn, turns.NewUserTextBlock(prompt))
  ctx, b.cancel = context.WithCancel(ctx)
  runCtx := events.WithEventSinks(ctx, b.sink)
  if b.hook != nil { runCtx = toolhelpers.WithTurnSnapshotHook(runCtx, b.hook) }
  return func() tea.Msg {
    t2, err := toolhelpers.RunToolCallingLoop(runCtx, b.eng, b.turn, b.reg, toolhelpers.NewToolConfig().WithMaxIterations(5).WithTimeout(60*time.Second))
    b.running.Store(false); b.cancel = nil
    if err == nil && t2 != nil { b.turn = t2 }
    return chat.BackendFinishedMsg{}
  }, nil
}
```

### 2) Wire event forwarding from router to UI

- After creating the Bubble Tea program, add a handler on the event router to forward `geppetto` events into timeline UI messages:

```go
p := tea.NewProgram(model, tea.WithAltScreen())
router.AddHandler("ui-forward", "chat", ui.StepChatForwardFunc(p)) // uses pinocchio/pkg/ui/backend.go
```

This preserves the existing event publishing via `WatermillSink` and keeps the chat model in sync with the inference stream.

### 3) Replace the REPL UI with chat model + TimelineShell

- Build the chat model in `main.go`:
  - Create `timeline.Registry`, register renderers (`llm_text`, `tool_calls_panel`, `plain`).
  - Use `chat.InitialModel(backend, chat.WithTimelineRegister(func(reg *timeline.Registry){ /* register */ }))`.
  - Optionally: leverage `WithExternalInput(false)` to keep the built-in input.

Minimal construction sketch:

```go
backend := backendpkg.NewToolLoopBackend(eng, registry, sink, snapshotHook)
model := chat.InitialModel(backend,
  chat.WithTitle("Chat REPL"),
  chat.WithTimelineRegister(func(r *timeline.Registry) {
    r.RegisterModelFactory(renderers.NewLLMTextFactory())
    r.RegisterModelFactory(renderers.ToolCallsPanelFactory{})
    r.RegisterModelFactory(renderers.PlainFactory{})
  }),
)

p := tea.NewProgram(model, tea.WithAltScreen())
```

Note: the chat model already manages input focus/blur and streaming transitions; it calls `backend.Start(...)` on submit and expects `BackendFinishedMsg`.

### 4) Keep snapshot store and hook

- Retain current snapshot store logic from `main.go` and the wrapper middleware that saves snapshots on pre/post middleware; also pass a `hook` (as done in the existing evaluator path) into the backend to capture `pre_inference`, `post_inference`, `post_tools` phases during the tool loop.

### 5) Preserve tool registry and `generative-ui` form support

- Continue registering tools on the in-memory registry.
- Add a small overlay form component to handle `toolspkg.ToolUIRequest` similar to `ui/app.go`:
  - A wrapper `tea.Model` that wraps the chat model, and in its `Init` subscribes to a `toolReqCh`.
  - When a request arrives:
    - Send `chat.BlurInputMsg` to the inner chat model.
    - Mount a `huh.Form` model until completion; collect values and reply via the provided channel.
    - Send `chat.UnblurInputMsg` afterwards.
  - This keeps the chat model untouched and provides parity with the previous `AppModel` form integration.

Pseudo-structure:

```go
type OverlayModel struct {
  inner chat.Model
  toolReq <-chan toolspkg.ToolUIRequest
  active *huh.Form
  vals map[string]any
  reply chan toolspkg.ToolUIReply
}
// Init subscribes to toolReq; Update routes to active form or inner; when form completes, replies and unblurs.
```

### 6) Sidebar (optional, phase 2)

- Port the existing `SidebarModel` (`pkg/ui/sidebar.go`) as an optional right panel.
- This can be implemented by composing the `OverlayModel` to render a split layout (left: chat.View, right: sidebar.View) and forwarding `events.EventLog/EventInfo` so the sidebar tracks agent mode and calculator mini-log as before.

---

## Detailed changes in `main.go`

1) Keep existing sections (unchanged):
   - Viper/logger setup, help system.
   - Event router creation and stdout logging handler.
   - Engine construction and middlewares (`SystemPrompt`, `agentmode`, `sqlitetool`).
   - Tool registry registration (calc and generative-ui).
   - Snapshot store and pre/post snapshotting middleware.
   - Event persistence handler into SQLite.

2) Replace REPL/evaluator/app-model with chat model:
   - Remove `repl.NewModel(...)` and `uipkg.NewAppModel(...)` from `pkg/ui/app.go`.
   - Create `ToolLoopBackend` with `eng`, `registry`, `sink`, `snapshotHook`.
   - Build `chat.InitialModel(backend, ...)` with `timeline` renderers registered.
   - Construct `OverlayModel` to host `generative-ui` forms and (optionally) the sidebar.
   - Create the Bubble Tea `Program` with the overlay model; pass program to `StepChatForwardFunc` on the event router.

3) Concurrency wiring:
   - Keep `errgroup` to run router and UI concurrently.
   - Ensure the router handler for UI forwarding is installed before starting the UI program (or install after creating `Program` but before awaiting `Running()`).

4) Seeding prior context (optional):
   - If desired, seed the chat timeline with prior blocks using a helper similar to `EngineBackend.SetSeedTurn` that emits `UIEntityCreated/Completed` for existing user/assistant text blocks.

---

## Migration checklist (TODO)

1. Backend and tool loop
   - [ ] Add `ToolLoopBackend` implementing `chat.Backend` (new file under `pkg/backend`).
   - [ ] Use `toolhelpers.RunToolCallingLoop` with `events.WithEventSinks(ctx, sink)`.
   - [ ] Carry forward `*turns.Turn` state between submissions.

2. UI model
   - [ ] Replace REPL with `chat.InitialModel` configured with timeline renderers.
   - [ ] Wire `BlurInputMsg`/`UnblurInputMsg` handling into overlay for form display.

3. Event routing
   - [ ] Add router handler `StepChatForwardFunc(program)` to map events to timeline UI lifecycle.
   - [ ] Keep stdout logging and SQLite event persistence handlers.

4. Tools
   - [ ] Keep `calc` and `generative-ui` registration on the same registry instance used by the backend loop.
   - [ ] Implement overlay form controller that listens on `ToolUIRequest` channel and replies with `ToolUIReply`.

5. Snapshots and storage
   - [ ] Preserve snapshot store init and pre/post snapshot middleware.
   - [ ] Pass snapshot hook to backend so `pre_inference/post_inference/post_tools` phases are captured.

6. Optional
   - [ ] Port/compose `SidebarModel` on the right pane.
   - [ ] Implement lightweight `/dbg` inputs by detecting `/dbg ...` in user input and rendering output as a `plain` entity.

---

## Open questions / risks

- The existing `pinocchio/pkg/ui/EngineBackend` does not run the tool loop; we will implement `ToolLoopBackend` instead, using the same pattern as `pkg/eval/chat.go`.
- `generative-ui` tool UI: we need a small overlay wrapper to display Huh forms on top of the chat model; ensure it doesn’t starve the event loop, and blur the input during form entry.
- Ensure `events.WithEventSinks(ctx, sink)` is consistently used when the backend runs the tool loop; otherwise the router→UI forwarder will have nothing to forward.
- Order of initialization for router and UI program matters; install the UI-forwarder handler once the program exists.

---

## Minimal code mapping (pseudocode snippets)

1) Registry and tools (unchanged):

```go
reg := tools.NewInMemoryToolRegistry()
toolspkg.RegisterCalculatorTool(reg)
toolReqCh := make(chan toolspkg.ToolUIRequest, 4)
toolspkg.RegisterGenerativeUITool(reg, toolReqCh)
```

2) Backend:

```go
backend := backendpkg.NewToolLoopBackend(eng, reg, sink, snapshotHook)
```

3) Chat model with renderers:

```go
chatModel := boba_chat.InitialModel(backend,
  boba_chat.WithTitle("Chat REPL"),
  boba_chat.WithTimelineRegister(func(r *timeline.Registry) {
    r.RegisterModelFactory(renderers.NewLLMTextFactory())
    r.RegisterModelFactory(renderers.ToolCallsPanelFactory{})
    r.RegisterModelFactory(renderers.PlainFactory{})
  }),
)
```

4) Overlay model (form integration) and program:

```go
overlay := NewOverlayModel(chatModel, toolReqCh) // handles Blur/Unblur and Huh form
p := tea.NewProgram(overlay, tea.WithAltScreen())
router.AddHandler("ui-forward", "chat", ui.StepChatForwardFunc(p))
```

5) Run concurrently as before with `errgroup`.

---

## Outcome

This refactor keeps the engine/middleware/tooling and event logging intact, switches the UI to TimelineShell + input (via the chat model), and introduces a purpose-built backend that drives the tool-calling loop while forwarding events into timeline entities. It also preserves the generative UI tool experience with a small overlay and leaves room to add the sidebar back as a follow-up.


