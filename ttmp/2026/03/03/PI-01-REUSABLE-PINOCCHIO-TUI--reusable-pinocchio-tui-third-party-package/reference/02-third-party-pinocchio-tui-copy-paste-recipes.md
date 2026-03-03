---
Title: 'Third-party Pinocchio TUI: copy/paste recipes'
Ticket: PI-01-REUSABLE-PINOCCHIO-TUI
Status: active
Topics:
    - tui
    - pinocchio
    - refactor
    - thirdparty
    - bobatea
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Ready-to-run wiring snippets for building a custom Bubble Tea TUI on top of Pinocchio + Geppetto + Bobatea.
LastUpdated: 2026-03-03T08:06:05.67455786-05:00
WhatFor: ""
WhenToUse: ""
---

# Third-party Pinocchio TUI: copy/paste recipes

## Goal

Provide copy/paste-ready recipes for building a **third-party** Bubble Tea terminal UI that runs Pinocchio/Geppetto inference and renders output using Bobatea’s timeline-based chat UI.

## Context

Assumptions:

- Your third-party module is allowed to depend on these Go modules:
  - `github.com/go-go-golems/pinocchio`
  - `github.com/go-go-golems/geppetto`
  - `github.com/go-go-golems/bobatea`
- You are building a terminal UI using Bubble Tea (`github.com/charmbracelet/bubbletea`).
- You want to avoid importing `github.com/go-go-golems/pinocchio/cmd/...` from your third-party code.

Monorepo note (only relevant when prototyping inside this repo): if you create a standalone third-party module under `pinocchio/ttmp/...`, the top-level `go.work` may interfere with `go test`. Run with `GOWORK=off` to mimic real third-party builds.

Key idea:

- Pinocchio already provides reusable glue for a “basic chat” UI in `pinocchio/pkg/ui/runtime` + `pinocchio/pkg/ui`.
- “Agent-style/tool-loop” chat is currently implemented under `pinocchio/cmd/agents/simple-chat-agent/...` and should be extracted to `pinocchio/pkg/...` if you want to reuse it cleanly.

## Quick Reference

### Imports you will typically need

```go
import (
  "context"

  tea "github.com/charmbracelet/bubbletea"
  boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
  "github.com/go-go-golems/geppetto/pkg/events"
  "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
  "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
  "github.com/go-go-golems/pinocchio/pkg/ui/runtime"
)
```

### Minimal `go.mod` sketch (third-party module)

```go
module example.com/my-pinocchio-tui

go 1.22

require (
  github.com/go-go-golems/pinocchio v0.0.0-00010101000000-000000000000 // pick a real version
  github.com/go-go-golems/geppetto v0.0.0-00010101000000-000000000000 // pick a real version
  github.com/go-go-golems/bobatea v0.0.0-00010101000000-000000000000 // pick a real version
  github.com/charmbracelet/bubbletea v1.2.4 // pick a real version
)
```

Replace the pseudo versions above with real tags/commits when publishing.

### Recipe A: Standalone “basic chat” program (fastest path)

This uses Pinocchio’s `ChatBuilder.BuildProgram()` to return a ready-to-run `*tea.Program`.

```go
func main() {
  ctx := context.Background()

  stepSettings, err := settings.NewStepSettings()
  if err != nil { panic(err) }

  ef := factory.NewStandardEngineFactory()

  router, err := events.NewEventRouter()
  if err != nil { panic(err) }

  // Build chat program + session.
  sess, prog, err := runtime.NewChatBuilder().
    WithContext(ctx). // currently a no-op; safe to omit
    WithEngineFactory(ef).
    WithSettings(stepSettings).
    WithRouter(router).
    WithProgramOptions(tea.WithAltScreen()).
    WithModelOptions(boba_chat.WithTitle("my-pinocchio-tui")).
    BuildProgram()
  if err != nil { panic(err) }

  // Register the Watermill handler that forwards engine events → timeline entities.
  router.AddHandler("ui", "ui", sess.EventHandler())
  if err := router.RunHandlers(ctx); err != nil { panic(err) }

  // Run the UI.
  if _, err := prog.Run(); err != nil { panic(err) }
}
```

### Recipe B: Embed chat into your own Bubble Tea app (custom layout)

Use `BuildComponents()` so you can wrap the chat timeline in your own host model.

Important rule:

- Call `sess.BindHandlerWithProgram(p)` **after** you create the `*tea.Program`.

```go
type hostModel struct{
  chat tea.Model
}

func (m hostModel) Init() tea.Cmd { return m.chat.Init() }
func (m hostModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  updated, cmd := m.chat.Update(msg)
  m.chat = updated
  return m, cmd
}
func (m hostModel) View() string {
  // Replace with your own layout, sidebar, header, status bar, etc.
  return m.chat.View()
}

func main() {
  ctx := context.Background()
  stepSettings, _ := settings.NewStepSettings()
  ef := factory.NewStandardEngineFactory()
  router, _ := events.NewEventRouter()

  sess, chatModel, _, handler, err := runtime.NewChatBuilder().
    WithEngineFactory(ef).
    WithSettings(stepSettings).
    WithRouter(router).
    WithModelOptions(
      boba_chat.WithTitle("embedded"),
      // Optional: hide built-in input and drive it yourself.
      boba_chat.WithExternalInput(true),
    ).
    BuildComponents()
  if err != nil { panic(err) }

  host := hostModel{chat: chatModel}
  p := tea.NewProgram(host, tea.WithAltScreen())

  // Bind handler once program exists (required in embedding flow).
  sess.BindHandlerWithProgram(p)

  router.AddHandler("ui", "ui", handler)
  _ = router.RunHandlers(ctx)

  // Example: drive input programmatically (external input mode).
  go func() {
    <-router.Running()
    p.Send(boba_chat.ReplaceInputTextMsg{Text: "Hello from the host"})
    p.Send(boba_chat.SubmitMessageMsg{})
  }()

  _, _ = p.Run()
}
```

### Recipe C: Add custom timeline renderers

Renderers are registered on the timeline registry. You can use Bobatea-provided renderers and Pinocchio-provided renderer factories.

Example: register Pinocchio’s `agent_mode` renderer factory (if you emit those entities):

```go
import (
  "github.com/go-go-golems/bobatea/pkg/timeline"
  "github.com/go-go-golems/pinocchio/pkg/middlewares/agentmode"
)

// inside WithModelOptions:
boba_chat.WithTimelineRegister(func(r *timeline.Registry) {
  r.RegisterModelFactory(agentmode.AgentModeFactory{})
})
```

### Recipe D: Custom event handler (when you want different event → UI mapping)

If you want to translate Geppetto events to *your own* Bubble Tea messages (or to different timeline entity kinds), use `WithHandlerFactory`.

```go
import (
  "github.com/ThreeDotsLabs/watermill/message"
  "github.com/go-go-golems/geppetto/pkg/events"
)

myFactory := func(hc runtime.HandlerContext) func(*message.Message) error {
  return func(msg *message.Message) error {
    defer msg.Ack()
    ev, err := events.NewEventFromJson(msg.Payload)
    if err != nil { return err }

    // TODO: switch on ev.(type) and send your own UI messages via hc.Program.Send(...)
    _ = ev
    return nil
  }
}
```

### Recipe E: Persist timeline snapshots (optional)

Pinocchio provides a best-effort “persist from UI topic” helper:

- `pinocchio/pkg/ui.StepTimelinePersistFunc` (`pinocchio/pkg/ui/timeline_persist.go:19`)

It listens to topic `"ui"` (Geppetto events) and writes snapshot entities into a `chatstore.TimelineStore`.

Wiring sketch:

```go
// store := chatstore.New... (sqlite or memory)
router.AddHandler("ui-persist", "ui", ui.StepTimelinePersistFunc(store, convID))
```

## Usage Examples

### “I want to build my own TUI layout but keep the Pinocchio chat runtime”

Start from Recipe B (embedding). Then:

- Decide what your host model owns (sidebar, header, status bar).
- Decide whether you want Bobatea’s input widget or your own:
  - Use `WithExternalInput(true)` if you want your own input UX.
- Add renderers via `WithTimelineRegister`.
- If needed, customize the handler mapping via `WithHandlerFactory`.

### “I want the agent/tool-loop UI (tool calls, logs, web search) in my own package”

Today:

- The reference implementation is in:
  - `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`
- The recommended path is to extract this into `pinocchio/pkg/...` (see the design doc) so your third-party package never imports `cmd/`.

## Related

- Design doc: `design-doc/01-reusable-pinocchio-tui-analysis-extraction-guide.md`
- Existing internal guide: `pinocchio/pkg/doc/topics/01-chat-builder-guide.md`
