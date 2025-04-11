package main

import (
	"context"
	"os"

	"github.com/ThreeDotsLabs/watermill/message"
	bobachat "github.com/go-go-golems/bobatea/pkg/chat"
	clay "github.com/go-go-golems/clay/pkg"
	geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/chat"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/openai"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/chatrunner"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type ToolUiCommand struct {
	*glazed_cmds.CommandDescription
}

var _ glazed_cmds.BareCommand = (*ToolUiCommand)(nil)

func NewToolUiCommand() (*ToolUiCommand, error) {
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		return nil, err
	}
	geppettoLayers, err := pinocchio_cmds.CreateGeppettoLayers(stepSettings, pinocchio_cmds.WithHelpersLayer())
	if err != nil {
		return nil, err
	}

	return &ToolUiCommand{
		CommandDescription: glazed_cmds.NewCommandDescription(
			"tool-ui",
			glazed_cmds.WithShort("Tool UI Example using ChatRunner"),
			glazed_cmds.WithFlags(
				parameters.NewParameterDefinition(
					"ui",
					parameters.ParameterTypeBool,
					parameters.WithDefault(false),
					parameters.WithHelp("start in interactive chat UI mode")),
			),
			glazed_cmds.WithLayersList(geppettoLayers...),
		),
	}, nil
}

type ToolUiSettings struct {
	UI bool `glazed.parameter:"ui"`
}

func (t *ToolUiCommand) Run(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
) error {
	settings_ := &ToolUiSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, settings_)
	if err != nil {
		return err
	}

	stepSettings, err := settings.NewStepSettingsFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}
	stepSettings.Chat.Stream = true
	engine := "gpt-4o-mini"
	stepSettings.Chat.Engine = &engine
	apiType := types.ApiTypeOpenAI
	stepSettings.Chat.ApiType = &apiType

	manager := geppetto_conversation.NewManager(
		geppetto_conversation.WithMessages(
			geppetto_conversation.NewChatMessage(
				geppetto_conversation.RoleUser,
				"Give me the weather in Boston on november 9th 1924, please, including the windspeed for me, an old ass american. Also, the weather in paris today, with temperature.",
			),
		))

	reflector := &jsonschema.Reflector{
		DoNotReference: true,
	}
	err = reflector.AddGoComments("github.com/go-go-golems/pinocchio", "./cmd/experiments/tool-ui")
	if err != nil {
		log.Warn().Err(err).Msg("Could not add go comments")
	}

	stepFactory := func(publisher message.Publisher, topic string) (chat.Step, error) {
		toolStep, err := openai.NewChatToolStep(
			stepSettings.Clone(),
			openai.WithReflector(reflector),
			openai.WithToolFunctions(map[string]any{
				"getWeather":      getWeather,
				"getWeatherOnDay": getWeatherOnDay,
			}),
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create tool step")
		}

		if publisher != nil && topic != "" {
			err = toolStep.AddPublishedTopic(publisher, topic)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to add published topic %s", topic)
			}
		}
		return toolStep, nil
	}

	var mode chatrunner.RunMode
	if settings_.UI {
		mode = chatrunner.RunModeChat
	} else {
		mode = chatrunner.RunModeBlocking
	}

	builder := chatrunner.NewChatBuilder().
		WithMode(mode).
		WithManager(manager).
		WithStepFactory(stepFactory).
		WithContext(ctx).
		WithOutputWriter(os.Stdout).
		WithUIOptions(bobachat.WithTitle("Tool UI Chat"))

	session, err := builder.Build()
	if err != nil {
		return errors.Wrap(err, "failed to build chat session")
	}

	err = session.Run()
	if err != nil {
		return errors.Wrap(err, "chat session failed")
	}

	return nil
}

func main() {
	toolUiCommand, err := NewToolUiCommand()
	cobra.CheckErr(err)

	toolUICobraCommand, err := pinocchio_cmds.BuildCobraCommandWithGeppettoMiddlewares(toolUiCommand)
	cobra.CheckErr(err)

	err = clay.InitViper("pinocchio", toolUICobraCommand)
	cobra.CheckErr(err)

	err = toolUICobraCommand.Execute()
	cobra.CheckErr(err)
}
