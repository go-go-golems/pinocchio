---
Title: Building a Middleware with a Renderer and Wiring It into a Timeline-Driven App
Slug: building-middleware-with-renderer
Short: Step-by-step guide to creating a Geppetto middleware that emits UI events and a Bubble Tea renderer to display them in the Pinocchio timeline.
Topics:
- pinocchio
- timeline
- bubbletea
- middleware
- renderer
- geppetto
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

## Overview

This tutorial walks you through building a complete UI feature powered by Geppetto middlewares and Bubble Tea renderers in Pinocchio. You will implement a middleware that detects a condition during inference and emits a structured event, build a matching renderer that displays the event in the `bobatea/pkg/timeline`, and wire the whole system together using the `tool_loop_backend.go` forwarder. The end result is an end-to-end, event-driven UI element (like `agentmode`) that can be toggled interactively.

For background, read:
- `glaze help geppetto-inference-engines` (reference: `geppetto/pkg/doc/topics/06-inference-engines.md`)
- `glaze help geppetto-middlewares` (reference: `geppetto/pkg/doc/topics/09-middlewares.md`)

## What we’ll build

We’ll create a simple “mode switch” feature inspired by `agentmode`:
- A middleware that, when a certain inference condition is met, emits an `EventAgentModeSwitch` with metadata and optional analysis text.
- A renderer that shows a compact summary by default and expands details when the user presses TAB while the entity is selected.
- A backend forwarder that converts engine/middleware events into timeline UI messages.

## Architecture at a glance

The flow is event-driven and append-only:
- Engine + Middlewares run on a `*turns.Turn` and publish events via a Watermill sink.
- A backend forwarder (`tool_loop_backend.go`) consumes those events and emits `timeline.UIEntity*` messages into the Bubble Tea program.
- The `timeline.Controller` creates entity models (renderers) based on `RendererDescriptor.Kind`.
- Interactive models receive focus/selection messages and key events (TAB/shift+TAB are routed to the selected entity even when not in entering mode).

## 1) Implement the middleware

Middlewares wrap `RunInference(ctx, *turns.Turn)` to add cross-cutting behavior. We’ll define a middleware that decides when to switch “mode” and publishes a structured event. The publishing happens through an event sink attached to the engine/context. See `glaze help geppetto-middlewares` for the core interfaces and composition rules.

```go
package agentmode

import (
    "context"
    "time"

    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/pkg/errors"
)

// ModeSwitch contains the information we want to show in the UI.
type ModeSwitch struct {
    From     string
    To       string
    Analysis string
}

// NewMiddleware returns a Middleware that detects mode switches and emits a UI-friendly event.
func NewMiddleware() middleware.Middleware {
    return func(next middleware.HandlerFunc) middleware.HandlerFunc {
        return func(ctx context.Context, t *turns.Turn) (*turns.Turn, error) {
            // Run downstream first (or do pre-processing before as needed)
            updated, err := next(ctx, t)
            if err != nil {
                return updated, err
            }

            // Detect a mode decision (this is domain-specific; replace with your logic)
            ms := detectModeSwitch(updated)
            if ms != nil {
                // Publish a structured event for the UI
                e := &events.EventAgentModeSwitch{
                    Message: "Agent mode changed",
                    Data: map[string]any{
                        "from": ms.From,
                        "to": ms.To,
                        "analysis": ms.Analysis,
                        "ts": time.Now().Format(time.RFC3339),
                    },
                }

                // Retrieve sinks from context and publish
                sinks := events.GetEventSinks(ctx)
                for _, s := range sinks {
                    if err := s.PublishEvent("chat", e); err != nil {
                        return updated, errors.Wrap(err, "publish mode switch event")
                    }
                }
            }

            return updated, nil
        }
    }
}

func detectModeSwitch(t *turns.Turn) *ModeSwitch {
    // Replace with real detection logic (e.g., inspect blocks or Turn.Data)
    return nil
}
```

Attach this middleware when constructing the engine wrapper (see “Composition” in `glaze help geppetto-middlewares`).

```go
engineWithMw := middleware.NewEngineWithMiddleware(baseEngine, agentmode.NewMiddleware())
```

## 2) Build the renderer (EntityModel)

Renderers implement the `EntityModel` interface from `bobatea/pkg/timeline/renderers`. They receive properties from `UIEntityCreated.Props` and can update their internal state in response to selection and key events. Use TAB to toggle details.

Key points for interactive models:
- Keep internal fields for toggles like `showDetails`.
- Reset state on `timeline.EntityUnselectedMsg`.
- Toggle on `tea.KeyMsg` when `key.String()` is "tab" or "shift+tab" and the model is selected.
- Keep views single-responsibility: no layout side-effects.

```go
package agentmode

import (
    "fmt"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/go-go-golems/bobatea/pkg/timeline"
)

type AgentModeModel struct {
    title       string
    from        string
    to          string
    analysis    string
    selected    bool
    showDetails bool
}

func (m *AgentModeModel) Init() tea.Cmd { return nil }

func (m *AgentModeModel) OnProps(props map[string]any) tea.Cmd {
    if v, ok := props["title"].(string); ok { m.title = v }
    if v, ok := props["from"].(string); ok { m.from = v }
    if v, ok := props["to"].(string); ok { m.to = v }
    if v, ok := props["analysis"].(string); ok { m.analysis = v }
    return nil
}

func (m *AgentModeModel) Update(msg tea.Msg) (timeline.EntityModel, tea.Cmd) {
    switch msg := msg.(type) {
    case timeline.EntitySelectedMsg:
        m.selected = true
    case timeline.EntityUnselectedMsg:
        m.selected = false
        m.showDetails = false
    case tea.KeyMsg:
        if m.selected && (msg.String() == "tab" || msg.String() == "shift+tab") {
            m.showDetails = !m.showDetails
        }
    }
    return m, nil
}

func (m *AgentModeModel) View() string {
    head := fmt.Sprintf("Agent Mode: %s → %s", m.from, m.to)
    if m.title != "" {
        head = fmt.Sprintf("%s — %s", head, m.title)
    }
    if m.showDetails && m.analysis != "" {
        return head + "\n\n" + m.analysis
    }
    return head
}
```

Register the renderer with the timeline controller using a factory:

```go
type AgentModeFactory struct{}

func (AgentModeFactory) Kind() string { return "agent_mode" }
func (AgentModeFactory) New() timeline.EntityModel { return &AgentModeModel{} }
```

In your app wiring (for example in `main.go`), register it:

```go
shell := timeline.NewShell()
shell.Controller().RegisterModelFactory(agentmode.AgentModeFactory{})
```

## 3) Forward events to UI in the backend

Your backend converts engine/middleware events into UI messages for the Bubble Tea program. Pinocchio includes `ToolLoopBackend` which provides `MakeUIForwarder(p *tea.Program)`, a Watermill handler that parses Geppetto events and emits `timeline.UIEntity*` messages. See `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`.

Add/ensure a case for your event (e.g., `EventAgentModeSwitch`) that creates a timeline entity and completes it:

```go
case *events.EventAgentModeSwitch:
    props := map[string]any{"title": e_.Message}
    for k, v := range e_.Data { props[k] = v }
    localID := fmt.Sprintf("agentmode-%s-%d", md.TurnID, time.Now().UnixNano())
    p.Send(timeline.UIEntityCreated{
        ID:       timeline.EntityID{LocalID: localID, Kind: "agent_mode"},
        Renderer: timeline.RendererDescriptor{Kind: "agent_mode"},
        Props:    props,
    })
    p.Send(timeline.UIEntityCompleted{ID: timeline.EntityID{LocalID: localID, Kind: "agent_mode"}})
```

Notes:
- Use unique `LocalID`s to avoid collisions.
- Keep entities append-only; update with `UIEntityUpdated` and finalize with `UIEntityCompleted`.
- To remove an entity, emit `timeline.UIEntityDeleted{ID: ...}`.

## 4) Wire everything together in the application

Putting it all together in a `main.go`-style setup:

```go
// 1) Create event router + sink (for streaming)
router, _ := events.NewEventRouter()
watermillSink := middleware.NewWatermillSink(router.Publisher, "chat")

// 2) Create engine and wrap with our middleware
baseEngine, _ := factory.NewEngineFromParsedLayers(parsed)
engineWithMw := middleware.NewEngineWithMiddleware(baseEngine, agentmode.NewMiddleware())

// 3) Build Bubble Tea program with timeline shell and register renderer
shell := timeline.NewShell()
shell.Controller().RegisterModelFactory(agentmode.AgentModeFactory{})
p := tea.NewProgram(shell)

// 4) Forward events from router to UI
backend := backendpkg.NewToolLoopBackend(engineWithMw, registry, watermillSink, nil)
router.AddHandler("chat-ui", "chat", backend.MakeUIForwarder(p))

// 5) Run router and TUI concurrently, then start the backend
go router.Run(context.Background())
go p.Run()
// Start the tool loop (which will emit events processed by our middleware)
cmd, _ := backend.Start(context.Background(), "What is 2+2?")
p.Send(cmd())
```

## 5) Interaction and UX details

- Selection vs. entering: The controller routes TAB/shift+TAB to the selected entity, even when the entity is not in entering mode. This enables quick toggles without extra keystrokes.
- Scrolling: Keep the renderer’s `View()` pure; the shell updates the viewport content without auto-jumping to the bottom after interactive toggles.
- Markdown: If your renderer shows markdown, prefer rendering labels (like role prefixes) outside the markdown block and normalize leading symbols to avoid Glamour mis-parsing.

## Best practices

- **Keep middleware stateless** where possible; store small hints on `Turn.Data`.
- **Emit structured events** with clear, stable fields; avoid screen-oriented strings in middleware.
- **Isolate UI state** in renderers; use concise toggles like `showDetails`.
- **Use unique IDs** for timeline entities; never mutate the ID after creation.
- **Prefer append-only UI**: create -> update -> complete; delete via `UIEntityDeleted` if needed.
- **Test end-to-end** by simulating events and verifying renderer behavior with key routing.

## References

- Background on engines: `glaze help geppetto-inference-engines`
- Background on middlewares: `glaze help geppetto-middlewares`
- Renderer examples: `pinocchio/pkg/middlewares/agentmode/agent_mode_model.go`
- Middleware example: `pinocchio/pkg/middlewares/agentmode/middleware.go`
- Backend forwarder: `pinocchio/cmd/agents/simple-chat-agent/pkg/backend/tool_loop_backend.go`


