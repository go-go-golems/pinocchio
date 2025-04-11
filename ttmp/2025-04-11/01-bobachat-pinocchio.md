# Bobatea Chat UI System Architecture

This document provides a detailed overview of the UI system architecture in the Bobatea/Pinocchio codebase, explaining how various components interact to create an interactive chat interface.

## Core Components Overview

The UI system consists of several key components that work together:

1. **Conversation Manager** - Manages chat history and state
2. **Step** - Handles AI/LLM interactions
3. **Step Backend** - Bridges UI and AI processing
4. **Event Router** - Manages event communication
5. **Bobachat Model** - Main UI component
6. **Runtime Orchestration** - Coordinates all components

Let's explore each component in detail.

## 1. Conversation Manager

The conversation manager (`geppetto_conversation.Manager`) is responsible for maintaining the chat history and state.

```go
// From pinocchio/cmd/experiments/ui/main.go
manager := conversation.NewManager(
    conversation.WithMessages(
        conversation.NewChatMessage(conversation.RoleSystem, "hahahahaha"),
    ))
```

Key responsibilities:

- Stores conversation messages
- Manages message relationships (parent-child)
- Handles conversation persistence
- Provides methods for appending/retrieving messages

## 2. Step Component

The Step component (`chat.Step`) is responsible for executing AI/LLM operations. It's an interface that defines how to process messages and generate responses.

```go
// Example from pinocchio/cmd/experiments/ui/main.go
step := steps.NewEchoStep()
step.AddPublishedTopic(router.Publisher, "ui")
```

Key aspects:

- Implements actual AI/LLM interaction
- Publishes events during processing
- Can be interrupted/cancelled
- Returns results through a channel

## 3. Step Backend

The Step Backend (`ui.StepBackend`) bridges the UI and AI processing:

```go
// From pinocchio/pkg/ui/backend.go
type StepBackend struct {
    step       chat.Step
    stepResult steps.StepResult[*conversation.Message]
}

func (s *StepBackend) Start(ctx context.Context, msgs []*conversation.Message) (tea.Cmd, error) {
    if !s.IsFinished() {
        return nil, errors.New("Step is already running")
    }
    stepResult, err := s.step.Start(ctx, msgs)
    // ...
}
```

Responsibilities:

- Manages step execution lifecycle
- Handles interruption/cancellation
- Provides status information
- Converts between step results and UI messages

## 4. Event Router

The Event Router (`events.EventRouter`) manages event communication between components:

```go
// From pinocchio/cmd/experiments/ui/main.go
router, err := events.NewEventRouter()
if err != nil {
    panic(err)
}

router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
```

Features:

- Pub/sub event system
- Topic-based routing
- Handler registration
- Event forwarding to UI

## 5. Bobachat Model

The Bobachat Model (`bobatea_chat.model`) is the main UI component:

```go
// From bobatea/pkg/chat/model.go
type model struct {
    conversationManager geppetto_conversation.Manager
    autoStartBackend    bool
    viewport           viewport.Model
    textArea           textarea.Model
    conversation       conversationui.Model
    backend            Backend
    state             State
    // ...
}
```

Key components:

- Viewport for message display
- Text area for input
- State management
- Event handling
- Rendering logic

## 6. Runtime Orchestration

The runtime orchestration ties everything together using goroutines and error groups:

```go
// From pinocchio/cmd/experiments/ui/main.go
eg := errgroup.Group{}
ctx, cancel := context.WithCancel(context.Background())

// Router goroutine
eg.Go(func() error {
    defer f()
    ret := router.Run(ctx)
    return ret
})

// UI goroutine
eg.Go(func() error {
    defer f()
    options := []tea.ProgramOption{
        tea.WithMouseCellMotion(),
        tea.WithAltScreen(),
    }

    backend := ui.NewStepBackend(step)
    p := tea.NewProgram(
        boba_chat.InitialModel(manager, backend,
            boba_chat.WithTitle("ui"),
            boba_chat.WithAutoStartBackend(true),
        ),
        options...,
    )

    router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
    // ...
})
```

## Message Flow

1. **User Input**

   - User types in text area
   - Input captured by Bobachat Model
   - Message added to Conversation Manager

2. **Processing**

   - Step Backend starts processing
   - Step executes AI/LLM operation
   - Events published through router

3. **UI Updates**
   - Events forwarded to UI
   - Conversation updated
   - Display refreshed

## State Management

The UI has several states (`bobatea/pkg/chat/model.go`):

```go
const (
    StateUserInput        State = "user-input"
    StateMovingAround     State = "moving-around"
    StateStreamCompletion State = "stream-completion"
    StateSavingToFile     State = "saving-to-file"
    StateError            State = "error"
)
```

Each state affects:

- Available keyboard shortcuts
- UI component behavior
- Event handling

## Developer Guidelines

When working with the UI system:

1. **Component Creation**

   - Use `InitialModel` for Bobachat setup
   - Implement necessary Backend interface
   - Configure event routing

2. **Event Handling**

   - Register handlers for relevant topics
   - Use appropriate message types
   - Handle errors gracefully

3. **State Management**

   - Track UI state changes
   - Update components accordingly
   - Handle transitions properly

4. **Error Handling**
   - Use error states for display
   - Propagate errors through channels
   - Clean up resources properly

## Common Patterns

1. **Starting the UI**

```go
backend := ui.NewStepBackend(step)
model := boba_chat.InitialModel(manager, backend,
    boba_chat.WithTitle("ui"),
    boba_chat.WithAutoStartBackend(true),
)
p := tea.NewProgram(model, options...)
```

2. **Event Routing**

```go
router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
err = router.RunHandlers(ctx)
```

3. **Goroutine Management**

```go
eg := errgroup.Group{}
ctx, cancel := context.WithCancel(context.Background())
// Add router and UI goroutines
err = eg.Wait()
```

## Next Steps

To extend or modify the UI system:

1. **New Features**

   - Add new UI components
   - Implement additional event types
   - Create new backend types

2. **Customization**

   - Modify styling
   - Add keyboard shortcuts
   - Enhance state management

3. **Integration**
   - Connect to different AI/LLM systems
   - Add persistence layers
   - Implement new message types

## Related Files

Key files in the codebase:

- `bobatea/pkg/chat/model.go` - Main UI model
- `pinocchio/pkg/ui/backend.go` - Step backend implementation
- `pinocchio/cmd/experiments/ui/main.go` - Example implementation
- `bobatea/pkg/chat/conversation/model.go` - Conversation UI model
- `pinocchio/pkg/cmds/cmd.go` - Command integration

## Event System Details

### Event Router Implementation

The Event Router is implemented in `geppetto/pkg/events/event-router.go`:

```go
type EventRouter struct {
    logger     watermill.LoggerAdapter
    Publisher  message.Publisher
    Subscriber message.Subscriber
    router     *message.Router
    verbose    bool
}
```

The router uses Watermill's gochannel implementation for in-memory pub/sub:

```go
goPubSub := gochannel.NewGoChannel(gochannel.Config{
    BlockPublishUntilSubscriberAck: true,
}, ret.logger)
ret.Publisher = goPubSub
ret.Subscriber = goPubSub
```

### Event Types

The system defines several event types in `geppetto/pkg/events/chat-events.go`:

```go
const (
    EventTypeStart             EventType = "start"
    EventTypeFinal             EventType = "final"
    EventTypePartialCompletion EventType = "partial"
    EventTypeStatus            EventType = "status"
    EventTypeToolCall          EventType = "tool-call"
    EventTypeToolResult        EventType = "tool-result"
    EventTypeError            EventType = "error"
    EventTypeInterrupt        EventType = "interrupt"
)
```

Each event carries metadata:

```go
type EventMetadata struct {
    ID       conversation.NodeID
    ParentID conversation.NodeID
    LLMMessageMetadata
}

type StepMetadata struct {
    StepID     uuid.UUID
    Type       string
    InputType  string
    OutputType string
    Metadata   map[string]interface{}
}
```

### Event Flow

1. **Event Publishing**:

   - Steps use a `PublisherManager` to manage multiple publishers
   - Events are published using methods like `NewStartEvent`, `NewPartialCompletionEvent`, etc.
   - Events include metadata about the step and message context

2. **Event Routing**:

   - The router forwards events to registered handlers
   - Handlers process events based on type (start, partial, final, etc.)
   - UI components receive and update based on events

3. **Event Handling**:
   - UI components implement `ChatEventHandler` interface
   - Events trigger UI updates through Bubbletea messages
   - Different event types trigger different UI behaviors

### Example Step: EchoStep

The `EchoStep` (in `geppetto/pkg/steps/ai/chat/steps/echo.go`) is a simple example step that demonstrates the event system:

```go
type EchoStep struct {
    TimePerCharacter    time.Duration
    cancel             context.CancelFunc
    eg                 *errgroup.Group
    subscriptionManager *events.PublisherManager
}
```

Key features:

- Simulates typing by sending character-by-character updates
- Publishes start, partial completion, and final events
- Demonstrates proper event metadata handling
- Shows cancellation and error handling

### StepBackend Implementation

The `StepBackend` (in `pinocchio/pkg/ui/backend.go`) bridges between steps and the UI:

```go
func StepChatForwardFunc(p *tea.Program) func(msg *message.Message) error {
    return func(msg *message.Message) error {
        e, err := events.NewEventFromJson(msg.Payload)
        if err != nil {
            return err
        }

        // Convert events to UI messages
        switch e_ := e.(type) {
        case *events.EventError:
            p.Send(conversation2.StreamCompletionError{...})
        case *events.EventPartialCompletion:
            p.Send(conversation2.StreamCompletionMsg{...})
        case *events.EventFinal:
            p.Send(conversation2.StreamDoneMsg{...})
        // ... handle other events
        }
        return nil
    }
}
```

Key responsibilities:

- Converts Watermill messages to Bubbletea messages
- Handles different event types appropriately
- Maintains conversation state
- Manages UI updates

## References

- [Watermill Documentation](https://watermill.io/)
- [Bubbletea Documentation](https://github.com/charmbracelet/bubbletea)
- [Errgroup Package](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [Geppetto Events Package](https://github.com/go-go-golems/geppetto/tree/main/pkg/events)
- [Pinocchio UI Package](https://github.com/go-go-golems/pinocchio/tree/main/pkg/ui)

## Putting It All Together: Orchestration and Flow

The previous sections described the individual components of the Bobatea/Pinocchio chat UI system. This section focuses on how these components are orchestrated and how information flows between them, particularly using the `pinocchio/pkg/cmds/cmd.go` implementation as a reference.

### Initialization and Setup

The process typically starts within a `PinocchioCommand`'s execution logic (e.g., `RunIntoWriter` or `RunWithOptions`).

1.  **Configuration Loading:** Command-line flags and configuration files are parsed using `glazed` layers (`layers.ParsedLayers`). This includes UI settings (`cmdlayers.HelpersSettings`) and AI step settings (`settings.StepSettings`).
2.  **Conversation Manager:** A `conversation.Manager` is created using `CreateConversationManager`. This manager is initialized with potential system prompts, existing messages, variables from parsed layers, and image paths. Autosave settings are also configured here.
3.  **Event Router:** An `events.EventRouter` is instantiated. This router will handle the communication between the AI step (backend) and the UI.
4.  **Step Factory:** An `ai.StepFactory` (often `ai.StandardStepFactory`) is created, configured with the loaded `StepSettings`. This factory is responsible for creating the actual `chat.Step` instance that will perform the AI interaction.
5.  **Run Mode Determination:** Based on flags like `--interactive` or `--chat`, the execution mode (`run.RunModeBlocking`, `run.RunModeInteractive`, `run.RunModeChat`) is determined.

### Execution Flow (Blocking vs. Chat)

The system supports different execution modes:

**1. Blocking Mode (`runBlocking`)**

- **Step Creation:** A `chat.Step` is created using the `StepFactory`.
- **Router Setup (Optional):** If an `EventRouter` is provided:
  - The step is potentially recreated using `chat.WithPublishedTopic` to send events to the `chat` topic on the router.
  - A printer handler (`events.StepPrinterFunc` or `events.NewStructuredPrinter`) is added to the `chat` topic. This handler formats and writes step events (like partial completions) directly to the output writer (`io.Writer`).
  - The `EventRouter` is started in a separate goroutine using an `errgroup.Group`.
  - Another goroutine waits for the router to be running (`rc.Router.Running()`) and then executes the core step logic (`runStepAndCollectMessages`).
- **Step Execution:** The `runStepAndCollectMessages` function takes the current conversation from the `ConversationManager`, resolves it into a step input (`steps.Resolve`), binds it to the `chat.Step` (`steps.Bind`), and iterates through the result channel. Any resulting messages are appended back to the `ConversationManager`.
- **Output:** In blocking mode with a router, output comes primarily from the printer handler attached to the event router. Without a router, the final conversation might be printed depending on other settings.

**2. Chat/Interactive Mode (`runChat`)**

This mode introduces the Bubbletea UI.

- **Router Requirement:** An `EventRouter` is mandatory for chat modes.
- **Terminal Setup:** Bubbletea program options are configured, potentially redirecting output to `stderr` if `stdout` is not a TTY and using the alternate screen buffer.
- **Step Creation:** The `chat.Step` is created specifically to publish events to the `ui` topic (`chat.WithPublishedTopic(rc.Router.Publisher, "ui")`). Streaming is enabled (`rc.StepFactory.Settings.Chat.Stream = true`).
- **Router Goroutine:** The `EventRouter` is started in a background goroutine managed by an `errgroup.Group`. This goroutine also handles graceful shutdown via context cancellation.
- **UI Goroutine:** A second goroutine is launched:
  - It waits for the router to start (`rc.Router.Running()`).
  - **Interactive Pre-flight (Optional):** If in `RunModeInteractive`, an _initial_ blocking step is executed first (similar to `runBlocking`) to get the first response. This step publishes to the `chat` topic, and a printer handler displays the output. Afterwards, the user might be prompted (`askForChatContinuation`) whether to proceed to the full chat UI.
  - **UI Initialization:**
    - A `ui.StepBackend` is created, wrapping the `chat.Step` designated for the UI.
    - The `bobatea_chat.InitialModel` is created, passing the `ConversationManager` and the `StepBackend`. Options like `WithTitle` and `WithAutoStartBackend` (which triggers an initial `StartBackendMsg`) are applied.
    - A `tea.Program` (the Bubbletea application) is initialized with the model and options.
  - **UI Event Handler:** The crucial link is made: `ui.StepChatForwardFunc(p)` is added as a handler to the `ui` topic on the router. This function receives `message.Message` objects from the router (published by the `chat.Step`), decodes them into `events.Event` types, translates them into corresponding `conversationui` messages (like `StreamCompletionMsg`, `StreamDoneMsg`), and sends them to the Bubbletea program (`p.Send(...)`).
  - **Run UI:** The Bubbletea program is started (`p.Run()`).
- **Concurrency:** The `errgroup.Wait()` ensures the command only exits after both the router and the UI goroutines have completed (or one has errored). Context cancellation (`cancel()`) is used to signal shutdown between goroutines.

### Event Flow in Chat Mode

1.  **User Input:** User types in the `textarea` component of the `bobatea_chat.model`.
2.  **Submit:** User hits Enter (or the submit key). The `SubmitMessageMsg` is handled.
3.  **Append Message:** The user's text is added to the `ConversationManager` as a `RoleUser` message.
4.  **Start Backend:** A `StartBackendMsg` is triggered (either automatically via `WithAutoStartBackend` or by the submit action). The `model.startBackend` method changes the state to `StateStreamCompletion` and calls `backend.Start(...)`.
5.  **Step Execution:** The `StepBackend.Start` method calls `step.Start(...)` on the underlying `chat.Step`, providing the current conversation history. This returns a `StepResult` channel. The backend returns a `tea.Cmd` that listens on this channel in the background.
6.  **Step Events:** As the `chat.Step` executes (e.g., calls an LLM API), it publishes events (`EventPartialCompletion`, `EventFinal`, etc.) to the `ui` topic via the `EventRouter`.
7.  **Router Forwarding:** The `EventRouter` delivers these events to the registered `ui.StepChatForwardFunc`.
8.  **Event Translation:** `StepChatForwardFunc` converts the `events.Event` into a `conversationui` message (e.g., `StreamCompletionMsg`).
9.  **UI Update:** The translated message is sent to the Bubbletea program (`p.Send`). The `bobatea_chat.model.Update` method receives this message.
10. **Conversation UI Update:** The message is passed to the `conversationui.Model`, which updates its internal state (e.g., appending delta to the last message).
11. **Viewport Update:** The `bobatea_chat.model` re-renders the `conversationui.Model` view, updates the `viewport.Model` content, and potentially scrolls (`viewport.GotoBottom()`).
12. **Step Completion:** When the `chat.Step` finishes, the channel monitored by the `tea.Cmd` returned in step 5 closes.
13. **Backend Finished:** This triggers a `boba_chat.BackendFinishedMsg`.
14. **Cleanup:** The `model.finishCompletion` method handles this message, transitions the state back to `StateUserInput`, clears the text area, and re-enables input.

This intricate interplay of components, goroutines, channels, and event routing allows for a responsive chat interface while managing the asynchronous nature of AI step execution.
