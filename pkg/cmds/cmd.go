package cmds

import (
	"context"
	_ "embed"
	"io"
	"os"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdcontext"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/pkg/errors"
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
	StepSettings                   *settings.StepSettings  `yaml:"stepSettings,omitempty"`
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
	settings *settings.StepSettings,
	options ...PinocchioCommandOption,
) (*PinocchioCommand, error) {
	helpersParameterLayer, err := cmdlayers.NewHelpersParameterLayer()
	if err != nil {
		return nil, err
	}

	description.Layers.PrependLayers(helpersParameterLayer)

	ret := &PinocchioCommand{
		CommandDescription: description,
		StepSettings:       settings,
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

// CreateCommandContextFromParsedLayers creates a new command context from the parsed layers
func (g *PinocchioCommand) CreateCommandContextFromParsedLayers(
	parsedLayers *layers.ParsedLayers,
) (*cmdcontext.CommandContext, *cmdcontext.ConversationContext, error) {
	if g.Prompt != "" && len(g.Messages) != 0 {
		return nil, nil, errors.Errorf("Prompt and messages are mutually exclusive")
	}

	val, present := parsedLayers.Get(layers.DefaultSlug)
	if !present {
		return nil, nil, errors.New("could not get default layer")
	}

	helpersSettings := &cmdlayers.HelpersSettings{}
	err := parsedLayers.InitializeStruct(cmdlayers.GeppettoHelpersSlug, helpersSettings)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize settings")
	}

	// Update step settings from parsed layers
	// NOTE: I think these StepSettings stored in the command were a way to provide base stuff to be overridden later
	// or they were a previous attempt to make it easier to run commands from code (see experiments/agent/uppercase.go)
	stepSettings := g.StepSettings.Clone()
	err = stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return nil, nil, err
	}

	// Create conversation context options from helperSettings
	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	options := []cmdcontext.ConversationContextOption{
		cmdcontext.WithImages(imagePaths),
		cmdcontext.WithAutosaveSettings(cmdcontext.AutosaveSettings{
			Enabled:  strings.ToLower(helpersSettings.Autosave.Enabled) == "yes",
			Template: helpersSettings.Autosave.Template,
			Path:     helpersSettings.Autosave.Path,
		}),
	}

	return g.CreateCommandContextFromSettings(
		stepSettings,
		val.Parameters.ToMap(),
		options...,
	)
}

// CreateConversationContext creates a new conversation context with the given settings
func (g *PinocchioCommand) CreateConversationContext(
	variables map[string]interface{},
	options ...cmdcontext.ConversationContextOption,
) (*cmdcontext.ConversationContext, error) {
	if g.Prompt != "" && len(g.Messages) != 0 {
		return nil, errors.Errorf("Prompt and messages are mutually exclusive")
	}

	defaultOptions := []cmdcontext.ConversationContextOption{
		cmdcontext.WithSystemPrompt(g.SystemPrompt),
		cmdcontext.WithMessages(g.Messages),
		cmdcontext.WithPrompt(g.Prompt),
		cmdcontext.WithVariables(variables),
	}

	// Combine default options with provided options, with provided options taking precedence
	return cmdcontext.NewConversationContext(append(defaultOptions, options...)...)
}

// CreateCommandContextFromSettings creates a new command context directly from settings
func (g *PinocchioCommand) CreateCommandContextFromSettings(
	stepSettings *settings.StepSettings,
	variables map[string]interface{},
	options ...cmdcontext.ConversationContextOption,
) (*cmdcontext.CommandContext, *cmdcontext.ConversationContext, error) {
	conversationContext, err := g.CreateConversationContext(variables, options...)
	if err != nil {
		return nil, nil, err
	}

	cmdCtx, err := cmdcontext.NewCommandContextFromSettings(
		stepSettings,
		conversationContext.GetManager(),
		nil, // helpersSettings is no longer needed here
	)
	if err != nil {
		return nil, nil, err
	}

	return cmdCtx, conversationContext, nil
}

// RunWithSettings runs the command with the given settings and variables
func (g *PinocchioCommand) RunWithSettings(
	ctx context.Context,
	stepSettings *settings.StepSettings,
	variables map[string]interface{},
	w io.Writer,
	options ...cmdcontext.ConversationContextOption,
) error {
	cmdCtx, _, err := g.CreateCommandContextFromSettings(
		g.StepSettings,
		variables,
		options...,
	)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	return cmdCtx.Run(ctx, w)
}

// RunStepBlockingWithSettings runs the command in blocking mode with the given settings and variables
func (g *PinocchioCommand) RunStepBlockingWithSettings(
	ctx context.Context,
	stepSettings *settings.StepSettings,
	variables map[string]interface{},
	options ...cmdcontext.ConversationContextOption,
) ([]*conversation.Message, error) {

	cmdCtx, _, err := g.CreateCommandContextFromSettings(
		stepSettings,
		variables,
		options...,
	)
	if err != nil {
		return nil, err
	}
	defer func(cmdCtx *cmdcontext.CommandContext) {
		_ = cmdCtx.Close()
	}(cmdCtx)

	cmdCtx.Router.AddHandler("chat", "chat", chat.StepPrinterFunc("", os.Stdout))

	return cmdCtx.RunStepBlocking(ctx)
}

// RunIntoWriter runs the command and writes the output into the given writer.
func (g *PinocchioCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	w io.Writer,
) error {
	cmdCtx, _, err := g.CreateCommandContextFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}
	defer func(cmdCtx *cmdcontext.CommandContext) {
		_ = cmdCtx.Close()
	}(cmdCtx)

	return cmdCtx.Run(ctx, w)
}
