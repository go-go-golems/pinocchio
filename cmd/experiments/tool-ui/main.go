package main

import (
	"context"
	"os"

	bobachat "github.com/go-go-golems/bobatea/pkg/chat"
	clay "github.com/go-go-golems/clay/pkg"
	geppetto_conversation "github.com/go-go-golems/geppetto/pkg/conversation"
	"github.com/go-go-golems/geppetto/pkg/inference"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/pinocchio/pkg/chatrunner"
	pinocchio_cmds "github.com/go-go-golems/pinocchio/pkg/cmds"
	"github.com/pkg/errors"
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

	// Create engine factory
	// NOTE: For now, tools are not fully implemented in the Engine interface
	// This is a simplified version that will need to be enhanced when tool support is added
	engineFactory := inference.NewStandardEngineFactory()

	var mode chatrunner.RunMode
	if settings_.UI {
		mode = chatrunner.RunModeChat
	} else {
		mode = chatrunner.RunModeBlocking
	}

	builder := chatrunner.NewChatBuilder().
		WithMode(mode).
		WithManager(manager).
		WithEngineFactory(engineFactory).
		WithSettings(stepSettings).
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
