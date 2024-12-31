package cmds

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	tea "github.com/charmbracelet/bubbletea"
	bobatea_chat "github.com/go-go-golems/bobatea/pkg/chat"
	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/events"
	"github.com/go-go-golems/geppetto/pkg/steps"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/helpers/templating"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdcontext"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/go-go-golems/pinocchio/pkg/ui"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/tcnksm/go-input"
	"golang.org/x/sync/errgroup"
)

type GeppettoCommandDescription struct {
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

type GeppettoCommand struct {
	*glazedcmds.CommandDescription `yaml:",inline"`
	StepSettings                   *settings.StepSettings  `yaml:"stepSettings,omitempty"`
	Prompt                         string                  `yaml:"prompt,omitempty"`
	Messages                       []*conversation.Message `yaml:"messages,omitempty"`
	SystemPrompt                   string                  `yaml:"system-prompt,omitempty"`
}

var _ glazedcmds.WriterCommand = &GeppettoCommand{}

type GeppettoCommandOption func(*GeppettoCommand)

func WithPrompt(prompt string) GeppettoCommandOption {
	return func(g *GeppettoCommand) {
		g.Prompt = prompt
	}
}

func WithMessages(messages []*conversation.Message) GeppettoCommandOption {
	return func(g *GeppettoCommand) {
		g.Messages = messages
	}
}

func WithSystemPrompt(systemPrompt string) GeppettoCommandOption {
	return func(g *GeppettoCommand) {
		g.SystemPrompt = systemPrompt
	}
}

func NewGeppettoCommand(
	description *glazedcmds.CommandDescription,
	settings *settings.StepSettings,
	options ...GeppettoCommandOption,
) (*GeppettoCommand, error) {
	helpersParameterLayer, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		return nil, err
	}

	description.Layers.PrependLayers(helpersParameterLayer)

	ret := &GeppettoCommand{
		CommandDescription: description,
		StepSettings:       settings,
	}

	for _, option := range options {
		option(ret)
	}

	return ret, nil
}

func (g *GeppettoCommand) initializeConversationManager(
	conversationManager conversation.Manager,
	helperSettings *cmdlayers.HelpersSettings,
	ps map[string]interface{},
) error {
	if g.SystemPrompt != "" {
		systemPromptTemplate, err := templating.CreateTemplate("system-prompt").Parse(g.SystemPrompt)
		if err != nil {
			return err
		}

		var systemPromptBuffer strings.Builder
		err = systemPromptTemplate.Execute(&systemPromptBuffer, ps)
		if err != nil {
			return err
		}

		// TODO(manuel, 2023-12-07) Only do this conditionally, or maybe if the system prompt hasn't been set yet, if you use an agent.
		conversationManager.AppendMessages(conversation.NewChatMessage(
			conversation.RoleSystem,
			systemPromptBuffer.String(),
		))
	}

	for _, message_ := range g.Messages {
		switch content := message_.Content.(type) {
		case *conversation.ChatMessageContent:
			messageTemplate, err := templating.CreateTemplate("message").Parse(content.Text)
			if err != nil {
				return err
			}

			var messageBuffer strings.Builder
			err = messageTemplate.Execute(&messageBuffer, ps)
			if err != nil {
				return err
			}
			s_ := messageBuffer.String()

			conversationManager.AppendMessages(conversation.NewChatMessage(
				content.Role, s_, conversation.WithTime(message_.Time)))
		}
	}

	// render the prompt
	if g.Prompt != "" {
		// TODO(manuel, 2023-02-04) All this could be handle by some prompt renderer kind of thing
		promptTemplate, err := templating.CreateTemplate("prompt").Parse(g.Prompt)
		if err != nil {
			return err
		}

		// TODO(manuel, 2023-02-04) This is where multisteps would work differently, since
		// the prompt would be rendered at execution time
		var promptBuffer strings.Builder
		err = promptTemplate.Execute(&promptBuffer, ps)
		if err != nil {
			return err
		}

		images := []*conversation.ImageContent{}
		for _, img := range helperSettings.Images {
			image, err := conversation.NewImageContentFromFile(img.Path)
			if err != nil {
				return err
			}

			images = append(images, image)
		}
		initialPrompt := promptBuffer.String()
		messageContent := &conversation.ChatMessageContent{
			Role:   conversation.RoleUser,
			Text:   initialPrompt,
			Images: images,
		}
		conversationManager.AppendMessages(conversation.NewMessage(messageContent))
	}

	return nil
}

func (g *GeppettoCommand) startInitialStep(
	ctx context.Context,
	cmdCtx *cmdcontext.CommandContext,
) (steps.StepResult[*conversation.Message], error) {
	chatStep, err := cmdCtx.StepFactory.NewStep(chat.WithPublishedTopic(cmdCtx.Router.Publisher, "chat"))
	if err != nil {
		return nil, err
	}

	conversation_ := cmdCtx.ConversationManager.GetConversation()
	if cmdCtx.Settings.PrintPrompt {
		fmt.Println(conversation_.GetSinglePrompt())
		return nil, nil
	}

	messagesM := steps.Resolve(conversation_)
	m := steps.Bind[conversation.Conversation, *conversation.Message](ctx, messagesM, chatStep)

	return m, nil
}

// RunIntoWriter runs the command and writes the output into the given writer.
func (g *GeppettoCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	w io.Writer,
) error {
	if g.Prompt != "" && len(g.Messages) != 0 {
		return errors.Errorf("Prompt and messages are mutually exclusive")
	}

	cmdCtx, err := cmdcontext.NewCommandContextFromLayers(parsedLayers, g.StepSettings)
	if err != nil {
		return err
	}
	defer func() {
		err := cmdCtx.Router.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to close pubSub")
		}
	}()

	val, present := parsedLayers.Get(layers.DefaultSlug)
	if !present {
		return errors.New("could not get default layer")
	}

	err = g.initializeConversationManager(cmdCtx.ConversationManager, cmdCtx.Settings, val.Parameters.ToMap())
	if err != nil {
		return err
	}

	// load and render the system prompt
	if cmdCtx.Settings.System != "" {
		g.SystemPrompt = cmdCtx.Settings.System
	}

	// load and render messages
	if cmdCtx.Settings.MessageFile != "" {
		messages_, err := conversation.LoadFromFile(cmdCtx.Settings.MessageFile)
		if err != nil {
			return err
		}

		g.Messages = messages_
	}

	if cmdCtx.Settings.AppendMessageFile != "" {
		messages_, err := conversation.LoadFromFile(cmdCtx.Settings.AppendMessageFile)
		if err != nil {
			return err
		}
		g.Messages = append(g.Messages, messages_...)
	}

	if cmdCtx.Settings.StartInChat {
		return g.runChatMode(ctx, cmdCtx)
	}
	return g.runNonChatMode(ctx, cmdCtx, w)
}

func (g *GeppettoCommand) setupPrinter(w io.Writer, settings *cmdlayers.HelpersSettings) func(msg *message.Message) error {
	if settings.Output != "text" || settings.WithMetadata || settings.FullOutput {
		return chat.NewStructuredPrinter(w, chat.PrinterOptions{
			Format:          chat.PrinterFormat(settings.Output),
			Name:            "",
			IncludeMetadata: settings.WithMetadata,
			Full:            settings.FullOutput,
		})
	}
	return chat.StepPrinterFunc("", w)
}

func chat_(
	ctx context.Context,
	step chat.Step,
	router *events.EventRouter,
	contextManager conversation.Manager,
	autoStartBackend bool,
) error {
	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())

	options := []tea.ProgramOption{
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	}
	if !isOutputTerminal {
		options = append(options, tea.WithOutput(os.Stderr))
	} else {
		options = append(options, tea.WithAltScreen())
	}

	backend := ui.NewStepBackend(step)

	model := bobatea_chat.InitialModel(
		contextManager,
		backend,
		bobatea_chat.WithTitle("pinocchio"),
		bobatea_chat.WithAutoStartBackend(autoStartBackend),
	)

	p := tea.NewProgram(
		model,
		options...,
	)

	router.AddHandler("ui", "ui", ui.StepChatForwardFunc(p))
	err := router.RunHandlers(ctx)
	if err != nil {
		return err
	}

	if _, err = p.Run(); err != nil {
		return err
	}
	return nil
}

func (g *GeppettoCommand) runChatMode(ctx context.Context, cmdCtx *cmdcontext.CommandContext) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer cancel()
		return cmdCtx.Router.Run(ctx)
	})

	eg.Go(func() error {
		defer cancel()

		cmdCtx.StepFactory.Settings.Chat.Stream = true
		chatStep, err := cmdCtx.StepFactory.NewStep(chat.WithPublishedTopic(cmdCtx.Router.Publisher, "ui"))
		if err != nil {
			return err
		}

		err = chat_(ctx, chatStep, cmdCtx.Router, cmdCtx.ConversationManager, true)
		if err != nil {
			return err
		}

		return nil
	})

	return eg.Wait()
}

func (g *GeppettoCommand) runNonChatMode(
	ctx context.Context,
	cmdCtx *cmdcontext.CommandContext,
	w io.Writer,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	printer := g.setupPrinter(w, cmdCtx.Settings)
	cmdCtx.Router.AddHandler("chat", "chat", printer)

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer cancel()
		return cmdCtx.Router.Run(ctx)
	})

	eg.Go(func() error {
		defer cancel()

		m, err := g.startInitialStep(ctx, cmdCtx)
		if err != nil {
			return err
		}

		endedInNewline := false
		res := m.Return()
		for _, msg := range res {
			s, err := msg.Value()
			if err != nil {
				return err
			}
			cmdCtx.ConversationManager.AppendMessages(s)

			endedInNewline = strings.HasSuffix(s.Content.String(), "\n")
		}

		if err := g.handleInteractiveContinuation(ctx, cmdCtx, endedInNewline, w); err != nil {
			return err
		}

		return nil
	})

	return eg.Wait()
}

func (g *GeppettoCommand) handleInteractiveContinuation(
	ctx context.Context,
	cmdCtx *cmdcontext.CommandContext,
	endedInNewline bool,
	_ io.Writer,
) error {
	isOutputTerminal := isatty.IsTerminal(os.Stdout.Fd())
	forceInteractive := cmdCtx.Settings.ForceInteractive

	// Skip interactive mode if non-interactive is set
	if cmdCtx.Settings.NonInteractive {
		cmdCtx.Settings.Interactive = false
	}

	continueInChat := cmdCtx.Settings.Interactive
	askChat := (isOutputTerminal || forceInteractive) && cmdCtx.Settings.Interactive && !cmdCtx.Settings.NonInteractive

	lengthBeforeChat := len(cmdCtx.ConversationManager.GetConversation())

	if askChat {
		if !endedInNewline {
			fmt.Println()
		}

		var err error
		continueInChat, err = g.askForChatContinuation(continueInChat)
		if err != nil {
			return err
		}
	}

	if continueInChat {
		cmdCtx.StepFactory.Settings.Chat.Stream = true
		chatStep, err := cmdCtx.StepFactory.NewStep(
			chat.WithPublishedTopic(cmdCtx.Router.Publisher, "ui"),
		)
		if err != nil {
			return err
		}

		err = chat_(ctx, chatStep, cmdCtx.Router, cmdCtx.ConversationManager, false)
		if err != nil {
			return err
		}

		fmt.Printf("\n---\n")
		for idx, msg := range cmdCtx.ConversationManager.GetConversation() {
			if idx < lengthBeforeChat {
				continue
			}
			view := msg.Content.View()
			fmt.Printf("\n%s\n", view)
		}
	}

	return nil
}

func (g *GeppettoCommand) askForChatContinuation(continueInChat bool) (bool, error) {
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
