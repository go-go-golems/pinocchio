package chatrunner

import (
    "context"
    "fmt"
    "os"
    "github.com/go-go-golems/geppetto/pkg/inference/engine"
    "github.com/go-go-golems/geppetto/pkg/inference/engine/factory"
    "github.com/go-go-golems/geppetto/pkg/inference/middleware"
    "github.com/go-go-golems/geppetto/pkg/turns"
    "io"

    tea "github.com/charmbracelet/bubbletea"
    bobachat "github.com/go-go-golems/bobatea/pkg/chat" // Alias for clarity
    geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
    "github.com/go-go-golems/geppetto/pkg/events"
    "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
    "github.com/go-go-golems/pinocchio/pkg/ui"
    "github.com/mattn/go-isatty" // Needed for askForChatContinuation
    "github.com/pkg/errors"
    "github.com/rs/zerolog/log"
    input "github.com/tcnksm/go-input" // Needed for askForChatContinuation
    "golang.org/x/sync/errgroup"
)

// RunMode defines the execution mode for the chat session.
type RunMode string

const (
	RunModeChat        RunMode = "chat"
	RunModeInteractive RunMode = "interactive"
	RunModeBlocking    RunMode = "blocking"
)

// ChatSession holds the validated configuration and executes the chat logic.
// It's typically created and run by the ChatBuilder.
type ChatSession struct {
	ctx            context.Context
	engineFactory  factory.EngineFactory
	settings       *settings.StepSettings
	manager        geppetto_conversation.Manager
	uiOptions      []bobachat.ModelOption
	programOptions []tea.ProgramOption
	mode           RunMode
	outputWriter   io.Writer
	router         *events.EventRouter // Optional external router
}

// Run executes the chat session based on its configured mode.
func (cs *ChatSession) Run() error {
	switch cs.mode {
	case RunModeChat:
		return cs.runChatInternal()
	case RunModeInteractive:
		return cs.runInteractiveInternal()
	case RunModeBlocking:
		return cs.runBlockingInternal()
	default:
		return errors.Errorf("unknown run mode: %v", cs.mode)
	}
}

// runChatInternal handles the pure chat UI mode.
func (cs *ChatSession) runChatInternal() error {
	router := cs.router
	var err error
	if router == nil {
		router, err = events.NewEventRouter()
		if err != nil {
			return errors.Wrap(err, "failed to create event router")
		}
	}

	// Create engine with UI sink for event publishing
    uiSink := middleware.NewWatermillSink(router.Publisher, "ui")
    log.Debug().Str("component", "chatrunner").Msg("Created UI watermill sink")
    engine, err := cs.engineFactory.CreateEngine(cs.settings, engine.WithSink(uiSink))
	if err != nil {
		return errors.Wrap(err, "failed to create engine from factory")
	}
    log.Debug().Str("component", "chatrunner").Msg("Engine created with sink")

	eg, childCtx := errgroup.WithContext(cs.ctx)
	childCtx, cancel := context.WithCancel(childCtx) // Create cancellable context if router is internal

    f := func() {
		cancel()
		defer func(router *events.EventRouter) {
			log.Debug().Msg("Closing router")
			_ = router.Close()
			log.Debug().Msg("Router closed")
		}(router)
	}
	if !router.IsRunning() {
		eg.Go(func() error {
			defer f()

			return router.Run(childCtx)
		})
	}

	// UI Goroutine
	eg.Go(func() error {
		log.Debug().Msg("Starting UI goroutine")
		defer f()

		<-router.Running()

		// RunHandlers blocks until the router is ready and handler is registered.
		log.Debug().Msg("Running router handlers")
        if err := router.RunHandlers(childCtx); err != nil {
			// Don't wrap context cancelled/closed errors if the context was intentionally cancelled
			if errors.Is(err, context.Canceled) && childCtx.Err() == context.Canceled {
				log.Debug().Msg("Router handlers stopped due to context cancellation")
				return nil // Normal exit path on cancellation
			}
			return errors.Wrap(err, "failed to run router handlers")
		}
		log.Debug().Msg("Router handlers running")
        log.Debug().Str("component", "chatrunner").Msg("Router handlers running")

        backend := ui.NewEngineBackend(engine)
        // Seed backend from the existing conversation (system/history) so chat UI reflects prior run
        if cs.manager != nil {
            backend.SetSeedFromConversation(cs.manager.GetConversation())
            log.Debug().Str("component", "chatrunner").Msg("Seeded backend from conversation for chat UI")
        }
        model := bobachat.InitialModel(backend, cs.uiOptions...)
		p := tea.NewProgram(model, cs.programOptions...)

		// Setup forwarding handler
		log.Debug().Msg("Adding UI event handler")
        log.Debug().Str("component", "chatrunner").Msg("Adding UI event handler")
        router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p)) // Use the forwarding func

		err = router.RunHandlers(childCtx)
		if err != nil {
			return errors.Wrap(err, "failed to run router handlers")
		}

		// Run the UI program, which blocks until quit.
		log.Debug().Msg("Running Bubbletea program")
        log.Debug().Str("component", "chatrunner").Msg("Starting Bubble Tea program")
        _, runErr := p.Run()
		log.Debug().Err(runErr).Msg("Bubbletea program finished")
        log.Debug().Err(runErr).Str("component", "chatrunner").Msg("Bubble Tea program finished")

		// If the UI exits (even successfully), cancel the context
		// to signal the router goroutine (if internal) to stop.
		// This is handled by the defer cancel() above.

		// Don't return context cancellation errors if the context was cancelled externally
		if errors.Is(runErr, context.Canceled) && childCtx.Err() == context.Canceled {
			return nil
		}
		return runErr // Return UI error (or nil) to errgroup
	})

	log.Debug().Msg("Waiting for errgroup")
	err = eg.Wait()
	log.Debug().Err(err).Msg("Errgroup finished")

	// Don't return context cancellation errors if the original context was cancelled
	if errors.Is(err, context.Canceled) && cs.ctx.Err() == context.Canceled {
		return nil
	}
	return err
}

// runBlockingInternal handles non-interactive execution using Engine directly.
func (cs *ChatSession) runBlockingInternal() error {
	// Create engine for blocking execution (no event sink needed)
	engine, err := cs.engineFactory.CreateEngine(cs.settings)
	if err != nil {
		return errors.Wrap(err, "failed to create engine for blocking execution")
	}

	// Get current conversation and seed a Turn
	conversation_ := cs.manager.GetConversation()
	seed := &turns.Turn{}
	turns.AppendBlocks(seed, turns.BlocksFromConversationDelta(conversation_, 0)...)

	// Run inference directly on the Turn
	updatedTurn, err := engine.RunInference(cs.ctx, seed)
	if err != nil {
		// Don't return context cancellation errors if the context was cancelled externally
		if errors.Is(err, context.Canceled) && cs.ctx.Err() == context.Canceled {
			log.Debug().Msg("Blocking inference cancelled by context")
			return nil // Exit gracefully
		}
		return errors.Wrap(err, "inference failed")
	}

	// Convert back to conversation and extract only the new messages
	conv := turns.BuildConversationFromTurn(updatedTurn)
	newMessages := conv[len(conversation_):]

	// Append the new messages to the conversation
	if err := cs.manager.AppendMessages(newMessages...); err != nil {
		return fmt.Errorf("failed to append messages: %w", err)
	}

	// Print the last message content to the output writer
	if len(newMessages) > 0 {
		lastMsg := newMessages[len(newMessages)-1]
		// TODO: Handle different content types more robustly
		if content, ok := lastMsg.Content.(*geppetto_conversation.ChatMessageContent); ok {
			_, err := fmt.Fprintln(cs.outputWriter, content.View())
			if err != nil {
				return errors.Wrap(err, "failed to write output")
			}
		} else {
			_, err := fmt.Fprintf(cs.outputWriter, "%v", lastMsg.Content)
			if err != nil {
				return errors.Wrap(err, "failed to write output")
			}
		}
	}

	return nil
}

// runInteractiveInternal handles initial blocking run + optional chat transition.
func (cs *ChatSession) runInteractiveInternal() error {
	// 1. Run blocking step first
	log.Debug().Msg("Running initial blocking step for interactive mode")
	err := cs.runBlockingInternal()
	if err != nil {
		// Allow context cancelled errors to pass through without stopping interaction
		if errors.Is(err, context.Canceled) && cs.ctx.Err() == context.Canceled {
			log.Debug().Msg("Initial blocking step cancelled by context")
			return nil // Exit if context was cancelled externally
		}
		return errors.Wrap(err, "error during initial blocking step")
	}

	// 2. Check if we should ask (TTY available?)
	// Use Stderr for prompt asking, as Stdout might be redirected.
	isOutputTerminal := isatty.IsTerminal(os.Stderr.Fd())
	if !isOutputTerminal {
		log.Debug().Msg("Stderr is not a TTY, skipping chat continuation prompt")
		return nil // Don't proceed to chat if not interactive
	}

	// 3. Ask user if they want to continue
	continueInChat, err := askForChatContinuation(os.Stderr) // Ask on Stderr
	if err != nil {
		return errors.Wrap(err, "failed to ask for chat continuation")
	}

	if !continueInChat {
		log.Debug().Msg("User chose not to continue in chat mode")
		return nil
	}

	log.Debug().Msg("User chose to continue, starting chat UI")
	// 4. Run chat UI
	// We need to adjust program options for interactive mode potentially
	// (e.g., maybe don't use AltScreen if already interacted with?)
	// For now, use the same options.
	return cs.runChatInternal()
}

// --- ChatBuilder ---

// ChatBuilder provides a fluent API for configuring and running a chat session.
type ChatBuilder struct {
	err            error // To collect errors during build steps
	ctx            context.Context
	engineFactory  factory.EngineFactory
	settings       *settings.StepSettings
	manager        geppetto_conversation.Manager
	uiOptions      []bobachat.ModelOption
	programOptions []tea.ProgramOption
	mode           RunMode
	outputWriter   io.Writer
	router         *events.EventRouter
}

// NewChatBuilder creates a new builder with default settings.
func NewChatBuilder() *ChatBuilder {
	return &ChatBuilder{
		ctx:            context.Background(), // Default context
		manager:        geppetto_conversation.NewManager(),
		programOptions: []tea.ProgramOption{tea.WithMouseCellMotion(), tea.WithAltScreen()},
		uiOptions:      []bobachat.ModelOption{bobachat.WithTitle("Chat")},
		outputWriter:   os.Stdout,
		mode:           RunModeChat, // Default mode
	}
}

// WithContext sets the context for the chat session.
func (b *ChatBuilder) WithContext(ctx context.Context) *ChatBuilder {
	if b.err != nil {
		return b
	}
	if ctx == nil {
		b.err = errors.New("context cannot be nil")
		return b
	}
	b.ctx = ctx
	return b
}

// WithManager sets the conversation manager. (Required)
func (b *ChatBuilder) WithManager(manager geppetto_conversation.Manager) *ChatBuilder {
	if b.err != nil {
		return b
	}
	if manager == nil {
		b.err = errors.New("manager cannot be nil")
		return b
	}
	b.manager = manager
	return b
}

// WithEngineFactory sets the factory used to create engines. (Required)
func (b *ChatBuilder) WithEngineFactory(factory factory.EngineFactory) *ChatBuilder {
	if b.err != nil {
		return b
	}
	if factory == nil {
		b.err = errors.New("engine factory cannot be nil")
		return b
	}
	b.engineFactory = factory
	return b
}

// WithSettings sets the step settings for engine configuration. (Required)
func (b *ChatBuilder) WithSettings(settings *settings.StepSettings) *ChatBuilder {
	if b.err != nil {
		return b
	}
	if settings == nil {
		b.err = errors.New("settings cannot be nil")
		return b
	}
	b.settings = settings
	return b
}

// WithUIOptions adds options for configuring the bobatea chat model.
func (b *ChatBuilder) WithUIOptions(opts ...bobachat.ModelOption) *ChatBuilder {
	if b.err != nil {
		return b
	}
	b.uiOptions = append(b.uiOptions, opts...)
	return b
}

// WithProgramOptions adds options for configuring the bubbletea program.
func (b *ChatBuilder) WithProgramOptions(opts ...tea.ProgramOption) *ChatBuilder {
	if b.err != nil {
		return b
	}
	b.programOptions = append(b.programOptions, opts...)
	return b
}

// WithMode sets the execution mode (chat, interactive, blocking).
func (b *ChatBuilder) WithMode(mode RunMode) *ChatBuilder {
	if b.err != nil {
		return b
	}
	switch mode {
	case RunModeChat, RunModeInteractive, RunModeBlocking:
		b.mode = mode
	default:
		b.err = errors.Errorf("invalid run mode: %s", mode)
	}
	return b
}

// WithOutputWriter sets the writer for blocking or interactive modes.
// Defaults to os.Stdout.
func (b *ChatBuilder) WithOutputWriter(w io.Writer) *ChatBuilder {
	if b.err != nil {
		return b
	}
	if w == nil {
		b.err = errors.New("output writer cannot be nil")
		return b
	}
	b.outputWriter = w
	return b
}

// WithExternalRouter provides an existing EventRouter instance to use.
// If not provided, an internal router will be created and managed.
func (b *ChatBuilder) WithExternalRouter(router *events.EventRouter) *ChatBuilder {
	if b.err != nil {
		return b
	}
	b.router = router
	return b
}

// Run executes the chat session after validating the builder configuration.
func (b *ChatBuilder) Build() (*ChatSession, error) {
	// Check for accumulated errors during build steps
	if b.err != nil {
		return nil, b.err
	}

	// Final validation of required fields
	if b.manager == nil {
		return nil, errors.New("manager is required (use WithManager)")
	}
	if b.engineFactory == nil {
		return nil, errors.New("engine factory is required (use WithEngineFactory)")
	}
	if b.settings == nil {
		return nil, errors.New("settings is required (use WithSettings)")
	}
	if b.mode == "" {
		// Should be set by default or WithMode, but check anyway
		return nil, errors.New("run mode is required (use WithMode)")
	}

	// Validate dependencies between fields
	if (b.mode == RunModeBlocking || b.mode == RunModeInteractive) && b.outputWriter == nil {
		// Should be set by default, but check anyway
		return nil, errors.New("output writer cannot be nil for blocking or interactive mode (use WithOutputWriter or rely on default)")
	}

	// Create the ChatSession instance from the builder's state
	session := &ChatSession{
		ctx:            b.ctx,
		engineFactory:  b.engineFactory,
		settings:       b.settings,
		manager:        b.manager,
		uiOptions:      b.uiOptions,
		programOptions: b.programOptions,
		mode:           b.mode,
		outputWriter:   b.outputWriter,
		router:         b.router,
	}

	return session, nil
}

// askForChatContinuation prompts the user on the given writer (should be a TTY like os.Stderr)
// whether they want to continue in chat mode.
func askForChatContinuation(tty io.ReadWriter) (bool, error) {
	// Ensure the writer is provided and likely a TTY before proceeding.
	// The check should happen before calling this function.

	ui := &input.UI{
		Writer: tty,
		Reader: tty.(io.Reader), // Assuming the ReadWriter is also a Reader
	}

	// Use Fprintf to write to the specific tty
	_, _ = fmt.Fprint(tty, "\n") // Add newline before prompt
	query := "Do you want to continue in chat mode? [Y/n]"
	answer, err := ui.Ask(query, &input.Options{
		Default:  "y", // Default to yes
		Required: true,
		Loop:     true,
		ValidateFunc: func(answer string) error {
			switch answer {
			case "y", "Y", "n", "N", "": // Allow empty for default 'y'
				return nil
			default:
				return errors.Errorf("please enter 'y' or 'n'")
			}
		},
	})

	if err != nil {
		// Avoid printing errors directly to stdout/stderr here, return them
		return false, errors.Wrap(err, "failed to get user input")
	}

	_, _ = fmt.Fprint(tty, "\n") // Add newline after prompt

	return answer == "y" || answer == "Y" || answer == "", nil // Yes if 'y', 'Y', or empty (default)
}
