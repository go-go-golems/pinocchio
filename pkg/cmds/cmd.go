package cmds

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/conversation/builder"
	"github.com/go-go-golems/geppetto/pkg/events"

	tea "github.com/charmbracelet/bubbletea"
	bobatea_chat "github.com/go-go-golems/bobatea/pkg/chat"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/cmds/run"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/tcnksm/go-input"
	"golang.org/x/sync/errgroup"
)

type PinocchioCommandDescription struct {
	Name      string                            `yaml:"name"`
	Short     string                            `yaml:"short"`
	Long      string                            `yaml:"long,omitempty"`
	Flags     []*parameters.ParameterDefinition `yaml:"flags,omitempty"`
	Arguments []*parameters.ParameterDefinition `yaml:"arguments,omitempty"`
	Layers    []layers.ParameterLayer           `yaml:"layers,omitempty"`

	Prompt       string                  `yaml:"prompt,omitempty"`
	Messages     []*conversation.Message `yaml:"messages,omitempty"`
	SystemPrompt string                  `yaml:"system-prompt,omitempty"`
}

type PinocchioCommand struct {
	*glazedcmds.CommandDescription `yaml:",inline"`
	Prompt                         string                  `yaml:"prompt,omitempty"`
	Messages                       []*conversation.Message `yaml:"messages,omitempty"`
	SystemPrompt                   string                  `yaml:"system-prompt,omitempty"`
}

var _ glazedcmds.WriterCommand = &PinocchioCommand{}

type PinocchioCommandOption func(*PinocchioCommand)

func WithPrompt(prompt string) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		g.Prompt = prompt
	}
}

func WithMessages(messages []*conversation.Message) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		g.Messages = messages
	}
}

func WithSystemPrompt(systemPrompt string) PinocchioCommandOption {
	return func(g *PinocchioCommand) {
		g.SystemPrompt = systemPrompt
	}
}

func NewPinocchioCommand(
	description *glazedcmds.CommandDescription,
	options ...PinocchioCommandOption,
) (*PinocchioCommand, error) {
	helpersParameterLayer, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		return nil, err
	}

	description.Layers.PrependLayers(helpersParameterLayer)

	ret := &PinocchioCommand{
		CommandDescription: description,
	}

	for _, option := range options {
		option(ret)
	}

	return ret, nil
}

// XXX this is a mess with all its run methods and all, it would be good to have a RunOption pattern here:
// - WithStepSettings
// - WithParsedLayers
// - WithPrinter / Handlers
// - WithEngine / Temperature / a whole set of LLM specific parameters
//   - WithMessages
//   - WithPrompt
//   - WithSystemPrompt
//   - WithImages
//   - WithAutosaveSettings
//   - WithVariables
//   - WithRouter
//   - WithStepFactory
//   - WithSettings

// CreateConversationManager creates a new conversation manager with the given settings
func (g *PinocchioCommand) CreateConversationManager(
	variables map[string]interface{},
	options ...builder.ConversationManagerOption,
) (conversation.Manager, error) {
	if g.Prompt != "" && len(g.Messages) != 0 {
		return nil, errors.Errorf("Prompt and messages are mutually exclusive")
	}

	defaultOptions := []builder.ConversationManagerOption{
		builder.WithSystemPrompt(g.SystemPrompt),
		builder.WithMessages(g.Messages),
		builder.WithPrompt(g.Prompt),
		builder.WithVariables(variables),
	}

	// Combine default options with provided options, with provided options taking precedence
	builder, err := builder.NewConversationManagerBuilder(append(defaultOptions, options...)...)
	if err != nil {
		return nil, err
	}

	return builder.Build()
}

// RunIntoWriter runs the command and writes the output into the given writer.
func (g *PinocchioCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	w io.Writer,
) error {
	// Get helpers settings from parsed layers
	helpersSettings := &cmdlayers.HelpersSettings{}
	err := parsedLayers.InitializeStruct(cmdlayers.GeppettoHelpersSlug, helpersSettings)
	if err != nil {
		return errors.Wrap(err, "failed to initialize helpers settings")
	}

	// Update step settings from parsed layers
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		return errors.Wrap(err, "failed to create step settings")
	}
	err = stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to update step settings from parsed layers")
	}

	// Create image paths from helper settings
	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	// First create the conversation manager with all its settings
	manager, err := g.CreateConversationManager(
		parsedLayers.GetDefaultParameterLayer().Parameters.ToMap(),
		builder.WithImages(imagePaths),
		builder.WithAutosaveSettings(builder.AutosaveSettings{
			Enabled:  strings.ToLower(helpersSettings.Autosave.Enabled) == "yes",
			Template: helpersSettings.Autosave.Template,
			Path:     helpersSettings.Autosave.Path,
		}),
	)
	if err != nil {
		return err
	}

	// Determine run mode based on helper settings
	runMode := run.RunModeBlocking
	if helpersSettings.StartInChat {
		runMode = run.RunModeChat
	} else if helpersSettings.Interactive {
		runMode = run.RunModeInteractive
	}

	// Create UI settings from helper settings
	uiSettings := &run.UISettings{
		Interactive:      helpersSettings.Interactive,
		ForceInteractive: helpersSettings.ForceInteractive,
		NonInteractive:   helpersSettings.NonInteractive,
		StartInChat:      helpersSettings.StartInChat,
		PrintPrompt:      helpersSettings.PrintPrompt,
		Output:           helpersSettings.Output,
		WithMetadata:     helpersSettings.WithMetadata,
		FullOutput:       helpersSettings.FullOutput,
	}

	router, err := events.NewEventRouter()
	if err != nil {
		return err
	}

	// Run with options
	messages, err := g.RunWithOptions(ctx,
		run.WithStepSettings(stepSettings),
		run.WithWriter(w),
		run.WithRunMode(runMode),
		run.WithUISettings(uiSettings),
		run.WithConversationManager(manager),
		run.WithRouter(router),
	)
	if err != nil {
		return err
	}

	// If we're just printing the prompt, do that and return
	if helpersSettings.PrintPrompt {
		if len(messages) > 0 {
			_, _ = fmt.Fprintf(w, "%s\n", strings.TrimSpace(messages[len(messages)-1].Content.View()))
		}
	}

	return nil
}

// RunWithOptions executes the command with the given options
func (g *PinocchioCommand) RunWithOptions(ctx context.Context, options ...run.RunOption) ([]*conversation.Message, error) {
	// We need at least one option that provides the manager
	runCtx := &run.RunContext{}

	// Apply options
	for _, opt := range options {
		if err := opt(runCtx); err != nil {
			return nil, err
		}
	}

	// Verify we have a manager
	if runCtx.ConversationManager == nil {
		return nil, errors.New("no conversation manager provided")
	}

	// Create step factory if not provided
	if runCtx.StepFactory == nil {
		runCtx.StepFactory = &ai.StandardStepFactory{
			Settings: runCtx.StepSettings.Clone(),
		}
	}

	// Verify router for chat mode
	if runCtx.RunMode == run.RunModeChat && runCtx.Router == nil {
		return nil, errors.New("chat mode requires a router")
	}

	switch runCtx.RunMode {
	case run.RunModeBlocking:
		return g.runBlocking(ctx, runCtx)
	case run.RunModeInteractive:
		return g.runInteractive(ctx, runCtx)
	case run.RunModeChat:
		return g.runChat(ctx, runCtx)
	default:
		return nil, errors.Errorf("unknown run mode: %v", runCtx.RunMode)
	}
}

// runBlocking handles blocking execution mode
func (g *PinocchioCommand) runBlocking(ctx context.Context, rc *run.RunContext) ([]*conversation.Message, error) {
	chatStep, err := rc.StepFactory.NewStep()
	if err != nil {
		return nil, err
	}

	// If we have a router, set up the printer and run the router loop
	if rc.Router != nil {
		chatStep, err = rc.StepFactory.NewStep(chat.WithPublishedTopic(rc.Router.Publisher, "chat"))
		if err != nil {
			return nil, err
		}

		// Add default printer if none is set
		if rc.UISettings == nil || rc.UISettings.Output == "" {
			rc.Router.AddHandler("chat", "chat", chat.StepPrinterFunc("", rc.Writer))
		} else {
			printer := chat.NewStructuredPrinter(rc.Writer, chat.PrinterOptions{
				Format:          chat.PrinterFormat(rc.UISettings.Output),
				Name:            "",
				IncludeMetadata: rc.UISettings.WithMetadata,
				Full:            rc.UISettings.FullOutput,
			})
			rc.Router.AddHandler("chat", "chat", printer)
		}

		// Start router
		eg := errgroup.Group{}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		eg.Go(func() error {
			defer cancel()
			return rc.Router.Run(ctx)
		})

		eg.Go(func() error {
			defer cancel()
			<-rc.Router.Running()
			return g.runStepAndCollectMessages(ctx, rc, chatStep)
		})

		err = eg.Wait()
		if err != nil {
			return nil, err
		}
	} else {
		// No router, just run the step directly
		err = g.runStepAndCollectMessages(ctx, rc, chatStep)
		if err != nil {
			return nil, err
		}
	}

	return rc.ConversationManager.GetConversation(), nil
}

// runStepAndCollectMessages handles the actual step execution and message collection
func (g *PinocchioCommand) runStepAndCollectMessages(ctx context.Context, rc *run.RunContext, chatStep chat.Step) error {
	conversation_ := rc.ConversationManager.GetConversation()
	messagesM := steps.Resolve(conversation_)
	m := steps.Bind(ctx, messagesM, chatStep)

	for r := range m.GetChannel() {
		if r.Error() != nil {
			return r.Error()
		}
		msg := r.Unwrap()
		rc.ConversationManager.AppendMessages(msg)
	}
	return nil
}

// runInteractive handles interactive execution mode
func (g *PinocchioCommand) runInteractive(ctx context.Context, rc *run.RunContext) ([]*conversation.Message, error) {
	// First run blocking to get initial response
	messages, err := g.runBlocking(ctx, rc)
	if err != nil {
		return nil, err
	}

	// If we're not in interactive mode or it's explicitly disabled, return early
	if rc.UISettings == nil || rc.UISettings.NonInteractive {
		return messages, nil
	}

	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())
	forceInteractive := rc.UISettings.ForceInteractive

	// Check if we should ask for chat continuation
	askChat := (isOutputTerminal || forceInteractive) && !rc.UISettings.NonInteractive
	if !askChat {
		return messages, nil
	}

	// Ask user if they want to continue in chat mode
	continueInChat, err := askForChatContinuation()
	if err != nil {
		return messages, err
	}

	if continueInChat {
		// Switch to chat mode
		rc.RunMode = run.RunModeChat
		return g.runChat(ctx, rc)
	}

	return messages, nil
}

// runChat handles chat execution mode
func (g *PinocchioCommand) runChat(ctx context.Context, rc *run.RunContext) ([]*conversation.Message, error) {
	if rc.Router == nil {
		return nil, errors.New("chat mode requires a router")
	}

	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())

	options := []tea.ProgramOption{
		tea.WithMouseCellMotion(),
	}
	if !isOutputTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	} else {
		options = append(options, tea.WithAltScreen())
	}

	rc.StepFactory.Settings.Chat.Stream = true
	chatStep, err := rc.StepFactory.NewStep(chat.WithPublishedTopic(rc.Router.Publisher, "ui"))
	if err != nil {
		return nil, err
	}

	backend := ui.NewStepBackend(chatStep)

	// Determine if we should auto-start the backend
	autoStartBackend := rc.UISettings != nil && rc.UISettings.StartInChat

	model := bobatea_chat.InitialModel(
		rc.ConversationManager,
		backend,
		bobatea_chat.WithTitle("pinocchio"),
		bobatea_chat.WithAutoStartBackend(autoStartBackend),
	)

	p := tea.NewProgram(
		model,
		options...,
	)

	rc.Router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))

	// Start router
	eg := errgroup.Group{}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		defer cancel()
		return rc.Router.Run(ctx)
	})

	eg.Go(func() error {
		defer cancel()
		<-rc.Router.Running()
		_, err := p.Run()
		return err
	})

	err = eg.Wait()
	if err != nil {
		return nil, err
	}

	return rc.ConversationManager.GetConversation(), nil
}

// Helper function to ask user about continuing in chat mode
func askForChatContinuation() (bool, error) {
	tty_, err := bobatea_chat.OpenTTY()
	if err != nil {
		return false, err
	}
	defer func() {
		err := tty_.Close()
		if err != nil {
			fmt.Println("Failed to close tty:", err)
		}
	}()

	ui := &input.UI{
		Writer: tty_,
		Reader: tty_,
	}

	query := "\nDo you want to continue in chat? [y/n]"
	answer, err := ui.Ask(query, &input.Options{
		Default:  "y",
		Required: true,
		Loop:     true,
		ValidateFunc: func(answer string) error {
			switch answer {
			case "y", "Y", "n", "N":
				return nil
			default:
				return errors.Errorf("please enter 'y' or 'n'")
			}
		},
	})

	if err != nil {
		fmt.Println("Failed to get user input:", err)
		return false, err
	}

	return answer == "y" || answer == "Y", nil
}
