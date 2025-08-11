---
Title: Pinocchio ChatRunner API Documentation
Slug: pinocchio-chatrunner-api
Short: Build chat interfaces with the Engine/Turn architecture and streaming events from Geppetto.
Topics:
- pinocchio
- chatrunner
- architecture
- api
- events
- engines
- turns
- ui
Commands: []
Flags: []
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# ChatRunner API Documentation

## Overview

The ChatRunner API provides a streamlined way to create and manage chat-based interactions in Pinocchio using Geppetto’s latest Engine/Turn architecture. Engines handle provider I/O and publish streaming events; the Bubbletea-based UI consumes those events for real-time updates. This page explains the core concepts, how to wire an engine to the UI through the event router, and how to run sessions in different modes.

## Import Paths

Essential packages to import:

```go
import (
    "context"
    "fmt"
    "io"
    "os"
    
    tea "github.com/charmbracelet/bubbletea"
    bobachat "github.com/go-go-golems/bobatea/pkg/chat"
    geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
    "github.com/go-go-golems/pinocchio/pkg/chatrunner"
    "github.com/go-go-golems/pinocchio/pkg/ui"
    "github.com/rs/zerolog/log"
    "golang.org/x/sync/errgroup"
)
```

## Core Concepts: Engines, Turns, and Events

The ChatRunner now builds on Geppetto’s Engine/Turn model rather than the older chat step abstraction:

- Engines (`engine.Engine`): Provider-specific clients that implement a single method:
  ```go
  type Engine interface {
      RunInference(ctx context.Context, t *turns.Turn) (*turns.Turn, error)
  }
  ```
- Turns (`turns.Turn`): The unit of inference. Convert to/from conversations when needed.
- Events (Watermill): Engines publish streaming events (start/partial/final; tool-call/tool-result if configured) through sinks. The UI subscribes and renders incremental output.

This separation provides:
- Immediate feedback even for long-running operations
- Clear boundaries between provider I/O (engine) and orchestration/UI
- Observable pipelines where each turn’s progress can be tracked

The ChatRunner orchestrates engine creation, event routing, and UI consumption—so you can focus on conversation and settings.

## Core Components

### ChatBuilder

The ChatBuilder (`chatrunner.ChatBuilder`) implements a fluent builder for configuring chat sessions (engine factory, settings, conversation manager, mode, and UI options).

```go
builder := chatrunner.NewChatBuilder().
    WithManager(manager).
    WithEngineFactory(factory.NewStandardEngineFactory()).
    WithSettings(stepSettings).
    WithMode(chatrunner.RunModeChat).
    WithUIOptions(bobachat.WithTitle("Echo Chat Runner")).
    WithContext(context.Background())
```

Key features:
- Chainable configuration methods
- Built-in validation at each step
- Error accumulation during the build process
- Sensible defaults for optional components

### EngineFactory Pattern

The builder expects an `engine/factory.EngineFactory` to create a provider-specific engine based on `settings.StepSettings`:

```go
// EngineFactory interface (from geppetto)
type EngineFactory interface {
    CreateEngine(settings *settings.StepSettings, options ...engine.Option) (engine.Engine, error)
}
```

This allows the ChatRunner to instantiate an engine with the right sink (for event streaming) without coupling to specific providers.

### Run Modes

The API supports different execution modes (defined in `chatrunner` package):
- `chatrunner.RunModeChat`: Full interactive terminal UI
- `chatrunner.RunModeInteractive`: Initial blocking execution with optional chat
- `chatrunner.RunModeBlocking`: Non-interactive step execution

## Configuration Options

### Required Components

1. **Conversation Manager** (`conversation.Manager` from `github.com/go-go-golems/geppetto/pkg/conversation`)
   ```go
   manager := conversation.NewManager(
       conversation.WithMessages(
           conversation.NewChatMessage(conversation.RoleSystem, "System Prompt"),
       ),
   )
   ```

2. **Step Factory**
   ```go
   stepFactory := func(publisher message.Publisher, topic string) (chat.Step, error) {
       step := steps.NewYourStep()
       if publisher != nil && topic != "" {
           err := step.AddPublishedTopic(publisher, topic)
           if err != nil {
               return nil, err
           }
       }
       return step, nil
   }
   ```

### Optional Configuration

- **UI Options**: Customize the Bubbletea UI appearance
  ```go
  // bobachat.ModelOption from github.com/go-go-golems/bobatea/pkg/chat
  WithUIOptions(bobachat.WithTitle("Custom Title"))
  ```

- **Program Options**: Configure Bubbletea program behavior
  ```go
  // tea.ProgramOption from github.com/charmbracelet/bubbletea
  WithProgramOptions(tea.WithMouseCellMotion())
  ```

- **Context**: Provide custom context for execution control
  ```go
  WithContext(ctx)
  ```

- **External Router**: Use an existing event router
  ```go
  // events.EventRouter from github.com/go-go-golems/geppetto/pkg/events
  WithExternalRouter(router)
  ```

## Event Routing and Architecture

The ChatRunner wires an engine’s streaming events to the UI via a Watermill-backed event router.

1.  **EventRouter Creation:** An `events.EventRouter` is created (unless provided via `WithExternalRouter`).
2.  **Engine Creation with Sink:** A `middleware.NewWatermillSink(router.Publisher, "ui")` is passed via `engine.WithSink(...)` to the engine so it can publish start/partial/final events.
3.  **Handler Registration:** The UI forwarding handler subscribes to the same topic and forwards events to Bubbletea.
4.  **Lifecycle Management:** Router and handlers are run under an `errgroup` and controlled by context cancellation.

This encapsulates the boilerplate required to connect an engine’s event stream to the UI.

## Advanced Event Handling

Understanding the different event types published by chat steps is crucial for building custom handlers or interpreting the flow.

### Custom Event Router Setup

For advanced use cases, you can create and configure your own `EventRouter` instance before passing it to the ChatRunner:

```go
// Create a custom event router with options
router, err := events.NewEventRouter(
    events.WithVerbose(true),
    events.WithLogger(customLogger),
)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to create event router")
}

// Pass it to the builder
builder := chatrunner.NewChatBuilder().
    WithManager(manager).
    WithEngineFactory(engFactory).
    WithSettings(stepSettings).
    WithMode(chatrunner.RunModeChat).
    WithExternalRouter(router)
```

This approach is particularly useful when:
- You need to share an event router across multiple components
- You require custom routing behaviors
- You want to directly subscribe to or publish events outside of steps

### Implementing Custom Chat Event Handlers

You can implement and register custom handlers for chat events by implementing the `events.ChatEventHandler` interface from `github.com/go-go-golems/geppetto/pkg/events`.

This is useful for:
- Logging events to a file or database.
- Triggering other actions based on chat progress.
- Building alternative UIs or integrations.

```go
// ChatEventHandler interface defined in geppetto/pkg/events/event-router.go
type ChatEventHandler interface {
    HandlePartialCompletion(ctx context.Context, e *events.EventPartialCompletion) error
    HandleText(ctx context.Context, e *events.EventText) error // Note: May be deprecated/merged
    HandleFinal(ctx context.Context, e *events.EventFinal) error
    HandleError(ctx context.Context, e *events.EventError) error
    HandleInterrupt(ctx context.Context, e *events.EventInterrupt) error
    // Potentially HandleToolCall, HandleToolResult in the future
}
```

Example implementation:

```go
import (
    "context"
    "fmt"
    "github.com/go-go-golems/geppetto/pkg/events"
)

type CustomChatHandler struct {
    // Your handler state
}

func (h *CustomChatHandler) HandlePartialCompletion(ctx context.Context, e *events.EventPartialCompletion) error {
    fmt.Printf("Partial completion: %s\n", e.Content)
    return nil
}

func (h *CustomChatHandler) HandleText(ctx context.Context, e *events.EventText) error {
    fmt.Printf("Text: %s\n", e.Content)
    return nil
}

func (h *CustomChatHandler) HandleFinal(ctx context.Context, e *events.EventFinal) error {
    fmt.Println("Final event received")
    return nil
}

func (h *CustomChatHandler) HandleError(ctx context.Context, e *events.EventError) error {
    fmt.Printf("Error: %s\n", e.Error)
    return nil
}

func (h *CustomChatHandler) HandleInterrupt(ctx context.Context, e *events.EventInterrupt) error {
    fmt.Println("Interrupt received")
    return nil
}
```

### Event Types Reference

Events are published by chat engines to signal different stages and outcomes of their execution. All events implement the `events.Event` interface and carry `EventMetadata` and `events.StepMetadata`.

They are typically created using constructors like `events.NewStartEvent(...)` and serialized to JSON for transport via Watermill.

1.  **`events.EventTypeStart` (`*events.EventPartialCompletionStart`)**: Signals the beginning of a step's execution, specifically one that might produce partial completions.
    ```go
    // Represents the start of a potentially streaming operation.
    type EventPartialCompletionStart struct {
        events.EventImpl
    }
    ```

2.  **`events.EventTypePartialCompletion` (`*events.EventPartialCompletion`)**: Represents an incremental update, typically a chunk of text from an AI model during streaming.
    ```go
    // Event for textual partial completion. Tool call chunks are not typically streamed this way.
    type EventPartialCompletion struct {
        events.EventImpl
        Delta      string `json:"delta"`      // The incremental change
        Completion string `json:"completion"` // The complete text generated so far
    }
    ```

3.  **`events.EventTypeToolCall` (`*events.EventToolCall`)**: Signals that the AI model has decided to call a function/tool.
    ```go
    // Represents a request from the AI to execute a tool.
    type EventToolCall struct {
        events.EventImpl
        ToolCall events.ToolCall `json:"tool_call"`
    }

    type ToolCall struct {
        ID    string `json:"id"`    // Unique ID for the tool call
        Name  string `json:"name"`  // Name of the function to call
        Input string `json:"input"` // Arguments for the function (often JSON string)
    }
    ```

4.  **`events.EventTypeToolResult` (`*events.EventToolResult`)**: Provides the result of a tool execution back to the step (and potentially the AI model).
    ```go
    // Represents the outcome of a tool execution.
    type EventToolResult struct {
        events.EventImpl
        ToolResult events.ToolResult `json:"tool_result"`
    }

    type ToolResult struct {
        ID     string `json:"id"`     // ID matching the corresponding ToolCall
        Result string `json:"result"` // Result of the tool execution (often JSON string)
    }
    ```

5.  **`events.EventTypeFinal` (`*events.EventFinal`)**: Signals the successful completion of the step's execution. Contains the final aggregated text response.
    ```go
    // Signals successful completion of the step.
    type EventFinal struct {
        events.EventImpl
        Text string `json:"text"` // The final, complete text output
        // TODO(manuel, 2024-07-04) Add all collected tool calls so far
    }
    ```

6.  **`events.EventTypeError` (`*events.EventError`)**: Indicates that an error occurred during the step's execution.
    ```go
    // Signals an error during step execution.
    type EventError struct {
        events.EventImpl
        ErrorString string `json:"error_string"` // The error message
    }
    ```

7.  **`events.EventTypeInterrupt` (`*events.EventInterrupt`)**: Signals that the step's execution was interrupted (e.g., by context cancellation).
    ```go
    // Signals that the step was interrupted.
    type EventInterrupt struct {
        events.EventImpl
        Text string `json:"text"` // Potentially partial text generated before interrupt
        // TODO(manuel, 2024-07-04) Add all collected tool calls so far
    }
    ```

8.  **`events.EventText` (`*events.EventText`)**: Represents a simple text message event. Its role might overlap with `EventPartialCompletion` and `EventFinal` and could potentially be refactored.
    ```go
    // Generic text event. Usage might be limited compared to Final/Partial.
    type EventText struct {
        events.EventImpl
        Text string `json:"text"`
    }
    ```

All event types embed `events.EventImpl`, which contains common fields like `Type_`, `Metadata_`, and `Step_`.

Use `events.NewEventFromJson(payload)` to deserialize a received message payload back into a specific `Event` interface type, and `events.ToTypedEvent[T](event)` to safely cast it to its concrete type.

### Registering Custom Handlers with the Router

Register your handler with the router using the `RegisterChatEventHandler` method:

```go
// Create a custom handler
handler := &CustomChatHandler{}

// Register it with the router for a specific ID
err = router.RegisterChatEventHandler(
    context.Background(),
    step,           // Your chat step or equivalent context
    "client-123",   // Unique identifier for this handler
    handler,        // Your ChatEventHandler implementation
)
if err != nil {
    log.Fatal().Err(err).Msg("Failed to register chat event handler")
}
```

The EventRouter will:
1. Configure the step to publish events to a topic based on the provided ID (`chat-{id}`)
2. Create a dispatch handler function that routes events to your handler's methods
3. Register the dispatch handler with the router

### Manual Event Handling

For even more control, you can add handlers directly using the low-level API:

```go
// Import github.com/ThreeDotsLabs/watermill/message
// Define a custom message handler function
handler := func(msg *message.Message) error {
    // Process the message
    fmt.Printf("Received message on topic: %s\n", msg.Metadata.Get("topic"))
    return nil
}

// Add the handler to the router
router.AddHandler(
    "my-custom-handler",  // Handler name
    "my-custom-topic",    // Topic to subscribe to
    handler,              // Handler function
)
```

### Event Router Lifecycle Management

When using an external `events.EventRouter`, you are responsible for managing its lifecycle. The router typically relies on [Watermill](https://github.com/ThreeDotsLabs/watermill), a library providing abstractions for message publishing, subscribing, and routing.

```go
// Import golang.org/x/sync/errgroup
// Start the router in a goroutine
eg, ctx := errgroup.WithContext(context.Background())
eg.Go(func() error {
    // router.Run blocks until the context is cancelled or an error occurs
    return router.Run(ctx)
})

// Run your application logic...

// When done, cancel the context to signal the router to stop
// cancel()

// Wait for the router goroutine to finish
// if err := eg.Wait(); err != nil { ... }

// Optionally, explicitly close the router (depends on implementation)
err = router.Close()
if err != nil {
    log.Error().Err(err).Msg("Error closing router")
}
```

The ChatRunner handles this lifecycle management automatically when it creates the router internally.

## Error Handling

The API implements comprehensive error handling:
1. Validation during configuration
2. Error accumulation in builder chain
3. Goroutine error propagation
4. Clean shutdown on errors

## Best Practices

1. **Step Factory Implementation**
   ```go
   // Package-level function to create a StepFactory
   func createStepFactory() chatrunner.StepFactory {
       return func(publisher message.Publisher, topic string) (chat.Step, error) {
           step := NewYourStep()
           if publisher != nil && topic != "" {
               return step.WithPublisher(publisher, topic)
           }
           return step, nil
       }
   }
   ```

2. **Error Handling**
   ```go
   session, err := builder.Build()
   if err != nil {
       log.Error().Err(err).Msg("Failed to build chat runner")
       return err
   }
   if err := session.Run(); err != nil {
       log.Error().Err(err).Msg("Chat Runner failed")
       return err
   }
   ```

3. **Resource Cleanup**
   ```go
   ctx, cancel := context.WithCancel(context.Background())
   defer cancel()
   builder.WithContext(ctx)
   ```

## Example Usage

### Basic Chat UI (Engine/Turn)

```go
package main

import (
    "context"
    "os"

    bobachat "github.com/go-go-golems/bobatea/pkg/chat"
    geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
    "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
    "github.com/go-go-golems/pinocchio/pkg/chatrunner"
    "github.com/rs/zerolog/log"
)

func main() {
    // 1. Create manager
    manager := geppetto_conversation.NewManager(
        geppetto_conversation.WithMessages(
            geppetto_conversation.NewChatMessage(geppetto_conversation.RoleSystem, "System Prompt"),
        ),
    )

    // 2. Prepare engine factory + settings
    engFactory := factory.NewStandardEngineFactory()
    stepSettings := &settings.StepSettings{ /* set provider + model */ }

    // 3. Configure and run
    builder := chatrunner.NewChatBuilder().
        WithManager(manager).
        WithEngineFactory(engFactory).
        WithSettings(stepSettings).
        WithMode(chatrunner.RunModeChat).
        WithUIOptions(bobachat.WithTitle("Echo Chat"))

    session, err := builder.Build()
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to build chat runner")
    }
    
    if err := session.Run(); err != nil {
        log.Fatal().Err(err).Msg("Chat Runner failed")
    }
}
```

### Non-Interactive Mode

```go
builder := chatrunner.NewChatBuilder().
    WithManager(manager).
    WithEngineFactory(engFactory).
    WithSettings(stepSettings).
    WithMode(chatrunner.RunModeBlocking).
    WithOutputWriter(os.Stdout)
```

## Implementation Details

### Internal Architecture

The ChatRunner implementation consists of three main layers:
1. **Builder Layer**: Configuration and validation
2. **Session Layer**: Execution coordination
3. **Runtime Layer**: Goroutine and event management

### Goroutine Management

The runner manages two main goroutines:
1. **Router Goroutine**: Handles event routing
2. **UI Goroutine**: Manages the Bubbletea UI

These are coordinated using `errgroup` and proper context cancellation.

## Future Considerations

1. **Enhanced Mode Support**
   - Additional run modes for different interaction patterns
   - Custom mode configuration options

2. **UI Customization**
   - More granular UI control
   - Custom component injection

3. **Event Handling**
   - Custom event handler registration
   - Event filtering and transformation

4. **Testing Support**
   - Mock implementations
   - Test utilities
   - Recorder/replay functionality

## Related Documentation

- [Pinocchio Chat UI Setup API Design](../2025-04-11/02-pinocchio-chat-ui-design.md)
- [Geppetto Steps, PubSub, and Watermill Explanation](../../geppetto/ttmp/2025-03-29/06-sonnet-3.7-step-pubsub-explanation.md)
- [Bubbletea Documentation](https://github.com/charmbracelet/bubbletea)
- [Geppetto Step Interface (`steps.Step`)](https://github.com/go-go-golems/geppetto/pkg/steps)
- [Geppetto Chat Step Interface (`chat.Step`)](https://github.com/go-go-golems/geppetto/pkg/steps/ai/chat)
- [Geppetto Events (`events.Event`)](https://github.com/go-go-golems/geppetto/pkg/events)
- [Watermill Messaging Library](https://github.com/ThreeDotsLabs/watermill) 