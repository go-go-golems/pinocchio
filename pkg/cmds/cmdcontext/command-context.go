package cmdcontext

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	bobatea_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/tcnksm/go-input"
	"golang.org/x/sync/errgroup"
)

type CommandContext struct {
	Router              *events.EventRouter
	ConversationManager conversation.Manager
	StepFactory         *ai.StandardStepFactory
	Settings            *cmdlayers.HelpersSettings
}

type CommandContextOption func(*CommandContext) error

func WithRouter(router *events.EventRouter) CommandContextOption {
	return func(c *CommandContext) error {
		c.Router = router
		return nil
	}
}

func WithStepFactory(factory *ai.StandardStepFactory) CommandContextOption {
	return func(c *CommandContext) error {
		c.StepFactory = factory
		return nil
	}
}

func WithSettings(settings *cmdlayers.HelpersSettings) CommandContextOption {
	return func(c *CommandContext) error {
		c.Settings = settings
		return nil
	}
}

func NewCommandContext(conversationManager conversation.Manager, options ...CommandContextOption) (*CommandContext, error) {
	ctx := &CommandContext{
		ConversationManager: conversationManager,
	}
	for _, opt := range options {
		if err := opt(ctx); err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

func NewCommandContextFromSettings(
	initialStepSettings *settings.StepSettings,
	conversationManager conversation.Manager,
	helpersSettings *cmdlayers.HelpersSettings,
	options ...CommandContextOption,
) (*CommandContext, error) {
	stepFactory := &ai.StandardStepFactory{
		Settings: initialStepSettings,
	}

	router, err := events.NewEventRouter()
	if err != nil {
		return nil, err
	}

	options_ := append(options,
		WithRouter(router),
		WithStepFactory(stepFactory),
		WithSettings(helpersSettings),
	)

	return NewCommandContext(conversationManager, options_...)
}

func (c *CommandContext) Close() error {
	if c.Router != nil {
		return c.Router.Close()
	}
	return nil
}

func (c *CommandContext) StartInitialStep(
	ctx context.Context,
) (steps.StepResult[*conversation.Message], error) {
	chatStep, err := c.StepFactory.NewStep(chat.WithPublishedTopic(c.Router.Publisher, "chat"))
	if err != nil {
		return nil, err
	}

	conversation_ := c.ConversationManager.GetConversation()
	if c.Settings.PrintPrompt {
		fmt.Println(conversation_.GetSinglePrompt())
		return nil, nil
	}

	messagesM := steps.Resolve(conversation_)
	m := steps.Bind[conversation.Conversation, *conversation.Message](ctx, messagesM, chatStep)

	return m, nil
}

func (c *CommandContext) handleChat(
	ctx context.Context,
	autoStartBackend bool,
) error {
	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())

	options := []tea.ProgramOption{
		tea.WithMouseCellMotion(),
	}
	if !isOutputTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	} else {
		options = append(options, tea.WithAltScreen())
	}

	c.StepFactory.Settings.Chat.Stream = true
	chatStep, err := c.StepFactory.NewStep(chat.WithPublishedTopic(c.Router.Publisher, "ui"))
	if err != nil {
		return err
	}

	backend := ui.NewStepBackend(chatStep)

	model := bobatea_chat.InitialModel(
		c.ConversationManager,
		backend,
		bobatea_chat.WithTitle("pinocchio"),
		bobatea_chat.WithAutoStartBackend(autoStartBackend),
	)

	p := tea.NewProgram(
		model,
		options...,
	)

	c.Router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
	err = c.Router.RunHandlers(ctx)
	if err != nil {
		return err
	}

	if _, err = p.Run(); err != nil {
		return err
	}
	return nil
}

func (c *CommandContext) handleInteractiveContinuation(
	ctx context.Context,
) error {
	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())
	forceInteractive := c.Settings.ForceInteractive

	if c.Settings.NonInteractive {
		c.Settings.Interactive = false
	}

	continueInChat := c.Settings.Interactive
	askChat := (isOutputTerminal || forceInteractive) && c.Settings.Interactive && !c.Settings.NonInteractive

	lengthBeforeChat := len(c.ConversationManager.GetConversation())

	if askChat {
		var err error
		continueInChat, err = c.askForChatContinuation(continueInChat)
		if err != nil {
			return err
		}
	}

	if continueInChat {
		c.StepFactory.Settings.Chat.Stream = true
		err := c.handleChat(ctx, false)
		if err != nil {
			return err
		}

		fmt.Printf("\n---\n")
		for idx, msg := range c.ConversationManager.GetConversation() {
			if idx < lengthBeforeChat {
				continue
			}
			view := msg.Content.View()
			fmt.Printf("\n%s\n", view)
		}
	}

	return nil
}

func (c *CommandContext) askForChatContinuation(continueInChat bool) (bool, error) {
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

	switch answer {
	case "y", "Y":
		continueInChat = true
	case "n", "N":
		return false, nil
	}
	return continueInChat, nil
}

func (c *CommandContext) SetupPrinter(w io.Writer) func(msg *message.Message) error {
	if c.Settings.Output != "text" || c.Settings.WithMetadata || c.Settings.FullOutput {
		return chat.NewStructuredPrinter(w, chat.PrinterOptions{
			Format:          chat.PrinterFormat(c.Settings.Output),
			Name:            "",
			IncludeMetadata: c.Settings.WithMetadata,
			Full:            c.Settings.FullOutput,
		})
	}
	return chat.StepPrinterFunc("", w)
}

func (c *CommandContext) handleNonChatMode(
	ctx context.Context,
	w io.Writer,
) error {
	printer := c.SetupPrinter(w)
	c.Router.AddHandler("chat", "chat", printer)
	err := c.Router.RunHandlers(ctx)
	if err != nil {
		return err
	}

	m, err := c.StartInitialStep(ctx)
	if err != nil {
		return err
	}

	res := m.Return()
	for _, msg := range res {
		s, err := msg.Value()
		if err != nil {
			return err
		}
		c.ConversationManager.AppendMessages(s)
	}

	if err := c.handleInteractiveContinuation(ctx); err != nil {
		return err
	}

	return nil
}

func (c *CommandContext) Run(ctx context.Context, w io.Writer) error {
	eg := errgroup.Group{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		defer cancel()
		return c.Router.Run(ctx)
	})

	eg.Go(func() error {
		defer cancel()
		<-c.Router.Running()
		if c.Settings.StartInChat {
			return c.handleChat(ctx, true)
		}
		return c.handleNonChatMode(ctx, w)
	})

	return eg.Wait()
}

func (c *CommandContext) RunStepBlocking(
	ctx context.Context,
) ([]*conversation.Message, error) {
	eg := errgroup.Group{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		defer cancel()
		return c.Router.Run(ctx)
	})

	eg.Go(func() error {
		defer cancel()
		<-c.Router.Running()

		m, err := c.StartInitialStep(ctx)
		if err != nil {
			return err
		}

		for r := range m.GetChannel() {
			if r.Error() != nil {
				return r.Error()
			}
			msg := r.Unwrap()
			c.ConversationManager.AppendMessages(msg)
		}
		return nil
	})

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	return c.ConversationManager.GetConversation(), nil
}
