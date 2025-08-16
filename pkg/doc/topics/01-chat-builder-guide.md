---
Title: Pinocchio ChatBuilder Guide
Slug: pinocchio-chatbuilder-guide
Short: Build terminal chat UIs with the unified ChatBuilder/ChatSession API and Watermill event routing.
Topics:
- pinocchio
- chat
- bubbletea
- events
- engines
- turns
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# ChatBuilder Guide

## Overview

The ChatBuilder/ChatSession API provides a unified way to wire Geppetto engines (which stream events) to a Bubble Tea chat UI in Pinocchio. It eliminates duplicated orchestration across CLI and embedding use cases. Engines emit streaming events via Watermill; a bound handler translates those into UI timeline updates. This guide shows how to configure, build, and run chat sessions, how to embed the chat model into a parent UI, and how to provide custom handlers with access to both the Bubble Tea program and the session.

## Import Paths

```go
import (
    "context"

    tea "github.com/charmbracelet/bubbletea"
    boba_chat "github.com/go-go-golems/bobatea/pkg/chat"
    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/go-go-golems/pinocchio/pkg/ui/runtime"
)
```

## Core Concepts

A Geppetto engine performs inference over a `turns.Turn` and, when configured with a Watermill sink, publishes streaming events. The ChatBuilder wires an engine and a UI backend to a chat model, returns a ready-to-run `tea.Program` for CLI usage, or returns components for embedding into a parent Bubble Tea app. Event handling is bound to the `ChatSession`, so callers don’t import free functions to handle UI updates.

## Quick Start (Standalone CLI-style)

This example builds a complete chat program using ChatBuilder and registers the session’s event handler. Autosubmit and seeding remain external for clarity and control.

```go
ctx := context.Background()

stepSettings, _ := settings.NewStepSettings()
ef := factory.NewStandardEngineFactory()
router, _ := events.NewEventRouter()

// Build a seed turn (system/messages/prompt) as needed
seed := &turns.Turn{}

// Program options (TTY etc.)
opts := []tea.ProgramOption{tea.WithMouseCellMotion(), tea.WithAltScreen()}

// Build program and session
sess, prog, err := runtime.NewChatBuilder().
    WithContext(ctx).
    WithEngineFactory(ef).
    WithSettings(stepSettings).
    WithRouter(router).
    WithProgramOptions(opts...).
    WithModelOptions(boba_chat.WithTitle("pinocchio")).
    WithSeedTurn(seed). // optional
    BuildProgram()
if err != nil { panic(err) }

// Register handler and start router handlers
router.AddHandler("ui", "ui", sess.EventHandler())
_ = router.RunHandlers(ctx)

// Seed and autosubmit after router readiness (optional)
go func(){ <-router.Running(); sess.Backend.SetSeedTurn(seed) }()

_, _ = prog.Run()
```

## Embedding into a Parent Bubble Tea App

When embedding, build components (model, backend, handler) and integrate the chat model into your parent model. Bind the handler after a program exists.

```go
ctx := context.Background()
stepSettings, _ := settings.NewStepSettings()
ef := factory.NewStandardEngineFactory()
router, _ := events.NewEventRouter()

sess, chatModel, backend, handler, err := runtime.NewChatBuilder().
    WithContext(ctx).
    WithEngineFactory(ef).
    WithSettings(stepSettings).
    WithRouter(router).
    WithModelOptions(boba_chat.WithTitle("embedded-chat")).
    BuildComponents()
if err != nil { /* handle */ }

parent := NewParentModel(chatModel) // your parent model wraps the chat model
p := tea.NewProgram(parent)

// Bind handler now that a program exists
sess.BindHandlerWithProgram(p)
router.AddHandler("ui", "ui", handler)
_ = router.RunHandlers(ctx)

_, _ = p.Run()
_ = backend // avoid unused variable if not referenced directly
```

## Custom Handlers (Access to Program and Session)

For advanced scenarios, provide a handler factory that receives the `Program`, `Session`, and `Router` at bind-time.

```go
myFactory := func(hc runtime.HandlerContext) func(*message.Message) error {
    // hc.Session, hc.Program, hc.Router are available
    return func(msg *message.Message) error {
        // Inspect events and interact with the program/session
        return nil
    }
}

sess, prog, err := runtime.NewChatBuilder().
    WithContext(ctx).
    WithEngineFactory(ef).
    WithSettings(stepSettings).
    WithRouter(router).
    WithProgramOptions(tea.WithAltScreen()).
    WithModelOptions(boba_chat.WithTitle("pinocchio")).
    WithHandlerFactory(myFactory).
    BuildProgram()
if err != nil { /* handle */ }

router.AddHandler("ui", "ui", sess.EventHandler())
_ = router.RunHandlers(ctx)

_, _ = prog.Run()
```

If you only need a plain handler without program/session context, use `WithEventHandler(handler)`.

## Seeding and Autosubmit

Seeding prior context and autosubmitting an initial message should be done after the router signals readiness so that timeline entities are created deterministically.

```go
go func(){
    <-router.Running()
    sess.Backend.SetSeedTurn(seed)
    prog.Send(boba_chat.ReplaceInputTextMsg{Text: "Hello"})
    prog.Send(boba_chat.SubmitMessageMsg{})
}()
```

## Error Handling and Lifecycles

- Always check for builder errors when calling `BuildProgram()` or `BuildComponents()`.
- Manage the `events.EventRouter` lifecycle yourself when using an external router.
- On embedding, remember to invoke `sess.BindHandlerWithProgram(p)` before adding the handler to the router.

## Best Practices

- Keep autosubmit logic outside of the builder to avoid policy coupling.
- Pass TTY-aware `tea.ProgramOption` values (e.g., `WithAltScreen()` only when in a terminal).
- Use `WithHandlerFactory` for handlers that must coordinate UI state and session state.
- Seed after `<-router.Running()>` to ensure UI displays prior context reliably.

## Related Resources

- `pinocchio/pkg/ui/runtime/builder.go` — ChatBuilder and ChatSession implementation
- `pinocchio/pkg/ui/backend.go` — Engine-backed UI backend and default forwarding handler
- `bobatea/pkg/chat/model.go` — Chat model and message types
- `geppetto/pkg/events` — EventRouter and event types
- `geppetto/pkg/inference/engine` — Engine interface
- `geppetto/pkg/turns` — Turn structure


