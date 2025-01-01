package cmds

import (
	"context"
	_ "embed"
	"io"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdcontext"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/pkg/errors"
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

// XXX this is a mess with all its run methods and all, it would be good to have a RunOption pattern here:
// - WithStepSettings
// - WithParsedLayers
// - WithPrinter / Handlers
// - potentially others
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
func (g *GeppettoCommand) CreateCommandContextFromParsedLayers(
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
	stepSettings := g.StepSettings.Clone()
	err = stepSettings.UpdateFromParsedLayers(parsedLayers)
	if err != nil {
		return nil, nil, err
	}

	return g.CreateCommandContextFromSettings(
		helpersSettings,
		stepSettings,
		val.Parameters.ToMap(),
	)
}

// CreateCommandContextFromSettings creates a new command context directly from settings
func (g *GeppettoCommand) CreateCommandContextFromSettings(
	helpersSettings *cmdlayers.HelpersSettings,
	stepSettings *settings.StepSettings,
	variables map[string]interface{},
) (*cmdcontext.CommandContext, *cmdcontext.ConversationContext, error) {
	if g.Prompt != "" && len(g.Messages) != 0 {
		return nil, nil, errors.Errorf("Prompt and messages are mutually exclusive")
	}

	imagePaths := make([]string, len(helpersSettings.Images))
	for i, img := range helpersSettings.Images {
		imagePaths[i] = img.Path
	}

	conversationContext, err := cmdcontext.NewConversationContext(
		cmdcontext.WithSystemPrompt(g.SystemPrompt),
		cmdcontext.WithMessages(g.Messages),
		cmdcontext.WithPrompt(g.Prompt),
		cmdcontext.WithVariables(variables),
		cmdcontext.WithImages(imagePaths),
		cmdcontext.WithAutosaveSettings(cmdcontext.AutosaveSettings{
			Enabled:  strings.ToLower(helpersSettings.Autosave.Enabled) == "yes",
			Template: helpersSettings.Autosave.Template,
			Path:     helpersSettings.Autosave.Path,
		}),
	)
	if err != nil {
		return nil, nil, err
	}

	cmdCtx, err := cmdcontext.NewCommandContextFromSettings(
		stepSettings,
		conversationContext.GetManager(),
		helpersSettings,
	)
	if err != nil {
		return nil, nil, err
	}

	return cmdCtx, conversationContext, nil
}

// RunWithSettings runs the command with the given settings and variables
func (g *GeppettoCommand) RunWithSettings(
	ctx context.Context,
	helpersSettings *cmdlayers.HelpersSettings,
	variables map[string]interface{},
	w io.Writer,
) error {
	cmdCtx, _, err := g.CreateCommandContextFromSettings(helpersSettings, g.StepSettings, variables)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	return cmdCtx.Run(ctx, w)
}

// RunStepBlockingWithSettings runs the command in blocking mode with the given settings and variables
func (g *GeppettoCommand) RunStepBlockingWithSettings(
	ctx context.Context,
	helpersSettings *cmdlayers.HelpersSettings,
	variables map[string]interface{},
) ([]*conversation.Message, error) {
	cmdCtx, _, err := g.CreateCommandContextFromSettings(helpersSettings, g.StepSettings, variables)
	if err != nil {
		return nil, err
	}
	defer cmdCtx.Close()

	return cmdCtx.RunStepBlocking(ctx)
}

// RunIntoWriter runs the command and writes the output into the given writer.
func (g *GeppettoCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	w io.Writer,
) error {
	cmdCtx, _, err := g.CreateCommandContextFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}
	defer cmdCtx.Close()

	return cmdCtx.Run(ctx, w)
}
