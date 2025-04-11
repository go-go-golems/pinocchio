# Pinocchio Chat UI Setup API - Design Ideas

This document outlines potential approaches to refactor the setup process for the Bobatea/Pinocchio chat UI, aiming to create a more straightforward and reusable API for integrating `chat.Step` implementations with the interactive UI.

The current setup, demonstrated in `pinocchio/pkg/cmds/cmd.go` and simplified in the example `pinocchio/cmd/experiments/ui/main.go`, still involves manual orchestration of components like the `EventRouter`, `StepBackend`, `bobatea_chat.Model`, goroutine management (`errgroup`), context handling, and wiring the event handler. This makes it complex to quickly stand up a new chat interface for different steps or in different contexts, even beyond the complexity shown in `cmd.go`.

**Example Complexity (`pinocchio/cmd/experiments/ui/main.go`):**

```go
// Requires manual setup for:
// - Manager, Step, Router creation
// - Linking Step to Router topic
// - Errgroup, context management
// - Router goroutine
// - UI goroutine (Backend, Model, Program creation)
// - Adding the event handler to the router
// - Running handler setup
// - Waiting for completion
```

Our goal is to encapsulate most of this boilerplate.

## Goals

- **Simplicity:** Reduce the boilerplate code required to launch a chat UI session, making it much simpler than the `main.go` example.
- **Encapsulation:** Hide the underlying complexity of event routing, backend management, goroutine synchronization, and handler wiring.
- **Reusability:** Make it easy to embed or launch the chat UI from different parts of an application.
- **Flexibility:** Allow customization of the Step, Conversation Manager, and basic UI parameters.

## Proposed API Ideas

### Idea 1: The `ChatRunner` Struct (Revised)

Introduce a dedicated struct responsible for managing the entire chat lifecycle. Focuses on taking essential components and handling the orchestration.

```go
package chatrunner

import (
	"context"
	"io" // For blocking/interactive modes

	tea "github.com/charmbracelet/bubbletea"
	bobachat "github.com/go-go-golems/bobatea/pkg/chat" // Alias for clarity
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	steprunner "github.com/go-go-golems/geppetto/pkg/steps/run" // Hypothetical package for step execution logic
	"github.com/go-go-golems/pinocchio/pkg/ui"                  // Assuming StepBackend and Forwarder remain useful
	"golang.org/x/sync/errgroup"
	// Assume run modes defined elsewhere (like pinocchio/pkg/cmds/run)
)

type ChatRunner struct {
	// Required
	step    chat.Step // Should be pre-configured to publish events if needed by UI
	manager conversation.Manager

	// Configuration
	uiOptions      []bobachat.ModelOption
	programOptions []tea.ProgramOption
	router         *events.EventRouter // Allow providing an external router
	// Add options for blocking/interactive writer, initial message etc.
}

// Constructor simplifies initialization
func NewChatRunner(step chat.Step, manager conversation.Manager) *ChatRunner {
	return &ChatRunner{
		step:           step,
		manager:        manager,
		uiOptions:      []bobachat.ModelOption{bobachat.WithTitle("Chat")}, // Default title
		programOptions: []tea.ProgramOption{tea.WithMouseCellMotion(), tea.WithAltScreen()}, // Sensible defaults
	}
}

// Options for further configuration (fluent style within struct methods)
func (cr *ChatRunner) WithUIOptions(opts ...bobachat.ModelOption) *ChatRunner {
	cr.uiOptions = append(cr.uiOptions, opts...)
	return cr
}

func (cr *ChatRunner) WithProgramOptions(opts ...tea.ProgramOption) *ChatRunner {
	cr.programOptions = append(cr.programOptions, opts...)
	return cr
}

func (cr *ChatRunner) WithExternalRouter(router *events.EventRouter) *ChatRunner {
    cr.router = router
    return cr
}


// RunChat launches the full interactive terminal UI.
func (cr *ChatRunner) RunChat(ctx context.Context) error {
	// Use provided router or create a new one
    router := cr.router
    var err error
    if router == nil {
        router, err = events.NewEventRouter()
        if err != nil {
            return err
        }
    }

    // NOTE: Assumption: The provided cr.step is *already* configured
    // to publish events to the desired topic ("ui") if it needs to interact
    // with the router (e.g., using step.AddPublishedTopic beforehand).
    // This avoids complexity around cloning/reconfiguring steps inside the runner.

	eg, childCtx := errgroup.WithContext(ctx)

	// Router Goroutine (only if internally created)
	if cr.router == nil {
		eg.Go(func() error {
			defer router.Close()
			// Ensure router stops if UI goroutine exits or context is cancelled
			<-childCtx.Done()
			return router.Close() // Attempt graceful shutdown
			// return router.Run(childCtx) // Run might block shutdown signal? Needs testing.
		})
	}

	// UI Goroutine
	eg.Go(func() error {
		// TODO(manuel, 2025-04-11) This Running() channel doesn't exist yet
		// We might need to implement it, or simply call RunHandlers() which blocks until ready.
		// <-router.Running() // Wait for router if needed

		backend := ui.NewStepBackend(cr.step) // Uses the StepBackend
		model := bobachat.InitialModel(cr.manager, backend, cr.uiOptions...)
		p := tea.NewProgram(model, cr.programOptions...)

		// Setup forwarding handler - encapsulates this wiring
		router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
		// RunHandlers blocks until the handler is registered and ready.
		if err := router.RunHandlers(childCtx); err != nil {
			return err // Return error to errgroup
		}

		// Run the UI program, which blocks until quit.
		_, runErr := p.Run()
		// If the UI exits (even successfully), cancel the context
		// to signal the router goroutine (if internal) to stop.
		if cr.router == nil {
			// TODO(manuel, 2025-04-11) Need access to cancel() from the parent context here.
            // This design might need the childCtx's cancel func passed down or handled differently.
            // For now, assume context cancellation propagates correctly.
		}
		return runErr // Return UI error to errgroup
	})

	return eg.Wait() // Handles context cancellation and error propagation
}


// TODO: Implement RunBlocking and RunInteractive similarly,
// potentially reusing logic from a dedicated step execution package.
// func (cr *ChatRunner) RunBlocking(ctx context.Context, w io.Writer) error { ... }
// func (cr *ChatRunner) RunInteractive(ctx context.Context, w io.Writer) error { ... }

```

**Pros:**

- Simple `NewChatRunner` constructor for basic use.
- Encapsulates goroutine management and router setup (if not external).
- Clear `RunChat` method hides the complex interaction logic.

**Cons:**

- Step must be pre-configured to publish events to the "ui" topic by the caller.
- Context cancellation between goroutines needs careful handling, especially `p.Run()` exiting.
- Adding `RunBlocking` and `RunInteractive` might bloat the struct or require more complex internal logic.

### Idea 2: Functional Options Builder (Revised)

A builder pattern using functional options, focusing on assembling a `ChatSession` configuration that is then executed.

```go
package chatrunner

// ... imports ...
import "github.com/ThreeDotsLabs/watermill/message" // For StepFactory signature

type ChatSession struct {
	// Configuration built by options
	ctx            context.Context
	stepFactory    func(publisher message.Publisher, topic string) (chat.Step, error) // Required factory
	manager        conversation.Manager                                               // Required manager
	uiOptions      []bobachat.ModelOption
	programOptions []tea.ProgramOption
	mode           run.RunMode // Required mode
	outputWriter   io.Writer   // Required for Blocking/Interactive modes
	router         *events.EventRouter // Optional external router
}

type SessionOption func(*ChatSession) error // Options can return errors for validation

// --- Option examples ---
func WithContext(ctx context.Context) SessionOption {
	return func(cs *ChatSession) error { cs.ctx = ctx; return nil }
}
func WithStepFactory(factory func(publisher message.Publisher, topic string) (chat.Step, error)) SessionOption {
	return func(cs *ChatSession) error { cs.stepFactory = factory; return nil }
}
func WithManager(manager conversation.Manager) SessionOption {
	return func(cs *ChatSession) error { cs.manager = manager; return nil }
}
func WithUIOptions(opts ...bobachat.ModelOption) SessionOption {
	return func(cs *ChatSession) error { cs.uiOptions = append(cs.uiOptions, opts...); return nil }
}
func WithProgramOptions(opts ...tea.ProgramOption) SessionOption {
	return func(cs *ChatSession) error { cs.programOptions = append(cs.programOptions, opts...); return nil }
}
func WithMode(mode run.RunMode) SessionOption {
	return func(cs *ChatSession) error {
		// Validation: Ensure writer is provided for relevant modes
		if (mode == run.RunModeBlocking || mode == run.RunModeInteractive) && cs.outputWriter == nil {
			// This validation needs to happen *after* all options are applied,
			// so better done in NewChatSession or Run. Let's check in Run.
		}
		cs.mode = mode
		return nil
	}
}
func WithOutputWriter(w io.Writer) SessionOption {
	return func(cs *ChatSession) error { cs.outputWriter = w; return nil }
}
func WithExternalRouter(router *events.EventRouter) SessionOption {
    return func(cs *ChatSession) error { cs.router = router; return nil }
}
// --- End Options ---

func NewChatSession(opts ...SessionOption) (*ChatSession, error) {
	session := &ChatSession{ // Sensible defaults
		ctx:            context.Background(),
		programOptions: []tea.ProgramOption{tea.WithMouseCellMotion(), tea.WithAltScreen()},
		uiOptions:      []bobachat.ModelOption{bobachat.WithTitle("Chat")},
		outputWriter:   os.Stdout, // Default writer, might be unused
	}
	for _, opt := range opts {
		if err := opt(session); err != nil {
			return nil, err // Handle configuration errors
		}
	}

	// Final validation
	if session.manager == nil {
		return nil, errors.New("conversation manager is required")
	}
	if session.stepFactory == nil {
		return nil, errors.New("step factory is required")
	}
	if session.mode == "" {
		return nil, errors.New("run mode is required")
	}
     if (session.mode == run.RunModeBlocking || session.mode == run.RunModeInteractive) && session.outputWriter == nil {
         return nil, errors.New("output writer is required for blocking or interactive mode")
     }


	return session, nil
}

func (cs *ChatSession) Run() error {
	// Core execution logic based on cs.mode
	switch cs.mode {
	case run.RunModeChat:
		return cs.runChatInternal() // Implements logic similar to ChatRunner.RunChat
	case run.RunModeInteractive:
		return cs.runInteractiveInternal() // Implements logic for initial blocking + optional chat
	case run.RunModeBlocking:
		return cs.runBlockingInternal() // Implements logic for non-interactive step run
	default:
		return errors.Errorf("unknown run mode: %v", cs.mode)
	}
}

// --- Internal implementation methods ---
// These would contain the errgroup/goroutine/router/UI setup,
// using the cs.stepFactory to create steps with appropriate topics ("ui" or "chat")
// and using cs.outputWriter where needed.

func (cs *ChatSession) runChatInternal() error {
	router := cs.router
	var err error
	if router == nil {
		router, err = events.NewEventRouter()
		if err != nil { return err }
	}

	// Use factory to create step for UI
	step, err := cs.stepFactory(router.Publisher, "ui")
	if err != nil { return err }

	eg, childCtx := errgroup.WithContext(cs.ctx) // Use configured context

	// Router Goroutine (if internal)
	if cs.router == nil {
		eg.Go(func() error {
			defer router.Close()
            <-childCtx.Done() // Wait for cancellation
            return router.Close()
		})
	}

	// UI Goroutine
	eg.Go(func() error {
		// TODO(manuel, 2025-04-11) Router readiness check / RunHandlers timing
		backend := ui.NewStepBackend(step)
		model := bobachat.InitialModel(cs.manager, backend, cs.uiOptions...)
		p := tea.NewProgram(model, cs.programOptions...)

		router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
		if err := router.RunHandlers(childCtx); err != nil {
			return err
		}

		_, runErr := p.Run()
        // TODO(manuel, 2025-04-11) Context cancellation on UI exit
		return runErr
	})

	return eg.Wait()
}

// func (cs *ChatSession) runInteractiveInternal() error { ... }
// func (cs *ChatSession) runBlockingInternal() error { ... }

```

**Pros:**

- Highly configurable via options.
- Uses a `StepFactory` for better flexibility in creating steps for different topics/modes internally.
- Clear separation between configuration (`NewChatSession`) and execution (`Run`).
- Can incorporate validation logic within options or `NewChatSession`.

**Cons:**

- Can lead to many options (though often hidden behind the functional options pattern).
- The caller needs to select the correct `RunMode` and provide necessary components (like `outputWriter`).

### Idea 3: Fluent Builder Pattern

Provides a chainable API for configuration before running.

```go
package chatrunner

// ... imports ...

type ChatBuilder struct {
	// Internal state
	err            error // To collect errors during build steps
	ctx            context.Context
	stepFactory    func(publisher message.Publisher, topic string) (chat.Step, error)
	manager        conversation.Manager
	uiOptions      []bobachat.ModelOption
	programOptions []tea.ProgramOption
	mode           run.RunMode
	outputWriter   io.Writer
    router         *events.EventRouter
}

// Initial constructor
func NewChatBuilder() *ChatBuilder {
	return &ChatBuilder{
		ctx:            context.Background(), // Default context
		programOptions: []tea.ProgramOption{tea.WithMouseCellMotion(), tea.WithAltScreen()},
		uiOptions:      []bobachat.ModelOption{bobachat.WithTitle("Chat")},
		outputWriter:   os.Stdout,
        mode:           run.RunModeChat, // Default mode
	}
}

// Configuration methods (chainable)
func (b *ChatBuilder) WithContext(ctx context.Context) *ChatBuilder {
	if b.err != nil { return b } // Don't proceed if already errored
	b.ctx = ctx
	return b
}

func (b *ChatBuilder) WithManager(manager conversation.Manager) *ChatBuilder {
	if b.err != nil { return b }
	if manager == nil {
		b.err = errors.New("manager cannot be nil")
		return b
	}
	b.manager = manager
	return b
}

func (b *ChatBuilder) WithStepFactory(factory func(publisher message.Publisher, topic string) (chat.Step, error)) *ChatBuilder {
	if b.err != nil { return b }
    if factory == nil {
		b.err = errors.New("step factory cannot be nil")
		return b
	}
	b.stepFactory = factory
	return b
}

func (b *ChatBuilder) WithUIOptions(opts ...bobachat.ModelOption) *ChatBuilder {
	if b.err != nil { return b }
	b.uiOptions = append(b.uiOptions, opts...)
	return b
}

func (b *ChatBuilder) WithProgramOptions(opts ...tea.ProgramOption) *ChatBuilder {
	if b.err != nil { return b }
	b.programOptions = append(b.programOptions, opts...)
	return b
}

func (b *ChatBuilder) WithMode(mode run.RunMode) *ChatBuilder {
	if b.err != nil { return b }
	if mode == "" {
        b.err = errors.New("run mode cannot be empty")
        return b
    }
	b.mode = mode
	return b
}

func (b *ChatBuilder) WithOutputWriter(w io.Writer) *ChatBuilder {
	if b.err != nil { return b }
    if w == nil {
        b.err = errors.New("output writer cannot be nil")
        return b
    }
	b.outputWriter = w
	return b
}

func (b *ChatBuilder) WithExternalRouter(router *events.EventRouter) *ChatBuilder {
    if b.err != nil { return b }
    b.router = router
    return b
}


// Final execution method
func (b *ChatBuilder) Run() error {
	// Check for accumulated errors
	if b.err != nil {
		return b.err
	}

	// Final validation (dependencies between fields)
	if b.manager == nil {
		return errors.New("manager is required")
	}
	if b.stepFactory == nil {
		return errors.New("step factory is required")
	}
    if (b.mode == run.RunModeBlocking || b.mode == run.RunModeInteractive) && b.outputWriter == nil {
        return errors.New("output writer is required for blocking or interactive mode")
    }
    // NOTE: Could default writer to os.Stdout if mode is blocking/interactive and writer is nil


	// Create a ChatSession internally from the builder state
    // This avoids duplicating the runXInternal logic.
	session := &ChatSession{
        ctx:            b.ctx,
        stepFactory:    b.stepFactory,
        manager:        b.manager,
        uiOptions:      b.uiOptions,
        programOptions: b.programOptions,
        mode:           b.mode,
        outputWriter:   b.outputWriter,
        router:         b.router,
    }

	// Delegate to ChatSession's Run logic
	return session.Run() // Assumes ChatSession and its runXInternal methods exist (as in Idea 2)
}

```

**Pros:**

- Fluent, chainable API can be very readable.
- Configuration steps are explicit method calls.
- Can embed validation within each configuration step or at the final `Run()`.

**Cons:**

- Can be more verbose than functional options for simple cases.
- Error handling during the build chain needs careful management (e.g., passing errors along).
- Might feel slightly less idiomatic Go compared to functional options to some developers.

## Common Considerations

- **Step Configuration:** Using a `StepFactory` (as in Ideas 2 & 3) seems essential for allowing the runner/session/builder to correctly configure the step instance with the appropriate event publisher and topic (`"ui"` vs. `"chat"`) depending on the execution mode. Requiring the caller to pre-configure the step (Idea 1) is less robust.
- **Error Handling & Validation:** Needs to be robust during configuration (checking for nil, mode/writer consistency) and execution (goroutine errors).
- **Context Management:** Needs careful implementation to ensure cancellation propagates correctly, especially when the Bubbletea `p.Run()` loop exits. Passing the `cancel` function down or using shared channels might be necessary.
- **Router Readiness:** The UI goroutine needs to reliably wait for the router to be ready and handlers registered before `p.Run()` is called. `router.RunHandlers()` seems suitable for this.
- **Dependencies:** Keep dependencies minimal.

## Recommendation (Updated)

Both **Idea 2 (Functional Options)** and **Idea 3 (Fluent Builder)** offer significant improvements over the manual setup.

- **Idea 2 (Functional Options)** is very idiomatic Go and flexible. It's concise for users familiar with the pattern.
- **Idea 3 (Fluent Builder)** can be more explicit and potentially easier to read for those less familiar with functional options, clearly showing the configuration sequence. It can also cleanly integrate validation within the build chain.

Given the goal of simplifying the complex orchestration, **Idea 3 (Fluent Builder)** might offer a slightly clearer narrative for configuration, hiding the underlying `ChatSession` details more effectively from the end-user perspective during setup. It guides the user through the necessary configuration steps before the final `Run()`.

Therefore, the **Fluent Builder (Idea 3)**, potentially building upon the internal logic structure of **Idea 2 (ChatSession)** for execution, is now the recommended approach.
