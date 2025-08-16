Title: Refactoring Approaches for a Unified Chat UI Architecture (brainstorm)

Audience: Maintainers and contributors exploring ways to simplify and unify Pinocchio’s chat UI architecture around Geppetto engines and bobatea timeline.

Outcome: A set of alternative designs (with pros/cons) for unifying orchestration code, making the backend reusable, simplifying immediate UI responsiveness, removing legacy non-timeline messages, and reorganizing the chat model to become a reusable chat timeline for agents.

---

## 1) Who uses `ChatSession` today? Can/should it be unified with `PinocchioCommand`?

Scan results suggest `ChatSession` (in `pinocchio/pkg/chatrunner/chat_runner.go`) is primarily a general orchestrator that mirrors the CLI path but is not directly invoked from `pinocchio/pkg/cmds/cmd.go`. The CLI wiring (`runChat`) performs its own setup: it creates an `events.EventRouter`, builds an engine with `engine.WithSink(ui)`, instantiates `ui.EngineBackend`, calls `bobatea/pkg/chat.InitialModel`, registers `ui.StepChatForwardFunc` as a handler, then seeds with `SetSeedTurn` and optionally auto-submits. Functionally, this matches what `ChatSession.runChatInternal()` does. In other words, both code paths maintain nearly identical orchestration logic (router lifecycle, sink creation, backend, program, handler installation, seeding, auto-start). As of now, PinocchioCommand does not call `ChatSession`; historically it may have, but the current `runChat` executes a standalone flow.

Conclusion: There is duplicative orchestration. Either `ChatSession` should become the single orchestrator invoked by the CLI, or the CLI path should be abstracted and reused by `ChatSession`. A unification would reduce drift and simplify maintenance.

### 1B) Autosubmit without passing via builder/session

You can autosubmit the first prompt entirely outside the builder/session by sending messages into the Bubble Tea program after the router is ready. This avoids adding autosubmit concerns to `ChatBuilder`/`ChatSession`.

Pseudocode (works today with `cmd.go` structure):
```go
p := tea.NewProgram(model, options...)
router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
_ = router.RunHandlers(ctx)

go func() {
  <-router.Running()                 // wait until handlers are ready
  p.Send(chat.ReplaceInputTextMsg{Text: firstPrompt})
  p.Send(chat.SubmitMessageMsg{})
}()

_, _ = p.Run()
```
This pattern can be used by any caller (CLI or embedding app) and keeps autosubmit as a caller concern, not a builder API.

### 1B + 2) Clean shared API for builder/session (CLI and embedding)

Design a single API that supports both the CLI use case (standalone chat program) and embedding chat into an existing Bubble Tea program.

Proposed types and methods:
```go
// pinocchio/pkg/chatrunner or pinocchio/pkg/ui/runtime
// Unified builder
 type ChatBuilder struct {
   // required
   ctx context.Context
   engineFactory factory.EngineFactory
   settings *settings.StepSettings
   // optional
   router *events.EventRouter
   programOptions []tea.ProgramOption
   modelOptions []boba_chat.ModelOption
   seedTurn *turns.Turn
 }

 func NewChatBuilder() *ChatBuilder
 func (b *ChatBuilder) WithContext(ctx context.Context) *ChatBuilder
 func (b *ChatBuilder) WithEngineFactory(f factory.EngineFactory) *ChatBuilder
 func (b *ChatBuilder) WithSettings(s *settings.StepSettings) *ChatBuilder
 func (b *ChatBuilder) WithRouter(r *events.EventRouter) *ChatBuilder
 func (b *ChatBuilder) WithProgramOptions(opts ...tea.ProgramOption) *ChatBuilder
 func (b *ChatBuilder) WithModelOptions(opts ...boba_chat.ModelOption) *ChatBuilder
 func (b *ChatBuilder) WithSeedTurn(t *turns.Turn) *ChatBuilder

 // Standalone mode: returns a ready-to-run program, plus accessors
 func (b *ChatBuilder) BuildProgram() (*ChatSession, *tea.Program, error)

 // Embedding mode: returns components to integrate into an existing program
 // - chat model to embed into a parent model
 // - backend
 // - handler to register on the router (StepChatForwardFunc equivalent)
 func (b *ChatBuilder) BuildComponents() (*ChatSession, tea.Model, boba_chat.Backend, func(*message.Message) error, error)

 // Accessors
 type ChatSession struct {
   Router *events.EventRouter
   Backend *ui.EngineBackend
   Handler func(*message.Message) error // bound handler for "ui" topic
 }
 func (cs *ChatSession) EventHandler() func(*message.Message) error // returns Handler
```

Usage examples:

- Standalone (CLI-style):
```go
sess, prog, _ := NewChatBuilder().
  WithContext(ctx).
  WithEngineFactory(factory.NewStandardEngineFactory()).
  WithSettings(stepSettings).
  WithRouter(router).                      // or omit to self-manage
  WithProgramOptions(tea.WithAltScreen()).
  WithModelOptions(boba_chat.WithTitle("pinocchio")).
  WithSeedTurn(seed).
  BuildProgram()

router.AddHandler("ui", "ui", sess.EventHandler())
_ = router.RunHandlers(ctx)

// Optional autosubmit (outside the builder)
go func(){ <-router.Running(); prog.Send(chat.SubmitMessageMsg{}) }()
_, _ = prog.Run()
```

- Embedding into an existing Bubble Tea app:
```go
sess, chatModel, backend, handler, _ := NewChatBuilder().
  WithContext(ctx).
  WithEngineFactory(factory.NewStandardEngineFactory()).
  WithSettings(stepSettings).
  WithRouter(router).
  WithModelOptions(boba_chat.WithTitle("embedded-chat")).
  WithSeedTurn(seed).
  BuildComponents()

parent := NewParentModel(chatModel) // your app's aggregate model
p := tea.NewProgram(parent)
router.AddHandler("ui", "ui", handler)
_ = router.RunHandlers(ctx)
_, _ = p.Run()
```
This API lets `cmd.go` and third-party apps share the same construction path while leaving autosubmit as a simple caller-driven p.Send sequence when desired.

---

## 3) Immediate blur on start without a blocking `tea.Cmd` (keep 3B)

We want immediate input blur right after submit without waiting for `RunInference` to complete. Keep 3B: batch two commands—one to blur now, another to spawn the run—compatible with current model.

Detailed pseudocode (EngineBackend + model integration):

```go
// pinocchio/pkg/ui/backend.go
func (e *EngineBackend) Start(ctx context.Context, prompt string) (tea.Cmd, error) {
  if e.isRunning { return nil, errors.New("Engine is already running") }
  ctx, cancel := context.WithCancel(ctx)
  e.cancel = cancel
  e.isRunning = true

  // Command 1: immediate blur
  blurCmd := func() tea.Msg { return boba_chat.BlurInputMsg{} }

  // Command 2: fire-and-forget run (no blocking)
  runCmd := func() tea.Msg {
    go func() {
      seed := e.reduceHistory()
      if prompt != "" { turns.AppendBlock(seed, turns.NewUserTextBlock(prompt)) }
      updated, err := e.engine.RunInference(ctx, seed)
      if updated != nil {
        e.historyMu.Lock(); e.history = append(e.history, updated); e.historyMu.Unlock()
      }
      e.isRunning = false; e.cancel = nil
      // finalization messages delivered via event handler on final/interrupt/error
    }()
    return nil
  }

  return tea.Batch(blurCmd, runCmd), nil
}
```

Notes:
- Finalization is driven solely by the event handler (`StepChatForwardFunc`) on `EventFinal`/`EventInterrupt`/`EventError` which emits `BackendFinishedMsg`.
- Avoids double-finish signals and guarantees immediate blur.

---

## 4) Remove legacy non-timeline `Stream*` messages from `model.go` (4A)

To fully remove `StreamStartMsg`, `StreamCompletionMsg`, `StreamStatusMsg`, `StreamDoneMsg`, `StreamCompletionError` from `bobatea/pkg/chat/model.go`, the following steps are required:

- Audit and migrate all backends to use timeline lifecycle events via the router handler (`StepChatForwardFunc`) and/or direct timeline messages (for tests/demos). Update `bobatea/cmd/chat/FakeBackend` if needed.
- Delete handling branches in `(model).Update` for `Stream*` messages; keep only `timeline.UIEntity*` and `BackendFinishedMsg`.
- Migrate any CLI/demo code that relied on `Stream*` to the timeline path.
- Update docs and examples, and add a test ensuring no references to `Stream*` remain.

---

## 5) Reorganize `model.go` into a reusable chat timeline for agents

If we separate the input widget from the timeline, the chat model’s remaining responsibilities become clearer. The primary jobs are: maintaining overall UI state (streaming vs input), orchestrating timeline selection and navigation, and bridging backend start/cancel to state transitions (blur/focus). Much of the message rendering is already delegated to the `timeline.Controller` and entity models (`LLMTextModel`).

Deeper analysis:
- Input handling: The chat model currently owns `textarea.Model`, keymaps, and blur/focus transitions. If the input is split into a separate component (or hosted by the embedding app), the chat model can expose control messages (`BlurInputMsg`, `UnblurInputMsg`, `ReplaceInputTextMsg`, `SubmitMessageMsg`) and let the host own the widget.
- Timeline management: Promote selection/navigation (SelectNext/Prev, Enter/Exit selection, Copy) into a reusable `TimelineShell` wrapper for `timeline.Controller`. The chat model becomes a thin coordinator that forwards lifecycle messages and maps keybindings to shell operations.
- Backend orchestration: Submit continues to call `Backend.Start(ctx, userMessage)` and inject a user entity into the timeline. With the immediate-blur backend change, UI responsiveness improves without extra code.

What is left in the chat model after the split:
- Minimal state machine: `user-input` vs `streaming` (+ a boolean `inputBlurred`).
- Wiring: forward `timeline.UIEntity*` messages to `TimelineShell`, and handle `BackendFinishedMsg` → `finishCompletion()`.
- Optional utilities: helpers to read/replace/append input text for host convenience.

Potential surface after split:
```go
// chat package
 type ChatShell struct {
   Timeline *timeline.Controller // or a specialized TimelineShell
   InputBlurred bool
   State State // user-input | streaming
 }
 // Update(msg tea.Msg) processes only:
 // - Blur/Unblur/Submit
 // - Forward timeline lifecycle to the controller
 // - Finish on BackendFinishedMsg
```

Pros: A reusable chat timeline that agents can embed with their own input UX. Cons: Requires extracting selection/scroll helpers from the current model and documenting integration patterns.

---

## 6) Integrate `StepChatForwardFunc` directly with `ChatSession`

`StepChatForwardFunc` has felt like an odd free function. We can bind it to the session so callers do not need to import it explicitly.

Design:
```go
// pinocchio/pkg/ui/backend.go or pinocchio/pkg/chatrunner
 type UIEventAdapter struct { p *tea.Program }
 func (a UIEventAdapter) Handle(msg *message.Message) error { /* current StepChatForwardFunc logic */ }

 type ChatSession struct {
   Program *tea.Program
   Adapter UIEventAdapter
   Router  *events.EventRouter
 }
 func (cs *ChatSession) EventHandler() func(*message.Message) error { return cs.Adapter.Handle }
```

Usage (both CLI and embedding):
```go
sess, prog, _ := NewChatBuilder().WithRouter(router).BuildProgram()
router.AddHandler("ui", "ui", sess.EventHandler())
```
This keeps the event translation co-located with the session/program, reducing surface area and imports for callers while preserving the same functionality.

---

## 7) References (files/functions)

- Orchestration: `pinocchio/pkg/chatrunner/chat_runner.go` (`ChatSession`, `ChatBuilder`), `pinocchio/pkg/cmds/cmd.go` (`runChat`)
- Backend bridge: `pinocchio/pkg/ui/backend.go` (`EngineBackend`, `StepChatForwardFunc`)
- Events: `geppetto/pkg/events/chat-events.go`, `geppetto/pkg/inference/middleware/sink_watermill.go`
- UI core: `bobatea/pkg/chat/model.go`, `bobatea/pkg/timeline/controller.go`, `bobatea/pkg/timeline/renderers/llm_text_model.go`
